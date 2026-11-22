#include "project_b/engine/common/renderer_client.h"

#include <vector>

#include "base/command_line.h"
#include "base/containers/flat_map.h"
#include "base/functional/bind.h"
#include "base/json/string_escape.h"
#include "base/no_destructor.h"
#include "content/public/renderer/render_frame.h"
#include "content/public/renderer/render_frame_observer.h"
#include "content/public/renderer/render_thread.h"
#include "content/public/renderer/render_thread_observer.h"
#include "base/task/single_thread_task_runner.h"
#include "ipc/ipc_channel_proxy.h"
#include "mojo/public/cpp/bindings/associated_receiver.h"
#include "mojo/public/cpp/bindings/associated_remote.h"
#include "mojo/public/cpp/bindings/associated_receiver_set.h"
#include "project_b/engine/common/ad_rules.h"
#include "project_b/engine/common/worlds.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_provider.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_registry.h"
#include "third_party/blink/public/platform/web_isolated_world_info.h"
#include "third_party/blink/public/platform/web_security_origin.h"
#include "third_party/blink/public/platform/web_string.h"
#include "third_party/blink/public/web/web_document.h"
#include "third_party/blink/public/web/web_local_frame.h"
#include "third_party/blink/public/web/web_script_source.h"
#include "v8/include/v8-context.h"
#include "v8/include/v8-function.h"
#include "v8/include/v8-function-callback.h"
#include "v8/include/v8-isolate.h"
#include "v8/include/v8-local-handle.h"
#include "v8/include/v8-object.h"
#include "v8/include/v8-primitive.h"
#include "net/base/net_errors.h"
#include "net/url_request/redirect_info.h"
#include "project_b/engine/common/block_rules.h"
#include "project_b/engine/common/print_helper.h"
#include "services/network/public/cpp/resource_request.h"
#include "third_party/blink/public/common/loader/url_loader_throttle.h"
#include "third_party/blink/public/platform/url_loader_throttle_provider.h"

namespace project_b {

namespace {

// Tells the browser of each request the ad blocker refused, by its frame, so the frame's tab
// counts it. Throttles run on more than one thread; the browser is told from the main one.
class Reporter {
 public:
  static Reporter& Get() {
    static base::NoDestructor<Reporter> reporter;
    return *reporter;
  }

  // On the main thread, as the renderer starts.
  void Start() {
    main_ = base::SingleThreadTaskRunner::GetCurrentDefault();
    content::RenderThread::Get()->GetChannel()->GetRemoteAssociatedInterface(&remote_);
  }

  // Any thread.
  void Blocked(const blink::LocalFrameToken& frame) {
    if (!main_) {
      return;
    }
    main_->PostTask(FROM_HERE, base::BindOnce(
                                   [](base::UnguessableToken frame) {
                                     Reporter& r = Get();
                                     if (r.remote_) {
                                       r.remote_->Blocked(frame);
                                     }
                                   },
                                   frame.value()));
  }

 private:
  scoped_refptr<base::SingleThreadTaskRunner> main_;
  mojo::AssociatedRemote<mojom::AdReports> remote_;
};

// Fails a request to a blocked host as it starts or as it is redirected there, as the
// administrator's; and one the ad blocker keeps from its host, as the client's, which the
// browser counts for the tab.
class BlockThrottle : public blink::URLLoaderThrottle {
 public:
  explicit BlockThrottle(std::optional<blink::LocalFrameToken> frame)
      : frame_(frame) {}

  // A synchronous load, as a synchronous XMLHttpRequest or a worker's importScripts, runs
  // its throttles on another thread. This one keeps nothing tied to a thread (Reporter
  // posts to the main one), so it may go; the default refuses, which kills the page.
  void DetachFromCurrentSequence() override {}

  void WillStartRequest(network::ResourceRequest* request,
                        bool* defer) override {
    if (block_rules::Blocks(request->url)) {
      delegate_->CancelWithError(net::ERR_BLOCKED_BY_ADMINISTRATOR);
    } else if (ad_rules::Blocks(*request)) {
      if (frame_) {
        Reporter::Get().Blocked(*frame_);
      }
      delegate_->CancelWithError(net::ERR_BLOCKED_BY_CLIENT);
    }
  }

  void WillRedirectRequest(
      net::RedirectInfo* redirect_info,
      const network::mojom::URLResponseHead& response_head,
      bool* defer,
      network::HttpRequestHeadersUpdateParams* headers_update_params) override {
    if (block_rules::Blocks(redirect_info->new_url)) {
      delegate_->CancelWithError(net::ERR_BLOCKED_BY_ADMINISTRATOR);
    }
  }

 private:
  // The frame whose request it is; none for a worker's.
  std::optional<blink::LocalFrameToken> frame_;
};

class BlockThrottleProvider : public blink::URLLoaderThrottleProvider {
 public:
  std::unique_ptr<blink::URLLoaderThrottleProvider> Clone() override {
    return std::make_unique<BlockThrottleProvider>();
  }

  std::vector<std::unique_ptr<blink::URLLoaderThrottle>> CreateThrottles(
      base::optional_ref<const blink::LocalFrameToken> local_frame_token,
      const network::ResourceRequest& request) override {
    std::vector<std::unique_ptr<blink::URLLoaderThrottle>> throttles;
    throttles.push_back(std::make_unique<BlockThrottle>(
        local_frame_token.has_value()
            ? std::optional<blink::LocalFrameToken>(*local_frame_token)
            : std::nullopt));
    return throttles;
  }

  void SetOnline(bool is_online) override {}
};

// The form script Go gave, for every frame of this renderer (docs/password-autofill.md).
std::string& FormScript() {
  static base::NoDestructor<std::string> script;
  return *script;
}

// What the form script's world has before the script runs: its transport, made over the
// one function the engine binds there, which it takes for itself so nothing else in that
// world can call it; and the way Go's messages are handed in.
constexpr char kFormBridge[] = R"((() => {
  const send = globalThis.__pbFormsSend;
  delete globalThis.__pbFormsSend;
  let handler = null;
  globalThis.__pbFormsReceive = (kind, json) => handler && handler(kind, JSON.parse(json));
  globalThis.__pbFormsTransport = {
    send: (kind, payload) => send(kind, JSON.stringify(payload)),
    receive: (h) => { handler = h; },
  };
})();
)";

// Takes the ad blocker's rules and the form script from the browser.
class Rules : public content::RenderThreadObserver,
              public mojom::Renderer {
 public:
  void RegisterMojoInterfaces(
      blink::AssociatedInterfaceRegistry* registry) override {
    registry->AddInterface<mojom::Renderer>(
        base::BindRepeating(&Rules::Bind, base::Unretained(this)));
  }
  void UnregisterMojoInterfaces(
      blink::AssociatedInterfaceRegistry* registry) override {
    registry->RemoveInterface(mojom::Renderer::Name_);
  }

  void SetAdRules(const std::vector<std::string>& hosts,
                  const std::vector<std::string>& off) override {
    ad_rules::Set(hosts, off);
  }

  void SetFormScript(const std::string& source) override { FormScript() = source; }

 private:
  void Bind(mojo::PendingAssociatedReceiver<mojom::Renderer> receiver) {
    receivers_.Add(this, std::move(receiver));
  }

  mojo::AssociatedReceiverSet<mojom::Renderer> receivers_;
};

// Hands a frame's form script a message of the browser's (Forms, below).
void ToFormScript(content::RenderFrame* frame,
                  const std::string& kind,
                  const std::string& json);

// Runs a frame's sparks in its page, in the page's own world, as other userscript managers
// run a script that asks for no extension APIs, so a spark can change what the page itself
// does: those at document start, the rest once its document has loaded, at the moments content runs scripts of the embedder's (RunScriptsAtDocument*
// below), as Blink forbids script in its own callbacks. The browser tells the frame before
// the page commits; told late, a spark whose moment has passed runs at once.
class Sparks : public content::RenderFrameObserver, public mojom::Frame {
 public:
  explicit Sparks(content::RenderFrame* frame)
      : content::RenderFrameObserver(frame), frame_(frame) {
    frame->GetAssociatedInterfaceRegistry()->AddInterface<mojom::Frame>(
        base::BindRepeating(&Sparks::Bind, base::Unretained(this)));
    Frames()[frame] = this;
  }
  ~Sparks() override { Frames().erase(frame_); }

  static Sparks* Of(content::RenderFrame* frame) {
    auto it = Frames().find(frame);
    return it == Frames().end() ? nullptr : it->second;
  }

  void SetSparks(const std::string& url,
                 std::vector<mojom::SparkPtr> sparks) override {
    url_ = url;
    sparks_ = std::move(sparks);
    RunDue();
  }

  // For the frame's form script, which has it from here: this is the frame's one receiver.
  void FormMessage(const std::string& kind, const std::string& json) override {
    ToFormScript(frame_, kind, json);
  }

  void DidCommitProvisionalLoad(ui::PageTransition transition) override {
    started_ = loaded_ = false;
  }
  void AtDocumentStart() {
    started_ = true;
    RunDue();
  }
  void AtDocumentEnd() {
    loaded_ = true;
    RunDue();
  }
  void OnDestruct() override { delete this; }

 private:
  static base::flat_map<content::RenderFrame*, Sparks*>& Frames() {
    static base::NoDestructor<base::flat_map<content::RenderFrame*, Sparks*>> frames;
    return *frames;
  }

  void Bind(mojo::PendingAssociatedReceiver<mojom::Frame> receiver) {
    receiver_.reset();
    receiver_.Bind(std::move(receiver));
  }

  // Runs each spark for this page whose moment has come, once.
  void RunDue() {
    blink::WebLocalFrame* frame = render_frame()->GetWebFrame();
    if (!started_ || sparks_.empty() ||
        GURL(frame->GetDocument().Url()).spec() != url_) {
      return;
    }
    std::vector<mojom::SparkPtr> later;
    for (mojom::SparkPtr& spark : sparks_) {
      if (!spark->at_start && !loaded_) {
        later.push_back(std::move(spark));
        continue;
      }
      // Each in a function of its own, as other managers wrap them, so no two sparks, nor a
      // spark and the page, clash over a name declared at the top.
      const std::string code = "(function () {\n" + spark->source + "\n}).call(window);";
      frame->ExecuteScript(blink::WebScriptSource(blink::WebString::FromUtf8(code)));
    }
    sparks_ = std::move(later);
  }

  // Its frame, the key it is found by, which outlives its observers' notice.
  const raw_ptr<content::RenderFrame> frame_;
  std::string url_;
  std::vector<mojom::SparkPtr> sparks_;
  bool started_ = false;
  bool loaded_ = false;
  mojo::AssociatedReceiver<mojom::Frame> receiver_{this};
};

// Runs the form script in each frame's web page, at document start, in a world of its own
// that the page's scripts can't reach, and carries its messages: the script's send is a
// function bound only in that world's context, which reaches the browser as the frame's
// own FormMessage; and what the browser sends is handed to the script there.
class Forms : public content::RenderFrameObserver {
 public:
  explicit Forms(content::RenderFrame* frame)
      : content::RenderFrameObserver(frame), frame_(frame) {
    Frames()[frame] = this;
  }
  ~Forms() override { Frames().erase(frame_); }

  static Forms* Of(content::RenderFrame* frame) {
    auto it = Frames().find(frame);
    return it == Frames().end() ? nullptr : it->second;
  }

  // The world's context is made as the script first runs there: the send function goes in
  // then, and in no other world.
  void DidCreateScriptContext(v8::Local<v8::Context> context,
                              int32_t world_id) override {
    if (world_id != kFormsWorld) {
      return;
    }
    v8::Isolate* isolate = v8::Isolate::GetCurrent();
    v8::HandleScope handles(isolate);
    v8::Context::Scope scope(context);
    v8::Local<v8::Function> send;
    if (!v8::Function::New(context, &Forms::FromScript).ToLocal(&send)) {
      return;
    }
    context->Global()
        ->Set(context, v8::String::NewFromUtf8Literal(isolate, "__pbFormsSend"), send)
        .Check();
  }

  void DidCommitProvisionalLoad(ui::PageTransition transition) override {
    running_ = false;
  }

  // A web page's frame runs the script; a page of ours, or one with no address of its
  // own, runs nothing.
  void AtDocumentStart() {
    blink::WebLocalFrame* frame = render_frame()->GetWebFrame();
    if (running_ || FormScript().empty() ||
        !GURL(frame->GetDocument().Url()).SchemeIsHTTPOrHTTPS()) {
      return;
    }
    running_ = true;
    frame->ExecuteScriptInIsolatedWorld(
        kFormsWorld,
        blink::WebScriptSource(
            blink::WebString::FromUtf8(std::string(kFormBridge) + FormScript())),
        blink::BackForwardCacheAware::kAllow);
  }

  // The browser's message for the script.
  void Receive(const std::string& kind, const std::string& json) {
    if (!running_) {
      return;
    }
    const std::string code = "globalThis.__pbFormsReceive && globalThis.__pbFormsReceive(" +
                             base::GetQuotedJSONString(kind) + ", " +
                             base::GetQuotedJSONString(json) + ");";
    render_frame()->GetWebFrame()->ExecuteScriptInIsolatedWorld(
        kFormsWorld, blink::WebScriptSource(blink::WebString::FromUtf8(code)),
        blink::BackForwardCacheAware::kAllow);
  }

  void OnDestruct() override { delete this; }

 private:
  static base::flat_map<content::RenderFrame*, Forms*>& Frames() {
    static base::NoDestructor<base::flat_map<content::RenderFrame*, Forms*>> frames;
    return *frames;
  }

  // The script's send(kind, json), called in its own world: to the browser, as this
  // frame's. The frame is the one whose context called, found again each time.
  static void FromScript(const v8::FunctionCallbackInfo<v8::Value>& info) {
    if (info.Length() != 2 || !info[0]->IsString() || !info[1]->IsString()) {
      return;
    }
    blink::WebLocalFrame* web = blink::WebLocalFrame::FrameForCurrentContext();
    content::RenderFrame* frame = web ? content::RenderFrame::FromWebFrame(web) : nullptr;
    Forms* forms = frame ? Of(frame) : nullptr;
    if (!forms) {
      return;
    }
    v8::String::Utf8Value kind(info.GetIsolate(), info[0]);
    v8::String::Utf8Value json(info.GetIsolate(), info[1]);
    if (!forms->host_) {
      frame->GetRemoteAssociatedInterfaces()->GetInterface(&forms->host_);
    }
    forms->host_->FormMessage(std::string(*kind, kind.length()),
                              std::string(*json, json.length()));
  }

  // Its frame, the key it is found by, which outlives its observers' notice.
  const raw_ptr<content::RenderFrame> frame_;
  bool running_ = false;
  mojo::AssociatedRemote<mojom::FrameHost> host_;
};

void ToFormScript(content::RenderFrame* frame,
                  const std::string& kind,
                  const std::string& json) {
  if (Forms* forms = Forms::Of(frame)) {
    forms->Receive(kind, json);
  }
}

}  // namespace

RendererClient::RendererClient() {
  block_rules::SetFromCommandLine(*base::CommandLine::ForCurrentProcess());
}

RendererClient::~RendererClient() = default;

void RendererClient::RenderThreadStarted() {
  static base::NoDestructor<Rules> rules;
  content::RenderThread::Get()->AddObserver(rules.get());
  Reporter::Get().Start();
  // The form script's world, named for whoever looks at a page's worlds.
  blink::WebIsolatedWorldInfo forms_world;
  forms_world.human_readable_name = blink::WebString::FromUtf8("Project B forms");
  forms_world.stable_id = blink::WebString::FromUtf8("project-b-forms");
  blink::SetIsolatedWorldInfo(kFormsWorld, forms_world);
}

void RendererClient::RenderFrameCreated(content::RenderFrame* render_frame) {
  // Each goes with the frame, as its observer.
  new Sparks(render_frame);
  new Forms(render_frame);
  printing::Attach(render_frame);
}

void RendererClient::RunScriptsAtDocumentStart(content::RenderFrame* render_frame) {
  // The form script first, before any script of the page's or of the person's runs.
  if (Forms* forms = Forms::Of(render_frame)) {
    forms->AtDocumentStart();
  }
  if (Sparks* sparks = Sparks::Of(render_frame)) {
    sparks->AtDocumentStart();
  }
}

void RendererClient::RunScriptsAtDocumentEnd(content::RenderFrame* render_frame) {
  if (Sparks* sparks = Sparks::Of(render_frame)) {
    sparks->AtDocumentEnd();
  }
}

std::unique_ptr<blink::URLLoaderThrottleProvider>
RendererClient::CreateURLLoaderThrottleProvider(
    blink::URLLoaderThrottleProviderType provider_type) {
  return std::make_unique<BlockThrottleProvider>();
}

}  // namespace project_b

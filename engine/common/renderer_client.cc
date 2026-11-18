#include "project_b/engine/common/renderer_client.h"

#include <vector>

#include "base/command_line.h"
#include "base/containers/flat_map.h"
#include "base/functional/bind.h"
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
#include "project_b/engine/mojom/project_b.mojom.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_registry.h"
#include "third_party/blink/public/platform/web_security_origin.h"
#include "third_party/blink/public/platform/web_string.h"
#include "third_party/blink/public/web/web_document.h"
#include "third_party/blink/public/web/web_local_frame.h"
#include "third_party/blink/public/web/web_script_source.h"
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

// Takes the ad blocker's rules from the browser.
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

 private:
  void Bind(mojo::PendingAssociatedReceiver<mojom::Renderer> receiver) {
    receivers_.Add(this, std::move(receiver));
  }

  mojo::AssociatedReceiverSet<mojom::Renderer> receivers_;
};

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

}  // namespace

RendererClient::RendererClient() {
  block_rules::SetFromCommandLine(*base::CommandLine::ForCurrentProcess());
}

RendererClient::~RendererClient() = default;

void RendererClient::RenderThreadStarted() {
  static base::NoDestructor<Rules> rules;
  content::RenderThread::Get()->AddObserver(rules.get());
  Reporter::Get().Start();
}

void RendererClient::RenderFrameCreated(content::RenderFrame* render_frame) {
  new Sparks(render_frame);
  printing::Attach(render_frame);
}

void RendererClient::RunScriptsAtDocumentStart(content::RenderFrame* render_frame) {
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

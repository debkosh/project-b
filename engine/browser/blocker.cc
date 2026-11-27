#include "project_b/engine/browser/blocker.h"

#include <algorithm>
#include <memory>
#include <optional>
#include <tuple>
#include <utility>

#include "base/containers/flat_set.h"
#include "base/containers/lru_cache.h"
#include "base/containers/span.h"
#include "base/functional/bind.h"
#include "base/memory/weak_ptr.h"
#include "base/no_destructor.h"
#include "base/synchronization/lock.h"
#include "base/task/sequenced_task_runner.h"
#include "base/task/thread_pool.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/navigation_handle_user_data.h"
#include "content/public/browser/navigation_throttle.h"
#include "content/public/browser/navigation_throttle_registry.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "ipc/ipc_channel_proxy.h"
#include "mojo/public/cpp/bindings/associated_remote.h"
#include "mojo/public/cpp/bindings/receiver_set.h"
#include "mojo/public/cpp/bindings/remote.h"
#include "mojo/public/cpp/bindings/self_owned_receiver.h"
#include "mojo/public/cpp/system/data_pipe.h"
#include "net/base/data_url.h"
#include "net/base/net_errors.h"
#include "net/base/schemeful_site.h"
#include "net/http/http_response_headers.h"
#include "net/http/http_util.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/common/blocker_client.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "services/network/public/cpp/resource_request.h"
#include "services/network/public/cpp/url_loader_completion_status.h"
#include "services/network/public/cpp/url_loader_factory_builder.h"
#include "services/network/public/mojom/fetch_api.mojom-shared.h"
#include "services/network/public/mojom/url_loader.mojom.h"
#include "services/network/public/mojom/url_loader_factory.mojom.h"
#include "services/network/public/mojom/url_response_head.mojom.h"
#include "services/service_manager/public/cpp/binder_registry.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_provider.h"
#include "url/gurl.h"
#include "url/origin.h"

namespace project_b {
namespace blocker {

static_assert(mojom::kBlockerAllow == PB_BLOCKER_ALLOW);
static_assert(mojom::kBlockerBlock == PB_BLOCKER_BLOCK);
static_assert(mojom::kBlockerRedirect == PB_BLOCKER_REDIRECT);

namespace {

// The table Go set, as the engine copied it, and its watcher's script. Set on the main
// thread and read on any, each reader taking one whole copy, so no call sees half of an
// old table and half of a new one. The functions themselves may be called on any thread.
struct Table {
  base::Lock lock;
  std::optional<PBBlockerHooks> hooks;
  std::string watcher;
};

Table& TheTable() {
  static base::NoDestructor<Table> table;
  return *table;
}

std::optional<PBBlockerHooks> Hooks() {
  Table& table = TheTable();
  base::AutoLock hold(table.lock);
  return table.hooks;
}

// The sites of the pages where the blocker is off, and the origins Go named, which every
// renderer is told. Main thread.
struct Off {
  std::vector<std::string> origins;
  base::flat_set<net::SchemefulSite> sites;
};

Off& TheOff() {
  static base::NoDestructor<Off> off;
  return *off;
}

// The blocker's own sequence, where every question is asked of it: a request's verdict is
// on the way of a page's load, so it runs before background work.
scoped_refptr<base::SequencedTaskRunner> Sequence() {
  static base::NoDestructor<scoped_refptr<base::SequencedTaskRunner>> runner(
      base::ThreadPool::CreateSequencedTaskRunner(
          {base::TaskPriority::USER_BLOCKING,
           base::TaskShutdownBehavior::SKIP_ON_SHUTDOWN}));
  return *runner;
}

// A string the blocker returned, copied and given back to it.
std::string Take(const PBBlockerHooks& hooks, char* s) {
  if (!s) {
    return std::string();
  }
  std::string copy(s);
  if (hooks.free_string) {
    hooks.free_string(s);
  }
  return copy;
}

// What a frame at url hides and runs, as the blocker's JSON, "" for nothing. On the
// blocker's sequence.
std::string PageOf(const std::string& url) {
  std::optional<PBBlockerHooks> hooks = Hooks();
  if (!hooks || !hooks->page) {
    return std::string();
  }
  return Take(*hooks, hooks->page(hooks->blocker, url.c_str()));
}

// The stand-ins the blocker gave for requests it redirects, each a data: address, kept on
// the blocker's sequence until the request reaches the browser's loader factory
// (StandInFactory, below), which serves it there and the request never goes out. A page's
// requests are kept by their question, (url, source, type), as a renderer asked it: the
// same question has the same answer while the rules stand, so the entries are forgotten
// only when the rules, the blocker or the sites where it is off change. A frame's own
// navigation is kept by its navigation's id.
struct StandIns {
  base::LRUCache<std::tuple<std::string, std::string, std::string>, std::string> requests{256};
  base::LRUCache<int64_t, std::string> navigations{64};
};

StandIns& TheStandIns() {
  static base::NoDestructor<StandIns> stand_ins;
  return *stand_ins;
}

void ForgetStandIns() {
  Sequence()->PostTask(FROM_HERE, base::BindOnce([] {
                         TheStandIns().requests.Clear();
                         TheStandIns().navigations.Clear();
                       }));
}

// The blocker's verdict on a request, and what loads instead when it redirects.
struct Verdict {
  int32_t verdict = PB_BLOCKER_ALLOW;
  std::string redirect;
};

// On the blocker's sequence. A redirect to anything but a data: address, which is what the
// resources file gives, can't be served, and is a block.
Verdict VerdictOn(const std::string& url, const std::string& source, const std::string& type) {
  std::optional<PBBlockerHooks> hooks = Hooks();
  if (!hooks || !hooks->request) {
    return Verdict();
  }
  char* redirect = nullptr;
  Verdict v;
  v.verdict =
      hooks->request(hooks->blocker, url.c_str(), source.c_str(), type.c_str(), &redirect);
  v.redirect = Take(*hooks, redirect);
  if (v.verdict == PB_BLOCKER_REDIRECT && !GURL(v.redirect).SchemeIs(url::kDataScheme)) {
    v.verdict = PB_BLOCKER_BLOCK;
  }
  return v;
}

// A loader that has said all it has to: it stays until its client lets it go.
class ServedLoader : public network::mojom::URLLoader {
 public:
  void FollowRedirect(network::HttpRequestHeadersUpdateParams headers_update_params,
                      const std::optional<GURL>& new_url) override {}
  void SetPriority(net::RequestPriority priority, int32_t intra_priority_value) override {}
};

// Answers request with the stand-in at data_url, as the network would have answered it:
// the same response type the network's CORS check gives a response that passes it. A
// stand-in is harmless by design, so a request in cors mode passes, and one in no-cors
// mode from another origin, as a classic script's or an image's, is opaque, which still
// runs or draws. The network, and its CORS check, never see the request.
void Serve(mojo::PendingReceiver<network::mojom::URLLoader> loader,
           const network::ResourceRequest& request,
           const std::string& data_url,
           mojo::PendingRemote<network::mojom::URLLoaderClient> pending_client) {
  mojo::Remote<network::mojom::URLLoaderClient> client(std::move(pending_client));
  std::string mime_type, charset, data;
  if (!net::DataURL::Parse(GURL(data_url), &mime_type, &charset, &data)) {
    client->OnComplete(network::URLLoaderCompletionStatus(net::ERR_BLOCKED_BY_CLIENT));
    return;
  }
  std::string raw = "HTTP/1.1 200 OK\nContent-Type: " + mime_type;
  if (!charset.empty()) {
    raw += ";charset=" + charset;
  }
  auto head = network::mojom::URLResponseHead::New();
  const bool same_origin =
      request.mode == network::mojom::RequestMode::kNavigate ||
      (request.request_initiator && request.request_initiator->IsSameOriginWith(request.url));
  if (same_origin) {
    head->response_type = network::mojom::FetchResponseType::kBasic;
  } else if (request.mode == network::mojom::RequestMode::kNoCors) {
    head->response_type = network::mojom::FetchResponseType::kOpaque;
  } else {
    head->response_type = network::mojom::FetchResponseType::kCors;
    if (request.request_initiator) {
      raw += "\nAccess-Control-Allow-Origin: " + request.request_initiator->Serialize();
      raw += "\nAccess-Control-Allow-Credentials: true";
    }
  }
  head->headers =
      base::MakeRefCounted<net::HttpResponseHeaders>(net::HttpUtil::AssembleRawHeaders(raw));
  head->mime_type = mime_type;
  head->charset = charset;
  head->content_length = data.size();
  head->request_time = head->response_time = base::Time::Now();
  mojo::ScopedDataPipeProducerHandle producer;
  mojo::ScopedDataPipeConsumerHandle consumer;
  if (mojo::CreateDataPipe(std::max<uint32_t>(data.size(), 1), producer, consumer) !=
          MOJO_RESULT_OK ||
      (!data.empty() && producer->WriteAllData(base::as_byte_span(data)) != MOJO_RESULT_OK)) {
    client->OnComplete(network::URLLoaderCompletionStatus(net::ERR_INSUFFICIENT_RESOURCES));
    return;
  }
  producer.reset();
  client->OnReceiveResponse(std::move(head), std::move(consumer), std::nullopt);
  network::URLLoaderCompletionStatus status(net::OK);
  status.decoded_body_length = base::ByteSize(data.size());
  status.encoded_body_length = base::ByteSize(data.size());
  client->OnComplete(status);
  mojo::MakeSelfOwnedReceiver(std::make_unique<ServedLoader>(), std::move(loader));
}

// Sits in front of the network for a renderer's requests or a navigation's, on the
// blocker's sequence: a request the blocker redirected gets its stand-in here, and every
// other goes on to the network as it came, its pipes handed over, so the network talks to
// the renderer directly from then on.
class StandInFactory : public network::mojom::URLLoaderFactory {
 public:
  // navigation: the id of the navigation the factory loads, if it is a navigation's.
  static void Bind(mojo::PendingReceiver<network::mojom::URLLoaderFactory> receiver,
                   mojo::PendingRemote<network::mojom::URLLoaderFactory> network,
                   std::optional<int64_t> navigation) {
    auto* factory = new StandInFactory(std::move(network), navigation);
    factory->receivers_.Add(factory, std::move(receiver));
  }

  void CreateLoaderAndStart(
      mojo::PendingReceiver<network::mojom::URLLoader> loader,
      int32_t request_id,
      uint32_t options,
      const network::ResourceRequest& request,
      mojo::PendingRemote<network::mojom::URLLoaderClient> client,
      const net::MutableNetworkTrafficAnnotationTag& traffic_annotation) override {
    if (std::optional<std::string> stand_in = StandInFor(request)) {
      Serve(std::move(loader), request, *stand_in, std::move(client));
      return;
    }
    network_->CreateLoaderAndStart(std::move(loader), request_id, options, request,
                                   std::move(client), traffic_annotation);
  }

  void Clone(mojo::PendingReceiver<network::mojom::URLLoaderFactory> receiver) override {
    receivers_.Add(this, std::move(receiver));
  }

 private:
  StandInFactory(mojo::PendingRemote<network::mojom::URLLoaderFactory> network,
                 std::optional<int64_t> navigation)
      : network_(std::move(network)), navigation_(navigation) {
    receivers_.set_disconnect_handler(base::BindRepeating(
        &StandInFactory::Disconnected, base::Unretained(this)));
    network_.set_disconnect_handler(
        base::BindOnce(&StandInFactory::Gone, base::Unretained(this)));
  }
  ~StandInFactory() override = default;

  std::optional<std::string> StandInFor(const network::ResourceRequest& request) {
    StandIns& stand_ins = TheStandIns();
    if (navigation_) {
      auto it = stand_ins.navigations.Peek(*navigation_);
      if (it == stand_ins.navigations.end()) {
        return std::nullopt;
      }
      std::string stand_in = std::move(it->second);
      stand_ins.navigations.Erase(it);
      return stand_in;
    }
    if (stand_ins.requests.empty()) {
      return std::nullopt;
    }
    std::optional<blocker_client::Question> question = blocker_client::QuestionFor(request);
    if (!question) {
      return std::nullopt;
    }
    auto it = stand_ins.requests.Get(
        std::make_tuple(question->url, question->source, question->type));
    if (it == stand_ins.requests.end()) {
      return std::nullopt;
    }
    return it->second;
  }

  void Disconnected() {
    if (receivers_.empty()) {
      delete this;
    }
  }
  void Gone() { delete this; }

  mojo::ReceiverSet<network::mojom::URLLoaderFactory> receivers_;
  mojo::Remote<network::mojom::URLLoaderFactory> network_;
  const std::optional<int64_t> navigation_;
};

// The renderers' questions, on the blocker's sequence: each renderer's pipe is bound here.
class Answers : public mojom::Blocker {
 public:
  static Answers& Get() {
    static base::NoDestructor<Answers> answers;
    return *answers;
  }

  void Bind(mojo::PendingReceiver<mojom::Blocker> receiver) {
    receivers_.Add(this, std::move(receiver));
  }

  void Check(const std::string& url,
             const std::string& source,
             const std::string& type,
             CheckCallback callback) override {
    Verdict v = VerdictOn(url, source, type);
    if (v.verdict == PB_BLOCKER_REDIRECT) {
      TheStandIns().requests.Put(std::make_tuple(url, source, type), std::move(v.redirect));
    }
    std::move(callback).Run(v.verdict);
  }

  void Hide(const std::string& page_url,
            const std::string& query,
            HideCallback callback) override {
    std::optional<PBBlockerHooks> hooks = Hooks();
    std::string selectors;
    if (hooks && hooks->hide) {
      selectors = Take(*hooks, hooks->hide(hooks->blocker, page_url.c_str(), query.c_str()));
    }
    std::move(callback).Run(selectors.empty() ? "[]" : selectors);
  }

 private:
  mojo::ReceiverSet<mojom::Blocker> receivers_;
};

// What the blocker said of a frame's page, kept from the response until the commit.
class Parts : public content::NavigationHandleUserData<Parts> {
 public:
  ~Parts() override = default;
  const std::string& json() const { return json_; }

 private:
  Parts(content::NavigationHandle& handle, std::string json) : json_(std::move(json)) {}
  friend content::NavigationHandleUserData<Parts>;
  NAVIGATION_HANDLE_USER_DATA_KEY_DECL();

  const std::string json_;
};

NAVIGATION_HANDLE_USER_DATA_KEY_IMPL(Parts);

// The site of the page a frame's navigation is in: the frame's own for a tab's top
// frame, else its top frame's, as cookies see it.
net::SchemefulSite PageSiteOf(content::NavigationHandle* handle) {
  content::RenderFrameHost* parent = handle->GetParentFrameOrOuterDocument();
  if (handle->IsInOutermostMainFrame() || !parent) {
    return net::SchemefulSite(handle->GetURL());
  }
  return net::SchemefulSite(parent->GetOutermostMainFrame()->GetLastCommittedOrigin());
}

// The blocker's part in a frame's navigation in a tab, over http(s), where it is on, each
// step held while the blocker, on its sequence, answers. A frame inside a page is a request
// of the page, which no renderer's throttle sees: it is asked as it starts and on each
// redirect, as sub_frame, and a refused one is taken out of its page and counted for the
// tab, as a renderer's refusal is. A tab's own page is the person's, and Safe Browsing's,
// at Go's gate. And at its response every frame's page waits while the blocker says what
// it hides and runs: the response takes its time to arrive anyway, so this is a few
// hundred microseconds at most on a phone.
class FrameThrottle : public content::NavigationThrottle {
 public:
  explicit FrameThrottle(content::NavigationThrottleRegistry& registry)
      : content::NavigationThrottle(registry) {}

  ThrottleCheckResult WillStartRequest() override { return AskOfFrame(/*redirect=*/false); }
  ThrottleCheckResult WillRedirectRequest() override { return AskOfFrame(/*redirect=*/true); }

  ThrottleCheckResult WillProcessResponse() override {
    content::NavigationHandle* handle = navigation_handle();
    if (!Asked(handle)) {
      return PROCEED;
    }
    Sequence()->PostTaskAndReplyWithResult(
        FROM_HERE, base::BindOnce(&PageOf, handle->GetURL().spec()),
        base::BindOnce(&FrameThrottle::PageAnswered, weak_factory_.GetWeakPtr()));
    return DEFER;
  }

  const char* GetNameForLogging() override { return "project_b::blocker::FrameThrottle"; }

 private:
  // Whether the blocker is asked about the navigation where it goes now.
  static bool Asked(content::NavigationHandle* handle) {
    return handle->GetURL().SchemeIsHTTPOrHTTPS() &&
           hooks::TabOf(handle->GetWebContents()) > 0 && Hooks() &&
           !TheOff().sites.contains(PageSiteOf(handle));
  }

  // A frame redirected keeps its stand-in for its navigation, where the navigation's loader
  // factory serves it (StandInFactory). A redirect answered for a redirect of the frame's
  // can't be: the network follows that redirect itself, so it is a block.
  static int32_t FrameVerdict(int64_t navigation,
                              const std::string& url,
                              const std::string& source,
                              bool redirect) {
    Verdict v = VerdictOn(url, source, "sub_frame");
    if (v.verdict == PB_BLOCKER_REDIRECT) {
      if (redirect) {
        return PB_BLOCKER_BLOCK;
      }
      TheStandIns().navigations.Put(navigation, std::move(v.redirect));
    }
    return v.verdict;
  }

  ThrottleCheckResult AskOfFrame(bool redirect) {
    content::NavigationHandle* handle = navigation_handle();
    if (handle->IsInOutermostMainFrame() || !Asked(handle)) {
      return PROCEED;
    }
    const GURL source = PageSiteOf(handle).GetURL();
    if (!source.SchemeIsHTTPOrHTTPS()) {
      return PROCEED;
    }
    Sequence()->PostTaskAndReplyWithResult(
        FROM_HERE,
        base::BindOnce(&FrameVerdict, handle->GetNavigationId(), handle->GetURL().spec(),
                       source.spec(), redirect),
        base::BindOnce(&FrameThrottle::FrameAnswered, weak_factory_.GetWeakPtr()));
    return DEFER;
  }

  // A redirect is counted for the tab as a block is, and goes on to its stand-in.
  void FrameAnswered(int32_t verdict) {
    if (verdict == PB_BLOCKER_ALLOW) {
      Resume();
      return;
    }
    hooks::Args args;
    args.Int(hooks::TabOf(navigation_handle()->GetWebContents()));
    hooks::Fire(PB_EVENT_AD_BLOCKED, args);
    if (verdict == PB_BLOCKER_REDIRECT) {
      Resume();
      return;
    }
    CancelDeferredNavigation(
        ThrottleCheckResult(BLOCK_REQUEST_AND_COLLAPSE, net::ERR_BLOCKED_BY_CLIENT));
  }

  void PageAnswered(std::string json) {
    Parts::CreateForNavigationHandle(*navigation_handle(), std::move(json));
    Resume();
  }

  base::WeakPtrFactory<FrameThrottle> weak_factory_{this};
};

mojo::AssociatedRemote<mojom::Renderer> RendererOf(content::RenderProcessHost* host) {
  mojo::AssociatedRemote<mojom::Renderer> renderer;
  if (IPC::ChannelProxy* channel = host->GetChannel()) {
    channel->GetRemoteAssociatedInterface(&renderer);
  }
  return renderer;
}

// Each live renderer, told one thing.
template <typename Tell>
void TellEach(Tell tell) {
  for (auto it = content::RenderProcessHost::AllHostsIterator(); !it.IsAtEnd();
       it.Advance()) {
    if (it.GetCurrentValue()->IsInitializedAndNotDead()) {
      if (mojo::AssociatedRemote<mojom::Renderer> renderer =
              RendererOf(it.GetCurrentValue())) {
        tell(*renderer);
      }
    }
  }
}

void TellBlocker(mojom::Renderer& renderer) {
  bool on;
  std::string watcher;
  {
    Table& table = TheTable();
    base::AutoLock hold(table.lock);
    on = table.hooks.has_value();
    watcher = table.watcher;
  }
  renderer.SetBlocker(on, watcher);
}

void TellOff(mojom::Renderer& renderer) {
  renderer.SetBlockingOff(TheOff().origins);
}

}  // namespace

void Set(const PBBlockerHooks* hooks) {
  {
    Table& table = TheTable();
    base::AutoLock hold(table.lock);
    if (hooks) {
      table.hooks = *hooks;
      table.watcher = hooks->watcher_script ? hooks->watcher_script : "";
      // Go's string may go; the engine's copy is the one used.
      table.hooks->watcher_script = nullptr;
    } else {
      table.hooks.reset();
      table.watcher.clear();
    }
  }
  ForgetStandIns();
  TellEach(&TellBlocker);
}

void Changed() {
  ForgetStandIns();
  TellEach([](mojom::Renderer& renderer) { renderer.BlockerChanged(); });
}

void SetOff(const std::vector<std::string>& origins) {
  Off& off = TheOff();
  off.origins = origins;
  off.sites.clear();
  for (const std::string& origin : origins) {
    off.sites.insert(net::SchemefulSite(GURL(origin)));
  }
  ForgetStandIns();
  TellEach(&TellOff);
}

void RendererLaunching(content::RenderProcessHost* host) {
  if (mojo::AssociatedRemote<mojom::Renderer> renderer = RendererOf(host)) {
    TellBlocker(*renderer);
    TellOff(*renderer);
  }
}

void ExposeTo(service_manager::BinderRegistry* registry) {
  registry->AddInterface<mojom::Blocker>(
      base::BindRepeating([](mojo::PendingReceiver<mojom::Blocker> receiver) {
        Answers::Get().Bind(std::move(receiver));
      }),
      Sequence());
}

void AddThrottle(content::NavigationThrottleRegistry& registry) {
  registry.AddThrottle(std::make_unique<FrameThrottle>(registry));
}

void WillCreateURLLoaderFactory(content::ContentBrowserClient::URLLoaderFactoryType type,
                                std::optional<int64_t> navigation,
                                network::URLLoaderFactoryBuilder& factory_builder) {
  using Type = content::ContentBrowserClient::URLLoaderFactoryType;
  if (type != Type::kNavigation && type != Type::kDocumentSubResource &&
      type != Type::kWorkerSubResource && type != Type::kServiceWorkerSubResource) {
    return;
  }
  auto [receiver, network] = factory_builder.Append();
  Sequence()->PostTask(
      FROM_HERE,
      base::BindOnce(&StandInFactory::Bind, std::move(receiver), std::move(network),
                     type == Type::kNavigation ? navigation : std::nullopt));
}

void ReadyToCommit(content::NavigationHandle* handle) {
  Parts* parts = Parts::GetForNavigationHandle(*handle);
  if (!parts) {
    return;
  }
  mojo::AssociatedRemote<mojom::BlockerFrame> frame;
  handle->GetRenderFrameHost()->GetRemoteAssociatedInterfaces()->GetInterface(&frame);
  frame->SetPage(handle->GetURL().spec(), parts->json());
}

}  // namespace blocker
}  // namespace project_b

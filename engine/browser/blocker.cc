#include "project_b/engine/browser/blocker.h"

#include <memory>
#include <optional>
#include <utility>

#include "base/containers/flat_set.h"
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
#include "net/base/net_errors.h"
#include "net/base/schemeful_site.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/mojom/project_b.mojom.h"
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

// The verdict on a request (pb_abi.h). On the blocker's sequence.
int32_t VerdictOn(const std::string& url, const std::string& source, const std::string& type) {
  std::optional<PBBlockerHooks> hooks = Hooks();
  if (!hooks || !hooks->request) {
    return PB_BLOCKER_ALLOW;
  }
  char* redirect = nullptr;
  const int32_t verdict =
      hooks->request(hooks->blocker, url.c_str(), source.c_str(), type.c_str(), &redirect);
  // What a redirect would load instead isn't served yet, so it is blocked
  // (docs/blocking.md, Requests).
  Take(*hooks, redirect);
  return verdict;
}

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
    std::move(callback).Run(VerdictOn(url, source, type));
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

  ThrottleCheckResult WillStartRequest() override { return AskOfFrame(); }
  ThrottleCheckResult WillRedirectRequest() override { return AskOfFrame(); }

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

  ThrottleCheckResult AskOfFrame() {
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
        base::BindOnce(&VerdictOn, handle->GetURL().spec(), source.spec(), "sub_frame"),
        base::BindOnce(&FrameThrottle::FrameAnswered, weak_factory_.GetWeakPtr()));
    return DEFER;
  }

  void FrameAnswered(int32_t verdict) {
    if (verdict == PB_BLOCKER_ALLOW) {
      Resume();
      return;
    }
    hooks::Args args;
    args.Int(hooks::TabOf(navigation_handle()->GetWebContents()));
    hooks::Fire(PB_EVENT_AD_BLOCKED, args);
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
  TellEach(&TellBlocker);
}

void Changed() {
  TellEach([](mojom::Renderer& renderer) { renderer.BlockerChanged(); });
}

void SetOff(const std::vector<std::string>& origins) {
  Off& off = TheOff();
  off.origins = origins;
  off.sites.clear();
  for (const std::string& origin : origins) {
    off.sites.insert(net::SchemefulSite(GURL(origin)));
  }
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

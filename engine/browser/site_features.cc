#include "project_b/engine/browser/site_features.h"

#include <utility>

#include "base/json/json_reader.h"
#include "base/no_destructor.h"
#include "base/values.h"
#include "content/public/browser/global_routing_id.h"
#include "content/public/browser/navigation_handle.h"
#include "mojo/public/cpp/bindings/associated_receiver_set.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_registry.h"
#include "third_party/blink/public/common/tokens/tokens.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_contents_observer.h"
#include "content/public/browser/web_contents_user_data.h"
#include "ipc/ipc_channel_proxy.h"
#include "mojo/public/cpp/bindings/associated_remote.h"
#include "net/base/net_errors.h"
#include "services/network/public/cpp/resource_request.h"
#include "services/network/public/mojom/content_security_policy.mojom.h"
#include "services/network/public/mojom/parsed_headers.mojom.h"
#include "services/network/public/mojom/url_response_head.mojom.h"
#include "third_party/blink/public/common/loader/url_loader_throttle.h"
#include "url/url_constants.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_provider.h"
#include "third_party/blink/public/mojom/loader/resource_load_info.mojom.h"

namespace project_b {
namespace site_features {

namespace {

struct AdRules {
  std::vector<std::string> hosts;
  std::vector<std::string> off;
};

AdRules& Rules() {
  static base::NoDestructor<AdRules> rules;
  return *rules;
}

void Send(content::RenderProcessHost* host) {
  IPC::ChannelProxy* channel = host->GetChannel();
  if (!channel) {
    return;
  }
  mojo::AssociatedRemote<mojom::Renderer> renderer;
  channel->GetRemoteAssociatedInterface(&renderer);
  renderer->SetAdRules(Rules().hosts, Rules().off);
}

// The sparks Go says run in a frame of the tab at url, the tab's own frame when main_frame.
std::vector<mojom::SparkPtr> SparksFor(content::WebContents* contents,
                                       const GURL& url,
                                       bool main_frame) {
  std::vector<mojom::SparkPtr> sparks;
  if (!url.SchemeIsHTTPOrHTTPS() && !url.SchemeIsFile()) {
    return sparks;
  }
  hooks::Args args;
  args.Int(hooks::TabOf(contents));
  args.Str(url.spec());
  args.Int(main_frame ? 1 : 0);
  PBValue result;
  const std::string json = hooks::Ask(PB_EVENT_SPARKS_FOR, args, &result)
                               ? hooks::TakeString(result)
                               : std::string();
  std::optional<base::Value> list =
      json.empty() ? std::nullopt : base::JSONReader::Read(json, base::JSON_PARSE_RFC);
  if (!list || !list->is_list()) {
    return sparks;
  }
  for (const base::Value& v : list->GetList()) {
    if (!v.is_dict()) {
      continue;
    }
    const base::DictValue& d = v.GetDict();
    const std::string* id = d.FindString("id");
    const std::string* source = d.FindString("source");
    const std::string* run_at = d.FindString("runAt");
    if (id && source) {
      sparks.push_back(
          mojom::Spark::New(*id, *source, run_at && *run_at == "document-start"));
    }
  }
  return sparks;
}

// A policy's style rules give way: styles from anywhere, and inline ones, as other
// userscript managers ease a page's policy for the scripts they run in it. Its other
// rules stay as the site set them.
void EaseStyles(network::mojom::ContentSecurityPolicy& policy) {
  for (auto name : {network::mojom::CSPDirectiveName::StyleSrc,
                    network::mojom::CSPDirectiveName::StyleSrcElem,
                    network::mojom::CSPDirectiveName::StyleSrcAttr}) {
    auto open = network::mojom::CSPSourceList::New();
    open->allow_self = true;
    open->allow_star = true;
    open->allow_inline = true;
    for (const char* scheme : {"data", "blob"}) {
      open->sources.push_back(
          network::mojom::CSPSource::New(scheme, "", url::PORT_UNSPECIFIED, "", false, false));
    }
    policy.directives[name] = std::move(open);
    policy.raw_directives[name] = "'self' * data: blob: 'unsafe-inline'";
  }
}

// Where a spark runs, the page's policy would block the styles it adds, as the spark runs
// in the page's own world: the response's policies have their style rules eased before
// the page commits with them.
class StyleEasing : public blink::URLLoaderThrottle {
 public:
  StyleEasing(base::RepeatingCallback<content::WebContents*()> contents, bool main_frame)
      : contents_(std::move(contents)), main_frame_(main_frame) {}

  void WillProcessResponse(const GURL& response_url,
                           network::mojom::URLResponseHead* head,
                           bool* defer) override {
    if (!head->parsed_headers || head->parsed_headers->content_security_policy.empty()) {
      return;
    }
    content::WebContents* contents = contents_.Run();
    if (!contents || SparksFor(contents, response_url, main_frame_).empty()) {
      return;
    }
    for (auto& policy : head->parsed_headers->content_security_policy) {
      EaseStyles(*policy);
    }
  }

 private:
  base::RepeatingCallback<content::WebContents*()> contents_;
  const bool main_frame_;
};

// One web tab's: its frames' sparks, and its blocked requests.
class Follower : public content::WebContentsObserver,
                 public content::WebContentsUserData<Follower> {
 public:
  ~Follower() override = default;

  void ReadyToCommitNavigation(content::NavigationHandle* handle) override {
    const GURL& url = handle->GetURL();
    std::vector<mojom::SparkPtr> sparks =
        SparksFor(&GetWebContents(), url, handle->IsInPrimaryMainFrame());
    if (sparks.empty()) {
      return;
    }
    mojo::AssociatedRemote<mojom::Frame> frame;
    handle->GetRenderFrameHost()->GetRemoteAssociatedInterfaces()->GetInterface(
        &frame);
    frame->SetSparks(url.spec(), std::move(sparks));
  }

 private:
  explicit Follower(content::WebContents* contents)
      : content::WebContentsObserver(contents),
        content::WebContentsUserData<Follower>(*contents) {}
  friend class content::WebContentsUserData<Follower>;
  WEB_CONTENTS_USER_DATA_KEY_DECL();
};

WEB_CONTENTS_USER_DATA_KEY_IMPL(Follower);

// A renderer's reports: each names the frame whose request was refused, which leads to its
// tab.
class Reports : public mojom::AdReports {
 public:
  void Blocked(const base::UnguessableToken& frame) override {
    const int process = receivers_.current_context();
    content::RenderFrameHost* host = content::RenderFrameHost::FromFrameToken(
        content::GlobalRenderFrameHostToken(process, blink::LocalFrameToken(frame)));
    const int64_t tab =
        host ? hooks::TabOf(content::WebContents::FromRenderFrameHost(host)) : 0;
    if (tab <= 0) {
      return;
    }
    hooks::Args args;
    args.Int(tab);
    hooks::Fire(PB_EVENT_AD_BLOCKED, args);
  }

  void Bind(int process,
            mojo::PendingAssociatedReceiver<mojom::AdReports> receiver) {
    receivers_.Add(this, std::move(receiver), process);
  }

 private:
  mojo::AssociatedReceiverSet<mojom::AdReports, int> receivers_;
};

Reports& TheReports() {
  static base::NoDestructor<Reports> reports;
  return *reports;
}

}  // namespace

void SetAdRules(std::vector<std::string> hosts, std::vector<std::string> off) {
  Rules() = AdRules{std::move(hosts), std::move(off)};
  for (auto it = content::RenderProcessHost::AllHostsIterator(); !it.IsAtEnd();
       it.Advance()) {
    if (it.GetCurrentValue()->IsInitializedAndNotDead()) {
      Send(it.GetCurrentValue());
    }
  }
}

void RendererLaunching(content::RenderProcessHost* host) {
  Send(host);
}

void ExposeTo(blink::AssociatedInterfaceRegistry* registry,
              content::RenderProcessHost* host) {
  registry->AddInterface<mojom::AdReports>(base::BindRepeating(
      [](int process, mojo::PendingAssociatedReceiver<mojom::AdReports> r) {
        TheReports().Bind(process, std::move(r));
      },
      host->GetDeprecatedID()));
}

void Follow(content::WebContents* contents) {
  Follower::CreateForWebContents(contents);
}

std::unique_ptr<blink::URLLoaderThrottle> EaseStylesFor(
    const network::ResourceRequest& request,
    base::RepeatingCallback<content::WebContents*()> contents) {
  if (request.destination != network::mojom::RequestDestination::kDocument &&
      request.destination != network::mojom::RequestDestination::kIframe) {
    return nullptr;
  }
  return std::make_unique<StyleEasing>(std::move(contents), request.is_outermost_main_frame);
}

}  // namespace site_features
}  // namespace project_b

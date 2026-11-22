#include "project_b/engine/browser/forms.h"

#include <utility>

#include "base/no_destructor.h"
#include "base/strings/string_number_conversions.h"
#include "content/public/browser/document_user_data.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_entry.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/ssl_status.h"
#include "content/public/browser/web_contents.h"
#include "ipc/ipc_channel_proxy.h"
#include "mojo/public/cpp/bindings/associated_remote.h"
#include "net/cert/cert_status_flags.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "services/network/public/mojom/web_sandbox_flags.mojom.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_provider.h"
#include "ui/gfx/geometry/point_f.h"
#include "url/origin.h"

namespace project_b::forms {

namespace {

std::string& Script() {
  static base::NoDestructor<std::string> script;
  return *script;
}

void Tell(content::RenderProcessHost* host) {
  IPC::ChannelProxy* channel = host->GetChannel();
  if (!channel || Script().empty()) {
    return;
  }
  mojo::AssociatedRemote<mojom::Renderer> renderer;
  channel->GetRemoteAssociatedInterface(&renderer);
  renderer->SetFormScript(Script());
}

// A document's name for Go: a number of its own, new with each document, so a message
// for a document that has gone reaches no other. The frame's token alone doesn't say: a
// frame can go on to another page of its site and keep it.
class Document : public content::DocumentUserData<Document> {
 public:
  ~Document() override = default;
  const std::string& name() const { return name_; }

 private:
  explicit Document(content::RenderFrameHost* frame)
      : content::DocumentUserData<Document>(frame) {
    static uint64_t last = 0;
    name_ = base::NumberToString(++last);
  }
  friend content::DocumentUserData<Document>;
  DOCUMENT_USER_DATA_KEY_DECL();

  std::string name_;
};

DOCUMENT_USER_DATA_KEY_IMPL(Document);

}  // namespace

void SetScript(std::string source) {
  Script() = std::move(source);
  for (auto it = content::RenderProcessHost::AllHostsIterator(); !it.IsAtEnd();
       it.Advance()) {
    if (it.GetCurrentValue()->IsInitializedAndNotDead()) {
      Tell(it.GetCurrentValue());
    }
  }
}

void RendererLaunching(content::RenderProcessHost* host) {
  Tell(host);
}

void FromFrame(content::RenderFrameHost* frame,
               const std::string& kind,
               const std::string& json) {
  content::WebContents* contents =
      frame ? content::WebContents::FromRenderFrameHost(frame) : nullptr;
  const int64_t tab = contents ? hooks::TabOf(contents) : 0;
  if (tab <= 0 || !frame->IsActive()) {
    return;
  }
  const url::Origin& origin = frame->GetLastCommittedOrigin();
  // A frame of another origin anywhere above it.
  bool cross = false;
  for (content::RenderFrameHost* up = frame->GetParent(); up; up = up->GetParent()) {
    cross = cross || !up->GetLastCommittedOrigin().IsSameOriginWith(origin);
  }
  const bool opaque =
      origin.opaque() ||
      frame->IsSandboxed(network::mojom::WebSandboxFlags::kOrigin);
  // https with a certificate that checks: the page's own certificate is the one a person
  // was shown and is asked about, and a frame is https or it isn't.
  bool secure = origin.scheme() == "https";
  if (content::NavigationEntry* entry =
          contents->GetController().GetLastCommittedEntry()) {
    secure = secure && !net::IsCertStatusError(entry->GetSSL().cert_status);
  }
  // Where the frame's view begins in the tab's page: a frame of another site has a view of
  // its own, and one of the page's site shares the page's, whose corner this is. The
  // script adds its own place inside that view.
  gfx::PointF corner;
  if (content::RenderWidgetHostView* view = frame->GetView()) {
    corner = view->TransformPointToRootCoordSpaceF(gfx::PointF());
  }
  int flags = 0;
  flags |= frame->IsInPrimaryMainFrame() ? 1 : 0;
  flags |= cross ? 2 : 0;
  flags |= opaque ? 4 : 0;
  flags |= secure ? 8 : 0;

  hooks::Args args;
  args.Int(tab);
  args.Str(frame->GetFrameToken().ToString());
  args.Str(Document::GetOrCreateForCurrentDocument(frame)->name());
  args.Str(opaque ? "null" : origin.Serialize());
  args.Int(flags);
  args.Int(static_cast<int64_t>(corner.x()));
  args.Int(static_cast<int64_t>(corner.y()));
  args.Str(kind);
  args.Str(json);
  hooks::Fire(PB_EVENT_FORM_MESSAGE, args);
}

bool Send(content::WebContents* contents,
          const std::string& frame,
          const std::string& document,
          const std::string& kind,
          const std::string& json) {
  bool went = false;
  contents->ForEachRenderFrameHost(
      [&](content::RenderFrameHost* host) {
        if (went || !host->IsActive() ||
            host->GetFrameToken().ToString() != frame) {
          return;
        }
        Document* doc = Document::GetForCurrentDocument(host);
        if (!doc || doc->name() != document) {
          return;
        }
        mojo::AssociatedRemote<mojom::Frame> remote;
        host->GetRemoteAssociatedInterfaces()->GetInterface(&remote);
        remote->FormMessage(kind, json);
        went = true;
      });
  return went;
}

}  // namespace project_b::forms

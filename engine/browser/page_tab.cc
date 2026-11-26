#include "project_b/engine/browser/page_tab.h"

#include <algorithm>
#include <cstdlib>
#include <string>

#include "base/base64.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/stringprintf.h"
#include "base/strings/utf_string_conversions.h"
#include "base/task/thread_pool.h"
#include "base/values.h"
#include "build/build_config.h"
#include "components/input/native_web_keyboard_event.h"
#include "components/viz/common/frame_sinks/copy_output_result.h"
#include "content/public/browser/context_menu_params.h"
#include "content/public/browser/keyboard_event_processing_result.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_entry.h"
#include "content/public/common/url_constants.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_widget_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/ssl_status.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_ui.h"
#include "net/cert/cert_status_flags.h"
#include "net/base/net_errors.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/events.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/permissions.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "project_b/engine/browser/dialogs.h"
#include "project_b/engine/browser/file_chooser.h"
#include "third_party/blink/public/mojom/context_menu/context_menu.mojom.h"
#include "third_party/blink/public/mojom/favicon/favicon_url.mojom.h"
#include "third_party/skia/include/core/SkColor.h"
#include "ui/base/window_open_disposition.h"
#include "ui/gfx/codec/jpeg_codec.h"
#include "ui/gfx/codec/png_codec.h"
#include "ui/gfx/geometry/rect.h"
#include "ui/gfx/geometry/size_conversions.h"
#include "url/url_constants.h"

#if BUILDFLAG(IS_ANDROID)
#include "components/embedder_support/android/delegate/html_color_picker_bridge.h"
#endif

namespace project_b {

namespace {

// A page's picture for the tab switcher: a JPEG of about this quality, small enough to
// keep one for every tab, and the longest its copy may take once the compositor has the
// request before it is given up.
constexpr int kThumbnailQuality = 70;
constexpr base::TimeDelta kThumbnailTimeout = base::Seconds(2);

// What a press was on, in the words the context menu event has.
const char* MediumOf(blink::mojom::ContextMenuDataMediaType type) {
  switch (type) {
    case blink::mojom::ContextMenuDataMediaType::kImage:
      return "image";
    case blink::mojom::ContextMenuDataMediaType::kVideo:
      return "video";
    case blink::mojom::ContextMenuDataMediaType::kAudio:
      return "audio";
    case blink::mojom::ContextMenuDataMediaType::kCanvas:
      return "canvas";
    case blink::mojom::ContextMenuDataMediaType::kFile:
      return "file";
    case blink::mojom::ContextMenuDataMediaType::kPlugin:
      return "plugin";
    default:
      return "";
  }
}

// Why a navigation failed, in engine.Failure's words: the one place a net error becomes
// ours.
const char* FailureOf(net::Error error) {
  if (net::IsCertificateError(error)) {
    return "tls";
  }
  switch (error) {
    case net::ERR_NAME_NOT_RESOLVED:
    case net::ERR_NAME_RESOLUTION_FAILED:
      return "not-found";
    case net::ERR_INTERNET_DISCONNECTED:
      return "offline";
    case net::ERR_CONNECTION_REFUSED:
    case net::ERR_CONNECTION_RESET:
    case net::ERR_CONNECTION_CLOSED:
    case net::ERR_CONNECTION_ABORTED:
    case net::ERR_CONNECTION_FAILED:
      return "refused";
    case net::ERR_ADDRESS_UNREACHABLE:
    case net::ERR_ADDRESS_INVALID:
      return "unreachable";
    case net::ERR_TIMED_OUT:
    case net::ERR_CONNECTION_TIMED_OUT:
      return "timeout";
    case net::ERR_SSL_PROTOCOL_ERROR:
    case net::ERR_SSL_VERSION_OR_CIPHER_MISMATCH:
    case net::ERR_BAD_SSL_CLIENT_AUTH_CERT:
      return "tls";
    case net::ERR_PROXY_CONNECTION_FAILED:
    case net::ERR_TUNNEL_CONNECTION_FAILED:
    case net::ERR_MANDATORY_PROXY_CONFIGURATION_FAILED:
      return "proxy";
    case net::ERR_TOO_MANY_REDIRECTS:
      return "redirects";
    case net::ERR_EMPTY_RESPONSE:
    case net::ERR_INVALID_RESPONSE:
    case net::ERR_INVALID_HTTP_RESPONSE:
    case net::ERR_CONTENT_DECODING_FAILED:
    case net::ERR_RESPONSE_HEADERS_TOO_BIG:
      return "bad-response";
    case net::ERR_BLOCKED_BY_CLIENT:
    case net::ERR_BLOCKED_BY_ADMINISTRATOR:
    case net::ERR_BLOCKED_BY_RESPONSE:
    case net::ERR_BLOCKED_BY_CSP:
    case net::ERR_UNSAFE_PORT:
    case net::ERR_UNSAFE_REDIRECT:
      return "blocked";
    case net::ERR_FILE_NOT_FOUND:
    case net::ERR_ACCESS_DENIED:
      return "file";
    default:
      return "other";
  }
}

}  // namespace

PageTab::PageTab(int64_t id,
                 std::unique_ptr<content::WebContents> contents,
                 const std::string& workspace,
                 bool internal)
    : WebContentsObserverHooks(contents.get()),
      id_(id),
      workspace_(workspace),
      internal_(internal),
      contents_(std::move(contents)) {
  hooks::TagTab(contents_.get(), id_);
  contents_->SetDelegate(this);
  if (internal_) {
    contents_->SetPageBaseBackgroundColor(SK_ColorTRANSPARENT);
    // The first main frame was made inside WebContents::Create, before this observes, and
    // a page of ours may load into it.
    if (content::RenderWidgetHostView* view = contents_->GetRenderWidgetHostView()) {
      view->SetBackgroundColor(SK_ColorTRANSPARENT);
    }
  }
}

// One of our pages is transparent until it paints its own background, so the window's
// colour under it, our pages' background (window_android.cc SetColor, window_mac.mm
// PaintUnder), shows rather than white. The view belongs to the main frame, so it is set
// as each comes alive, as ShellPage does.
void PageTab::RenderFrameCreated(content::RenderFrameHost* frame) {
  if (internal_ && !frame->GetParent()) {
    if (content::RenderWidgetHostView* view = frame->GetView()) {
      view->SetBackgroundColor(SK_ColorTRANSPARENT);
    }
  }
  WebContentsObserverHooks::RenderFrameCreated(frame);
}

void PageTab::SendToPage(const std::string& json) {
  if (content::WebUI* web_ui = contents_->GetWebUI()) {
    std::vector<base::ValueView> args = {json};
    web_ui->CallJavascriptFunctionUnsafe("__shellReceive", args);
  }
}

PageTab::~PageTab() {
  // Tearing down the WebContents finishes its pending navigations, which would
  // otherwise arrive here mid-destruction.
  contents_->SetDelegate(nullptr);
  Observe(nullptr);
  contents_.reset();
}

// A page coming back from history already has its icon and may not announce it again,
// nor its title. One of our pages in a tab of ours is also on the timeline by its name,
// as the shell is.
void PageTab::DidFinishNavigation(content::NavigationHandle* handle) {
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsSameDocument()) {
    timeline::Mark("tab.committed", "tab=" + base::NumberToString(id_));
    if (internal()) {
      timeline::PageCommitted(handle->GetURL().host(), handle);
    }
  }
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsErrorPage() && !contents_->GetFaviconURLs().empty()) {
    FetchFavicon(contents_->GetFaviconURLs());
  }
  if (handle->IsInPrimaryMainFrame()) {
    ReportNavigation(handle);
  }
  WebContentsObserverHooks::DidFinishNavigation(handle);
  // After Go has the commit, so the tab it describes is the page now shown.
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted()) {
    ReportConnection();
  }
}

void PageTab::DidChangeVisibleSecurityState() {
  WebContentsObserverHooks::DidChangeVisibleSecurityState();
  ReportConnection();
}

// How the page shown reached it, in our terms (docs/daily-driver.md): content's security
// state stays here.
void PageTab::ReportConnection() {
  content::NavigationEntry* entry = contents_->GetController().GetVisibleEntry();
  int32_t state = PB_CONNECTION_NONE;
  std::string reason;
  if (entry) {
    const GURL& url = entry->GetURL();
    const content::SSLStatus& ssl = entry->GetSSL();
    if (url.SchemeIs(url::kHttpScheme)) {
      state = PB_CONNECTION_INSECURE;
      reason = "not-encrypted";
    } else if (url.SchemeIs(url::kHttpsScheme)) {
      state = PB_CONNECTION_SECURE;
      if (net::IsCertStatusError(ssl.cert_status)) {
        state = PB_CONNECTION_INSECURE;
        reason = "certificate";
      } else if (ssl.content_status &
                 (content::SSLStatus::DISPLAYED_INSECURE_CONTENT |
                  content::SSLStatus::RAN_INSECURE_CONTENT |
                  content::SSLStatus::DISPLAYED_FORM_WITH_INSECURE_ACTION)) {
        state = PB_CONNECTION_INSECURE;
        reason = "mixed-content";
      }
    }
  }
  if (state == connection_ && reason == connection_reason_) {
    return;
  }
  connection_ = state;
  connection_reason_ = reason;
  hooks::Args args;
  args.Int(id_);
  args.Int(state);
  args.Str(reason);
  hooks::Fire(PB_EVENT_CONNECTION_CHANGED, args);
}

std::string ShownURL(content::NavigationHandle* handle) {
  content::NavigationController& controller = handle->GetWebContents()->GetController();
  content::NavigationEntry* entry = handle->HasCommitted() ? controller.GetLastCommittedEntry()
                                                           : controller.GetPendingEntry();
  if (entry && entry->IsViewSourceMode()) {
    return std::string(content::kViewSourceScheme) + ":" + handle->GetURL().spec();
  }
  return handle->GetURL().spec();
}

// What a top-level navigation came to: where the tab's history stands, then the page it
// finished on, or why it failed. One a newer navigation replaced did neither.
void PageTab::ReportNavigation(content::NavigationHandle* handle) {
  {
    hooks::Args args;
    args.Int(id_);
    args.Bool(contents_->GetController().CanGoBack());
    args.Bool(contents_->GetController().CanGoForward());
    hooks::Fire(PB_EVENT_HISTORY_CHANGED, args);
  }
  if (handle->HasCommitted() && !handle->IsErrorPage()) {
    hooks::Args args;
    args.Int(id_);
    args.Int(handle->GetNavigationId());
    args.Str(ShownURL(handle));
    hooks::Fire(PB_EVENT_NAVIGATION_FINISHED, args);
    if (!Title().empty()) {
      ReportTitle();
    }
    return;
  }
  const net::Error error = handle->GetNetErrorCode();
  if (error == net::OK || error == net::ERR_ABORTED) {
    // Nothing shows instead: the navigation became a download, was stopped, or had nothing
    // to show (a 204). The tab is still at the page it shows, so it ends as a failure with
    // no failure named, at that page's address: Go's address goes back to it, rather than
    // keeping one that would load, and download again, on a restore, and History counts no
    // visit.
    hooks::Args args;
    args.Int(id_);
    args.Int(handle->GetNavigationId());
    content::NavigationEntry* shown = contents_->GetController().GetLastCommittedEntry();
    args.Str(!shown                        ? std::string()
             : shown->IsViewSourceMode() ? std::string(content::kViewSourceScheme) + ":" + shown->GetURL().spec()
                                         : contents_->GetLastCommittedURL().spec());
    args.Str(std::string());
    args.Str(std::string());
    hooks::Fire(PB_EVENT_NAVIGATION_FAILED, args);
    return;
  }
  {
    hooks::Args args;
    args.Int(id_);
    args.Int(handle->GetNavigationId());
    args.Str(ShownURL(handle));
    args.Str(FailureOf(error));
    args.Str(net::ErrorToShortString(error));
    hooks::Fire(PB_EVENT_NAVIGATION_FAILED, args);
  }
}

// The page's own title, empty when it has none: the shell falls back to the address.
std::string PageTab::Title() const {
  content::NavigationEntry* entry =
      contents_->GetController().GetLastCommittedEntry();
  return entry ? base::UTF16ToUTF8(entry->GetTitle()) : std::string();
}

void PageTab::ReportTitle() {
  hooks::Args args;
  args.Int(id_);
  args.Str(Title());
  hooks::Fire(PB_EVENT_TITLE_CHANGED, args);
}

void PageTab::Fire(PBEvent event) {
  hooks::Args args;
  args.Int(id_);
  hooks::Fire(event, args);
}

void PageTab::Fire(PBEvent event, bool value) {
  hooks::Args args;
  args.Int(id_);
  args.Bool(value);
  hooks::Fire(event, args);
}

void PageTab::DidFirstVisuallyNonEmptyPaint() {
  timeline::Mark("tab.first_paint", "tab=" + base::NumberToString(id_));
  WebContentsObserverHooks::DidFirstVisuallyNonEmptyPaint();
}

void PageTab::DidStartLoading() {
  Fire(PB_EVENT_LOADING_CHANGED, true);
  WebContentsObserverHooks::DidStartLoading();
}

void PageTab::DidStopLoading() {
  Fire(PB_EVENT_LOADING_CHANGED, false);
  WebContentsObserverHooks::DidStopLoading();
}

void PageTab::LoadProgressChanged(double progress) {
  hooks::Args args;
  args.Int(id_);
  args.Double(progress);
  hooks::Fire(PB_EVENT_LOAD_PROGRESS_CHANGED, args);
  WebContentsObserverHooks::LoadProgressChanged(progress);
}

void PageTab::DidChangeVerticalScrollDirection(
    viz::VerticalScrollDirection scroll_direction) {
  if (scroll_direction != viz::VerticalScrollDirection::kNull) {
    Fire(PB_EVENT_SCROLL_DIRECTION_CHANGED,
         scroll_direction == viz::VerticalScrollDirection::kDown);
  }
  WebContentsObserverHooks::DidChangeVerticalScrollDirection(scroll_direction);
}

void PageTab::TitleWasSet(content::NavigationEntry* entry) {
  Engine::Get()->TabTitleChanged(this);
  ReportTitle();
  WebContentsObserverHooks::TitleWasSet(entry);
}

void PageTab::OnWebContentsFocused(
    content::RenderWidgetHost* render_widget_host) {
  Engine::Get()->TabFocused(this);
  Fire(PB_EVENT_TAB_FOCUSED);
  WebContentsObserverHooks::OnWebContentsFocused(render_widget_host);
}

void PageTab::PrimaryMainFrameRenderProcessGone(base::TerminationStatus status) {
  Fire(PB_EVENT_PAGE_CRASHED);
  WebContentsObserverHooks::PrimaryMainFrameRenderProcessGone(status);
}

void PageTab::DidUpdateFaviconURL(
    content::RenderFrameHost* render_frame_host,
    const std::vector<blink::mojom::FaviconURLPtr>& candidates,
    blink::mojom::FaviconUpdateReason reason) {
  FetchFavicon(candidates);
}

// The page's plain favicon, preferring one declared at a small size. Touch icons are
// for home screens and much bigger than a tab row wants.
void PageTab::FetchFavicon(
    const std::vector<blink::mojom::FaviconURLPtr>& candidates) {
  const blink::mojom::FaviconURL* best = nullptr;
  for (const auto& candidate : candidates) {
    if (candidate->icon_type != blink::mojom::FaviconIconType::kFavicon ||
        !candidate->icon_url.is_valid()) {
      continue;
    }
    bool small = false;
    for (const gfx::Size& size : candidate->icon_sizes) {
      small = small || size.width() <= 32;
    }
    if (!best || small) {
      best = candidate.get();
    }
  }
  const int request = ++favicon_request_;
  if (!best) {
    hooks::Args args;
    args.Int(id_);
    args.Str("");
    hooks::Fire(PB_EVENT_FAVICON_CHANGED, args);
    return;
  }
  contents_->DownloadImage(
      best->icon_url, /*is_favicon=*/true, gfx::Size(32, 32),
      /*max_bitmap_size=*/128, /*bypass_cache=*/false,
      base::BindOnce(&PageTab::OnFaviconDownloaded, weak_factory_.GetWeakPtr(),
                     request));
}

void PageTab::OnFaviconDownloaded(int request,
                                  int id,
                                  int http_status_code,
                                  const GURL& image_url,
                                  const std::vector<SkBitmap>& bitmaps,
                                  const std::vector<gfx::Size>& sizes) {
  if (request != favicon_request_) {
    return;
  }
  // The one nearest 32px, which the row shows at 16px on a 2x screen.
  const SkBitmap* best = nullptr;
  for (const SkBitmap& bitmap : bitmaps) {
    if (bitmap.empty()) {
      continue;
    }
    if (!best || std::abs(bitmap.width() - 32) < std::abs(best->width() - 32)) {
      best = &bitmap;
    }
  }
  std::optional<std::vector<uint8_t>> png;
  if (best) {
    png = gfx::PNGCodec::EncodeBGRASkBitmap(*best, /*discard_transparency=*/false);
  }
  hooks::Args args;
  args.Int(id_);
  args.Str(png ? "data:image/png;base64," + base::Base64Encode(*png) : "");
  hooks::Fire(PB_EVENT_FAVICON_CHANGED, args);
}

// The copy is the compositor's, of the page's surface as last drawn, scaled down there.
// Go asks just before the tab is hidden, and on Android it still completes once it is: the
// window's one compositor keeps the surface until the copy is done (DelegatedFrameHostAndroid
// takes a ScopedKeepSurfaceAlive for it). On a Mac a tab's view has a compositor of its own,
// which goes as it is hidden, so a copy not yet made may time out there.
void PageTab::CaptureThumbnail(int max_width) {
  content::RenderWidgetHostView* view = contents_->GetRenderWidgetHostView();
  if (!view || !view->IsSurfaceAvailableForCopy() || max_width <= 0) {
    return;
  }
  const gfx::Size pixels = gfx::ScaleToCeiledSize(view->GetViewBounds().size(),
                                                  view->GetDeviceScaleFactor());
  if (pixels.IsEmpty()) {
    return;
  }
  const int width = std::min(max_width, pixels.width());
  const gfx::Size size(
      width, std::max(1, pixels.height() * width / pixels.width()));
  view->CopyFromSurface(
      gfx::Rect(), size, kThumbnailTimeout,
      base::BindOnce(&PageTab::OnThumbnailCopied, weak_factory_.GetWeakPtr(),
                     ++thumbnail_request_, base::TimeTicks::Now()));
}

namespace {

// Writes the largest of bitmaps at path as a PNG, off the main thread, making its folder.
bool WriteImage(const base::FilePath& path, const std::vector<SkBitmap>& bitmaps) {
  const SkBitmap* best = nullptr;
  for (const SkBitmap& bitmap : bitmaps) {
    if (!bitmap.drawsNothing() && (!best || bitmap.width() > best->width())) {
      best = &bitmap;
    }
  }
  if (!best) {
    return false;
  }
  std::optional<std::vector<uint8_t>> png =
      gfx::PNGCodec::EncodeBGRASkBitmap(*best, /*discard_transparency=*/false);
  return png && base::CreateDirectory(path.DirName()) && base::WriteFile(path, *png);
}

void FireImageSaved(const std::string& path, bool ok) {
  hooks::Args args;
  args.Str(path);
  args.Int(ok ? 1 : 0);
  hooks::Fire(PB_EVENT_IMAGE_SAVED, args);
}

}  // namespace

// Not bound to the tab: an image fetched just before its tab closed is still written and
// told, as Go waits on every save it asked for.
void PageTab::SaveImage(const GURL& url, const std::string& path, int max_size) {
  contents_->DownloadImage(
      url, /*is_favicon=*/false, /*preferred_size=*/gfx::Size(),
      /*max_bitmap_size=*/std::max(0, max_size), /*bypass_cache=*/false,
      base::BindOnce(
          [](const std::string& path, int id, int http_status_code, const GURL& image_url,
             const std::vector<SkBitmap>& bitmaps, const std::vector<gfx::Size>& sizes) {
            if (bitmaps.empty()) {
              LOG(WARNING) << "save image: " << image_url.possibly_invalid_spec()
                           << " gave no image, status " << http_status_code;
              FireImageSaved(path, false);
              return;
            }
            base::ThreadPool::PostTaskAndReplyWithResult(
                FROM_HERE, {base::MayBlock(), base::TaskPriority::USER_VISIBLE},
                base::BindOnce(&WriteImage, base::FilePath::FromUTF8Unsafe(path), bitmaps),
                base::BindOnce(&FireImageSaved, path));
          },
          path));
}

// The JPEG is made off the main thread: a phone's screenful takes some milliseconds.
void PageTab::OnThumbnailCopied(int request,
                                base::TimeTicks asked,
                                const content::CopyFromSurfaceResult& result) {
  if (request != thumbnail_request_) {
    return;
  }
  if (!result.has_value() || result->bitmap.drawsNothing()) {
    LOG(WARNING) << "thumbnail: tab=" << id_ << " the copy failed";
    return;
  }
  const base::TimeTicks copied = base::TimeTicks::Now();
  const SkBitmap& bitmap = result->bitmap;
  base::ThreadPool::PostTaskAndReplyWithResult(
      FROM_HERE, {base::TaskPriority::USER_VISIBLE},
      base::BindOnce(
          [](const SkBitmap& bitmap) {
            return gfx::JPEGCodec::Encode(bitmap, kThumbnailQuality);
          },
          bitmap),
      base::BindOnce(&PageTab::OnThumbnailEncoded, weak_factory_.GetWeakPtr(),
                     request, copied - asked, copied,
                     gfx::Size(bitmap.width(), bitmap.height())));
}

void PageTab::OnThumbnailEncoded(int request,
                                 base::TimeDelta copy,
                                 base::TimeTicks copied,
                                 gfx::Size size,
                                 std::optional<std::vector<uint8_t>> jpeg) {
  if (request != thumbnail_request_) {
    return;
  }
  if (!jpeg) {
    LOG(WARNING) << "thumbnail: tab=" << id_ << " the JPEG failed";
    return;
  }
  const size_t bytes = jpeg->size();
  thumbnail_ = base::MakeRefCounted<base::RefCountedBytes>(std::move(*jpeg));
  ++thumbnail_revision_;
  timeline::Mark("tab.thumbnail",
                 base::StringPrintf("tab=%lld copy=%.1fms jpeg=%.1fms %dx%d %zuKB",
                                    static_cast<long long>(id_),
                                    copy.InMillisecondsF(),
                                    (base::TimeTicks::Now() - copied).InMillisecondsF(),
                                    size.width(), size.height(), bytes / 1024));
  hooks::Args args;
  args.Int(id_);
  args.Int(thumbnail_revision_);
  hooks::Fire(PB_EVENT_THUMBNAIL, args);
}

// A link in a tab of ours navigates the tab itself, whatever it asked for: Go's one check
// on every navigation keeps it to our pages there, and takes any other address to a new
// tab beside it.
content::WebContents* PageTab::OpenURLFromTab(
    content::WebContents* source,
    const content::OpenURLParams& params,
    base::OnceCallback<void(content::NavigationHandle&)> callback) {
  content::WebContents* target = source;
  if (params.disposition != WindowOpenDisposition::kCurrentTab && !internal()) {
    // A link asked for another tab, which is in this tab's workspace. Go hears about
    // the tab before its first navigation, so the gate finds it.
    PageTab* tab = Engine::Get()->CreateTab(workspace_);
    Engine::TabOpened(id_, tab->id(), params.url,
                      params.disposition != WindowOpenDisposition::kNewBackgroundTab,
                      /*new_window=*/false, params.user_gesture);
    target = tab->contents();
  }
  base::WeakPtr<content::NavigationHandle> handle =
      target->GetController().LoadURLWithParams(
          content::NavigationController::LoadURLParams(params));
  if (handle && callback) {
    std::move(callback).Run(*handle);
  }
  return target;
}

// target=_blank and window.open arrive here with a WebContents content already made.
// Content loads it after this returns, in the tab it is given back.
//
// The new tab is in this tab's workspace. Content makes a tab that keeps its opener in the
// opener's SiteInstance, so in its workspace. One without, from target=_blank, rel=noopener
// or a page that COOP cuts off from its opener, it makes in a SiteInstance of its own in
// the profile's own storage. That one is dropped before it loads anything, and a tab made in
// this tab's workspace takes its place: content loads the WebContents returned here when
// its own is gone (kPwaNavigationCapturing, on by default), with the navigation it had for
// its own, referrer and form data included. Its sessionStorage starts empty in the
// workspace, as a tab's without an opener does anywhere, so no storage of the dropped one's
// is left with it.
//
// A tab of ours opens no window: its page's window.open goes nowhere, and the tab is asked
// to load the address itself, as its links are.
content::WebContents* PageTab::AddNewContents(
    content::WebContents* source,
    std::unique_ptr<content::WebContents> new_contents,
    const GURL& target_url,
    WindowOpenDisposition disposition,
    const blink::mojom::WindowFeatures& window_features,
    bool user_gesture,
    bool* was_blocked) {
  if (internal()) {
    new_contents.reset();
    contents_->GetController().LoadURLWithParams(
        content::NavigationController::LoadURLParams(target_url));
    return nullptr;
  }
  Engine* engine = Engine::Get();
  PageTab* tab = nullptr;
  if (engine->WorkspaceOf(new_contents.get()) == workspace_) {
    tab = engine->AdoptTab(std::move(new_contents));
  } else {
    new_contents.reset();
    tab = engine->CreateTab(workspace_);
  }
  Engine::TabOpened(id_, tab->id(), target_url,
                    disposition != WindowOpenDisposition::kNewBackgroundTab,
                    /*new_window=*/false, user_gesture);
  return tab->contents();
}

// Go gets the key before the page does, and again when the page didn't use it; one Go
// doesn't use then goes to the menu bar.
content::KeyboardEventProcessingResult PageTab::PreHandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  if (events::Key(source, event, PB_KEY_BEFORE_PAGE)) {
    return content::KeyboardEventProcessingResult::HANDLED;
  }
  return WebContentsDelegateHooks::PreHandleKeyboardEvent(source, event);
}

bool PageTab::HandleKeyboardEvent(content::WebContents* source,
                                  const input::NativeWebKeyboardEvent& event) {
  return events::Key(source, event, PB_KEY_UNHANDLED) ||
         WebContentsDelegateHooks::HandleKeyboardEvent(source, event) ||
         Window::HandleUnhandledKey(event);
}

// A page's dialogs are Go's, drawn by the shell, on a phone too.
content::JavaScriptDialogManager* PageTab::GetJavaScriptDialogManager(
    content::WebContents* source) {
  return Dialogs::Get();
}

// A page's file input goes through Go first, then opens the system's picker: the Mac's
// panel, or the phone's own. Without it content says no and the input never opens.
void PageTab::RunFileChooser(content::RenderFrameHost* render_frame_host,
                             scoped_refptr<content::FileSelectListener> listener,
                             const blink::mojom::FileChooserParams& params) {
  FileChooser::Run(id_, render_frame_host, std::move(listener), params);
}

void PageTab::EnterFullscreenModeForTab(content::RenderFrameHost* requesting_frame,
                                        const blink::mojom::FullscreenOptions& options) {
  Fire(PB_EVENT_PAGE_FULL_SCREEN, true);
  WebContentsDelegateHooks::EnterFullscreenModeForTab(requesting_frame, options);
  SystemBarsHidden(true);
  SynchronizeVisualProperties();
}

void PageTab::ExitFullscreenModeForTab(content::WebContents* web_contents) {
  Fire(PB_EVENT_PAGE_FULL_SCREEN, false);
  WebContentsDelegateHooks::ExitFullscreenModeForTab(web_contents);
  SystemBarsHidden(false);
  SynchronizeVisualProperties();
}

// Go knows whether the page has the screen: it decided how the window shows it.
bool PageTab::IsFullscreenForTabOrPending(
    const content::WebContents* web_contents) {
  hooks::Args args;
  args.Int(id_);
  PBValue result;
  if (hooks::Ask(PB_EVENT_IS_PAGE_FULL_SCREEN, args, &result)) {
    return result.i != 0;
  }
  return WebContentsDelegateHooks::IsFullscreenForTabOrPending(web_contents);
}

void PageTab::DidReceiveFindReply(int request_id,
                                  int number_of_matches,
                                  const gfx::Rect& selection_rect,
                                  int active_match_ordinal,
                                  bool final_update) {
  auto go = find_requests_.find(request_id);
  int go_request = go == find_requests_.end() ? request_id : go->second;
  if (final_update && go != find_requests_.end()) {
    find_requests_.erase(go);
  }
  hooks::Args args;
  args.Int(id_);
  args.Int(go_request);
  args.Int(number_of_matches);
  args.Int(active_match_ordinal);
  args.Bool(final_update);
  hooks::Fire(PB_EVENT_FIND_REPLY, args);
  WebContentsObserverHooks::DidReceiveFindReply(request_id, number_of_matches,
                                                selection_rect, active_match_ordinal,
                                                final_update);
}

// A right click, or a long press, in the page: Go says whether the shell draws the menu.
bool PageTab::HandleContextMenu(content::RenderFrameHost& render_frame_host,
                                const content::ContextMenuParams& params) {
  menu_referrer_ = content::Referrer(params.frame_url.GetAsReferrer(), params.referrer_policy);
  hooks::Args args;
  args.Int(id_);
  args.Double(params.x);
  args.Double(params.y);
  args.Str(params.link_url.spec());
  args.Str(base::UTF16ToUTF8(params.link_text));
  args.Str(params.src_url.spec());
  args.Str(MediumOf(params.media_type));
  args.Str(base::UTF16ToUTF8(params.selection_text));
  args.Bool(params.is_editable);
  PBValue result;
  if (hooks::Ask(PB_EVENT_CONTEXT_MENU, args, &result) && result.i != 0) {
    return true;
  }
  return WebContentsDelegateHooks::HandleContextMenu(render_frame_host, params);
}

content::Referrer PageTab::MenuReferrer(const GURL& url) const {
  return content::Referrer::SanitizeForRequest(url, menu_referrer_);
}

// A person's close ran the page's beforeunload (Engine::AskToCloseTab): its unload runs,
// and then CloseContents, only when they chose to leave. Content's default goes on either
// way, so Stay closed the tab too.
void PageTab::BeforeUnloadFired(content::WebContents* tab,
                                bool proceed,
                                bool* proceed_to_fire_unload) {
  *proceed_to_fire_unload = proceed;
}

// Content asks the tab's delegate to close or show it; Go decides.
void PageTab::CloseContents(content::WebContents* source) {
  Fire(PB_EVENT_CLOSE_REQUESTED);
  WebContentsDelegateHooks::CloseContents(source);
}

void PageTab::ActivateContents(content::WebContents* contents) {
  Fire(PB_EVENT_ACTIVATE_REQUESTED);
  WebContentsDelegateHooks::ActivateContents(contents);
}

// A phone's window always fills the screen, so a page's full screen there is its window
// without the system's bars.
void PageTab::SystemBarsHidden(bool hidden) {
  if (Engine::AppWindow* w = Engine::Get()->window(window_)) {
    w->window->SetSystemBarsHidden(hidden);
  }
}

// Content asks Go whether the page is in full screen as it sizes the page, and tells the
// page what it found.
void PageTab::SynchronizeVisualProperties() {
  if (content::RenderWidgetHostView* view = contents_->GetRenderWidgetHostView()) {
    view->GetRenderWidgetHost()->SynchronizeVisualProperties();
  }
}

bool PageTab::CheckMediaAccessPermission(
    content::RenderFrameHost* render_frame_host,
    const url::Origin& security_origin,
    blink::mojom::MediaStreamType type) {
  return Permissions::CheckMediaAccess(render_frame_host, security_origin, type);
}

void PageTab::RequestMediaAccessPermission(
    content::WebContents* web_contents,
    const content::MediaStreamRequest& request,
    content::MediaResponseCallback callback) {
  Permissions::RequestMediaAccess(web_contents, request, std::move(callback));
}

bool PageTab::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusShellForTraversal(id_, reverse);
  return true;
}

#if BUILDFLAG(IS_ANDROID)
std::unique_ptr<content::ColorChooser> PageTab::OpenColorChooser(
    content::WebContents* web_contents,
    SkColor color,
    const std::vector<blink::mojom::ColorSuggestionPtr>& suggestions) {
  return std::make_unique<web_contents_delegate_android::HtmlColorPickerBridge>(
      web_contents, color, suggestions);
}
#endif

content::WebContents* PageTab::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b

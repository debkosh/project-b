#include "project_b/engine/browser/page_tab.h"

#include <cstdlib>
#include <string>

#include "base/base64.h"
#include "base/functional/bind.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/utf_string_conversions.h"
#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/context_menu_params.h"
#include "build/build_config.h"
#include "content/public/browser/keyboard_event_processing_result.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_entry.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/render_widget_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/web_contents.h"
#include "net/base/net_errors.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/events.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#if BUILDFLAG(IS_MAC)
#include "project_b/engine/browser/dialogs.h"
#include "project_b/engine/browser/file_chooser.h"
#endif
#include "third_party/blink/public/mojom/context_menu/context_menu.mojom.h"
#include "third_party/blink/public/mojom/favicon/favicon_url.mojom.h"
#include "ui/base/window_open_disposition.h"
#include "ui/gfx/codec/png_codec.h"

namespace project_b {

namespace {

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

}  // namespace

PageTab::PageTab(int64_t id,
                 std::unique_ptr<content::WebContents> contents,
                 const std::string& workspace)
    : WebContentsObserverHooks(contents.get()),
      id_(id),
      workspace_(workspace),
      contents_(std::move(contents)) {
  hooks::TagTab(contents_.get(), id_);
  contents_->SetDelegate(this);
}

PageTab::~PageTab() {
  // Tearing down the WebContents finishes its pending navigations, which would
  // otherwise arrive here mid-destruction.
  contents_->SetDelegate(nullptr);
  Observe(nullptr);
  contents_.reset();
}

// A page coming back from history already has its icon and may not announce it again,
// nor its title.
void PageTab::DidFinishNavigation(content::NavigationHandle* handle) {
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsSameDocument()) {
    timeline::Mark("tab.committed", "tab=" + base::NumberToString(id_));
  }
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsErrorPage() && !contents_->GetFaviconURLs().empty()) {
    FetchFavicon(contents_->GetFaviconURLs());
  }
  if (handle->IsInPrimaryMainFrame()) {
    ReportNavigation(handle);
  }
  WebContentsObserverHooks::DidFinishNavigation(handle);
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
    args.Str(handle->GetURL().spec());
    hooks::Fire(PB_EVENT_NAVIGATION_FINISHED, args);
    if (!Title().empty()) {
      ReportTitle();
    }
    return;
  }
  const net::Error error = handle->GetNetErrorCode();
  if (error != net::OK && error != net::ERR_ABORTED) {
    hooks::Args args;
    args.Int(id_);
    args.Int(handle->GetNavigationId());
    args.Str(handle->GetURL().spec());
    args.Str(net::ErrorToString(error));
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

content::WebContents* PageTab::OpenURLFromTab(
    content::WebContents* source,
    const content::OpenURLParams& params,
    base::OnceCallback<void(content::NavigationHandle&)> callback) {
  content::WebContents* target = source;
  if (params.disposition != WindowOpenDisposition::CURRENT_TAB) {
    // A link asked for another tab, which is in this tab's workspace. Go hears about
    // the tab before its first navigation, so the gate finds it.
    PageTab* tab = Engine::Get()->CreateTab(workspace_);
    Engine::TabOpened(id_, tab->id(), params.url,
                      params.disposition != WindowOpenDisposition::NEW_BACKGROUND_TAB,
                      /*new_window=*/false);
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
content::WebContents* PageTab::AddNewContents(
    content::WebContents* source,
    std::unique_ptr<content::WebContents> new_contents,
    const GURL& target_url,
    WindowOpenDisposition disposition,
    const blink::mojom::WindowFeatures& window_features,
    bool user_gesture,
    bool* was_blocked) {
  Engine* engine = Engine::Get();
  PageTab* tab = nullptr;
  if (engine->WorkspaceOf(new_contents.get()) == workspace_) {
    tab = engine->AdoptTab(std::move(new_contents));
  } else {
    new_contents.reset();
    tab = engine->CreateTab(workspace_);
  }
  Engine::TabOpened(id_, tab->id(), target_url,
                    disposition != WindowOpenDisposition::NEW_BACKGROUND_TAB,
                    /*new_window=*/false);
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

#if BUILDFLAG(IS_MAC)
// A page's dialogs are Go's, drawn by the shell. A phone keeps content's way until the
// Mobile shell draws them.
content::JavaScriptDialogManager* PageTab::GetJavaScriptDialogManager(
    content::WebContents* source) {
  return Dialogs::Get();
}

// The phone's own picker already opens; the Mac's goes through Go first.
void PageTab::RunFileChooser(content::RenderFrameHost* render_frame_host,
                             scoped_refptr<content::FileSelectListener> listener,
                             const blink::mojom::FileChooserParams& params) {
  FileChooser::Run(id_, render_frame_host, std::move(listener), params);
}
#endif

void PageTab::EnterFullscreenModeForTab(content::RenderFrameHost* requesting_frame,
                                        const blink::mojom::FullscreenOptions& options) {
  Fire(PB_EVENT_PAGE_FULL_SCREEN, true);
  WebContentsDelegateHooks::EnterFullscreenModeForTab(requesting_frame, options);
  SynchronizeVisualProperties();
}

void PageTab::ExitFullscreenModeForTab(content::WebContents* web_contents) {
  Fire(PB_EVENT_PAGE_FULL_SCREEN, false);
  WebContentsDelegateHooks::ExitFullscreenModeForTab(web_contents);
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

void PageTab::FindReply(content::WebContents* web_contents,
                        int request_id,
                        int number_of_matches,
                        const gfx::Rect& selection_rect,
                        int active_match_ordinal,
                        bool final_update) {
  hooks::Args args;
  args.Int(id_);
  args.Int(request_id);
  args.Int(number_of_matches);
  args.Int(active_match_ordinal);
  args.Bool(final_update);
  hooks::Fire(PB_EVENT_FIND_REPLY, args);
  WebContentsDelegateHooks::FindReply(web_contents, request_id, number_of_matches,
                                      selection_rect, active_match_ordinal,
                                      final_update);
}

// A right click, or a long press, in the page: Go says whether the shell draws the menu.
bool PageTab::HandleContextMenu(content::RenderFrameHost& render_frame_host,
                                const content::ContextMenuParams& params) {
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

// Content asks the tab's delegate to close or show it; Go decides.
void PageTab::CloseContents(content::WebContents* source) {
  Fire(PB_EVENT_CLOSE_REQUESTED);
  WebContentsDelegateHooks::CloseContents(source);
}

void PageTab::ActivateContents(content::WebContents* contents) {
  Fire(PB_EVENT_ACTIVATE_REQUESTED);
  WebContentsDelegateHooks::ActivateContents(contents);
}

// Content asks Go whether the page is in full screen as it sizes the page, and tells the
// page what it found.
void PageTab::SynchronizeVisualProperties() {
  if (content::RenderWidgetHostView* view = contents_->GetRenderWidgetHostView()) {
    view->GetRenderWidgetHost()->SynchronizeVisualProperties();
  }
}

bool PageTab::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusShellForTraversal(id_, reverse);
  return true;
}

content::WebContents* PageTab::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b

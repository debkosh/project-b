#ifndef PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_
#define PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_

#include <cstdint>
#include <memory>
#include <optional>
#include <string>
#include <vector>

#include "base/memory/ref_counted_memory.h"
#include "base/memory/weak_ptr.h"
#include "base/time/time.h"
#include "build/build_config.h"
#include "content/public/browser/render_widget_host_view.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "third_party/skia/include/core/SkBitmap.h"
#include "ui/gfx/geometry/size.h"

namespace project_b {

// One tab's connection to content. What is overridden here tells Go what its page does,
// as the events in pb_abi.h, or needs content's objects first. Each still calls its
// generated base after, which carries the raw hook to Go when something subscribed.
class PageTab : public WebContentsObserverHooks, public WebContentsDelegateHooks {
 public:
  PageTab(int64_t id,
          std::unique_ptr<content::WebContents> contents,
          const std::string& workspace,
          bool internal);
  PageTab(const PageTab&) = delete;
  PageTab& operator=(const PageTab&) = delete;
  ~PageTab() override;

  int64_t id() const { return id_; }
  content::WebContents* contents() const { return contents_.get(); }
  // Go's id for the workspace the tab is in, "" for the profile's own storage. It never
  // changes.
  const std::string& workspace() const { return workspace_; }
  // Whether it is one of ours, a tab for our own pages, made for them (CreateInternalTab),
  // not a web tab. It never changes. On Android our pages and the web share one context, so
  // the context can't tell.
  bool internal() const { return internal_; }
  // Go's id for the window whose layout last showed this tab, 0 before any has.
  int64_t window() const { return window_; }
  void set_window(int64_t window) { window_ = window; }
  // Delivers one message from Go to our page in a tab of ours, as the shell gets its own.
  void SendToPage(const std::string& json);
  // Copies what the page shows now, at most max_width pixels across, and keeps it as a
  // JPEG in thumbnail(), telling Go with PB_EVENT_THUMBNAIL. A newer copy asked for while
  // one is on its way replaces it.
  void CaptureThumbnail(int max_width);
  // The latest picture of the page, or null before the first. It goes with the tab.
  scoped_refptr<base::RefCountedMemory> thumbnail() const { return thumbnail_; }

 private:
  // content::WebContentsObserver:
  void DidFinishNavigation(content::NavigationHandle* handle) override;
  void RenderFrameCreated(content::RenderFrameHost* frame) override;
  void DidFirstVisuallyNonEmptyPaint() override;
  void DidStartLoading() override;
  void DidStopLoading() override;
  void LoadProgressChanged(double progress) override;
  void DidChangeVerticalScrollDirection(
      viz::VerticalScrollDirection scroll_direction) override;
  void DidChangeVisibleSecurityState() override;
  void TitleWasSet(content::NavigationEntry* entry) override;
  void OnWebContentsFocused(
      content::RenderWidgetHost* render_widget_host) override;
  void PrimaryMainFrameRenderProcessGone(
      base::TerminationStatus status) override;
  void DidUpdateFaviconURL(
      content::RenderFrameHost* render_frame_host,
      const std::vector<blink::mojom::FaviconURLPtr>& candidates,
      blink::mojom::FaviconUpdateReason reason) override;

  // Fetches the best of the page's icon candidates and hands it to Go as a PNG data
  // URL, or "" when there is none to show.
  void FetchFavicon(const std::vector<blink::mojom::FaviconURLPtr>& candidates);
  void OnFaviconDownloaded(int request,
                           int id,
                           int http_status_code,
                           const GURL& image_url,
                           const std::vector<SkBitmap>& bitmaps,
                           const std::vector<gfx::Size>& sizes);

  void OnThumbnailCopied(int request,
                         base::TimeTicks asked,
                         const content::CopyFromSurfaceResult& result);
  void OnThumbnailEncoded(int request,
                          base::TimeDelta copy,
                          base::TimeTicks copied,
                          gfx::Size size,
                          std::optional<std::vector<uint8_t>> jpeg);

  // content::WebContentsDelegate: these hand over content's objects, so they stay here.
  content::WebContents* hook_contents() const override;
  content::WebContents* OpenURLFromTab(
      content::WebContents* source,
      const content::OpenURLParams& params,
      base::OnceCallback<void(content::NavigationHandle&)> callback) override;
  content::WebContents* AddNewContents(
      content::WebContents* source,
      std::unique_ptr<content::WebContents> new_contents,
      const GURL& target_url,
      WindowOpenDisposition disposition,
      const blink::mojom::WindowFeatures& window_features,
      bool user_gesture,
      bool* was_blocked) override;
  content::KeyboardEventProcessingResult PreHandleKeyboardEvent(
      content::WebContents* source,
      const input::NativeWebKeyboardEvent& event) override;
  bool HandleKeyboardEvent(content::WebContents* source,
                           const input::NativeWebKeyboardEvent& event) override;
  bool TakeFocus(content::WebContents* source, bool reverse) override;
  bool HandleContextMenu(content::RenderFrameHost& render_frame_host,
                         const content::ContextMenuParams& params) override;
  void CloseContents(content::WebContents* source) override;
  void ActivateContents(content::WebContents* contents) override;
  bool IsFullscreenForTabOrPending(
      const content::WebContents* web_contents) override;
  void FindReply(content::WebContents* web_contents,
                 int request_id,
                 int number_of_matches,
                 const gfx::Rect& selection_rect,
                 int active_match_ordinal,
                 bool final_update) override;

  // Tells Go one thing about this tab: the event with the tab's id, and a yes or no.
  void Fire(PBEvent event);
  void Fire(PBEvent event, bool value);
  void ReportNavigation(content::NavigationHandle* handle);
  void ReportTitle();
  std::string Title() const;
  void SynchronizeVisualProperties();
  void SystemBarsHidden(bool hidden);
  // Go hears of a page's full screen first, then the page is told its new size.
  void EnterFullscreenModeForTab(content::RenderFrameHost* requesting_frame,
                                 const blink::mojom::FullscreenOptions& options) override;
  void ExitFullscreenModeForTab(content::WebContents* web_contents) override;
  content::JavaScriptDialogManager* GetJavaScriptDialogManager(
      content::WebContents* source) override;
  void RunFileChooser(content::RenderFrameHost* render_frame_host,
                      scoped_refptr<content::FileSelectListener> listener,
                      const blink::mojom::FileChooserParams& params) override;
#if BUILDFLAG(IS_ANDROID)
  // Android has no colour picker inside the page, so a colour input asks for one.
  std::unique_ptr<content::ColorChooser> OpenColorChooser(
      content::WebContents* web_contents,
      SkColor color,
      const std::vector<blink::mojom::ColorSuggestionPtr>& suggestions)
      override;
#endif
  // getUserMedia, and whether a page may see its devices' names: Go decides.
  bool CheckMediaAccessPermission(content::RenderFrameHost* render_frame_host,
                                  const url::Origin& security_origin,
                                  blink::mojom::MediaStreamType type) override;
  void RequestMediaAccessPermission(content::WebContents* web_contents,
                                    const content::MediaStreamRequest& request,
                                    content::MediaResponseCallback callback) override;

  // Tells Go how the page shown reached it, when that changed.
  void ReportConnection();

  const int64_t id_;
  // What Go was last told of the connection: a PBConnection, and its reason.
  int32_t connection_ = 0;
  std::string connection_reason_;
  const std::string workspace_;
  const bool internal_;
  int64_t window_ = 0;
  std::unique_ptr<content::WebContents> contents_;
  // Counts favicon fetches, so a slow one for an earlier page is dropped.
  int favicon_request_ = 0;
  // Counts the copies asked for, so only the newest is kept; revision counts those kept.
  int thumbnail_request_ = 0;
  int64_t thumbnail_revision_ = 0;
  scoped_refptr<base::RefCountedMemory> thumbnail_;
  base::WeakPtrFactory<PageTab> weak_factory_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_

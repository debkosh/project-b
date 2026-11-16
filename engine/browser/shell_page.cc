#include "project_b/engine/browser/shell_page.h"

#include <algorithm>
#include <string_view>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/containers/to_vector.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/memory/ref_counted_memory.h"
#include "base/memory/weak_ptr.h"
#include "base/no_destructor.h"
#include "base/path_service.h"
#include "base/strings/strcat.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/string_util.h"
#include "base/task/thread_pool.h"
#include "base/values.h"
#include "build/build_config.h"
#include "content/public/browser/keyboard_event_processing_result.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/url_data_source.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_ui.h"
#include "content/public/browser/web_ui_controller.h"
#include "content/public/browser/web_ui_controller_factory.h"
#include "content/public/browser/web_ui_message_handler.h"
#include "content/public/common/bindings_policy.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/events.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "services/network/public/mojom/content_security_policy.mojom.h"
#include "third_party/skia/include/core/SkColor.h"
#include "url/gurl.h"

#if BUILDFLAG(IS_ANDROID)
#include "base/android/apk_assets.h"
#include "base/files/file.h"
#include "base/files/memory_mapped_file.h"
#include "base/strings/string_view_util.h"
#endif

namespace project_b {

namespace {

// Our pages, each an app of its own (shell/src/apps/<app>) at chrome://<app>/, served from
// shell/<app>/ beside the binary. Every shell is an app of "shell": shell <id> is
// chrome://shell/<id>/, served from shell/shell/<id>/. A window's page, a shell or the
// picker, belongs in the WebContents that covers its window, and its messages cross to Go
// as one event with the window. A page a tab shows, settings, belongs in a tab of ours, and
// its messages cross with the tab. Settings, History and Bookmarks are such pages.
struct App {
  const char* host;
  PBEvent event;
  bool in_tab;
};
constexpr App kApps[] = {
    {"shell", PB_EVENT_SHELL_MESSAGE, false},
    {"settings", PB_EVENT_PAGE_MESSAGE, true},
    {"history", PB_EVENT_PAGE_MESSAGE, true},
    {"bookmarks", PB_EVENT_PAGE_MESSAGE, true},
    {"downloads", PB_EVENT_PAGE_MESSAGE, true},
    {"picker", PB_EVENT_PICKER_MESSAGE, false},
};

const App* AppFor(const GURL& url) {
  if (!url.SchemeIs("chrome")) {
    return nullptr;
  }
  for (const App& app : kApps) {
    if (url.host() == app.host) {
      return &app;
    }
  }
  return nullptr;
}

// Whom this WebContents is app's page for: Go's id for the tab of ours it is, for a page a
// tab shows; for a window's page, the window it covers when that page is app's. 0 when it
// is neither.
int64_t Home(const App& app, content::WebContents* contents) {
  if (app.in_tab) {
    const int64_t tab = hooks::TabOf(contents);
    PageTab* t = tab > 0 ? Engine::Get()->tab(tab) : nullptr;
    return t && t->internal() ? tab : 0;
  }
  for (const auto& [id, w] : Engine::Get()->windows()) {
    if (w->shell && w->shell->app() == app.host &&
        w->shell->contents() == contents) {
      return id;
    }
  }
  return 0;
}

// Whether a page's home is still there: its tab, or its window.
bool HomeIsThere(const App& app, int64_t home) {
  return app.in_tab ? Engine::Get()->tab(home) != nullptr
                    : Engine::Get()->window(home) != nullptr;
}

// Forwards chrome.send(name, [json]) to Go as one event, with the page's home: 'shell' from
// a shell, with its window; 'settings' from the settings page, with its tab.
class ShellMessageHandler : public content::WebUIMessageHandler {
 public:
  ShellMessageHandler(const App& app, int64_t home) : app_(app), home_(home) {}

  void RegisterMessages() override {
    web_ui()->RegisterMessageCallback(
        app_.host, base::BindRepeating(&ShellMessageHandler::OnMessage,
                                       base::Unretained(this)));
  }

 private:
  void OnMessage(const base::ListValue& args) {
    if (args.size() != 1 || !args[0].is_string()) {
      LOG(WARNING) << "dropped a malformed message from " << app_.host;
      return;
    }
    // One sent just before its window or its tab closed arrives with it gone.
    if (!HomeIsThere(app_, home_)) {
      return;
    }
    hooks::Args hook_args;
    hook_args.Int(home_);
    // A page in a tab shares one event with the others, so it says which it is.
    if (app_.in_tab) {
      hook_args.Str(app_.host);
    }
    hook_args.Str(args[0].GetString());
    hooks::Fire(app_.event, hook_args);
  }

  const App app_;
  const int64_t home_;
};

// The bridge to Go belongs to each app's own WebContents: a window's page, or a tab of
// ours. The factory sees only the URL, so content makes a controller for chrome://shell/
// or chrome://settings/ wherever it loads; anywhere else the controller takes away the
// page's bindings, so it has no chrome.send, and drops whatever arrives anyway.
class AppController : public content::WebUIController {
 public:
  AppController(content::WebUI* web_ui, const App& app)
      : content::WebUIController(web_ui),
        app_(app),
        home_(Home(app, web_ui->GetWebContents())) {
    if (!home_) {
      LOG(WARNING) << "chrome://" << app_.host << "/ outside its page: no bindings";
      web_ui->SetBindings(content::BindingsPolicySet());
      return;
    }
    web_ui->AddMessageHandler(std::make_unique<ShellMessageHandler>(app_, home_));
  }

  bool OverrideHandleWebUIMessage(const GURL& source_url,
                                  const std::string& message,
                                  const base::ListValue& args) override {
    if (home_) {
      return false;
    }
    LOG(WARNING) << "dropped \"" << message << "\" from chrome://" << app_.host
                 << "/ outside its page";
    return true;
  }

  WEB_UI_CONTROLLER_TYPE_DECL();

 private:
  const App app_;
  // The window or the tab this is the page of, or 0 when it is outside its page.
  const int64_t home_;
};

WEB_UI_CONTROLLER_TYPE_IMPL(AppController)

// Our pages are WebUI only in our pages' context. Anywhere else, as in a tab, their
// address is no page at all. The factory lives as long as the process, past the context.
class AppControllerFactory : public content::WebUIControllerFactory {
 public:
  explicit AppControllerFactory(content::BrowserContext* pages_context)
      : pages_context_(pages_context->GetWeakPtr()) {}

  std::unique_ptr<content::WebUIController> CreateWebUIControllerForURL(
      content::WebUI* web_ui,
      const GURL& url) override {
    const App* app = AppFor(url);
    return app ? std::make_unique<AppController>(web_ui, *app) : nullptr;
  }

  content::WebUI::TypeID GetWebUIType(content::BrowserContext* browser_context,
                                      const GURL& url) override {
    const App* app = AppIn(browser_context, url);
    return app ? reinterpret_cast<content::WebUI::TypeID>(app)
               : content::WebUI::kNoWebUI;
  }

  bool UseWebUIForURL(content::BrowserContext* browser_context,
                      const GURL& url) override {
    return AppIn(browser_context, url) != nullptr;
  }

 private:
  const App* AppIn(content::BrowserContext* browser_context,
                   const GURL& url) const {
    return browser_context == pages_context_.get() ? AppFor(url) : nullptr;
  }

  const base::WeakPtr<content::BrowserContext> pages_context_;
};

// Serves one app's built files from disk. No remote content ever reaches its page.
class AppDataSource : public content::URLDataSource {
 public:
  explicit AppDataSource(const char* host) : host_(host) {}

  std::string GetSource() override { return host_; }

  void StartDataRequest(const GURL& url,
                        const content::WebContents::Getter& wc_getter,
                        GotDataCallback callback) override {
    std::string name = Name(url);
    if (name.empty()) {
      std::move(callback).Run(nullptr);
      return;
    }
#if BUILDFLAG(IS_ANDROID)
    // On Android the apps are in the APK, not in a folder.
    base::MemoryMappedFile::Region region;
    base::File asset(
        base::android::OpenApkAsset("assets/shell/" + name, &region));
    base::MemoryMappedFile mapped;
    // A file the APK doesn't hold is a page's 404, as a missing one is on the Mac.
    if (!asset.IsValid() || !mapped.Initialize(std::move(asset), region)) {
      std::move(callback).Run(nullptr);
      return;
    }
    std::string data(base::as_string_view(mapped.bytes()));
    std::move(callback).Run(
        base::MakeRefCounted<base::RefCountedString>(std::move(data)));
    return;
#else
    base::FilePath dir;
    CHECK(base::PathService::Get(base::DIR_ASSETS, &dir));
    base::FilePath file = dir.Append("shell").AppendASCII(name);
    base::ThreadPool::PostTaskAndReplyWithResult(
        FROM_HERE, {base::MayBlock(), base::TaskPriority::USER_BLOCKING},
        base::BindOnce(
            [](const base::FilePath& file) -> scoped_refptr<base::RefCountedMemory> {
              std::string data;
              if (!base::ReadFileToString(file, &data)) {
                return nullptr;
              }
              return base::MakeRefCounted<base::RefCountedString>(std::move(data));
            },
            file),
        std::move(callback));
#endif
  }

  std::string GetMimeType(const GURL& url) override {
    std::string name = Name(url);
    if (base::EndsWith(name, ".js")) {
      return "text/javascript";
    }
    if (base::EndsWith(name, ".css")) {
      return "text/css";
    }
    if (base::EndsWith(name, ".woff2")) {
      return "font/woff2";
    }
    // A page's icon, which the engine fetches for its tab as it does a web page's.
    if (base::EndsWith(name, ".svg")) {
      return "image/svg+xml";
    }
    if (base::EndsWith(name, ".png")) {
      return "image/png";
    }
    if (base::EndsWith(name, ".ico")) {
      return "image/x-icon";
    }
    return "text/html";
  }

  bool ShouldServeMimeTypeAsContentTypeHeader() override { return true; }

  std::string GetContentSecurityPolicy(
      network::mojom::CSPDirectiveName directive) override {
    using Name = network::mojom::CSPDirectiveName;
    switch (directive) {
      case Name::DefaultSrc:
        return "default-src 'none';";
      case Name::ScriptSrc:
        return "script-src 'self';";
      // The inline <style> in index.html paints before any JS has run.
      case Name::StyleSrc:
        return "style-src 'self' 'unsafe-inline';";
      // A shell draws the tabs' pictures, from chrome://thumb, and no other page of ours.
      case Name::ImgSrc:
        return host_ == "shell" ? "img-src 'self' data: chrome://thumb;"
                                : "img-src 'self' data:;";
      case Name::FontSrc:
        return "font-src 'self';";
      case Name::RequireTrustedTypesFor:
        return "require-trusted-types-for 'script';";
      // The policies the frameworks our pages are built with make for their templates.
      // The mock engine serves our pages with the same policy (core/cmd/dev, pageCSP).
      case Name::TrustedTypes:
        return "trusted-types vue svelte-trusted-html;";
      default:
        return content::URLDataSource::GetContentSecurityPolicy(directive);
    }
  }

 private:
  // The file a URL names, as a path in the folder of our pages, or "" for none. Every app
  // has any file its build put in its folder, such as its script, its stylesheet, its fonts
  // or its icon, by a plain name: lower-case letters, digits, dashes and dots, not starting
  // with a dot. Settings and the picker have their own folders, and any other path of
  // theirs, such as a section of settings, is their index.html. A shell's folder is its id,
  // the path's first part. So no path reaches outside its folder.
  std::string Name(const GURL& url) const {
    std::string_view path = url.path().substr(1);
    if (host_ != "shell") {
      return host_ + "/" +
             (IsFile(path) ? std::string(path) : std::string("index.html"));
    }
    size_t slash = path.find('/');
    if (slash == std::string_view::npos) {
      return "";
    }
    std::string_view id = path.substr(0, slash);
    std::string_view file = path.substr(slash + 1);
    if (file.empty()) {
      file = "index.html";
    }
    if (id.empty() || !std::ranges::all_of(id, [](char c) {
          return base::IsAsciiLower(c) || base::IsAsciiDigit(c) || c == '-';
        }) ||
        !IsPlainName(file)) {
      return "";
    }
    return base::StrCat({"shell/", id, "/", file});
  }

  // Whether name is a plain file name: lower-case letters, digits, dashes and dots, not
  // starting with a dot.
  static bool IsPlainName(std::string_view name) {
    return !name.empty() && name.front() != '.' &&
           std::ranges::all_of(name, [](char c) {
             return base::IsAsciiLower(c) || base::IsAsciiDigit(c) || c == '-' ||
                    c == '.';
           });
  }

  // Whether a path of settings or the picker names a file: a plain name with a dot in it,
  // where a section's path has none.
  static bool IsFile(std::string_view path) {
    return IsPlainName(path) && path.find('.') != std::string_view::npos;
  }

  const std::string host_;
};

// Serves the latest picture of a tab's page, chrome://thumb/<tab>, that
// PageTab::CaptureThumbnail kept; a query, the revision Go adds so the address changes with
// the picture, is ignored. None yet, or no such tab, is a 404. It is in our pages' context
// only, and only a page at a chrome:// address has a loader for it there, so a web page
// can't load it, even on Android, where the web shares that context.
class ThumbDataSource : public content::URLDataSource {
 public:
  std::string GetSource() override { return "thumb"; }

  void StartDataRequest(const GURL& url,
                        const content::WebContents::Getter& wc_getter,
                        GotDataCallback callback) override {
    std::string_view path = url.path();
    if (path.starts_with('/')) {
      path.remove_prefix(1);
    }
    int64_t id = 0;
    PageTab* tab =
        base::StringToInt64(path, &id) ? Engine::Get()->tab(id) : nullptr;
    std::move(callback).Run(tab ? tab->thumbnail() : nullptr);
  }

  std::string GetMimeType(const GURL& url) override { return "image/jpeg"; }

  bool ShouldServeMimeTypeAsContentTypeHeader() override { return true; }
};

}  // namespace

void RegisterShellWebUI(content::BrowserContext* pages_context) {
  static base::NoDestructor<AppControllerFactory> factory(pages_context);
  content::WebUIControllerFactory::RegisterFactory(factory.get());
  for (const App& app : kApps) {
    content::URLDataSource::Add(pages_context,
                                std::make_unique<AppDataSource>(app.host));
  }
  content::URLDataSource::Add(pages_context,
                              std::make_unique<ThumbDataSource>());
}

ShellPage::ShellPage(content::BrowserContext* browser_context,
                     int64_t window,
                     const std::string& page)
    : window_(window),
      page_(page),
      contents_(content::WebContents::Create(
          content::WebContents::CreateParams(browser_context))) {
  Observe(contents_.get());
  contents_->SetDelegate(this);
  contents_->SetPageBaseBackgroundColor(SK_ColorTRANSPARENT);
}

ShellPage::~ShellPage() {
  contents_->SetDelegate(nullptr);
}

std::string_view ShellPage::app() const {
  return std::string_view(page_).substr(0, page_.find('/'));
}

void ShellPage::Load() {
  contents_->GetController().LoadURLWithParams(
      content::NavigationController::LoadURLParams(
          GURL("chrome://" + page_ + "/")));
  Watch();
}

void ShellPage::Watch() {
#if BUILDFLAG(IS_ANDROID)
  // Longer than Android gives a sandboxed process to attach, so the system has killed one
  // that didn't by then; a renderer that attached and is only slow is left to finish.
  constexpr base::TimeDelta kAttachWait = base::Seconds(15);
  watch_.Start(FROM_HERE, kAttachWait,
               base::BindOnce(&ShellPage::Unattached, base::Unretained(this)));
#endif
}

#if BUILDFLAG(IS_ANDROID)
// Content can't stop a launch it is still waiting on, so neither a shutdown nor a reload
// gets this page a renderer: the engine makes the page again, with a process of its own.
// Go hears of it as of a crash, and waits for the new shell to say it's ready.
void ShellPage::Unattached() {
  if (contents_->GetPrimaryMainFrame()->GetProcess()->IsReady()) {
    Watch();
    return;
  }
  timeline::Mark("shell.unattached");
  hooks::Args args;
  args.Int(window_);
  hooks::Fire(PB_EVENT_SHELL_CRASHED, args);
  // Making the page again ends this one, so it waits for the timer's task to return.
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(&Engine::RenewShell,
                                base::Unretained(Engine::Get()), window_));
}
#endif

void ShellPage::SetPage(const std::string& page) {
  page_ = page;
  Load();
}

void ShellPage::Send(const std::string& json) {
  if (content::WebUI* web_ui = contents_->GetWebUI()) {
    std::vector<base::ValueView> args = {json};
    web_ui->CallJavascriptFunctionUnsafe("__shellReceive", args);
  }
}

// The shell's view is transparent wherever the shell paints nothing, so the page under
// it shows. Content treats a view it was never told about as opaque, and the page area
// then stays white until the shell next paints. The view belongs to the main frame, so
// it is set as each main frame comes alive: the first one, one after a renderer crash,
// and any later swap. A frame change is not enough: the first frame is made inside
// WebContents::Create, before this observes, and the shell loads into that same frame.
void ShellPage::RenderFrameCreated(content::RenderFrameHost* frame) {
  if (frame->GetParent()) {
    return;
  }
  if (content::RenderWidgetHostView* view = frame->GetView()) {
    view->SetBackgroundColor(SK_ColorTRANSPARENT);
  }
}

// A new document, the first, another shell or the same one again, shows the window's pages
// only once it has drawn a layout that places them.
void ShellPage::DidFinishNavigation(content::NavigationHandle* handle) {
  timeline::PageCommitted(app(), handle);
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted()) {
    watch_.Stop();
  }
#if !BUILDFLAG(IS_ANDROID)
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsSameDocument()) {
    Engine::Get()->ShellGone(window_);
  }
#endif
}

// Go's window manager takes back what a dead shell claimed, so the page keeps working
// until the new shell reports its layout again. The window holds it unseen meanwhile, as
// nothing of the shell is drawn around it.
void ShellPage::PrimaryMainFrameRenderProcessGone(
    base::TerminationStatus status) {
#if !BUILDFLAG(IS_ANDROID)
  Engine::Get()->ShellGone(window_);
#endif
  hooks::Args args;
  args.Int(window_);
  hooks::Fire(PB_EVENT_SHELL_CRASHED, args);
  // The reload waits for a later task. Content is still telling its observers the
  // process is gone, and a renderer started before they have all heard is one that
  // those who track renderers lose.
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(
                     [](base::WeakPtr<ShellPage> self) {
                       if (self) {
                         self->contents_->GetController().Reload(
                             content::ReloadType::NORMAL, false);
                         self->Watch();
                       }
                     },
                     weak_factory_.GetWeakPtr()));
}

// Go gets the key first; one it doesn't use goes to the menu bar.
content::KeyboardEventProcessingResult ShellPage::PreHandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  if (events::Key(source, event, PB_KEY_BEFORE_PAGE)) {
    return content::KeyboardEventProcessingResult::HANDLED;
  }
  return WebContentsDelegateHooks::PreHandleKeyboardEvent(source, event);
}

bool ShellPage::HandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  return events::Key(source, event, PB_KEY_UNHANDLED) ||
         WebContentsDelegateHooks::HandleKeyboardEvent(source, event) ||
         Window::HandleUnhandledKey(event);
}

bool ShellPage::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusPageForTraversal(window_, reverse);
  return true;
}

content::WebContents* ShellPage::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b

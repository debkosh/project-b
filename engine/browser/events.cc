#include "project_b/engine/browser/events.h"

#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "net/base/registry_controlled_domains/registry_controlled_domain.h"
#include "ui/base/page_transition_types.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/common/block_rules.h"
#include "third_party/blink/public/common/input/web_input_event.h"

namespace project_b {
namespace events {

namespace {

// The shell's id is 0, and a tab's is positive.
PBSurface SurfaceOf(content::WebContents* source) {
  return hooks::TabOf(source) == 0 ? PB_SURFACE_SHELL : PB_SURFACE_PAGE;
}

int64_t ModsOf(const input::NativeWebKeyboardEvent& event) {
  const int held = event.GetModifiers();
  int64_t mods = 0;
  if (held & blink::WebInputEvent::kControlKey) {
    mods |= PB_MOD_CTRL;
  }
  if (held & blink::WebInputEvent::kAltKey) {
    mods |= PB_MOD_ALT;
  }
  if (held & blink::WebInputEvent::kShiftKey) {
    mods |= PB_MOD_SHIFT;
  }
  if (held & blink::WebInputEvent::kMetaKey) {
    mods |= PB_MOD_CMD;
  }
  return mods;
}

}  // namespace

bool Key(content::WebContents* source,
         const input::NativeWebKeyboardEvent& event,
         PBKeyPhase phase) {
  // Key up and typed characters are no key press.
  if (event.GetType() != blink::WebInputEvent::Type::kRawKeyDown &&
      event.GetType() != blink::WebInputEvent::Type::kKeyDown) {
    return false;
  }
  hooks::Args args;
  args.Int(event.windows_key_code);
  args.Int(ModsOf(event));
  args.Int(phase);
  args.Int(SurfaceOf(source));
  PBValue result;
  return hooks::Ask(PB_EVENT_KEY, args, &result) && result.i != 0;
}

namespace {

// Asks the app about a page of tab's, at from, going to another site at to; opener is the
// tab whose page asks for a new tab, or 0. A PBNavigation.
int64_t Ask(int64_t tab,
            const GURL& from,
            const GURL& to,
            bool pressed,
            int64_t opener) {
  hooks::Args args;
  args.Int(tab);
  args.Str(from.spec());
  args.Str(to.spec());
  args.Int(pressed ? 1 : 0);
  args.Int(opener);
  PBValue result;
  if (!hooks::Ask(PB_EVENT_LEAVING_SITE, args, &result)) {
    return PB_NAVIGATION_PROCEED;
  }
  return result.i;
}

bool SameSite(const GURL& a, const GURL& b) {
  return net::registry_controlled_domains::SameDomainOrHost(
      a, b, net::registry_controlled_domains::INCLUDE_PRIVATE_REGISTRIES);
}

int64_t AskLeavingSite(int64_t tab,
                       const GURL& from,
                       const GURL& to,
                       bool pressed,
                       int64_t opener) {
  if (!from.SchemeIsHTTPOrHTTPS() || !to.SchemeIsHTTPOrHTTPS() ||
      SameSite(from, to)) {
    return PB_NAVIGATION_PROCEED;
  }
  return Ask(tab, from, to, pressed, opener);
}

}  // namespace

bool MayOpenWindow(content::RenderFrameHost* opener,
                   const GURL& target,
                   bool pressed) {
  content::WebContents* contents =
      content::WebContents::FromRenderFrameHost(opener);
  const int64_t tab = hooks::TabOf(contents);
  const GURL& from = contents ? contents->GetLastCommittedURL() : GURL();
  // window.open() with no address opens the blank page, which is no site of the page's.
  const GURL to = target.is_empty() ? GURL(url::kAboutBlankURL) : target;
  if (tab <= 0 || !from.SchemeIsHTTPOrHTTPS() ||
      !(to.IsAboutBlank() || (to.SchemeIsHTTPOrHTTPS() && !SameSite(from, to)))) {
    return true;
  }
  return Ask(tab, from, to, pressed, tab) == PB_NAVIGATION_PROCEED;
}

bool MayGo(content::WebContents* source,
           const GURL& url,
           bool pressed,
           bool new_tab) {
  const int64_t tab = hooks::TabOf(source);
  return tab <= 0 ||
         AskLeavingSite(tab, source->GetLastCommittedURL(), url, pressed,
                        new_tab ? tab : 0) == PB_NAVIGATION_PROCEED;
}

Gate::Gate(content::NavigationThrottleRegistry& registry)
    : content::NavigationThrottle(registry) {}

content::NavigationThrottle::ThrottleCheckResult Gate::WillStartRequest() {
  return Ask();
}

content::NavigationThrottle::ThrottleCheckResult Gate::WillRedirectRequest() {
  return Ask();
}

const char* Gate::GetNameForLogging() {
  return "project_b::events::Gate";
}

content::NavigationThrottle::ThrottleCheckResult Gate::Ask() {
  content::NavigationHandle* handle = navigation_handle();
  const int64_t tab = hooks::TabOf(handle->GetWebContents());
  if (tab <= 0) {
    return PROCEED;
  }
  if (!handle->IsInPrimaryMainFrame()) {
    return block_rules::Blocks(handle->GetURL()) ? BLOCK_REQUEST : PROCEED;
  }
  // A page leaving its site is asked about first, so one that stays never starts in Go's
  // tab, which keeps its title and icon.
  PBValue result;
  result.i = AskLeaving(tab);
  if (result.i == PB_NAVIGATION_PROCEED) {
    hooks::Args args;
    args.Int(tab);
    args.Int(handle->GetNavigationId());
    args.Str(handle->GetURL().spec());
    if (!hooks::Ask(PB_EVENT_NAVIGATION_STARTING, args, &result)) {
      return PROCEED;
    }
  }
  switch (result.i) {
    case PB_NAVIGATION_PROCEED:
      return PROCEED;
    case PB_NAVIGATION_ELSEWHERE:
      return CANCEL_AND_IGNORE;
  }
  return BLOCK_REQUEST;
}

int64_t Gate::AskLeaving(int64_t tab) {
  content::NavigationHandle* handle = navigation_handle();
  const ui::PageTransition transition = handle->GetPageTransition();
  // Only the page's own way out: what the person asks for from the shell goes anywhere (the
  // app loads it typed, or as back and forward). A page's comes from it, names it as its
  // initiator, or is a link or a form: Android loads a page's new tab, from a link or
  // window.open, in the page's own tab, as the browser's own navigation.
  const bool by_page =
      handle->IsRendererInitiated() || handle->GetInitiatorOrigin() ||
      ((ui::PageTransitionCoreTypeIs(transition, ui::PAGE_TRANSITION_LINK) ||
        ui::PageTransitionCoreTypeIs(transition, ui::PAGE_TRANSITION_FORM_SUBMIT)) &&
       !(transition & ui::PAGE_TRANSITION_FORWARD_BACK));
  if (!by_page) {
    return PB_NAVIGATION_PROCEED;
  }
  // A popup with no page of its own yet (window.open, target=_blank) leaves the site of
  // the tab that opened it.
  content::WebContents* contents = handle->GetWebContents();
  GURL from = contents->GetLastCommittedURL();
  int64_t opener = 0;
  if (!from.SchemeIsHTTPOrHTTPS()) {
    if (content::WebContents* by =
            contents->GetFirstWebContentsInLiveOriginalOpenerChain()) {
      from = by->GetLastCommittedURL();
      opener = hooks::TabOf(by);
    }
  }
  return AskLeavingSite(tab, from, handle->GetURL(), handle->HasUserGesture(),
                        opener);
}

}  // namespace events
}  // namespace project_b

#include "project_b/engine/browser/events.h"

#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/common/block_rules.h"
#include "third_party/blink/public/common/input/web_input_event.h"

namespace project_b {
namespace events {

namespace {

// The scheme of the app's own links, as core/app's OpenAppLink takes them.
constexpr char kAppScheme[] = "project-b";

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

Gate::Gate(content::NavigationThrottleRegistry& registry)
    : content::NavigationThrottle(registry) {}

content::NavigationThrottle::ThrottleCheckResult Gate::WillStartRequest() {
  return Ask(/*redirect=*/false);
}

content::NavigationThrottle::ThrottleCheckResult Gate::WillRedirectRequest() {
  return Ask(/*redirect=*/true);
}

const char* Gate::GetNameForLogging() {
  return "project_b::events::Gate";
}

content::NavigationThrottle::ThrottleCheckResult Gate::Ask(bool redirect) {
  content::NavigationHandle* handle = navigation_handle();
  const int64_t tab = hooks::TabOf(handle->GetWebContents());
  // The app's own links, which no page loads: Go hears only of one a press started in a
  // tab's own page, so a page can't open one by script, in a frame or by a redirect.
  if (handle->GetURL().SchemeIs(kAppScheme)) {
    if (tab > 0 && handle->IsInPrimaryMainFrame() && !redirect &&
        handle->HasUserGesture()) {
      hooks::Args args;
      args.Int(tab);
      args.Str(handle->GetURL().spec());
      hooks::Fire(PB_EVENT_APP_LINK, args);
    }
    return CANCEL_AND_IGNORE;
  }
  if (tab <= 0) {
    return PROCEED;
  }
  if (!handle->IsInPrimaryMainFrame()) {
    return block_rules::Blocks(handle->GetURL()) ? BLOCK_REQUEST : PROCEED;
  }
  hooks::Args args;
  args.Int(tab);
  args.Int(handle->GetNavigationId());
  args.Str(ShownURL(handle));
  PBValue result;
  if (!hooks::Ask(PB_EVENT_NAVIGATION_STARTING, args, &result)) {
    return PROCEED;
  }
  switch (result.i) {
    case PB_NAVIGATION_PROCEED:
      return PROCEED;
    case PB_NAVIGATION_ELSEWHERE:
      return CANCEL_AND_IGNORE;
  }
  return BLOCK_REQUEST;
}

}  // namespace events
}  // namespace project_b

#include "project_b/engine/browser/events.h"

#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "net/base/registry_controlled_domains/registry_controlled_domain.h"
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
  // Only the page's own way out: what the person asks for from the shell goes anywhere.
  if (!handle->IsRendererInitiated()) {
    return PB_NAVIGATION_PROCEED;
  }
  const GURL& from = handle->GetWebContents()->GetLastCommittedURL();
  const GURL& to = handle->GetURL();
  if (!from.SchemeIsHTTPOrHTTPS() || !to.SchemeIsHTTPOrHTTPS() ||
      net::registry_controlled_domains::SameDomainOrHost(
          from, to,
          net::registry_controlled_domains::INCLUDE_PRIVATE_REGISTRIES)) {
    return PB_NAVIGATION_PROCEED;
  }
  hooks::Args args;
  args.Int(tab);
  args.Str(from.spec());
  args.Str(to.spec());
  args.Int(handle->HasUserGesture() ? 1 : 0);
  PBValue result;
  if (!hooks::Ask(PB_EVENT_LEAVING_SITE, args, &result)) {
    return PB_NAVIGATION_PROCEED;
  }
  return result.i;
}

}  // namespace events
}  // namespace project_b

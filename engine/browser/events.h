// What the adapter tells Go about a tab's page and about keys: the events 60 to 79 in
// pb_abi.h. They say what happened in the engine interface's own terms, so nothing in
// Go reads content's hooks to know it. A tab's own are fired by PageTab; the two that
// more than a tab needs are here.

#ifndef PROJECT_B_ENGINE_BROWSER_EVENTS_H_
#define PROJECT_B_ENGINE_BROWSER_EVENTS_H_

#include "content/public/browser/navigation_throttle.h"
#include "project_b/engine/abi/pb_abi.h"

namespace content {
class WebContents;
}

namespace input {
struct NativeWebKeyboardEvent;
}

namespace project_b {
namespace events {

// Asks Go about a key press in one of our pages or a tab's, source, and returns whether
// Go used it. Anything but a key press is no question, and nobody's.
bool Key(content::WebContents* source,
         const input::NativeWebKeyboardEvent& event,
         PBKeyPhase phase);

// The gate: every top-level navigation in a tab, ours too, asks Go before it starts and
// on each redirect. One Go takes to another tab is dropped with nothing shown, the tab
// staying as it was. A frame inside a tab's page is refused on a blocked host
// (block_rules.h). The shell's and the picker's navigations go on unasked.
class Gate : public content::NavigationThrottle {
 public:
  explicit Gate(content::NavigationThrottleRegistry& registry);

  // content::NavigationThrottle:
  ThrottleCheckResult WillStartRequest() override;
  ThrottleCheckResult WillRedirectRequest() override;
  const char* GetNameForLogging() override;

 private:
  ThrottleCheckResult Ask();
};

}  // namespace events
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_EVENTS_H_

// The engine's side of the blocker (docs/blocking.md): it keeps the table Go hands over,
// asks it of every request a renderer sends and of each frame's parts as the frame's page
// is about to commit, and tells the renderers whether there is one, its watcher's script,
// the sites where it is off and when its rules change. The blocker is asked on a sequence
// of its own in the thread pool, never on the main thread, which may be busy drawing, nor
// on the IO thread, which carries every other message; C++ calls its functions directly,
// with no Go in between. Shared by the Mac, Linux and Android.

#ifndef PROJECT_B_ENGINE_BROWSER_BLOCKER_H_
#define PROJECT_B_ENGINE_BROWSER_BLOCKER_H_

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

#include "content/public/browser/content_browser_client.h"
#include "project_b/engine/abi/pb_abi.h"

namespace content {
class NavigationHandle;
class NavigationThrottleRegistry;
class RenderProcessHost;
}  // namespace content

namespace network {
class URLLoaderFactoryBuilder;
}

namespace service_manager {
template <typename... BinderArgs>
class BinderRegistryWithArgs;
using BinderRegistry = BinderRegistryWithArgs<>;
}  // namespace service_manager

namespace project_b {
namespace blocker {

// Go's calls (pb_abi.h), on the main thread. Set copies the table, and its watcher's
// script, so Go's copy may go once it returns; null is no blocker at all.
void Set(const PBBlockerHooks* hooks);
void Changed();
void SetOff(const std::vector<std::string>& origins);

// Tells a renderer about to start what the others were told. Main thread.
void RendererLaunching(content::RenderProcessHost* host);
// Takes a renderer's pipe for its questions, bound on the blocker's sequence.
void ExposeTo(service_manager::BinderRegistry* registry);
// Every navigation's: a frame inside a tab's page, over http(s), where the blocker is on,
// is asked about as a request of the page, and every frame's page there waits at its
// response while the blocker says what the page hides and runs.
void AddThrottle(content::NavigationThrottleRegistry& registry);
// A loader factory for a navigation or for a renderer's requests is being made: the
// blocker's stand-ins are served in front of it, for the requests it redirects, so they
// never go out. Main thread.
void WillCreateURLLoaderFactory(content::ContentBrowserClient::URLLoaderFactoryType type,
                                std::optional<int64_t> navigation,
                                network::URLLoaderFactoryBuilder& factory_builder);
// The page is about to commit in its frame: the frame is told what the blocker said, on
// the frame's own channel, so it has it before its document starts.
void ReadyToCommit(content::NavigationHandle* handle);

}  // namespace blocker
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_BLOCKER_H_

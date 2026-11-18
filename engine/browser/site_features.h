// The engine's part of the site features (docs/site-features.md): it hands each renderer
// the ad blocker's rules Go set, asks Go for a frame's sparks as a page is about to commit
// in it and hands them over, eases the style rules of the policy of a page a spark runs
// in, and tells Go of each request the ad blocker kept from its host.
// Shared by the Mac and Android.

#ifndef PROJECT_B_ENGINE_BROWSER_SITE_FEATURES_H_
#define PROJECT_B_ENGINE_BROWSER_SITE_FEATURES_H_

#include <memory>
#include <string>
#include <vector>

#include "base/functional/callback.h"

namespace blink {
class AssociatedInterfaceRegistry;
class URLLoaderThrottle;
}  // namespace blink

namespace network {
struct ResourceRequest;
}

namespace content {
class RenderProcessHost;
class WebContents;
}  // namespace content

namespace project_b {
namespace site_features {

// Keeps the ad blocker's rules and hands them to every renderer. Main thread.
void SetAdRules(std::vector<std::string> hosts, std::vector<std::string> off);
// Hands a renderer about to start the rules as they are.
void RendererLaunching(content::RenderProcessHost* host);
// Takes a renderer's reports of what the ad blocker refused.
void ExposeTo(blink::AssociatedInterfaceRegistry* registry,
              content::RenderProcessHost* host);
// Follows a web tab for its sparks and its blocked requests, for its life.
void Follow(content::WebContents* contents);
// For a page's or a frame's navigation: eases the style rules and Trusted Types of the
// response's policy where a spark runs. Null for any other request.
std::unique_ptr<blink::URLLoaderThrottle> EasePoliciesFor(
    const network::ResourceRequest& request,
    base::RepeatingCallback<content::WebContents*()> contents);

}  // namespace site_features
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_SITE_FEATURES_H_

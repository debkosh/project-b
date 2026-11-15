// The ad blocker's rules, as a renderer keeps them: the hosts Go's list names, and the
// origins of the pages where the person switched it off. The browser sends them to each
// renderer as it starts and again on each change (project_b.mojom); a request follows the
// rules of when it starts.

#ifndef PROJECT_B_ENGINE_COMMON_AD_RULES_H_
#define PROJECT_B_ENGINE_COMMON_AD_RULES_H_

#include <string>
#include <vector>

namespace network {
struct ResourceRequest;
}

namespace project_b {
namespace ad_rules {

// Any thread.
void Set(const std::vector<std::string>& hosts, const std::vector<std::string>& off);

// Whether the blocker keeps the request from its host: one to a listed host or under one,
// from a page whose site isn't the host's own and where the blocker isn't off. Any thread.
bool Blocks(const network::ResourceRequest& request);

}  // namespace ad_rules
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_AD_RULES_H_

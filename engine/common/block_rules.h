// The hosts Go's policy blocks, which no request of a page may reach: not the page, its
// frames, its images, scripts, fetches and workers, nor its WebSockets. Go hands them
// over once, at start (set_block_rules in pb_abi.h); each renderer gets them on its
// command line, so a list set later reaches only renderers started after.

#ifndef PROJECT_B_ENGINE_COMMON_BLOCK_RULES_H_
#define PROJECT_B_ENGINE_COMMON_BLOCK_RULES_H_

#include <string>
#include <vector>

class GURL;

namespace base {
class CommandLine;
}

namespace project_b {
namespace block_rules {

// The switch that carries the hosts to a renderer, separated by commas.
inline constexpr char kSwitch[] = "pb-block-hosts";

// Sets the hosts, each lower case. On the main thread, before anything asks.
void Set(std::vector<std::string> hosts);
// Reads them from this process's command line: a renderer's, as it starts.
void SetFromCommandLine(const base::CommandLine& command_line);
// Adds them to a child's command line, when there are any.
void AppendTo(base::CommandLine* command_line);

bool Any();
// Whether url's host is one of the hosts or under one. Any thread, once set.
bool Blocks(const GURL& url);

}  // namespace block_rules
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_BLOCK_RULES_H_

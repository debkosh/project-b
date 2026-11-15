#include "project_b/engine/common/block_rules.h"

#include <utility>

#include "base/command_line.h"
#include "base/no_destructor.h"
#include "base/strings/string_split.h"
#include "base/strings/string_util.h"
#include "url/gurl.h"

namespace project_b {
namespace block_rules {

namespace {

// Written once, on the main thread, before any request; read from any thread after.
std::vector<std::string>& Hosts() {
  static base::NoDestructor<std::vector<std::string>> hosts;
  return *hosts;
}

}  // namespace

void Set(std::vector<std::string> hosts) {
  Hosts() = std::move(hosts);
}

void SetFromCommandLine(const base::CommandLine& command_line) {
  Set(base::SplitString(command_line.GetSwitchValueASCII(kSwitch), ",",
                        base::TRIM_WHITESPACE, base::SPLIT_WANT_NONEMPTY));
}

void AppendTo(base::CommandLine* command_line) {
  if (Any()) {
    command_line->AppendSwitchASCII(kSwitch, base::JoinString(Hosts(), ","));
  }
}

bool Any() {
  return !Hosts().empty();
}

bool Blocks(const GURL& url) {
  if (!url.has_host()) {
    return false;
  }
  const std::string host = base::ToLowerASCII(url.host());
  for (const std::string& blocked : Hosts()) {
    if (host == blocked || base::EndsWith(host, "." + blocked)) {
      return true;
    }
  }
  return false;
}

}  // namespace block_rules
}  // namespace project_b

#include "project_b/engine/common/ad_rules.h"

#include <optional>

#include "base/containers/flat_set.h"
#include "base/no_destructor.h"
#include "base/strings/string_util.h"
#include "base/synchronization/lock.h"
#include "net/base/schemeful_site.h"
#include "services/network/public/cpp/resource_request.h"
#include "url/gurl.h"
#include "url/origin.h"

namespace project_b {
namespace ad_rules {

namespace {

struct Rules {
  base::Lock lock;
  base::flat_set<std::string> hosts;
  // The sites of the pages where the blocker is off.
  base::flat_set<net::SchemefulSite> off;
};

Rules& Get() {
  static base::NoDestructor<Rules> rules;
  return *rules;
}

// Whether host or a host it is under is listed: a.ads.test is under ads.test.
bool Listed(const base::flat_set<std::string>& hosts, std::string_view host) {
  while (!host.empty()) {
    if (hosts.contains(host)) {
      return true;
    }
    const size_t dot = host.find('.');
    if (dot == std::string_view::npos) {
      return false;
    }
    host.remove_prefix(dot + 1);
  }
  return false;
}

// The site of the page a request is for: its top frame's, as cookies see it, or else the
// site that asked for it.
std::optional<net::SchemefulSite> PageSite(const network::ResourceRequest& request) {
  if (!request.site_for_cookies.IsNull()) {
    return request.site_for_cookies.site();
  }
  if (request.request_initiator) {
    return net::SchemefulSite(*request.request_initiator);
  }
  return std::nullopt;
}

}  // namespace

void Set(const std::vector<std::string>& hosts, const std::vector<std::string>& off) {
  base::flat_set<net::SchemefulSite> sites;
  for (const std::string& origin : off) {
    sites.insert(net::SchemefulSite(GURL(origin)));
  }
  Rules& rules = Get();
  base::AutoLock hold(rules.lock);
  rules.hosts = base::flat_set<std::string>(hosts.begin(), hosts.end());
  rules.off = std::move(sites);
}

bool Blocks(const network::ResourceRequest& request) {
  const GURL& url = request.url;
  if (!url.SchemeIsHTTPOrHTTPS() && !url.SchemeIsWSOrWSS()) {
    return false;
  }
  const std::string host = base::ToLowerASCII(url.host());
  const std::optional<net::SchemefulSite> page = PageSite(request);
  Rules& rules = Get();
  base::AutoLock hold(rules.lock);
  if (!Listed(rules.hosts, host)) {
    return false;
  }
  // A site's own requests aren't ads to it, and a page where it is off asks for anything.
  return !page || (*page != net::SchemefulSite(url) && !rules.off.contains(*page));
}

}  // namespace ad_rules
}  // namespace project_b

#include "project_b/engine/browser/fetch.h"

#include <map>
#include <memory>
#include <optional>
#include <utility>

#include "base/functional/bind.h"
#include "base/no_destructor.h"
#include "build/build_config.h"
#include "content/public/browser/storage_partition.h"
#include "net/base/isolation_info.h"
#include "net/base/net_errors.h"
#include "net/cookies/site_for_cookies.h"
#include "net/http/http_content_disposition.h"
#include "net/http/http_request_headers.h"
#include "net/http/http_response_headers.h"
#include "net/traffic_annotation/network_traffic_annotation.h"
#include "net/url_request/referrer_policy.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "services/network/public/cpp/ip_address_space_util.h"
#include "services/network/public/cpp/resource_request.h"
#include "services/network/public/cpp/shared_url_loader_factory.h"
#include "services/network/public/cpp/simple_url_loader.h"
#include "services/network/public/cpp/url_loader_completion_status.h"
#include "services/network/public/mojom/client_security_state.mojom.h"
#include "services/network/public/mojom/fetch_api.mojom.h"
#include "services/network/public/mojom/ip_address_space.mojom.h"
#include "services/network/public/mojom/url_response_head.mojom.h"
#include "url/origin.h"

namespace project_b {

namespace {

std::map<int64_t, std::unique_ptr<network::SimpleURLLoader>>& Running() {
  static base::NoDestructor<
      std::map<int64_t, std::unique_ptr<network::SimpleURLLoader>>>
      running;
  return *running;
}

network::mojom::IPAddressSpace SpaceOf(int32_t from) {
  switch (from) {
    case PB_SPACE_LOOPBACK:
      return network::mojom::IPAddressSpace::kLoopback;
    case PB_SPACE_LOCAL:
      return network::mojom::IPAddressSpace::kLocal;
    default:
      return network::mojom::IPAddressSpace::kPublic;
  }
}

// An address space in Go's numbers; one content doesn't know is public, as its checks
// have it.
int32_t ToGo(network::mojom::IPAddressSpace space) {
  switch (space) {
    case network::mojom::IPAddressSpace::kLoopback:
      return PB_SPACE_LOOPBACK;
    case network::mojom::IPAddressSpace::kLocal:
      return PB_SPACE_LOCAL;
    default:
      return PB_SPACE_PUBLIC;
  }
}

void Tell(int64_t id,
          int32_t result,
          int status,
          int32_t space,
          const std::string& url,
          const std::string& mime_type,
          const std::string& name) {
  hooks::Args args;
  args.Int(id);
  args.Int(result);
  args.Int(status);
  args.Int(space);
  args.Str(url);
  args.Str(mime_type);
  args.Str(name);
  hooks::Fire(PB_EVENT_FETCHED, args);
}

// The network service's own refusals of a server more private than the asker.
bool RefusedAsLocal(const network::SimpleURLLoader& loader) {
  switch (loader.NetError()) {
    case net::ERR_BLOCKED_BY_LOCAL_NETWORK_ACCESS_CHECKS:
    case net::ERR_CACHED_IP_ADDRESS_SPACE_BLOCKED_BY_LOCAL_NETWORK_ACCESS_POLICY:
    case net::ERR_INCONSISTENT_IP_ADDRESS_SPACE:
    case net::ERR_UNSAFE_REDIRECT:
    case net::ERR_UNKNOWN_URL_SCHEME:
    case net::ERR_DISALLOWED_URL_SCHEME:
      return true;
    default:
      return loader.CompletionStatus() &&
             loader.CompletionStatus()->cors_error_status.has_value();
  }
}

void Fetched(int64_t id, base::FilePath file) {
  auto it = Running().find(id);
  if (it == Running().end()) {
    return;
  }
  std::unique_ptr<network::SimpleURLLoader> loader = std::move(it->second);
  Running().erase(it);
  const network::mojom::URLResponseHead* head = loader->ResponseInfo();
  int status = 0;
  int32_t space = PB_SPACE_PUBLIC;
  std::string mime_type, name;
  if (head) {
    if (head->headers) {
      status = head->headers->response_code();
      if (std::optional<std::string> disposition =
              head->headers->GetNormalizedHeader("content-disposition")) {
        name = net::HttpContentDisposition(*disposition, std::string()).filename();
      }
    }
    mime_type = head->mime_type;
    space = ToGo(head->remote_endpoint.address().IsValid()
                     ? network::IPEndPointToIPAddressSpace(head->remote_endpoint)
                     : head->response_address_space);
  }
  int32_t result = PB_FETCH_OK;
  if (file.empty()) {
    result = RefusedAsLocal(*loader) ? PB_FETCH_REFUSED
             : status >= 400         ? PB_FETCH_SERVER
                                     : PB_FETCH_CONNECTION;
  }
  Tell(id, result, status, space, loader->GetFinalURL().spec(), mime_type, name);
}

}  // namespace

void Fetch(int64_t id,
           content::StoragePartition* partition,
           const GURL& page,
           const GURL& url,
           const base::FilePath& path,
           const std::string& range,
           int32_t from) {
  if (!url.SchemeIsHTTPOrHTTPS() || !partition) {
    Tell(id, PB_FETCH_REFUSED, 0, PB_SPACE_PUBLIC, url.spec(), std::string(),
         std::string());
    return;
  }
  auto request = std::make_unique<network::ResourceRequest>();
  request->url = url;
  // As a page's own request for a file of another site: no CORS is asked of the server,
  // and the cookies that go are the ones Chromium would send from that page.
  request->mode = network::mojom::RequestMode::kNoCors;
  request->credentials_mode = network::mojom::CredentialsMode::kInclude;
  // The person's own request is the address's own first party.
  const url::Origin origin = url::Origin::Create(page.is_valid() ? page : url);
  const net::SiteForCookies site =
      net::SiteForCookies::FromUrl(page.is_valid() ? page : url);
  request->site_for_cookies = site;
  network::ResourceRequest::TrustedParams trusted;
  trusted.isolation_info = net::IsolationInfo::Create(
      net::IsolationInfo::RequestType::kOther, origin, origin, site);
  if (page.is_valid()) {
    request->request_initiator = origin;
    request->referrer = page.GetAsReferrer();
    request->referrer_policy =
        net::ReferrerPolicy::REDUCE_GRANULARITY_ON_TRANSITION_CROSS_ORIGIN;
  }
  if (from != PB_SPACE_ANY) {
    auto state = network::mojom::ClientSecurityState::New();
    state->is_web_secure_context = false;
    state->ip_address_space = SpaceOf(from);
    state->local_network_access_request_policy =
        network::mojom::LocalNetworkAccessRequestPolicy::kBlock;
    trusted.client_security_state = std::move(state);
  }
  request->trusted_params = std::move(trusted);
  if (!range.empty()) {
    request->headers.SetHeader(net::HttpRequestHeaders::kRange, "bytes=" + range);
  }
  // Linux and Windows want every request to say why it is made; the Mac doesn't yet.
#if BUILDFLAG(IS_LINUX) || BUILDFLAG(IS_WIN)
  const net::NetworkTrafficAnnotationTag annotation = MISSING_TRAFFIC_ANNOTATION;
#else
  const net::NetworkTrafficAnnotationTag annotation = NO_TRAFFIC_ANNOTATION_YET;
#endif
  auto loader = network::SimpleURLLoader::Create(std::move(request), annotation);
  network::SimpleURLLoader* raw = loader.get();
  Running()[id] = std::move(loader);
  raw->DownloadToFile(partition->GetURLLoaderFactoryForBrowserProcess().get(),
                      base::BindOnce(&Fetched, id), path);
}

void CancelFetch(int64_t id) {
  Running().erase(id);
}

}  // namespace project_b

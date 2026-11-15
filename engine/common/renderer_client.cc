#include "project_b/engine/common/renderer_client.h"

#include <vector>

#include "base/command_line.h"
#include "net/base/net_errors.h"
#include "net/url_request/redirect_info.h"
#include "project_b/engine/common/block_rules.h"
#include "services/network/public/cpp/resource_request.h"
#include "third_party/blink/public/common/loader/url_loader_throttle.h"
#include "third_party/blink/public/platform/url_loader_throttle_provider.h"

namespace project_b {

namespace {

// Fails a request to a blocked host as it starts or as it is redirected there.
class BlockThrottle : public blink::URLLoaderThrottle {
 public:
  void WillStartRequest(network::ResourceRequest* request,
                        bool* defer) override {
    if (block_rules::Blocks(request->url)) {
      delegate_->CancelWithError(net::ERR_BLOCKED_BY_CLIENT);
    }
  }

  void WillRedirectRequest(
      net::RedirectInfo* redirect_info,
      const network::mojom::URLResponseHead& response_head,
      bool* defer,
      network::HttpRequestHeadersUpdateParams* headers_update_params) override {
    if (block_rules::Blocks(redirect_info->new_url)) {
      delegate_->CancelWithError(net::ERR_BLOCKED_BY_CLIENT);
    }
  }
};

class BlockThrottleProvider : public blink::URLLoaderThrottleProvider {
 public:
  std::unique_ptr<blink::URLLoaderThrottleProvider> Clone() override {
    return std::make_unique<BlockThrottleProvider>();
  }

  std::vector<std::unique_ptr<blink::URLLoaderThrottle>> CreateThrottles(
      base::optional_ref<const blink::LocalFrameToken> local_frame_token,
      const network::ResourceRequest& request) override {
    std::vector<std::unique_ptr<blink::URLLoaderThrottle>> throttles;
    throttles.push_back(std::make_unique<BlockThrottle>());
    return throttles;
  }

  void SetOnline(bool is_online) override {}
};

}  // namespace

RendererClient::RendererClient() {
  block_rules::SetFromCommandLine(*base::CommandLine::ForCurrentProcess());
}

RendererClient::~RendererClient() = default;

std::unique_ptr<blink::URLLoaderThrottleProvider>
RendererClient::CreateURLLoaderThrottleProvider(
    blink::URLLoaderThrottleProviderType provider_type) {
  if (!block_rules::Any()) {
    return nullptr;
  }
  return std::make_unique<BlockThrottleProvider>();
}

}  // namespace project_b

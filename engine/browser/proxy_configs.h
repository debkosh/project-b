#ifndef PROJECT_B_ENGINE_BROWSER_PROXY_CONFIGS_H_
#define PROJECT_B_ENGINE_BROWSER_PROXY_CONFIGS_H_

#include <memory>

#include "base/no_destructor.h"
#include "mojo/public/cpp/bindings/receiver_set.h"
#include "mojo/public/cpp/bindings/remote_set.h"
#include "net/proxy_resolution/proxy_config_service.h"
#include "services/network/public/mojom/network_context.mojom.h"
#include "services/network/public/mojom/proxy_config_with_annotation.mojom.h"

// The proxy every network context goes through, as Chrome's ProxyConfigMonitor gives it
// without its prefs: --no-proxy-server, --proxy-pac-url or --proxy-server when the command
// line has one, else the system's own settings, and a change to them reaches every context.
// With no config at all content made every connection direct, so a network reached only
// through a proxy loaded nothing. One for the app, on the UI thread.
class ProxyConfigs : public net::ProxyConfigService::Observer,
                     public network::mojom::ProxyConfigPollerClient {
 public:
  static ProxyConfigs* Get();

  ProxyConfigs(const ProxyConfigs&) = delete;
  ProxyConfigs& operator=(const ProxyConfigs&) = delete;

  // Gives a context about to be made the config now, and a way to hear of changes.
  void AddTo(network::mojom::NetworkContextParams* params);

 private:
  friend class base::NoDestructor<ProxyConfigs>;

  ProxyConfigs();
  ~ProxyConfigs() override;

  // net::ProxyConfigService::Observer:
  void OnProxyConfigChanged(
      const net::ProxyConfigWithAnnotation& config,
      net::ProxyConfigService::ConfigAvailability availability) override;
  // network::mojom::ProxyConfigPollerClient:
  void OnLazyProxyConfigPoll() override;

  std::unique_ptr<net::ProxyConfigService> service_;
  mojo::RemoteSet<network::mojom::ProxyConfigClient> clients_;
  mojo::ReceiverSet<network::mojom::ProxyConfigPollerClient> pollers_;
};

#endif  // PROJECT_B_ENGINE_BROWSER_PROXY_CONFIGS_H_

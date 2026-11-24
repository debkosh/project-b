#include "project_b/engine/browser/proxy_configs.h"

#include <string>

#include "base/command_line.h"
#include "base/no_destructor.h"
#include "base/task/single_thread_task_runner.h"
#include "net/proxy_resolution/proxy_config.h"
#include "net/proxy_resolution/proxy_config_service_fixed.h"
#include "net/proxy_resolution/proxy_config_with_annotation.h"
#include "net/traffic_annotation/network_traffic_annotation.h"
#include "url/gurl.h"

namespace {

// Chrome's switches, by the same names (chrome/common/chrome_switches.h).
constexpr char kNoProxyServer[] = "no-proxy-server";
constexpr char kProxyPacUrl[] = "proxy-pac-url";
constexpr char kProxyServer[] = "proxy-server";
constexpr char kProxyBypassList[] = "proxy-bypass-list";

constexpr net::NetworkTrafficAnnotationTag kCommandLineProxy =
    net::DefineNetworkTrafficAnnotation("project_b_command_line_proxy", R"(
      semantics {
        sender: "Proxy config"
        description: "The proxy given on the command line, which every request goes through."
        trigger: "Any request, when the app was started with a proxy switch."
        data: "Whatever the request sends."
        destination: OTHER
      }
      policy {
        cookies_allowed: NO
        setting: "The --proxy-server, --proxy-pac-url and --no-proxy-server switches."
      })");

// The command line's proxy, or null when it names none.
std::unique_ptr<net::ProxyConfigService> FromCommandLine() {
  const base::CommandLine& line = *base::CommandLine::ForCurrentProcess();
  net::ProxyConfig config;
  if (line.HasSwitch(kNoProxyServer)) {
    config = net::ProxyConfig::CreateDirect();
  } else if (line.HasSwitch(kProxyPacUrl)) {
    config = net::ProxyConfig::CreateFromCustomPacURL(
        GURL(line.GetSwitchValueASCII(kProxyPacUrl)));
  } else if (line.HasSwitch(kProxyServer)) {
    config.proxy_rules().ParseFromString(line.GetSwitchValueASCII(kProxyServer));
    config.proxy_rules().bypass_rules.ParseFromString(
        line.GetSwitchValueASCII(kProxyBypassList));
  } else {
    return nullptr;
  }
  return std::make_unique<net::ProxyConfigServiceFixed>(
      net::ProxyConfigWithAnnotation(config, kCommandLineProxy));
}

}  // namespace

// static
ProxyConfigs* ProxyConfigs::Get() {
  static base::NoDestructor<ProxyConfigs> configs;
  return configs.get();
}

ProxyConfigs::ProxyConfigs() {
  service_ = FromCommandLine();
  if (!service_) {
    service_ = net::ProxyConfigService::CreateSystemProxyConfigService(
        base::SingleThreadTaskRunner::GetCurrentDefault());
  }
  service_->AddObserver(this);
}

ProxyConfigs::~ProxyConfigs() {
  service_->RemoveObserver(this);
}

void ProxyConfigs::AddTo(network::mojom::NetworkContextParams* params) {
  mojo::PendingRemote<network::mojom::ProxyConfigClient> client;
  params->proxy_config_client_receiver = client.InitWithNewPipeAndPassReceiver();
  clients_.Add(std::move(client));
  if (service_->UsesPolling()) {
    pollers_.Add(this, params->proxy_config_poller_client.InitWithNewPipeAndPassReceiver());
  }
  net::ProxyConfigWithAnnotation config;
  if (service_->GetLatestProxyConfig(&config) !=
      net::ProxyConfigService::CONFIG_PENDING) {
    params->initial_proxy_config = config;
  }
}

void ProxyConfigs::OnProxyConfigChanged(
    const net::ProxyConfigWithAnnotation& config,
    net::ProxyConfigService::ConfigAvailability availability) {
  if (availability == net::ProxyConfigService::CONFIG_PENDING) {
    return;
  }
  for (const auto& client : clients_) {
    client->OnProxyConfigUpdated(
        availability == net::ProxyConfigService::CONFIG_VALID
            ? config
            : net::ProxyConfigWithAnnotation::CreateDirect());
  }
}

void ProxyConfigs::OnLazyProxyConfigPoll() {
  service_->OnLazyPoll();
}

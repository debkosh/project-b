#include "project_b/engine/browser/secure_dns.h"

#include <memory>
#include <optional>
#include <set>
#include <utility>

#include "base/feature_list.h"
#include "base/logging.h"
#include "base/no_destructor.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/network_service_instance.h"
#include "content/public/browser/storage_partition.h"
#include "mojo/public/cpp/bindings/receiver.h"
#include "mojo/public/cpp/bindings/remote.h"
#include "net/base/features.h"
#include "net/base/host_port_pair.h"
#include "net/base/network_anonymization_key.h"
#include "net/dns/public/dns_config_overrides.h"
#include "net/dns/public/dns_over_https_config.h"
#include "net/dns/public/resolve_error_info.h"
#include "net/dns/public/secure_dns_mode.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "services/network/public/mojom/host_resolver.mojom.h"
#include "services/network/public/mojom/network_context.mojom.h"
#include "services/network/public/mojom/network_service.mojom.h"

namespace secure_dns {

namespace {

// The name a check looks up, Chrome's DnsProbeRunner::kKnownGoodHostname.
constexpr char kKnownName[] = "google.com";

struct Config {
  net::SecureDnsMode mode = net::SecureDnsMode::kOff;
  net::DnsOverHttpsConfig doh;
  // set is whether Set was ever called: until then the network service keeps its own
  // start, which is Off.
  bool set = false;
};

Config& Current() {
  static base::NoDestructor<Config> config;
  return *config;
}

// One check, a probe: a resolver of its own on the network context, asked once, which fires the
// event with the answer and goes.
class Probe final : public network::mojom::ResolveHostClient {
 public:
  Probe(content::BrowserContext* context,
        const std::string& doh_template,
        net::DnsOverHttpsConfig doh)
      : template_(doh_template) {
    net::DnsConfigOverrides overrides;
    overrides.search = std::vector<std::string>();
    overrides.attempts = 1;
    overrides.secure_dns_mode = net::SecureDnsMode::kSecure;
    overrides.dns_over_https_config = std::move(doh);
    context->GetDefaultStoragePartition()->GetNetworkContext()->CreateHostResolver(
        overrides, resolver_.BindNewPipeAndPassReceiver());
    auto parameters = network::mojom::ResolveHostParameters::New();
    parameters->dns_query_type = net::DnsQueryType::A;
    parameters->source = net::HostResolverSource::DNS;
    parameters->cache_usage =
        network::mojom::ResolveHostParameters::CacheUsage::DISALLOWED;
    resolver_->ResolveHost(network::mojom::HostResolverHost::NewHostPortPair(
                               net::HostPortPair(kKnownName, 443)),
                           net::NetworkAnonymizationKey::CreateTransient(),
                           std::move(parameters),
                           receiver_.BindNewPipeAndPassRemote());
    receiver_.set_disconnect_handler(
        base::BindOnce(&Probe::Done, base::Unretained(this), false));
  }

  static std::set<std::unique_ptr<Probe>, std::less<>>& Running() {
    static base::NoDestructor<std::set<std::unique_ptr<Probe>, std::less<>>> running;
    return *running;
  }

  // network::mojom::ResolveHostClient:
  void OnComplete(int32_t result,
                  const net::ResolveErrorInfo& error,
                  const net::AddressList& addresses,
                  const net::HostResolverEndpointResults&) override {
    Done(error.error == net::OK && !addresses.empty());
  }
  void OnTextResults(const std::vector<std::string>&) override {}
  void OnHostnameResults(const std::vector<net::HostPortPair>&) override {}

 private:
  // Fires the answer and deletes this check.
  void Done(bool ok) {
    project_b::hooks::Args args;
    args.Str(template_);
    args.Int(ok ? 1 : 0);
    project_b::hooks::Fire(PB_EVENT_SECURE_DNS_CHECKED, args);
    auto& running = Running();
    for (auto it = running.begin(); it != running.end(); ++it) {
      if (it->get() == this) {
        running.erase(it);  // Deletes this.
        return;
      }
    }
  }

  std::string template_;
  mojo::Remote<network::mojom::HostResolver> resolver_;
  mojo::Receiver<network::mojom::ResolveHostClient> receiver_{this};
};

}  // namespace

void Set(int mode, const std::string& doh_template) {
  Config& c = Current();
  c.set = true;
  c.mode = net::SecureDnsMode::kOff;
  c.doh = net::DnsOverHttpsConfig();
  if (mode != 0) {
    std::optional<net::DnsOverHttpsConfig> doh =
        net::DnsOverHttpsConfig::FromString(doh_template);
    if (doh && !doh->servers().empty()) {
      c.mode = mode == 2 ? net::SecureDnsMode::kSecure
                         : net::SecureDnsMode::kAutomatic;
      c.doh = std::move(*doh);
    } else {
      LOG(WARNING) << "secure dns: not a DNS-over-HTTPS template: "
                   << doh_template;
    }
  }
  Apply(content::GetNetworkService());
}

void Apply(network::mojom::NetworkService* service) {
  const Config& c = Current();
  if (!c.set || !service) {
    return;
  }
  // The plain part stays as content starts it, the system's own resolver
  // (ContentBrowserClient::GetInitialInsecureDnsMode), and Happy Eyeballs as
  // net's feature has it; only the secure part is ours.
  service->ConfigureStubHostResolver(
      net::InsecureDnsMode::kDisabled,
      base::FeatureList::IsEnabled(net::features::kHappyEyeballsV3), c.mode,
      c.doh, /*additional_dns_types_enabled=*/true,
      /*fallback_doh_nameservers=*/{});
}

void Check(content::BrowserContext* context, const std::string& doh_template) {
  std::optional<net::DnsOverHttpsConfig> doh =
      net::DnsOverHttpsConfig::FromString(doh_template);
  if (!doh || doh->servers().empty()) {
    project_b::hooks::Args args;
    args.Str(doh_template);
    args.Int(0);
    project_b::hooks::Fire(PB_EVENT_SECURE_DNS_CHECKED, args);
    return;
  }
  Probe::Running().insert(
      std::make_unique<Probe>(context, doh_template, std::move(*doh)));
}

}  // namespace secure_dns

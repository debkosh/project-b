#ifndef PROJECT_B_ENGINE_BROWSER_SECURE_DNS_H_
#define PROJECT_B_ENGINE_BROWSER_SECURE_DNS_H_

#include <string>

namespace content {
class BrowserContext;
}

namespace network::mojom {
class NetworkService;
}

// Secure DNS (docs/secure-dns.md): Chromium's own DNS-over-HTTPS, which the network
// service runs for every network context once told the mode and the server, as Chrome's
// StubResolverConfigReader tells it. On the UI thread.
namespace secure_dns {

// Sets the mode, net::SecureDnsMode's value, and the server's template, and tells the
// network service. A template net::DnsOverHttpsConfig doesn't take is Off.
void Set(int mode, const std::string& doh_template);

// Tells a network service just made, or made again after a crash, what Set last said.
void Apply(network::mojom::NetworkService* service);

// Asks template's server alone, in Secure mode, to look a known name up through context's
// network context, as Chrome's DnsProbeRunner does, and fires
// PB_EVENT_SECURE_DNS_CHECKED with the answer.
void Check(content::BrowserContext* context, const std::string& doh_template);

}  // namespace secure_dns

#endif  // PROJECT_B_ENGINE_BROWSER_SECURE_DNS_H_

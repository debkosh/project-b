#ifndef PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_
#define PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_

#include <memory>
#include <string>
#include <vector>

#include "build/build_config.h"
#include "content/public/browser/login_delegate.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/buildflags/buildflags.h"

namespace project_b {

// The general counter: content asks this object questions. The generated base carries
// them to Go; what is answered here needs content's objects or runs before Go starts.
class ContentBrowserClient : public ContentBrowserClientHooks {
 public:
  ContentBrowserClient();
  ContentBrowserClient(const ContentBrowserClient&) = delete;
  ContentBrowserClient& operator=(const ContentBrowserClient&) = delete;
  ~ContentBrowserClient() override;

  // Empty binders for the interfaces Chrome binds that pages ask for with no feature to
  // turn off, so a page that asks keeps its renderer: app badges, and view-source's line
  // wrap box. Their calls go nowhere.
  void RegisterBrowserInterfaceBindersForFrame(
      content::RenderFrameHost* render_frame_host,
      mojo::BinderMapWithContext<content::RenderFrameHost*>* map) override;
  void RegisterBrowserInterfaceBindersForServiceWorker(
      content::BrowserContext* browser_context,
      const content::ServiceWorkerVersionBaseInfo& service_worker_version_info,
      mojo::BinderMapWithContext<const content::ServiceWorkerVersionBaseInfo&>* map)
      override;

  // A renderer learns the blocked hosts on its command line, and refuses them itself.
  void RenderProcessWillLaunch(content::RenderProcessHost* host) override;
  void ExposeInterfacesToRenderer(
      service_manager::BinderRegistry* registry,
      blink::AssociatedInterfaceRegistry* associated_registry,
      content::RenderProcessHost* render_process_host) override;
  // Each frame's printing: Chromium's print manager, and its window.print().
  void RegisterAssociatedInterfaceBindersForRenderFrameHost(
      content::RenderFrameHost& render_frame_host,
      blink::AssociatedInterfaceRegistry& associated_registry) override;
  void AppendExtraCommandLineSwitches(base::CommandLine* command_line,
                                      int child_process_id) override;
  // The blocker's stand-ins are served in front of the network (blocker.h). A build with
  // extensions calls it first from its own.
  void WillCreateURLLoaderFactory(
      content::BrowserContext* browser_context,
      content::RenderFrameHost* frame,
      int render_process_id,
      URLLoaderFactoryType type,
      const url::Origin& request_initiator,
      const net::IsolationInfo& isolation_info,
      std::optional<int64_t> navigation_id,
      ukm::SourceIdObj ukm_source_id,
      network::URLLoaderFactoryBuilder& factory_builder,
      mojo::PendingRemote<network::mojom::TrustedURLLoaderHeaderClient>*
          header_client,
      bool* bypass_redirect_checks,
      bool* disable_secure_dns,
      network::mojom::URLLoaderFactoryOverridePtr* factory_override,
      scoped_refptr<base::SequencedTaskRunner> navigation_response_task_runner,
      bool is_for_network_service) override;

 private:
  std::unique_ptr<content::BrowserMainParts> CreateBrowserMainParts(
      bool is_integration_test) override;
#if BUILDFLAG(IS_ANDROID)
  void GetAdditionalMappedFilesForChildProcess(
      const base::CommandLine& command_line,
      int child_process_id,
      content::PosixFileDescriptorInfo* mappings) override;
  bool IsContentSchemeURLNavigationAllowed(const GURL& url) override;
#else
  base::FilePath GetChildProcessPath(int flags,
                                     content::ChildProcessId child_process_id) override;
  std::unique_ptr<content::DevToolsManagerDelegate>
  CreateDevToolsManagerDelegate() override;
  // A page's passkey request goes to Go as a dialog with steps (docs/passkeys.md). A
  // phone's never reach content's own authenticator, so this is the Mac's.
  content::WebAuthenticationDelegate* GetWebAuthenticationDelegate() override;
  std::unique_ptr<content::AuthenticatorRequestClientDelegate>
  GetWebAuthenticationRequestDelegate(
      content::RenderFrameHost* render_frame_host) override;
#endif
  void CreateThrottlesForNavigation(
      content::NavigationThrottleRegistry& registry) override;
  // A navigation whose page a spark runs in has its policy's style rules eased.
  std::vector<std::unique_ptr<blink::URLLoaderThrottle>> CreateURLLoaderThrottles(
      const network::ResourceRequest& request,
      content::BrowserContext* browser_context,
      const base::RepeatingCallback<content::WebContents*()>& wc_getter,
      content::NavigationUIData* navigation_ui_data,
      content::FrameTreeNodeId frame_tree_node_id,
      std::optional<int64_t> navigation_id) override;
  // A page's WebSocket to a blocked host never opens.
  bool WillInterceptWebSocket(content::RenderFrameHost* frame) override;
  void CreateWebSocket(
      content::RenderFrameHost* frame,
      WebSocketFactory factory,
      const GURL& url,
      const net::SiteForCookies& site_for_cookies,
      const std::optional<std::string>& user_agent,
      mojo::PendingRemote<network::mojom::WebSocketHandshakeClient>
          handshake_client,
      WebSocketOptions options) override;
  std::string GetProduct() override;
  // A site's or a proxy's request for a username and password goes to Go as a dialog.
  std::unique_ptr<content::LoginDelegate> CreateLoginDelegate(
      const net::AuthChallengeInfo& auth_info,
      content::WebContents* web_contents,
      content::BrowserContext* browser_context,
      const content::GlobalRequestID& request_id,
      bool is_request_for_primary_main_frame_navigation,
      bool is_request_for_navigation,
      const GURL& url,
      scoped_refptr<net::HttpResponseHeaders> response_headers,
      bool first_auth_attempt,
      bool do_not_prompt_for_login,
      content::GuestPageHolder* guest_page_holder,
      content::LoginDelegate::LoginAuthRequiredCallback auth_required_callback) override;
  std::string GetUserAgent() override;
  blink::UserAgentMetadata GetUserAgentMetadata() override;
  // A site that asks for a client certificate gets none: the request goes on without one, as
  // Chrome's does when it has none to offer. Content's own answer drops the request instead,
  // which fails the page with ERR_SSL_CLIENT_AUTH_CERT_NEEDED even where the certificate is
  // optional. Choosing a certificate isn't built.
  base::OnceClosure SelectClientCertificate(
      content::BrowserContext* browser_context,
      content::ChildProcessId process_id,
      content::WebContents* web_contents,
      net::SSLCertRequestInfo* cert_request_info,
      net::ClientCertIdentityList client_certs,
      std::unique_ptr<content::ClientCertificateDelegate> delegate) override;
  // A page's preferred colour scheme is the app's theme, or as the system is.
  void OverrideWebPreferences(content::WebContents* web_contents,
                              content::SiteInstance& main_frame_site,
                              blink::web_pref::WebPreferences* prefs) override;
  bool WebPreferencesNeedUpdateForColorRelatedStateChanges(
      content::WebContents& web_contents,
      const content::SiteInstance& main_frame_site) const override;
  bool ShouldForceDownloadResource(content::BrowserContext* browser_context,
                                   const GURL& url,
                                   const std::string& mime_type) override;
  void OnNetworkServiceCreated(
      network::mojom::NetworkService* network_service) override;
  void ConfigureNetworkContextParams(
      content::BrowserContext* context,
      bool in_memory,
      const base::FilePath& relative_partition_path,
      network::mojom::NetworkContextParams* network_context_params,
      cert_verifier::mojom::CertVerifierCreationParams*
          cert_verifier_creation_params) override;
  std::vector<base::FilePath> GetNetworkContextsParentDirectory() override;
  // A certificate error: Go says whether the request goes through, in our own terms
  // (docs/daily-driver.md).
  void AllowCertificateError(
      content::WebContents* web_contents,
      int cert_error,
      const net::SSLInfo& ssl_info,
      const GURL& request_url,
      bool is_primary_main_frame_request,
      bool strict_enforcement,
      base::OnceCallback<void(content::CertificateRequestResultType)> callback)
      override;

};

#if BUILDFLAG(PB_EXTENSIONS)
// The counter when the Extensions flag is on (ExtensionsOn()): it adds what //extensions
// needs from content's embedder: an extension's pages in a process of their own, the
// chrome-extension: scheme, its Mojo interfaces, and its place in front of the network
// for the request APIs. All in engine/extensions/browser/content_hooks.cc. With the flag
// off the plain counter is used and none of it runs.
class ExtensionsContentBrowserClient : public ContentBrowserClient {
 private:
  void SiteInstanceGotProcessAndSite(
      content::SiteInstance* site_instance) override;
  bool DoesSiteRequireDedicatedProcess(content::BrowserContext* browser_context,
                                       const GURL& effective_site_url) override;
  bool ShouldUseProcessPerSite(
      content::BrowserContext* browser_context,
      const content::SecurityPrincipal& security_principal) override;
  bool IsSuitableHost(
      content::RenderProcessHost* process_host,
      const content::SecurityPrincipal& security_principal) override;
  bool CanCommitURL(content::RenderProcessHost* process_host,
                    const GURL& url) override;
  bool IsHandledURL(const GURL& url) override;
  bool ShouldUseSpareRenderProcessHost(
      content::BrowserContext* browser_context,
      const GURL& site_url,
      std::optional<SpareProcessRefusedByEmbedderReason>& refused_reason)
      override;
  void AppendExtraCommandLineSwitches(base::CommandLine* command_line,
                                      int child_process_id) override;
  void ExposeInterfacesToRenderer(
      service_manager::BinderRegistry* registry,
      blink::AssociatedInterfaceRegistry* associated_registry,
      content::RenderProcessHost* render_process_host) override;
  void RegisterAssociatedInterfaceBindersForRenderFrameHost(
      content::RenderFrameHost& render_frame_host,
      blink::AssociatedInterfaceRegistry& associated_registry) override;
  void RegisterAssociatedInterfaceBindersForServiceWorker(
      const content::ServiceWorkerVersionBaseInfo& service_worker_version_info,
      blink::AssociatedInterfaceRegistry& associated_registry) override;
  void RegisterBrowserInterfaceBindersForFrame(
      content::RenderFrameHost* render_frame_host,
      mojo::BinderMapWithContext<content::RenderFrameHost*>* map) override;
  mojo::PendingRemote<network::mojom::URLLoaderFactory>
  CreateNonNetworkNavigationURLLoaderFactory(
      const std::string& scheme,
      content::FrameTreeNodeId frame_tree_node_id) override;
  void RegisterNonNetworkWorkerMainResourceURLLoaderFactories(
      content::BrowserContext* browser_context,
      const std::optional<url::Origin>& request_initiator,
      network::mojom::RequestDestination request_destination,
      NonNetworkURLLoaderFactoryMap* factories) override;
  void RegisterNonNetworkServiceWorkerUpdateURLLoaderFactories(
      content::BrowserContext* browser_context,
      NonNetworkURLLoaderFactoryMap* factories) override;
  void RegisterNonNetworkSubresourceURLLoaderFactories(
      int render_process_id,
      int render_frame_id,
      const std::optional<url::Origin>& request_initiator_origin,
      NonNetworkURLLoaderFactoryMap* factories) override;
  void WillCreateURLLoaderFactory(
      content::BrowserContext* browser_context,
      content::RenderFrameHost* frame,
      int render_process_id,
      URLLoaderFactoryType type,
      const url::Origin& request_initiator,
      const net::IsolationInfo& isolation_info,
      std::optional<int64_t> navigation_id,
      ukm::SourceIdObj ukm_source_id,
      network::URLLoaderFactoryBuilder& factory_builder,
      mojo::PendingRemote<network::mojom::TrustedURLLoaderHeaderClient>*
          header_client,
      bool* bypass_redirect_checks,
      bool* disable_secure_dns,
      network::mojom::URLLoaderFactoryOverridePtr* factory_override,
      scoped_refptr<base::SequencedTaskRunner> navigation_response_task_runner,
      bool is_for_network_service) override;
  void OverrideURLLoaderFactoryParams(
      content::BrowserContext* browser_context,
      const url::Origin& origin,
      bool is_for_isolated_world,
      bool is_for_service_worker,
      network::mojom::URLLoaderFactoryParams* factory_params) override;
};
#endif


}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_

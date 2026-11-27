#ifndef PROJECT_B_ENGINE_BROWSER_PERMISSIONS_H_
#define PROJECT_B_ENGINE_BROWSER_PERMISSIONS_H_

#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>

#include "base/functional/callback.h"
#include "content/public/browser/permission_controller_delegate.h"
#include "content/public/browser/web_contents_delegate.h"

namespace project_b {

// What a page may use (docs/daily-driver.md): the adapter between content's permissions
// and Go's. Every request and every read goes to Go, by the kind's name and our statuses;
// Go decides, and answers a request later, with Answer. Content's types stop here.
class Permissions : public content::PermissionControllerDelegate {
 public:
  Permissions();
  Permissions(const Permissions&) = delete;
  Permissions& operator=(const Permissions&) = delete;
  ~Permissions() override;

  // Go's answer to a request: a PBPermissionStatus for each of its kinds. A request
  // answered already is ignored.
  static void Answer(uint32_t request, const int32_t* statuses, size_t count);

  // A page's download about to start (docs/video-streams.md), asked of Go as a request of
  // our own kind, "automatic-downloads", from the origin of contents' page: clicked says a
  // click or a key reached the page since its last download. allowed hears whether it goes;
  // Go may ask the person first.
  static void AskDownload(content::WebContents* contents,
                          bool clicked,
                          base::OnceCallback<void(bool)> allowed);

  // Whether a page may see its devices' names (enumerateDevices): as it may use them.
  static bool CheckMediaAccess(content::RenderFrameHost* frame,
                               const url::Origin& origin,
                               blink::mojom::MediaStreamType type);

  // getUserMedia's camera and microphone: asks this delegate, as any request does, and
  // gives the page the devices it was allowed, the ones it named or the system's first.
  static void RequestMediaAccess(content::WebContents* contents,
                                 const content::MediaStreamRequest& request,
                                 content::MediaResponseCallback callback);

  void RequestPermissionsFromCurrentDocument(
      content::RenderFrameHost* render_frame_host,
      const content::PermissionRequestDescription& request_description,
      base::OnceCallback<void(const std::vector<content::PermissionResult>&)>
          callback) override;
  blink::mojom::PermissionStatus GetPermissionStatus(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      const GURL& requesting_origin,
      const GURL& embedding_origin) override;
  content::PermissionResult GetPermissionResultForOriginWithoutContext(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      const url::Origin& requesting_origin,
      const url::Origin& embedding_origin) override;
  content::PermissionResult GetPermissionResultForCurrentDocument(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      content::RenderFrameHost* render_frame_host,
      bool should_include_device_status) override;
  content::PermissionResult GetPermissionResultForWorker(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      content::RenderProcessHost* render_process_host,
      const GURL& worker_origin) override;
  content::PermissionResult GetPermissionResultForEmbeddedRequester(
      const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
      content::RenderFrameHost* render_frame_host,
      const url::Origin& overridden_origin) override;
  void ResetPermission(blink::PermissionType permission,
                       const GURL& requesting_origin,
                       const GURL& embedding_origin) override;

 private:
  // A kind's status for origin in tab's page, 0 for none, as Go says it.
  static blink::mojom::PermissionStatus StatusOf(
      const blink::mojom::PermissionDescriptorPtr& descriptor,
      int64_t tab,
      const url::Origin& origin);
  static blink::mojom::PermissionStatus StatusOf(const std::string& kind,
                                                 int64_t tab,
                                                 const url::Origin& origin);
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PERMISSIONS_H_

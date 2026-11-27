#ifndef PROJECT_B_ENGINE_COMMON_RENDERER_CLIENT_H_
#define PROJECT_B_ENGINE_COMMON_RENDERER_CLIENT_H_

#include <memory>

#include "content/public/renderer/content_renderer_client.h"

namespace project_b {

// What every renderer of ours does: refuse a page's requests to a blocked host
// (block_rules.h) and, unless it is off for the page, those the blocker refuses
// (blocker_client.h), whether a frame or a worker makes them; put in what the blocker hides
// and runs in a frame's page, and its watcher; and run the form script and the person's
// sparks in a frame's page. A build with extensions derives its renderer client from this
// one, and calls its overrides.
class RendererClient : public content::ContentRendererClient {
 public:
  RendererClient();
  RendererClient(const RendererClient&) = delete;
  RendererClient& operator=(const RendererClient&) = delete;
  ~RendererClient() override;

  void RenderThreadStarted() override;
  void RenderFrameCreated(content::RenderFrame* render_frame) override;
  void RunScriptsAtDocumentStart(content::RenderFrame* render_frame) override;
  void RunScriptsAtDocumentEnd(content::RenderFrame* render_frame) override;

  std::unique_ptr<blink::URLLoaderThrottleProvider>
  CreateURLLoaderThrottleProvider(
      blink::URLLoaderThrottleProviderType provider_type) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_RENDERER_CLIENT_H_

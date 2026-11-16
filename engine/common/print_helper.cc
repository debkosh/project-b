#include "project_b/engine/common/print_helper.h"

#include <memory>

#include "base/memory/raw_ptr.h"
#include "components/printing/renderer/print_render_frame_helper.h"
#include "content/public/renderer/render_frame.h"
#include "mojo/public/cpp/bindings/associated_remote.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_provider.h"
#include "third_party/blink/public/web/web_element.h"

namespace project_b {
namespace printing {

namespace {

// Chromium's helper asks its embedder these. There is no print preview of ours: the
// system's panel is the preview, so a page's window.print() goes to the browser, which
// prints the frame the way Print… does.
class Delegate : public ::printing::PrintRenderFrameHelper::Delegate {
 public:
  explicit Delegate(content::RenderFrame* frame) : frame_(frame) {}

  blink::WebElement GetPdfElement(blink::WebLocalFrame* frame) override {
    return blink::WebElement();
  }

  bool IsPrintPreviewEnabled() override { return false; }

  bool OverridePrint(blink::WebLocalFrame* frame) override {
    if (!host_) {
      frame_->GetRemoteAssociatedInterfaces()->GetInterface(&host_);
    }
    host_->Print();
    return true;
  }

 private:
  // The frame this helper is for, which outlives it.
  const raw_ptr<content::RenderFrame> frame_;
  mojo::AssociatedRemote<mojom::FrameHost> host_;
};

}  // namespace

void Attach(content::RenderFrame* frame) {
  // The helper is the frame's observer, and goes with it.
  new ::printing::PrintRenderFrameHelper(frame, std::make_unique<Delegate>(frame));
}

}  // namespace printing
}  // namespace project_b

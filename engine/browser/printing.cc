#include "project_b/engine/browser/printing.h"

#include <optional>
#include <string>
#include <utility>
#include <variant>

#include "base/functional/bind.h"
#include "base/logging.h"
#include "base/no_destructor.h"
#include "components/embedder_support/user_agent_utils.h"
#include "components/printing/browser/headless/headless_print_manager.h"
#include "components/printing/browser/print_manager_utils.h"
#include "components/printing/browser/print_to_pdf/pdf_print_result.h"
#include "components/printing/browser/print_to_pdf/pdf_print_utils.h"
#include "content/public/browser/global_routing_id.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/web_contents.h"
#include "mojo/public/cpp/bindings/associated_receiver_set.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "third_party/blink/public/common/associated_interfaces/associated_interface_registry.h"

namespace project_b {
namespace printing {

namespace {

// Renders frame to a PDF and shows the system's panel with it. Margins are Chromium's
// default; no header or footer, and no backgrounds, as Chrome prints by default.
void PrintFrame(content::WebContents* contents, content::RenderFrameHost* frame) {
  auto* manager = headless::HeadlessPrintManager::FromWebContents(contents);
  if (!manager || !frame->IsRenderFrameLive()) {
    return;
  }
  const gfx::SizeF paper = PaperInches();
  auto params = print_to_pdf::GetPrintPagesParams(
      frame->GetLastCommittedURL(), /*landscape=*/std::nullopt,
      /*display_header_footer=*/false, /*print_background=*/false,
      /*scale=*/std::nullopt, paper.width(), paper.height(),
      /*margin_top=*/std::nullopt, /*margin_bottom=*/std::nullopt,
      /*margin_left=*/std::nullopt, /*margin_right=*/std::nullopt,
      /*header_template=*/std::nullopt, /*footer_template=*/std::nullopt,
      /*prefer_css_page_size=*/true, /*generate_tagged_pdf=*/std::nullopt,
      /*generate_document_outline=*/std::nullopt);
  if (auto* error = std::get_if<std::string>(&params)) {
    LOG(ERROR) << "print: " << *error;
    return;
  }
  manager->PrintToPdf(
      frame, /*page_ranges=*/"",
      std::move(std::get<::printing::mojom::PrintPagesParamsPtr>(params)),
      base::BindOnce(
          [](base::WeakPtr<content::WebContents> contents,
             print_to_pdf::PdfPrintResult result,
             scoped_refptr<base::RefCountedMemory> pdf) {
            if (result != print_to_pdf::PdfPrintResult::kPrintSuccess) {
              LOG(ERROR) << "print: "
                         << print_to_pdf::PdfPrintResultToString(result);
              return;
            }
            if (contents) {
              ShowPanel(contents.get(), contents->GetTitle(), std::move(pdf));
            }
          },
          contents->GetWeakPtr()));
}

// Each frame's window.print(), by the frame that asked.
class Hosts : public mojom::FrameHost {
 public:
  void Bind(content::GlobalRenderFrameHostId frame,
            mojo::PendingAssociatedReceiver<mojom::FrameHost> receiver) {
    receivers_.Add(this, std::move(receiver), frame);
  }

  void Print() override {
    content::RenderFrameHost* frame =
        content::RenderFrameHost::FromID(receivers_.current_context());
    content::WebContents* contents =
        frame ? content::WebContents::FromRenderFrameHost(frame) : nullptr;
    const int64_t tab = contents ? hooks::TabOf(contents) : 0;
    if (tab <= 0) {
      return;
    }
    hooks::Args args;
    args.Int(tab);
    PBValue yes;
    if (hooks::Ask(PB_EVENT_PRINT_REQUESTED, args, &yes) && yes.i) {
      PrintFrame(contents, frame);
    }
  }

 private:
  mojo::AssociatedReceiverSet<mojom::FrameHost, content::GlobalRenderFrameHostId>
      receivers_;
};

Hosts& TheHosts() {
  static base::NoDestructor<Hosts> hosts;
  return *hosts;
}

}  // namespace

void Follow(content::WebContents* contents) {
  headless::HeadlessPrintManager::CreateForWebContents(contents);
  // Where frames of other sites have processes of their own, the compositor puts the
  // page together from them.
  ::printing::CreateCompositeClientIfNeeded(contents,
                                            embedder_support::GetUserAgent());
}

void BindFrame(content::RenderFrameHost& frame,
               blink::AssociatedInterfaceRegistry& registry) {
  registry.AddInterface<::printing::mojom::PrintManagerHost>(base::BindRepeating(
      [](content::RenderFrameHost* frame,
         mojo::PendingAssociatedReceiver<::printing::mojom::PrintManagerHost>
             receiver) {
        headless::HeadlessPrintManager::BindPrintManagerHost(std::move(receiver),
                                                             frame);
      },
      &frame));
  registry.AddInterface<mojom::FrameHost>(base::BindRepeating(
      [](content::GlobalRenderFrameHostId frame,
         mojo::PendingAssociatedReceiver<mojom::FrameHost> receiver) {
        TheHosts().Bind(frame, std::move(receiver));
      },
      frame.GetGlobalId()));
}

void Print(content::WebContents* contents) {
  PrintFrame(contents, contents->GetPrimaryMainFrame());
}

}  // namespace printing
}  // namespace project_b

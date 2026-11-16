#include "project_b/engine/common/utility_client.h"

#include <memory>

#include "components/services/print_compositor/print_compositor_impl.h"
#include "components/services/print_compositor/public/mojom/print_compositor.mojom.h"
#include "content/public/utility/utility_thread.h"
#include "mojo/public/cpp/bindings/pending_receiver.h"
#include "mojo/public/cpp/bindings/service_factory.h"

namespace project_b {

namespace {

auto RunPrintCompositor(
    mojo::PendingReceiver<printing::mojom::PrintCompositor> receiver) {
  return std::make_unique<printing::PrintCompositorImpl>(
      std::move(receiver), /*initialize_environment=*/true,
      content::UtilityThread::Get()->GetIOTaskRunner());
}

}  // namespace

UtilityClient::UtilityClient() = default;
UtilityClient::~UtilityClient() = default;

void UtilityClient::RegisterMainThreadServices(mojo::ServiceFactory& services) {
  services.Add(RunPrintCompositor);
}

}  // namespace project_b

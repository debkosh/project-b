#ifndef PROJECT_B_ENGINE_COMMON_UTILITY_CLIENT_H_
#define PROJECT_B_ENGINE_COMMON_UTILITY_CLIENT_H_

#include "content/public/utility/content_utility_client.h"

namespace project_b {

// The services of ours a utility process runs: the print compositor, which puts a page's
// frames from their own processes together into one PDF as it prints (docs/daily-driver.md,
// Printing). A build with extensions adds its own to these.
class UtilityClient : public content::ContentUtilityClient {
 public:
  UtilityClient();
  UtilityClient(const UtilityClient&) = delete;
  UtilityClient& operator=(const UtilityClient&) = delete;
  ~UtilityClient() override;

  void RegisterMainThreadServices(mojo::ServiceFactory& services) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_UTILITY_CLIENT_H_

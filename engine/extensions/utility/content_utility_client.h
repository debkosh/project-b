#ifndef PROJECT_B_ENGINE_EXTENSIONS_UTILITY_CONTENT_UTILITY_CLIENT_H_
#define PROJECT_B_ENGINE_EXTENSIONS_UTILITY_CONTENT_UTILITY_CLIENT_H_

#include "project_b/engine/common/utility_client.h"

namespace project_b {

// What a utility process runs with extensions: ours, and unzipping, which unpacking an
// extension package asks for in a sandboxed process.
class ContentUtilityClient : public UtilityClient {
 public:
  ContentUtilityClient();
  ContentUtilityClient(const ContentUtilityClient&) = delete;
  ContentUtilityClient& operator=(const ContentUtilityClient&) = delete;
  ~ContentUtilityClient() override;

 private:
  void RegisterMainThreadServices(mojo::ServiceFactory& services) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_UTILITY_CONTENT_UTILITY_CLIENT_H_

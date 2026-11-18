// What a workspace's storage holds, by host, for Settings' Sites (docs/settings.md): the
// cookies each domain set, and how many bytes each host keeps in quota-managed storage
// (IndexedDB, the Cache API, file systems, service workers) and local storage. Go hears
// it in PB_EVENT_SITE_DATA and groups it by site. Shared by the Mac and Android.

#ifndef PROJECT_B_ENGINE_BROWSER_SITE_DATA_H_
#define PROJECT_B_ENGINE_BROWSER_SITE_DATA_H_

#include <string>

namespace content {
class StoragePartition;
}

namespace project_b {
namespace site_data {

// Counts what the partition holds and fires PB_EVENT_SITE_DATA with the workspace's id
// once it has it all. Main thread.
void List(content::StoragePartition* partition, const std::string& workspace);

}  // namespace site_data
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_SITE_DATA_H_

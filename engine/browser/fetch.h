#ifndef PROJECT_B_ENGINE_BROWSER_FETCH_H_
#define PROJECT_B_ENGINE_BROWSER_FETCH_H_

#include <cstdint>
#include <string>

#include "base/files/file_path.h"
#include "url/gurl.h"

namespace content {
class StoragePartition;
}  // namespace content

namespace project_b {

// Fetches an address into a file as a page would, for Go (pb_abi.h fetch,
// docs/video-streams.md): through partition's own network, so Chromium decides which of its
// cookies go, with page's origin as the request's initiator, its site as the site for
// cookies and its address as the referrer; with no page, as the person's own request of the
// address. Only http and https. from is a PBAddressSpace, where the request is made from:
// other than PB_SPACE_ANY, the request carries a client security state that makes the
// network service refuse a server on a more private address, as Chromium's local network
// access does for a page. range is "first-last" in bytes, or "" for all of it.
// PB_EVENT_FETCHED says how it ended, always, unless CancelFetch came first.
void Fetch(int64_t id,
           content::StoragePartition* partition,
           const GURL& page,
           const GURL& url,
           const base::FilePath& path,
           const std::string& range,
           int32_t from);

// Stops a fetch and deletes what it wrote; nothing is told of it after.
void CancelFetch(int64_t id);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_FETCH_H_

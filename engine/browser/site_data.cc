#include "project_b/engine/browser/site_data.h"

#include <cstdint>
#include <map>
#include <memory>
#include <utility>
#include <vector>

#include "base/barrier_closure.h"
#include "base/functional/bind.h"
#include "base/json/string_escape.h"
#include "base/memory/scoped_refptr.h"
#include "base/strings/string_number_conversions.h"
#include "base/task/bind_post_task.h"
#include "components/services/storage/public/mojom/local_storage_control.mojom.h"
#include "components/services/storage/public/mojom/storage_usage_info.mojom.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/storage_partition.h"
#include "net/cookies/canonical_cookie.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "services/network/public/mojom/cookie_manager.mojom.h"
#include "storage/browser/quota/quota_manager.h"
#include "url/url_constants.h"

namespace project_b {
namespace site_data {
namespace {

struct Host {
  int64_t cookies = 0;
  int64_t bytes = 0;
};

// What the three counts have found so far, on the main thread.
struct Tally {
  std::string workspace;
  std::map<std::string, Host> hosts;
};

// Sends the tally to Go: s: the workspace, s: json, [{"host", "cookies", "bytes"}]. Written
// here, as base's writer has only doubles for numbers past an int and writes them "7.0".
void Report(std::shared_ptr<Tally> tally) {
  std::string json = "[";
  for (const auto& [host, h] : tally->hosts) {
    if (host.empty()) {
      continue;
    }
    if (json.size() > 1) {
      json += ",";
    }
    json += "{\"host\":";
    base::EscapeJSONString(host, /*put_in_quotes=*/true, &json);
    json += ",\"cookies\":" + base::NumberToString(h.cookies) +
            ",\"bytes\":" + base::NumberToString(h.bytes) + "}";
  }
  json += "]";
  hooks::Args args;
  args.Str(tally->workspace);
  args.Str(std::move(json));
  hooks::Fire(PB_EVENT_SITE_DATA, args);
}

// A web page's storage: an extension's own isn't a site's.
bool IsWeb(const url::Origin& origin) {
  return origin.scheme() == url::kHttpsScheme ||
         origin.scheme() == url::kHttpScheme;
}

}  // namespace

void List(content::StoragePartition* partition, const std::string& workspace) {
  DCHECK_CURRENTLY_ON(content::BrowserThread::UI);
  auto tally = std::make_shared<Tally>();
  tally->workspace = workspace;
  base::RepeatingClosure done =
      base::BarrierClosure(3, base::BindOnce(&Report, tally));

  partition->GetCookieManagerForBrowserProcess()->GetAllCookies(base::BindOnce(
      [](std::shared_ptr<Tally> tally, base::RepeatingClosure done,
         const std::vector<net::CanonicalCookie>& cookies) {
        for (const net::CanonicalCookie& c : cookies) {
          tally->hosts[c.DomainWithoutDot()].cookies++;
        }
        done.Run();
      },
      tally, done));

  partition->GetLocalStorageControl()->GetUsage(base::BindOnce(
      [](std::shared_ptr<Tally> tally, base::RepeatingClosure done,
         std::vector<storage::mojom::StorageUsageInfoPtr> usage) {
        for (const auto& u : usage) {
          const url::Origin& origin = u->storage_key.origin();
          if (IsWeb(origin)) {
            tally->hosts[origin.host()].bytes += u->total_size_bytes;
          }
        }
        done.Run();
      },
      tally, done));

  // The quota manager lives on the IO thread, and answers by host alone: an extension's
  // is its id, which has no dot, and Go leaves it out.
  scoped_refptr<storage::QuotaManager> quota = partition->GetQuotaManager();
  content::GetIOThreadTaskRunner({})->PostTask(
      FROM_HERE,
      base::BindOnce(
          &storage::QuotaManager::GetUsageInfo, quota,
          base::BindPostTask(
              content::GetUIThreadTaskRunner({}),
              base::BindOnce(
                  [](std::shared_ptr<Tally> tally, base::RepeatingClosure done,
                     storage::UsageInfoEntries entries) {
                    for (const storage::UsageInfo& e : entries) {
                      tally->hosts[e.host].bytes += e.usage;
                    }
                    done.Run();
                  },
                  tally, done))));
}

}  // namespace site_data
}  // namespace project_b

#include "project_b/engine/browser/downloads.h"

#include <memory>
#include <optional>
#include <utility>

#include "base/check.h"
#include "base/functional/bind.h"
#include "base/json/json_reader.h"
#include "base/task/thread_pool.h"
#include "base/values.h"
#include "components/download/public/common/download_interrupt_reasons.h"
#include "components/download/public/common/download_item_impl.h"
#include "components/download/public/common/download_target_info.h"
#include "components/download/public/common/download_url_parameters.h"
#include "content/public/browser/download_item_utils.h"
#include "content/public/browser/download_request_utils.h"
#include "content/public/browser/web_contents.h"
#include "net/base/filename_util.h"
#include "net/traffic_annotation/network_traffic_annotation.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"

namespace project_b {

namespace {

Downloads* g_downloads = nullptr;

// A running download that receives nothing for this long has a connection that died
// without saying so, as one does across the Mac's sleep: it is interrupted as a network
// timeout, which content resumes at once on a new connection from where the file got to.
// The time stops while the Mac sleeps, so it counts from the wake.
constexpr base::TimeDelta kStalled = base::Seconds(30);
constexpr base::TimeDelta kStallCheck = base::Seconds(5);

// The file name content would give the download, from its headers and address.
std::string NameOf(const GURL& url,
                   const std::string& content_disposition,
                   const std::string& suggested,
                   const std::string& mime_type) {
  return net::GenerateFileName(url, content_disposition, std::string(), suggested,
                               mime_type, "download")
      .AsUTF8Unsafe();
}

int32_t StateOf(const download::DownloadItem* item) {
  switch (item->GetState()) {
    case download::DownloadItem::COMPLETE:
      return PB_DOWNLOAD_DONE;
    case download::DownloadItem::CANCELLED:
      return PB_DOWNLOAD_CANCELLED;
    case download::DownloadItem::INTERRUPTED:
      return PB_DOWNLOAD_FAILED;
    default:
      return PB_DOWNLOAD_PROGRESS;
  }
}

}  // namespace

Downloads::Downloads() {
  g_downloads = this;
}

void Downloads::SetDownloadManager(content::DownloadManager* manager) {
  manager_ = manager;
  manager_->AddObserver(this);
}

Downloads::~Downloads() {
  if (manager_) {
    manager_->RemoveObserver(this);
  }
  if (g_downloads == this) {
    g_downloads = nullptr;
  }
}

// static
Downloads* Downloads::Get() {
  return g_downloads;
}

void Downloads::Shutdown() {
  weak_factory_.InvalidateWeakPtrs();
}

void Downloads::OnDownloadDestroyed(download::DownloadItem* item) {
  heard_.erase(item->GetId());
  item->RemoveObserver(this);
}

void Downloads::ManagerGoingDown(content::DownloadManager* manager) {
  manager_->RemoveObserver(this);
  manager_ = nullptr;
}

void Downloads::GetNextId(content::DownloadIdCallback callback) {
  std::move(callback).Run(next_id_++);
}

void Downloads::OnDownloadCreated(content::DownloadManager* manager,
                                  download::DownloadItem* item) {
  item->AddObserver(this);
}

void Downloads::OnDownloadUpdated(download::DownloadItem* item) {
  Watch(item);
  // A finished file is handed on first, so what Go does on hearing it is done, such as a
  // phone's notification that opens it, comes after the copy it opens.
  if (item->IsDone()) {
    item->RemoveObserver(this);
    if (item->GetState() == download::DownloadItem::COMPLETE) {
      PublishFile(item->GetTargetFilePath(), item->GetMimeType());
    }
  }
  // Nothing until Go has placed it: its path is what the list shows.
  if (!item->GetTargetFilePath().empty()) {
    Report(item);
  }
}

void Downloads::Watch(download::DownloadItem* item) {
  const uint32_t id = item->GetId();
  if (item->GetState() != download::DownloadItem::IN_PROGRESS || item->IsPaused() ||
      item->GetTargetFilePath().empty()) {
    heard_.erase(id);
  } else {
    auto it = heard_.find(id);
    if (it == heard_.end() || it->second.received != item->GetReceivedBytes()) {
      heard_[id] = {item->GetReceivedBytes(), base::TimeTicks::Now()};
    }
  }
  if (heard_.empty()) {
    stall_timer_.Stop();
  } else if (!stall_timer_.IsRunning()) {
    stall_timer_.Start(FROM_HERE, kStallCheck, this, &Downloads::CheckStalls);
  }
}

void Downloads::CheckStalls() {
  const base::TimeTicks now = base::TimeTicks::Now();
  std::vector<uint32_t> stalled;
  for (const auto& [id, heard] : heard_) {
    if (now - heard.at >= kStalled) {
      stalled.push_back(id);
    }
  }
  for (uint32_t id : stalled) {
    heard_.erase(id);
    download::DownloadItem* item = manager_ ? manager_->GetDownload(id) : nullptr;
    if (!item || item->GetState() != download::DownloadItem::IN_PROGRESS || item->IsPaused()) {
      continue;
    }
    // Content's items are all DownloadItemImpl; this is the error its file would report.
    static_cast<download::DownloadItemImpl*>(item)->DestinationError(
        download::DOWNLOAD_INTERRUPT_REASON_NETWORK_TIMEOUT, item->GetReceivedBytes(),
        std::nullopt);
  }
  if (heard_.empty()) {
    stall_timer_.Stop();
  }
}

void Downloads::Report(download::DownloadItem* item) {
  hooks::Args args;
  args.Int(item->GetId());
  args.Str(item->GetTargetFilePath().AsUTF8Unsafe());
  args.Str(item->GetURL().spec());
  args.Str(item->GetMimeType());
  args.Int(StateOf(item));
  args.Int(item->GetReceivedBytes());
  args.Int(item->GetTotalBytes());
  args.Bool(item->IsPaused());
  hooks::Fire(PB_EVENT_DOWNLOAD_UPDATED, args);
}

bool Downloads::DetermineDownloadTarget(download::DownloadItem* item,
                                        download::DownloadTargetCallback* callback) {
  if (!item->GetForcedFilePath().empty()) {
    download::DownloadTargetInfo target;
    target.target_path = item->GetForcedFilePath();
    target.intermediate_path = item->GetForcedFilePath();
    std::move(*callback).Run(std::move(target));
    return true;
  }
  content::WebContents* contents = content::DownloadItemUtils::GetWebContents(item);
  hooks::Args args;
  args.Int(item->GetId());
  args.Int(contents ? hooks::TabOf(contents) : 0);
  args.Str(item->GetURL().spec());
  args.Str(NameOf(item->GetURL(), item->GetContentDisposition(),
                  item->GetSuggestedFilename(), item->GetMimeType()));
  args.Str(item->GetMimeType());
  args.Bool(item->GetTargetDisposition() ==
            download::DownloadItem::TARGET_DISPOSITION_PROMPT);
  // Go's answer is {"path", "prompt"}; none, or no path, refuses the download.
  PBValue result;
  base::FilePath path;
  bool prompt = false;
  if (hooks::Ask(PB_EVENT_DOWNLOAD_TARGET, args, &result)) {
    std::optional<base::Value> answer =
        base::JSONReader::Read(hooks::TakeString(result), 0);
    if (answer && answer->is_dict()) {
      if (const std::string* p = answer->GetDict().FindString("path")) {
        path = base::FilePath::FromUTF8Unsafe(*p);
      }
      prompt = answer->GetDict().FindBool("prompt").value_or(false);
    }
  }
  auto targeted = base::BindOnce(&Downloads::Targeted, weak_factory_.GetWeakPtr(),
                                 item->GetId(), std::move(*callback));
  if (prompt && !path.empty() && contents) {
    project_b::ChooseSavePath(contents, path, std::move(targeted));
  } else {
    std::move(targeted).Run(path);
  }
  return true;
}

void Downloads::Targeted(uint32_t id,
                         download::DownloadTargetCallback callback,
                         const base::FilePath& path) {
  download::DownloadTargetInfo target;
  if (path.empty()) {
    // An empty target cancels it.
    target.interrupt_reason = download::DOWNLOAD_INTERRUPT_REASON_USER_CANCELED;
  } else {
    target.target_path = path;
    target.intermediate_path = path.AddExtension(FILE_PATH_LITERAL("crdownload"));
    target.target_disposition = download::DownloadItem::TARGET_DISPOSITION_OVERWRITE;
  }
  std::move(callback).Run(std::move(target));
}

void Downloads::DownloadURL(content::WebContents* contents, const GURL& url, bool save_as) {
  if (!manager_) {
    return;
  }
  std::unique_ptr<download::DownloadUrlParameters> params =
      contents ? content::DownloadRequestUtils::CreateDownloadForWebContentsMainFrame(
                     contents, url, NO_TRAFFIC_ANNOTATION_YET)
               : std::make_unique<download::DownloadUrlParameters>(
                     url, NO_TRAFFIC_ANNOTATION_YET);
  params->set_prompt(save_as);
  manager_->DownloadUrl(std::move(params));
}

void Downloads::Command(uint32_t id, int32_t command) {
  download::DownloadItem* item = manager_ ? manager_->GetDownload(id) : nullptr;
  if (!item) {
    return;
  }
  switch (command) {
    case PB_DOWNLOAD_CANCEL:
      item->Cancel(/*user_cancel=*/true);
      break;
    case PB_DOWNLOAD_PAUSE:
      item->Pause();
      break;
    case PB_DOWNLOAD_RESUME:
      // Retry: an interrupted download content can't pick up again starts over, as a
      // new download of its address, and this one goes.
      if (item->GetState() == download::DownloadItem::INTERRUPTED && !item->CanResume()) {
        const GURL url = item->GetURL();
        item->RemoveObserver(this);
        item->Remove();
        DownloadURL(nullptr, url, false);
        break;
      }
      item->Resume(/*user_resume=*/true);
      break;
    case PB_DOWNLOAD_REMOVE:
      item->RemoveObserver(this);
      item->Remove();
      break;
  }
}

}  // namespace project_b

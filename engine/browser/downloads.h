#ifndef PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_
#define PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_

#include <cstdint>
#include <map>
#include <optional>
#include <string>

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "base/memory/weak_ptr.h"
#include "base/time/time.h"
#include "base/timer/timer.h"
#include "build/build_config.h"
#include "components/download/public/common/download_item.h"
#include "content/public/common/referrer.h"
#include "content/public/browser/download_manager.h"
#include "content/public/browser/download_manager_delegate.h"
#include "content/public/browser/web_contents.h"
#include "ui/base/page_transition_types.h"
#include "url/gurl.h"
#include "url/origin.h"

namespace project_b {

// A profile's downloads (docs/daily-driver.md). Go says where each goes, or refuses it,
// and hears every change to each; the Save panel is the system's own. The engine keeps
// no list: Go does.
class Downloads : public content::DownloadManagerDelegate,
                  public content::DownloadManager::Observer,
                  public download::DownloadItem::Observer {
 public:
  Downloads();
  Downloads(const Downloads&) = delete;
  Downloads& operator=(const Downloads&) = delete;
  ~Downloads() override;

  // The profile's, or null before its context has made its download manager.
  static Downloads* Get();

  void SetDownloadManager(content::DownloadManager* manager);

  // Downloads url as tab's page would, showing the Save panel first when save_as; with
  // no page, as the browser itself, as Retry does for a download that can't resume. It
  // says it comes from referrer: a site may serve its files only to its own pages.
  void DownloadURL(content::WebContents* contents,
                   const GURL& url,
                   const content::Referrer& referrer,
                   bool save_as);
  // Runs a PBDownloadCommand on download id.
  void Command(uint32_t id, int32_t command);

  // content::DownloadManagerDelegate:
  void Shutdown() override;
  bool DetermineDownloadTarget(download::DownloadItem* item,
                               download::DownloadTargetCallback* callback) override;
  void GetNextId(content::DownloadIdCallback callback) override;
  // A page's download about to start: its tab is asked, WebContentsDelegate::CanDownload,
  // as Chrome asks its tab's limiter (docs/video-streams.md).
  void CheckDownloadAllowed(
      const content::WebContents::Getter& web_contents_getter,
      const GURL& url,
      const std::string& request_method,
      std::optional<url::Origin> request_initiator,
      bool from_download_cross_origin_redirect,
      bool content_initiated,
      const std::string& mime_type,
      std::optional<ui::PageTransition> page_transition,
      content::CheckDownloadAllowedCallback check_download_allowed_cb) override;

  // content::DownloadManager::Observer:
  void OnDownloadCreated(content::DownloadManager* manager,
                         download::DownloadItem* item) override;
  void ManagerGoingDown(content::DownloadManager* manager) override;

  // download::DownloadItem::Observer:
  void OnDownloadUpdated(download::DownloadItem* item) override;
  void OnDownloadDestroyed(download::DownloadItem* item) override;

 private:
  void Report(download::DownloadItem* item);
  void Targeted(uint32_t id,
                download::DownloadTargetCallback callback,
                const base::FilePath& path);
  // Watch keeps the time each running download last received anything; CheckStalls
  // interrupts the ones silent too long.
  void Watch(download::DownloadItem* item);
  void CheckStalls();

  struct Heard {
    int64_t received;
    base::TimeTicks at;
  };
  std::map<uint32_t, Heard> heard_;
  base::RepeatingTimer stall_timer_;

  raw_ptr<content::DownloadManager> manager_;
  uint32_t next_id_ = download::DownloadItem::kInvalidId + 1;
  base::WeakPtrFactory<Downloads> weak_factory_{this};
};

// What the platform does: the Save dialog over the page's window, which answers with the
// path chosen or an empty one: Chromium's own on a desktop, none on a phone.
void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen);
// Hands a finished download, or a file Go kept as one, to the system where other apps find
// it: on a phone, a copy in the shared Downloads, through which it is then opened. Nothing on
// a desktop, where it already is in the user's Downloads folder.
void PublishFile(const base::FilePath& path, const std::string& mime_type);
#if BUILDFLAG(IS_ANDROID)
// Deletes a finished download's copy in a phone's shared Downloads, by the app's path for
// it; false when there is none. Android only.
bool DeleteDownload(const base::FilePath& path);
// Shows how a download stands in the phone's notification for it (pb_abi.h show_download).
void ShowDownload(int64_t id,
                  const std::string& name,
                  const base::FilePath& path,
                  int32_t state,
                  int64_t received,
                  int64_t total,
                  bool paused);
#endif

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_

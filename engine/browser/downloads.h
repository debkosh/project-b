#ifndef PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_
#define PROJECT_B_ENGINE_BROWSER_DOWNLOADS_H_

#include <cstdint>
#include <map>
#include <string>

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "base/memory/weak_ptr.h"
#include "base/time/time.h"
#include "base/timer/timer.h"
#include "build/build_config.h"
#include "components/download/public/common/download_item.h"
#include "content/public/browser/download_manager.h"
#include "content/public/browser/download_manager_delegate.h"
#include "url/gurl.h"

namespace content {
class WebContents;
}  // namespace content

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
  // no page, as the browser itself, as Retry does for a download that can't resume.
  void DownloadURL(content::WebContents* contents, const GURL& url, bool save_as);
  // Runs a PBDownloadCommand on download id.
  void Command(uint32_t id, int32_t command);

  // content::DownloadManagerDelegate:
  void Shutdown() override;
  bool DetermineDownloadTarget(download::DownloadItem* item,
                               download::DownloadTargetCallback* callback) override;
  void GetNextId(content::DownloadIdCallback callback) override;

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

// What the platform does: the Save panel over the page's window, which answers with the
// path chosen or an empty one; and opening a file, or showing it in the file manager.
void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen);
void OpenFile(const base::FilePath& path);
void RevealFile(const base::FilePath& path);
// Hands a finished download to the system where other apps find it: on a phone, a copy in
// the shared Downloads, through which it is then opened. Nothing on the Mac, where it already
// is in the user's Downloads folder.
void PublishFile(const base::FilePath& path, const std::string& mime_type);
// Puts a copy of a file of ours in the user's Downloads and shows it there: on the Mac in
// ~/Downloads under a name not yet taken, selected in Finder; on a phone in the shared
// Downloads, with a toast.
void SaveFile(const base::FilePath& path);
#if BUILDFLAG(IS_ANDROID)
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

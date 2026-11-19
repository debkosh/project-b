// The desktop's part of downloads: the Save dialog over the page's window, Chromium's own
// (choose_path.h); and nothing to publish a finished file to, as it is in the user's
// Downloads already.

#include <utility>

#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/choose_path.h"
#include "project_b/engine/browser/downloads.h"

namespace project_b {

void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen) {
  ShowSaveChooser(contents->GetTopLevelNativeWindow(), suggested, std::move(chosen));
}

void PublishFile(const base::FilePath& path, const std::string& mime_type) {}

}  // namespace project_b

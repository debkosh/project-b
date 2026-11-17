// The Mac's part of downloads: the Save panel as a sheet on the page's window, and Finder.

#import <AppKit/AppKit.h>

#include <utility>

#include "base/apple/foundation_util.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/functional/callback.h"
#include "base/strings/sys_string_conversions.h"
#include "base/task/thread_pool.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/downloads.h"
#include "ui/gfx/native_ui_types.h"

namespace project_b {

void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen) {
  NSSavePanel* panel = [NSSavePanel savePanel];
  panel.directoryURL = base::apple::FilePathToNSURL(suggested.DirName());
  panel.nameFieldStringValue = base::SysUTF8ToNSString(suggested.BaseName().AsUTF8Unsafe());
  panel.canCreateDirectories = YES;
  // The block may run once, and only once; the callback moves into it.
  __block base::OnceCallback<void(const base::FilePath&)> answer = std::move(chosen);
  void (^done)(NSModalResponse) = ^(NSModalResponse response) {
    base::FilePath path;
    if (response == NSModalResponseOK) {
      path = base::apple::NSURLToFilePath(panel.URL);
    }
    std::move(answer).Run(path);
  };
  NSWindow* window = contents->GetTopLevelNativeWindow().GetNativeNSWindow();
  if (window) {
    [panel beginSheetModalForWindow:window completionHandler:done];
  } else {
    [panel beginWithCompletionHandler:done];
  }
}

void OpenFile(const base::FilePath& path) {
  [NSWorkspace.sharedWorkspace openURL:base::apple::FilePathToNSURL(path)];
}

void PublishFile(const base::FilePath& path, const std::string& mime_type) {}

void RevealFile(const base::FilePath& path) {
  [NSWorkspace.sharedWorkspace activateFileViewerSelectingURLs:@[ base::apple::FilePathToNSURL(path) ]];
}

void SaveFile(const base::FilePath& path) {
  base::FilePath downloads;
  if (!base::apple::GetUserDirectory(NSDownloadsDirectory, &downloads)) {
    return;
  }
  // The copy off the main thread, which may not block; Finder once it is there.
  base::ThreadPool::PostTaskAndReplyWithResult(
      FROM_HERE, {base::MayBlock()},
      base::BindOnce(
          [](const base::FilePath& from, const base::FilePath& dir) {
            base::FilePath to = base::GetUniquePath(dir.Append(from.BaseName()));
            return !to.empty() && base::CopyFile(from, to) ? to : base::FilePath();
          },
          path, downloads),
      base::BindOnce([](const base::FilePath& to) {
        if (!to.empty()) {
          RevealFile(to);
        }
      }));
}

}  // namespace project_b

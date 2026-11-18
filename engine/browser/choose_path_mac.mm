#include "project_b/engine/browser/choose_path.h"

#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>

#include <utility>

#include "base/apple/foundation_util.h"
#include "base/no_destructor.h"
#include "project_b/engine/browser/window.h"

namespace project_b {

namespace {

base::FilePath& DriverChoice() {
  static base::NoDestructor<base::FilePath> choice;
  return *choice;
}

}  // namespace

void ChooseNextForDriver(const base::FilePath& path) {
  DriverChoice() = path;
}

void ShowPathChooser(Window* window,
                     PathKind kind,
                     base::OnceCallback<void(const base::FilePath&)> done) {
  if (!DriverChoice().empty()) {
    std::move(done).Run(std::exchange(DriverChoice(), base::FilePath()));
    return;
  }
  NSOpenPanel* panel = [NSOpenPanel openPanel];
  panel.allowsMultipleSelection = NO;
  panel.canChooseDirectories = kind == PathKind::kFolder;
  panel.canChooseFiles = kind == PathKind::kPackage;
  // A browser's folders are under Library, which the panel hides; it shows them.
  panel.showsHiddenFiles = kind == PathKind::kFolder;
  if (kind == PathKind::kPackage) {
    panel.allowedContentTypes = @[ [UTType typeWithFilenameExtension:@"crx"] ];
  }
  __block base::OnceCallback<void(const base::FilePath&)> answer =
      std::move(done);
  [panel beginSheetModalForWindow:window->native_window().GetNativeNSWindow()
                completionHandler:^(NSModalResponse response) {
                  std::move(answer).Run(
                      response == NSModalResponseOK
                          ? base::apple::NSURLToFilePath(panel.URL)
                          : base::FilePath());
                }];
}

}  // namespace project_b

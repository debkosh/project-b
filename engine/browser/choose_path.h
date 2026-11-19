#ifndef PROJECT_B_ENGINE_BROWSER_CHOOSE_PATH_H_
#define PROJECT_B_ENGINE_BROWSER_CHOOSE_PATH_H_

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "ui/gfx/native_ui_types.h"

namespace project_b {

// What ShowPathChooser lets the user pick. The numbers are PBPathKind's.
enum class PathKind { kFolder = 1, kPackage = 2 };

// Asks the user for a folder or an extension package, in Chromium's own file dialog
// (ui::SelectFileDialog), as a sheet on owner, opening at start, or where it last was when
// start is empty. done runs with the choice, or an empty path for none.
void ShowPathChooser(gfx::NativeWindow owner,
                     PathKind kind,
                     const base::FilePath& start,
                     base::OnceCallback<void(const base::FilePath&)> done);

// Asks the user where to save a file, in the same dialog, at suggested.
void ShowSaveChooser(gfx::NativeWindow owner,
                     const base::FilePath& suggested,
                     base::OnceCallback<void(const base::FilePath&)> done);

// The driver's choice: the next ShowPathChooser answers with path at once, in place of the
// dialog, as tests can't press a dialog's buttons.
void ChooseNextForDriver(const base::FilePath& path);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_CHOOSE_PATH_H_

#ifndef PROJECT_B_ENGINE_BROWSER_CHOOSE_PATH_H_
#define PROJECT_B_ENGINE_BROWSER_CHOOSE_PATH_H_

#include "base/files/file_path.h"
#include "base/functional/callback.h"

namespace project_b {

class Window;

// What ShowPathChooser lets the user pick. The numbers are PBPathKind's.
enum class PathKind { kFolder = 1, kPackage = 2 };

// Asks the user for a folder or an extension package, in the system's own panel on the
// window. done runs with the choice, or an empty path for none.
void ShowPathChooser(Window* window,
                     PathKind kind,
                     base::OnceCallback<void(const base::FilePath&)> done);

// The driver's choice: the next ShowPathChooser answers with path at once, in place of the
// system's panel, as tests can't press a panel's buttons.
void ChooseNextForDriver(const base::FilePath& path);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_CHOOSE_PATH_H_

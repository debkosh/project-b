// The desktop's folder, package and Save choosers, on Chromium's own file dialog, which each
// desktop has (docs/layers.md, 2b): no code of ours touches the OS for them.

#include "project_b/engine/browser/choose_path.h"

#include <memory>
#include <utility>

#include "base/files/file_path.h"
#include "base/memory/scoped_refptr.h"
#include "base/no_destructor.h"
#include "ui/shell_dialogs/select_file_dialog.h"
#include "ui/shell_dialogs/select_file_policy.h"
#include "ui/shell_dialogs/selected_file_info.h"

namespace project_b {

namespace {

base::FilePath& DriverChoice() {
  static base::NoDestructor<base::FilePath> choice;
  return *choice;
}

// One question to the dialog. It owns itself until the dialog answers.
class Chooser : public ui::SelectFileDialog::Listener {
 public:
  static void Show(gfx::NativeWindow owner,
                   ui::SelectFileDialog::Type type,
                   const base::FilePath& start,
                   std::unique_ptr<ui::SelectFileDialog::FileTypeInfo> types,
                   base::OnceCallback<void(const base::FilePath&)> done) {
    auto* chooser = new Chooser(std::move(types), std::move(done));
    chooser->dialog_ = ui::SelectFileDialog::Create(chooser, nullptr);
    if (!chooser->dialog_) {
      chooser->Answer(base::FilePath());
      return;
    }
    chooser->dialog_->SelectFile(type, std::u16string(), start, chooser->types_.get(),
                                 chooser->types_ ? 1 : 0, base::FilePath::StringType(), owner);
  }

  Chooser(const Chooser&) = delete;
  Chooser& operator=(const Chooser&) = delete;

 private:
  Chooser(std::unique_ptr<ui::SelectFileDialog::FileTypeInfo> types,
          base::OnceCallback<void(const base::FilePath&)> done)
      : types_(std::move(types)), done_(std::move(done)) {}

  ~Chooser() override {
    if (dialog_) {
      dialog_->ListenerDestroyed();
    }
  }

  // ui::SelectFileDialog::Listener:
  void FileSelected(const ui::SelectedFileInfo& file, int index) override {
    Answer(file.path());
  }
  void FileSelectionCanceled() override { Answer(base::FilePath()); }

  void Answer(const base::FilePath& path) {
    std::move(done_).Run(path);
    delete this;
  }

  std::unique_ptr<ui::SelectFileDialog::FileTypeInfo> types_;
  base::OnceCallback<void(const base::FilePath&)> done_;
  scoped_refptr<ui::SelectFileDialog> dialog_;
};

}  // namespace

void ChooseNextForDriver(const base::FilePath& path) {
  DriverChoice() = path;
}

void ShowPathChooser(gfx::NativeWindow owner,
                     PathKind kind,
                     const base::FilePath& start,
                     base::OnceCallback<void(const base::FilePath&)> done) {
  if (!DriverChoice().empty()) {
    std::move(done).Run(std::exchange(DriverChoice(), base::FilePath()));
    return;
  }
  if (kind == PathKind::kFolder) {
    Chooser::Show(owner, ui::SelectFileDialog::SELECT_FOLDER, start, nullptr,
                  std::move(done));
    return;
  }
  auto types = std::make_unique<ui::SelectFileDialog::FileTypeInfo>();
  types->extensions = {{FILE_PATH_LITERAL("crx")}};
  Chooser::Show(owner, ui::SelectFileDialog::SELECT_OPEN_FILE, start, std::move(types),
                std::move(done));
}

void ShowSaveChooser(gfx::NativeWindow owner,
                     const base::FilePath& suggested,
                     base::OnceCallback<void(const base::FilePath&)> done) {
  Chooser::Show(owner, ui::SelectFileDialog::SELECT_SAVEAS_FILE, suggested, nullptr,
                std::move(done));
}

}  // namespace project_b

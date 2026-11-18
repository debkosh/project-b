#include "project_b/engine/browser/file_chooser.h"

#include <optional>
#include <string>
#include <utility>

#include "base/json/json_reader.h"
#include "base/strings/string_util.h"
#include "base/strings/utf_string_conversions.h"
#include "base/values.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/web_contents.h"
#include "net/base/mime_util.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "ui/shell_dialogs/select_file_policy.h"
#include "ui/shell_dialogs/selected_file_info.h"

namespace project_b {

namespace {

using Mode = blink::mojom::FileChooserParams::Mode;

int32_t ModeOf(Mode mode) {
  switch (mode) {
    case Mode::kOpenMultiple:
      return 1;
    case Mode::kUploadFolder:
      return 2;
    case Mode::kSave:
      return 3;
    default:
      return 0;
  }
}

ui::SelectFileDialog::Type DialogOf(Mode mode) {
  switch (mode) {
    case Mode::kOpenMultiple:
      return ui::SelectFileDialog::SELECT_OPEN_MULTI_FILE;
    case Mode::kUploadFolder:
      return ui::SelectFileDialog::SELECT_UPLOAD_FOLDER;
    case Mode::kSave:
      return ui::SelectFileDialog::SELECT_SAVEAS_FILE;
    default:
      return ui::SelectFileDialog::SELECT_OPEN_FILE;
  }
}

// The extensions the page's accept list allows, as one group, or none for any file:
// ".pdf" as it is, and a type such as "image/*" through the types the system knows.
std::unique_ptr<ui::SelectFileDialog::FileTypeInfo> TypesOf(
    const std::vector<std::u16string>& accept) {
  auto types = std::make_unique<ui::SelectFileDialog::FileTypeInfo>();
  std::vector<base::FilePath::StringType> extensions;
  for (const std::u16string& type : accept) {
    if (type.empty()) {
      continue;
    }
    if (type[0] == '.') {
      extensions.push_back(base::FilePath::FromUTF16Unsafe(type).value().substr(1));
    } else if (base::IsStringASCII(type)) {
      net::GetExtensionsForMimeType(base::UTF16ToASCII(type), &extensions);
    }
  }
  if (!extensions.empty()) {
    types->extensions.push_back(std::move(extensions));
    types->include_all_files = true;
  }
  return types;
}

}  // namespace

// static
void FileChooser::Run(int64_t tab,
                      content::RenderFrameHost* frame,
                      scoped_refptr<content::FileSelectListener> listener,
                      const blink::mojom::FileChooserParams& params) {
  content::WebContents* contents = content::WebContents::FromRenderFrameHost(frame);
  // Go says whether the page may ask, and where the panel opens.
  hooks::Args args;
  args.Int(tab);
  args.Int(ModeOf(params.mode));
  std::vector<std::string> accept;
  for (const std::u16string& type : params.accept_types) {
    accept.push_back(base::UTF16ToUTF8(type));
  }
  args.Str(base::JoinString(accept, ","));
  PBValue result;
  bool allow = false;
  base::FilePath folder;
  if (hooks::Ask(PB_EVENT_FILE_CHOOSER, args, &result)) {
    std::optional<base::Value> answer = base::JSONReader::Read(hooks::TakeString(result), 0);
    if (answer && answer->is_dict()) {
      allow = answer->GetDict().FindBool("allow").value_or(false);
      if (const std::string* f = answer->GetDict().FindString("folder")) {
        folder = base::FilePath::FromUTF8Unsafe(*f);
      }
    }
  }
  if (!allow || !contents) {
    listener->FileSelectionCanceled();
    return;
  }
  scoped_refptr<FileChooser> chooser(new FileChooser(tab, std::move(listener)));
  chooser->Open(contents, params, folder);
}

FileChooser::FileChooser(int64_t tab, scoped_refptr<content::FileSelectListener> listener)
    : tab_(tab), listener_(std::move(listener)) {}

FileChooser::~FileChooser() {
  if (dialog_) {
    dialog_->ListenerDestroyed();
  }
}

void FileChooser::Open(content::WebContents* contents,
                       const blink::mojom::FileChooserParams& params,
                       const base::FilePath& folder) {
  dialog_ = ui::SelectFileDialog::Create(this, nullptr);
  if (!dialog_) {
    listener_->FileSelectionCanceled();
    return;
  }
  mode_ = params.mode;
  types_ = TypesOf(params.accept_types);
  types_->allowed_paths = params.need_local_path
                              ? ui::SelectFileDialog::FileTypeInfo::NATIVE_PATH
                              : ui::SelectFileDialog::FileTypeInfo::ANY_PATH;
  base::FilePath start = params.default_file_name.empty()
                             ? folder
                             : folder.Append(params.default_file_name.BaseName());
  // Held until the panel answers: Answer lets go.
  AddRef();
  dialog_->SelectFile(DialogOf(mode_), std::u16string(), start, types_.get(),
                      types_->extensions.empty() ? 0 : 1, base::FilePath::StringType(),
                      contents->GetTopLevelNativeWindow(), nullptr);
}

void FileChooser::FileSelected(const ui::SelectedFileInfo& file, int index) {
  if (mode_ == Mode::kUploadFolder) {
    folder_ = file.local_path;
    lister_ = std::make_unique<net::DirectoryLister>(
        folder_, net::DirectoryLister::NO_SORT_RECURSIVE, this);
    lister_->Start();
    return;
  }
  Answer({file}, base::FilePath());
}

void FileChooser::MultiFilesSelected(const std::vector<ui::SelectedFileInfo>& files) {
  Answer(files, base::FilePath());
}

void FileChooser::FileSelectionCanceled() {
  Answer({}, base::FilePath());
}

void FileChooser::OnListFile(const net::DirectoryLister::DirectoryListerData& data) {
  if (!data.info.IsDirectory()) {
    listed_.push_back(data.path);
  }
}

void FileChooser::OnListDone(int error) {
  lister_.reset();
  std::vector<ui::SelectedFileInfo> files;
  if (!error) {
    for (const base::FilePath& path : listed_) {
      files.emplace_back(path);
    }
  }
  Answer(std::move(files), folder_);
}

void FileChooser::Answer(std::vector<ui::SelectedFileInfo> files,
                         const base::FilePath& base_dir) {
  if (files.empty()) {
    listener_->FileSelectionCanceled();
  } else {
    // Go hears where they came from, for the next panel to open there.
    hooks::Args args;
    args.Int(tab_);
    args.Int(static_cast<int64_t>(files.size()));
    args.Str((mode_ == Mode::kUploadFolder ? base_dir : files[0].local_path.DirName())
                 .AsUTF8Unsafe());
    hooks::Fire(PB_EVENT_FILES_CHOSEN, args);
    std::vector<blink::mojom::FileChooserFileInfoPtr> chosen;
    for (const ui::SelectedFileInfo& file : files) {
      // A phone's picker hands a content address, whose last part is no name: the file's
      // own name comes with it.
      std::u16string name = file.display_name.empty()
                                ? file.local_path.BaseName().AsUTF16Unsafe()
                                : base::FilePath(file.display_name).AsUTF16Unsafe();
      chosen.push_back(blink::mojom::FileChooserFileInfo::NewNativeFile(
          blink::mojom::NativeFileInfo::New(file.local_path, name,
                                            std::vector<std::u16string>())));
    }
    listener_->FileSelected(std::move(chosen), base_dir, mode_);
  }
  listener_.reset();
  Release();
}

}  // namespace project_b

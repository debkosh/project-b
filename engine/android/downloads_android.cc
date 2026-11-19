// A phone's part of downloads (docs/daily-driver.md): no Save panel, so the file goes where
// Go said; once done it is copied into the phone's shared Downloads, where the Files app and
// other apps find it, and opened from there.

#include <utility>

#include "base/android/jni_android.h"
#include "base/android/jni_string.h"
#include "base/functional/callback.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/android/jni_headers/Downloads_jni.h"
#include "project_b/engine/browser/downloads.h"
#include "project_b/engine/browser/hooks.h"

namespace project_b {

using base::android::AttachCurrentThread;
using base::android::ConvertUTF8ToJavaString;

void ChooseSavePath(content::WebContents* contents,
                    const base::FilePath& suggested,
                    base::OnceCallback<void(const base::FilePath&)> chosen) {
  std::move(chosen).Run(suggested);
}

void PublishFile(const base::FilePath& path, const std::string& mime_type) {
  JNIEnv* env = AttachCurrentThread();
  Java_Downloads_publish(env, ConvertUTF8ToJavaString(env, path.AsUTF8Unsafe()),
                         ConvertUTF8ToJavaString(env, mime_type));
}

void OpenFile(const base::FilePath& path) {
  JNIEnv* env = AttachCurrentThread();
  Java_Downloads_open(env, ConvertUTF8ToJavaString(env, path.AsUTF8Unsafe()));
}

void RevealFile(const base::FilePath& path) {
  Java_Downloads_reveal(AttachCurrentThread());
}

void ShowDownload(int64_t id,
                  const std::string& name,
                  const base::FilePath& path,
                  int32_t state,
                  int64_t received,
                  int64_t total,
                  bool paused) {
  JNIEnv* env = AttachCurrentThread();
  Java_Downloads_show(env, id, ConvertUTF8ToJavaString(env, name),
                      ConvertUTF8ToJavaString(env, path.AsUTF8Unsafe()), state, received, total,
                      paused);
}

// A notification's Pause, Resume or Cancel, by Go's id for the download, to Go as the
// bar's and the Downloads page's buttons go.
static void JNI_Downloads_Command(JNIEnv* env, int64_t id, int32_t command) {
  hooks::Args args;
  args.Int(id);
  args.Int(command);
  hooks::Fire(PB_EVENT_DOWNLOAD_ACTION, args);
}

}  // namespace project_b

DEFINE_JNI(Downloads)

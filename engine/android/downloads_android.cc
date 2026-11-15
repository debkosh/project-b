// A phone's part of downloads (docs/daily-driver.md): no Save panel, so the file goes where
// Go said; once done it is copied into the phone's shared Downloads, where the Files app and
// other apps find it, and opened from there.

#include <utility>

#include "base/android/jni_android.h"
#include "base/android/jni_string.h"
#include "base/functional/callback.h"
#include "project_b/engine/android/jni_headers/Downloads_jni.h"
#include "project_b/engine/browser/downloads.h"

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

}  // namespace project_b

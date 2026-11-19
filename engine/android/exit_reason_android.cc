// Why an earlier run's process ended, as Android keeps it (exit_reason.h).

#include "project_b/engine/browser/exit_reason.h"

#include "base/android/jni_android.h"
#include "project_b/engine/android/jni_headers/ExitReasons_jni.h"

namespace project_b {

int32_t ExitReason(int64_t pid) {
  return Java_ExitReasons_reason(base::android::AttachCurrentThread(), pid);
}

}  // namespace project_b

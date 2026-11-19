// A phone's passkey requests (docs/passkeys.md, Our own passkeys): content hands them to Java
// alone, so Passkeys.java is each frame's authenticator, and this carries its word of a
// request to Go as the Mac's PasskeyRequest does, a dialog of kind passkey with steps, and
// Go's answers back.

#include <string>

#include "base/android/jni_android.h"
#include "base/android/jni_string.h"
#include "base/strings/string_number_conversions.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/android/jni_headers/Passkeys_jni.h"
#include "project_b/engine/android/passkeys_android.h"
#include "project_b/engine/browser/dialogs.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/timeline.h"

namespace project_b {

using base::android::AttachCurrentThread;
using base::android::ConvertJavaStringToUTF8;
using base::android::ConvertUTF8ToJavaString;

void AnswerPasskey(int64_t id, const std::string& action, const std::string& text) {
  JNIEnv* env = AttachCurrentThread();
  Java_Passkeys_answer(env, id, ConvertUTF8ToJavaString(env, action),
                       ConvertUTF8ToJavaString(env, text));
}

static int64_t JNI_Passkeys_Open(JNIEnv* env,
                                 const jni_zero::JavaRef<jobject>& jcontents,
                                 const jni_zero::JavaRef<jstring>& jsite,
                                 const jni_zero::JavaRef<jstring>& jorigin) {
  std::string site = ConvertJavaStringToUTF8(env, jsite);
  std::string origin = ConvertJavaStringToUTF8(env, jorigin);
  int64_t tab = hooks::TabOf(content::WebContents::FromJavaWebContents(jcontents));
  if (tab <= 0) {
    return 0;
  }
  int64_t id = Dialogs::Get()->NextId();
  hooks::Args args;
  args.Int(id);
  args.Int(tab);
  args.Int(PB_DIALOG_PASSKEY);
  args.Str(site);
  args.Str(std::string());
  args.Str(origin);
  args.Bool(false);
  hooks::Fire(PB_EVENT_DIALOG_OPENED, args);
  return id;
}

static void JNI_Passkeys_Step(JNIEnv* env,
                              int64_t id,
                              const jni_zero::JavaRef<jstring>& jname,
                              const jni_zero::JavaRef<jstring>& jcarries) {
  std::string name = ConvertJavaStringToUTF8(env, jname);
  std::string carries = ConvertJavaStringToUTF8(env, jcarries);
  timeline::Mark("passkey.step", "dialog=" + base::NumberToString(id) + " step=" + name);
  hooks::Args args;
  args.Int(id);
  args.Str(name);
  args.Str(carries);
  hooks::Fire(PB_EVENT_PASSKEY_STEP, args);
}

static void JNI_Passkeys_Closed(JNIEnv* env, int64_t id) {
  hooks::Args args;
  args.Int(id);
  hooks::Fire(PB_EVENT_DIALOG_CLOSED, args);
}

}  // namespace project_b

DEFINE_JNI(Passkeys)

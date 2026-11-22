#include "open_android.h"

#include "_cgo_export.h"

static jclass open_;
static jmethodID open_file, reveal, is_default, ask_default;

static void JNICALL DefaultBrowserAnswered(JNIEnv* env, jclass cls, jboolean yes) {
  oskitOpenDefaultBrowser(yes ? 1 : 0);
}

static void JNICALL Opened(JNIEnv* env, jclass cls, jstring url) {
  const char* u = (*env)->GetStringUTFChars(env, url, NULL);
  oskitOpenURLs((char*)u);
  (*env)->ReleaseStringUTFChars(env, url, u);
}

int oskit_open_register(JNIEnv* env) {
  jclass c = (*env)->FindClass(env, "io/github/sudiptadeb/oskit/Open");
  if (c == NULL) {
    (*env)->ExceptionClear(env);
    return 0;
  }
  open_ = (jclass)(*env)->NewGlobalRef(env, c);
  (*env)->DeleteLocalRef(env, c);
  open_file = (*env)->GetStaticMethodID(env, open_, "openFile", "(Ljava/lang/String;)V");
  reveal = (*env)->GetStaticMethodID(env, open_, "reveal", "(Ljava/lang/String;)V");
  is_default = (*env)->GetStaticMethodID(env, open_, "isDefaultBrowser", "()Z");
  ask_default = (*env)->GetStaticMethodID(env, open_, "askDefaultBrowser", "()V");
  static const JNINativeMethod natives[] = {
      {"defaultBrowserAnswered", "(Z)V", (void*)DefaultBrowserAnswered},
      {"opened", "(Ljava/lang/String;)V", (void*)Opened},
  };
  return (*env)->RegisterNatives(env, open_, natives, 2) == JNI_OK;
}

static void WithString(JNIEnv* env, jmethodID method, const char* s) {
  jstring j = (*env)->NewStringUTF(env, s);
  (*env)->CallStaticVoidMethod(env, open_, method, j);
  (*env)->DeleteLocalRef(env, j);
}

void oskit_open_file(JNIEnv* env, const char* path) {
  WithString(env, open_file, path);
}

void oskit_open_reveal(JNIEnv* env, const char* path) {
  WithString(env, reveal, path);
}

int oskit_open_is_default_browser(JNIEnv* env) {
  jboolean yes = (*env)->CallStaticBooleanMethod(env, open_, is_default);
  return !(*env)->ExceptionCheck(env) && yes;
}

void oskit_open_ask_default_browser(JNIEnv* env) {
  (*env)->CallStaticVoidMethod(env, open_, ask_default);
}

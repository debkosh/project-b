#include "media_android.h"

#include "_cgo_export.h"

static jclass media;
static jmethodID allowed, ask, open_settings;

static void JNICALL Answered(JNIEnv* env, jclass cls, jboolean camera, jboolean microphone) {
  oskitMediaAnswered(camera ? 1 : 0, microphone ? 1 : 0);
}

static void JNICALL Returned(JNIEnv* env, jclass cls) {
  oskitMediaReturned();
}

int oskit_media_register(JNIEnv* env) {
  jclass c = (*env)->FindClass(env, "io/github/sudiptadeb/oskit/Media");
  if (c == NULL) {
    (*env)->ExceptionClear(env);
    return 0;
  }
  media = (jclass)(*env)->NewGlobalRef(env, c);
  (*env)->DeleteLocalRef(env, c);
  allowed = (*env)->GetStaticMethodID(env, media, "allowed", "(I)Z");
  ask = (*env)->GetStaticMethodID(env, media, "ask", "(ZZ)V");
  open_settings = (*env)->GetStaticMethodID(env, media, "openSettings", "()V");
  static const JNINativeMethod natives[] = {
      {"answered", "(ZZ)V", (void*)Answered},
      {"returned", "()V", (void*)Returned},
  };
  return (*env)->RegisterNatives(env, media, natives, 2) == JNI_OK;
}

int oskit_media_allowed(JNIEnv* env, int kind) {
  jboolean yes = (*env)->CallStaticBooleanMethod(env, media, allowed, (jint)kind);
  return !(*env)->ExceptionCheck(env) && yes;
}

void oskit_media_ask(JNIEnv* env, int camera, int microphone) {
  (*env)->CallStaticVoidMethod(env, media, ask, (jboolean)(camera != 0), (jboolean)(microphone != 0));
}

void oskit_media_open_settings(JNIEnv* env) {
  (*env)->CallStaticVoidMethod(env, media, open_settings);
}

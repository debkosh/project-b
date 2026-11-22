#include "clip_android.h"

static jclass clip;
static jmethodID copy_secret;

int oskit_clip_register(JNIEnv* env) {
  jclass c = (*env)->FindClass(env, "io/github/sudiptadeb/oskit/Clip");
  if (c == NULL) {
    (*env)->ExceptionClear(env);
    return 0;
  }
  clip = (jclass)(*env)->NewGlobalRef(env, c);
  (*env)->DeleteLocalRef(env, c);
  copy_secret = (*env)->GetStaticMethodID(env, clip, "copySecret", "(Ljava/lang/String;)V");
  return copy_secret != NULL;
}

void oskit_clip_copy_secret(JNIEnv* env, const char* text) {
  jstring t = (*env)->NewStringUTF(env, text);
  (*env)->CallStaticVoidMethod(env, clip, copy_secret, t);
  (*env)->DeleteLocalRef(env, t);
}

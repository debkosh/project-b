#include "notify_android.h"

#include "_cgo_export.h"

static jclass notify;
static jmethodID allowed, ask, show, remove_;

static void JNICALL Clicked(JNIEnv* env, jclass cls, jstring id, jstring action) {
  const char* i = (*env)->GetStringUTFChars(env, id, NULL);
  const char* a = (*env)->GetStringUTFChars(env, action, NULL);
  oskitNotifyClicked((char*)i, (char*)a);
  (*env)->ReleaseStringUTFChars(env, id, i);
  (*env)->ReleaseStringUTFChars(env, action, a);
}

static void JNICALL Answered(JNIEnv* env, jclass cls, jboolean yes) {
  oskitNotifyAnswered(yes ? 1 : 0);
}

int oskit_notify_register(JNIEnv* env) {
  jclass c = (*env)->FindClass(env, "io/github/sudiptadeb/oskit/Notify");
  if (c == NULL) {
    (*env)->ExceptionClear(env);
    return 0;
  }
  notify = (jclass)(*env)->NewGlobalRef(env, c);
  (*env)->DeleteLocalRef(env, c);
  allowed = (*env)->GetStaticMethodID(env, notify, "allowed", "()Z");
  ask = (*env)->GetStaticMethodID(env, notify, "ask", "()V");
  show = (*env)->GetStaticMethodID(
      env, notify, "show",
      "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;"
      "[Ljava/lang/String;[Ljava/lang/String;ZDZ)V");
  remove_ = (*env)->GetStaticMethodID(env, notify, "remove", "(Ljava/lang/String;)V");
  static const JNINativeMethod natives[] = {
      {"clicked", "(Ljava/lang/String;Ljava/lang/String;)V", (void*)Clicked},
      {"answered", "(Z)V", (void*)Answered},
  };
  return (*env)->RegisterNatives(env, notify, natives, 2) == JNI_OK;
}

int oskit_notify_allowed(JNIEnv* env) {
  jboolean yes = (*env)->CallStaticBooleanMethod(env, notify, allowed);
  return !(*env)->ExceptionCheck(env) && yes;
}

void oskit_notify_ask(JNIEnv* env) {
  (*env)->CallStaticVoidMethod(env, notify, ask);
}

static jobjectArray Strings(JNIEnv* env, const char** s, int n) {
  jclass string = (*env)->FindClass(env, "java/lang/String");
  jobjectArray a = (*env)->NewObjectArray(env, n, string, NULL);
  for (int i = 0; i < n; i++) {
    jstring j = (*env)->NewStringUTF(env, s[i]);
    (*env)->SetObjectArrayElement(env, a, i, j);
    (*env)->DeleteLocalRef(env, j);
  }
  (*env)->DeleteLocalRef(env, string);
  return a;
}

void oskit_notify_show(JNIEnv* env,
                       const char* id,
                       const char* title,
                       const char* subtitle,
                       const char* body,
                       const char** action_ids,
                       const char** action_labels,
                       int n_actions,
                       int ongoing,
                       double progress,
                       int quiet) {
  jstring i = (*env)->NewStringUTF(env, id);
  jstring t = (*env)->NewStringUTF(env, title);
  jstring s = (*env)->NewStringUTF(env, subtitle);
  jstring b = (*env)->NewStringUTF(env, body);
  jobjectArray ids = Strings(env, action_ids, n_actions);
  jobjectArray labels = Strings(env, action_labels, n_actions);
  (*env)->CallStaticVoidMethod(env, notify, show, i, t, s, b, ids, labels, (jboolean)(ongoing != 0),
                               (jdouble)progress, (jboolean)(quiet != 0));
  (*env)->DeleteLocalRef(env, i);
  (*env)->DeleteLocalRef(env, t);
  (*env)->DeleteLocalRef(env, s);
  (*env)->DeleteLocalRef(env, b);
  (*env)->DeleteLocalRef(env, ids);
  (*env)->DeleteLocalRef(env, labels);
}

void oskit_notify_remove(JNIEnv* env, const char* id) {
  jstring i = (*env)->NewStringUTF(env, id);
  (*env)->CallStaticVoidMethod(env, notify, remove_, i);
  (*env)->DeleteLocalRef(env, i);
}

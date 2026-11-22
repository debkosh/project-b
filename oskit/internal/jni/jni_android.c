#include "jni_android.h"

JavaVM* oskit_vm(JNIEnv* env) {
  JavaVM* vm = NULL;
  (*env)->GetJavaVM(env, &vm);
  return vm;
}

JNIEnv* oskit_env(JavaVM* vm, int* attached) {
  JNIEnv* env = NULL;
  *attached = 0;
  if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION_1_6) == JNI_EDETACHED) {
    if ((*vm)->AttachCurrentThread(vm, &env, NULL) != JNI_OK) {
      return NULL;
    }
    *attached = 1;
  }
  return env;
}

void oskit_done(JavaVM* vm, JNIEnv* env, int attached) {
  if ((*env)->ExceptionCheck(env)) {
    (*env)->ExceptionDescribe(env);
    (*env)->ExceptionClear(env);
  }
  if (attached) {
    (*vm)->DetachCurrentThread(vm);
  }
}

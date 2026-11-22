// oskit's way to Java on Android (jni_android.go).

#ifndef OSKIT_INTERNAL_JNI_JNI_ANDROID_H_
#define OSKIT_INTERNAL_JNI_JNI_ANDROID_H_

#include <jni.h>

// The VM env belongs to.
JavaVM* oskit_vm(JNIEnv* env);
// The calling thread's env, attaching it to vm when it has none: *attached then says so, for
// oskit_done.
JNIEnv* oskit_env(JavaVM* vm, int* attached);
// Ends a call into Java: an exception it threw goes to the log, not on into Go.
void oskit_done(JavaVM* vm, JNIEnv* env, int attached);

#endif  // OSKIT_INTERNAL_JNI_JNI_ANDROID_H_

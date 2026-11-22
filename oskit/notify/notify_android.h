// Android's notifications, through oskit's Notify.java (notify_android.go).

#ifndef OSKIT_NOTIFY_NOTIFY_ANDROID_H_
#define OSKIT_NOTIFY_NOTIFY_ANDROID_H_

#include <jni.h>

// Finds Notify.java and registers its natives, on a Java thread. 0 when it isn't there.
int oskit_notify_register(JNIEnv* env);
int oskit_notify_allowed(JNIEnv* env);
void oskit_notify_ask(JNIEnv* env);
void oskit_notify_show(JNIEnv* env,
                       const char* id,
                       const char* title,
                       const char* subtitle,
                       const char* body,
                       const char** action_ids,
                       const char** action_labels,
                       int n_actions);
void oskit_notify_remove(JNIEnv* env, const char* id);

#endif  // OSKIT_NOTIFY_NOTIFY_ANDROID_H_

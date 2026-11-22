// Android's opening of files and links, through oskit's Open.java (open_android.go).

#ifndef OSKIT_OPEN_OPEN_ANDROID_H_
#define OSKIT_OPEN_OPEN_ANDROID_H_

#include <jni.h>

// Finds Open.java and registers its natives, on a Java thread. 0 when it isn't there.
int oskit_open_register(JNIEnv* env);
void oskit_open_file(JNIEnv* env, const char* path);
void oskit_open_reveal(JNIEnv* env, const char* path);
int oskit_open_is_default_browser(JNIEnv* env);
void oskit_open_ask_default_browser(JNIEnv* env);

#endif  // OSKIT_OPEN_OPEN_ANDROID_H_

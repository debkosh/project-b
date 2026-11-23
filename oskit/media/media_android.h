// Android's camera and microphone permission, through oskit's Media.java (media_android.go).

#ifndef OSKIT_MEDIA_MEDIA_ANDROID_H_
#define OSKIT_MEDIA_MEDIA_ANDROID_H_

#include <jni.h>

int oskit_media_register(JNIEnv* env);
int oskit_media_allowed(JNIEnv* env, int kind);
void oskit_media_ask(JNIEnv* env, int camera, int microphone);
void oskit_media_open_settings(JNIEnv* env);

#endif  // OSKIT_MEDIA_MEDIA_ANDROID_H_

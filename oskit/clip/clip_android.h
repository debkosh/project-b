// Android's secret copy, through oskit's Clip.java (clip_android.go).

#ifndef OSKIT_CLIP_CLIP_ANDROID_H_
#define OSKIT_CLIP_CLIP_ANDROID_H_

#include <jni.h>

// Finds Clip.java, on a Java thread. 0 when it isn't there.
int oskit_clip_register(JNIEnv* env);
void oskit_clip_copy_secret(JNIEnv* env, const char* text);

#endif  // OSKIT_CLIP_CLIP_ANDROID_H_

// Android's device key, through oskit's Secret.java (secret_android.go).

#ifndef OSKIT_SECRET_SECRET_ANDROID_H_
#define OSKIT_SECRET_SECRET_ANDROID_H_

#include <jni.h>
#include <stdint.h>

// Finds Secret.java, on a Java thread. 0 when it isn't there.
int oskit_secret_register(JNIEnv* env);
// Secret.get: 1 with the 32 bytes in out, 0 for none, -1 when Java threw, which goes to the
// log.
int oskit_secret_get(JNIEnv* env, const char* name, int create, uint8_t* out);

#endif  // OSKIT_SECRET_SECRET_ANDROID_H_

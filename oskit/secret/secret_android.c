#include "secret_android.h"

static jclass secret;
static jmethodID get;

int oskit_secret_register(JNIEnv* env) {
  jclass c = (*env)->FindClass(env, "io/github/sudiptadeb/oskit/Secret");
  if (c == NULL) {
    (*env)->ExceptionClear(env);
    return 0;
  }
  secret = (jclass)(*env)->NewGlobalRef(env, c);
  (*env)->DeleteLocalRef(env, c);
  get = (*env)->GetStaticMethodID(env, secret, "get", "(Ljava/lang/String;Z)[B");
  return get != NULL;
}

int oskit_secret_get(JNIEnv* env, const char* name, int create, uint8_t* out) {
  jstring n = (*env)->NewStringUTF(env, name);
  jbyteArray key = (jbyteArray)(*env)->CallStaticObjectMethod(env, secret, get, n, (jboolean)(create != 0));
  (*env)->DeleteLocalRef(env, n);
  int got = -1;
  if ((*env)->ExceptionCheck(env)) {
    (*env)->ExceptionDescribe(env);
    (*env)->ExceptionClear(env);
  } else if (key == NULL) {
    got = 0;
  } else if ((*env)->GetArrayLength(env, key) == 32) {
    (*env)->GetByteArrayRegion(env, key, 0, 32, (jbyte*)out);
    got = 1;
  }
  if (key != NULL) {
    (*env)->DeleteLocalRef(env, key);
  }
  return got;
}

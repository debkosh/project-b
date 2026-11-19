// The table of Go's functions. Only the main process loads this library, and loading it starts
// Go's runtime. Java carries the table's address across to the content library, which has no
// link-time reference to Go, the same way app/main.cc does it on the Mac.

#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "project_b/engine/abi/pb_exports.h"

// Go's own: where it asks for the device key (core/cmd/lib/keys_android.go).
void PBSetDeviceKeyReader(int (*read)(int create, uint8_t* out));

static JavaVM* vm;
static jclass device_key;
static jmethodID device_key_get;

// read_device_key is DeviceKey.get, on whichever of Go's threads asks: 1 with the 32 bytes in
// out, 0 for none, -1 when Java threw, which goes to the log.
static int read_device_key(int create, uint8_t* out) {
  JNIEnv* env;
  int attached = 0;
  if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION_1_6) == JNI_EDETACHED) {
    if ((*vm)->AttachCurrentThread(vm, &env, NULL) != JNI_OK) {
      return -1;
    }
    attached = 1;
  }
  int got = -1;
  jbyteArray key = (jbyteArray)(*env)->CallStaticObjectMethod(env, device_key, device_key_get,
                                                              (jboolean)(create != 0));
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
  if (attached) {
    (*vm)->DetachCurrentThread(vm);
  }
  return got;
}

JNIEXPORT jlong JNICALL Java_dev_projectb_browser_GoCore_table(JNIEnv* env, jclass cls) {
  static const PBGoCalls go = {
      .copy_dir = PBCopyDir,
      .start = PBStart,
      .stop = PBStop,
      .run_posted = PBRunPosted,
      .hook = PBHook,
      .force_gc = PBForceGC,
      .recover_nil_deref = PBRecoverNilDeref,
      .crash = PBCrash,
      .go_stats = PBGoStats,
      .timeline_start = PBTimelineStart,
      .timeline_now = PBTimelineNow,
      .timeline_record = PBTimelineRecord,
      .timeline_page_origin = PBTimelinePageOrigin,
      .free = PBFree,
  };
  // DeviceKey is found here, on Java's thread, by this class's loader: a thread of Go's that
  // attaches later finds only the system's classes.
  if (vm == NULL) {
    (*env)->GetJavaVM(env, &vm);
    device_key = (*env)->NewGlobalRef(env, (*env)->FindClass(env, "dev/projectb/browser/DeviceKey"));
    device_key_get = (*env)->GetStaticMethodID(env, device_key, "get", "(Z)[B");
    PBSetDeviceKeyReader(read_device_key);
  }
  return (jlong)(intptr_t)&go;
}

// PBInit, the Go core's start, for BrowserActivity before content starts: given is the
// command line, the program first. NULL when this copy quits; else the switches Go added,
// for content's command line. Go puts those just after the program, so they are the words
// before the given ones. Both lists are walked by index, within the counts JNI and Go's
// closing NULL give: what UNSAFE_BUFFERS marks in C++.
#pragma clang unsafe_buffer_usage begin
JNIEXPORT jobjectArray JNICALL Java_dev_projectb_browser_GoCore_init(JNIEnv* env, jclass cls,
                                                                     jobjectArray given) {
  jsize argc = (*env)->GetArrayLength(env, given);
  const char** argv = calloc(argc + 1, sizeof(char*));
  for (jsize i = 0; i < argc; i++) {
    jstring word = (jstring)(*env)->GetObjectArrayElement(env, given, i);
    const char* s = (*env)->GetStringUTFChars(env, word, NULL);
    argv[i] = strdup(s);
    (*env)->ReleaseStringUTFChars(env, word, s);
    (*env)->DeleteLocalRef(env, word);
  }
  const char** finished = PBInit(argc, argv);
  for (jsize i = 0; i < argc; i++) {
    free((void*)argv[i]);
  }
  free(argv);
  if (finished == NULL) {
    return NULL;
  }
  jsize count = 0;
  while (finished[count] != NULL) {
    count++;
  }
  jsize added = count - argc;
  jobjectArray out =
      (*env)->NewObjectArray(env, added, (*env)->FindClass(env, "java/lang/String"), NULL);
  for (jsize i = 0; i < added; i++) {
    jstring word = (*env)->NewStringUTF(env, finished[1 + i]);
    (*env)->SetObjectArrayElement(env, out, i, word);
    (*env)->DeleteLocalRef(env, word);
  }
  return out;
}
#pragma clang unsafe_buffer_usage end

// Android's own: the link another app sent, from BrowserActivity, after Go started.
void PBOpenURL(char* url);

JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_openUrl(JNIEnv* env, jclass cls,
                                                                 jstring url) {
  const char* s = (*env)->GetStringUTFChars(env, url, NULL);
  PBOpenURL((char*)s);
  (*env)->ReleaseStringUTFChars(env, url, s);
}

// The table of Go's functions. Only the main process loads this library, and loading it starts
// Go's runtime. Java carries the table's address across to the content library, which has no
// link-time reference to Go, the same way app/main.cc does it on the Mac.

#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/abi/pb_system_android.h"

// Go's own: where it asks for the device key (core/cmd/lib/keys_android.go), and the OS's
// calls and their answers (core/system/platform).
void PBSetDeviceKeyReader(int (*read)(int create, uint8_t* out));
void PBSetSystemCalls(const PBSystemCalls* calls);
void PBSystemAnswered(int32_t question, int yes);
void PBSystemPersonVerified(int32_t id, int verdict);

static JavaVM* vm;
static jclass device_key;
static jmethodID device_key_get;
static jclass go_system;
static jmethodID go_system_open_file;
static jmethodID go_system_reveal_file;
static jmethodID go_system_is_default_browser;
static jmethodID go_system_ask_default_browser;
static jmethodID go_system_notifications_allowed;
static jmethodID go_system_ask_notifications;
static jmethodID go_system_verify_person;
static jmethodID go_system_cancel_person;

// The calling thread's JNIEnv, attaching a thread of Go's that has none; *attached says to
// detach it after, with detach.
static JNIEnv* attach(int* attached) {
  JNIEnv* env;
  *attached = 0;
  if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION_1_6) == JNI_EDETACHED) {
    if ((*vm)->AttachCurrentThread(vm, &env, NULL) != JNI_OK) {
      return NULL;
    }
    *attached = 1;
  }
  return env;
}

// Ends a call into Java: an exception it threw goes to the log, not up into Go.
static void detach(JNIEnv* env, int attached) {
  if ((*env)->ExceptionCheck(env)) {
    (*env)->ExceptionDescribe(env);
    (*env)->ExceptionClear(env);
  }
  if (attached) {
    (*vm)->DetachCurrentThread(vm);
  }
}

// GoSystem's static methods, as PBSystemCalls has them.
static void system_void(jmethodID method) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return;
  }
  (*env)->CallStaticVoidMethod(env, go_system, method);
  detach(env, attached);
}

static int system_bool(jmethodID method) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return 0;
  }
  jboolean yes = (*env)->CallStaticBooleanMethod(env, go_system, method);
  if ((*env)->ExceptionCheck(env)) {
    yes = JNI_FALSE;
  }
  detach(env, attached);
  return yes ? 1 : 0;
}

static void system_open_file(const char* path) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return;
  }
  jstring p = (*env)->NewStringUTF(env, path);
  (*env)->CallStaticVoidMethod(env, go_system, go_system_open_file, p);
  (*env)->DeleteLocalRef(env, p);
  detach(env, attached);
}

static void system_reveal_file(void) { system_void(go_system_reveal_file); }
static int system_is_default_browser(void) { return system_bool(go_system_is_default_browser); }
static void system_ask_default_browser(void) { system_void(go_system_ask_default_browser); }
static int system_notifications_allowed(void) { return system_bool(go_system_notifications_allowed); }
static void system_ask_notifications(void) { system_void(go_system_ask_notifications); }

static void system_verify_person(int32_t id, const char* title, const char* subtitle) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return;
  }
  jstring t = (*env)->NewStringUTF(env, title);
  jstring s = (*env)->NewStringUTF(env, subtitle);
  (*env)->CallStaticVoidMethod(env, go_system, go_system_verify_person, (jint)id, t, s);
  (*env)->DeleteLocalRef(env, t);
  (*env)->DeleteLocalRef(env, s);
  detach(env, attached);
}

static void system_cancel_person(int32_t id) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return;
  }
  (*env)->CallStaticVoidMethod(env, go_system, go_system_cancel_person, (jint)id);
  detach(env, attached);
}

static const PBSystemCalls system_calls = {
    .open_file = system_open_file,
    .reveal_file = system_reveal_file,
    .is_default_browser = system_is_default_browser,
    .ask_default_browser = system_ask_default_browser,
    .notifications_allowed = system_notifications_allowed,
    .ask_notifications = system_ask_notifications,
    .verify_person = system_verify_person,
    .cancel_person = system_cancel_person,
};

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
      .shell_file = PBShellFile,
      .timeline_start = PBTimelineStart,
      .timeline_now = PBTimelineNow,
      .timeline_record = PBTimelineRecord,
      .timeline_page_origin = PBTimelinePageOrigin,
      .free = PBFree,
  };
  // DeviceKey and GoSystem are found here, on Java's thread, by this class's loader: a thread
  // of Go's that attaches later finds only the system's classes.
  if (vm == NULL) {
    (*env)->GetJavaVM(env, &vm);
    device_key = (*env)->NewGlobalRef(env, (*env)->FindClass(env, "dev/projectb/browser/DeviceKey"));
    device_key_get = (*env)->GetStaticMethodID(env, device_key, "get", "(Z)[B");
    PBSetDeviceKeyReader(read_device_key);
    go_system = (*env)->NewGlobalRef(env, (*env)->FindClass(env, "dev/projectb/browser/GoSystem"));
    go_system_open_file = (*env)->GetStaticMethodID(env, go_system, "openFile", "(Ljava/lang/String;)V");
    go_system_reveal_file = (*env)->GetStaticMethodID(env, go_system, "revealFile", "()V");
    go_system_is_default_browser = (*env)->GetStaticMethodID(env, go_system, "isDefaultBrowser", "()Z");
    go_system_ask_default_browser = (*env)->GetStaticMethodID(env, go_system, "askDefaultBrowser", "()V");
    go_system_notifications_allowed = (*env)->GetStaticMethodID(env, go_system, "notificationsAllowed", "()Z");
    go_system_ask_notifications = (*env)->GetStaticMethodID(env, go_system, "askNotifications", "()V");
    go_system_verify_person = (*env)->GetStaticMethodID(env, go_system, "verifyPerson", "(ILjava/lang/String;Ljava/lang/String;)V");
    go_system_cancel_person = (*env)->GetStaticMethodID(env, go_system, "cancelPerson", "(I)V");
    PBSetSystemCalls(&system_calls);
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

// An answer to a question Go asked of Android, from GoSystem on the main thread.
JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_answered(JNIEnv* env, jclass cls,
                                                                  jint question, jboolean yes) {
  PBSystemAnswered(question, yes ? 1 : 0);
}

// How a check of the person ended, from GoSystem on the main thread.
JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_personVerified(JNIEnv* env, jclass cls,
                                                                        jint id, jint verdict) {
  PBSystemPersonVerified(id, verdict);
}

// The link another app sent, from BrowserActivity on the main thread, after Go started: the
// links the OS hands the app, core/system's (PBSystemOpenURLs, one per line).
void PBSystemOpenURLs(char* urls);

JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_openUrl(JNIEnv* env, jclass cls,
                                                                 jstring url) {
  const char* s = (*env)->GetStringUTFChars(env, url, NULL);
  PBSystemOpenURLs((char*)s);
  (*env)->ReleaseStringUTFChars(env, url, s);
}

// The table of Go's functions. Only the main process loads this library, and loading it starts
// Go's runtime. Java carries the table's address across to the content library, which has no
// link-time reference to Go, the same way app/main.cc does it on the Mac.

#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/abi/pb_system_android.h"

// Go's own: the OS's calls and their answers (core/system/platform).
void PBSetSystemCalls(const PBSystemCalls* calls);
void PBSystemPersonVerified(int32_t id, int verdict);
void PBSystemCodeScanned(int32_t id, char* text, int ok);
// oskit's (oskit/internal/jni): its packages find their Java and register their natives.
void OskitAttach(void* env);

static JavaVM* vm;
static jclass go_system;
static jmethodID go_system_verify_person;
static jmethodID go_system_cancel_person;
static jmethodID go_system_scan_code;
static jmethodID go_system_show_update;

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

static void system_scan_code(int32_t id, const char* prompt) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return;
  }
  jstring p = (*env)->NewStringUTF(env, prompt);
  (*env)->CallStaticVoidMethod(env, go_system, go_system_scan_code, (jint)id, p);
  (*env)->DeleteLocalRef(env, p);
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

static void system_show_update(const char* state_json) {
  int attached;
  JNIEnv* env = attach(&attached);
  if (env == NULL) {
    return;
  }
  jstring t = (*env)->NewStringUTF(env, state_json);
  (*env)->CallStaticVoidMethod(env, go_system, go_system_show_update, t);
  (*env)->DeleteLocalRef(env, t);
  detach(env, attached);
}

static const PBSystemCalls system_calls = {
    .verify_person = system_verify_person,
    .cancel_person = system_cancel_person,
    .scan_code = system_scan_code,
    .show_update = system_show_update,
};

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
  // GoSystem and oskit's classes are found here, on Java's thread, by this class's loader: a thread
  // of Go's that attaches later finds only the system's classes.
  if (vm == NULL) {
    (*env)->GetJavaVM(env, &vm);
    go_system = (*env)->NewGlobalRef(env, (*env)->FindClass(env, "dev/projectb/browser/GoSystem"));
    go_system_verify_person = (*env)->GetStaticMethodID(env, go_system, "verifyPerson", "(ILjava/lang/String;Ljava/lang/String;)V");
    go_system_cancel_person = (*env)->GetStaticMethodID(env, go_system, "cancelPerson", "(I)V");
    go_system_scan_code = (*env)->GetStaticMethodID(env, go_system, "scanCode", "(ILjava/lang/String;)V");
    go_system_show_update = (*env)->GetStaticMethodID(env, go_system, "showUpdate", "(Ljava/lang/String;)V");
    PBSetSystemCalls(&system_calls);
    // oskit's packages find their Java classes here too (oskit/internal/jni).
    OskitAttach(env);
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

// How a check of the person ended, from GoSystem on the main thread.
JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_personVerified(JNIEnv* env, jclass cls,
                                                                        jint id, jint verdict) {
  PBSystemPersonVerified(id, verdict);
}

// How a scan of a code ended, from GoSystem on the main thread: its text, or nothing read.
JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_codeScanned(JNIEnv* env, jclass cls,
                                                                     jint id, jstring text, jboolean ok) {
  const char* chars = text != NULL ? (*env)->GetStringUTFChars(env, text, NULL) : NULL;
  PBSystemCodeScanned(id, (char*)(chars != NULL ? chars : ""), ok && chars != NULL ? 1 : 0);
  if (chars != NULL) {
    (*env)->ReleaseStringUTFChars(env, text, chars);
  }
}

// A tap on the update's notification, "cancel", "retry" or "relaunch", from GoSystem.
void PBUpdateCommand(char* name);

JNIEXPORT void JNICALL Java_dev_projectb_browser_GoCore_updateCommand(JNIEnv* env, jclass cls,
                                                                       jstring name) {
  const char* s = (*env)->GetStringUTFChars(env, name, NULL);
  PBUpdateCommand((char*)s);
  (*env)->ReleaseStringUTFChars(env, name, s);
}


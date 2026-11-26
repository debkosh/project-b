// Android's half of the Go core's system calls (core/system/platform, docs/layers.md):
// go_table.c makes each through JNI to Java's GoSystem, and hands Go the table with
// PBSetSystemCalls as the library is handed to Java, before Go starts. Any of Go's threads may call them: each
// attaches to the JavaVM, and Java hops to the main thread where it needs the activity. An
// answer comes back to Go on the main thread, after the call that asked has returned. The
// engine never sees these.

#ifndef PROJECT_B_ENGINE_ABI_PB_SYSTEM_ANDROID_H_
#define PROJECT_B_ENGINE_ABI_PB_SYSTEM_ANDROID_H_

#include <stdint.h>

typedef struct PBSystemCalls {
  // Checks the person is the phone's owner, in Android's own sheet (BiometricPrompt): a
  // fingerprint, a face or the screen lock, under title and subtitle. How it ended reaches
  // Go as PBSystemPersonVerified(id, 0 verified, 1 not, 2 no screen lock), on the main
  // thread. Cancel takes the sheet of check id away.
  void (*verify_person)(int32_t id, const char* title, const char* subtitle);
  void (*cancel_person)(int32_t id);
  // Opens the QR scanner's own screen over the app, under prompt (ZXing's, in
  // engine/android/third_party/zxing): it asks for the camera the first time. How it ended
  // reaches Go as PBSystemCodeScanned(id, text, 1), or (id, "", 0) when the person left it
  // or it couldn't open, on the main thread.
  void (*scan_code)(int32_t id, const char* prompt);
  // The updater's state as its JSON, at each change: amarit's UpdateNotification shows it.
  void (*show_update)(const char* state_json);
  // What Android says of the app's processes that ended, newest first, for a crash report
  // (ProcessEnds.java): each one's reason, signal and memory, renderers' among them. The
  // text is malloc'd, for the caller to free; NULL when it can't be had.
  char* (*process_ends)(void);
} PBSystemCalls;

#endif  // PROJECT_B_ENGINE_ABI_PB_SYSTEM_ANDROID_H_

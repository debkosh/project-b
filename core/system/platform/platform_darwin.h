// The Mac's system calls, in platform_darwin.m. Any thread may call them: each hops to the main
// thread, where AppKit wants them, itself. An answer reaches Go as PBSystemAnswered, on the
// main thread, after the call that asked has returned.

#ifndef PROJECT_B_CORE_SYSTEM_PLATFORM_PLATFORM_DARWIN_H_
#define PROJECT_B_CORE_SYSTEM_PLATFORM_PLATFORM_DARWIN_H_

#include <stdint.h>

void pb_system_open_file(const char* path);
void pb_system_reveal_file(const char* path);
int pb_system_is_default_browser(void);
void pb_system_ask_default_browser(void);
// Answers the notifications question yes: a Mac's are the app's own to show.
void pb_system_answer_notifications(void);
// rgb is 0xRRGGBB, or -1 for no dot.
void pb_system_set_dock_color(int32_t rgb);
// Posts a page's notification to the notification centre; a click on it reaches Go as
// PBSystemNotificationClicked, on the main thread. Remove takes one away. A bare binary,
// as the tests run, has no bundle, and posts nothing.
void pb_system_show_notification(const char* id,
                                 const char* title,
                                 const char* subtitle,
                                 const char* body);
void pb_system_remove_notification(const char* id);
// Takes the links and files macOS opens with the app, from now on, as NSApp's delegate: they
// reach Go as PBSystemOpenURLs, on the main thread.
void pb_system_receive_links(void);

#endif  // PROJECT_B_CORE_SYSTEM_PLATFORM_PLATFORM_DARWIN_H_

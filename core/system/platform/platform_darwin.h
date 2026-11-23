// The Mac's system calls, in platform_darwin.m. Any thread may call them: each hops to the main
// thread, where AppKit wants them, itself. An answer reaches Go on the main thread, after the
// call that asked has returned.

#ifndef PROJECT_B_CORE_SYSTEM_PLATFORM_PLATFORM_DARWIN_H_
#define PROJECT_B_CORE_SYSTEM_PLATFORM_PLATFORM_DARWIN_H_

#include <stdint.h>

// Checks the person is this Mac's owner, in macOS's own sheet: Touch ID, else the Mac's
// password. The sheet reads "Project B is trying to" and then reason. How it ended reaches
// Go as PBSystemPersonVerified(id, 0 verified, 1 not, 2 no password set), on the main
// thread. Cancel takes the sheet of check id away.
void pb_system_verify_person(int32_t id, const char* reason);
void pb_system_cancel_person(int32_t id);
// A progress bar along the foot of the Dock icon, fraction of the way, 0 to 1; below 0, none.
void pb_system_dock_progress(double fraction);
// rgb is 0xRRGGBB, or -1 for no dot.
void pb_system_set_dock_color(int32_t rgb);

#endif  // PROJECT_B_CORE_SYSTEM_PLATFORM_PLATFORM_DARWIN_H_

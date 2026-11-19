#ifndef PROJECT_B_ENGINE_BROWSER_EXIT_REASON_H_
#define PROJECT_B_ENGINE_BROWSER_EXIT_REASON_H_

#include <stdint.h>

#include "build/build_config.h"

namespace project_b {

#if BUILDFLAG(IS_ANDROID)
// Why the app's process with this id, one from an earlier run, ended, as Android 11 and
// later keep it: ApplicationExitInfo's REASON_ number, or -1 when Android doesn't say
// (pb_abi.h exit_reason). Go asks it of the process its crash.log names, so a run Android
// ended for its own reasons isn't told as a crash.
int32_t ExitReason(int64_t pid);
#endif

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_EXIT_REASON_H_

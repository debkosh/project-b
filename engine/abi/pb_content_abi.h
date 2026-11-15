// What an engine built on Chromium's content layer adds to the boundary: content's own
// hooks, generated into engine/browser/hooks_gen.h and core/content/hooks_gen.go by
// tools/gen_hooks.py, numbered from 100 and crossing through the same hook call as the
// events in pb_abi.h. Go's app is built on none of them; they are for trying what content
// can do before it has an event of ours (docs/content-hooks.md).
//
// Go reaches it through PBEngineCalls.content_hooks. As in pb_abi.h, every call is on the
// main thread.

#ifndef PROJECT_B_ENGINE_ABI_PB_CONTENT_ABI_H_
#define PROJECT_B_ENGINE_ABI_PB_CONTENT_ABI_H_

#include <stdint.h>

typedef struct PBContentHooks {
  // The hash of the generated hook list the adapter was built with. Go refuses a library
  // whose list differs from its own.
  const char* table_hash;
  // Lets a hook cross to Go, or stops it. Unsubscribed hooks never cross.
  void (*set_subscribed)(int32_t hook, int on);
} PBContentHooks;

#endif  // PROJECT_B_ENGINE_ABI_PB_CONTENT_ABI_H_

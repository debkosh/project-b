// Go's exported functions, from core/cmd/lib. Only the wrapper includes this: it calls
// PBInit, and fills in PBGoCalls. The library reaches Go through that table, and the helper
// has no Go at all.

#ifndef PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
#define PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_

#include "project_b/engine/abi/pb_abi.h"

#ifdef __cplusplus
extern "C" {
#endif

// The wrapper's first call, the Go core's start, before content's (docs/layers.md). It reads
// the command line and decides whether this copy runs: NULL when it printed --version or
// handed the command line's addresses to the copy that owns the profile, and this copy
// quits. Else the command line for content, NULL-terminated, which lives as long as the
// process.
const char** PBInit(int argc, const char** argv);
char* PBCopyDir(void);
void PBStart(const PBEngineCalls* calls,
             const char* blocked_hosts,
             int skip_welcome);
void PBStop(void);
void PBRunPosted(void);
int PBHook(int32_t hook, const PBValue* args, size_t count, PBValue* result);

void PBForceGC(void);
int PBRecoverNilDeref(void);
void PBCrash(int32_t kind);
char* PBGoStats(void);
void PBOpenURLs(const char* urls);
void PBTimelineStart(const char* profile_dir, int64_t process_age);
int64_t PBTimelineNow(void);
void PBTimelineRecord(int32_t kind,
                      int64_t t,
                      const char* name,
                      size_t name_length,
                      const char* text,
                      size_t text_length);
void PBTimelinePageOrigin(const char* page, int64_t t);
void PBFree(void* p);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_

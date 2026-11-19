// The main binary, the Mac's wrapper: it starts the Go core, then content, and that is all
// (docs/layers.md). Go is linked in; content and the C++ adapter are in
// libproject_b_content.dylib, which the helper loads too.

#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/abi/pb_main.h"

int main(int argc, const char** argv) {
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
  const char** args = PBInit(argc, argv);  // the Go core starts
  if (!args) {
    return 0;  // it printed the version, or handed over to the copy that owns the profile
  }
  return ProjectBMain(args, &go);  // content starts
}

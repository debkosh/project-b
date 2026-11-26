#!/usr/bin/env python3
"""Our resource paks hold everything content_shell's do (engine/paks.gni): reads the .pak
files content/shell/BUILD.gn's repack("pak") packs, on any platform, and fails on each one
engine/paks.gni doesn't list, but for content_shell's own and its test one. Chromium keeps
no list of what content and Blink ask for, and a missing string aborts the process that asks:
Google's page crashed its renderer for the <usermedia> element's words (docs/chromium-upgrade.md,
"Resources"). Usage: tools/pak_check.py [chromium/src]."""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
# content_shell's own pages, its test resources, which a shipped build can't depend on, and
# iOS's alone, which we don't build.
LEFT_OUT = {
    "content/shell/shell_resources.pak": "content_shell's own pages",
    "content/test/web_ui_mojo_test_resources.pak": "test-only",
    "device/bluetooth/strings/bluetooth_strings_en-US.pak": "iOS only",
}


def paks(text):
    return set(re.findall(r'\$\{?root_gen_dir\}?/([\w./-]+\.pak)', text))


def main():
    src = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, "chromium", "src")
    shell_gn = os.path.join(src, "content", "shell", "BUILD.gn")
    if not os.path.exists(shell_gn):
        print("skip: no Chromium tree at %s to compare with" % src)
        return 0
    text = open(shell_gn).read()
    start = text.index('repack("pak") {')
    end = text.index('output = "$root_out_dir/content_shell.pak"', start)
    theirs = paks(text[start:end])
    ours = paks(open(os.path.join(ROOT, "engine", "paks.gni")).read())
    if len(theirs) < 20:
        print("FAIL read only %d paks from content_shell's repack: its BUILD.gn changed shape" % len(theirs))
        return 1
    missing = sorted(theirs - ours - set(LEFT_OUT))
    gone = sorted(set(LEFT_OUT) - theirs)
    ok = True
    if missing:
        ok = False
        print("FAIL content_shell packs what engine/paks.gni doesn't: " + ", ".join(missing))
        print("     add each, with its target to project_b_pak_deps, or say in LEFT_OUT why not")
    if gone:
        print("note: content_shell no longer packs " + ", ".join(gone) + "; LEFT_OUT can drop it")
    if ok:
        print("ok   engine/paks.gni packs all %d of content_shell's paks but its %d left out" % (len(theirs - set(LEFT_OUT)), len(theirs & set(LEFT_OUT))))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())

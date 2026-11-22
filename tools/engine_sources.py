#!/usr/bin/env python3
"""Android's list of the engine's sources has every shared one (tools/test.sh mock runs it).

The engine's browser/ and common/ sources are listed twice: engine/BUILD.gn, which the Mac and
Linux build from, and engine/android/BUILD.gn, the APK's. A file added to the first alone
compiles and passes on the Mac, and the APK fails at its link, an hour into a container
build (password filling's forms.cc, 2026-11-22). This fails first: every .cc of browser/ and
common/ in the shared list is in Android's too, or is named below as the desktop's own.
"""

import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# The desktop's own, as the two lists stood on 2026-11-22: a file a phone has another for
# (browser_main_parts_android.cc, downloads_android.cc), Linux's, and what the APK is built
# without.
DESKTOP_ONLY = {
    "browser/browser_main_parts_desktop.cc",
    "browser/choose_path.cc",
    "browser/devtools.cc",
    "browser/devtools_session.cc",
    "browser/downloads_desktop.cc",
    "browser/driver.cc",
    "browser/driver_linux.cc",
    "browser/notifications.cc",
    "browser/passkeys.cc",
    "browser/printing_linux.cc",
    "browser/window_linux.cc",
}


def listed(path, prefix):
    with open(os.path.join(ROOT, path), encoding="utf-8") as f:
        return set(re.findall(r'"%s((?:browser|common)/[\w/]+\.cc)"' % re.escape(prefix), f.read()))


def main():
    shared = listed("engine/BUILD.gn", "")
    android = listed("engine/android/BUILD.gn", "../")
    missing = sorted(shared - android - DESKTOP_ONLY)
    stale = sorted(DESKTOP_ONLY - shared) + sorted(DESKTOP_ONLY & android)
    for name in missing:
        print("engine/%s is in engine/BUILD.gn and not in engine/android/BUILD.gn: add it there, or to DESKTOP_ONLY in tools/engine_sources.py with why" % name)
    for name in stale:
        print("engine/%s is named DESKTOP_ONLY in tools/engine_sources.py, and is in Android's list or no longer in the shared one: take it off" % name)
    if missing or stale:
        return 1
    print("%d shared engine sources, %d of them the desktop's own, the rest in Android's list" % (len(shared), len(DESKTOP_ONLY)))
    return 0


if __name__ == "__main__":
    sys.exit(main())

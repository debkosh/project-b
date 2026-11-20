#!/bin/bash
# Runs the tests, a group at a time. The groups differ in what they need, not in what they test:
#
#   tools/test.sh mock      Go's own tests, and the shells in a browser on the mock engine:
#                           the contract every shell keeps, and each one's own tests.
#                           Needs Go and `npm install`. No Chromium build, runs anywhere.
#   tools/test.sh app       The real app through its driver and through ChromeDriver.
#                           Needs the build, or PB_BIN, and ChromeDriver. No display.
#   tools/test.sh quit      The real app downloads a file and quits, and must end cleanly.
#                           Needs what the app group needs.
#   tools/test.sh basics    One check per area of the whole-browser checklist, on the real
#                           app. Most fail today, on purpose: the result is a map of what
#                           isn't built. Needs what the app group needs. Not part of a release.
#   tools/test.sh screen    The real app in a person's desktop session: full screen, copy and
#                           paste, and first paint in the real window.
#                           NEEDS A PERSON LOGGED IN at this Mac's screen, with it unlocked,
#                           and deskrig allowed to record the screen.
#   tools/test.sh vm        Real clicks, drags and keys in the macOS VM, on build/project-b.dmg
#                           or PB_DMG (a path or URL): tools/vm/run.sh starts the VM, installs
#                           it, checks and stops the VM again. Known bugs don't fail it.
#                           NEEDS A PERSON to have logged in at this Mac's desktop once since
#                           it booted.
#   tools/test.sh android   The Android app on each emulator, one after another: Android 10,
#                           14, 16 with 16 KB pages, and 17 (the amarit units in EMULATORS).
#                           Starts each that isn't running and stops it after. Needs the APK,
#                           or PB_APK; EMULATORS names fewer, as unit:port.
#
# The tests follow the build flags (core/flags/registry.json) as tools/build.sh does: all
# in unless WITHOUT names them. With extensions in, mock and app add Go's tests with the
# tag, the extension UI on the mock engine's fake runtime, and the real app installing
# from the mock store with the Extensions flag on, and running none of it with the flag
# off. They need tools/fetch-extensions.sh and tools/pack-test-extension.py to have filled
# .tools/extensions/. WITHOUT=extensions tests a build made without them (PB_BIN), and app
# checks that it has no extension code in it.
#
# Several groups can be named at once. A group or a test that lacks what it needs says so
# and ends with exit 2 instead of running. Each test prints ok or FAIL per check; the end is
# one line per test: passed, FAILED or DID NOT RUN. Exit 1 if any failed. README.md, "Test
# it", says how to get each need.
set -u
cd "$(dirname "$0")/.."

export WITHOUT=${WITHOUT:-}
# The build flags this run has, comma-separated: Go's build tags.
BUILT=$(python3 -c 'import json, os, sys
out = set(filter(None, os.environ["WITHOUT"].split(",")))
print(",".join(f["id"] for f in json.load(open("core/flags/registry.json")) if f.get("build") and f["id"] not in out))')
built() { case ",$BUILT," in *",$1,"*) return 0 ;; esac; return 1; }
BIN=${PB_BIN:-build/pb/project_b}
APK=${PB_APK:-build/android/project-b-content.apk}
ADB="$HOME/Library/Android/sdk/platform-tools/adb"
# Each emulator the android group runs on, as its amarit unit and its adb port.
EMULATORS=${EMULATORS:-project-b-emu-29:5604 project-b-emu:5600 project-b-emu-16k:5602 project-b-emu-37:5606}
VM_PYTHON=$HOME/PW_TOOLS/macos-vm/venv/bin/python
SUMMARY=""
FAILED=0
DID_NOT_RUN=0

# Each test's result, its log, and the app's memory and CPU while it ran go to the run's
# folder (tools/results.py, docs/testing.md), unless PB_RESULTS=off. One run per call to this
# script, unless PB_RUN names one already, so several calls can share it.
export PB_RUN=${PB_RUN:-$(date -u +%Y%m%dT%H%M%SZ)-$(echo "$*" | tr ' ' '-')}
run() {
    local name=$1
    shift
    printf '\n=== %s\n' "$name"
    if [ "${PB_RESULTS:-}" = off ]; then
        "$@"
    else
        python3 tools/results.py run --group "$GROUP" --name "$name" -- "$@"
    fi
    case $? in
        0) SUMMARY="$SUMMARY"$'\n'"passed       $name" ;;
        2) SUMMARY="$SUMMARY"$'\n'"DID NOT RUN  $name"; DID_NOT_RUN=2 ;;
        *) SUMMARY="$SUMMARY"$'\n'"FAILED       $name"; FAILED=1 ;;
    esac
}

cannot() {
    echo "can't run the $1 group: $2" >&2
    exit 2
}

need_store() {
    [ -f .tools/extensions/catalogue.json ] || cannot "$1" "no mock store in .tools/extensions. Run tools/fetch-extensions.sh and tools/pack-test-extension.py."
}

need_go() {
    command -v go >/dev/null || cannot "$1" "go is not on PATH. Install it from go.dev/dl, or add /usr/local/go/bin to PATH."
}

need_app() {
    [ -x "$BIN" ] || cannot "$1" "no app at $BIN. Build it with tools/build.sh, or point PB_BIN at a project_b binary."
}

mock() {
    [ -d shell/node_modules ] || cannot mock "run \`npm install\` in shell/ first."
    need_go mock
    run "Go managers" go -C core test -tags sqlite_fts5 ./...
    run "reading other browsers" go -C browserdata test ./...
    [ -n "$BUILT" ] && run "Go managers, with $BUILT" go -C core test -tags "sqlite_fts5,$BUILT" ./...
    run "every action a person can take has a test (docs/action-coverage.md)" python3 tools/action_coverage.py --check
    run "build the shell" npm --prefix shell run build
    run "the shell on the mock engine" node shell/e2e/first-app.cjs
    run "the region outlines on the mock engine" node shell/e2e/outlines.cjs
    run "the welcome on the mock engine" node shell/e2e/welcome.cjs
    run "bringing things over from other browsers, on the mock engine" node shell/e2e/imports.cjs
    # What every shell does, for each shell in the registry, and each shell's own tests.
    for id in $(python3 -c 'import json; print(" ".join(s["id"] for s in json.load(open("core/shells/registry.json")) if not s.get("native")))'); do
        run "the $id shell keeps the shells' contract" node shell/e2e/shell-contract.cjs "$id"
    done
    for test in shell/e2e/shells/*.cjs; do
        [ -e "$test" ] || continue
        run "the $(basename "$test" .cjs) shell's own tests" node "$test"
    done
    run "the history and bookmarks pages on the mock engine" node shell/e2e/pages.cjs
    run "the settings page on the mock engine" node shell/e2e/settings.cjs
    run "Settings' Sync against a sync server, on the mock engine" node shell/e2e/sync.cjs
    run "the picker on the mock engine" node shell/e2e/picker.cjs
    run "the crash report on the mock engine" node shell/e2e/crash-report.cjs
    run "a download's details, on the desktop and a phone" node shell/e2e/download-details.cjs
    run "a passkey request's steps, on every desktop shell" node shell/e2e/passkeys.cjs
    run "Settings' Passkeys, on the mock engine" node shell/e2e/passkey-settings.cjs
    run "Settings' Passwords, on the mock engine" node shell/e2e/password-settings.cjs
    run "a ready release's notice and Settings' About, on the desktop and a phone" node shell/e2e/update-notice.cjs
    built extensions || return
    need_store mock
    run "extensions on the mock engine" node shell/e2e/extensions.cjs
}

app() {
    need_app app
    built extensions && need_store app && need_go app
    run "the real app through its driver" python3 tools/drive_first_app.py
    run "the real app through ChromeDriver" python3 tools/drive_chromedriver.py
    if built extensions; then
        run "extensions in the real app" env PB_BIN="$BIN" python3 tools/drive_extensions.py
    else
        run "no extension code in the app" python3 tools/check_no_extensions.py "$BIN"
    fi
}

quit() {
    need_app quit
    run "quitting after a download" env PB_BIN="$BIN" python3 tools/check_quit_after_download.py
}

basics() {
    need_app basics
    run "the basics, one check per area" python3 tools/check_basics.py
}

screen() {
    need_app screen
    echo "The screen group NEEDS A PERSON LOGGED IN at this Mac's screen. The app's window will show there."
    python3 tools/on_screen.py || exit 2
    run "full screen on the real screen" python3 tools/on_screen.py python3 tools/check_full_screen.py
    run "copy and paste through the Mac's clipboard" python3 tools/on_screen.py python3 tools/check_clipboard.py
    run "first paint in the real window" python3 tools/on_screen.py python3 tools/check_first_paint.py
}

vm() {
    echo "The vm group NEEDS A PERSON to have logged in at this Mac's desktop once since it booted."
    [ -x "$VM_PYTHON" ] || cannot vm "no $VM_PYTHON, the Python with numpy that the VM's tools use. Run tools/vm/setup.sh once."
    run "real clicks in the macOS VM" tools/vm/run.sh "${PB_DMG:-build/project-b.dmg}"
}

android() {
    [ -f "$APK" ] || cannot android "no APK at $APK. Build it (docs/android.md), or point PB_APK at one."
    command -v amarit >/dev/null || cannot android "amarit is not on PATH: it starts the emulators."
    # Every emulator at once, side by side: each run has its own serial, output folder and
    # ports, and one after another took the release run 70 minutes, where the longest alone
    # takes 42. Each one's output goes to its own log and is printed whole once all are done.
    local logs=build/android-drive-content emulator
    mkdir -p "$logs"
    for emulator in $EMULATORS; do
        (
            unit=${emulator%%:*} serial=emulator-${emulator##*:} started=
            SUMMARY="" FAILED=0 DID_NOT_RUN=0
            # Started here, and so stopped after, only when amarit wasn't already running it:
            # adb can lose a running emulator for a moment (its server restarting), and
            # stopping that one took it from whoever else was using it.
            if ! amarit ls 2>/dev/null | grep -q "^$unit  *running"; then
                amarit start "$unit" >/dev/null || { run "the Android app on $unit" false; printf '%s' "$SUMMARY" > "$logs/$unit.summary"; exit; }
                started=1
            fi
            until [ "$($ADB -s "$serial" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = 1 ]; do /bin/sleep 2; done
            run "the Android app on $unit ($($ADB -s "$serial" shell getprop ro.build.version.release | tr -d '\r'))" \
                env PB_APK="$APK" ADB_SERIAL="$serial" OUT="$logs/$unit" python3 tools/drive_android.py
            [ -z "$started" ] || amarit stop "$unit" >/dev/null
            printf '%s' "$SUMMARY" > "$logs/$unit.summary"
        ) > "$logs/${emulator%%:*}.log" 2>&1 &
    done
    wait
    for emulator in $EMULATORS; do
        local unit=${emulator%%:*} said
        cat "$logs/$unit.log"
        said=$(cat "$logs/$unit.summary" 2>/dev/null) || said=$'\n'"FAILED       the Android app on $unit (no result)"
        SUMMARY="$SUMMARY$said"
        case $said in *FAILED*) FAILED=1 ;; esac
        case $said in *"DID NOT RUN"*) DID_NOT_RUN=2 ;; esac
        rm -f "$logs/$unit.summary"
    done
}

[ $# -gt 0 ] || { sed -n '2,/^set -u/p' "$0" | sed '$d' | cut -c3-; exit 2; }
for group in "$@"; do
    GROUP=$group
    case $group in
        mock | app | quit | basics | screen | vm | android) $group ;;
        *) echo "no group called $group: mock, app, quit, basics, screen, vm or android" >&2; exit 2 ;;
    esac
done
echo "$SUMMARY"
[ $FAILED = 1 ] && exit 1
exit $DID_NOT_RUN

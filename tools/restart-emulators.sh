#!/bin/bash
# Restarts test emulators and checks each is usable: it boots, and the app starts with a working
# GPU. The Android 10 image now and then boots with its GPU broken, and the app then dies at start
# with "GPU process isn't usable"; that emulator is restarted again, up to three times.
# Their qemu grows over days of runs, so restart them after each release's Android group and
# daily (roles/tester.md). With the app not installed, the GPU goes unchecked, and it says so.
#
#   tools/restart-emulators.sh                          # the two release emulators
#   tools/restart-emulators.sh project-b-emu-16k:5602   # amarit unit:port, any number
set -u
ADB="$HOME/Library/Android/sdk/platform-tools/adb"
PACKAGE=dev.projectb.browser
[ $# -gt 0 ] || set -- project-b-emu-37:5606 project-b-emu-29:5604
status=0
for emulator in "$@"; do
    unit=${emulator%%:*} serial=emulator-${emulator##*:} ok=
    for attempt in 1 2 3; do
        amarit restart "$unit" >/dev/null || { echo "$unit: amarit can't restart it"; break; }
        sleep 20
        for _ in $(seq 1 90); do
            [ "$("$ADB" -s "$serial" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = 1 ] && break
            sleep 5
        done
        sleep 30   # the GPU is ready a while after boot says so
        # The keyboard's autocorrect off: it once turned a driver's typed "Sparks" into
        # "Spark's", and a search found nothing. Gboard keeps it in its preferences, as root.
        "$ADB" -s "$serial" root >/dev/null 2>&1; sleep 2
        prefs=/data/data/com.google.android.inputmethod.latin/shared_prefs/com.google.android.inputmethod.latin_preferences.xml
        "$ADB" -s "$serial" shell "[ -f $prefs ] && { grep -q auto_correction $prefs || sed -i 's|</map>|    <boolean name=\"auto_correction\" value=\"false\" />\n</map>|' $prefs; sed -i 's|name=\"auto_correction\" value=\"true\"|name=\"auto_correction\" value=\"false\"|' $prefs; am force-stop com.google.android.inputmethod.latin; }" >/dev/null 2>&1
        "$ADB" -s "$serial" logcat -c
        # Recent apps survive a restart. Seven old Files tasks there made download_outlives_screen's
        # swipe hit the wrong card, and the app "never ended" (2026-11-27), so remove every task
        # but home. Android 10 lists TaskRecord{...} with its own ids and is left alone.
        "$ADB" -s "$serial" shell dumpsys activity recents | tr -d '\r' | grep "Recent #" | grep -v "type=home" |
            sed -nE 's/.* Task\{[0-9a-f]+ #([0-9]+) .*/\1/p' | while read -r task; do
            "$ADB" -s "$serial" shell am stack remove "$task" </dev/null >/dev/null 2>&1
        done
        # A release's launch check leaves the app uninstalled; with no app to start, its GPU
        # goes unchecked, and restarting the emulator again can't change that.
        if [ -z "$("$ADB" -s "$serial" shell pm list packages "$PACKAGE" | tr -d '\r')" ]; then
            ok=nogpu; break
        fi
        "$ADB" -s "$serial" shell am start -n "$PACKAGE/.BrowserActivity" -d about:blank >/dev/null
        sleep 20
        pid=$("$ADB" -s "$serial" shell pidof "$PACKAGE" | tr -d '\r')
        dead=$("$ADB" -s "$serial" logcat -d | grep -c "GPU process isn't usable")
        "$ADB" -s "$serial" shell am force-stop "$PACKAGE"
        if [ -n "$pid" ] && [ "$dead" = 0 ]; then ok=1; break; fi
        echo "$unit: attempt $attempt, the app didn't start (GPU process died $dead times); restarting it again"
    done
    if [ "$ok" = nogpu ]; then echo "$unit: up, booted; the app isn't installed, so its GPU went unchecked"
    elif [ -n "$ok" ]; then echo "$unit: up, the app starts"; else echo "$unit: NOT USABLE"; status=1; fi
done
exit $status

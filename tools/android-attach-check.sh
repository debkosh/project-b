#!/bin/bash
# The shell-attach check (docs/todo.md): a loop on the device stops the first sandboxed process the moment
# it appears, before it can attach; Android kills it at 10 s, and the shell should be back
# soon after the 15 s watch. Flat on purpose. S=emulator serial, APK=the APK, N=runs.
S=${S:-emulator-5600}; P=dev.projectb.browser; N=${N:-3}
OUT=$(cd "$(dirname "$0")/.." && pwd)/build/android-attach; mkdir -p $OUT
adb() { ~/Library/Android/sdk/platform-tools/adb -s $S "$@"; }
adb root >/dev/null; sleep 2
[ -n "$APK" ] && adb install -r $APK | tail -1
for run in $(seq 1 $N); do
  adb shell am force-stop $P
  while adb shell ps -A -o NAME | grep -q ${P}_zygote; do sleep 0.5; done
  adb logcat -c
  adb shell "while :; do p=\$(ps -A -o PID,NAME | grep 'ProcessService0:0' | head -1 | cut -d' ' -f1); [ -z \"\$p\" ] && p=\$(ps -A -o PID,NAME | grep 'ProcessService0:0' | head -1 | awk '{print \$1}'); if [ -n \"\$p\" ]; then kill -STOP \$p; echo \$p > /data/local/tmp/frozen; break; fi; done" &
  loop=$!
  sleep 0.5
  t0=$(date +%s); adb shell am start -n $P/.BrowserActivity >/dev/null
  shown=never
  for i in $(seq 1 60); do adb exec-out uiautomator dump /dev/tty 2>/dev/null | grep -q 'content-desc="Menu"' && { shown=+$(( $(date +%s) - t0 ))s; break; }; sleep 1; done
  kill $loop 2>/dev/null
  adb logcat -b all -d > $OUT/run$run.txt
  echo "run $run: froze pid $(adb shell cat /data/local/tmp/frozen | tr -d '\r'); shell $shown; failed to attach $(grep -c 'failed to attach' $OUT/run$run.txt); unattached $(grep -c shell.unattached $OUT/run$run.txt); renderers started $(grep -c 'am_proc_start.*ProcessService0' $OUT/run$run.txt)"
  adb shell rm -f /data/local/tmp/frozen
done
adb shell am force-stop $P

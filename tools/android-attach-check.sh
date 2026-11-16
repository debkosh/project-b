#!/bin/bash
# The shell-attach check (docs/todo.md): starts the app while a loop on the device lists its
# processes as fast as it can, which slows the system enough that Android now and then kills
# the shell's renderer for not attaching ("failed to attach"), the real failure. A run where
# that happened passes when the shell shows again; a run where it didn't proves nothing either
# way. Freezing the renderer instead doesn't model it: one stopped after it bound its service
# blocks the browser's calls into it, and one killed after it bound is heard as a crash.
# Flat on purpose. S=emulator serial, APK=an APK to install first, N=runs.
S=${S:-emulator-5600}; P=dev.projectb.browser; N=${N:-6}
OUT=$(cd "$(dirname "$0")/.." && pwd)/build/android-attach; mkdir -p $OUT
adb() { ~/Library/Android/sdk/platform-tools/adb -s $S "$@"; }
adb root >/dev/null; sleep 2
[ -n "$APK" ] && adb install -r $APK | tail -1
hit=0; failed=0
for run in $(seq 1 $N); do
  adb shell am force-stop $P
  while adb shell ps -A -o NAME | grep -q ${P}_zygote; do sleep 0.5; done
  adb logcat -c
  adb shell 'end=$(($(date +%s)+12)); while [ $(date +%s) -lt $end ]; do ps -A -o PID,NAME >/dev/null; done' &
  t0=$(date +%s); adb shell am start -n $P/.BrowserActivity >/dev/null
  shown=never
  for i in $(seq 1 45); do adb exec-out uiautomator dump /dev/tty 2>/dev/null | grep -q 'content-desc="Menu"' && { shown=+$(( $(date +%s) - t0 ))s; break; }; sleep 1; done
  wait
  adb logcat -b all -d > $OUT/run$run.txt
  missed=$(grep -c 'NativeOnlySandboxedProcessService0:0.*failed to attach\|SandboxedProcessService0:0.*failed to attach' $OUT/run$run.txt)
  [ $missed -gt 0 ] && hit=$((hit + 1))
  [ "$shown" = never ] && failed=$((failed + 1))
  echo "run $run: shell renderer failed to attach: $missed; shell $shown; unattached marks $(grep -c shell.unattached $OUT/run$run.txt)"
done
adb shell am force-stop $P
echo "the race came $hit of $N times; the shell never showed $failed times"
[ $failed -gt 0 ] && exit 1
# No race, no proof: say so rather than pass.
[ $hit -eq 0 ] && { echo "inconclusive: the race never came"; exit 2; }
exit 0

#!/bin/bash
# The real app's test tools for the Chromium in CHROMIUM_VERSION, in this checkout's .tools/:
# ChromeDriver, and the mock store's extensions, which are what the Web Store serves that
# Chromium. Run it in any checkout whose .tools/ is for another version, as after the shared
# tree moves (tools/chromium-upgrade.sh runs it too). Flat on purpose; safe to re-run.
#
#   tools/fetch-test-tools.sh                 # fetch for CHROMIUM_VERSION
#   tools/fetch-test-tools.sh --which [V]     # only say which ChromeDriver it takes
#
# Chrome for Testing lags a patch at times, its list more than its files, so it takes the
# exact version when its file is there, else the newest of the same major: ChromeDriver
# drives any browser of its major. WITHOUT=extensions skips the store, as tools/test.sh does.
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
CFT=https://googlechromelabs.github.io/chrome-for-testing/latest-versions-per-milestone-with-downloads.json
CFT_FILES=https://storage.googleapis.com/chrome-for-testing-public
CD=$ROOT/.tools/chromedriver

driver_zip() { echo "$CFT_FILES/$1/mac-arm64/chromedriver-mac-arm64.zip"; }
# driver_for V prints the ChromeDriver version for Chromium V, or nothing when there is none yet.
driver_for() {
  if curl -fsI --max-time 20 "$(driver_zip "$1")" >/dev/null 2>&1; then echo "$1"; return; fi
  curl -fsS --max-time 30 "$CFT" | python3 -I -c 'import json, sys
print(json.load(sys.stdin)["milestones"].get(sys.argv[1], {}).get("version", ""))' "${1%%.*}" 2>/dev/null
}

if [ "${1:-}" = --which ]; then
  driver_for "${2:-$(tr -d '[:space:]' < CHROMIUM_VERSION)}"
  exit 0
fi

V=$(tr -d '[:space:]' < CHROMIUM_VERSION)
DRIVER=$(driver_for "$V")
[ -n "$DRIVER" ] || { echo "RESULT=failed Chrome for Testing has no ChromeDriver for ${V%%.*} yet"; exit 1; }
echo "=== ChromeDriver $DRIVER, for Chromium $V, into .tools/chromedriver"
mkdir -p "$CD"
if [ ! -f "$CD/cd-$DRIVER.zip" ]; then
  curl -fsSL -o "$CD/cd-$DRIVER.zip.part" "$(driver_zip "$DRIVER")" || { echo "RESULT=failed download of ChromeDriver $DRIVER"; exit 1; }
  mv "$CD/cd-$DRIVER.zip.part" "$CD/cd-$DRIVER.zip"
fi
(cd "$CD" && unzip -qo "cd-$DRIVER.zip") || { echo "RESULT=failed unzip of ChromeDriver $DRIVER"; exit 1; }
"$CD/chromedriver-mac-arm64/chromedriver" --version

case " ${WITHOUT:-} " in
  *" extensions "*) ;;
  *) echo "=== the mock store's extensions for Chromium $V"
     tools/fetch-extensions.sh && python3 tools/pack-test-extension.py || { echo "RESULT=failed the mock store's fetch"; exit 1; } ;;
esac
echo "RESULT=ok ChromeDriver $DRIVER for Chromium $V"

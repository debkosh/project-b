#!/bin/bash
# Fresh, unpatched Chromium checkout for project-b. Flat on purpose; safe to re-run.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TAG=$(tr -d "[:space:]" < "$ROOT/CHROMIUM_VERSION")
DT=$ROOT/depot_tools
CR=$ROOT/chromium
URL=https://chromium.googlesource.com/chromium/src.git

export PATH=$DT:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin
export PYTHONUNBUFFERED=1

echo "=== $(date '+%F %T') depot_tools"
[ -d "$DT/.git" ] || git clone --depth 1 https://chromium.googlesource.com/chromium/tools/depot_tools.git "$DT"

mkdir -p "$CR"
cd "$CR"
cat > .gclient <<GCLIENT
solutions = [
  {
    "managed": False,
    "name": "src",
    "url": "$URL",
    "custom_deps": {},
    "custom_vars": {"checkout_pgo_profiles": False},
  }
]
GCLIENT

echo "=== $(date '+%F %T') src at $TAG"
[ -d src/.git ] || { git init -q src; git -C src remote add origin "$URL"; }
git -C src fetch --depth 1 origin "+refs/tags/$TAG:refs/tags/$TAG"
git -C src checkout -q -f "refs/tags/$TAG"

# chromium.googlesource.com answers 429 to too many fetches at once (2026-11-24, at 32 jobs):
# 8 at a time, and a sync that fails goes again, up to 3 times, carrying on from where it was.
echo "=== $(date '+%F %T') gclient sync"
for try in 1 2 3; do
  gclient sync --no-history -D --force --jobs 8 && break
  [ "$try" = 3 ] && { echo "RESULT=failed gclient sync, 3 tries"; exit 1; }
  echo "=== $(date '+%F %T') gclient sync failed; again in a minute (try $((try + 1)) of 3)"
  sleep 60
done

echo "=== $(date '+%F %T') verify"
git -C src log -1 --format='%h %s'
ls src/buildtools/mac/gn src/third_party/llvm-build/Release+Asserts/bin/clang
du -sh src
echo "RESULT=ok $(date '+%F %T')"

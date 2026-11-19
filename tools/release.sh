#!/bin/bash
# Publishes the version in VERSION as a GitHub release: the release build, the signed app,
# the DMG people install from, the zip the updater downloads, the signed Android APK, and the
# manifest every installed copy checks. Runs on the build Mac only, where Chromium, the
# Android build's VM and the signing keys are. Flat on purpose.
#
#   tools/release.sh
#
# Needs gh (logged in) and amarit (go install github.com/sudiptadeb/amarit/cmd/amarit@latest).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# The release builds into the shared out/, so it holds out/.pb-build.lock from here to the end,
# as tools/build.sh does for one build, and its builds see PB_BUILD_LOCKED=1 and don't wait.
if [ "${PB_BUILD_LOCKED:-}" != 1 ]; then
  mkdir -p "$ROOT/chromium/src/out"
  exec python3 -I -c 'import fcntl, os, sys
f = open(sys.argv[1], "a")
fcntl.flock(f, fcntl.LOCK_EX)
os.set_inheritable(f.fileno(), True)
os.environ["PB_BUILD_LOCKED"] = "1"
os.execv("/bin/bash", ["/bin/bash"] + sys.argv[2:])' "$ROOT/chromium/src/out/.pb-build.lock" "$0" "$@"
fi
REPO=sudiptadeb/project-b
VERSION=$(tr -d '[:space:]' < "$ROOT/VERSION")
TAG=v$VERSION
DIST=$ROOT/build/dist
PATH=$PATH:$HOME/go/bin

cd "$ROOT"
[ -z "$(git status --porcelain)" ] || { echo "uncommitted changes; a release is what is committed"; exit 1; }
# A release is main as pushed: other sessions' checkouts or resets here would publish whatever
# they left, so refuse anything else.
[ "$(git branch --show-current)" = main ] || { echo "the checkout is not on main ($(git rev-parse --short HEAD)); a release is main"; exit 1; }
git fetch -q origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || { echo "main is not origin/main; push the bump first, or fast-forward to origin/main"; exit 1; }
if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then echo "$TAG is already released; bump VERSION"; exit 1; fi

echo "=== $(date '+%F %T') release build"
RELEASE=1 OUT=out/rel tools/build.sh | tail -3
echo "=== $(date '+%F %T') package"
OUT=out/rel tools/package.sh

echo "=== $(date '+%F %T') assets and manifest"
rm -rf "$DIST" && mkdir -p "$DIST"
cp build/project-b.dmg "$DIST/Project-B-$VERSION.dmg"
ditto -c -k --keepParent "build/package/Project B.app" "$DIST/Project-B-$VERSION-darwin-arm64.zip"

echo "=== $(date '+%F %T') Android"
tools/build.sh android arm64
tools/package.sh android
# The APK people install from, and the same file under the name amarit.json's one asset
# pattern gives every target, for the updater. An APK is a zip.
cp build/android/project-b-content.apk "$DIST/Project-B-$VERSION.apk"
cp build/android/project-b-content.apk "$DIST/Project-B-$VERSION-android-arm64.zip"
# The manifest keeps every earlier release, so start from the one published last.
gh release download --repo "$REPO" --pattern releases.json -O "$DIST/releases.json" 2>/dev/null || true
amarit release -version "$VERSION" -dist "$DIST" -out "$DIST/releases.json"

echo "=== $(date '+%F %T') publish $TAG"
git tag -a "$TAG" -m "Project B $VERSION"
git push origin "$TAG"
gh release create "$TAG" "$DIST"/* --repo "$REPO" --title "Project B $VERSION" \
    --notes "Project B $VERSION for Apple silicon Macs and arm64 Android phones. Install once from the DMG or the APK; an installed copy updates itself from here.$(
        [ -f "docs/test-reports/$VERSION.md" ] && printf '\n\nTests: https://github.com/%s/blob/%s/docs/test-reports/%s.md' "$REPO" "$TAG" "$VERSION")"
echo "RESULT=ok $TAG"

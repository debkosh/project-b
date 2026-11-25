#!/bin/bash
# Publishes the version in VERSION as a GitHub release: the release build, the signed app,
# the DMG people install from, the zip the updater downloads, the signed Android APK, and the
# manifest every installed copy checks. Runs on the build Mac only, where Chromium, the
# Android build's VM and the signing keys are. Flat on purpose.
#
#   tools/release.sh       # Stable: VERSION, after the full run (docs/releasing.md)
#   tools/release.sh dev   # Dev: main as it is, after Go's tests and the mock suite
#
# Both channels' manifest lives on the pre-release tagged channels, which every copy reads
# (docs/release-channels.md). A Dev build is the next patch with the time, 0.3.9-dev.<UTC
# minute> while VERSION is 0.3.8; its assets go on that pre-release, and the Dev build two back
# goes: the one before stays, so a rollback is one manifest edit (tools/dev-rollback.sh).
#
# Needs gh (logged in) and amarit v0.8.0 or later (go install github.com/sudiptadeb/amarit/cmd/amarit@latest).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
CHANNEL=${1:-stable}
case $CHANNEL in stable | dev) ;; *) echo "usage: tools/release.sh [dev]"; exit 2 ;; esac
cd "$ROOT"
[ -z "$(git status --porcelain)" ] || { echo "uncommitted changes; a release is what is committed"; exit 1; }
# A release is main as pushed: other sessions' checkouts or resets here would publish whatever
# they left, so refuse anything else.
[ "$(git branch --show-current)" = main ] || { echo "the checkout is not on main ($(git rev-parse --short HEAD)); a release is main"; exit 1; }
git fetch -q origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || { echo "main is not origin/main; push the bump first, or fast-forward to origin/main"; exit 1; }
# A Dev build's tests, before the build lock: Go's and the mock suite, which build nothing in out/.
if [ "$CHANNEL" = dev ] && [ "${PB_BUILD_LOCKED:-}" != 1 ]; then
  echo "=== $(date '+%F %T') Go's tests and the mock suite"
  mkdir -p build
  tools/test.sh mock > build/dev-tests.log 2>&1 || { tail -40 build/dev-tests.log; echo "RESULT=failed tests; build/dev-tests.log"; exit 1; }
fi
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
CHANNELS=channels
DIST=$ROOT/build/dist
PATH=$PATH:$HOME/go/bin
NOTES=shell/src/apps/settings/release-notes.md

if [ "$CHANNEL" = stable ]; then
  TAG=v$VERSION
  if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then echo "$TAG is already released; bump VERSION"; exit 1; fi
  # This release's notes, which the app shows in Settings' About and the release carries beside
  # its manifest: they must be this version's (docs/releasing.md).
  [ "$(sed -n 's/^# //p' "$NOTES" | head -1)" = "$VERSION" ] || { echo "$NOTES is for $(sed -n 's/^# //p' "$NOTES" | head -1), not $VERSION; write this release's notes first"; exit 1; }
else
  STABLE=$VERSION
  VERSION=$(amarit next-version -pre dev "$VERSION")
  # Every build reads VERSION: the app's bundle, the Go library and the APK; and its notes are
  # the page's own. Both go back to main's when the script ends, however it ends.
  trap 'git checkout -q -- VERSION "$NOTES"' EXIT
  printf '%s\n' "$VERSION" > VERSION
  # The notes: the changes since the Dev build before this one, from the commit its notes
  # name, or since the Stable release when there is none (tools/dev_notes.py).
  prev=$(mktemp -d)
  since=""
  if gh release download "$CHANNELS" --repo "$REPO" --pattern releases.json -D "$prev" 2>/dev/null; then
    last=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("channels", {}).get("dev", ""))' "$prev/releases.json")
    if [ -n "$last" ] && gh release download "$CHANNELS" --repo "$REPO" --pattern "notes-$last.md" -D "$prev" 2>/dev/null; then
      since=$(sed -n 's/^<!-- commit: \([0-9a-f]*\) -->$/\1/p' "$prev/notes-$last.md")
    fi
  fi
  rm -rf -- "${prev:?}"
  git cat-file -e "${since:-none}^{commit}" 2>/dev/null || since="v$STABLE"
  python3 tools/dev_notes.py "$VERSION" "$since" > "$NOTES"
fi

# The channels release keeps the newest Dev build's files besides the one just published, and
# drops older ones (tools/release_versions.py prune-dev). amarit's -prune dropped the Dev build
# Dev pointed at until now, so on 2026-11-26 a Dev build that crashed at start couldn't be rolled
# back to the one before it: its files were gone.
prune_dev_files() {
  local names gone n
  names=$(gh release view "$CHANNELS" --repo "$REPO" --json assets --jq '.assets[].name')
  # shellcheck disable=SC2086
  gone=$(python3 tools/release_versions.py prune-dev "$VERSION" $names)
  for n in $gone; do
    gh release delete-asset "$CHANNELS" "$n" --repo "$REPO" -y
  done
}

echo "=== $(date '+%F %T') release build"
RELEASE=1 OUT=out/rel tools/build.sh | tail -3
echo "=== $(date '+%F %T') package"
OUT=out/rel tools/package.sh

echo "=== $(date '+%F %T') assets and manifest"
rm -rf "$DIST" && mkdir -p "$DIST"
cp build/project-b.dmg "$DIST/Project-B-$VERSION.dmg"
# The notes beside the manifest, by version, which a copy shows as "Coming in" while it
# downloads the release (docs/updates.md).
cp "$NOTES" "$DIST/notes-$VERSION.md"
ditto -c -k --keepParent "build/package/Project B.app" "$DIST/Project-B-$VERSION-darwin-arm64.zip"

echo "=== $(date '+%F %T') Android"
tools/build.sh android arm64
tools/package.sh android
# The APK people install from, and the same file under the name amarit.json's one asset
# pattern gives every target, for the updater. An APK is a zip.
cp build/android/project-b-content.apk "$DIST/Project-B-$VERSION.apk"
cp build/android/project-b-content.apk "$DIST/Project-B-$VERSION-android-arm64.zip"
# amarit publish keeps both channels' manifest on the channels pre-release, making it the first
# time from the Stable release published last, which is where copies from before channels read.
CODE=$(amarit version-code "$VERSION")
if [ "$CHANNEL" = dev ]; then
  # A Dev build's assets are on the channels pre-release, beside the manifest. The Dev build it
  # replaces keeps its files, so pointing Dev back at it is one manifest edit; the one before
  # that goes (prune_dev_files).
  echo "=== $(date '+%F %T') publish $VERSION on $CHANNELS"
  amarit publish -github "$REPO" -tag "$CHANNELS" -version "$VERSION" -version-code "$CODE" -channel dev \
    -url '{asset}' -dist "$DIST" -out "$DIST/releases.json" "$DIST/Project-B-$VERSION.dmg" "$DIST/Project-B-$VERSION.apk" "$DIST/notes-$VERSION.md"
  prune_dev_files
  echo "RESULT=ok $VERSION https://github.com/$REPO/releases/download/$CHANNELS/Project-B-$VERSION.dmg https://github.com/$REPO/releases/download/$CHANNELS/Project-B-$VERSION.apk"
  exit 0
fi
cp "$NOTES" "$DIST/release-notes.md"
# The same DMG and APK under names that don't change, which the homepage links to as
# releases/latest/download/Project-B.dmg and .apk: always the newest Stable.
cp "$DIST/Project-B-$VERSION.dmg" "$DIST/Project-B.dmg"
cp "$DIST/Project-B-$VERSION.apk" "$DIST/Project-B.apk"

echo "=== $(date '+%F %T') publish $TAG"
git tag -a "$TAG" -m "Project B $VERSION"
git push origin "$TAG"
# The numbered release first, so the manifest never names a file not there yet; then the
# manifest on channels, and the same manifest on the release, where copies from before
# channels read it.
gh release create "$TAG" "$DIST"/* --repo "$REPO" --title "Project B $VERSION" \
    --notes "Project B $VERSION for Apple silicon Macs and arm64 Android phones. Install once from the DMG or the APK; an installed copy updates itself from here.

$(grep '^- ' "$NOTES")$(
        [ -f "docs/test-reports/$VERSION.md" ] && printf '\n\nTests: https://github.com/%s/blob/%s/docs/test-reports/%s.md' "$REPO" "$TAG" "$VERSION")"
amarit publish -github "$REPO" -tag "$CHANNELS" -version "$VERSION" -version-code "$CODE" -dist "$DIST" -out "$DIST/releases.json"
# Dev takes the Stable release too: a Stable release holds every Dev build before it, and a Dev
# build sorts below its Stable (0.3.9-dev.* < 0.3.9), so a copy on Dev was never offered 0.3.9.
# The last Dev build keeps its files, as after a Dev build's own publish; the next Dev build is
# 0.3.10-dev.*, its notes since this tag.
amarit publish -github "$REPO" -tag "$CHANNELS" -version "$VERSION" -version-code "$CODE" -channel dev \
  -dist "$DIST" -out "$DIST/releases.json"
channels=$(python3 -c 'import json,sys; c=json.load(open(sys.argv[1]))["channels"]; print(c.get("stable", ""), c.get("dev", ""))' "$DIST/releases.json")
[ "$channels" = "$VERSION $VERSION" ] || { echo "RESULT=failed the manifest's channels are $channels, not stable and dev both at $VERSION"; exit 1; }
gh release upload "$TAG" "$DIST/releases.json" --repo "$REPO" --clobber
gh release upload "$CHANNELS" "$DIST/notes-$VERSION.md" --repo "$REPO" --clobber
prune_dev_files
echo "RESULT=ok $TAG"

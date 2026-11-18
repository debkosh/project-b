#!/bin/bash
# Builds and signs the APK of this checkout, for tools/release.sh: the files git tracks in the
# trees the APK takes, and the built shell, copied into the Linux VM's container and built there
# incrementally, whichever worktree the container mounts. Starts the VM, and stops it after.
# Flat on purpose.
#
#   tools/android-release.sh      # build/android/project-b-content.apk
set -euo pipefail

# One run at a time on this Mac, from any checkout, waiting its turn: every run empties the
# container's /work/release and stops the VM at its end, so a second run beside a first wiped
# its tree or stopped the VM under it (0.2.101's APK, 2026-11-19). The lock is the Mac's, held
# for the whole run, as the build's own lock inside the container is taken too late for that.
if [ -z "${PB_APK_LOCKED:-}" ]; then
    mkdir -p "$HOME/.cache"
    echo "=== $(date '+%F %T') waiting for any other APK build on this Mac to end"
    exec env PB_APK_LOCKED=1 /usr/bin/lockf -t 7200 "$HOME/.cache/project-b-android-release.lock" "$0" "$@"
fi

ROOT=$(cd "$(dirname "$0")/.." && pwd)
export PATH=$HOME/.local/bin:/opt/homebrew/bin:$PATH
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock

cd "$ROOT"
# The APK carries the shell built from this checkout, not whatever shell/dist last held.
echo "=== $(date '+%F %T') build the shell"
(cd shell && npm run build --silent)

echo "=== $(date '+%F %T') start the VM"
colima start -p pb-android --activate=false
# Colima can keep a stopped VM's state as running: start says "already running" and there is no
# docker socket. A VM with no socket is no use to anyone, so stop it and start it again.
if [ ! -S "$HOME/.colima/pb-android/docker.sock" ]; then
    colima stop -p pb-android
    colima start -p pb-android --activate=false
fi
# Stopped after, unless another session's build holds the VM (tools/android-content-build.sh's
# lock), which stopping would kill.
trap 'docker exec pb flock -n /work/.android-build.lock true && colima stop -p pb-android' EXIT
docker start pb >/dev/null

echo "=== $(date '+%F %T') copy the tree in"
docker exec pb sh -c 'rm -rf /work/release && mkdir /work/release'
{ git ls-files -z core browserdata engine tools/android-content-build.sh VERSION; find shell/dist -type f -print0; } |
  COPYFILE_DISABLE=1 tar -c --no-xattrs --no-mac-metadata --null -T - |
  docker exec -i pb tar -x -C /work/release

echo "=== $(date '+%F %T') build"
docker exec -e REPO=/work/release pb nice -n 10 bash /work/release/tools/android-content-build.sh | tail -5

echo "=== $(date '+%F %T') sign"
tools/android-content-package.sh

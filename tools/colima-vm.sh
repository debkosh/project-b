#!/bin/bash
# The build container's VM, colima's pb-android, started and stopped by one caller at a time,
# under $HOME/.cache/project-b-colima.lock (docs/android.md). Two `colima start`s of the profile
# at once fail at its vsock and boot scripts (2026-11-22), and the container's own locks only
# cover what happens once it is up; and a build that ends can stop the VM just as another has
# started it, before that one holds it. So starting it, starting the container and taking this
# caller's hold are one step, and letting go of the hold and stopping the VM when nothing holds
# it are another.
#   tools/colima-vm.sh up <hold>     the VM and the container running, held by <hold>: a shared
#                                    lock on /work/.vm-in-use.lock that lasts until down
#   tools/colima-vm.sh down <hold>   let go of <hold>, and stop the VM unless a build or another
#                                    holder still has it
set -uo pipefail
if [ "${PB_COLIMA_LOCKED:-}" != 1 ]; then
  mkdir -p "$HOME/.cache"
  exec env PB_COLIMA_LOCKED=1 /usr/bin/lockf -t 1800 "$HOME/.cache/project-b-colima.lock" "$0" "$@"
fi
export PATH=$HOME/.local/bin:/opt/homebrew/bin:$PATH
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock
hold=${2:?a hold name, as vm-hold-\$\$}
case ${1:-} in
up)
  colima start -p pb-android --activate=false
  # Colima can keep a stopped VM's state as running: start says "already running" and there is
  # no docker socket. A VM with no socket is no use to anyone, so stop it and start it again.
  if [ ! -S "$HOME/.colima/pb-android/docker.sock" ]; then
    colima stop -p pb-android
    colima start -p pb-android --activate=false
  fi
  # Nice inside the guest doesn't lower the VM on macOS: its host process, found by its disk.
  "$(dirname "$0")/linux/renice-vm.sh" colima-pb-android/ || true
  docker start pb >/dev/null || exit 1
  # Waits while a job holds the lock alone, for a minute at most: this runs under the Mac's
  # lock, which every other build waits for.
  docker exec -d pb flock -s /work/.vm-in-use.lock sh -c "echo \$\$ > /tmp/$hold; exec sleep 86400" || exit 1
  for _ in $(seq 60); do
    docker exec pb test -s "/tmp/$hold" && exit 0
    sleep 1
  done
  # The waiting flock would take the lock later and hold it for a day: it goes.
  docker exec pb pkill -f "/tmp/$hold" 2>/dev/null
  echo "colima-vm.sh: no hold on /work/.vm-in-use.lock after a minute: a job holds it alone"
  exit 1
  ;;
down)
  docker exec pb sh -c "kill \$(cat /tmp/$hold) 2>/dev/null; rm -f /tmp/$hold"
  if docker exec pb flock -n /work/.android-build.lock true && docker exec pb flock -n /work/.vm-in-use.lock true; then
    colima stop -p pb-android
  fi
  ;;
*)
  echo "usage: tools/colima-vm.sh up|down <hold>"
  exit 2
  ;;
esac

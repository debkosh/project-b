#!/bin/bash
# Moves this checkout, and the Chromium tree it builds on, to another version of Chromium:
# the newest Stable by default. A person runs it and answers it; it says where we are and
# what is out first, and changes nothing before a yes. Flat on purpose; safe to re-run, so
# after a stop, fix the cause and run the same line again (docs/chromium-upgrade.md).
#
#   tools/chromium-upgrade.sh                    # asks for the platform, offers the newest Stable
#   tools/chromium-upgrade.sh mac                # the Mac build
#   tools/chromium-upgrade.sh mac 158.0.8127.89  # a version of your choosing
#   tools/chromium-upgrade.sh android            # then the APK, on container pb's tree
#   tools/chromium-upgrade.sh linux              # and the Linux app, on that same tree
#
# Chromium is under the engine interface (docs/engine-interface.md), so an upgrade builds the
# engine and what is under it, and tests that interface; the core and the shells above it
# don't change. For one platform it writes CHROMIUM_VERSION, fetches the tree at that tag,
# runs web_features_bump.py, builds, regenerates content's hooks (gen_hooks.py), fetches
# ChromeDriver and runs tools/test.sh app: the real engine through its driver and ChromeDriver.
# It stops, saying why, when one of them needs a person: a gone or missing web feature, a
# compile error, hooks that changed beyond their version line (content's API moved), or a
# failed test. It commits nothing: it ends with what changed.
#
# The Mac goes first: the web features and the hooks are checked there, and CHROMIUM_VERSION
# follows its tree. android and linux then move container pb's tree, /work/chromium, which
# both build from (docs/android.md), build there, and run the phone's or Linux's driver: the
# APK on Android 17 (tools/test.sh android), the Linux app headless (tools/linux/headless.sh).
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
SRC=$ROOT/chromium/src
OUT_PB=$SRC/out/pb
URL=https://chromium.googlesource.com/chromium/src.git
DASH='https://chromiumdash.appspot.com/fetch_releases?channel=Stable&platform=Mac&num=1'
# The platforms this script builds, and the ones it lists as not yet.
BUILDS="mac android linux"
LATER=""
# What gen_hooks.py writes; only their "from Chromium <version>" lines may change in a patch.
GENERATED="core/content engine/browser/hooks_gen.h engine/browser/hooks_gen.cc docs/content-hooks.md"

say() { echo "$*"; }
step() { echo; echo "=== $(date '+%F %T') $*"; }
stop() { echo; echo "RESULT=stopped $*"; exit 1; }
tree_version() { [ -f "$SRC/chrome/VERSION" ] && sed -n 's/^[A-Z]*=//p' "$SRC/chrome/VERSION" | paste -sd. -; }
# newer A B: A is a later version than B.
newer() { python3 -I -c 'import sys; v=lambda s: tuple(map(int, s.split("."))); sys.exit(0 if v(sys.argv[1]) > v(sys.argv[2]) else 1)' "$1" "$2"; }
ask() { local a; read -r -p "$1 " a || a=; echo "$a"; }

# A step after the yes runs under the build lock (tools/build.sh): every checkout here builds
# on this one tree, and a build mid-checkout would mix two versions.
if [ "${1:-}" = --locked ]; then
  WANT=$2 LOGS=$3
  step "CHROMIUM_VERSION is $WANT"
  printf '%s\n' "$WANT" > CHROMIUM_VERSION

  # Always: chrome/VERSION is at the tag as soon as src is checked out, before its
  # dependencies are, so it can't say a fetch finished; a sync of a synced tree is quick.
  step "fetch chromium/src at $WANT (long the first time; log $LOGS/fetch.log)"
  tools/fetch-chromium.sh > "$LOGS/fetch.log" 2>&1
  grep -q '^RESULT=ok' "$LOGS/fetch.log" || { tail -30 "$LOGS/fetch.log"; stop "fetch: tools/fetch-chromium.sh failed; $LOGS/fetch.log"; }
  [ "$(tree_version)" = "$WANT" ] || stop "fetch: chromium/src is $(tree_version) after the fetch, not $WANT"

  step "web features (tools/web_features_bump.py)"
  python3 tools/web_features_bump.py > "$LOGS/web-features.log" 2>&1 || {
    cat "$LOGS/web-features.log"
    stop "web features: Chromium binds or dropped something the registry doesn't say; docs/web-features.md, \"After a Chromium bump\""
  }
  tail -5 "$LOGS/web-features.log"

  build() {
    step "build the Mac app, $1 (log $LOGS/build-$1.log)"
    tools/build.sh > "$LOGS/build-$1.log" 2>&1
    grep -q '^RESULT=ok' "$LOGS/build-$1.log" && return 0
    tail -40 "$LOGS/build-$1.log"
    return 1
  }
  # A new Chromium brings a new clang, which the remote build server's workers must have
  # before they compile (docs/remote-build.md); with no server answering, build.sh builds here.
  REMOTE=${PB_REMOTE:-localhost:8980}
  if [ "$REMOTE" != off ] && nc -z -G 3 "${REMOTE%:*}" "${REMOTE##*:}" 2>/dev/null; then
    step "clang for the remote build server (tools/remote/toolchain.sh)"
    case ${REMOTE%:*} in localhost | 127.0.0.1) export PB_RBE_SSH=local ;; esac
    tools/remote/toolchain.sh || stop "build: tools/remote/toolchain.sh failed, above"
  fi
  # gen_hooks.py reads content's headers through out/pb's compile commands, so it runs after the
  # first build, failed or not: with a new major the old generated hooks are themselves among
  # what fails to compile (hooks_gen.h overriding methods content dropped). The build goes
  # again after it.
  first=ok
  build first || first=failed

  step "content's hooks (tools/gen_hooks.py)"
  python3 tools/gen_hooks.py > "$LOGS/gen-hooks.log" 2>&1 || {
    tail -30 "$LOGS/gen-hooks.log"
    [ "$first" = ok ] || stop "build: tools/build.sh failed, and gen_hooks.py couldn't read content without it; $LOGS/build-first.log"
    stop "hooks: tools/gen_hooks.py failed; $LOGS/gen-hooks.log"
  }
  # shellcheck disable=SC2086
  moved=$(git diff --stat -I 'Chromium [0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' -- $GENERATED)
  if [ -n "$moved" ]; then
    echo "$moved"
    # What the hand-written adapter no longer compiles against, by file, when the build failed.
    [ "$first" = ok ] || sed 's/\x1b\[[0-9;]*m//g' "$OUT_PB/siso_output" 2>/dev/null |
      grep -o '\.\./\.\./project_b/[^:]*:[0-9]*:[0-9]*: error: .*' | grep -v '_gen\.' | sort -u | head -40
    stop "hooks: content's API moved: the generated hooks changed beyond their version line${first:+ (the first build: $first)}. See git diff -- $GENERATED and the errors above; the adapter needs a person (docs/content-hooks.md, \"When Chromium is upgraded\")"
  fi
  say "only the version line changed"
  # Hooks the same and the build still failing: the hand-written adapter is what moved.
  build again || stop "build: tools/build.sh failed after gen_hooks.py, above; $LOGS/build-again.log"
  exit 0
fi

PLATFORM=${1:-}
WANT=${2:-}
HAVE=$(tr -d '[:space:]' < CHROMIUM_VERSION)
TREE=$(tree_version)

STABLE=$(curl -fsS --max-time 20 "$DASH" | python3 -I -c 'import json, sys, time
r = json.load(sys.stdin)[0]
print(r["version"], time.strftime("%Y-%m-%d", time.localtime(r["time"] / 1000)))' 2>/dev/null)
STABLE_V=${STABLE% *}

say "Project B is on Chromium $HAVE."
[ "$TREE" = "$HAVE" ] || say "The tree in chromium/src is at ${TREE:-nothing}, not $HAVE."
if [ -n "$STABLE" ]; then say "Newest Stable is $STABLE_V, out ${STABLE#* } (chromiumdash, Mac)."
else say "chromiumdash didn't answer, so the newest Stable is unknown."; fi
say "From this Mac you can build: $BUILDS${LATER:+. Not yet in this script: $LATER}."

if [ -z "$PLATFORM" ]; then PLATFORM=$(ask "Platform [mac]:"); PLATFORM=${PLATFORM:-mac}; fi
case " $BUILDS " in
  *" $PLATFORM "*) ;;
  *) case " $LATER " in
       *" $PLATFORM "*) say "$PLATFORM isn't in this script yet (docs/chromium-upgrade.md)."; exit 2 ;;
       *) say "no platform called $PLATFORM: $BUILDS"; exit 2 ;;
     esac ;;
esac

if [ -z "$WANT" ]; then
  [ -n "$STABLE_V" ] || stop "version: chromiumdash didn't answer; give the version: tools/chromium-upgrade.sh $PLATFORM <version>"
  WANT=$STABLE_V
fi
echo "$WANT" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || { say "$WANT is not a Chromium version, like 158.0.8127.89"; exit 2; }
MAJOR=${WANT%%.*}

if [ "$PLATFORM" = mac ] && [ "$WANT" = "$HAVE" ] && [ "$TREE" = "$WANT" ] && [ ! -f "build/chromium-upgrade/$WANT/stopped" ]; then
  say "This checkout and its tree are on $WANT already; nothing to do."
  exit 0
fi
newer "$HAVE" "$WANT" && say "Note: $WANT is older than $HAVE."

if ! git ls-remote --exit-code --tags "$URL" "refs/tags/$WANT" >/dev/null 2>&1; then
  stop "version: Chromium has no tag $WANT ($URL)"
fi
if [ "$PLATFORM" != mac ]; then
  [ "$HAVE" = "$WANT" ] && [ "$TREE" = "$WANT" ] && [ ! -f "build/chromium-upgrade/$WANT/stopped" ] ||
    stop "order: upgrade mac to $WANT first, to its end: the web features and the hooks are checked on the Mac's tree"
  say
  say "This re-checks out container pb's /work/chromium at $WANT, which every checkout's APK and"
  say "Linux builds use, under the container's build lock, and builds $PLATFORM there. It starts"
  say "the pb-android VM and lowers it to nice 15. Its test runs on an emulator or a Linux"
  say "container; ask the tester first. It commits nothing."
  [ "$(ask "Upgrade $PLATFORM to $WANT? [y/N]")" = y ] || { say "Nothing changed."; exit 0; }
  LOGS=$ROOT/build/chromium-upgrade/$WANT/$PLATFORM
  mkdir -p "$LOGS"
  export PATH=$HOME/.local/bin:/opt/homebrew/bin:$PATH
  export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock
  hold=chromium-upgrade-$$
  trap 'tools/colima-vm.sh down "$hold"' EXIT
  step "the pb-android VM, held up while this runs"
  tools/colima-vm.sh up "$hold" || stop "container: the VM didn't start"
  tools/linux/renice-vm.sh colima-pb-android/ | tail -1
  container_version() { docker exec pb sh -c 'sed -n "s/^[A-Z]*=//p" /work/chromium/src/chrome/VERSION | paste -sd. -'; }
  say "container pb's tree is at $(container_version)"

  # Always, as on the Mac: the version file is at the tag before the dependencies are synced.
  step "fetch /work/chromium at $WANT (long the first time; log $LOGS/fetch.log)"
  docker exec -i -e TAG="$WANT" pb flock /work/.android-build.lock bash -s < tools/android-content-fetch.sh > "$LOGS/fetch.log" 2>&1
  grep -q 'RESULT=ok' "$LOGS/fetch.log" || { tail -30 "$LOGS/fetch.log"; stop "fetch: tools/android-content-fetch.sh failed; $LOGS/fetch.log"; }
  [ "$(container_version)" = "$WANT" ] || stop "fetch: /work/chromium is $(container_version) after the fetch, not $WANT"

  step "build $PLATFORM (tools/build.sh $PLATFORM arm64; log $LOGS/build.log)"
  tools/build.sh "$PLATFORM" arm64 > "$LOGS/build.log" 2>&1
  grep -q '^RESULT=ok' "$LOGS/build.log" || { tail -40 "$LOGS/build.log"; stop "build: tools/build.sh $PLATFORM failed, above; $LOGS/build.log"; }
  if [ "$PLATFORM" = android ]; then
    step "sign the APK (tools/package.sh android; log $LOGS/package.log)"
    tools/package.sh android > "$LOGS/package.log" 2>&1 || { tail -20 "$LOGS/package.log"; stop "tests: tools/package.sh android failed; $LOGS/package.log"; }
  fi

  echo
  if [ "$(ask "The test needs the tester's emulator or Linux slot. Run it now? [Y/n]")" = n ]; then
    echo "RESULT=stopped tests: not run yet; run the same line again to run them"
    exit 1
  fi
  if [ "$PLATFORM" = android ]; then
    step "the engine interface on Android 17 (tools/test.sh android; log $LOGS/test.log)"
    PB_APK=$ROOT/build/android/project-b-content.apk EMULATORS='project-b-emu-37:5606' tools/test.sh android > "$LOGS/test.log" 2>&1
  else
    step "the engine interface on Linux, headless (tools/linux/headless.sh; log $LOGS/test.log)"
    tools/linux/headless.sh > "$LOGS/test.log" 2>&1
  fi
  code=$?
  tail -12 "$LOGS/test.log"
  [ $code = 0 ] || stop "tests: $PLATFORM's driver didn't pass, above; $LOGS/test.log"
  step "done: $PLATFORM builds on Chromium $WANT"
  git status --short
  echo "RESULT=ok $WANT $PLATFORM"
  exit 0
fi

DRIVER=$(tools/fetch-test-tools.sh --which "$WANT")
if [ -z "$DRIVER" ]; then say "ChromeDriver: Chrome for Testing has none for $MAJOR yet; tools/test.sh app will wait for one."
elif [ "$DRIVER" = "$WANT" ]; then say "ChromeDriver: $DRIVER."
else say "ChromeDriver: $DRIVER, the newest of $MAJOR (Chrome for Testing has no $WANT yet)."; fi

say
say "This re-checks out chromium/src at $WANT, which every checkout here builds on, and holds"
say "the build lock while it fetches and builds: other builds wait. Its test, tools/test.sh app,"
say "opens the app's windows on this desktop. It commits nothing."
[ "$(ask "Upgrade $PLATFORM to $WANT? [y/N]")" = y ] || { say "Nothing changed."; exit 0; }

LOGS=$ROOT/build/chromium-upgrade/$WANT
mkdir -p "$LOGS" "$SRC/out"
# Set while a run hasn't finished, so a run again after a stop carries on rather than saying
# there is nothing to do.
: > "$LOGS/stopped"

step "wait for the build lock (another build may hold it)"
PB_BUILD_LOCKED=1 python3 -I -c 'import fcntl, os, sys
f = open(sys.argv[1], "a")
fcntl.flock(f, fcntl.LOCK_EX)
os.set_inheritable(f.fileno(), True)
os.execv("/bin/bash", ["/bin/bash"] + sys.argv[2:])' "$SRC/out/.pb-build.lock" "$0" --locked "$WANT" "$LOGS" || exit 1

[ -n "$DRIVER" ] || stop "tests: Chrome for Testing has no ChromeDriver for $MAJOR yet; run this again once it has"
step "the test tools for $WANT: ChromeDriver and the mock store (tools/fetch-test-tools.sh; log $LOGS/test-tools.log)"
tools/fetch-test-tools.sh > "$LOGS/test-tools.log" 2>&1 || { tail -20 "$LOGS/test-tools.log"; stop "tests: tools/fetch-test-tools.sh failed; $LOGS/test-tools.log"; }
tail -1 "$LOGS/test-tools.log"
echo
if [ "$(ask "The test opens the app's windows on this desktop. Run it now? [Y/n]")" = n ]; then
  echo "RESULT=stopped tests: not run yet; run the same line again to run them"
  exit 1
fi
# The packed app, as people get it and as the release run tests it: relaunching to update
# is checked on the app in its bundle only.
step "pack the app (tools/package.sh; log $LOGS/package.log)"
tools/package.sh > "$LOGS/package.log" 2>&1 || { tail -20 "$LOGS/package.log"; stop "tests: tools/package.sh failed; $LOGS/package.log"; }
step "the engine interface on the real engine (tools/test.sh app; log $LOGS/app.log)"
PB_BIN="$ROOT/build/package/Project B.app/Contents/MacOS/project_b" tools/test.sh app > "$LOGS/app.log" 2>&1
code=$?
tail -12 "$LOGS/app.log"
[ $code = 0 ] || stop "tests: tools/test.sh app didn't pass, above; $LOGS/app.log"

rm -f "$LOGS/stopped"
step "done: this checkout builds on Chromium $WANT"
changed=$(git status --short)
say
if [ -n "$changed" ]; then
  echo "$changed"
  say "Nothing is committed. Next: commit these with the line"
  say "  Notes: Chromium $WANT, with its security fixes"
  say "and hand the branch to release, which runs the release run (docs/releasing.md)."
else
  say "Everything is committed. Next: hand the branch to release (docs/releasing.md)."
fi
say "Logs: $LOGS"
echo "RESULT=ok $WANT"

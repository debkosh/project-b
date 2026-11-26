#!/bin/bash
# Points the Dev channel at an earlier version, so no copy is offered the Dev build that's there
# now: a build that crashes at start, say. One manifest edit on the channels release; every
# file stays, and pointing it back undoes it.
#
#   tools/dev-rollback.sh 0.3.11-dev.202611240729     a Dev build whose files are still there
#   tools/dev-rollback.sh 0.3.10                      or a Stable release
#
# The updater never downgrades, so a copy already on the bad build stays on it (a phone needs
# the fixed APK by hand); every other copy is offered nothing newer than the version named.
# It checks that version's files download before it changes anything, and the live manifest
# after. Only on the orchestrator's or Sudipta's word: every Dev install reads this manifest.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
REPO=debkosh/project-b
CHANNELS=channels
TO=${1:?usage: tools/dev-rollback.sh VERSION}
WORK=$(mktemp -d)
trap 'rm -rf -- "${WORK:?}"' EXIT

gh release download "$CHANNELS" --repo "$REPO" --pattern releases.json -D "$WORK" --clobber
echo "now: $(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["channels"])' "$WORK/releases.json")"
# The version must be in the manifest, with assets, and every asset must download: one byte
# each, with a time limit.
urls=$(python3 -c 'import json,sys; r=json.load(open(sys.argv[1]))["releases"].get(sys.argv[2], {}); [print(a["url"]) for a in r.get("assets", {}).values()]' "$WORK/releases.json" "$TO")
[ -n "$urls" ] || { echo "RESULT=failed $TO isn't in the manifest with any files; Dev can't point there"; exit 1; }
for url in $urls; do
  code=$(curl -s -o /dev/null -r 0-0 -L --max-time 30 -w '%{http_code}' "$url" || true)
  case $code in 200 | 206) echo "ok   $url" ;; *) echo "RESULT=failed $url gives $code; its files are gone, so Dev can't point there"; exit 1 ;; esac
done
python3 "$ROOT/tools/release_versions.py" set-dev "$WORK/releases.json" "$TO"
gh release upload "$CHANNELS" "$WORK/releases.json" --repo "$REPO" --clobber
live=$(curl -sL --max-time 30 "https://github.com/$REPO/releases/download/$CHANNELS/releases.json?t=$(date +%s)" | python3 -c 'import json,sys; print(json.load(sys.stdin)["channels"]["dev"])')
[ "$live" = "$TO" ] || { echo "RESULT=failed the live manifest's dev is $live, not $TO"; exit 1; }
echo "RESULT=ok dev is $TO"

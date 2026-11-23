#!/bin/bash
# DuckDuckGo's field matching, which finds a page's sign-in fields (docs/password-autofill.md,
# "Detection"): takes theirs, builds it into the form script, and measures it.
#
#   tools/form-matching.sh update          # their latest; run at each release and with each
#                                          # new version of content
#   tools/form-matching.sh update <ref>    # a commit or tag of theirs
#   tools/form-matching.sh score           # measure what is built, on their labelled forms
#   tools/form-matching.sh check           # the built file is the one recorded (no network)
#
# update fetches github.com/duckduckgo/duckduckgo-autofill into build/duckduckgo-autofill,
# bundles core/forms/page/matching/entry.js with their matching into
# core/forms/page/matching.gen.js, keeps their LICENSE beside it, scores it, and records
# their commit, the file's SHA-256 and the scores in core/forms/page/matching/SOURCE.json.
# The scores before and after are printed: a pull that finds fewer fields shows at once.
# Then: tools/test.sh mock, and commit the three files.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
THEIRS=build/duckduckgo-autofill
GEN=core/forms/page/matching.gen.js
SOURCE=core/forms/page/matching/SOURCE.json
sum() { shasum -a 256 "$1" | cut -d' ' -f1; }

score() {
  [ -d "$THEIRS/test-forms" ] || { echo "no $THEIRS: run tools/form-matching.sh update first"; exit 1; }
  local work=build/form-matching
  mkdir -p "$work"
  node tools/form-matching/collect.cjs "$THEIRS/test-forms" > "$work/fields.jsonl"
  (cd core && go run ./cmd/formroles < "../$work/fields.jsonl" > "../$work/roles.jsonl")
  python3 tools/form-matching/score.py "$work/fields.jsonl" "$work/roles.jsonl" "$@"
}

case "${1:-}" in
update)
  ref=${2:-}
  if [ -d "$THEIRS/.git" ]; then git -C "$THEIRS" fetch -q origin; else git clone -q https://github.com/duckduckgo/duckduckgo-autofill.git "$THEIRS"; fi
  git -C "$THEIRS" checkout -q --detach "${ref:-origin/HEAD}"
  commit=$(git -C "$THEIRS" rev-parse HEAD)
  date=$(git -C "$THEIRS" log -1 --format=%cs)
  echo "=== their matching at $commit ($date)"
  size=$(node tools/form-matching/build.cjs "$ROOT/$THEIRS" "$commit" "$date")
  cp "$THEIRS/LICENSE.md" core/forms/page/matching/LICENSE
  echo "=== built $GEN, $size bytes; scoring on their $(ls "$THEIRS/test-forms" | grep -c '\.html$') labelled forms"
  score --record "$SOURCE" --commit "$commit" --date "$date" --sha256 "$(sum "$GEN")" --bytes "$size"
  ;;
score)
  score
  ;;
check)
  want=$(python3 -c "import json,sys; print(json.load(open('$SOURCE'))['sha256'])")
  [ "$(sum "$GEN")" = "$want" ] || { echo "$GEN is not the file $SOURCE records: build it with tools/form-matching.sh update <commit>, never by hand"; exit 1; }
  echo "$GEN is the file $SOURCE records, DuckDuckGo's matching at $(python3 -c "import json; d=json.load(open('$SOURCE')); print(d['commit'][:12], d['date'])")"
  ;;
*)
  sed -n 2,12p "$0"
  exit 2
  ;;
esac

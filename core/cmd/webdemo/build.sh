#!/bin/bash
# Builds the web demo into OUT, a static folder any web server can serve from any path:
# the core as WebAssembly (core.wasm), Go's wasm_exec.js, the page that starts it, dev's
# OS window and mock pages, and the shell's built pages (shell/dist, from npm run build),
# each page of ours with shim.js and dev's webui.js before its own scripts.
#
#   [VERSION=0.3.8] core/cmd/webdemo/build.sh OUT
set -euo pipefail
OUT=${1:?usage: build.sh OUT}
HERE=$(cd "$(dirname "$0")" && pwd)
CORE=$(cd "$HERE/../.." && pwd)
DEV=$CORE/cmd/dev
DIST=$CORE/../shell/dist
[ -d "$DIST/shell" ] || { echo "no $DIST/shell: run npm run build in shell/ first" >&2; exit 1; }

mkdir -p "$OUT"
rm -rf "${OUT:?}"/*
(cd "$CORE" && GOOS=js GOARCH=wasm go build -tags sqlite_fts5 -trimpath -ldflags='-s -w' -o "$OUT/core.wasm" ./cmd/webdemo)
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/"
cp "$HERE/web/index.html" "$HERE/web/fs.js" "$HERE/web/shim.js" "$HERE/web/stage.css" "$HERE/web/stage.js" "$DEV/icon.svg" "$DEV/icon-small.svg" "$OUT/"

# webui.js names its channel by the first folder of the page's address; here the demo may
# sit under a path of its own, so the channel is the first folder under the demo's.
sed "s|location.pathname.split('/')\[1\]|location.pathname.slice(new URL('.', top.location.href).pathname.length).split('/')[0]|" \
  "$DEV/webui.js" > "$OUT/webui.js"
grep -q "top.location.href" "$OUT/webui.js" || { echo "webui.js: its channel line changed" >&2; exit 1; }

# Dev's window and mock page, their pages' addresses relative to the demo's folder.
sed -e 's|<head>|<head><script src="shim.js"></script>|' \
  -e 's|`/mock/page?|`mock-page.html?|g' \
  -e 's|`/shell/\${name}/?|`shell/${name}/?|' \
  -e 's|`/\${o.page}/?|`${o.page}/?|' \
  "$DEV/window.html" > "$OUT/window.html"
for s in 'mock-page.html?' 'shell/${name}/?' '`${o.page}/?'; do
  grep -qF "$s" "$OUT/window.html" || { echo "window.html: no $s after rewriting" >&2; exit 1; }
done
# Dev's mock OS, the desktop the windows sit on, its addresses relative to the demo's folder.
sed -e 's|<head>|<head><script src="shim.js"></script>|' -e 's|src="/mock/os.js"|src="os.js"|' "$DEV/os.html" > "$OUT/os.html"
sed 's|`/mock/window?window=|`window.html?window=|' "$DEV/os.js" > "$OUT/os.js"
grep -q 'src="os.js"' "$OUT/os.html" && grep -q 'window.html?window=' "$OUT/os.js" || { echo "os page: its addresses changed" >&2; exit 1; }

# Each tab's page is the homepage (web/page.html), where dev has its mock page.
sed 's|<head>|<head><script src="shim.js"></script>|' "$HERE/web/page.html" > "$OUT/mock-page.html"
# The Stable version it names: VERSION, as the Pages workflow gives it, else the repo's.
VERSION=${VERSION:-$(cat "$CORE/../VERSION")} perl -pi -e 's/\{\{VERSION\}\}/$ENV{VERSION}/g' "$OUT/mock-page.html" "$OUT/index.html"
cp -R "$HERE/web/shots" "$OUT/"
# The shells' ids, names and words, which the homepage and the tour read.
cp "$CORE/shells/registry.json" "$OUT/shells.json"

# Our pages, each with the shim and webui.js first, as dev's server puts webui.js in.
cp -R "$DIST"/. "$OUT/"
find "$OUT" -mindepth 2 -name index.html | while read -r f; do
  depth=$(( $(echo "${f#"$OUT"/}" | tr -cd / | wc -c) ))
  up=$(printf '../%.0s' $(seq 1 "$depth"))
  # perl, as sed -i takes its suffix one way on the Mac and another on Linux, where Pages builds.
  UP=$up perl -pi -e 's|</head>|<script src="$ENV{UP}shim.js"></script><script src="$ENV{UP}webui.js"></script></head>|' "$f"
done
# The core's size, for the page to say how much of it has come while it loads.
WASM_BYTES=$(wc -c < "$OUT/core.wasm" | tr -d ' ') perl -pi -e 's/\{\{WASM_BYTES\}\}/$ENV{WASM_BYTES}/g' "$OUT/index.html"
grep -q '{{WASM_BYTES}}' "$OUT/index.html" && { echo "index.html: the core's size not written in" >&2; exit 1; }
ls -l "$OUT/core.wasm"

#!/bin/bash
# Builds the demo as out/oskit demo.app, which macOS's notification centre needs: a bare
# binary posts nothing. Signed ad hoc. Run it with `open -W "out/oskit demo.app"`, or its
# binary inside for the printed clicks.
set -euo pipefail
cd "$(dirname "$0")"
APP="out/oskit demo.app"
rm -rf "${APP:?}"
mkdir -p "$APP/Contents/MacOS"
go build -C .. -o "$PWD/$APP/Contents/MacOS/demo" .
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleIdentifier</key><string>io.github.sudiptadeb.oskit.demo</string>
  <key>CFBundleName</key><string>oskit demo</string>
  <key>CFBundleExecutable</key><string>demo</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>0.1</string>
  <key>LSUIElement</key><true/>
</dict></plist>
PLIST
codesign -s - --force "$APP"
echo "$APP"

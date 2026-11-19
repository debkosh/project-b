#!/bin/bash
# Packs a built app into "Project B.app" inside build/project-b.dmg. Flat on purpose.
# OUT=out/rel tools/package.sh packs the release build instead.
#
# It packs this checkout's own copy of its last build, build/<out dir's name>/, which
# tools/build.sh makes under its lock, never chromium/src/$OUT itself: every checkout shares
# that, and another's build between this one's build and its package would mix the two.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=${OUT:-out/pb}
FROM=$ROOT/build/$(basename "$OUT")
if [ ! -x "$FROM/project_b" ]; then
    echo "RESULT=failed no build in $FROM: run OUT=$OUT tools/build.sh in this checkout first"
    exit 1
fi
HEAD=$(git -C "$ROOT" rev-parse HEAD)
if [ "$(cat "$FROM/commit" 2>/dev/null)" != "$HEAD" ]; then
    echo "RESULT=failed $FROM was built from $(cat "$FROM/commit" 2>/dev/null || echo 'an unknown commit'), not HEAD $HEAD: build again"
    exit 1
fi
STAGE=$ROOT/build/package
DMG=$ROOT/build/project-b.dmg
APP="$STAGE/Project B.app"
MACOS=$APP/Contents/MacOS
RESOURCES=$APP/Contents/Resources
LIBRARIES=$APP/Contents/Libraries
FRAMEWORKS=$APP/Contents/Frameworks
# Every child process runs the helper, in an app of its own whose Info.plist says
# LSUIElement: none of them can ever show in the Dock as a second Project B.
HELPER_APP="$FRAMEWORKS/Project B Helper.app"
HELPER=$HELPER_APP/Contents/MacOS/project_b_helper
. "$ROOT/tools/keys.sh"                       # the signing key, from tools/signing-setup.sh
need_key identity.p12
RCODESIGN=$HOME/PW_TOOLS/project-b-signing/rcodesign   # tools/signing-setup.sh fetches it
VERSION=$(tr -d '[:space:]' < "$ROOT/VERSION")   # what the installed app says it is, and compares releases against

rm -rf "$STAGE" "$DMG"
mkdir -p "$MACOS" "$RESOURCES" "$LIBRARIES" "$FRAMEWORKS" "$HELPER_APP/Contents/MacOS"

cp "$FROM/project_b" "$MACOS/"
cp "$FROM/project_b_helper" "$HELPER"
cp "$FROM/libproject_b_content.dylib" "$FRAMEWORKS/"
cp "$FROM/commit" "$RESOURCES/commit"   # what tools/results.py says was tested
cp "$FROM/project_b.pak" "$FROM/icudtl.dat" "$FROM/snapshot_blob.bin" "$FROM"/v8_context_snapshot.*.bin "$RESOURCES/"
cp "$FROM/libEGL.dylib" "$FROM/libGLESv2.dylib" "$FROM/libvk_swiftshader.dylib" "$FROM/libvulkan.dylib" \
   "$FROM/vk_swiftshader_icd.json" "$LIBRARIES/"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDisplayName</key><string>Project B</string>
  <key>CFBundleExecutable</key><string>project_b</string>
  <key>CFBundleIdentifier</key><string>dev.projectb.browser</string>
  <key>CFBundleName</key><string>Project B</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>155.0.8059.41</string>
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSSupportsAutomaticGraphicsSwitching</key><true/>
  <!-- Phone as key (docs/passkeys.md): macOS asks the person the first time, with these
       words, and content uses no Bluetooth without them. -->
  <key>NSBluetoothAlwaysUsageDescription</key><string>Project B uses Bluetooth to check your phone is near when you sign in with a passkey on it.</string>
  <!-- A browser to macOS, which offers it as the default one and opens web links and web
       pages with it (docs/onboarding.md, Make default browser); and the app's own links,
       project-b:, such as a sync server's set-up link (docs/sync.md). -->
  <key>CFBundleURLTypes</key>
  <array>
    <dict>
      <key>CFBundleURLName</key><string>Web site URL</string>
      <key>CFBundleURLSchemes</key><array><string>http</string><string>https</string></array>
    </dict>
    <dict>
      <key>CFBundleURLName</key><string>Project B link</string>
      <key>CFBundleURLSchemes</key><array><string>project-b</string></array>
    </dict>
  </array>
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeName</key><string>Web page</string>
      <key>CFBundleTypeRole</key><string>Viewer</string>
      <key>LSHandlerRank</key><string>Default</string>
      <key>LSItemContentTypes</key><array><string>public.html</string><string>public.xhtml</string></array>
    </dict>
  </array>
</dict>
</plist>
PLIST

cat > "$HELPER_APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDisplayName</key><string>Project B Helper</string>
  <key>CFBundleExecutable</key><string>project_b_helper</string>
  <key>CFBundleIdentifier</key><string>dev.projectb.helper</string>
  <key>CFBundleName</key><string>Project B Helper</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>155.0.8059.41</string>
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>LSUIElement</key><true/>
</dict>
</plist>
PLIST

# Symbol names are about a third of a non-official binary and do nothing at run time.
# Stripping breaks the linker's signature, and rcodesign crashes reading what's left on a
# release build, so the old signature goes first. The graphics libraries keep the linker's
# signature, because rcodesign can't parse them after a release build.
for binary in "$MACOS/project_b" "$HELPER" "$FRAMEWORKS"/*.dylib; do
    codesign --remove-signature "$binary"
    strip -x "$binary" 2>/dev/null
done
"$RCODESIGN" sign \
    --p12-file "$KEYS/identity.p12" \
    --p12-password-file <(grep '^PROJECT_B_SIGNING_P12_PASSWORD=' "$HOME/PW_TOOLS/lab-secrets.env" | cut -d= -f2- | tr -d '\n') \
    --timestamp-url none \
    --exclude "Contents/Libraries/**" \
    "$APP"
codesign --verify --deep --strict "$APP"
codesign --verify -R="certificate leaf = H\"$(cat "$KEYS/identity.sha1")\"" "$APP"

ln -s /Applications "$STAGE/Applications"
# hdiutil create -srcfolder gives up with "Resource busy" when its temporary volume is slow
# to unmount, so we mount and unmount it ourselves. Its -hfs makehybrid avoids the mount but
# adds an attribute that fails strict signature checks.
RAW=$ROOT/build/project-b-rw.dmg
MOUNT=$ROOT/build/package-mount
mkdir -p "$MOUNT"
hdiutil create -quiet -size "$(( $(du -sk "$STAGE" | cut -f1) / 1024 + 64 ))m" -fs HFS+ -volname "Project B" -ov "$RAW"
hdiutil attach -quiet -nobrowse -noverify -mountpoint "$MOUNT" "$RAW"
ditto "$STAGE" "$MOUNT"
for attempt in 1 2 3 4 5 6; do
    hdiutil detach -quiet "$MOUNT" && break
    sleep 5
done
hdiutil convert -quiet "$RAW" -format ULMO -ov -o "$DMG"
rm -f "$RAW"
rmdir "$MOUNT"
hdiutil verify -quiet "$DMG"

du -sh "$APP" "$DMG"
echo "RESULT=ok $DMG"

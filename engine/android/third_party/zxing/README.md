# ZXing's scanner for Android

The QR scanner the Android app opens to read a sync backup code (docs/sync.md, "Backup
code"): a ready screen with the camera and the reading in it. Used as it is; nothing here is
changed. Both are under the Apache License 2.0 (LICENSE).

| File | What | From | SHA-256 |
|---|---|---|---|
| `zxing-android-embedded-4.3.0.aar` | The scanner screen, `com.journeyapps:zxing-android-embedded` 4.3.0 | https://repo1.maven.org/maven2/com/journeyapps/zxing-android-embedded/4.3.0/zxing-android-embedded-4.3.0.aar | `4ab03353127c34e55cbb80fbc9a34031decbcfb1ec5e8f991944d2d494621a33` |
| `core-3.4.1.jar` | The reading, `com.google.zxing:core` 3.4.1, which the screen is built on | https://repo1.maven.org/maven2/com/google/zxing/core/3.4.1/core-3.4.1.jar | `acfa30338c26267b6cab665509c2251bb96e8350bfcad89d59b949ba77011f8d` |
| `zxing-android-embedded-4.3.0.info` | What the .aar holds, for the build | Chromium's `build/android/gyp/aar.py list <aar> --output <info>` | |

Sources: https://github.com/journeyapps/zxing-android-embedded and https://github.com/zxing/zxing.

To take a newer one: fetch the two files from Maven Central, check each against the
`.sha1` beside it there, make the `.info` again, and put the names, addresses and sums here
and in `engine/android/BUILD.gn`. The APK's build has no network, so the files are kept here.

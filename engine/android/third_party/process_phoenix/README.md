# ProcessPhoenix for Android

Restarts the Android app for Settings' Relaunch, which a flag's change asks for: its screen,
in a process of its own, ends the app's process and opens the app again in a new task, as
Chrome's own restart does. The Go core starts no program on a phone, since Android 10
forbids the call Go makes first. Used as it is; nothing here is changed. Under the Apache
License 2.0 (LICENSE).

| File | What | From | SHA-256 |
|---|---|---|---|
| `process-phoenix-3.0.0.aar` | `com.jakewharton:process-phoenix` 3.0.0 | https://repo1.maven.org/maven2/com/jakewharton/process-phoenix/3.0.0/process-phoenix-3.0.0.aar | `ff18e68bb76f92338d2a72b854bcc47e978d9a483b548875a84cfba9e2f7a6c7` |
| `process-phoenix-3.0.0.info` | What the .aar holds, for the build | Chromium's `build/android/gyp/aar.py list <aar> --output <info>` | |

Source: https://github.com/JakeWharton/ProcessPhoenix.

To take a newer one: fetch the .aar from Maven Central, check it against the `.sha1` beside
it there, make the `.info` again, list `PhoenixActivity` in the app's manifest as the .aar's
own manifest does, and put the name, address and sum here and in `engine/android/BUILD.gn`.
The APK's build has no network, so the file is kept here.

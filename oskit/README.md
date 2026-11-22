# oskit

What an app asks of the OS itself, apart from its windows, as one Go module: one interface per
capability, with a Mac half (Objective-C through cgo), an Android half (Java through JNI), a
Linux half (the desktop portals over D-Bus) and a fake for tests.

| Package | What | State |
|---|---|---|
| `notify` | The system's notifications: show with buttons, remove, clicks, asking to be allowed | Mac, Android; Linux next |
| `secret` | The device key: the login keychain, a file sealed by the Android Keystore, the Secret portal | Mac, Android, Linux |
| `verify` | A check of the person: Touch ID, a fingerprint, the screen lock | to come |
| `open` | Open a file, reveal it, the default browser, links the OS hands the app | Mac, Android, Linux |
| `clip` | Copy a secret so clipboard history leaves it out | Mac, Android; Linux's app writes its clipboard itself |

It imports nothing of the app that uses it. Project B's core takes it with
`replace github.com/sudiptadeb/oskit => ../oskit`; it can leave as a repo of its own with
`git subtree split -P oskit`.

## Threading

Nothing needs calling on a particular thread. Answers and events (a click, an answer to Ask)
never run inside the call that asked; the Mac's and Android's halves bring them to the OS's
main thread, then through `oskit.Post`, which an app with a main thread of its own sets once:

```go
oskit.SetPost(func(f func()) { mainThread.Post(f) })
```

Each call that answers later has a twin that waits, for code off the main thread:

```go
ok, err := notify.AskWait(ctx, n) // the context's error when ctx ends first
```

## Using notify

```go
n := notify.New() // notify.NewFake(true) in tests
n.OnClick(func(id, action string) { ... }) // set once, as the app starts; early clicks wait for it
n.Show(notify.Note{ID: "42", Title: "Build done", Body: "All green",
    Actions: []notify.Action{{ID: "open", Label: "Open"}}})
n.Remove("42")
```

- **Mac:** UNUserNotificationCenter; oskit becomes its delegate when OnClick is set, so a
  click on a note another part of the app posted comes through OnClick too, by its request's
  identifier. A bare binary has no bundle and shows nothing.
- **Android:** the app builds `android/java` into its APK and keeps it with
  `android/proguard.flags`; its activity calls `OsKit.attach(this)` as it is made, its JNI code
  calls `OskitAttach(env)` once from a Java thread, and it hands on its intents
  (`OsKit.onIntent`) and permission answers (`OsKit.onRequestPermissionsResult`). Notes go in
  a channel of their own, "Notifications".

## Tests

`go test ./...` runs on the fakes on any OS. The Android half cross-compiles with the NDK:

```sh
CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=$NDK/bin/aarch64-linux-android29-clang go build ./...
```

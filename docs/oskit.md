# oskit: the OS's capabilities as one Go module

Status: agreed 2026-11-22; notify, secret and open built on the Mac, Android and Linux.

On Android, open does what Chromium does (his call, 2026-11-22): a finished download's path is
its copy's content:// address in the shared Downloads, which OpenFile hands to the app the
person picks; Reveal shows the phone's Downloads.

oskit is what an app asks of the OS itself, apart from its windows: notifications, a device
key, a check of the person, opening files and links, and the clipboard's secret copy. It is a
Go module of its own, in `oskit/` at the top of this repo, so it can leave as its own repo
later with `git subtree split -P oskit`. Until then it lives here and nothing is published.

## Layout

```
oskit/
  go.mod                 module github.com/sudiptadeb/oskit; needs only godbus (Linux)
  README.md              what it is, the threading rule, how an app wires it in
  oskit.go               Post (the threading rule) and the shared Question helper
  notify/   notify.go  notify_darwin.go/.m  notify_android.go  notify_linux.go  fake.go
  secret/   secret.go  secret_darwin.go     secret_android.go  secret_linux.go  fake.go
  verify/   verify.go  verify_darwin.go/.m  verify_android.go  verify_linux.go  fake.go
  open/     open.go    open_darwin.go/.m    open_android.go    open_linux.go    fake.go
  clip/     clip.go    clip_darwin.go/.m    clip_android.go    clip_linux.go    fake.go
  internal/jni/          the Android side's JNI: the VM, the app's class loader, calls
  internal/portal/       the Linux side's D-Bus: the session bus, a portal Request's Response
  android/java/io/github/sudiptadeb/oskit/   OsKit.java, Notify.java, Secret.java, ...
  demo/                  one small app per OS that shows each capability, for deskrig
```

- No import of core, ever. core imports oskit through `replace github.com/sudiptadeb/oskit =>
  ../oskit` in core/go.mod, as it does browserdata.
- Each package is an interface, `New()` for this OS, and `fake.go`: a fake that records calls
  and answers when a test says, built on every OS with no cgo. Tests in core and in oskit run on
  the fakes.
- The Mac's halves are Objective-C beside the Go through cgo, as core/system/platform is now.
- Android's halves call Java through JNI from Go itself (`internal/jni`), not through
  go_table.c. The Java lives in `oskit/android/java`, and engine/android/BUILD.gn builds it as an
  `android_library("oskit_java")` from `//project_b/oskit/android/java`, as it builds amarit's
  Java, kept from R8 by `oskit/android/proguard.flags`. The activity calls
  `OsKit.attach(activity)` as it is made, and go_table.c calls Go's `OskitAttach(env)` once on
  a Java thread, where each package finds its class with the app's class loader and registers
  its natives. A tap on a note comes back as an intent, which the activity hands to
  `OsKit.onIntent`.
- Linux's halves are pure Go over godbus and the desktop portals, so they work inside the
  Flatpak.

## The threading rule

oskit never needs to be called on any particular thread, and calls each answer and event
through `oskit.Post(func())`, which the app sets once at start: core sets it to its main
thread's poster. Unset, an answer runs on oskit's own thread. An answer never runs inside the
call that asked.

## Waiting for an answer

Each call that answers later has a twin that waits for the answer, for code that would rather
block: it runs on a goroutine, never on the main thread, as the OS's sheet needs the main
thread to show. The twin takes a context; when it ends, the question is taken back (the
person's sheet goes away) and the twin returns the context's error.

```go
v, err := verify.Wait(ctx, v, check)     // Verified, Denied or NoScreenLock
ok, err := notify.AskWait(ctx, n)
ok, err := open.AskDefaultBrowserWait(ctx, o)
key, err := secret.DeviceKey(ctx, "Project B", true) // waits already: it has no callback form
```

Each twin is the callback form with `oskit.Wait`, one generic helper:
`func Wait[T any](ctx context.Context, ask func(answered func(T)) (cancel func())) (T, error)`.
Its answer only fills a channel the waiting goroutine reads, so a Post to a busy main thread
never blocks it, and the main thread never waits.

## API

```go
package notify
type Note struct {
    ID, Title, Subtitle, Body string
    Actions []Action                // buttons: Mac and Android up to 3, Linux the portal's
}
type Action struct{ ID, Label string }
type Notifier interface {
    Allowed() bool
    Ask(answered func(allowed bool))   // Android 13+ asks the person; Mac and Linux: allowed
    Show(n Note)                       // replaces a shown note with the same ID
    Remove(id string)
    OnClick(clicked func(id, action string)) // action "" is a click on the note itself
}

package secret
var ErrNone = errors.New("secret: no device key")
type Store interface {
    // DeviceKey is the 32-byte key this device keeps for the app under name: the login
    // keychain on the Mac, a file wrapped by the Android Keystore, the Secret portal on
    // Linux. create makes one when there is none; else ErrNone. It may wait on the person
    // (the Mac's keychain, a Linux keyring to unlock, up to 2 minutes), so call it off the
    // main thread. Linux's portal makes its secret on the first call, so create changes
    // nothing there.
    DeviceKey(ctx context.Context, name string, create bool) ([]byte, error)
}

package verify
type Check struct{ Reason, Title, Subtitle string }
type Verdict int // Verified, Denied, NoScreenLock
type Verifier interface {
    Person(c Check, answered func(Verdict)) (cancel func())
}

package open
type Opener interface {
    OpenFile(path string)
    Reveal(path string)
    IsDefaultBrowser() bool
    AskDefaultBrowser(answered func(isDefault bool))
    OnOpenURLs(opened func(urls []string))
}

package clip
type Clipboard interface{ CopySecret(text string) }
```

The names and meanings are core/system's today, so the adapter is a line per method.
Show's `Actions` and OnClick's `action` are new: amarit's update note (Relaunch) is one of them
once amarit moves onto oskit/notify, and the Mac's one notification delegate stops routing
amarit's category by hand.

## What moves from core

| Now | Goes to |
|---|---|
| `system.Notification`, ShowNotification/RemoveNotification/OnNotificationClicked, NotificationsAllowed/AskNotifications; the Mac's UNUserNotificationCenter code; Android's ask for notifications | `notify` |
| `keys.Keychain` (keys/keychain_darwin.go), `DeviceKey.java`, cmd/lib's keys_android.go reader, Linux's secretportal_linux.go | `secret` |
| VerifyPerson, PersonCheck, Verdict; LAContext; BiometricPrompt | `verify` |
| OpenFile, RevealFile, Is/AskDefaultBrowser, OnOpenURLs; Linux's portal_linux.go OpenURI and mimeapps.list | `open` |
| CopySecret | `clip` |
| `system.Question` | `oskit` (internal use) |

Stays in core:

- `core/system`: the System interface the managers use, unchanged for them, and
  `core/system/platform` shrinks to an adapter onto oskit plus what is the app's own: the Dock
  colour, Full Disk Access, the crash report's section, DownloadsDir, and Linux's dark mode
  (WatchDark). mocksystem stays the managers' fake.
- `core/keys`: the profile key, profile.key, folder.key and the secrets table. cmd/lib's
  deviceKeyFor becomes `secret.New().DeviceKey("Project B", create)`.
- Downloads' own notifications (Downloads.java) stay with downloads; they are the engine's.

Not in oskit: the single instance (left out, his call), the profile importer (already its own
module, browserdata) and passkeys (its own module, separately).

## Linux's code

The Linux session's OpenURI, mimeapps.list and Secret-portal code goes straight into oskit, not
into core/system first: it is not on main yet, so landing it twice is churn. It writes
notify_linux.go (the Notification portal: AddNotification, ActionInvoked) to the interface
above, open_linux.go and secret_linux.go from its commits, and its fake-portal D-Bus test moves
to `internal/portal`. The portal has no subtitle, so Linux's notify puts Subtitle as the
body's first line; Allowed is always true and Ask answers true, as the person turns
notifications off in the desktop's settings. Which desktop this is (XDG_CURRENT_DESKTOP), for
the default-browser line's words, stays in core/system beside WatchDark: it is the app's copy,
not a capability.

Linux's verify answers NoScreenLock: no portal checks a person. The consequence is that our
own passkeys can't be made or used on Linux, as they need a check of the person. That is his
decision later, between three: no person check on Linux, the profile's own passphrase as the
check, or no passkeys on Linux.

Linux's clip puts the text on the clipboard as the engine does, with no secret mark: no portal
marks one. KDE's Klipper honours a password manager's hint (the `x-kde-passwordManagerHint`
type), which clip can set if the engine lets it offer an extra type later.

## Order

1. oskit module, `oskit.go`, `notify` with its fake and tests; core/system's notifications
   onto it on Mac and Android; Linux's notify from the Linux session. One branch, one release.
2. `secret`, 3. `verify`, 4. `open`, 5. `clip`, each its own branch onto the same module.
6. amarit's presenters onto oskit/notify (amarit's repo, after oskit has its own).

Each Android step changes the engine's Java and go_table.c, so it gets the full drive_android on
both emulators before release. A demo app per OS, driven by deskrig, comes with notify.

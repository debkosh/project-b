# Passkeys and security keys

How a page's passkey request (`navigator.credentials.create` and `.get` with `publicKey`)
reaches the person. The research behind it, with the options not taken yet, is in
[research/webauthn.md](research/webauthn.md).

## Decisions

- 2026-11-19, Sudipta, on the research's options: "do what you said". So: the Mac gets the
  security key steps (insert and touch, PIN, choose an account, errors) and phone as key
  (a QR code and Bluetooth) now. Touch ID and the Mac's keychain, iCloud Keychain, Android,
  and our own synced passkeys wait (research, options C, D and E).
- 2026-11-19, the ux review of this page, applied below: each step takes the keyboard (its
  first field, else its primary button, else Cancel); a PIN's errors sit under its field;
  one primary button, on the right, and none on the steps that only wait; the words, the
  account rows and the QR code's tile as they now read here; `example.com` in examples.

## What it covers

| | Mac | Android |
|---|---|---|
| USB security key, with or without a PIN | Yes | No (needs the system's approval of the app) |
| A phone's passkey, by QR code and Bluetooth | Yes | No |
| Touch ID, the Mac's keychain, iCloud Keychain | No (needs an Apple team) | |
| The phone's own passkeys | | No (needs the system's approval of the app) |

## How it flows

Like a page's other dialogs ([daily-driver.md](daily-driver.md)): the engine tells Go, Go words
it and sends it to every shell, the shell draws it over the active tab's page and sends back
what the person did, and Go tells the engine.

- **Engine** (`engine/browser/passkeys.{h,cc}`, Mac only): `PasskeyRequest`, our
  `AuthenticatorRequestClientDelegate`, from
  `ContentBrowserClient::GetWebAuthenticationRequestDelegate`. One request per call. It fires
  `PB_EVENT_DIALOG_OPENED` with kind `PB_DIALOG_PASSKEY` (its message the site's RP ID) and
  then `PB_EVENT_PASSKEY_STEP` with the `key` step when content has found what there is to
  use, `PB_EVENT_PASSKEY_STEP` again each time what the person must do changes, and
  `PB_EVENT_DIALOG_CLOSED` when the request ends any way: content drops the delegate. The id
  comes from the same counter as the other dialogs. Go answers with one new call,
  `answer_passkey(id, action, text)`. `PasskeyPolicy`, our `WebAuthenticationDelegate`,
  allows keys that remember their accounts (`SupportsResidentKeys`), which content refuses
  by default.
- **Which requests get a card.** Only one content asks a modal UI for
  (`UIPresentation::kModal`), from a tab. A sign-in field's passkey list (conditional
  mediation), an immediate request, and a test's virtual key with its UI off tell Go
  nothing and go as content's own delegate would: no PIN, the first account.
- **Go** (`core/app/passkeys.go`): keeps the request as a page dialog of kind `passkey`,
  replaced in place at each step, with the words and buttons for the step. It shows from its
  first step. It turns the phone's link into a QR code for the shells.
- **Shell** (`shell/src/ui/PasskeyCard.vue` inside `PageDialog.vue`, and
  `PasskeyCard.svelte` for the Svelte shells; what they share is `shell/src/ui/passkey.js`):
  draws the step and answers with `answerPasskey {id, action, text}`. A shell that doesn't
  know the kind shows the title and words; its `answerDialog` with no cancels the request.
- **Mock engine** (`core/mockengine/passkeys.go`): plays a request's steps, so the shell and
  Go are tested without Chromium. `/mock/fire` kind `passkey` (`create` or `get`, `nophone`
  after it for no phone as key) starts one, and `passkeyStep` (a step's name and what it
  carries, or `end`) brings the newest to a step; `passkeyAnswers` in `/mock/regions` is what
  Go told the engine.

### A step, engine to Go

`PB_EVENT_PASSKEY_STEP`: the id, the step's name, and a JSON object with what the step
carries. The engine's own vocabulary stays in the engine: Go gets these names only.

| Step | When | Carries |
|---|---|---|
| `key` | The request starts (`OnTransportAvailabilityEnumerated`), after Use a security key, and after Try again | `request`: `create` or `get`; `site`: the RP ID; `phone`: whether phone as key is there |
| `pin` | The key asks for its PIN (`CollectPIN`) | `reason`: `enter`, `set` or `change`; `error`: `""`, `wrong`, `uv-locked`, or for a new PIN the key's rules refuse `short`, `invalid` or `same`; `attempts` left; `minLength` |
| `touch` | The PIN was taken and the key wants a touch (`FinishCollectToken`), or a fingerprint retry (`OnRetryUserVerification`) | `attempts` for a fingerprint retry |
| `account` | More than one account for this site on the key (`SelectAccount`); one is taken without asking | `accounts`: `[{name, displayName}]` |
| `phone` | After Use a phone, and when Bluetooth changes while it shows | `link`: the `FIDO:/` text for the QR code; `bluetooth`: `on`, `off` or `denied`; `canTurnOn` |
| `phoneBusy` | The phone has the request (the first of the hybrid tunnel's events) | |
| `error` | A failure content lets us show (`DoesBlockRequestOnFailure` returns true) | `reason`: `timeout`, `not-registered`, `already-registered`, `pin-soft-blocked`, `pin-hard-blocked`, `key-removed`, `no-passkey-support`, `no-uv`, `no-large-blob`, `no-algorithm`, `full`, `phone-failed`, `other` |

`kUserConsentDenied`, `kWinUserCancelled`, `kNoPasskeys` and the enclave reasons don't block:
the request ends and the page hears `NotAllowedError`, as the person meant. Nor does any
failure once a phone is ready for the request: the person most likely said no on the phone.

A key that offers to add a fingerprint during the request (`StartBioEnrollment`) is told to
go on without: that is its maker's app's work.

### An answer, Go to engine

`answer_passkey(id, action, text)`:

| Action | Text | Does |
|---|---|---|
| `cancel` | | Ends the request; the page hears `NotAllowedError`, or after an error step that error |
| `pin` | The PIN, or for `set` and `change` the new PIN | `provide_pin_cb` |
| `account` | The index | The chosen response to `SelectAccount` |
| `phone` | | Shows the `phone` step; the first time, macOS asks for Bluetooth first (`request_ble_permission_callback`) and the step follows its answer |
| `key` | | Back to the `key` step |
| `bluetoothOn` | | `bluetooth_adapter_power_on_callback` |
| `bluetoothSettings` | | Opens System Settings at Privacy & Security, Bluetooth |
| `retry` | | `start_over_callback`, from an error; the `key` step follows |

For `change` content asks only for the new PIN: the key has just taken the current one at
an `enter` step, and content keeps it for the change. A PIN never goes to the timeline, the
log or the shell's state.

## The steps, as the person sees them

One card over the active tab's page, the design system's dialog, as a page's other dialogs;
the tabs and the toolbar still work. The site's host is in bold, as a sign-in's is. Go words
every step; the shell only draws. Each step takes the keyboard: its first field, else its
primary button, else Cancel. There is one primary button at most, on the right; the steps
that only wait (`key`, `touch`, `phone`, `phoneBusy`, `account`) have none, and Cancel is on
the right. The way to the other kind of key (Use a phone, Use a security key) is a quiet text
button on the left. Escape is Cancel.

| Step | Title | Words | Controls |
|---|---|---|---|
| `key`, create | Create a passkey | **example.com** wants to save a passkey on your security key. Plug it in and touch it. | Use a phone (when there); Cancel |
| `key`, get | Sign in with a passkey | **example.com** wants you to sign in with your security key. Plug it in and touch it. | Use a phone (when there); Cancel |
| `pin`, enter | Enter your security key's PIN | | PIN; Cancel, Next |
| `pin`, set | Set a PIN for your security key | **example.com** needs your key to have a PIN. At least 4 characters. | PIN, Confirm PIN; Cancel, Set PIN |
| `pin`, change | Change your security key's PIN | Your key needs a new PIN before it can be used. | New PIN, Confirm new PIN; Cancel, Change PIN |
| `touch` | Touch your security key | Touch your security key again to finish. After a fingerprint that didn't match: That fingerprint didn't match. Touch your security key again. 2 tries left. | Cancel |
| `account` | Choose an account | Your security key has 3 accounts for **example.com**. | One row per account; Cancel |
| `phone` | Use your phone | Scan this code with your phone's camera. | The QR code; a Bluetooth line when it isn't on; Use a security key; Cancel |
| `phoneBusy` | Check your phone | Follow the steps on your phone. | Cancel |
| `error` | (per reason, below) | | Close, Try again (where it can help); Close alone otherwise, the primary |

**A PIN's fields** are password fields, each with its label above it, as a sign-in's. What
is wrong shows under the field, in `.text-field-error`, never in the words, and typing in
the field takes it away:

| When | Under which field | Words |
|---|---|---|
| The key refused the PIN (`wrong`) | PIN, empty again | Wrong PIN. 3 tries left. (Wrong PIN. 1 try left before the key locks., at one) |
| The key's fingerprint reader locked (`uv-locked`) | PIN | Too many fingerprint tries. Enter your PIN instead. |
| Next or Enter with fewer than `minLength` characters | The first | At least 4 characters (the key's own minimum) |
| The two differ | The confirmation | The PINs don't match |
| The key's rules refuse a new PIN (`invalid`, `same`) | The first | A PIN can't have those characters. The new PIN has to differ from the current one. |

The primary button is always on; Enter presses it. With something wrong nothing is sent,
and the keyboard goes to the field it is under.

**An account's row** is a button, the card's full width and at least 48px high: the display
name, with the name under it as `.secondary`, or the name alone when there is no display
name. Each is one line, cut with an ellipsis, the whole in the tooltip. Hover is
`--accent-tint`, focus `--focus-ring-inset`. Past about five rows the list scrolls inside
the card, Cancel still in view.

**Bluetooth**, 12px under the code, its button on the same row; nothing when it is on. Off
and `canTurnOn`: "Bluetooth is off." with Turn on Bluetooth. Off otherwise: "Turn on
Bluetooth on this Mac." `denied`: "Project B can't use Bluetooth. Allow it in System
Settings › Privacy & Security › Bluetooth." with Open System Settings, which the engine
opens.

Errors:

| Reason | Title | Words | Try again |
|---|---|---|---|
| `timeout` | That took too long | **example.com** stopped waiting. Try again when your key or phone is ready. | Yes |
| `not-registered` | No passkey on this key | This security key has no passkey for **example.com**. Try another key or a phone. | Yes |
| `already-registered` | Already saved | This security key already has a passkey for **example.com**. | No |
| `pin-soft-blocked` | Too many wrong PINs | Unplug your security key, plug it in again, and try again. | Yes |
| `pin-hard-blocked` | Your security key is locked | Too many wrong PINs. The key has to be reset before it can be used again, which erases its passkeys. | No |
| `key-removed` | Your security key was unplugged | Plug it in and try again. | Yes |
| `no-passkey-support`, `no-uv`, `no-large-blob`, `no-algorithm` | Use another key | This security key can't do what **example.com** asks. Try another key or a phone. | Yes |
| `full` | Your security key is full | It has no room for another passkey. Remove one with the key maker's app, or use another key. | No |
| `phone-failed` | Couldn't reach your phone | Make sure Bluetooth is on for this Mac and your phone, and try again. | Yes |
| `other` | Something went wrong | The request didn't finish. | Yes |

## What Go sends a shell

A passkey request is one entry of `state.dialogs`, kind `passkey`: `title`, `message`,
`host` (the site, for the bold), `ok` (the primary button's label, `""` for none), `cancel`
(the button beside it, `""` for none), and `passkey`:

| Key | |
|---|---|
| `step` | The step's name |
| `seq` | Greater for each step sent: a shell starts the card afresh when it changes |
| `primary` | What the primary button sends: `pin`, `retry` or `cancel` |
| `other` | The quiet button on the left: `{label, action}` |
| `pin` | `{labels, error, minLength, short, mismatch}`: the fields' labels, the words under the first as the step starts, and the shell's own two errors |
| `accounts` | `[{title, secondary}]`; a row sends `account` with its index |
| `code` | The QR code, a string of `0` and `1` for each row of modules |
| `bluetooth` | `{words, button: {label, action}}`, the line under the code |

## The QR code

The engine sends the phone's link as text (`FIDO:/` and digits, from
`device::cablev2::qr::Encode`); Go makes the code's modules (`rsc.io/qr`) and sends them to
the shell as rows of `0` and `1`, which the shell draws as an SVG with `role="img"` named
"QR code for your phone". No HTML crosses. The code is dark on a white tile whatever the
theme, never dimmed: 200px, centred, radius 4, its quiet zone of four modules inside the tile.

## Phone as key on the Mac

- `ConfigureDiscoveries` sets a fresh random QR key per request
  (`set_cable_data`), the tab's own network for the tunnel
  (`set_network_context_factory`: the asking frame's storage partition), and the event
  callback that tells us the phone has the request (`phoneBusy`). Phones aren't remembered
  between requests: each time is a new code.
- The app's Info.plist (`tools/package.sh`) gets `NSBluetoothAlwaysUsageDescription`:
  "Project B uses Bluetooth to check your phone is near when you sign in with a passkey on
  it." macOS asks the person the first time.
- Content turns hybrid off when the app isn't its own responsible process, as when started
  from a terminal; the `phone` flag in the `key` step is then false, and Use a phone isn't
  shown. Tests that need it start the app with `open`.

## Tests

- **Go and the mock** (`go test ./...` in `core/app/passkeys_test.go`, and
  `shell/e2e/passkeys.cjs` in `tools/test.sh mock`, on every desktop shell): every step's
  words and buttons, each answer reaching the engine, the request ending from the engine's
  side, another tab hiding the card and bringing it back, the QR modules, the keyboard's
  place at each step, the PIN field's errors, Escape, and a PIN leaving no trace.
- **Real engine**: the DevTools protocol's virtual authenticator
  (`WebAuthn.addVirtualAuthenticator`, CTAP2 over USB, with and without user verification)
  on a local test page: create, then get, through our steps; two accounts on one key to see
  the `account` step; an error (`already-registered`). The PIN step can't be driven by a
  virtual key; it is checked by hand with a real key.
- **By hand, once**: a real security key with a PIN, and an Android phone and an iPhone by
  QR code, on webauthn.io.
- The `webauthn` row of `core/webfeatures/registry.json` moves from `unknown` to what the Mac
  now gives, with its `why`.

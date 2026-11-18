# Passkeys and security keys

How a page's passkey request (`navigator.credentials.create` and `.get` with `publicKey`)
reaches the person. The research behind it, with the options not taken yet, is in
[research/webauthn.md](research/webauthn.md).

## Decisions

- 2026-11-19, Sudipta, on the research's options: "do what you said". So: the Mac gets the
  security key steps (insert and touch, PIN, choose an account, errors) and phone as key
  (a QR code and Bluetooth) now. Touch ID and the Mac's keychain, iCloud Keychain, Android,
  and our own synced passkeys wait (research, options C, D and E).

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

- **Engine** (`engine/browser/passkeys.{h,cc}`, Mac only): our
  `AuthenticatorRequestClientDelegate`, from
  `ContentBrowserClient::GetWebAuthenticationRequestDelegate`. One request per call. It fires
  `PB_EVENT_DIALOG_OPENED` with kind `PB_DIALOG_PASSKEY` when the request starts, a new
  event `PB_EVENT_PASSKEY_STEP` each time what the person must do changes, and
  `PB_EVENT_DIALOG_CLOSED` when the request ends any way. The id comes from the same counter
  as the other dialogs. Go answers with one new call, `answer_passkey(id, action, text)`.
- **Go** (`core/app/passkeys.go`): keeps the request as a page dialog of kind `passkey`,
  replaced in place at each step, with the words and buttons for the step. It turns the
  phone's link into a QR code for the shells.
- **Shell** (`shell/src/ui/PageDialog.vue`, or a component of its own beside it): draws the
  step. A shell that doesn't know the kind shows the title and words with Cancel.
- **Mock engine** (`core/mockengine`): plays a request's steps, so the shell and Go are
  tested without Chromium.

### A step, engine to Go

`PB_EVENT_PASSKEY_STEP`: the id, the step's name, and a JSON object with what the step
carries. The engine's own vocabulary stays in the engine: Go gets these names only.

| Step | When | Carries |
|---|---|---|
| `key` | The request starts, and after Use a security key | `request`: `create` or `get`; `site`: the RP ID; `phone`: whether phone as key is there |
| `pin` | The key asks for its PIN (`CollectPIN`) | `reason`: `enter`, `set` or `change`; `error`: `""`, `wrong` or `uv-locked`; `attempts` left; `minLength` |
| `touch` | The PIN was taken and the key wants a touch (`FinishCollectToken`), or a fingerprint retry (`OnRetryUserVerification`) | `attempts` for a fingerprint retry |
| `account` | More than one account for this site on the key (`SelectAccount`) | `accounts`: `[{name, displayName}]` |
| `phone` | After Use a phone | `link`: the `FIDO:/` text for the QR code; `bluetooth`: `on`, `off` or `denied`; `canTurnOn` |
| `phoneBusy` | The phone has the request (the hybrid tunnel is up) | |
| `error` | A failure content lets us show (`DoesBlockRequestOnFailure` returns true) | `reason`: `timeout`, `not-registered`, `already-registered`, `pin-soft-blocked`, `pin-hard-blocked`, `key-removed`, `no-passkey-support`, `no-uv`, `no-large-blob`, `no-algorithm`, `full`, `phone-failed`, `other` |

`kUserConsentDenied`, `kWinUserCancelled` and the enclave reasons don't block: the request ends
and the page hears `NotAllowedError`, as the person meant.

### An answer, Go to engine

`answer_passkey(id, action, text)`:

| Action | Text | Does |
|---|---|---|
| `cancel` | | Ends the request; the page hears `NotAllowedError` |
| `pin` | The PIN, or for `set` and `change` the new PIN | `provide_pin_cb` |
| `account` | The index | The chosen response to `SelectAccount` |
| `phone` | | Shows the `phone` step |
| `key` | | Back to the `key` step |
| `bluetoothOn` | | `bluetooth_adapter_power_on_callback` |
| `retry` | | `start_over_callback`, from an error |

For `change` the old PIN goes first: `text` is `old\nnew`. A PIN never goes to the timeline,
the log or the shell's state.

## The steps, as the person sees them

One card over the active tab's page, the design system's dialog, as a page's other dialogs;
the tabs and the toolbar still work. The site's host is in bold, as a sign-in's is. Go words
every step; the shell only draws.

| Step | Title | Words | Controls |
|---|---|---|---|
| `key`, create | Create a passkey | **npmjs.com** wants to save a passkey. Plug in your security key and touch it. | Use a phone (when there), Cancel |
| `key`, get | Sign in with a passkey | **npmjs.com** wants you to use your passkey. Plug in your security key and touch it. | Use a phone (when there), Cancel |
| `pin`, enter | Enter your security key's PIN | (`wrong`) Wrong PIN. 3 tries left. | PIN field; Cancel, Next |
| `pin`, set | Set a PIN for your security key | **npmjs.com** needs your key to have a PIN. At least 4 characters. | PIN and Confirm PIN; Cancel, Next |
| `pin`, change | Change your security key's PIN | Your key needs a new PIN before it can be used. | Current PIN, New PIN, Confirm; Cancel, Next |
| `touch` | Touch your security key | Touch your security key again to finish. | Cancel |
| `account` | Choose an account | Your security key has more than one account for **npmjs.com**. | One row per account: name, the display name under it; Cancel |
| `phone` | Use your phone | Scan this code with your phone's camera. | The QR code; a Bluetooth line when it isn't on; Use a security key, Cancel |
| `phoneBusy` | Check your phone | Follow the steps on your phone. | Cancel |
| `error` | (per reason, below) | | Try again (where it can help), Close |

Bluetooth, under the code: off and `canTurnOn`, "Bluetooth is off." with a Turn on Bluetooth
button; off otherwise, "Turn on Bluetooth on this Mac."; `denied`, "Project B isn't allowed
to use Bluetooth. Allow it in System Settings, Privacy & Security, Bluetooth."

Errors:

| Reason | Title | Words | Try again |
|---|---|---|---|
| `timeout` | That took too long | Nothing answered in time. | Yes |
| `not-registered` | Use another key | This security key has no passkey for **npmjs.com**. | Yes |
| `already-registered` | Already saved | This security key already has a passkey for **npmjs.com**. | No |
| `pin-soft-blocked` | Too many wrong PINs | Unplug your security key, plug it in again, and try again. | Yes |
| `pin-hard-blocked` | Your security key is locked | Too many wrong PINs. The key has to be reset before it can be used again, which erases its passkeys. | No |
| `key-removed` | The key was removed | Plug it in and try again. | Yes |
| `no-passkey-support`, `no-uv`, `no-large-blob`, `no-algorithm` | Use another key | This security key can't do what **npmjs.com** asks. Try another key or a phone. | Yes |
| `full` | Your security key is full | It has no room for another passkey. Remove one with the key maker's app, or use another key. | No |
| `phone-failed` | Couldn't reach your phone | Check that Bluetooth is on on both, and try again. | Yes |
| `other` | Something went wrong | The request didn't finish. | Yes |

The PIN field is a password field, the Next button is off until it has `minLength`
characters, and Enter presses Next. Escape is Cancel.

## The QR code

The engine sends the phone's link as text (`FIDO:/` and digits, from
`device::cablev2::qr::Encode`); Go makes the code's modules (a small Go QR library) and sends
them to the shell as rows of `0` and `1`, which the shell draws as an SVG. No HTML crosses.
The code is dark on light whatever the theme, with its quiet zone, about 200 px square.

## Phone as key on the Mac

- `ConfigureDiscoveries` sets a fresh random QR key per request
  (`set_cable_data`), the profile's network context for the tunnel
  (`set_network_context_factory`), and the event callback that tells us the phone has the
  request (`phoneBusy`). Phones aren't remembered between requests: each time is a new code.
- The app's Info.plist (`tools/package.sh`) gets `NSBluetoothAlwaysUsageDescription`:
  "Project B uses Bluetooth to check your phone is near when you sign in with a passkey on
  it." macOS asks the person the first time.
- Content turns hybrid off when the app isn't its own responsible process, as when started
  from a terminal; the `phone` flag in the `key` step is then false, and Use a phone isn't
  shown. Tests that need it start the app with `open`.

## Tests

- **Go and the mock** (`go test ./...`, `tools/test.sh mock`): every step's words and buttons,
  each answer reaching the engine, the request ending from the engine's side, another tab
  hiding the card and bringing it back, the QR modules, Escape.
- **Real engine**: the DevTools protocol's virtual authenticator
  (`WebAuthn.addVirtualAuthenticator`, CTAP2 over USB, with and without user verification)
  on a local test page: create, then get, through our steps; two accounts on one key to see
  the `account` step; an error (`already-registered`). The PIN step can't be driven by a
  virtual key; it is checked by hand with a real key.
- **By hand, once**: a real security key with a PIN, and an Android phone and an iPhone by
  QR code, on webauthn.io.
- The `webauthn` row of `core/webfeatures/registry.json` moves from `unknown` to what the Mac
  now gives, with its `why`.

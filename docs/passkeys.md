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
- 2026-11-19, Sudipta: our own passkeys, as part of the password manager, "stored locally and
  synced", unlocked per use by whatever the device has (Touch ID or the Mac password; a
  fingerprint, face or the screen lock on Android). The key stays in core, which signs; the
  engine checks the person was just verified, in one place, right before asking. The design
  is still to come, after the password manager's logins store.
- 2026-11-19, Sudipta: no Apple developer team for now. So option C is off, and our own
  passkeys use neither Apple's platform authenticator nor a keychain access group.
- 2026-11-19, Sudipta, yes to an AAGUID of our own. Project B's passkeys carry
  `81dc8063-9ac3-4b1f-ab00-9e8795c47c60`, fixed for good (sites name the provider by it),
  with no attestation.

## What it covers

| | Mac | Android |
|---|---|---|
| USB security key, with or without a PIN | Yes | No (needs the system's approval of the app) |
| A phone's passkey, by QR code and Bluetooth | Yes | No |
| Touch ID, the Mac's keychain, iCloud Keychain | No (needs an Apple team) | |
| The phone's own passkeys | | No (needs the system's approval of the app) |

## Limitations

- **No passkeys or security keys on Android.** A page's request is refused at once
  (`NotSupportedError`; webauthn.io says "No available authenticator supported any of the
  specified pubKeyCredParams algorithms"). Android answers a browser's request for another
  site only through Credential Manager or Play Services FIDO, and the providers behind them
  (Google Password Manager, other password managers) answer only browsers on their
  privileged lists, by package name and signing key. USB and NFC security keys go through
  the same gate. Project B (`dev.projectb.browser`) isn't on those lists. Lifting it: Google's
  approval, then registering content's `components/webauthn/android` in `BROWSER` mode
  (research, option D, 3 to 5 days); or passkeys of our own, which would still not reach the
  ones in the phone's password manager (option E).
- **No Touch ID, Mac keychain or iCloud Keychain passkeys on the Mac.** They need a signing
  team and Apple's browser entitlement; Project B signs with its own certificate and has no
  team, nor plans one (option C is off). A Mac signs in with a security key or a phone, and
  later with our own passkeys.
- **Phone as key needs the app opened from Finder** (or `open`): started from a terminal,
  the terminal is the responsible app, content keeps Bluetooth off, and Use a phone isn't
  offered.

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
- **Observing.** Content tells a delegate nothing of what there is to use, of a PIN or of a
  touch unless the delegate makes itself the request handler's observer in `StartObserving`
  (`FidoRequestHandlerBase::SetObserver`). Ours does, for a request with a card. Each step
  sent is on the timeline as `passkey.step` with its name, and a request with no card as
  `passkey.quiet`.
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
| The key's rules refuse a new PIN (`invalid`, `same`) | The first | A PIN can't have those characters. The new PIN must be different from your current one. |

The primary button is always on; Enter presses it. With something wrong nothing is sent,
and the keyboard goes to the field it is under.

**The accounts** sit in a card of their own, 16px under the words: a `--line` hairline round
it, radius 6, and a hairline between rows. Each row is a button, the card's full width and
48px high, with padding 8px 12px: the display name, with the name under it as `.secondary`,
or the name alone when there is no display name, or "No name" in `--muted` when the key has
neither. Each is one line, cut with an ellipsis, the whole in the tooltip. Hover is
`--accent-tint`, focus `--focus-ring-inset`. Past about five rows the list scrolls inside
the card, Cancel still in view.

**Bluetooth**, 12px under the code, its button on the same row; nothing when it is on. Off
and `canTurnOn`: "Bluetooth is off." with Turn on Bluetooth, a normal text button at 13px,
centred on the row. Off otherwise: "Turn on
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
| `accounts` | `[{title, secondary, unnamed}]`; a row sends `account` with its index |
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

## Our own passkeys

**A design to agree, not built.** Passkeys that Project B makes and keeps itself, in the
password manager, on the Mac and on Android. The interface with the store, sync and Profile
Lock was agreed with the passwords session on 2026-11-19. The decisions above (stored locally
and synced, unlocked per use, no Apple team, our AAGUID) hold. One change from where it
started: the device check is the OS's work, so with 0.3.0's layers ([layers.md](layers.md)) it is
core's, through `core/system`, not the engine's; it is still in one place, right before each
make and sign.

### What the person gets

- A site's "create a passkey" saves it in Project B. On the Mac, a security key or a phone is
  one press away on the same card. On Android ours is the only way, as the system's passkey
  providers don't answer us ([Limitations](#limitations)).
- Signing in shows the site's passkeys in Project B as account rows. Choosing one asks for
  Touch ID or the Mac's password, or the phone's fingerprint, face or screen lock, and signs in.
- Each use asks again. Nothing is remembered between uses.
- With sync on, a passkey made on the Mac signs in on the phone, and back.

### The item: core/passkeys

A passkey is a row in the profile's store, table `passkeys`:

| Column | On disk | |
|---|---|---|
| `id` | plain | The credential ID, 16 random bytes |
| `rp_id` | plain | The site's RP ID, as the page asked |
| `created` | plain | When it was made |
| `user_handle` | sealed | The site's user handle |
| `user_name`, `display_name` | sealed | As the site gave them |
| `key` | sealed | The P-256 private key (ES256) |

The site stays plain and who you are on it is sealed, as for a saved login
([password-autofill.md](password-autofill.md)). Every seal binds the row's `id` and `rp_id`. The
signature count is always 0, as for any synced passkey. Our AAGUID is put in by the engine;
core keeps nothing for it.

The calls, each with a context the caller cancels, and `asked` saying a person asked for it:

| Call | Returns |
|---|---|
| `Count(rpID, allow)` | How many passkeys the site has, of the credential IDs `allow` names when it names any. Plain columns only: never asks for the key |
| `List(ctx, asked, rpID, allow)` | `[{id, userName, displayName}]` |
| `Make(ctx, asked, rpID, user, exclude)` | The credential ID and the public key (COSE). A passkey with the same site and user handle is replaced; one of the `exclude` IDs answers `ErrExcluded` |
| `Sign(ctx, asked, id, data)` | A DER ES256 signature of `data` and the user handle |

Each answers success, `ErrCancelled` (the context ended), `ErrLocked` (the person pressed
Cancel on Profile Lock's unlock Dialog), `ErrNoScreenLock` (the device has nothing to check the
person with) or an error. Make and Sign check the person themselves, right before they make or
sign, through `system.System`; there is no way to sign without it.

- **Replace on Make** compares sealed user handles, so Make opens every row of the site first
  (it holds the key anyway); it can't be done in SQL.
- **Two devices offline** can each make one for the same site and account; after sync there are
  two rows with one user handle, and both work at the site, which kept both public keys. List
  shows the newer by `created`; the older stays in the store and in Settings, where the person
  can delete it. Nothing deletes it for them, as a delete syncs.
- **Settings' list** shows the account, which is sealed, so opening it is a person's call that
  raises the unlock Dialog when the profile is locked, and Delete sits behind the same unlock.

All use of the profile key is in one function of `core/passkeys`,
`sealed(ctx, asked, job)`. Today it is the store's `Wait` with `keys.Seal` and `keys.Open`. When
Profile Lock is built, the passwords session swaps its inside for the store's sealed job, which
waits on the unlock Dialog for a person's call and answers "locked" for any other. Until then a
profile is never locked, and `ErrLocked` is never answered.

**Sync.** A Mapping with Collection `passkeys`, the credential ID its key (not derived), the
private part every column in the clear, inside synckosh's sealing under the sync key. Read opens
the row and Write seals it, which is fine as sync stops while a profile is locked. A passkey
never changes once made, so Join takes the other when it is newer by `created`, and a delete
beats an older version, as sync's merger does already. Name is the `rp_id`, for Sync details, and the collection gets its words in Sync details'
maps (`SyncDetails.vue`: Passkeys, Passkey, passkeys), so a run reads "Passkey · example.com".

### The device check: core/system

A method on `system.System`, answering yes, no or no screen lock, async and cancellable as
`system.Question` is, through `PBSystemAnswered`:

- **Mac** (`platform_darwin.m`): LocalAuthentication's `LAPolicyDeviceOwnerAuthentication`,
  Touch ID, else the Mac's password, in macOS's own sheet. Its reason line reads "use a passkey
  for example.com" or "save a passkey for example.com" (macOS puts "Project B wants to" before
  it). No team needed.
- **Android** (`GoSystem.java`, with the activity it holds): BiometricPrompt titled "Use your
  passkey" or "Save your passkey" with the site under it, `setDeviceCredentialAllowed(true)` on
  Android 10 (our minimum), the `BIOMETRIC_STRONG | DEVICE_CREDENTIAL` authenticators from 11.
  No screen lock answers `ErrNoScreenLock`.
- **Mock** (`core/mocksystem`): answers as the test says.

### The authenticator: the engine

The engine is the request's side, as layers has it: it hands the request to Go and Go's answer
back, and keeps no keys.

- **Mac** (`engine/browser/passkeys.{h,cc}`): an authenticator of ours among the request's
  discoveries, beside the security key and the phone. It builds the authenticator data
  (`rpIdHash`, the flags UP and UV, BE and BS, count 0, and for a make the attested credential
  data with our AAGUID `81dc8063-9ac3-4b1f-ab00-9e8795c47c60` and the COSE key), with attestation
  `none`. Content checks the origin against the RP ID before any authenticator sees the request.
  The hook is content's own (Chromium 155): our request delegate's `CreatePlatformDiscoveries`
  returns a discovery of transport `kInternal` holding our authenticator, and `PasskeyPolicy`'s
  `IsUserVerifyingPlatformAuthenticatorAvailableOverride` says yes, so content asks for that
  transport and pages see a platform authenticator. `EmbedderControlsAuthenticatorDispatch`
  holds the request back from ours until the person presses Save or a row.
- **Android**: content hands WebAuthn only to Java
  (`RenderFrameHostImpl::GetWebAuthenticationService`), so we register a Java
  `blink.mojom.Authenticator` of our own. It checks the RP ID against the origin as Chromium's
  `components/webauthn/android` does, builds the same authenticator data, and passes the request
  to Go through the engine.
- When the page's request ends (timeout, Cancel, navigation, the tab closing) the engine tells
  Go, which cancels any call still waiting, the device check among them.
- New events from 84, and Go's answers through `PBEngineCalls`, numbered in `core/abi`.

### The card

Mocked 2026-11-19 and reviewed by ux that day (`pk-desktop.html`, `pk-phone.html` and
`pk-shots/` in the research mocks), its five changes applied.


The same card as a security key's, with new steps. Go words them; the shells draw them with
what they have.

| Step | When | Title | Words | Controls |
|---|---|---|---|---|
| `save` | A make | Create a passkey | Save a passkey for **example.com** in Project B? | Save, the primary; Use a security key or phone (the Mac only), quiet on the left; Cancel |
| `own` | A get, the site has passkeys here | Sign in to **example.com** | | One row per passkey, as the security key's accounts; Use a security key or phone (the Mac); Cancel |
| `ownLocked` | The Mac: a get, the site has passkeys here, the profile locked | Sign in to **example.com** | Your passkeys are locked. | Unlock…, the primary, which brings Profile Lock's Dialog and then `own`; Use a security key or phone; Cancel |

- A get for a site with no passkeys here goes as today, to the `key` step, on the Mac. On
  Android it is refused at once with `NotAllowedError`, as now.
- The `own` step always shows its rows, one row too: the press on the row is the person
  choosing, before the device check. The first row has the keyboard focus on open, so Enter
  picks it.
- A card with the quiet button on the left is wide enough (440px) for it, Cancel and the primary
  on one row.
- **While macOS or Android checks the person, the card steps aside**: their own sheet says what
  is asked, and two Cancels at once would be one too many. Go keeps a `check` step, which the
  shells draw as nothing; the system's Cancel or a failure ends the request as Cancel does.
- **On a phone a locked profile goes straight to Profile Lock's unlock Dialog** ("To use your
  passkey for **example.com**"), then `own`: the press on the site's button was the person
  asking, and there is nothing else to offer beside it.
- New errors: `no-screen-lock` "Set a screen lock first": "Project B checks it's you with your
  screen lock before using a passkey. Set one in your phone's Settings, then try again." (Close
  alone). `already-saved` "Already saved": "Project B already has a passkey for **example.com**
  for this account." (Close alone).
- The person saying no to Touch ID or the screen lock ends the request with `NotAllowedError`,
  as Cancel does.
- A sign-in field's passkey list (conditional mediation) stays quiet, as today.

### Seeing and deleting them

To agree: a Passkeys row under Privacy in Settings, beside Profile Lock which guards it, with
"4 passkeys" on its right, opens a list page with a back link, as Sites does: a row per passkey,
the site's mark, the site and the account, when it was made, and Delete…, which asks first. On a
phone Delete is a 40px trash button named for its passkey ("Delete passkey for github.com,
asha-rao"), and the date is left out. It becomes a section of Passwords when the
saved logins page is built. Deleting one deletes it on every synced device; the site keeps its
public key until the person removes it there, which the Delete Dialog says.

### Tests

- **Go**: make, list, sign and replace on a store; a signature that verifies against the public
  key; the authenticator data against the WebAuthn spec's examples; every seal's bound values; a
  cancel during the device check and during an unlock; `ErrNoScreenLock`; sync between two
  profiles through synckosh's server in process, and a delete.
- **The mock engine and the shells** (`shell/e2e/passkeys.cjs`): every new step's words and
  buttons, each answer reaching Go, the keyboard's place at each step, on every desktop shell
  and the Mobile shell. Every new action by name ([action-coverage.md](action-coverage.md)).
- **The real app on the Mac**: webauthn.io's flow and a local test page, create then sign in,
  in the VM, where deskrig types the Mac's password into macOS's sheet.
- **Android**: the same on emu-29 and emu-37 with a PIN set, the driver typing the PIN into
  BiometricPrompt; a phone with no screen lock gets `no-screen-lock`.
- **Sync**: made on the Mac, signs in on the phone, by hand once.

### The order of work

Small pieces, each to release when tested:

1. `core/passkeys`: the table, the three calls, `sealed`, Go tests.
2. Sync's Mapping.
3. The device check in `core/system`, Mac and mock.
4. The card's steps, the ABI and the mock engine, with the e2e checks.
5. The Mac engine's authenticator and the real-app test.
6. Android: the Java authenticator and BiometricPrompt, on the emulators.
7. Settings' Passkeys page.

About two weeks.

### Later

- Our AAGUID in the passkey provider list sites read, so they show the name Project B.
- Passkeys in a sign-in field's list, with saved logins' fill.
- Bringing passkeys in from other managers (FIDO's credential exchange format), his to decide.

# Passkeys and security keys (WebAuthn): research

Research for a discussion, 2026-11-19. Since agreed: A and B are built (docs/passkeys.md), and E
is the direction, as part of the password manager: stored locally and synced. Its design is
still to come. A reading of
Chromium 155.0.8059.41 and Project B 0.2.96.

## What WebAuthn is

A site asks the browser for a key pair made for that site, and later for a signature with
it. The private key never leaves the thing that holds it (the authenticator); the site keeps
only the public key. The browser puts the page's origin into what gets signed, so a key made
for `npmjs.com` can't be used by a look-alike site. That is why it resists phishing.

The page calls `navigator.credentials.create({publicKey})` to make a key and
`navigator.credentials.get({publicKey})` to sign in. The browser's job is to find an
authenticator, show the person what is happening (touch your key, enter its PIN, choose an
account, scan this code), and pass the answer back.

- **A passkey** is a key the authenticator remembers together with the account
  (a "discoverable" credential), so the person signs in without typing a name. Most passkeys
  are synced between a person's devices by a password manager.
- **A security key** is a small USB or NFC device (FIDO2/CTAP2). It may have a PIN, which the
  browser asks for and sends to the key.
- **A platform authenticator** is built into the device: Touch ID with the Mac's keychain or
  iCloud Keychain; on Android, the screen lock with Google Password Manager or another
  provider through Credential Manager.
- **Phone as key (hybrid)** lets a computer use a phone's passkey: the browser shows a QR
  code, the phone scans it, the two prove they are near each other over Bluetooth, and the
  messages then go through a relay server on the internet that the phone picks.

## What Project B does today

`ContentBrowserClient::GetWebAuthenticationRequestDelegate` is not overridden, so content
uses its own `DefaultAuthenticatorRequestClientDelegate`. The registry row is `webauthn`
(`core/webfeatures/registry.json`): Mac `unknown`, Android `quiet`.

**Mac.** Content's C++ authenticator runs with no UI:

| Path | What happens | Why |
|---|---|---|
| USB security key, no PIN | Should work: the key blinks, a touch answers. The first account is chosen when there are several | `FidoDiscoveryFactory::Create` makes a HID discovery by default; `SelectAccount` takes the first |
| USB key with a PIN, or a site that needs user verification | Fails | `SupportsPIN()` is false |
| NFC | None | Chromium has no NFC transport on the Mac, and Macs have no NFC reader |
| Touch ID / the Mac's keychain | None | `CreatePlatformDiscoveries()` is empty and no `TouchIdAuthenticatorConfig`; it also needs a keychain access group, which needs a signing team (below) |
| iCloud Keychain passkeys | None | Needs a window handed to the discovery factory and the Apple entitlement below |
| Phone as key | None | No `set_cable_data` (QR key), and the app's Info.plist has no Bluetooth usage string, which content checks before any Bluetooth call |

So on most sites today the request waits silently until it times out.

**Android.** Content doesn't run the authenticator in C++ on Android: it asks the Java side for
`blink.mojom.Authenticator` (`RenderFrameHostImpl::GetWebAuthenticationService`). Project B
registers no Java implementation, so the call is rejected at once.

**On webauthn.io (0.2.97, 2026-11-19, no security key plugged in):**

- Mac: Register with the default options shows nothing at all. The page sits waiting, still
  so at 30 seconds, and the log has one FIDO line: hybrid off for want of the Bluetooth
  metadata. The person is never told there is anything to do.
- Android: Register fails at once with "No available authenticator supported any of the
  specified pubKeyCredParams algorithms" (the site's text for the rejection).

## What content gives an embedder, and what it leaves to us

**Given (in `content/` and `device/fido/`, no patches needed):**

- The whole protocol: CTAP1/CTAP2, PIN protocol, the USB HID transport, the hybrid transport
  (`device/fido/cable`: QR data, BLE advert, the encrypted tunnel), the Mac Touch ID
  authenticator (`device/fido/mac`), the iCloud Keychain bridge (`device/fido/mac/icloud_keychain*`),
  origin and RP ID checks, client data.
- `AuthenticatorRequestClientDelegate`: the hooks a UI needs. `RegisterActionCallbacks`
  (cancel, start over, chose an account), `OnTransportAvailabilityEnumerated` (what's there:
  USB, Bluetooth on or off, platform credentials), `CollectPIN`, `FinishCollectToken`,
  `OnRetryUserVerification`, `SelectAccount`, `DoesBlockRequestOnFailure` (to show an error
  instead of failing at once), `ConfigureDiscoveries` (where hybrid, the Touch ID config and
  the window are switched on), `CreatePlatformDiscoveries`.
- `WebAuthenticationDelegate`: policy. `SupportsResidentKeys`, `IsFocused`,
  `IsUserVerifyingPlatformAuthenticatorAvailableOverride`, `GetTouchIdAuthenticatorConfig`
  (Mac), `ShouldPermitIndividualAttestation`, the RP ID override (for our own pages only).
- On Android, `components/webauthn/android` (not `//chrome`): `AuthenticatorImpl` and the
  Credential Manager and Play Services FIDO calls, with a mode switch (`WebauthnModeProvider`,
  `BROWSER`). The WebView registers it in `AwInterfaceRegistrar`, which is the pattern we'd copy.

**Left to us:**

- Every piece of UI: "insert your key", "touch it", the PIN field with retries left, choosing
  an account, the QR code, "turn on Bluetooth", errors (wrong key, already registered, PIN
  blocked). The full browser layer's version is about 35,000 lines in `chrome/browser/webauthn`
  and 11,000 in its dialogs; most of it is its own password manager and its cloud passkey
  service, which we don't want. Ours is a request delegate in the engine that sends steps to
  Go, and a sheet in the shell.
- The hybrid QR key (a random secret we keep per profile) and the network context for the
  tunnel.
- On the Mac: the signing identity and entitlements.
- On Android: Google's approval of the app.

## Platform rules

**Mac.**

- USB HID needs no entitlement: the app isn't sandboxed and doesn't use the hardened runtime.
- Phone as key needs `NSBluetoothAlwaysUsageDescription` in Info.plist and the person's yes
  to the Bluetooth prompt. Content refuses hybrid when the responsible process lacks it, and
  a run started from a terminal counts as the terminal's.
- Touch ID passkeys stored by the browser need a keychain access group
  (`<TeamID>.<bundle id>.webauthn`) in `keychain-access-groups`, and so a team ID.
- iCloud Keychain passkeys for any site need Apple's managed entitlement
  `com.apple.developer.web-browser.public-key-credential`, asked for by the account holder of
  an Apple Developer Program team, granted by Apple, carried in a provisioning profile
  ([Apple forum](https://developer.apple.com/forums/thread/852550)).
- Project B is signed by `tools/package.sh` with its own certificate (rcodesign, no team ID),
  so neither of the last two is possible today. Both need a paid team (Developer ID
  certificate) and a provisioning profile in the build; rcodesign can sign with both.

**Android.**

- Credential Manager (`android.credentials`, Android 14+) lets a browser set the page's
  origin on a request (`setOrigin`, permission `CREDENTIAL_MANAGER_SET_ORIGIN`). The provider
  then decides whether it trusts the browser: Google Password Manager answers only browsers on
  its privileged list, by package name and signing certificate, requested by
  [a form](https://developer.android.com/identity/sign-in/privileged-apps). Other providers
  (third-party password managers) keep lists of their own.
- Play Services FIDO (`Fido2PrivilegedApiClient`), which also carries USB and NFC security
  keys, is likewise allowlisted, with no published process.
- Digital Asset Links runs the other way: a site's `/.well-known/assetlinks.json` names the
  apps allowed to use its passkeys as themselves. That is for native apps, not for a
  browser; a browser's identity is its entry on the privileged lists.
- So without Google's yes, nothing on Android works through the system. The list entry is
  bound to the signing key, which stays fixed (the kosh).

## Options

| | What | Works for | Needs anyone's permission | Effort |
|---|---|---|---|---|
| A | Mac: our request delegate and a shell sheet: insert and touch, PIN, choose an account, errors | USB security keys | No | 3 to 4 days |
| B | Mac: phone as key on top of A: QR in the sheet, Bluetooth string and prompt | Any phone's passkeys (Android or iPhone) | No | 2 to 3 days |
| C | Mac: Touch ID and iCloud Keychain passkeys | The Mac's own passkeys | A paid Apple team and Apple's grant; the signing pipeline changes | 2 to 3 days of code, weeks of waiting |
| D | Android: register `components/webauthn/android` in BROWSER mode | Google Password Manager, security keys over USB and NFC, other providers | Google's privileged list; others' lists | 3 to 5 days, after approval |
| E | Our own passkeys: an authenticator inside the engine, keys in the profile sealed by the data key, unlocked by Touch ID or the screen lock, synced like the rest | Every site, on both platforms | No | 2 weeks or more |

Notes:

- A and B give the Mac security keys and phone passkeys with no one's permission. With B
  most people's passkeys (held on their phone) work on the Mac.
- C and D are forms to file now, as they cost nothing and take weeks; the code waits for the
  answer.
- E is the only route on Android that needs no one, but it replaces neither the person's
  existing passkeys (those are in their phone's password manager) nor security keys on Android
  (that needs our own USB and NFC CTAP code, another 1 to 2 weeks). It sits next to the
  passwords work and sync, and is a separate discussion.
- Testing: the DevTools protocol's virtual authenticator (`WebAuthn.addVirtualAuthenticator`)
  drives A's whole UI in the e2e tests without hardware; a real key and a phone are checked
  by hand once.

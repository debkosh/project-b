# Safari and iCloud Keychain password autofill: research notes

Written 2026-11-17 for the Project B password autofill design. Sources are cited inline.
Repos read at these commits:

- `apple/password-manager-resources` at `410bea26` (2026-11-16). Cited below as **PMR**.
- `WebKit/WebKit` at `0e0f8e6d` (2026-11-17). Sparse checkout, cited below as **WK**.
- The macOS 26.5 SDK (Xcode on this Mac), and the system files on this Mac (macOS 26.7.1, 25G241).

Safari's own form logic (field classification, the save prompt and the generator) is closed source,
in Safari and SafariShared. WebKit only provides hooks for it. Where these notes describe what Safari
does without an Apple document or source file behind it, they say so.

---

## 0. Headline facts for the design

1. **No public macOS API lets a browser read or fill iCloud Keychain or Passwords-app passwords
   for an arbitrary website.** `ASAuthorizationPasswordProvider` and `NSTextField.contentType` only
   return credentials for *the app's own* associated domains. Section 6 has the detail.
2. **Other browsers on macOS get iCloud passwords through Apple's "iCloud Passwords" browser
   extension.** It talks over Chrome native messaging to a system helper,
   `/System/Cryptexes/App/System/Library/CoreServices/PasswordManagerBrowserExtensionHelper.app`.
   That helper has a **parent launch constraint**: the launching process must be on Apple's signed
   allowlist (signing ID plus team ID), **or** hold the
   `com.apple.developer.web-browser.public-key-credential` entitlement. I read this from the
   helper's code signature on this Mac (section 6.4). Project B would need three things: working
   `nativeMessaging`, the extension installed, and either that entitlement or a place on Apple's list.
3. **macOS Sonoma added a system "AutoFill > Passwords…" menu item to text fields in any app,
   Chrome included.** It is user-driven and system-drawn, and it is not an API. Chromium already
   decides whether it appears (section 6.5).
4. **For Project B's own store, the macOS data protection keychain is the right place.** Use
   `kSecClassInternetPassword` items with `kSecAttrServer` and `kSecAttrAccount`, an optional
   `SecAccessControl` with `.userPresence` or `.biometryCurrentSet`, and
   `kSecUseDataProtectionKeychain = true`. It needs a provisioning profile, because the entitlements
   behind it must be authorized by one (section 7).
5. **Reusable open data:** PMR quirks (password rules, shared-credential groups, change-password URLs,
   2FA-appended sites, third-party credential iframes) and the WICG origin-bound SMS code format.
   PMR is MIT-licensed.

---

## 1. Field detection

### 1.1 Autocomplete tokens Apple documents

Apple's page "Enabling Password AutoFill on an HTML input element" lists four tokens: `username`,
`current-password`, `new-password` and `one-time-code`.
https://developer.apple.com/documentation/security/enabling-password-autofill-on-an-html-input-element

What it says about the heuristics:

- They "assume the user name and password inputs are on the same page". For a login form split
  across pages, tag the fields `username` and `current-password` so each can be filled on its own.
- They "assume that password and new password inputs always use secure text". A plain-text password
  field therefore needs `current-password` or `new-password`.
- Use `new-password` on both the new and the confirm field.

Safari **ignores `autocomplete="off"`**. Users control AutoFill in Safari Settings.
https://developer.apple.com/documentation/safari-developer-tools/autofill

Signals that same page says Safari uses:

- `<label>` and `aria-labelledby` association.
- Separate `<form>` elements per action, even when JS submits the form.
- `type`, `minlength`, `maxlength` and `pattern`.

It adds: "Apple archives copies of live webpages. Apple uses the layout and structure of the archived
pages to evaluate and improve the AutoFill experience". So the heuristics are tuned on a corpus.

### 1.2 WebKit side (open source)

- WK `Source/WebCore/html/Autofill.cpp` maps tokens to `AutofillFieldName`: `new-password`,
  `one-time-code`, `username`, and also `webauthn`, which is category Credential and sets
  `nonAutofillCredentialType`.
- WK `Source/WebCore/editing/cocoa/AutofillElements.cpp`, `computeAutofillElements()`, is the
  **structural username/password pairing** used on iOS:
  - **Focus on a password field:** the previous focusable text or email field becomes the username.
    The next password field becomes a "second password" (confirm), but only if it is empty.
    Comment in the code: "Always allow AutoFill in a password field, even if we fill information
    only into it."
  - **Focus on a text field:** if the next focusable field is a password field, they form a pair.
    An empty password field after that becomes the confirm field.
  - **A lone field** counts only if it is tagged `autocomplete=username` (or `webauthn`). This is
    the multi-page login case.
  - The search walks **focus order** with `FocusController::nextFocusableElement`. It does not walk
    DOM order. When the search reaches a site-isolated remote frame it stops and logs "not yet
    supported".
  - `autofill(username, password)` calls `setValueForUser`, then `didCompleteAutofill`, on each
    field, so the values are filled as if the user typed them.
- WK `Source/WebKit/WebProcess/WebPage/ios/WebPageIOS.mm`, around line 2880: the focused-element
  info sent to the UI process carries the following.
  - `acceptsAutofilledLoginCredentials` and `isAutofillableUsernameField`.
  - `autofillFieldName` and `representingPageURL`.
  - `hasEverBeenPasswordField`, which stays set after a show-password toggle turns the field into
    `type=text`.
- WK `Source/WebKit/UIProcess/ios/WKContentViewInteraction.mm`, `_autofillContext`, about line
  11796: this is the dictionary handed to UIKit and the QuickType bar. Its keys:
  - `_acceptsLoginCredentials`.
  - `_automaticPasswordKeyboard` plus `strongPasswordAdditionalContext`, for strong passwords.
  - `_WebViewURL`, which is the page URL the system matches credentials against.
  - `_page_id` and `_frame_id` for WebAuthn fields.

  `contentTypeFromFieldName` maps `username` and `webauthn` to `UITextContentTypeUsername`, and
  `one-time-code` to `UITextContentTypeOneTimeCode`.
- WebKit leaves the strong-password decision to the client (Safari). It asks the private input
  delegate `_webView:focusRequiresStrongPasswordAssistance:` (WKContentViewInteraction.mm, about
  line 8598). The new-password classification is Safari's own, closed code.
- WebKit does **not** know `passwordrules` as an attribute. It is absent from WK
  `Source/WebCore/html/HTMLAttributeNames.in`. Safari reads it as a plain DOM attribute.
- Safari's form code runs as a WebKit injected bundle in the web process and uses these hooks:
  - WK `Source/WebKit/WebProcess/InjectedBundle/API/c/WKBundlePageFormClient.h` has
    `textFieldDidBeginEditing`, `textDidChangeInTextField`, `willSendSubmitEvent`, `willSubmitForm`
    (with the form values), `didFocusTextField` and `didAssociateFormControls`. The last one fires
    for forms added dynamically.
  - WK `Source/WebKit/WebProcess/InjectedBundle/API/Cocoa/WKWebProcessPlugInNodeHandle.h` has
    `HTMLInputElementIsAutoFilled`, `…AndViewable`, `…AndObscured`, `HTMLInputElementIsUserEdited`
    and `isTextField`.
- WK `Source/WebCore/html/HTMLInputElement.h`, lines 254-276 and 1588-1676 in the .cpp:
  - Fields carry `setAutofilled` (`:autofill` pseudo-class), `setAutofilledAndViewable` and
    `setAutofilledAndObscured` (`WebKitAutofillAndObscured` pseudo-class; it also makes
    `isSecureField()` true).
  - They carry an `AutoFillButtonType`; `StrongPassword` is one value, tested by
    `hasAutofillStrongPasswordButton()`.
  - `AutofillVisibility` is Normal, Visible or Hidden.
  - These are the hooks for Safari's in-field key button and its yellow "Strong Password" overlay.

### 1.3 One-time codes

- **SMS (since iOS 12 and macOS Mojave).** "iOS and macOS use data detector heuristics to infer that
  an incoming message carries a security code … looking for words like code or passcode in proximity
  to the actual code string." `autocomplete="one-time-code"` makes it "work every time". Text Message
  Forwarding brings iPhone SMS to the Mac. Apple's position: it "does not alter the calculus around
  the security of second factor methods". WWDC18 session 204:
  https://developer.apple.com/videos/play/wwdc2018/204/
- **Timing.** "If the system can parse a security code from an SMS message, the QuickType bar shows
  the code for up to three minutes after it has been received. If a security code arrives while the
  text input view is selected, the system pushes the incoming code to the QuickType bar."
  https://developer.apple.com/documentation/security/about-the-password-autofill-workflow
- **Origin-bound SMS codes** (WICG spec, co-edited by Apple and Google; Safari has used it since
  iOS 14 and macOS Big Sur). https://wicg.github.io/sms-one-time-codes/
  - The **last line** of the message is `@example.com #747723`. Optionally it is followed by an
    embedded origin, ` @ecommerce.example`.
  - Exactly one space separates the values, and nothing may come before `@host`. Trailing text is
    ignored.
  - An embedded origin is honoured only when it is **same-site** with the top-level origin.
    Otherwise the user agent "should not assist".
  - The purpose is to mitigate phishing by "enabling User Agents to know what website the one-time
    code is intended for". In practice Safari offers the code only on a matching origin.
  - Background coverage: https://mjtsai.com/blog/2020/02/04/delivering-origin-bound-one-time-codes-over-sms
    and https://github.blog/2020-09-25-phishing-resistant-sms-autofill/
- **Mail codes (iOS 17 and macOS Sonoma).** Codes found in Apple Mail are offered in Safari as well.
  "Delete After Use" (shown as "Clean Up Automatically" in early builds) moves the message to the
  trash once its code has been autofilled.
  https://www.cultofmac.com/819421/ios-17-autofill-verification-codes-safari-mail-app/ and
  https://www.macrumors.com/how-to/auto-delete-verification-codes-messages-mail-ios/

  I found no Apple document on email-code parsing, or on whether a domain-bound format exists for
  email. **Unsure.**
- **TOTP.** The Passwords app (macOS 15, iOS 18) stores TOTP secrets and fills verification codes.
  Third-party providers can supply codes too: `prepareOneTimeCodeCredentialListForServiceIdentifiers:`
  and `ASOneTimeCodeCredential`, macOS 15 and iOS 18, in the SDK header
  `ASCredentialProviderViewController.h`. Sites should offer an `otpauth:` link as well as the QR
  code. WWDC24 session 10125: https://developer.apple.com/videos/play/wwdc2024/10125/
- **The OS keeps the code sources private.** No public API gives a browser the "From Messages" or
  "From Mail" codes. In UIKit and AppKit, a native field with content type `oneTimeCode` gets them
  from the system keyboard or AutoFill UI (`NSTextContentTypeOneTimeCode`, macOS 11+, SDK
  `AppKit/NSTextContent.h`).

  Chromium's `RenderWidgetHostViewCocoa` does not adopt the `NSTextContent` protocol; I grepped
  this tree. Whether AppKit would offer codes to a custom `NSTextInputClient` view that adopts
  `NSTextContent` is **untested**, and worth a spike.

---

## 2. Sign-in, sign-up and change-password

- Safari's classifier is closed. The open evidence:
  - Apple's tagging guidance (1.1).
  - WebKit's pairing: a second, empty password field after the first one means new plus confirm
    (1.2).
  - WWDC18 204: "Automatic Strong Passwords is compatible with change password forms if Autofill is
    able to detect username and password fields on the same screen. Note that the username field
    can be Read Only."
  - The Platform Security guide: heuristics decide "whether a password field is for creating a
    password", and the hint is `autocomplete="new-password"`.
    https://support.apple.com/guide/security/automatic-strong-passwords-secc84c811c4/web
- **`/.well-known/change-password`** (W3C webappsec):
  https://w3c.github.io/webappsec-change-password-url/
  - Servers redirect with 302, 303 or 307 plus `Location`. They "must not locate the actual change
    password page at the change password url".
  - Clients must follow redirects and must handle a 200 response, or a `<meta http-equiv=refresh>`.
  - The Passwords app's "Change Password" button uses it (WWDC24 10125).
  - For sites without it, PMR has `quirks/change-password-URLs.json`: **819** domain to URL entries.
    The README says to drop a site from that list once it adopts the well-known URL.
  - Spec issue: a client should check that the origin doesn't return 200 for every path
    ("Detecting the reliability of HTTP status codes"). I could not quote that draft. Chrome probes
    a nonsense well-known path first. That is my understanding, not verified here.
- The Passwords app shows the site name from OpenGraph metadata when a site provides it. Otherwise
  it guesses, then falls back to the domain (WWDC24 10125).

---

## 3. When Safari offers to save or update

- Safari's exact triggers are not documented for the web. The open evidence:
  - **WWDC18 204 (apps):**
    1. Infer a login scenario.
    2. Check that the app is associated with a domain ("Without this association saving passwords
       will not be offered").
    3. Find the username and password fields.
    4. "Detect that a sign in action has occurred". For apps the signal is the **fields being
       removed from the view hierarchy**.
    5. Decide between save and update, by whether the credential is new or changes an existing
       entry.

    The talk asks apps to clear the fields only *after* removing them, so AutoFill can still read
    the values.
  - **iOS 26.2 SDK, `ASSavePasswordRequest.h`.** This is what the OS now hands to third-party
    credential managers, and it names the system's save triggers. `ASSavePasswordRequestEvent` has
    three values:
    - `UserInitiated`: the user "expressly stated they wish to save".
    - `FormDidDisappear`: "a form is submitted or removed from the screen without prompting the
      user". Recommended: "save 'new' credentials, and ask the user if they wish to overwrite
      'updated' credentials".
    - `GeneratedPasswordFilled`: a generated password was put into a form not yet submitted.
      Recommended: save it as a **pending account**.

    These APIs are iOS and visionOS only; they are marked `API_UNAVAILABLE(macos)`.
  - **WebKit hooks Safari uses:** `willSendSubmitEvent` and `willSubmitForm` for real submits;
    `didAssociateFormControls` for forms added later; node-handle `IsUserEdited` (1.2). A fetch or
    XHR sign-in fires no submit. My inference is that Safari treats it like "form disappeared".
    **Unverified.**
- **Multi-step forms:** tag the fields `username` and `current-password` (1.1). WebKit accepts a
  lone `autocomplete=username` field (1.2). I found no Apple text on how Safari carries a step-one
  username into a step-two save. **Unsure.**
- **Pending generated passwords.** When Safari fills a generated password, it saves it before
  submit. I have seen this behaviour and it matches `GeneratedPasswordFilled` above, but I have no
  Safari-specific Apple doc for it. The same approach guards against losing a password when the
  submit fails.
- **Don't offer to save** credentials typed into third-party iframes known to ask for *other*
  services' credentials, such as bank logins inside Plaid. PMR
  `quirks/websites-that-ask-for-credentials-for-other-services-when-embedded-as-third-party.json`
  lists `aeropay.com`, `aerosync.com`, `plaid.com` and `schwab.com`.
- **Sites that append a 2FA code to the password** (`auth.redhat.com`, `etrade.com`, `usaa.com`):
  never auto-submit, and don't offer an update when the submitted password starts with the stored
  one. PMR `quirks/websites-that-append-2fa-to-password.json`.

---

## 4. Filling

- **Fill needs a user action.**
  - iOS: "When users select an item from the QuickType bar, the system asks them to authenticate
    using Face ID or Touch ID". Then the system moves first responder and fills all the related
    fields. https://developer.apple.com/documentation/security/about-the-password-autofill-workflow
  - Mac Safari: "Click in the user name field, then choose your user name". Touch ID, or the Mac
    login password, is needed to fill. The toggle is System Settings > Touch ID & Password > "Use
    Touch ID for autofilling passwords".
    https://support.apple.com/guide/safari/autofill-user-name-and-password-info-ibrwf71ba236/mac
    The login-password prompt goes back to Safari 12.1:
    https://talk.macpowerusers.com/t/solved-password-autofill-with-safari-12-1-always-prompts-for-user-password/11747
  - I found no current Apple source saying Safari fills on page load. The threat model for refusing
    to: CITP's "No boundaries for user identities". Trackers inject invisible login forms, let the
    manager autofill them, and read the email address.
    https://privacyinternational.org/examples-abuse/1932/no-boundaries-user-identities-web-trackers-exploit-browser-login-managers
- **Consent and isolation.** "Password AutoFill exposes no credential information to an app until a
  user consents". The credential lists "are drawn from or presented out of the app's process".
  https://support.apple.com/guide/security/password-autofill-security-sec7aefe77c3/web
- **iOS QuickType bar.** It appears only if at least one password is saved and AutoFill is on. The
  key icon opens all credentials; the bar itself shows matches for the associated domains (or, for
  web, `_WebViewURL`). WKWebView has had Password AutoFill since iOS 11.3, but "saving is not
  supported there" for apps' web views (WWDC18 204).
- **Iframes.** No Apple statement documents Safari's cross-origin iframe rule for passwords. The
  facts I found:
  - CVE-2013-5227: Safari before 6.1.1 and 7.0.1 let attackers "discover credentials via subframe
    form field autofill" across origins. https://tenable.com/cve/CVE-2013-5227
  - WebKit bug 310075 (fixed, 314590@main): `AutofillLoginCredentials` was broken for a focused
    field in a cross-origin iframe on iOS under Site Isolation. So filling *into* cross-origin
    frames is intended, at least on iOS. https://bugs.webkit.org/show_bug.cgi?id=310075
  - `AutofillElements` does not cross into remote frames to find the paired field (1.2).
  - Firefox stopped autofilling in cross-origin iframes but still offers to save from them
    (https://bugzilla.mozilla.org/show_bug.cgi?id=786276). Chromium's iframe policy doc covers
    addresses and payments, not passwords:
    https://chromium.googlesource.com/chromium/src.git/+/lkgr/docs/security/autofill-across-iframes.md
  - My reading, unverified for Safari: the fill is scoped to the origin of the frame holding the
    field, and the UI must show that origin.
- **Domain matching.** The system passes providers a list of service identifiers, most specific
  first, "[m.example.com, example.com]". SDK `ASCredentialProviderViewController.h`, doc comment on
  `prepareCredentialListForServiceIdentifiers:`.
- **Related domains:**
  - PMR `quirks/shared-credentials.json` has 133 entries: 67 `shared` groups, and 66 one-way
    `from`/`to` entries (46 of them `fromDomainsAreObsoleted`, for redirects such as
    discordapp.com to discord.com). Format in PMR `CONTRIBUTING.md`. Rule from the README: "should
    not be used as part of any user experience that releases user credentials to a website without
    the user's explicit review and consent … appropriate for allowing a credential saved for website
    A to appear on website B if the website the credential was saved for is clearly stated."
  - `quirks/shared-credentials-historical.json` is for suppressing reuse warnings only. The legacy
    `websites-with-shared-credential-backends` format is produced by
    `tools/convert-shared-credential-to-legacy-format.rb`.
  - PMR `quirks/apple-appIDs-to-domains-shared-credentials.json` (50 apps) is used by iOS 17.4+ and
    macOS 14.4+ to suggest credentials in apps that have no associated domains.
  - Whether Safari's own web matching reads `shared-credentials.json`: Apple runs the repo, but I
    found no Apple doc saying so. **Unsure.**
  - **Associated domains** are how apps get credentials. The app entitlement
    `com.apple.developer.associated-domains` lists `webcredentials:example.com`. The site serves
    `/.well-known/apple-app-site-association` listing the app IDs. Both sides together make "a
    trusted relationship", and the user still consents before anything is released.
    https://support.apple.com/guide/security/app-access-to-saved-passwords-sec8762eb992/web and
    https://developer.apple.com/documentation/xcode/supporting-associated-domains
- **HTTP pages.** Safari shows "Not Secure" on HTTP pages, with a stronger warning once a form field
  is focused (Safari 11.1 for login forms, all HTTP pages from 2019). https://support.apple.com/HT208672

---

## 5. Strong password generation

- **When it's offered.** On sign-up or change-password pages when iCloud Keychain is on, and on by
  default. With iCloud Keychain set up, Safari on Mac suggests one "when you first click the field".
  You can also click the key button and choose "Suggest New Password". The yellow field can be
  dismissed with "Don't Use".
  https://support.apple.com/guide/security/automatic-strong-passwords-secc84c811c4/web and the
  Safari Mac guide above.

  For apps it needs associated domains. The iCloud Passwords extension for other browsers does
  **not** generate passwords
  (https://discussions.apple.com/docs/DOC-250007756).
- **Format.** 20 characters: "one digit, one uppercase character, two hyphens, and 16 lowercase
  characters", **71 bits** of entropy (Platform Security guide). Example: `hupvEw-fodne1-qabjyg`.
  - Three groups of six, built from consonant-vowel-consonant "syllables". They are not real
    syllables, but they are easier to hold in short-term memory.
  - Mostly lowercase, so it is easy to type on awkward keyboards: "my colleague's game controller".
  - Hyphen is the one symbol, which satisfies "needs a special character" rules.
  - The length and mix were picked "to be compatible with a good mix of existing websites". The
    previous format had 69 bits.
  - Source: Ricky Mondello, who leads Apple's Authentication Experience team,
    https://rmondello.com/2024/10/07/apple-passwords-generated-strong-password-format/
  - Since iOS 16 and macOS Ventura, users can pick "No Special Characters", or edit the suggestion.
    https://www.macrumors.com/2022/06/07/macos-ventura-ios-16-edit-strong-passwords/
- **Password Rules language** (the `passwordrules` HTML attribute, and `UITextInputPasswordRules`).
  https://developer.apple.com/documentation/security/customizing-password-autofill-rules
  - Properties: `required:`, `allowed:`, `max-consecutive:`, `minlength:`, `maxlength:`.
  - Classes: `upper`, `lower`, `digit`, `special` (``-~!@#$%^&*_+=`|(){}[:;"'<>,.?]`` and space),
    `ascii-printable`, `unicode`, and custom `[abc]`.
  - Each `required:` property is a separate must-have. `required: upper, digit` means either one.
    Several `allowed:` properties form a union.
  - The default is `allowed: ascii-printable`.
  - **Rules weaker than length 12, or with fewer than two of upper/lower/digit, are ignored.**
  - Don't combine it with `pattern`. Putting the rule on one of the new and confirm fields is
    enough.
  - Apple hosts a validator: https://developer.apple.com/password-rules/
  - Reference parser: PMR `tools/PasswordRulesParser/PasswordRulesParser.js`. A Rust one is at
    https://github.com/1Password/password-rules-parser.
  - Per-site quirks: PMR `quirks/password-rules.json`, **460** domains. A key matches the domain
    and its subdomains unless `exact-domain-match-only: true`.
  - WHATWG proposal: https://github.com/whatwg/html/issues/3518
- **The generator itself is closed.** PMR ships only the parser. Apple's generator makes "the
  strongest password they can that fulfills these rules" (Platform Security guide).
- **What the OS hands providers.** On iOS 26.2+, `ASGeneratePasswordsRequest` gives a provider three
  separate rule strings: `passwordFieldPasswordRules`, `confirmPasswordFieldPasswordRules` and
  `passwordRulesFromQuirks`. That is the inputs a generator should combine (SDK
  `ASGeneratePasswordsRequest.h`; iOS only).

---

## 6. Credential provider extensions, and whether another browser can use iCloud passwords

### 6.1 Third-party managers plugging into the OS (public API)

- An extension subclasses `ASCredentialProviderViewController`, with the
  `com.apple.developer.authentication-services.autofill-credential-provider` entitlement on the
  extension and its app. Available on iOS 12+ and macOS 11+.
  https://developer.apple.com/documentation/authenticationservices/ascredentialproviderviewcontroller
- The provider pushes metadata to `ASCredentialIdentityStore`: service, user name, record ID and
  rank, "but not its password". The metadata is stored in the provider app's container. The system
  draws the suggestions and asks the extension for the secret only when one is picked:
  `provideCredentialWithoutUserInteractionForRequest:`, or `prepareInterfaceToProvideCredential…`
  when the extension must show UI.
  https://support.apple.com/guide/security/credential-provider-extensions-sec6319ac7b9/web
- Capability keys under `ASCredentialProviderExtensionCapabilities`: `ProvidesPasswords`,
  `ProvidesPasskeys`, `SupportsConditionalPasskeyRegistration`, `ProvidesOneTimeCodes` and
  `ProvidesTextToInsert` (for `prepareInterfaceForUserChoosingTextToInsert`, used by the AutoFill
  menu). Since iOS 18 and macOS 15 users can enable up to three providers (WWDC24 10125).
  `ASSettingsHelper.requestToTurnOnCredentialProviderExtension` (macOS 15+) asks the user to
  enable one.
- The OS sends providers updates: `reportUnusedPasswordCredentialForDomain:userName:` and the
  passkey report methods. Sites and apps send them through `ASCredentialDataManager` (macOS 26.2+).
- **Credential Exchange** (FIDO CXP, iOS 26 and macOS 26) uses `ASCredentialExportManager` and
  `ASCredentialImportManager`. The Passwords app can export passwords, passkeys and codes to another
  credential-manager app. The receiving app needs a credential provider extension with
  `SupportsCredentialExchange`. No file is written; the OS brokers the exchange.
  https://developer.apple.com/documentation/authenticationservices/ascredentialimportmanager and
  https://9to5mac.com/2025/06/13/ios-26-passkeys-password-transfer/

  **This is the only public, user-driven path from Apple Passwords into Project B's own store**,
  besides CSV export and import.
- **iOS 26.2 only** (`API_UNAVAILABLE(macos)` in the 26.5 SDK): the system passes save requests
  (`ASSavePasswordRequest`) and generate-password requests (`ASGeneratePasswordsRequest`) to
  providers, and apps can save with `ASCredentialDataManager.save(password:for:title:anchor:)`.

### 6.2 Could Project B be a *client* of system AutoFill for web pages on macOS? No public API

- **`ASAuthorizationController` with `ASAuthorizationPasswordProvider`** (macOS 10.15+) returns a
  credential only for the **app's associated domains**, which need the two-way webcredentials
  association. A browser can't claim arbitrary domains.
  https://support.apple.com/guide/security/app-access-to-saved-passwords-sec8762eb992/web
- **`NSTextField.contentType = .username/.password/.oneTimeCode`** (macOS 11+, WWDC20 10115 "AutoFill
  everywhere") gives the system AutoFill UI for native fields. It is also scoped to the app's
  domains, or to PMR's app ID list. It doesn't help with page content.
- **`SecRequestSharedWebCredential`** is deprecated in favour of `ASAuthorizationPasswordRequest`,
  with the same domain restriction (SDK `Security/SecSharedCredential.h`).
- **The browser entitlement covers passkeys only.** `com.apple.developer.web-browser.public-key-credential`
  plus `ASAuthorizationWebBrowserPublicKeyCredentialManager` (macOS 13.3+) and the
  `ASAuthorizationWebBrowser*` providers let a browser make WebAuthn requests "for any relying party
  identifier". Chrome 118+ uses this for iCloud Keychain passkeys on macOS 13.5+
  (https://developer.chrome.com/blog/passkeys-on-icloud-keychain). It doesn't return passwords.
  - Requirements to request it: the Account Holder role, Apple's review, http and https schemes in
    `Info.plist`, and on launch a URL field, search or bookmarks. The app must navigate "directly to
    the specified destination".
    https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.developer.web-browser.public-key-credential
  - Developers report the request process is slow and flaky (https://developer.apple.com/forums/thread/852550).
- **BrowserEngineKit** (`BEAutoFillTextSuggestion`, seen in WK `WKContentViewInteraction.mm`) gives
  autofill to alternative browser engines, but on iOS only.

### 6.3 How Chrome, Edge, Firefox and the rest get iCloud passwords: the iCloud Passwords extension

- Apple publishes the extension for Chromium stores and Firefox. Chrome Web Store ID
  `pejdijmoenmkgeppbflobdenhhabjlaj`. On macOS it needs Sonoma 14+. It fills user names, passwords
  and verification codes, and saves new passwords to iCloud Keychain.
  https://chromewebstore.google.com/detail/iCloud%20Passwords/pejdijmoenmkgeppbflobdenhhabjlaj
- **Mechanism** (read on this Mac):
  - Native messaging host `com.apple.passwordmanager`, stdio, at the path below. Its manifests are
    system-installed as symlinks:
    - `/Library/Google/Chrome/NativeMessagingHosts/com.apple.passwordmanager.json`, pointing into
      `/System/Cryptexes/App/Library/Google/Chrome/…`. `allowed_origins` is
      `chrome-extension://pejdijmoenmkgeppbflobdenhhabjlaj/` and
      `chrome-extension://mfbcdcnpokpoajjciilocoachedjkima/`.
    - `/Library/Application Support/Mozilla/NativeMessagingHosts/com.apple.passwordmanager.json`,
      with `allowed_extensions` `password-manager-firefox-extension@apple.com` and
      `apple-passwords-firefox-extension@apple.com`.
  - Host binary:
    `/System/Cryptexes/App/System/Library/CoreServices/PasswordManagerBrowserExtensionHelper.app`.
    Bundle ID `com.apple.PasswordManagerBrowserExtensionHelper`, version 27.0, needs macOS 26.4.
    Its private entitlements include `com.apple.authentication-services.access-credential-identities`
    and `com.apple.private.associated-domains`.
  - **User pairing:** after each browser launch the extension asks for a six-digit code shown by
    the Mac, and each fill needs Touch ID or the login password.
    https://discussions.apple.com/docs/DOC-250007756
- **Allowlist (PMR README, "How Apple Uses Web Browser Extension Distribution Information").** "As
  of macOS Sequoia version 15.5 and above, information in [`quirks/web-browser-extension-distribution-information.json`]
  is periodically ingested and re-packaged by Apple to limit the Native Messaging Host of the iCloud
  Passwords extension to only communicate with known web browsers … implemented by using launch
  environment constraints, which are signed into a helper binary distributed with macOS. This means
  that an operating system update is required for additional web browsers."
  - To be added, raise an issue or PR with: bundle ID, code-signing identifier, **team ID**, the
    extensions path relative to `~/Library`, and the store IDs.
  - Arc, Brave, Vivaldi, Opera, Helium, Comet, ChatGPT Atlas, ungoogled-chromium and many more are
    already listed. Reference is not.
  - Check a build with `codesign -d -vvvv …/PasswordManagerBrowserExtensionHelper`.
- **What the constraint actually allows** (dumped on this Mac, macOS 26.7.1). The *parent* launch
  constraint is an `$or-array`:
  - Its **first** alternative is
    `entitlements $query [com.apple.developer.web-browser.public-key-credential] == true`.
  - Then one `$and` per vendor of {signing-identifier in [...], team-identifier}: Google
    `EQHXZ8M8AV`, Microsoft `UBF8T346G9`, Mozilla `43AQ936H96`, The Browser Company, Brave, and
    about 40 more.

  So on this build, **any browser holding the web-browser public-key-credential entitlement passes,
  even if it isn't on the list.** This comes from the signed constraint dict, not from an Apple
  document, so it could change. My reading of query opcodes 1 and 5 as "key path" and "equals" is
  inferred.
- **Consequences for Project B, macOS:**
  1. Project B runs MV3 extensions behind a flag (`project-b/docs/extensions.md`).
     `nativeMessaging` isn't built. In Chromium it lives in `//chrome`
     (`chrome/browser/extensions/api/messaging`), and the host directories come from
     `chrome::DIR_NATIVE_MESSAGING` in `chrome/common/chrome_paths.cc`:
     `/Library/Google/Chrome/NativeMessagingHosts` for Chrome branding,
     `/Library/Application Support/Chromium/NativeMessagingHosts` for Chromium. Apple installs
     manifests only in Chrome's and Mozilla's system folders on this Mac, so Project B would have
     to look in Chrome's folder, or find the manifest some other way.
  2. The process that spawns the helper must pass the constraint, by entitlement or by signing ID
     plus team. With Go and the Chromium engine in separate processes, that has to be the signed
     process that actually launches the helper.
  3. The protocol between the extension and the helper is private and undocumented. Talking to the
     helper directly, without Apple's extension, is not API, and Apple can break it.
- **Windows:** the same extension works through iCloud for Windows. Not relevant here.

### 6.4 The system AutoFill menu (macOS Sonoma+)

- Right-click a text field in any app, Chrome included, then AutoFill > Passwords… (or Contacts).
  Authenticate, then search *all* saved passwords and codes and insert one into the focused field.
  https://sixcolors.com/post/2023/10/quick-tip-macos-sonoma-ios-17s-autofill-everywhere-is-a-lifesaver/
  and https://9to5mac.com/2024/04/24/ios-17-macos-sonoma-autofill-anywhere/
- It shows iCloud Keychain items, and providers that set `ProvidesTextToInsert`. It inserts text
  only. It doesn't match the page or fill username and password together (I found no source saying
  it does).
- **In Chromium**, AppKit adds these items when the first responder returns an `inputContext`.
  `content/app_shim_remote_cocoa/render_widget_host_view_cocoa.mm`, `-inputContext` at about line
  2610, returns nil:
  - for `TEXT_INPUT_TYPE_PASSWORD`;
  - for fields that have ever been a password field;
  - when `SetSupportsAutoFill(false)` is set while a native menu is up.

  So in a password field Chromium suppresses the menu, by design (IME privacy,
  https://crbug.com/40759416). It still appears in user name fields. Project B already has this
  behaviour as it stands.

---

## 7. Storage

### 7.1 iCloud Keychain (Apple Platform Security guide)

- **End to end:** passwords and passkeys sync "encrypted end-to-end so that Apple and other devices
  can't read their contents". There are two services, syncing and recovery.
  https://support.apple.com/guide/security/icloud-keychain-security-overview-sec1c89c6f3b/web
- **Sync:**
  - Each device makes a P-384 syncing identity. Devices join a "circle" when an existing member
    signs them in.
  - Items are encrypted for the circle members' keys. Items sync only when
    `kSecAttrSynchronizable` is set; Apple sets it for Safari data.
  - Conflicts resolve by "one or the other is chosen", which gives eventual consistency.

  https://support.apple.com/guide/security/secure-keychain-syncing-sec0a319b35f/web
- **At rest:**
  - The keychain is SQLite. Each item has a metadata key (the attributes; cached in the application
    processor) and a per-row secret key. Both are AES-256-GCM.
  - The secret key "always requires a round trip through the Secure Enclave".
  - ACLs are "evaluated inside the Secure Enclave". `securityd` enforces access groups from the
    app's entitlements.
  - `…ThisDeviceOnly` items never leave the device. `WhenPasscodeSetThisDeviceOnly` items are
    deleted if the passcode is removed.

  https://support.apple.com/guide/security/keychain-data-protection-secb0694df1a/web
- **Password Monitoring:**
  - Common leaked passwords are checked against a local list.
  - Everything else goes through private set intersection against about 1.5 billion leaked
    passwords in 2^15 buckets. Only a "15-bit prefix of a cryptographic hash" is revealed, with
    blinded P-256 points.
  - Checks run round-robin in fixed-size padded batches, unrelated to the number of passwords.

  https://support.apple.com/guide/security/password-monitoring-sec78e79fc3b/web
- **Safari's own items.** I believe they are internet-password items in an Apple-only access group
  (`com.apple.cfnetwork`), which third parties cannot read. That is not documented. **Unsure.**

### 7.2 A browser keeping its own passwords in the macOS keychain

- **Use the data protection keychain.** Set `kSecUseDataProtectionKeychain = true`, or
  `kSecAttrSynchronizable = true`, which also turns on iCloud sync.
  https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains
  - It is needed for biometric ACLs, Secure Enclave keys and iCloud sync.
  - The file-based keychain is "on the road to deprecation".
  - Access groups come from the code-signing entitlements, which "must be authorized by a
    provisioning profile". The app needs an app-like bundle.
  - It works only in a user login context, not from a daemon.
  - New in macOS 26.4: file keychains may depend on protected entropy files in `/var/db/SystemKeys`.
- **Item class `kSecClassInternetPassword`.**
  https://developer.apple.com/documentation/security/ksecclassinternetpassword
  - **Composite primary key:** `kSecAttrAccessGroup`, `kSecAttrAccount`,
    `kSecAttrAuthenticationType` (for example `kSecAttrAuthenticationTypeHTMLForm`),
    `kSecAttrPath`, `kSecAttrPort`, `kSecAttrProtocol` (`kSecAttrProtocolHTTPS`),
    `kSecAttrSecurityDomain`, `kSecAttrServer` and `kSecAttrSynchronizable`. Adding a duplicate
    returns `errSecDuplicateItem`, so an update is `SecItemUpdate`.
  - **Other attributes:** `kSecAttrCreationDate`, `kSecAttrModificationDate` (both set by the
    system), `kSecAttrLabel`, `kSecAttrDescription`, `kSecAttrComment`, `kSecAttrCreator`,
    `kSecAttrType`, `kSecAttrIsInvisible`, `kSecAttrIsNegative` (a "never save" marker) and
    `kSecAttrAccessible`.
  - Last-used time, the form signon realm and "saved for which origin" have no native attribute.
    They go in `kSecAttrComment`, or in Go's own database.
- **Gating reads behind Touch ID.**
  https://developer.apple.com/documentation/security/restricting-keychain-item-accessibility
  - Create the ACL with `SecAccessControlCreateWithFlags(NULL, kSecAttrAccessibleWhenUnlocked[ThisDeviceOnly], flags, &err)`
    and pass it as `kSecAttrAccessControl`.
  - Flags (SDK `Security/SecAccessControl.h`):
    - `kSecAccessControlUserPresence`: biometry or the device password, chosen by the system.
    - `kSecAccessControlBiometryAny`.
    - `kSecAccessControlBiometryCurrentSet`: the item becomes unreadable when fingerprints are
      added or removed. This "helps block an attacker who adds their own fingerprint".
    - `kSecAccessControlDevicePasscode`.
    - `kSecAccessControlCompanion`: Apple Watch.
    - `.or` and `.and`, and `kSecAccessControlApplicationPassword`.
  - To avoid one prompt per item, pass an evaluated `LAContext` with `kSecUseAuthenticationContext`.
- **What Chromium does today.** One AES key, "Chrome Safe Storage" or "Chromium Safe Storage", sits
  in the **file-based login keychain** (`components/os_crypt/common/keychain_password_mac.mm`). The
  passwords themselves are in the profile's SQLite. The reauth before filling is a separate
  `LAContext` check, `LAPolicyDeviceOwnerAuthentication…`
  (`chrome/browser/device_reauth/mac/authenticator_mac.mm`). It is not a keychain ACL.

---

## 8. Security traps Apple calls out or guards against

1. **Release only with consent, and out of process.** There is no credential exposure until the user
   picks one. The lists are drawn by the system (Platform Security, Password AutoFill security).
2. **Authenticate before filling.** Face ID or Touch ID on iOS; Touch ID or the login password in
   Mac Safari (section 4).
3. **Bind credentials to the domain.** Apps get them only through a two-way webcredentials
   association plus user consent. Cross-domain suggestions from `shared-credentials` must show which
   site the credential was saved for, and must never release it silently (PMR README).
4. **Bind codes to the origin.** Origin-bound SMS codes are honoured only on the matching origin,
   and an embedded origin only when it is same-site (WICG spec). SMS codes are shown for three
   minutes only.
5. **Subframes.** Apple fixed a cross-origin subframe autofill credential leak (CVE-2013-5227). PMR
   flags payment and aggregator iframes that ask for *other* services' credentials, so don't offer
   to save there.
6. **Invisible or injected forms.** Third-party scripts abuse auto-fill-on-load (CITP). Apple's
   design has no fill without a user gesture.
7. **2FA appended to the password.** Never auto-submit on those sites, and don't nag to update
   (PMR).
8. **Overly strict password rules lower strength.** Rules below 12 characters, or with fewer than
   two character classes, are ignored. "The more restrictions you have on a password, the higher the
   likelihood that it can be guessed" (Apple Password Rules doc).
9. **Insecure HTTP pages** get a "Not Secure" warning that grows stronger when a login field is
   focused.
10. **Allowlisting the native bridge.** Apple limits who may launch the Passwords helper using
    signed launch constraints (PMR README). The extension also asks for a per-launch six-digit
    pairing code.
11. **Biometric enrolment changes** should invalidate access (`biometryCurrentSet`). The Secure
    Enclave evaluates ACLs.
12. **Reuse and breach checks without leaking the password.** Use PSI with a 15-bit prefix, and
    fixed, padded batches so the traffic doesn't reveal how many passwords there are.
13. **Keep the password fields readable until they are gone.** WWDC18 tells apps to clear fields
    only after removing them, or AutoFill can't save. That matters for any save-on-disappear design.

---

## Open questions and unsure points (worth a spike or a question to Apple)

- Safari's exact save trigger for fetch and XHR sign-ins, and how it carries a step-one username into
  step two. Closed code; only indirect evidence (section 3).
- Safari's rule for filling *into* cross-origin iframes on the Mac (section 4).
- Whether AppKit offers "From Messages" or "From Mail" codes to a custom `NSTextInputClient` view
  that adopts `NSTextContent` with `contentType = .oneTimeCode`, which would mean Project B could
  pass through the web field's `autocomplete`. Untested (section 1.3).
- Whether the entitlement branch of the helper's launch constraint is intended, or stays, in later
  macOS builds (section 6.3). Apple's documented route is the allowlist PR.
- The extension-to-helper protocol is private. Apple's extension, through Project B's extension
  runtime, is the only supported way in.

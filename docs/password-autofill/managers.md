# How 1Password and Bitwarden do browser autofill

Research notes for the Project B password autofill design. Gathered 2026-11-17.

Sources:
- Bitwarden source is `github.com/bitwarden/clients` at commit `162f1faa` (main, 2026-11-17). Paths below are relative to `apps/browser/src/autofill/` unless they start with `libs/`.
- Bitwarden Android source is `github.com/bitwarden/android` (main).
- Chromium paths are from the Project B checkout (`chromium/src`).
- 1Password is closed source. Its facts come from its docs, its blog and staff forum posts. Where only forum posts back a claim, I say so.

---

## 0. Bitwarden's own design rules (worth copying)

Bitwarden now has design docs next to the code: `autofill.design.md`, `orchestrator.design.md` and `lifecycle.design.md`.
- **Fill only the "hot" tab**, meaning the active tab of the focused window. Never fill a background tab (`autofill.design.md`).
- **Resolve the target frame live at fill time, by identity.** Never fill from a snapshot taken when the page reported its fields. The frame may have navigated in between, which "would put a cipher chosen for the old page into whatever page now occupies that frame — a credential handed to the wrong origin" (`autofill.design.md`, "Autofill on page load").
- A fill has three outcomes: **Filled**, **Absent** (no field took it) and **Denied** (a check failed or the page navigated). A Denied fill also blocks the TOTP from being released (`autofill.design.md`, "Outcomes").
- Auto-submit (automated login) runs only on identity-provider hosts that an admin approved. Approval is re-checked at every step, because a redirect can move a frame off an approved host mid-login. "A frame is never trusted to declare itself part of an automated login" (`autofill.design.md`, "Automated login").

---

## 1. Field detection

### Bitwarden (open source)

**Collection.** `services/collect-autofill-content.service.ts` (2,283 lines) is the "collect page details" content script.
- It queries `input`, `textarea` and `select`. It skips these input types: hidden, submit, reset, button, image, file, search, url, date, time, datetime, datetime-local, week, color and range.
- It walks shadow roots (`shadow-host-hydration-tracker.ts`). It uses a MutationObserver and an IntersectionObserver to catch fields that SPAs add or reveal later.
- Each field gets an `opid` and a set of attributes: type, name, id, placeholder, autocomplete, `label-tag`, `label-aria`, `label-data`, `label-top`, `label-left` and `label-right`. The last three are text found near the field. It also records `viewable`, from `dom-element-visibility.service.ts`.
- Pages can mark fields with `data-bwignore` and `data-bwautofill`. These are honoured only if the user opts in, because the page controls them (`collect-autofill-content.service.ts` lines 112-190).

**Qualification.** `services/inline-menu-field-qualification.service.ts` (1,325 lines) decides what each field is. Keyword lists live in `services/autofill-constants.ts`.
- **Username field.** Type is text, email, number or tel. It must not be excluded or a TOTP field. It must match `UsernameFieldNames`, which merges an "unambiguous" list and an "ambiguous" list. `autocomplete` values `username`, `email` and `webauthn` count as login usernames, but only if the page has no visible new-password field.
- **Password field.** It is `type=password`, or a "password-like" text field. A text field counts if its autocomplete is `current-password` or `new-password`. This catches show-password toggles that flip the type to text. It also counts if its id, name, placeholder or label contains `password` or `passwd`, and none of `hint`, `captcha`, `findanything`, `forgot` or the TOTP names (`isLikePasswordField`, `valueIsLikePassword`).
- **Current versus new password.**
  - `isNewPasswordField`: a password field with `autocomplete=new-password`, or one that matches `AccountCreationFieldKeywords`. That list is: register, registration, create password, create a password, create an account, confirm password, new user, new email, new password, new-password, neuer benutzer, neues passwort, neue e-mail, pwdcheck, and similar.
  - `isCurrentPasswordField`: any other password field.
  - `isUpdateCurrentPasswordField`: a password field that matches `UpdatePasswordFieldKeywords` (update password, change password, current password, kennwort ändern).
- **TOTP field.** `autocomplete=one-time-code`, or a match on `TotpFieldNames`: 2facode, approvals_code, mfacode, onetimecode, onetimepassword, otc-code, otp-code, otpcode, second-factor, security_code, totp, totpcode, twofa, twofactor, twofactorcode, verificationcode, and others. A field is never TOTP if it matches `backup` or `recovery`. Those are recovery-code fields.
- **Ignored fields.** Search fields (matched by keyword on type, name, id or placeholder) and fields whose id, name or placeholder contains captcha, findanything or forgot.
- **Submit buttons.** Login buttons are matched by words: login, signin, submit, continue, next, verify. Change-password buttons by: change, save, savepassword, updatepassword, changepassword, resetpassword (`autofill-constants.ts` `SubmitLoginButtonNames`, `SubmitChangePasswordButtonNames`).

**Learned from users: site rules.** There is a newer "fill assist targeting rules" feed.
- `services/targeting-rules-data.service.ts` downloads a per-domain map of CSS selectors from a rules-feed URL. The feed is behind feature flag `FillAssistTargetingRules`, and the URL comes from policy or server config. `services/rule-qualification.service.ts` applies the map.
- A "webmapper" tool lets someone capture selectors for a host and path and export them in "map-the-web's forms.jsonc" shape (`webmapper/draft.ts`, `services/webmapper-draft.service.ts`).
- So Bitwarden is moving from pure heuristics to heuristics plus a curated per-site selector map. I did not find whether the public map is crowd-sourced or curated by staff.

### 1Password (closed source)

- No public technical description of its field detection. On 2026-09-03 the "Universal sign-in" post said 1Password shipped "new advanced field analysis" that decides "what 1Password can and should autofill". It also detects third-party sign-in buttons, such as Sign in with Google (https://1password.com/blog/introducing-universal-sign-in).
- In forum threads, 1Password staff have said they use "rules, heuristics — even machine learning", and asked for URLs "to include in our machine learning model's training data". These are forum posts found by search (for example https://1password.community/discussion/comment/269935), not official documentation. **Unsure** how much is ML today.
- Hidden fields are "generally not filled", except some hidden Identity fields that have a similar visible field (https://support.1password.com/browser-autofill-security/).

### Compare: Chromium

Chromium uses server-side crowd-sourced field-type predictions plus local heuristics. It already exposes its own field typing to Android as the `ua-autofill-hints` attribute (`components/android_autofill/browser/java/src/org/chromium/components/autofill/FormData.java`).

---

## 2. Telling sign-in from sign-up and change-password (Bitwarden)

All of this is in `services/inline-menu-field-qualification.service.ts`.
- **Explicit autocomplete wins.** If a field has `autocomplete=current-password`, it is a login field, unless the same form has a visible new-password field (`isPasswordFieldForLoginForm`, around line 394).
- **Counting fields.**
  - One username and one password on the page means a login.
  - A field with no form, on a page with more than one password field, means account creation.
  - More than one visible username field also means account creation.
  - Inside a form, more than one visible password field means not a login.
- **Keywords.** A field that matches `AccountCreationFieldKeywords` is never a login username.
- **Multi-step login.** A form with no password field and a single login-eligible text field is treated as step 1 of a multi-step login. When the page is ambiguous, a tie-breaker (`isAmbiguousFieldNonLogin`) decides.
- **Change-password.** A form is change-password when it has a current-password field that matches the update keywords, or both a current and a new password field. The save logic classes it as "password update" when it sees a password and a new password (see section 3).
- Account-creation forms get the generator in the inline menu instead of saved logins (`isFieldForAccountCreationForm`, line 261).

1Password says only that it "suggests strong passwords when you're changing a password or signing up" (https://support.1password.com/getting-started-browser/). How it tells the forms apart is not public.

---

## 3. When they offer to save or update

### Bitwarden

The code is `background/overlay-notifications.background.ts`, with field capture in `services/autofill-overlay-content.service.ts`.
1. While the user types, the content script sends the username, password and new password for the tab to the background. The background merges them across messages, so a username from step 1 survives into step 2 of a multi-step login (`modifyLoginCipherFormData`, around line 168).
2. **Network signal.** For tabs with fillable fields, it listens to `chrome.webRequest.onBeforeRequest` and `onCompleted`. The filter is that tab's origins, with request types `main_frame`, `sub_frame` and `xmlhttprequest`. Only POST, PUT and PATCH count as a submission.
   - A response outside 2xx cancels the prompt (`isInvalidResponseStatusCode`, `utils/index.ts` line 473).
   - A redirect after a POST also counts.
   - This is how fetch and XHR logins in SPAs are caught.
3. **DOM signal.** It listens for clicks and keyups on buttons that look like submit buttons, and for form `submit` events, including on formless fields. A 1.5-second fallback timer then fires the prompt even if no matching request is seen (`notificationFallbackTimeout`).
4. The prompt waits until the tab's `status` is "complete", so it shows on the page after login.
5. **Deciding the prompt type.**
   - **Add** needs a username plus a password or new password.
   - **Change** needs a username plus a password (for an unsaved change), or a password plus a new password (a password update).
   - Cached usernames from earlier visits make forms look like "password only". The code handles that case explicitly (around line 571).
6. The user can turn off both "ask to add" and "ask to update", and can exclude domains.

### 1Password

- "1Password will automatically offer to save your login", with editable details and a vault picker (https://support.1password.com/getting-started-browser/).
- Universal sign-in fills "across however many steps it takes", and can submit forms, enter one-time codes and use passkeys (https://1password.com/blog/introducing-universal-sign-in).
- Auto-submit after a fill is **on by default**. It can be turned off per login or globally (https://support.1password.com/browser-autofill-security/).
- How it detects submission (network or DOM) is not public.

### For Project B

A browser sees the real signals directly: form submission, navigation commit, and fetch or XHR responses on the same origin. Chromium's password manager already does this with "provisional save" plus a success heuristic. The extension approach (webRequest plus a timer) is a workaround for not being the browser.

---

## 4. Filling: triggers, page load, iframes, domain matching

### Triggers

**Bitwarden** (https://bitwarden.com/help/auto-fill-browser/):
- An inline menu on fields ("Show autofill suggestions on form fields"), optionally shown only when the icon is clicked.
- The popup.
- The context menu.
- A keyboard shortcut, **Ctrl/Cmd+Shift+L**. Pressing it again cycles through logins.
- Autofill on page load.
- With several matches, the shortcut and page load "always autofill the last-used login".
- The inline menu also offers passkeys, TOTP codes and SSH keys.

**1Password**:
- It "will never Autofill without your input, even when there's only one suggested item available". It calls that input "an anti-phishing mechanism" (https://support.1password.com/browser-autofill-security/).
- Triggers are the inline menu and the Cmd+\ shortcut.
- On a Mac, Universal Autofill (**Cmd+\**) works in any app. It needs the Accessibility permission. If one login matches it fills directly, and if several match it opens Quick Access (https://support.1password.com/mac-universal-autofill/).

### Autofill on page load (Bitwarden)

- **Off by default.** The setting text says: "If a login form is detected, autofill when the web page loads." The warning under it says: "Compromised or untrusted websites can exploit autofill on page load." There is a "Learn more about risks" link (`apps/browser/src/_locales/en/messages.json` keys `enableAutoFillOnPageLoadDesc`, `experimentalFeature`, `learnMoreAboutAutofillOnPageLoadLinkText`).
- The help page says: "This feature is disabled by default because, while generally safe, compromised or untrusted websites could take advantage of this to steal credentials" (https://bitwarden.com/help/auto-fill-browser/).
- It can be set per login item, and an organization can force it on by policy (`autofillPageLoadPolicyActivated`).
- **Page load never fills TOTP.**
- **Page load is quietly refused in untrusted iframes** (`services/autofill.service.ts` around line 534: "Autofill on page load was blocked due to an untrusted iframe.").
- **Page load raises no unlock prompt.** It fills only for the active, unlocked account (`autofill.design.md`).

### Untrusted iframes (Bitwarden)

- The check is `inUntrustedIframe` (`services/autofill.service.ts` around line 1579). If the frame's URL is not the tab's URL, the frame's URL must itself match one of the login's saved URIs, using that URI's match rule and equivalent domains.
- On a manual fill into an untrusted iframe, the extension shows a `window.confirm`: "The form is hosted by a different domain than the URI of your saved login. Choose OK to autofill anyway, or Cancel to stop." The tip adds: "save this URI, $HOSTNAME$, to your Bitwarden login item" (`services/insert-autofill-content.service.ts` around line 109; strings `autofillIframeWarning`, `autofillIframeWarningTip`).
- `confirm()` is blocked in sandboxed iframes, which makes the fill abort. The code calls this intended (same file, line 101).
- **HTTP downgrade.** If the page is HTTP and the login has an HTTPS URI for the same host, the extension asks for confirmation: "Warning: This is an unsecured HTTP page… This Login was originally saved on a secure (HTTPS) page" (`requiresInsecurePageConfirmation`, around line 1604).
- **History.** Flashpoint reported in March 2023 that Bitwarden filled cross-origin iframes, including on page load. The issue had been known since Bitwarden's 2018 audit (BWN-01-001). Bitwarden first said cross-domain iframes must keep working, then added the trusted-iframe rule (https://flashpoint.io/blog/bitwarden-password-pilfering/, https://www.bleepingcomputer.com/news/security/bitwarden-flaw-can-let-hackers-steal-passwords-using-iframes/).

### Iframes (1Password)

- It "won't autofill a Login item in an iframe if that item's URL doesn't match the origin of the iframe". This can leave forms partly filled.
- Credit cards *are* filled across cross-origin iframes, because payment forms split fields across frames. They always show a confirmation that "cannot be hidden or covered by a malicious or compromised webpage" (https://support.1password.com/browser-autofill-security/).
- The inline menu is an iframe loaded from the extension bundle. The page cannot look inside it because of the same-origin policy. Messages use extension messaging, not DOM events (https://support.1password.com/1password-browser-security/).

### Domain matching (Bitwarden)

- Six match types are set per URI (`libs/common/src/vault/models/view/login-uri.view.ts` `matchesUri`; https://bitwarden.com/help/uri-match-detection/):
  - **Base domain** is the default. It compares the registrable domain (eTLD+1).
  - **Host** compares hostname and port. Bitwarden help recommends it.
  - **Starts with** is a string prefix of the full URL.
  - **Regular expression** is a JS `RegExp(uri, "i")`.
  - **Exact** is full string equality.
  - **Never** turns the URI off for matching.
- The UI warns: "'Starts with' is an advanced option with increased risk of exposing credentials", with the same warning for regex (`startsWithAdvancedOptionWarning`, `regExAdvancedOptionWarning`).
- **The base domain uses the Public Suffix List.** `Utils.getDomain` calls `tldts.parse(uri, { allowPrivateDomains: true })`, so it uses the private section too: `github.io`, `blogspot.com` and so on are each their own suffix. localhost and IPs match by hostname (`libs/common/src/platform/misc/utils.ts` line 483).
- **Hardcoded exception.** `DomainMatchBlacklist` stops a `google.com` login from matching `script.google.com`, where users host their own content (same file, line 74). This was likely the fix for Flashpoint's "prominent hosting environment" finding. **Unsure**: Flashpoint did not name the host.
- **Equivalent domains.** Users can link domains under Domain rules. Bitwarden also ships a global list in the server repo, `src/Core/Utilities/StaticStore.cs`. At today's main it has **22 groups**: Google, Apple, Microsoft, Amazon, PayPal, Atlassian, Twitter/X, Proton, and others. For example, Google is `youtube.com`, `google.com` and `gmail.com`. Equivalence does not apply to Exact matches. I remember the list as longer; it may have been pruned, but **I have not checked its history**.
- Equivalent domains feed the base-domain match *and* the iframe trust check.

### Domain matching (1Password)

It fills "only on the exact domains they're saved for" (https://1password.com/blog/clickjacking-what-it-means-for-1password-users). In practice it offers subdomains of the saved domain. Tóth found all 11 tested managers fill subdomains by default (see section 6). There is no public description of PSL use, but earlier reporting says it uses a "trusted domain list" to find a page's effective top-level domain. **Unsure** of the details.

### Compare: Chromium's password manager

Chromium is stricter. `components/password_manager/core/browser/password_form_filling.cc` (around lines 220-300) fills on page load only when the match is exact. It waits for the user to pick an account ("fill on account select") when:
- the match is PSL (same eTLD+1, different host),
- the match is affiliated (Digital Asset Links or affiliation service) or grouped,
- the frame has a cross-origin ancestor,
- the main frame is not secure,
- the form is not a sign-in form, or a password is already filled,
- the browser is in incognito or biometric gating is on.

On Android and iOS it **always waits for user input** (`wait_for_username = true`). Affiliation between sites uses `/.well-known/assetlinks.json` with `delegate_permission/common.get_login_creds` (https://developer.chrome.com/docs/identity/site-affiliation). This is the curated alternative to Bitwarden's equivalent-domains list.

---

## 5. Generated passwords and TOTP

**Bitwarden generator defaults** (`libs/tools/generator/core/src/data/default-password-generation-options.ts`):
- Length 14. Uppercase, lowercase and digits are on, with at least one of each. Special characters are **off**.
- `ambiguous: true`, which I read as "ambiguous characters allowed". **Unsure** of the flag's direction.
- Length bounds are 5 to 128.
- Passphrase: 6 words, `-` separator, no capitals, no number (`default-passphrase-generation-options.ts`).
- The inline menu offers a generated password on new-password fields (`InlineMenuFillTypes.PasswordGeneration`, `services/autofill.service.ts` around line 2819).

**Bitwarden TOTP**:
- The inline menu can fill TOTP into one-time-code fields.
- "By default, your TOTP will also be copied to the clipboard when a login is autofilled." This is the "Copy TOTP automatically" setting.
- Page load never fills TOTP (https://bitwarden.com/help/auto-fill-browser/).
- A Denied fill never releases a TOTP (`autofill.design.md`).

**1Password generator**:
- The default is a "Smart Password" that adapts to the site's rules. Those rules come from the field's HTML and from open databases such as Apple's `password-manager-resources`. The fallback is about 20 random characters.
- This comes only from staff forum posts (for example https://1password.community/discussion/143931/how-to-change-default-password-suggestion-on-chrome). **Unsure** of current exact defaults.
- A user can pick "Random Password", choose length and symbols, and set it as the default.

**1Password TOTP**:
- After username and password, 1Password "automatically fills your one-time password". Copying codes to the clipboard is an opt-in setting, "Copy One-Time Passwords" (https://support.1password.com/one-time-passwords/).
- Watchtower flags sites that support 2FA but have no code saved (https://support.1password.com/watchtower/).

**Watchtower (1Password)** flags:
- compromised websites (a breach reported after the password's last change),
- vulnerable passwords (seen in breaches),
- reused and weak passwords,
- "unsecured websites" (saved as http:// on a site that supports HTTPS),
- "two-factor available" and "passkeys available",
- expiring items.

Source: https://support.1password.com/watchtower/. That page does not say how breach checks are done; I did not verify k-anonymity.

---

## 6. Incidents and research, and what each teaches a browser

### 2014: "Password Managers: Attacks and Defenses" (Silver, Jana, Boneh, Chen, Jackson, USENIX Security '14)

Link: https://www.usenix.org/conference/usenixsecurity14/technical-sessions/presentation/silver
- **Threat.** A "coffee shop" network attacker injects JS into any HTTP page, broken-HTTPS page, or page with an XSS hole on the target site.
- **Sweep attacks.**
  - An *iframe sweep* uses invisible iframes to many login pages. Managers that fill on load hand over each password.
  - *Window sweeps* and *redirect sweeps* need no iframes.
  - The paper measured about 10 passwords a second.
- **Who filled on load then.** Chrome on all platforms, Firefox, Safari, LastPass, Norton and LastPass Tab. 1Password, Keeper and KeePass always required interaction.
- **Iframe policies then.** Chrome desktop required interaction in any iframe. Chrome for iOS and the Android browser never filled iframes. Firefox, Safari and Chrome for Android filled iframes of any origin.
- **Defenses.**
  1. Always require user interaction, "through trusted browser UI that JavaScript cannot interact with". Show the domain before filling. Refuse to fill on broken HTTPS. The paper prototyped this in Chrome by hardcoding `wait_for_username = true`.
  2. **"Secure filling."** Store the form action at save time. After filling, make the password unreadable by JS. Abort if the fields change. On submit, check the action's domain against the stored one.
- **Lesson for Project B.** No fill without a gesture in trusted browser UI. Show the domain. Never fill HTTP pages or bad certificates. Secure filling is something only a browser can build. Chromium has no such feature today: a filled password is readable by page JS.

### 2018-2023: Bitwarden iframe and subdomain issues (Flashpoint)

Link: https://flashpoint.io/blog/bitwarden-password-pilfering/
- Filling on load and on manual fill reached cross-origin iframes. The base-domain default let users of a shared hosting domain receive credentials for login pages on the same base domain.
- **Fix.** Trusted-iframe rule (iframe URL must match a saved URI), a confirm prompt on manual fill, page load blocked in untrusted frames, and a hardcoded block for `script.google.com`.
- **Lesson.** Decide trust per frame by its committed origin, not the tab's URL. Base domain is too loose for shared-hosting domains. The PSL private section covers some of them, not all.

### 2023: AutoSpill (Gangwal, Singh, Srivastava, IIIT Hyderabad; Black Hat Europe 2023, CODASPY)

Links: https://i.blackhat.com/EU-23/Presentations/EU-23-Gangwal-AutoSpill-Zero-Effort-Credential-Stealing.pdf, https://techcrunch.com/2023/12/06/your-mobile-password-manager-might-be-exposing-your-credentials/
- When an Android app shows a login page in WebView, password managers using the Android Autofill framework could fill the credentials into the *host app's* native fields as well as the web page. That leaks them to the app, without any JS injection.
- Tested: 1Password 7.9.4, LastPass, Enpass, Keeper and Keepass2Android, on Android 10-12. Google rated it P2/S2. 1Password committed to its own fix.
- A vendor fix described in coverage: "only suggest to autofill the WebView field after checking the WebDomain" (https://www.expressvpn.com/blog/our-response-to-autospill/).
- **Lesson.** On Android, every web field handed to the framework must carry its own `webDomain`. That should be the origin of the frame the field is in, kept apart from native views. Chromium's `FormData.java` already sets `setWebDomain(host)` on the form and `setWebDomain(field.mOrigin)` on each field.
- Bitwarden Android builds the fill URI from the **focused** node's `webDomain`, falling back to `androidapp://<package>` (`app/src/main/kotlin/com/x8bit/bitwarden/data/autofill/parser/AutofillParserImpl.kt`, `.../util/ViewNodeExtensions.kt` `website`).

### 2025: DOM-based extension clickjacking (Marek Tóth, DEF CON 33, August 2025)

Link: https://marektoth.com/blog/dom-based-extension-clickjacking/
- **The attack.** Page JS makes the extension's injected UI invisible but still clickable, then gets the user to click a decoy such as a cookie banner or a captcha. Methods:
  - `opacity:0` on the extension's root or on children in an open shadow root,
  - `opacity:0` on `<html>` or `<body>`, with a screenshot of the page as the background,
  - overlays with `pointer-events:none`,
  - the Popover API (top layer) placed above the extension UI.
- **What leaked.** Cards and personal data from any attacker site, with no bug needed. Logins and TOTP needed XSS or user content on the same domain or *any subdomain*, because all 11 managers fill subdomains by default. Passkeys were hit too, via "signed assertion hijacking" when the server does not bind challenges to a session.
- **Scope.** 10 of 11 managers leaked credentials, 9 of 11 TOTP, 8 of 11 passkeys.
- **Status in Tóth's 2026-01-14 update.**
  - **Bitwarden fixed in 2025.8.2.** The current code uses a closed shadow root (`overlay/inline-menu/iframe-content/autofill-inline-menu-iframe-element.ts` line 14), `showPopover()` for the menu, and `checkPageRisks`, which closes the menu if the page is not opaque (`overlay/inline-menu/content/autofill-inline-menu-content.service.ts` around line 544).
  - **1Password still listed vulnerable (<=8.11.27.2)** to the parent-element and overlay methods.
  - **LastPass still listed vulnerable.** Dashlane, Keeper, NordPass, Proton Pass, RoboForm, Enpass, iCloud Passwords and others are fixed.
- **1Password's response.** Version 8.11.7 (2025-08-20) added an *optional* confirm-before-autofill alert. 1Password calls clickjacking "a browser-level limitation, not something a single browser extension can fully solve". It advises keeping autofill on (https://1password.com/blog/clickjacking-what-it-means-for-1password-users).
- Tóth: "Doesn't exist simple protection." He says the safe answer is UI the page cannot reach, such as a separate popup window.
- **Lesson.** This is the strongest argument for doing autofill in the browser. Project B's suggestion UI should be browser-drawn (a native popup or a widget outside the renderer), never DOM in the page. Chromium's autofill popup already is. Also:
  - check that the target field is visible and unobscured at fill time,
  - consider exact-host matching for fill-on-select,
  - keep cards behind an extra confirmation.

### Autofill-on-page-load harvesting

Bitwarden's own warning above covers it. The 2014 sweep attacks exploit it. Ad and analytics scripts have been seen reading auto-filled login fields, but I did not source a specific study in this pass.
- **Lesson.** Either fill on page load never, or only for an exact, secure, top-frame match with no cross-origin ancestor, as Chromium does. Never on mobile.

---

## 7. OS integration and what it means for Project B

### Android

- **Autofill framework.** A password manager registers an `AutofillService`. The app being filled exposes a `ViewStructure`. Web content exposes a virtual tree with `webDomain`, `htmlInfo` and autofill hints. Inline suggestions in the keyboard are available from Android 11 (https://developer.android.com/identity/autofill/autofill-services).
- **Compatibility mode** let services read the *accessibility* tree of browsers that do not implement the API. For browsers, it reads the URL from the address bar's resource ID, which is fragile and spoofable (same page, "Compatibility mode").
- **Chrome's change.**
  - Chrome on Android first handled only Google Password Manager natively. Third parties had to use compatibility mode, which caused "janky page scrolling" and "duplicate suggestions from Google and a third-party".
  - Chrome added the setting **Settings > Autofill Services > "Autofill using another service"**. It was announced for Chrome 131 (October 2024), then moved to **Chrome 135** (stable 2025-04-01).
  - When it is on, Chrome passes forms to Android Autofill natively. This is the same `components/android_autofill` path as WebView. When it is off, Chrome uses its built-in manager, which is the default.
  - Chrome stopped supporting compatibility mode **from Chrome 137**.
  - Chrome exposes the mode through a ContentProvider: `<package>.AutofillThirdPartyModeContentProvider`, path `autofill_third_party_mode`, column `autofill_third_party_state`, where 0 is off and 1 is on. Managers can deep-link to the setting with `ACTION_APPLICATION_PREFERENCES` plus `CATEGORY_APP_BROWSER`.
  - Sources: https://developers.googleblog.com/chrome-3p-autofill-services/, https://android-developers.googleblog.com/2025/02/chrome-3p-autofill-services-update.html, https://developer.android.com/identity/autofill/autofill-services.
  - Google's stated reason is a better user experience and better security for users of third-party services, and fewer compatibility-mode bugs.
- **What managers check.**
  - Bitwarden needs Chrome 135+. It hardcodes the browsers that get its "browser integration" toggle: Chrome stable and beta, Brave and Vivaldi (`app/src/main/kotlin/com/x8bit/bitwarden/data/autofill/model/browser/BrowserPackage.kt`).
  - Bitwarden warns that Edge, Opera and Samsung Internet "use a modified autofill logic" with a known risk of filling "an embedded or hidden iframe" (https://bitwarden.com/help/auto-fill-android/).
  - 1Password now needs Chrome 141+ or Brave 1.83+ with "Autofill using another service" on. It lists the browsers where it can create new Login items (https://support.1password.com/android-autofill/).
  - Android 17 Advanced Protection blocks Bitwarden's accessibility-based features (https://bitwarden.com/help/auto-fill-android/).
- **What this means for Project B on Android.**
  - If the shell routes forms through `components/android_autofill`, users of 1Password, Bitwarden and other managers get their manager in Project B with no extension system. That is the `AndroidAutofillProvider`, as WebView and Chrome's third-party mode use it.
  - Each field already carries its frame origin as `webDomain`, which is the AutoSpill lesson.
  - Project B would need a setting like Chrome's ("use our manager" versus "use another service"), so users do not get two sets of suggestions.
  - Optionally, Project B could expose the same ContentProvider so managers can read the mode. But managers hardcode browser package names, so Project B's toggle or deep link will not appear in their settings unless they add it. **Unsure** whether any manager treats unknown browser packages differently on the native path. Bitwarden's parser uses the focused node's `webDomain` whatever the package.
  - Passkeys from third-party providers go through Android Credential Manager on Android 14+, which is a separate integration from autofill.

### macOS

- **The system mechanism** is an AutoFill credential provider extension (`ASCredentialProviderViewController`, AuthenticationServices). The user enables it in System Settings > General > AutoFill & Passwords (on macOS 13 and later; https://support.apple.com/guide/security/sec6319ac7b9/web).
- **1Password** became a native macOS credential provider in 2026. Public beta came first, and staff announced general availability around August 2026. Safari's password fields then go through it, and the Safari extension still handles cards and addresses. This comes from 1Password community announcements (https://www.1password.community/announcements-52/macos-autofill-is-now-available-to-everyone-25254/index1.html). It is not yet on an official support page I could find.
- **Universal Autofill** (Cmd+\) is 1Password's own route. It fills any app through the Accessibility permission, including Terminal `sudo` (https://support.1password.com/mac-universal-autofill/). **Unsure** how it reads the current URL in a browser it does not have an extension in. It may only match by app, or ask "Allow Once / Always Allow".
- **Bitwarden on macOS.** The desktop repo has an Xcode project for an autofill extension (passwords and passkeys) under `apps/desktop/macos`. I found no official doc saying it ships as a system password provider. **Unsure**.
- **What this means for Project B on macOS.**
  - If Project B has no extension system, the 1Password and Bitwarden browser extensions will not run in it.
  - Users would rely on 1Password's Universal Autofill (Cmd+\), copy and paste, or Project B's own manager.
  - Credential provider extensions serve Safari and apps through AuthenticationServices. I found no public API that lets a third-party browser request *website passwords* from the user's chosen provider. The 2021 Firefox bug says Chrome did not support it either (https://bugzilla.mozilla.org/show_bug.cgi?id=1650212).
  - For *passkeys*, browsers can use AuthenticationServices' browser passkey APIs. **Unsure** whether third-party providers appear there for non-Safari browsers on current macOS. This needs a check before the design depends on it.

---

## Quick facts for the design discussion

1. Neither manager fills without a gesture by default. Bitwarden's page-load fill is opt-in, warned, never fills TOTP, and is refused in untrusted iframes. 1Password has no page-load fill. Chromium never fills on load on Android and iOS.
2. A frame is trusted when its own origin matches a saved URI. The tab's URL is not enough. Bitwarden asks to confirm in a mismatched frame. 1Password refuses logins there but allows cards, with a confirmation.
3. Bitwarden's base domain is eTLD+1 from the PSL, including the private section, plus a small global equivalent-domains list (22 groups today) and user rules. Chromium is stricter: it fills PSL and affiliated matches only after the user picks one.
4. Suggestion UI in the page DOM can be clickjacked. 1Password was still listed vulnerable in January 2026. A browser can draw it outside the page.
5. On Android, Chrome 135+ hands third-party managers native autofill through the same `android_autofill` component Project B can use. Compatibility mode ended in Chrome 137.

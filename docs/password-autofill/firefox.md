# How Firefox does password autofill

Research notes for the Project B password-autofill design. Gathered 2026-11-17.

**Source base.** Code was read from `mozilla-firefox/firefox` `main` at commit
`0e6902a13cee000e7891b2bb9d14ca39582c2533`. That is the GitHub mirror of mozilla-central,
so the same paths work on searchfox. Links use these two prefixes:

- **SF** = `https://searchfox.org/mozilla-central/source/`
- **GH** = `https://github.com/mozilla-firefox/firefox/blob/0e6902a13cee000e7891b2bb9d14ca39582c2533/`

Line numbers (`#Lnnn`) refer to that commit. Bug numbers point to `https://bugzilla.mozilla.org/show_bug.cgi?id=N`.

**The main files.** All paths are under `toolkit/components/passwordmgr/` unless noted.

| File | Role |
|---|---|
| `LoginManagerChild.sys.mjs` | Content-process actor: detects fields, fills, captures. Holds `LoginFormState`, `_getFormFields`, `_fillForm` and `_onFormSubmit`. |
| `LoginManagerParent.sys.mjs` | Parent-process actor: storage lookups, save/update decision (`showDoorhanger`), generated-password auto-save (`_onPasswordEditedOrGenerated`). |
| `LoginManager.shared.sys.mjs` | `Logic`, shared with Android and iOS: field type checks, new-password check, confirm-field finder. |
| `LoginHelper.sys.mjs` | Prefs, origin matching, dedupe, username and email heuristics, OS reauth. |
| `shared/LoginFormFactory.sys.mjs`, `toolkit/modules/FormLikeFactory.sys.mjs` | Group fields into "FormLike" objects. A formless page becomes one group rooted at the document. |
| `toolkit/components/satchel/FormHandlerChild.sys.mjs` | Detects submissions: submit event, navigation, form removal after fetch. |
| `shared/NewPasswordModel.sys.mjs` | Fathom model that scores new-password fields. |
| `toolkit/components/satchel/SignUpFormRuleset.sys.mjs`, `FormScenarios.sys.mjs` | Fathom model that scores sign-up forms. |
| `shared/PasswordGenerator.sys.mjs`, `shared/PasswordRulesParser.sys.mjs`, `PasswordRulesManager.sys.mjs` | Password generator and site rules. |
| `LoginRecipes.sys.mjs`, `LoginRelatedRealms.sys.mjs` | Per-site field-override recipes and shared-credential domain lists. Both come from Remote Settings. |
| `InsecurePasswordUtils.sys.mjs` | Decides whether a form is secure. Logs insecure-field warnings to the console. |
| `storage-json.sys.mjs`, `LoginStore.sys.mjs`, `crypto-SDR.sys.mjs` | Legacy store: `logins.json` encrypted with the NSS SDR key held in `key4.db`. |
| `storage-rust.sys.mjs`, `LoginStorageMigrator.sys.mjs` | New Rust store, `logins.db`, from application-services. On by default outside release. |
| `services/sync/modules/engines/passwords.sys.mjs` | Sync engine: legacy JS engine plus a Rust bridged engine. |
| `mobile/android/geckoview/.../Autocomplete.java`, `Autofill.java`, `GeckoView.java` | GeckoView login storage delegate and Android Autofill framework bridge. |

Default prefs are set in `modules/libpref/init/all.js` around line 2961, with desktop overrides
in `browser/app/profile/firefox.js` around line 3051. The defaults that matter here:

| Pref | Default | Meaning |
|---|---|---|
| `signon.rememberSignons` | true | Offer to save logins. |
| `signon.autofillForms` | true | Fill on page load. When off, logins are filled only when the user picks one. |
| `signon.autofillForms.http` | false | Never autofill insecure forms on page load. |
| `signon.autofillForms.autocompleteOff` | true | Ignore `autocomplete=off` when filling. |
| `signon.storeWhenAutocompleteOff` | true | Ignore `autocomplete=off` when saving. |
| `signon.formlessCapture.enabled` | true | Capture formless logins on navigation. |
| `signon.formRemovalCapture.enabled` | true | Capture when a form is removed after a fetch or XHR. |
| `signon.userInputRequiredToCapture.enabled` | true | Capture only after a user gesture. |
| `signon.passwordEditCapture.enabled` | false in all.js, true in firefox.js | Show a dismissed save prompt while the user edits. |
| `signon.generation.enabled` | true | Offer generated passwords. |
| `signon.generation.confidenceThreshold` | 0.75 | NewPasswordModel score needed to offer generation. |
| `signon.signupDetection.confidenceThreshold` | 0.75 | SignUpFormRuleset score threshold. |
| `signon.improvedPasswordRules.enabled` | true | Apply per-site password rules. |
| `signon.schemeUpgrades` | true | An `http:` login can fill the same host on `https:`. |
| `signon.includeOtherSubdomainsInLookup` | true | Show eTLD+1 logins in the dropdown. |
| `signon.relatedRealms.enabled` | **false** | Related-realms lookups are off. |
| `signon.usernameOnlyForm.enabled` (StaticPrefList) | true | Detect username-only forms. |
| `signon.usernameOnlyForm.lookupThreshold` | 5 | Maximum username-only lookups per document. |
| `signon.privateBrowsingCapture.enabled` | true | Capture in private windows. |
| `signon.masterPasswordReprompt.timeout_ms` | 900000 (15 min) | Do not re-ask for the primary password within this time after a cancel. |
| `signon.storage.rust.enabled` | true on non-release, false on release and ESR | Use the Rust store. |

---

## 1. Field detection

### 1.1 Which inputs count

- **Password field.** An input qualifies if it is or was `type=password` (`hasBeenTypePassword`). A field whose type was later changed to text, for show-password toggles, still counts. It must be connected and accept autocomplete. See `Logic.isPasswordFieldType` (GH `.../LoginManager.shared.sys.mjs`).
- **Username candidate.** The input type must be `text`, `email`, `url`, `tel`, `number` or `search`, or contain "user". Its `autocomplete` field name must be one of `username`, `webauthn`, `email`, `tel`, `tel-national`, `on`, `off`, or empty. Any other token rejects the input. So `autocomplete="one-time-code"`, `cc-number` or `name` on a text input means it is never taken as the username. See `Logic.isUsernameFieldType`, SF `toolkit/components/passwordmgr/LoginManager.shared.sys.mjs` (GH `#L103`).
- **Grouping.** A field belongs to its outermost `<form>`. A form inside a shadow root resolves to the host's form. Formless inputs group under the document root (`FormLikeFactory.findRootForField`, SF `toolkit/modules/FormLikeFactory.sys.mjs`).
- **Ignored contexts.** System principals, null principals (sandboxed frames) and `about:` pages are skipped (`shouldIgnoreLoginManagerEvent`).

### 1.2 `_getFormFields`

SF `toolkit/components/passwordmgr/LoginManagerChild.sys.mjs`, `LoginFormState._getFormFields`, GH `#L1003`.

1. **Recipes go first.** A site recipe can name `usernameSelector` and `passwordSelector`, or exclude fields with `notUsernameSelector` and `notPasswordSelector`. Recipes come from the Remote Settings collection `password-recipes`, with a bundled default file (`signon.recipes.path`). Recipe keys are `hosts`, `pathRegex` and the selectors (`LoginRecipes.sys.mjs`).
2. **Password fields.** Up to 5 are collected. With more than 5 the form is ignored (`#L893`). At submit time, values shorter than 2 characters are skipped (`#L1050`).
3. **No password field** means it may be a username-only form (§3.5).
4. **Username.** Walk backwards from the first password field over username-compatible inputs:
   - Default to the nearest text field before the password.
   - Replace it with any field that `isInferredUsernameField` accepts, and stop. That check passes for `autocomplete=username` or `webauthn`, or for `/username/i` in id, name, class, aria-label, placeholder or the first label.
   - Otherwise, an email-like field replaces it and the walk continues. Email-like means `type=email`, `autocomplete=email`, or `/email|邮箱/i` in those attributes.
   - Fields after the password are never considered. See LoginHelper GH `#L1375`.
5. **Generated-password fields win.** A field Firefox filled with a generated password is the new password. The next generated field is the confirm field. The password field just before it is the old password.
6. **Page load, or only one password field.** The first password field is the login password. The `current-password` and `new-password` tokens play no part in choosing it.
7. **Submit with 2 or 3 password fields.** Fields are classified by their values:
   - Two equal values: one new password, the second is a confirm field.
   - Two different values: old then new, a change-password form.
   - Three values: the identical pair is the new password and the odd one is the old password. If all three differ, the form is ignored.

### 1.3 Where the `autocomplete` tokens are actually used

| Token | Effect |
|---|---|
| `username`, `webauthn` | Certain username (`isInferredUsernameField`). `webauthn` also adds passkey items to the dropdown (`signon.webauthn.autocomplete`). |
| `email`, `tel` | Allowed as username (bug 1540154). |
| `current-password` | **Not read** by the field picker. It is one weak feature in NewPasswordModel (weight about -0.03) and one in SignUpFormRuleset (`formHasAcCurrentPassword`). |
| `new-password` | `isProbablyANewPasswordField` returns true without running Fathom, so generation is offered. It **blocks autofill on load** (`AUTOFILL_RESULT.PASSWORD_AUTOCOMPLETE_NEW_PASSWORD`, `#L2919`), though the user can still pick from the dropdown. It is preferred when looking for the confirm field (`findConfirmationField`, within 3 fields). It is the fallback sign-up signal when the Fathom form scenario is unknown. |
| `off` | Ignored for fill and save by default (the two `autocompleteOff` prefs). On the password field it blocks page-load fill only if `signon.autofillForms.autocompleteOff=false`. |
| `cc-number` | If the "password" looks like a card number, the save prompt is shown dismissed. A card number in the username plus a 3-digit "password" does the same (CVV heuristic). |
| `one-time-code` | **No password-manager handling.** Bug 1547292, "Don't autofill saved logins in password fields with `autocomplete="one-time-code"`", is still NEW. Bug 1888857, "should not offer to autogenerate passwords for one-time-code", is UNCONFIRMED. Only form history skips these values (bug 1547294, fixed in Fx 109). Side effects: a *text* OTP input is never taken as a username (§1.1), and `isInferredNonUsernameField` rejects `/\b(search|code|add)\b/i` for username-only forms. A `type=password` OTP field is still treated as a password and can be filled. |

### 1.4 Fathom models

[Fathom](https://mozilla.github.io/fathom/) is Mozilla's trained ruleset framework. It scores DOM elements with weighted features and a sigmoid.

- **NewPasswordModel**, SF `toolkit/components/passwordmgr/shared/NewPasswordModel.sys.mjs`.
  - It is trained out of tree at `github.com/mozilla-services/fathom-login-forms` (the `new-password` ruleset).
  - Features are new, confirm and current keywords in labels, aria-label, placeholder and attributes. Others are forgot-password links in the form or on the page, register or login form attributes, the form action and the button text, a remember-me checkbox, a newsletter checkbox, the heading above ("loginy" or "registery"), whether the next input looks like a confirm field, and multiple visible inputs.
  - Score at least 0.75 (`signon.generation.confidenceThreshold`) means a new-password field, so the generated password is offered in the dropdown. A value of -1 disables the model.
  - Used in `Logic.isProbablyANewPasswordField` (GH `.../LoginManager.shared.sys.mjs#L181`). Scores are cached per element.
- **SignUpFormRuleset**, SF `toolkit/components/satchel/SignUpFormRuleset.sys.mjs`.
  - Form-level score. Features are register, login or subscription regexes on form attributes, `ac=current-password` and `ac=new-password` fields, email, username, password and name fields, register, login, continue and subscribe buttons, terms and forgot-password links, "already signed up" links, remember-me and subscription checkboxes, the nearest header, and the document title (including "edit profile").
  - Bias -4.40. A score above `signon.signupDetection.confidenceThreshold` (0.75) means a sign-up form.
  - Run through `FormScenarios.detect` **only when Firefox Relay is interested in the field** (`FirefoxRelayUtils.isRelayInterestedField`), because the model is expensive. Its main consumer is offering Relay email masks.
- **Simple regex heuristics.** `LoginHelper.isInferredLoginForm` matches the form's attributes against `NewPasswordModel.LoginRegex`, or a lone `button[type=submit]` against the login-text regex. It is used only for username-only forms. The code comment says the authors would switch it to Fathom if needed.
- **FormAutofillHeuristics** (`toolkit/components/formautofill/shared/FormAutofillHeuristics.sys.mjs`) handles addresses and cards only. It has no password logic: a grep for password, username or one-time found none.

### 1.5 OTP

There is no OTP handling in the login manager beyond the side effects in §1.3. I found no SMS-code or WebOTP integration in passwordmgr. I did not verify Gecko's WebOTP status separately.

---

## 2. Sign-in, sign-up and change-password

Firefox has no single "form type" classifier. It decides from separate signals.

| Question | How it is decided |
|---|---|
| Autofill on load? | Refused if the password field has `autocomplete=new-password` (§4.2). No other sign-up signal blocks autofill. |
| Offer a generated password? | Field is `new-password`, or NewPasswordModel scores at least 0.75, or the user forced it from the context menu ("Suggest strong password", `forcePasswordGeneration`). Saving must also be enabled for the site (`LoginManagerParent.doAutocompleteSearch`). |
| Offer a Relay mask in the username dropdown? | SignUpFormRuleset, falling back to `new-password` (`FormScenarios`, `_fillForm` `#L2760`). |
| Change password at submit? | Two or three password fields with differing values (§1.2 step 7) give `oldPasswordField` and `newPasswordField`. |
| Save or update? | `LoginManagerParent.showDoorhanger` (GH `#L1037`) dedupes logins for origin + formActionOrigin. A match by username, or a username-less match whose password is the same, gives "Update" if the password differs. Otherwise it is "Save". With no username and an old password field: one saved login is updated; several saved logins mean the user picks a username (`promptToChangePasswordWithUsernames`). |

The filled login is tracked per form root (`fillsByRootElement`). A submit that is unchanged from the autofilled values only records a use (`timesUsed`, `timeLastUsed`) and shows no prompt.

---

## 3. When it offers to save or update

### 3.1 Submission signals

These are the `FORM_SUBMISSION_REASON` values in SF `toolkit/components/satchel/FormHandlerChild.sys.mjs`:

| Reason | Trigger |
|---|---|
| `form-submit-event` | Gecko's `DOMFormBeforeSubmit`. It runs before page `submit` handlers, so values are read before the page can scrub them (bug 257781). |
| `page-navigation` | A web-progress listener in the process root sees `STATE_START` of a **normal** load (`LOAD_CMD_NORMAL`) *triggered by the page*. Null or system triggering principals are skipped, so a URL typed by the user, bookmarks, reload and back/forward don't count. `history.pushState` also counts (`onLocationChange` with `LOCATION_CHANGE_SAME_DOCUMENT` and `LOAD_CMD_PUSHSTATE`). Cross-origin frames ask the parent to listen at the top level too. On this signal, LoginManagerChild runs capture over every connected FormLike in the document (`_onPageNavigation`, `#L2246`). Bug 1166947, fixed in 2016. |
| `form-removal-after-fetch`, `password-removal-after-fetch` | After the user types into a login form, Firefox calls `document.setNotifyFetchSuccess(true)`. After a **successful fetch or XHR** (`DOMDocFetchSuccess`) it switches on `DOMFormRemoved` and `DOMInputPasswordRemoved`, and LoginManagerChild snapshots each modified form or formless password field (`before-form-submission`). If one of those nodes is then removed from the DOM, it is treated as a submit, using the snapshot and `ignoreConnect` (`#onDOMElementRemoved`, `#L1617`). Bug 1166995, fixed in 2021. |
| `iframe-pagehide` | Declared in the enum, but not handled by LoginManagerChild. I believe form autofill (addresses and cards) uses it. Not verified. |

Firefox has no separate "pagehide means submit" rule for passwords. Navigation covers that case.

### 3.2 Filters before a prompt

`_maybeSendFormInteractionMessageContinue`, GH `#L2380` onward. No prompt is shown when:

- The window is private and `signon.privateBrowsingCapture.enabled` is off (it is on by default).
- The password is fully masked, for example `****` or `••••` (`#L2430`). A username of 3 or more mask characters is dropped instead.
- There was no user gesture since the last capture. This uses `document.lastUserGestureTimeStamp` against `captureLoginTimeStamp` (`_formHasModifiedFields`, `#L631`). The form must also have been user-modified, or have a `value` that differs from its `defaultValue`. The log line is "submitting values that are not changed by the user" (`#L2504`).
- The same username and password were already sent for this form root (`compareAndUpdatePreviouslySentValues`). This prevents double prompts when submit and navigation both fire.
- Saving is disabled for the origin (the never-save list) or globally.

### 3.3 Prompts while editing ("dismissed doorhanger")

On desktop, `signon.passwordEditCapture.enabled=true`. Editing a password field, or a username field next to a filled password, sends `PasswordManager:onPasswordEditedOrGenerated`. That shows a *dismissed* save or update prompt: only the key icon appears, the panel does not open. Every input event triggers it, and a sequence counter keeps only the latest one (`_editSeq`, stop-gap for bug 2055772).

### 3.4 Single-page apps

SPAs are covered by `pushState` navigation (§3.1), by form or password removal after a fetch, and by formless FormLikes rooted at the document. Detection and capture state lives per document (`#loginFormStateByDocument`, a WeakMap keyed by Document). A same-document SPA keeps its state across steps.

### 3.5 Multi-step: username first, password later

- **Username-only form.** It has no password field, exactly one username-compatible input, and that input looks like a username (§1.2 step 4) or the form looks like a login form (`isInferredLoginForm`). It also must not match `/\b(search|code|add)\b/`. See `getUsernameFieldFromUsernameOnlyForm`, `#L788`. Gecko fires `DOMPossibleUsernameInputAdded` only for candidate forms, or for formless inputs with `autocomplete=username`. At most 5 lookups run per document. Recipes are not supported for username-only forms (bug 1708455; bug 1710811 is open).
- **Filling.** A username-only form *is* autofilled with the username (`AUTOFILL_RESULT.FILLED_USERNAME_ONLY_FORM`).
- **Remembering the username.** On submit of a username-only form, Firefox stores `{name, value}` in `docState.mockUsernameOnlyField` (`#L2361`) and sends no prompt. When a password is later submitted **in the same document** with no username field, that remembered username is used.
- **Typed values.** The save prompt's username dropdown also lists every value typed into username-type inputs on the page (`possibleUsernames`, `signon.capture.inputChanges.enabled`).
- **Limit.** All of this is per document. When the password step is a new page load, the username is lost, and Firefox saves a username-less login that the user has to edit. Bug 348941, "Handle Logins with User ID and Password on Separate Pages", is still NEW as of 2026-10. A comment there from 2026-10-09 complains about exactly that.

---

## 4. Filling

### 4.1 When fill runs

- **Triggers.** Fill starts from `DOMFormHasPassword` (a form gained a password field), `DOMInputPasswordAdded` (formless password, coalesced through a 1 ms DeferredTask) and `DOMPossibleUsernameInputAdded`.
- **Waiting.** It is **deferred until `visibilityState == "visible"` and `readyState == "complete"`** (`#isReadyForAutofill`, `#L1784`). So background tabs don't fill until they are selected (bug 1149500). The exception: when a primary password is set, fill runs at once so the primary-password prompt appears early. Otherwise it would interrupt tab drags later (bugs 1539091, 1538460).
- **bfcache.** A restored page is refilled, because password values are not kept in bfcache.
- **How values go in.** Fill uses `setUserInput`, which fires real `input` and `change` events. The page sees the value immediately (§7).

### 4.2 `_fillForm` decision chain

`_fillForm`, GH `#L2732`. Each check returns an `AUTOFILL_RESULT` that Glean records.

1. Nothing to do if there are no saved logins and the form is secure.
2. If the password field is disabled or read-only: stop (`password_disabled_readonly`).
3. **Cross-origin subframe:** if `!userTriggered && !windowGlobalChild.sameOriginWithTop`, stop (`form_in_crossorigin_subframe`, `#L2841`). Bug 786276 shipped in Fx 83, Oct 2020. The reason given was clickjacking: a framed login page on a hostile site gets filled and the user is tricked into submitting. It also leaks credentials across dFPI partitions (bug 1658078, CVE-2020-26962). Dan Veditz noted the cost in the bug: sites that frame their own `accounts.` subdomain lose page-load autofill.
4. **Exact-origin filter** for non-user fills (`_filterForExactFormOriginLogins`, `#L2666`):
   - The login's origin must equal the page origin, except for an `http`→`https` upgrade. No other subdomain is accepted.
   - The login's formActionOrigin must match the form's action origin. An empty value acts as a wildcard, and a different subdomain or a scheme upgrade is accepted.
   - Logins that fail this are offered **only in the dropdown**.
5. **Insecure form:** stop when `!userTriggered && !signon.autofillForms.http && !isFormSecure` (`#L2867`). Secure means the page is a secure context, and the action is not plain `http:` unless that origin is potentially trustworthy. Local-IP pages framed by local-IP pages are exempt. Shipped in bug 1217152 (Jan 2017), because Firefox was warning about the field while filling it at the same time.
6. **`maxlength`:** drop logins that won't fit (bug 391514, which helps sites with an extra PIN field).
7. **Unmasked password field:** if `type != password` right now, refuse unless the user triggered the fill (`type_no_longer_password`).
8. **`autocomplete=new-password`:** refuse unless the user triggered the fill.
9. **Password already filled:** do not overwrite it (`existing_password`).
10. **Username already present** (typed, disabled or read-only): fill only the password, and only for a login with that username. Case-insensitive, with an exact-case match preferred.
11. **Several logins:** no autofill (`multiple_logins`). The user picks from the dropdown. The one exception is a site with exactly one login of the form's "shape": a user+password login versus a password-only login, such as a PIN.
12. If `signon.autofillForms` is false, or the window is private: no autofill on load. The dropdown still works.
13. If the field still isn't filled but is focused, the dropdown opens.

### 4.3 User-triggered fills

User-triggered fills come from the dropdown, from the context menu, or from leaving a username field with Tab or Enter (`onUsernameAutocompleted`). They skip the cross-origin, insecure, exact-origin, unmasked and `new-password` checks. Tab or Enter fills the password only for an *exact* origin and action match on that username.

The parent re-checks that the document origin is still the one the dropdown was built for (`fillForm`, `originMatches`). That guards against a navigation racing the fill.

### 4.4 Subdomains, related realms, HTTP to HTTPS

- **Lookup** (`LoginManagerParent.searchAndDedupeLogins`, GH `#L270`):
  - The same site's logins are included with `acceptDifferentSubdomains` (eTLD+1, `includeOtherSubdomainsInLookup`, bug 589628, shipped 2019).
  - The http/https variants are included (`schemeUpgrades`).
  - `shadowHTTPLogins` hides an `http:` login when an `https:` login with the same host, port and username exists.
  - Dedupe by username and password, preferring the same subdomain, then the action origin, then the scheme, then the newest `timePasswordChanged`. Logins whose scheme matches the page sort first.
- **Origin matching** (`LoginHelper.isOriginMatching`, GH `#L727`):
  - The scheme upgrade is one-way: an `http` login can match an `https` page with the same host and port. An `https` login never matches an `http` page.
  - With `acceptDifferentSubdomains`, the base domains must be equal and the scheme the same, or upgraded.
- **Autofill on load still needs the exact host** (§4.2 step 4). Other subdomains and related realms appear only in the dropdown, labelled with their origin.
- **Related realms.**
  - When enabled, `LoginRelatedRealmsParent.findRelatedRealms` reads the Remote Settings collection **`websites-with-shared-credential-backends`**. It is a list of domain groups that share one login backend, sourced from Apple's `password-manager-resources` `shared-credentials` data. Any group that contains the page's domain, as a root domain, adds its other domains to the lookup.
  - `signon.relatedRealms.enabled` is **false by default**. Bug 1699698, "Enable for all channels", is still NEW.
  - Even when it is on, those logins only reach the dropdown, because of the exact-origin filter.
- **HTTP auth logins** use `httpRealm` instead of `formActionOrigin`. A login has exactly one of the two (`buildModifiedLogin` enforces this).

### 4.5 iframes

- **Fill.** No autofill on load in cross-origin frames. User-triggered fill in them is allowed. Same-origin frames behave like the top level.
- **Capture.** Capture in frames is keyed by the *frame's* origin (`doc.documentURI`). Bug 1673714 (open) asks Firefox to stop saving credentials typed inside cross-origin iframes, since they later autofill when that origin is visited directly.
- **Navigation.** A navigation in the top level counts as a submit for cross-origin frames too, via `FormHandler:RegisterProgressListenerAtTopLevel`.

---

## 5. Generated passwords

- **When offered.** See §2. The item appears in the password field's dropdown as "Use a Securely Generated Password". It is also in the context menu ("Suggest strong password…"). It is not offered when saving is disabled for the site (bug 1554005), and never through the Rust or JSON storage while the primary password prompt was cancelled.
- **Generator.** `shared/PasswordGenerator.sys.mjs` is a port of Chromium's `password_generator.cc`.
  - Default length is 15. The default rules need at least one of each of upper, lower, digit and special.
  - The alphabets leave out look-alike characters: no `l`, `o`, `I`, `O`, `0` or `1`. Specials are `-~!@#$%^&*_+=)}:;"'>,.?]` (symbols added in bug 1559986).
  - It uses `crypto.getRandomValues`. It picks one character from each required class, fills the rest from the union of classes, then shuffles.
  - It honours `minlength`, `maxlength` and `max-consecutive`, the latter by up to 15 reshuffles. The input's own `maxLength` caps the length.
- **Site rules.**
  - `PasswordRulesManagerParent.generatePassword` reads the Remote Settings collection **`password-rules`**. The data is Apple's `password-rules.json` from `apple/password-manager-resources`, with one record per `Domain`.
  - It matches the record by root domain on the ASCII host, so IDN hosts are compared as ASCII.
  - It parses the rule with `PasswordRulesParser.sys.mjs`, a vendored copy of Apple's `PasswordRulesParser.js` (MIT, commit `5f6da894`).
  - This is gated by `signon.improvedPasswordRules.enabled` (true).
  - I found **no code that reads a page's `passwordrules` HTML attribute**. A grep of passwordmgr turned up only the Remote Settings path.
- **One password per origin per session.** `gGeneratedPasswordsByPrincipalOrigin` maps the principal origin (including the container or private suffix) to `{value, filled, edited, storageGUID}`. The same password is offered again for confirm fields and later keystrokes.
  - The entry is cleared when its auto-saved login gains a username, or is merged or deleted.
  - Private-origin entries are cleared when the last private window closes.
- **Auto-save before submit** (`_onPasswordEditedOrGenerated`, GH `#L1228`; bug 1565409).
  - Filling a generated password **immediately adds a login with an empty username** for origin and formActionOrigin, and remembers its GUID as `storageGUID`. This happens only if the site has no existing *username-less* login there. A dropdown footnote says whether it will be saved (`willAutoSaveGeneratedPassword`).
  - If a username-less login already exists, it is not overwritten: the user may still need the old password to finish a password change.
  - Later edits to the field update that same login (`modifyLoginAsync`).
  - A dismissed "update" prompt with a key icon and a "Password saved" message lets the user add the username. On submit, adding the username merges it into the auto-saved login (bug 1560042).
  - Clearing or wholly replacing the field's value stops treating it as generated, so no further auto-save happens.
- **Confirm field.** It is filled automatically: the next enabled password field within 3 non-hidden inputs, preferring another `new-password` (bug 1576490).
- **Masking.** Generated values stay unmasked while focused and are masked on blur (bug 1560029).

---

## 6. Storage

### 6.1 Legacy store, still the release default

- **Files.** `logins.json` in the profile, plus `logins-backup.json` (`signon.backup.enabled`). It is JSON without indentation, version 3 (`LoginStore.sys.mjs`).
- **Top level.** `logins[]`, `nextId`, `version`, `potentiallyVulnerablePasswords[]` (each `{encryptedPassword}`), and `sync: {syncID (encrypted), lastSync}`.
- **Per login** (`storage-json.sys.mjs` `#addLogin`):
  - **Plaintext:** `id`, `hostname` (the origin), `httpRealm`, `formSubmitURL` (the formActionOrigin), `usernameField`, `passwordField`, `guid`, `encType`, `timeCreated`, `timeLastUsed`, `timePasswordChanged`, `timesUsed`, `timeLastBreachAlertDismissed`, `syncCounter`, `everSynced`.
  - **Encrypted:** `encryptedUsername`, `encryptedPassword`, and `encryptedUnknownFields`, which holds Sync fields this client doesn't understand and round-trips them.
  - **What the plaintext reveals:** origins, field names, timestamps and use counts can be read without the key.
- **Matching IDL** (`nsILoginInfo.idl`, `nsILoginMetaInfo.idl`): origin, formActionOrigin, httpRealm, username, usernameField, password, passwordField, unknownFields, everSynced, syncCounter; guid, timeCreated, timeLastUsed, timePasswordChanged, timesUsed, timeLastBreachAlertDismissed.
  - `usernameField` and `passwordField` are "saved but not used".
  - `timePasswordChanged` is bumped only when the password changes (`buildModifiedLogin`), although the IDL comment says it covers the username too.
  - New logins get a `{uuid}` guid and `timesUsed=1`.
- **Origin rules.** `formActionOrigin` is the origin of the form's action, or of the page when there is no action. An empty string is a wildcard. `javascript:` is allowed as an action.
- **Encryption.**
  - Values are encrypted with NSS **SDR** (Secret Decoder Ring, `crypto-SDR.sys.mjs` → `security/manager/ssl/SecretDecoderRing.cpp`). The key lives in **`key4.db`**, the NSS softoken SQLite database used since Fx 58.
  - The mechanism comes from `security.sdr.mechanism`. The default is `1`, meaning **AES-CBC**; `0` is legacy 3DES-CBC. Older entries in 3DES still decrypt, and the output is base64.
- **Primary password.**
  - It encrypts the key in `key4.db` and leaves `logins.json` alone. Without one, `logins.json` and `key4.db` together are enough to decrypt everything ([SUMO](https://support.mozilla.org/kb/Protecting+stored+passwords+using+a+master+password)). The PBKDF2 strength history (bug 524403) was not checked here.
  - Usernames are encrypted too. So with a primary password set, even showing the username dropdown needs an unlock (MattN, bug 1755724 comment 4).
  - After a cancel, Firefox does not re-ask for 15 minutes (`signon.masterPasswordReprompt.timeout_ms`).
  - Sync pauses while the store is locked (`MASTER_PASSWORD_LOCKED`).
- **OS reauth.**
  - `LoginHelper.requestReauth` uses OS auth (`OSKeyStore.ensureLoggedIn`) when there is no primary password, or when one is set and already unlocked. Otherwise it logs the NSS token out and back in, which prompts for the primary password.
  - The pref `signon.management.page.os-auth.locked.enabled` is locked, and its default in `firefox.js` is `false`.
  - [SUMO, "Secure Firefox passwords with device sign-in"](https://support.mozilla.org/kb/firefox-password-authentification-prompt) describes the setting "Require device sign in to fill and manage passwords". It needs Fx 127 or later on Windows and macOS 10.12 or later. After one prompt it isn't asked again for 5 minutes or until a reload; export always asks. The article says it does not defend against local attackers.
  - I did not verify whether the default differs per platform or rollout. The code default is off.

### 6.2 New store: Rust `logins.db`

- **Status.**
  - `signon.storage.rust.enabled` is true on Nightly and Beta and false on release and ESR. `LoginStorageMigrator.sys.mjs` migrates JSON to Rust, with a restore path, attempt budgets, and limits on primary-password deferrals.
  - The store is application-services' `logins` component, opened with `createLoginStoreWithNssKeymanager(path, PrimaryPasswordAuthenticator)` and keyed through NSS.
  - Lookups on candidate fields (origin, action, realm, field names, guid, timestamps) can be answered **without the encryption key**, so no primary-password prompt is needed (`storage-rust.sys.mjs`, `CANDIDATE_MATCH_FIELDS`).
- **Schema** (`mozilla/application-services` `components/logins/src/schema.rs`):
  - `loginsL` is the local table, with `local_modified`, `is_deleted` and `sync_status`.
  - `loginsM` is the mirror of the last server state, with `server_modified`, `is_overridden` and `enc_unknown_fields`.
  - `loginsSyncMeta` holds sync metadata.
  - `breachesL` holds encrypted passwords seen in breaches, for cross-site reuse warnings (bug 2014119).
- **Android.** Firefox for Android stores logins in the app (android-components, Rust logins) through GeckoView's `Autocomplete.StorageDelegate` (§8). Gecko's `storage-geckoview.sys.mjs` has no crypto of its own.

### 6.3 Firefox Sync for logins

**Records.** The Sync collection is `passwords`. Each record is a BSO with `id = login guid`, end-to-end encrypted with the account's Sync keys. The server only sees ciphertext plus `modified`. The cleartext fields are:

- `hostname`, `formSubmitURL`, `httpRealm`, `username`, `password`, `usernameField`, `passwordField`, `timeCreated`, `timePasswordChanged`.
- The Rust payload adds `timeLastUsed`, `timesUsed` and `timeLastBreachAlertDismissed`.
- Unknown fields are kept and sent back up.

Sources: `services/sync/modules/engines/passwords.sys.mjs`; application-services `components/logins/src/sync/payload.rs`.

**What marks a login for upload** (legacy):

- `syncCounter` is bumped only when a *syncable* field changes. Syncable means the nsILoginInfo fields plus `timeCreated` and `timePasswordChanged`. Changes to `timeLastUsed` and `timesUsed` alone are not uploaded, so a "last used" bump cannot overwrite a password changed on another device before it arrives (bug 973166).
- Deleting a login that was ever synced leaves a **tombstone**: guid, `timePasswordChanged`, `syncCounter` and `deleted:true`, with all secrets removed. A login that was never synced is simply removed.
- The FxA account login itself is never synced.

**Conflict rules, legacy JS engine** (`SyncEngine._reconcile`, `services/sync/modules/engines.sys.mjs#L1750`): the whole record wins or loses.

- The local "modified" time for passwords is `timePasswordChanged` (`_getChangedIDs`), compared with the server's `modified` by age. Clock drift is not handled (bug 721181).
- An incoming delete wins unless the local copy was modified more recently. In that case the record is revived.
- **Dedupe.** An incoming guid that is unknown locally is matched against local logins with the same origin, formActionOrigin, httpRealm and username, ignoring the password (`_findDupe`). The local guid is then switched to the incoming one.

**Conflict rules, Rust engine** (`components/logins/src/sync/engine.rs`, `update_plan.rs`, `merge.rs`): a **three-way merge** against the mirror.

- Each side's change against the mirror is computed per field. Where both changed the same field, the newer side wins (remote age against local age).
- `timesUsed` is summed. `usernameField` and `passwordField` are treated as non-conflicting.
- An inbound deletion always wins.
- A local tombstone against a remote update keeps the update. The code comment says losing a login is the worse outcome, "just like desktop".
- With no mirror (a two-way merge), the side with the newer `timePasswordChanged` wins.
- Dedupe matches origin and httpRealm, a formActionOrigin that is empty or contains the same host:port (the code notes this was "stolen from iOS"), and an equal username.

---

## 7. Breach alerts, and the security traps Firefox guards against

### 7.1 Breach alerts and "vulnerable passwords"

Sources: `browser/components/aboutlogins/LoginBreaches.sys.mjs` and `toolkit/components/passwordmgr/BreachAlertsData.sys.mjs`.

- **Data.** Breach data is the Remote Settings collection **`fxmonitor-breaches`**. It is a public list published by Monitor (`mozilla/blurts-server`) from HIBP, with `Name`, `Domain`, `BreachDate`, `AddedDate` and `DataClasses` for each breach.
- **The check runs entirely on the device.** No username or password is sent anywhere. A login is flagged when all of these hold:
  - its host has the breach `Domain` as a root domain;
  - `DataClasses` includes `"Passwords"`;
  - `timePasswordChanged < BreachDate`.
- **Dismissals.** A dismissal is kept per login (`timeLastBreachAlertDismissed`). It hides the alert until a breach with a newer `AddedDate` arrives.
- **Vulnerable passwords** (Fx 76 and later). The password of each flagged login is stored encrypted in `potentiallyVulnerablePasswords`, or `breachesL` in the Rust store. Any other login that uses the same password is shown as "vulnerable", which catches reuse across sites.
- **Prefs.** `signon.management.page.breach-alerts.enabled` and `signon.management.page.vulnerable-passwords.enabled`, both true. See also [SUMO, "Alerts for breached websites"](https://support.mozilla.org/kb/firefox-password-manager-alerts-breached-websites).
- **Monitor email checks are a separate thing.** They use HIBP k-anonymity hash-prefix range queries ([Mozilla Security blog, 2018](https://blog.mozilla.org/security/2018/06/25/scanning-breached-accounts-k-anonymity/)). That is the Monitor service, not this about:logins code.

### 7.2 Traps and how Firefox handles each

| Trap | Firefox behaviour | Bugs and reasons |
|---|---|---|
| **Autofill on load read by page or third-party scripts** | Still autofills on load by default, and `setUserInput` exposes the value to scripts at once. The mitigations are the per-case refusals in §4.2: cross-origin frames, insecure forms, several logins, `new-password`, and unmasked fields. | Meta bug **1755724, "Consider removing autofill"** (NEW, opened 2022 by Sergey Galich). His reasons: (1) visiting a link is enough to fill, and sites steal values before submit; (2) the primary-password prompt pops up with no user intent, made worse by pinned tabs (bugs 1700647, 1736922); (3) filling before handlers attach breaks sites (bug 1679564). He cites the Leaky Forms study, USENIX Security 2022 (the link given in the bug, `homes.esat.kuleuven.be/~asenol/leaky-forms/`, returned 404 today). Bug 2030674, "autofill details are accessible to script prior to interaction", was duped to it in April 2026 by dveditz. The [Princeton "No boundaries" research, 2017](https://freedom-to-tinker.com/2017/12/27/no-boundaries-for-user-identities-web-trackers-exploit-browser-login-managers/) found ad scripts (AdThink, OnAudience) planting invisible login forms to collect autofilled emails. It also noted that Chrome waits for a click before filling the password, unlike the others. |
| **Hidden or invisible fields** | **No visibility check.** Firefox fills invisible fields. | Bug **1247245** (ASSIGNED, open since 2016). Comments: "invisible" has too many forms (display, offscreen, z-index, colour), and a script can show a form, wait for the fill, then hide it within milliseconds. An IntersectionObserver would start an arms race and cost performance (MattN, 2019). Bug 373153 (WONTFIX, 2007) is the older version. |
| **Insecure (HTTP) forms** | No autofill on load (bug 1217152). The dropdown gets a first row: "This connection is not secure. Logins entered here could be compromised." Console messages `InsecurePasswordsPresentOnPage`, `…OnIframe` and `InsecureFormActionPasswordsPresent`. Saving still works. Local-IP pages are exempt (`security.insecure_field_warning.ignore_local_ip_address`). | The reason was the contradiction of warning about a field while filling it ([Tanvi's blog, Jan 2016](https://blog.mozilla.org/tanvi/2016/01/28/no-more-passwords-over-http-please/)). Users with many HTTP-only accounts pushed back in the bug. |
| **Cross-origin iframes** | No autofill without user action (bug 786276, Fx 83). Saving in them still works (bug 1673714 open). | Clickjacking (bug 610997), and the dFPI cross-partition leak (bug 1658078, CVE-2020-26962). |
| **Form action to another origin** | No autofill on load unless the stored `formActionOrigin` matches, allowing subdomain or wildcard. The login is offered in the dropdown instead. | It stops a login form that posts credentials elsewhere from being filled silently. |
| **Background tabs or not yet visible** | Fill is deferred until the tab is visible and the page has loaded (bug 1149500). | Avoids filling and prompting in tabs the user isn't looking at. |
| **Navigation races** | A user-triggered fill re-checks the document origin in the child. | |
| **Masked placeholders or bogus saves** | Skips `****` passwords and card number + CVV pairs. Needs a user gesture before capture. | |
| **`autocomplete=off` abuse by sites** | Ignored for both fill and save, by default. | Bug 1105331, Okta. |
| **Sandboxed or null-principal documents, `about:` pages** | Login manager does not run. | Avoids using the wrong `location` as the origin. |

Autofill "timing attacks": I found no specific mitigation in the code beyond deferring fill until visible and loaded. The only timing-related case in the bugs is bug 1679564, about events missed when the fill comes early. I did not find work on detecting autofill through CSS `:autofill` or event timing.

---

## 8. Firefox for Android (GeckoView and Fenix)

- **Same engine.** Gecko's `LoginManagerChild` and `LoginManagerParent` run on Android too: detection, fill rules and capture are shared, and `LoginManager.shared.sys.mjs` is "shared across Firefox Desktop, Firefox Android and Firefox iOS". Storage and UI are delegated to the app:
  - **`Autocomplete.StorageDelegate`** (`mobile/android/geckoview/.../Autocomplete.java`). `onLoginFetch(domain)` returns `LoginEntry[]`. `onLoginSave(login)` and `onLoginUsed(login, usedFields)` report back. The app registers it with `GeckoRuntime.setAutocompleteStorageDelegate`.
  - **`GeckoSession.PromptDelegate`**. `onLoginSave(session, request)` is the save or update prompt. `onLoginSelect` is the chooser when several logins match.
  - The GeckoView docs say: autofill if there is only one suitable login, otherwise `onLoginSelect`. On submit or navigation, fetch again and then call `onLoginUsed` or `onLoginSave`.
  - Page-load autofill is the same `signon.autofillForms` pref, exposed as `GeckoRuntimeSettings.setLoginAutofillEnabled`.
- **Android Autofill framework, both directions.**
  1. **Third-party managers into Firefox.** `GeckoView` implements `onProvideAutofillVirtualStructure` and `autofill(SparseArray<AutofillValue>)`. Through `Autofill.Session.fillViewStructure` it exposes web form fields as a virtual `ViewStructure` with `HtmlInfo` and hints for username, password and email. So Bitwarden, 1Password or Google can fill web forms in Fenix. It is on by default (`GeckoView.setAutofillEnabled`). User reports say this is less reliable than in Chromium browsers, e.g. the [Bitwarden community](https://community.bitwarden.com/t/android-autofill-very-unreliable-with-firefox/28391) and SUMO threads. That is anecdotal and not verified.
  2. **Firefox as an autofill provider for other apps.** Fenix ships `org.mozilla.fenix.autofill.AutofillService`, built on android-components `feature-autofill`'s `AbstractAutofillService`, with `BIND_AUTOFILL_SERVICE` and unlock, confirm and search activities (`mobile/android/fenix/app/src/main/AndroidManifest.xml`).

---

## 9. Points for the Project B design

These are observations only, not recommendations.

- Firefox's fill policy is a **sequence of refusals**, not a model of each form. Most of it is portable as plain rules:
  - autofill on load only for the exact host (http→https allowed) and a matching action origin;
  - only in same-origin-with-top frames;
  - only for secure forms;
  - only when there is exactly one login;
  - never into `new-password` or unmasked fields;
  - never overwrite a typed password;
  - everything else goes to the dropdown.
- **Capture** has three triggers: submit, page-initiated navigation (including pushState), and removal of a modified form after a successful fetch. Each also needs a user gesture since the last capture and real modifications. A `(root, username, password)` "last sent" check stops duplicate prompts.
- **Gaps Firefox still has:** cross-page username-then-password (bug 348941), OTP fields (bug 1547292), the hidden-field check (bug 1247245), and the open question of keeping autofill on load at all (bug 1755724).
- **Generated passwords are saved before submit**, as a username-less login tracked by GUID. That avoids losing the password if the sign-up never "submits" in a way Firefox can detect.
- **Sync lessons:**
  - do not sync use-only bumps (bug 973166);
  - keep tombstones only for records that were ever synced;
  - dedupe on origin + action + realm + username;
  - the newer Rust engine moved from whole-record last-writer-wins to a per-field three-way merge with a mirror table, summing `timesUsed` and letting an update beat a concurrent delete.

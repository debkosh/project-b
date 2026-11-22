# How Chromium's password manager works (Chromium 155 checkout)

Research notes for Project B's password autofill design. All paths are relative to
`chromium/src` (the checkout `tools/fetch-chromium.sh` makes) and line numbers are from this
checkout (Chromium 155, read 2026-11-17). Abbreviations: PWM = password manager,
FOAS = fill on account select, UFF = username first flow.

Source of truth is the code. URLs to chromium.org and web.dev docs are listed at the end. I did not
fetch them this session.

---

## 0. Architecture in one screen

- PWM is a "layered component": `core/` (no content dependency), `content/` (content-layer glue),
  `ios/`. UI lives in `//chrome` (`components/password_manager/README.md:1-20`).
- One `PasswordManager` per tab, one `PasswordFormManager` per observed form, and one
  `ContentPasswordManagerDriver` per frame. In the renderer, `AutofillAgent` owns
  `PasswordAutofillAgent` and `PasswordGenerationAgent` (README diagram,
  `components/autofill/content/renderer/README.md:3-7`).
- The renderer extracts `FormData`. The browser parses it (`FormDataParser`), fetches stored
  logins (`FormFetcher`), and sends `PasswordFormFillData` back. The renderer fills.
- Mojo surfaces (`components/autofill/content/common/mojom/`): PasswordManagerDriver (about 12
  methods), PasswordAutofillAgent (about 13), PasswordGenerationDriver (about 7), AutofillDriver
  (about 18), AutofillAgent (about 21). I counted these with grep, so treat them as approximate.

---

## 1. Field detection

### 1.1 Parser pipeline (FormDataParser)

`components/password_manager/core/browser/form_parsing/form_data_parser.cc`

The parser works per renderer form. It runs four stages. A later stage only fills gaps the earlier
ones left. If two stages give a field conflicting roles, everything found so far is reset.
- (1) ML model predictions. These must cover every field (`:1393`, `ParseUsingModelPredictions`
  `:963`). The model output arrives from the client (Chrome's client forwards
  `kHeuristicsOrAutocomplete` and actor-model types, see
  `chrome/browser/password_manager/chrome_password_manager_client.cc:2040-2075`).
- (2) Server predictions (`:1408`, `ParseUsingServerPredictions` `:481`). If the model already
  finished, only server *overrides* (`is_override`) are applied (`ExtractServerOverrides`
  `:1253`, `ApplyServerOverrides` `:1268`).
- Fields the server marks `NOT_USERNAME`, `NOT_PASSWORD` or credit card are removed from the later
  stages and get no suggestions (`:1429-1446`).
- Manual generation is enabled on any field that is type=password now or was before, or whose
  name or id matches "password" (`:1448-1456`).
- (3) The autocomplete attribute (`:1458`, `ParseUsingAutocomplete` `:623-683`).
- (4) Base heuristics (`:1472`, `ParseUsingBaseHeuristics` `:873-958`). The renderer's HTML
  vocabulary username classifier runs after these
  (`components/autofill/content/renderer/html_based_username_detector.h`, and
  `FindUsernameInHtmlParserResult` used at `:1488-1503`).
- The parser gives up on forms with more than 10000 fields (`form_data_parser.h`,
  `kMaxParseableFields`).
- The parser has two modes. `kFilling` keeps empty fields. `kSaving` drops empty fields and fields
  whose value is one repeated symbol, such as `****` (`ProcessFields` `:1115-1180`,
  `StringMatchesHiddenValue` `:132`).

The Autofill README states the general rule: crowdsourcing beats local heuristics, and the
autocomplete attribute beats both, except `off` (`components/autofill/README.md:256-300`).
For passwords the order is model, then server, then autocomplete, then heuristics. So in the
password parser, server predictions come *before* autocomplete.

### 1.2 Autocomplete attribute

- Only the last token counts: `username`, `current-password`, `new-password`, `one-time-code`,
  any `cc-*`. A `webauthn` token sets `accepts_webauthn_credentials` (`ParseAutocomplete`
  `:58-107`).
- Rules (`:615-622`): at most 1 `username`, at most 1 `current-password`, and at most 2
  `new-password` (new plus confirm). `*-password` counts only on password fields, and `username`
  only on non-password fields. When a rule is broken, that attribute is ignored.
- Two `new-password` fields with different values in saving mode make the parser ignore the
  attribute (`:660-669`).
- `one-time-code` and `cc-*` mark a field as "not a password" (`IsNotPasswordField` `:148-162`).
- `webauthn`: no automatic generation. New-password is folded into password (`:1540-1549`).
  Filling waits for the user (see 4.1).
- A form with a username found by autocomplete and no password becomes a *single-username* (UFF)
  form (`:1523-1528`).

### 1.3 Heuristics when there is no attribute

- **Interactability** has three levels: `kCertain` = the user typed into the field,
  `kPossible` = the field is focusable or visible, `kUnlikely` = neither (`form_data_parser.h`
  enum, set in `ProcessFields` `:1172-1176`). Only fields at the best level present are
  considered (`MatchesInteractability` `:181-187`). Hidden fields are *not* excluded when they are
  all there is.
- **Password fields** (`GetRelevantPasswords` `:704-800`) are type=password, or were type=password
  at some point. The filter drops fields below the best interactability, drops empty fields when
  saving, and drops fields that look like OTP, CVC or SSN by name or id (`kOneTimePwdRe`,
  `password_manager/core/common/password_manager_constants.h:48-60`). It also drops readonly
  fields, unless the user typed or Chrome filled them earlier (`IsLikelyPassword` `:689-702`).
  If that leaves nothing, it falls back to the unfiltered list and marks the form `is_fallback`
  (fallback UI only).
- **Roles by count and value** (`LocateSpecificPasswords` `:268-328`):
  - 1 field gives current password.
  - 2 fields with equal non-empty values give new plus confirm. Otherwise, or if both are empty,
    they give current plus new. Chrome leans toward filling change-password forms.
  - 3 or more fields: use only the first three. All equal gives current. If 2 and 3 are equal,
    the result is current, new, confirm. If 1 and 2 are equal, it is new plus confirm. Otherwise
    the first field is current.
- **Username** (`FindUsernameFieldBaseHeuristics` `:808-855`): walk backwards from the first
  password field through the text fields before it. Skip any field that is a password, below the
  best interactability, looks like OTP, CVC or SSN, or (when saving) has a value that is empty or
  only 1-2 digits (`IsProbablyNotUsername` `:194-196`). If a value matches a stored username, pick
  it at once. Otherwise prefer the nearest focusable field, then the nearest field of any kind.
- **Values in saving**: if the password (or the new password) is at most 1 character, the form is
  only a fallback. This stops prompts on split OTP or PIN boxes (`:1564-1571`).
- **New password is "reliable"** only if it came from the server or from autocomplete, and its
  maxlength allows generation. That is the gate for *automatic* generation (`:1555-1560`).

### 1.4 Server predictions (Autofill crowdsourcing)

- Server: `https://content-autofill.googleapis.com/` (`kDefaultAutofillServerURL`,
  `components/autofill/core/browser/crowdsourcing/autofill_crowdsourcing_manager.cc:127-128`).
  It can be overridden with `--autofill-server-url` (`:178`). Paths are `/v1/pages` (query) and
  `/v1/forms:vote` (upload) (`:524-537`). Requests use a Google API key from
  `google_apis::GetAPIKey` or `--autofill-api-key` (`:577-586`, `:986-988`).
- What is sent (traffic annotations `:220-320`): a query carries "hashed descriptions of the form
  and its fields. User data is not sent". An upload carries the same plus the type of data the
  user gave each field. Cookies are not sent.
  - `FormSignature` is a 64-bit hash of the form's target URL and normalized field names.
    `FieldSignature` is a 32-bit hash of the field name and type
    (`components/autofill/README.md:242-254`).
- What a password upload adds (`components/password_manager/core/browser/votes_uploader.cc`):
  - Field-type votes: USERNAME, PASSWORD, NEW_PASSWORD, CONFIRMATION_PASSWORD, and
    ACCOUNT_CREATION_PASSWORD (`SetFieldLabelsOnSave` and `OnUpdate` `:81-150`).
  - Votes for single usernames and forgot-password forms (`:286-295`, `:606`, `:1018`).
  - A generation vote (`:681`).
  - Noisy password attributes (`GeneratePasswordAttributesMetadata` `:792-845`):
    - It only runs for ASCII passwords.
    - It picks one bit: "has special symbol" with chance 8/9, else "has letter".
    - It applies *randomized response*: half the time the answer is random.
    - It may send one special symbol that occurs in the password, or a random one.
    - It sends a length that is the true length 1 time in 5, else a random value in [1, len-1].
  - `RandomizedEncoder` adds differential-privacy noise to string metadata
    (`components/autofill/core/browser/crowdsourcing/randomized_encoder.h:21-23`). It uses a
    per-profile seed pref (`randomized_encoder.cc:112-117`).
- How predictions reach PWM: `AutofillManager` (browser) queries the server. The embedder's
  `PasswordManagerClient` watches `OnFieldTypesDetermined` and calls
  `PasswordManager::ProcessAutofillPredictions` once per renderer form
  (`chrome_password_manager_client.cc:2040-2071`, `password_manager.cc:1814`). **This glue is in
  //chrome, so an embedder must write it.**
- `PasswordFormManager` ignores server predictions on localhost (`password_form_manager.cc:1607-1613`).
  It also ignores them unless every credential prediction maps 1:1 to a field renderer id
  (`:1626-1647`).
- Mapping from server type to role (`password_field_prediction.cc:67-93`):
  - USERNAME and USERNAME_AND_EMAIL_ADDRESS give username.
  - SINGLE_USERNAME, SINGLE_USERNAME_FORGOT_PASSWORD and
    SINGLE_USERNAME_WITH_INTERMEDIATE_VALUES give single username.
  - PASSWORD gives current password.
  - ACCOUNT_CREATION_PASSWORD and NEW_PASSWORD give new password.
  - CONFIRMATION_PASSWORD gives confirmation.
  - NOT_PASSWORD, NOT_USERNAME and ONE_TIME_CODE give non-credential.
  - Payment groups give non-credential.
- Server responses can also carry a `password_requirements` spec per field (see 5.2).

### 1.5 OTP and one-time-code

- An OTP is detected by `autocomplete=one-time-code`, or by `kOneTimePwdRe` on the field name or
  id (`password_manager/core/common/password_manager_util.cc:107-117`, regex at
  `password_manager_constants.h:48-60`).
- OTP fields are never password candidates (`IsNotPasswordField` `form_data_parser.cc:148-162`).
- A server ONE_TIME_CODE prediction is non-credential (`password_field_prediction.cc:89`).
- An OTP-like field is not used as a UFF username unless the server says SINGLE_USERNAME
  (`password_form_manager.cc:1566-1571`).
- Saving is suppressed when the password value is at most 1 character, which happens with split
  digit boxes (`form_data_parser.cc:1564-1571`).
- WebOTP (SMS) is a separate content and Autofill path (`ContentAutofillClient::DocumentUsedWebOTP`).
  It is not part of PWM.

### 1.6 Username-first flow (UFF) and single-username forms

- Every text field the user edits outside a password form is recorded as a
  `PossibleUsernameData` in an LRU cache keyed by driver id and field id
  (`password_manager.cc:993-1026`).
- Each entry holds the field's signon realm, its value, the time, whether it has
  `autocomplete=username`, and whether it looks like an OTP.
- An entry expires after 5 minutes (`kSingleUsernameTimeToLive`,
  `password_manager_constants.h:36-38`; `possible_username_data.cc:51-53`). The cache is cleared on
  a navigation that cannot be a submission (`password_manager.cc:792-796`) and after a successful
  login (`:1652-1655`).
- When a password form is later submitted, `HandleUsernameFirstFlow` (`password_form_manager.cc:1698`)
  picks the best candidate (`FindBestPossibleUsernameCandidate` `:1584-1602`). A candidate must:
  - be a PSL match to the password form's realm (affiliated sites are not yet supported, TODO at
    `:1547`);
  - have a value;
  - not be stale;
  - not be an OTP (unless the server says SINGLE_USERNAME);
  - not be inside the password form itself (`IsPossibleSingleUsernameAvailable` `:1543-1580`).
- Candidate priority (`possible_username_data.h:95-111`), lowest first:
  - the user edited the field (voting only, not shown in the bubble);
  - `autocomplete=username`;
  - the value matches a prefilled username in the password form;
  - a server SINGLE_USERNAME prediction;
  - a server override.
- Filling a single-username form never happens on page load. It always waits for the user
  (`kSingleUsernameForm`, `password_form_filling.cc:263-264`).
- Votes teach the server SINGLE_USERNAME, NOT_USERNAME and FORGOT_PASSWORD
  (`votes_uploader.cc:895-950`, `:1018-1060`).
- The design doc is linked in code as `https://goo.gl/Mc2KRe` (two-usernames case,
  `form_data_parser.cc:485`). I did not open it, and it may be Google-internal.

---

## 2. Form kinds

Form kind comes purely from which roles the parser found (`password_form.cc:368-402`):
- Login: username plus current password, and no new password.
- Sign-up: username plus new password, and no current password.
- Change password: current password plus new password.
- Reset password: new password only, with no current password and no username.
- Single username: a username only (UFF).
- If none of these, it is "no password form".
- `ClassifyAsPasswordForm` (`form_data_parser.cc:1618-1648`) exposes this to Autofill
  (`autofill::PasswordFormClassification`).
- In the heuristic path, sign-up versus change versus reset mostly comes down to the count and
  value rules in 1.3. With two empty password fields the parser chooses current plus new, which
  makes a change form (`:283-291`). Server predictions and autocomplete are what make sign-up
  forms reliable.
- After submission, change and reset forms are treated differently in success detection. If the
  same form reappears with its new-password fields emptied, that is *not* a failure
  (`password_manager.cc:1604-1607`).
- `well_known_change_password/` implements `/.well-known/change-password` support.

---

## 3. When it offers to save or update

### 3.1 Submission signals (renderer)

`components/autofill/core/common/mojom/autofill_types.mojom:153-176`,
`components/autofill/content/renderer/form_submission_tracker.cc`:

| Source | Trigger | Used by PWM? |
|---|---|---|
| FORM_SUBMISSION | `WillSendSubmitEvent` / `WillSubmitForm` (`:383-416`) | Starts a *provisional save* only. It is not proof of success (`:582-584`). |
| PROBABLY_FORM_SUBMITTED | Main-frame, content-initiated navigation that is not a link click, back/forward, reload or form post (`DidStartNavigation` `:320-365`) | Ignored by PWM (`:585-586`). |
| SAME_DOCUMENT_NAVIGATION | `DidFinishSameDocumentNavigation`, when the last interacted form is gone or unfocusable (`:314-318`) | Yes, counts as success. |
| XHR_SUCCEEDED | `AjaxSucceeded`, when the form is gone or unfocusable (`:130-134`) | Yes, counts as success. |
| FRAME_DETACHED | The subframe holding the form is detached (`:367-381`) | Yes. |
| DOM_MUTATION_AFTER_AUTOFILL | The form or field was hidden or removed after an autofill (`:171`, `:178-191`) | Yes for PWM, ignored by Autofill (mojom `:171-175`). |

- "Gone" means `CanInferFormSubmitted`: the form element was removed, or none of its controls is
  focusable. For formless fields, the field was removed or is unfocusable (`:494-515`). Otherwise
  the tracker keeps watching the element and fires when it disappears (`ElementDisappeared`
  `:136-176`).
- Duplicate signals for the same form are dropped (`FireHostSubmitEvents` `:544-600`).
- The user typing also provisionally saves, on every change. The renderer sends
  `InformAboutUserInput` and the browser calls `ProvisionallySaveForm(..., is_manual_fallback=true)`
  (`password_manager.cc:1028-1064`). This also drives the manual-fallback save icon.

### 3.2 Success decision (browser)

- `OnPasswordFormSubmitted` (`password_manager.cc:870-881`): a provisional save (a real submit).
- `OnDynamicFormSubmission` (`:883-932`): an SPA signal. It is ignored when it comes from a frame
  other than the main frame or the form's frame, and ignored on accounts.google.com. Otherwise it
  calls `OnLoginSuccessful` straight away.
- `DidNavigateMainFrame(form_may_be_submitted)` (`:763-808`): keeps the submitted manager alive
  across the navigation.
- `OnPasswordFormsRendered` (`:1537-1630`) runs after load with only the *visible* forms. If the
  last navigation was an HTTP error, the login failed. If the submitted form is visible again with
  different values, the login failed. Otherwise the login succeeded.
- `ProvisionallySaveForm` (`:1209-1260`) refuses to save on http:// after a submission on https://
  for the same host (`ShouldBlockPasswordForSameOriginButDifferentScheme` `:1530-1535`).

### 3.3 Save versus Update versus silent

- Pending state (`password_save_manager_impl.cc:161-197`):
  - No similar stored credential gives NEW_LOGIN.
  - Same username with a different password gives UPDATE.
  - The presaved generated password gives NEW_LOGIN.
  - A weak (PSL, affiliated or grouped) match with the same password gives AUTOMATIC_SAVE, which
    stores an exact copy for the current origin.
  - Otherwise EQUAL_TO_SAVED_MATCH.
- Prompt or not (`ShouldPromptUserToSavePassword`, `password_manager.cc:237-272`):
  - An update always prompts.
  - A generated password never prompts. It is saved and a confirmation is shown
    (`AutomaticPasswordSave` `:1774`).
  - PSL, grouped and affiliated web matches save silently.
  - Otherwise it prompts only for a new login.
  - An unchanged login is "saved" without a prompt. That updates its last-used time.
- Order in `OnLoginSuccessful` (`:1632-1780`):
  - store a hash for password reuse detection;
  - start the leak check;
  - check the sync-credential filter;
  - check `IsSavingAllowed`;
  - prompt (`client_->PromptUserToSaveOrUpdatePassword`) or save.

### 3.4 Blocklist ("Never for this site")

- `OnNeverClicked` leads to `Blocklist()` (`password_form_manager.cc:616-640`). That stores a
  normalized row with `blacklisted_by_user=1` and empty credentials, keyed by the form digest
  (scheme, signon_realm, url) (`form_saver_impl.cc:110-116`).
- Saving later removes the blocklist row (`password_form_manager.cc:506-509`).
- A blocklisted form does not get automatic generation (`:1249`).
- The UI decides what "blocklisted" means. Chrome desktop returns `kExplicitlyBlocklisted`, which
  shows the key icon but no bubble (`chrome/browser/ui/passwords/manage_passwords_ui_controller.cc:1131-1136`).
  Chrome Android shows nothing (`chrome_password_manager_client.cc:393-399`).
- Filling still works on blocklisted sites.

---

## 4. Filling

### 4.1 On load or on user choice

`components/password_manager/core/browser/password_form_filling.cc:180-300`

The browser computes `wait_for_username`, which means "show suggestions, do not prefill". On
**Android and iOS it is always true** (`:281-283`). On desktop, the first reason that applies wins
(`:224-270`):
- incognito;
- biometric re-auth before filling is on (Mac, Windows, ChromeOS);
- the best match is *affiliated* (web-to-web), *PSL* or *grouped*;
- `driver->HasCrossOriginAncestor()` (a cross-origin iframe);
- the form is not a sign-in form (no current-password field);
- the password field is already prefilled;
- `!client->IsCommittedMainFrameSecure()` (an HTTP main frame).
  - Chrome implements this with `IsOriginPotentiallyTrustworthy`
    (`chrome_password_manager_client.cc:1180-1183`).
  - **The base class returns false** (`password_manager_client.cc:157-159`). An embedder that does
    not override it never fills on load.
- the `fill-on-account-select` feature, which is DISABLED by default (`features/password_features.cc:113-115`);
- the form `accepts_webauthn_credentials`;
- a single-username form;
- an actor (agent) task is running;
- a password change is in progress.

Renderer side (`components/autofill/content/renderer/password_autofill_agent.cc`):
- `ApplyFillDataOnParsingCompletion` (`:1628-1675`) stores the data for fill-on-select.
  - If waiting, it may show suggestions on the focused field.
  - Otherwise it calls `FillCredentialsAutomatically` (`:1985-2100`).
- The renderer checks the frame tree again on its own: any ancestor frame that is not a PSL match
  blocks the fill, and so does a fenced frame (`IsInCrossOriginIframeOrEmbeddedFrame` `:396-421`).
- The field must be enabled and not readonly (`IsElementEditable`, `form_autofill_util.cc:2322-2324`).
- A non-empty username the page typed is kept, unless it is a known placeholder such as
  "username or email" (`prefilled_values_detector.cc`), or a server hint marks it as a placeholder.
- **No visibility check on this path.** I found no `IsFocusable` or visibility check in
  `FillCredentialsAutomatically`. Hidden fields are kept out only by the parser's interactability
  rule, and only when visible fields also exist.
- **Protected value (gatekeeper).** A fill on load calls `SetSuggestedValue` plus
  `SetAutofillState(kAutofilled)`, not `SetValue` (`FillFieldAutomatically` `:2365-2386`). A
  suggested value is painted, but page JS cannot read it (blink
  `third_party/blink/public/web/web_form_control_element.h:141-149`).
  - `PasswordValueGatekeeper` (`:738-781`) promotes the suggested value to the real value
    (`SetAutofillValue`) on the *first user gesture anywhere in the frame*.
  - That gesture is blink's `WebAutofillClient::UserGestureObserved`
    (`third_party/blink/public/web/web_autofill_client.h:84-85`, raised from
    `local_frame_client_impl.cc:1111`), which goes through `AutofillAgent::UserGestureObserved`
    (`autofill_agent.cc:1074`) and `PasswordAutofillAgent::UserGestureObserved` (`:1375-1379`).
  - The gatekeeper resets on each new document (`:1603-1620`).
- A fill on load skips fields the user typed into (`:2370-2372`). If JS clears a filled field, the
  cached value is put back (`TryFixAutofilledForm` `:2346-2363`).
- Filling chosen from the suggestion list sets the real value at once (`DoFillField` `:1070-1074`).
  That is the user's gesture.
- Several forms on a page: each form gets its own `PasswordFormManager` and its own fill data.
  Each is filled on its own, up to `kMaxTimesAutofill = 5` refills per form manager
  (`password_form_manager.h:92`).
- Frame eligibility in the renderer: only http and https documents, and only when blink's
  `SecurityOrigin::CanAccessPasswordManager()` allows it. about:blank and about:srcdoc are blocked
  (crbug 756587) (`FrameCanAccessPasswordManager` `:1357-1368`).
- Frame eligibility in Chrome's client: no fill with opaque origins (`kPasswordBlockOpaqueOrigins`),
  no fill with **certificate errors** on the main frame, and none in guest profiles
  (`chrome_password_manager_client.cc:295-330`).
- Autofill (addresses and cards) can fill *across* iframes under a same-origin or top-origin policy
  (`docs/security/autofill-across-iframes.md`). PWM does not. It works per renderer form and per
  frame.

### 4.2 PSL, affiliation, grouping and Android apps

- `signon_realm` is scheme + host + port (`GetSignonRealm`, `form_data_parser.cc:1591-1599`). So
  http and https are separate realms.
- PSL match (`password_store/psl_matching_helper.cc:128-157`): the same eTLD+1 (private registries
  included), the same scheme and the same port. google.com is special: only accounts. and
  myaccount.google.com (`:21-24`, `:141-146`).
  - PSL matches show as suggestions, never fill on load, and are saved as an exact copy once used
    (3.3).
- Affiliation (`components/affiliations/core/browser/`): the client asks Google's
  `https://www.googleapis.com/affiliation/v1/affiliation:lookupByHashPrefix` with a **16-bit hash
  prefix** of each facet (`hash_affiliation_fetcher.cc:32`, `:171-194`). Results are cached in
  `affiliation_database.cc`.
  - Facets are `https://host` or `android://<cert-hash>@<package>`
    (`affiliation_utils.h:28`, `:168-177`).
  - Affiliated web matches wait for the user and save silently. An Android-app credential
    (`android://`) is offered like an exact match for filling on load. It is excluded from the
    `kAffiliatedWebsite` wait rule (`password_form_filling.cc:236-239`).
- "Grouped" credentials come from the affiliation service's grouping info
  (`affiliation_service.h:80-85`). They are weaker than affiliated ones. They appear only as
  "cross-domain" suggestions and need a confirmation popup before filling
  (`password_cross_domain_confirmation_popup_controller.h:14-16`,
  `password_suggestion_generator.cc:626-640`).
- Digital Asset Links: I found no DAL fetching in `components/password_manager` or
  `components/affiliations`. App-to-site links are built server-side, with DAL as one input. That
  is my understanding from Google's public docs, not from the code.
- HTTP to HTTPS: HTTP credentials are moved to HTTPS when the site uses HSTS, or copied when it does
  not (`http_password_store_migrator.h`).

---

## 5. Generated passwords

### 5.1 When it is offered

- Generation is enabled per frame (`password_generation_frame_helper.cc:107-142`). It is off on
  google.com, off when the store cannot save, and off when saving is disabled by the user or by
  policy. Then `PasswordFeatureManager::IsGenerationEnabled()` must be true. **In Chromium's
  implementation that requires password sync to be active** (`password_feature_manager_impl.cc:28-36`).
  An embedder supplies its own feature manager (`password_manager_client.h:414-415`) and can decide
  this itself.
- *Automatic* offer: only when the new-password field is "reliable" (server or autocomplete), the
  form is not blocklisted, and webauthn is not set (`password_form_manager.cc:1249-1258`, parser
  `:1540-1560`).
  - The renderer then shows it on focus while the field holds 5 characters or fewer
    (`kMaximumCharsForGenerationOffer = 5`, `password_generation_agent.h:42`;
    `password_generation_agent.cc:563-608`).
- *Manual* offer (context menu) works on any password-looking field (`form_data_parser.cc:1448-1456`).

### 5.2 Generator rules and learned site requirements

- `generation/password_generator.cc`:
  - The default length is 15 (`:25`). It matches the server's default.
  - Ambiguous characters are removed: `l I 1 O 0 o` (`:28-42`).
  - The default spec needs at least 1 lowercase, 1 uppercase and 1 digit, with no symbols (`:44-77`).
  - The symbol set, when a spec allows symbols, is `-_.:!` (`:42`).
  - Target length is 15, raised to the spec's minimum and capped at its maximum and at 200
    (`:98-107`).
  - It maximizes entropy and rejects runs of `--` or `__` (`IsDifficultToRead` `:82-86`).
- Requirements come from two sources:
  - Per field, in the Autofill server response (`password_requirements` in the prediction). These
    are stored by `PasswordRequirementsService::AddSpec` (`password_generation_frame_helper.cc:72-98`).
  - Per domain, from a static file on
    `https://www.gstatic.com/chrome/autofill/password_generation_specs/<version>/<hash-prefix>`
    (`password_requirements_spec_fetcher_impl.cc:96-100`). It is keyed by an MD5 hash prefix of
    eTLD+1 (`:70-77`), so the exact domain is not revealed.
  - The data is learned from the crowdsourced length and symbol votes in 1.4.

### 5.3 Presave before submit

- When the user accepts, the renderer sends `PresaveGeneratedPassword`
  (`password_generation_agent.cc:408`). It sends it again on every edit (`:665`, `:720`). The
  browser writes the credential into the store *before* submission
  (`PasswordGenerationManager::PresaveGeneratedPassword`, `password_generation_manager.cc:391-421`).
  - If the username clashes with a stored one, the username is cleared so nothing is overwritten.
  - Later edits replace the presaved row (`UpdateReplace`).
- `PasswordFormManager::PresaveGeneratedPassword` is at `password_form_manager.cc:702`. A backup
  variant exists at `:1417`.
- If the field is edited below the minimum length, generation is dropped
  (`PasswordNoLongerGenerated`), and the presaved row is removed by the generation manager.
- After a successful submit, a generated password is saved with no prompt (3.3).

---

## 6. Storage at rest

### 6.1 LoginDatabase (desktop, iOS, ChromeOS)

- `password_store/login_database.cc` is SQLite. The schema is version 43, and the oldest compatible
  version is 40 (`:80-83`). Columns come from `InitializeBuilders` (`:375-597`).
- Current `logins` columns:
  - Unique key: `origin_url`, `username_element`, `username_value`, `password_element`,
    `signon_realm`.
  - `action_url`, `password_value BLOB` (encrypted), `submit_element`.
  - Dates: `date_created`, `date_last_used` (v25), `date_password_modified` (v30),
    `date_last_filled` (v42), `date_received`.
  - `blacklisted_by_user`, `scheme`, `password_type`, `times_used`, `form_data BLOB`,
    `display_name`, `icon_url`, `federation_url`, `skip_zero_click`, `generation_upload_status`,
    `possible_username_pairs BLOB`.
  - `id` is an INTEGER PRIMARY KEY AUTOINCREMENT (v20, v26).
  - `moving_blocked_for BLOB`, the sharing fields (`sender_email`, `sender_name`,
    `sender_profile_image_url`, `sharing_notification_displayed`), `keychain_identifier BLOB`
    (v39), and `actor_login_approved` (v43).
  - There is an index on `signon_realm`.
  - **There is no GUID column.** The identity is the autoincrement `id` plus the 5-column unique
    key. There is no generic `date_modified` either. `date_password_modified` changes only when the
    password changes.
- Side tables: `insecure_credentials` (leaked, phished, weak, reused, with a foreign key to
  logins), `password_notes`, `stats` (bubble statistics), and sync metadata tables
  (`passwords_sync_entities_metadata`, `passwords_sync_model_metadata`).
- There are two stores: profile and account (`GetProfilePasswordStore` and
  `GetAccountPasswordStore` on the client).

### 6.2 Encryption

- The column is encrypted through `os_crypt_async::Encryptor` (`login_database.cc:1091-1124`).
- Mac (`login_database_mac.cc`, `components/os_crypt/async/browser/keychain_key_provider.mm:28-65`):
  - A random password is kept in the login Keychain as a generic item. The service is
    "Chrome Safe Storage" (branded) or "Chromium Safe Storage", with account "Chrome" or
    "Chromium" (`components/os_crypt/common/keychain_password_mac.mm:31-38`).
  - The key is PBKDF2-HMAC-SHA1(password, salt "saltysalt", 1003 iterations), 16 bytes, used for
    AES-128-CBC. Ciphertext gets the prefix `v10`.
  - If Keychain access is denied, the key is "temporarily unavailable" and the store cannot decrypt.
  - Every app that ships an unpatched content layer and uses this provider gets *its own* Keychain
    item only if the service name differs. The name comes from branding. Project B would want its
    own name (unverified whether this can be configured without a patch; check
    `KeychainPassword::GetServiceName`).
- Windows uses DPAPI to protect an AES-256-GCM key, with prefix `v10` (`dpapi_key_provider.cc:31-32`).
- Linux uses the Secret Service or the portal, or the hardcoded key "peanuts" when neither is there
  (`posix_key_provider.cc:17-19`, `freedesktop_secret_key_provider.h:158-162`).
- iOS writes `keychain_identifier`.

### 6.3 Android

- **On Android, LoginDatabase is not compiled at all** (`password_store/BUILD.gn:80-117`:
  `is_android` takes a branch that leaves out `login_database.cc` and
  `password_store_built_in_backend.cc`).
- Chrome on Android stores passwords in Google Play Services (the "UPM" backends in
  `chrome/browser/password_manager/android/password_store_android_*_backend.cc`). That is
  //chrome, and it needs GMS.
- An Android content embedder must therefore supply its own `PasswordStoreBackend`. That interface
  has 17 pure virtuals (`password_store_backend.h`): `InitBackend`, `Shutdown`,
  `GetAllLoginsAsync`, `GetAutofillableLoginsAsync`, `FillMatchingLoginsAsync`,
  `GetGroupedMatchingLoginsAsync`, `AddLoginAsync`, `UpdateLoginAsync`, `RemoveLoginAsync`,
  `DisableAutoSignInForOriginsAsync`, `GetSmartBubbleStatsStore`, and others. A `PasswordStore`
  takes any backend (`password_store.h:52`).
  - **This is the natural seam for "Go owns the passwords" on every platform.**

---

## 7. Security traps and the code that guards each

| Trap | Guard | Where |
|---|---|---|
| Page JS reads a password that was filled on load, without the user's intent | Suggested value plus `PasswordValueGatekeeper`: the real value is set only after a user gesture | `password_autofill_agent.cc:738-781`, `:2365-2386` |
| Harvesting through hidden or invisible fields and forms | Parser prefers interactable fields (`Interactability`). Readonly fields are skipped. Not-password regexes. No fill on load for a form without a current-password field. The gatekeeper. **There is no hard visibility check in the automatic fill path.** | `form_data_parser.cc:164-187`, `:689-702`, `password_form_filling.cc:249-251` |
| Fill into a cross-origin iframe | The browser waits (`HasCrossOriginAncestor`). The renderer refuses if any ancestor is not a PSL match, or the frame is fenced | `password_form_filling.cc:246-247`, `password_autofill_agent.cc:396-421` |
| about:blank, srcdoc, data: frames | Renderer `FrameCanAccessPasswordManager`. The browser *kills the renderer* if a form URL is about: or data:, or fails `ChildProcessSecurityPolicy::CanAccessDataForOrigin` | `password_autofill_agent.cc:1357-1368`, `password_manager/content/browser/bad_message.cc:34-73`, `content_password_manager_driver.cc:126-138` |
| Prerendering pages | `CheckFrameNotPrerendering` kills the renderer on PWM messages | `bad_message.cc:75-83` |
| HTTP pages | No fill on load (`IsCommittedMainFrameSecure`). http and https are separate realms. No saving on http after an https submit for the same host | `password_form_filling.cc:256-257`, `password_manager.cc:1530-1535` |
| TLS errors | Chrome's `IsFillingEnabled` returns false when the main frame has a cert error | `chrome_password_manager_client.cc:317-329` |
| Opaque origins | No filling (`kPasswordBlockOpaqueOrigins`) | `chrome_password_manager_client.cc:298-302` |
| Mixed content (https page, http action) | **Only a metric** in PWM (`is_mixed_content_form_`). I found no blocking | `password_form_metrics_recorder.cc:438-440`, `:769` |
| Clickjacking the suggestion popup | Popup clicks are ignored for the first 500 ms (crbug 40058217). This is *Chrome UI code* | `chrome/browser/ui/autofill/autofill_suggestion_controller.h:53-57` |
| Wrong site through PSL, affiliation or grouping | Those never fill on load. Grouped matches need a cross-domain confirmation popup. google.com PSL is limited | 4.2 refs |
| Credential leak | After a successful login, `LeakDetectionDelegate` checks the pair with `https://passwordsleakcheck-pa.googleapis.com/v1/leaks:lookupSingle`. It sends a 26-bit username-hash prefix, and the credential hashed with scrypt and encrypted with an EC commutative cipher (private set intersection). It uses an OAuth token when signed in, else an API key. It needs a pref and an **IdentityManager** (`CHECK(identity_manager)`) | `leak_detection_request.h:39-40`, `encryption_utils.h:15`, `encryption_utils.cc:103-145`, `leak_detection_delegate.cc:119-141`, `leak_detection_check_factory_impl.cc:44` |
| Phishing and password reuse | `PasswordReuseDetector` keeps hashes of saved, Gaia and enterprise passwords and checks the typed suffix (at least 4 characters). On a match, Safe Browsing `PasswordProtectionService` checks the page and warns. Focusing a password field also sends a reputation ping. The warning UI and service are in //chrome and safe_browsing | `password_reuse_detector.h:28`, `password_reuse_detector_impl.cc:166-175`, `password_autofill_agent.cc:1328-1355` |
| Saving the user's Google account password into the store | `SyncCredentialsFilter` / `StoreResultFilterAllowsSaving` | `password_manager.cc:1723-1728` |
| Renderer spoofing a generated password | `CheckGeneratedPassword` in bad_message | `bad_message.cc:85-` |

---

## 8. Can a content-layer embedder reuse this without //chrome?

### 8.1 Layering says yes

- `components/password_manager/DEPS`: core may not include `content/` (`-components/password_manager/content`).
- `core/browser/DEPS` allows `+content/public/browser`, but only a few specific files use it.
- `content/browser/DEPS` uses only `content/public` and `components/autofill/content`.
- Nothing in `components/password_manager` or `components/autofill` includes `//chrome`.
  Chromium's DEPS checks enforce this.
- Precedents:
  - **iOS WebView (`ios/web_view`)** is a non-chrome embedder that reuses PWM and Autofill fully.
    `WebViewPasswordManagerClient` is about 580 lines (`ios/web_view/internal/passwords/web_view_password_manager_client.{h,mm}`).
    It is on WebKit, not content.
  - **Android WebView** uses `AutofillAgent` with **no** `PasswordAutofillAgent`
    (`android_webview/renderer/aw_render_frame_ext.cc:156`). It hands everything, passwords
    included, to the Android Autofill framework through `AndroidAutofillClient` and
    `AndroidAutofillManager` (`components/android_autofill/browser/`, about 5.8k lines;
    `components/autofill/README.md:80-98` table).
  - **headless**: no autofill (no autofill files in `headless/lib/browser`).
  - I know of no content-layer embedder outside //chrome that runs the PWM.

### 8.2 What the embedder must write

The interfaces were counted with grep for `= 0;`:
- `PasswordManagerClient`: 625-line header, 92 virtuals, **39 pure**
  (`password_manager_client.h`). Examples:
  - `PromptUserToSaveOrUpdatePassword`, `ShowManualFallbackForSaving`,
    `PromptUserToChooseCredentials`, `NotifyUserCredentialsWereLeaked`, `AutomaticPasswordSave`;
  - `GetPrefs`, `GetSyncService` (null is fine), `GetIdentityManager` (null is fine except for leak
    check), `GetAffiliationService`, `GetProfilePasswordStore`, `GetAccountPasswordStore`,
    `GetPasswordReuseManager`, `GetURLLoaderFactory`, `GetFaviconService`,
    `GetLastCommittedURL` and `Origin`, `GetStoreResultFilter`, `GetMetricsRecorder`,
    `GetUkmSourceId`, and others.
  - `StubPasswordManagerClient` (`stub_password_manager_client.cc`, 265 lines) shows that most can
    return null or defaults.
  - Chrome's version is `chrome_password_manager_client.cc`, 2311 lines plus a 659-line header.
- `AutofillClient`: 953-line header, 122 virtuals, **43 pure**
  (`components/autofill/core/browser/foundations/autofill_client.h`). Most concern addresses,
  payments, sync, translate and UKM, and can return null.
- `ContentAutofillClient` adds 2 more: `CreateManager` and `GetContentCredentialManager`
  (`components/autofill/content/browser/content_autofill_client.h`).
  - The embedder picks the `AutofillManager`. `BrowserAutofillManager` is needed to get *server
    predictions* (crowdsourcing), so PWM benefits from it even if address and card filling is off.
  - `AndroidAutofillManager` is the lighter option that WebView uses.
  - Chrome's `chrome_autofill_client.cc` is 1770 lines.
- `PasswordManagerDriver` is supplied as `ContentPasswordManagerDriver` (756 lines) plus a factory.
  **The embedder does not write these.** `PasswordFeatureManager` has 4 pure virtuals.
  `PasswordStoreBackend` has 17 (see 6.3).
- Wiring, all through public content APIs (no content patch):
  - Renderer: in `ContentRendererClient::RenderFrameCreated`, create `PasswordAutofillAgent`,
    `PasswordGenerationAgent` and `AutofillAgent` (`chrome/renderer/chrome_content_renderer_client.cc:745-751`).
  - Browser: in `RegisterAssociatedInterfaceBindersForRenderFrameHost`, bind
    `mojom::AutofillDriver`, `mojom::PasswordManagerDriver` and `mojom::PasswordGenerationDriver`
    (`chrome/browser/chrome_content_browser_client_receiver_bindings.cc:421-462`).
  - Per WebContents: create the Autofill client and the PWM client, and call
    `ContentPasswordManagerDriverFactory` with the client
    (`content_password_manager_driver_factory.h:66-67`).
  - Forward Autofill predictions into `PasswordManager::ProcessAutofillPredictions`
    (`chrome_password_manager_client.cc:2040-2071`).
  - Build the profile services: `PasswordStore` over `PasswordStoreBuiltInBackend`
    (LoginDatabase) or a custom backend, `OSCryptAsync`, `PrefService` with
    `PasswordManager::RegisterProfilePrefs` and the Autofill prefs, and optionally the affiliation
    service and `PasswordRequirementsService`.
  - Override `IsCommittedMainFrameSecure`, or fill on load never happens (4.1).
- UI the embedder must build. All of Chrome's is in //chrome:
  - the save and update prompt, and "Never";
  - the suggestion dropdown, with its 500 ms early-click guard;
  - the generation popup;
  - the manual-fallback key icon;
  - the leak and reuse warnings, if wanted;
  - a password settings and management page.
  - Chrome's UI and glue for this is about 29k lines in `chrome/browser/password_manager` (Android
    bridges included), about 10.6k in `chrome/browser/ui/passwords`, about 8.3k in
    `ui/views/passwords`, and about 25.6k in `chrome/browser/ui/autofill` (non-test line counts).

### 8.3 What it pulls in

- Non-test lines of C++ (`.cc`, `.h`, `.mm`) per component in this checkout:
  - `password_manager/core`: about 73k
  - `password_manager/content`: about 2.3k
  - `autofill/core`: about 215k
  - `autofill/content`: about 20k
  - `affiliations`: about 6k
  - `os_crypt`: about 3.4k
  - `sync`: about 70k
  - `signin`: about 44k
  - `safe_browsing/core`: about 47k (only `core/common` is a direct dep)
  - `webdata`: about 1.9k
- `password_manager/core/browser:browser` depends directly on: autofill/core/browser, affiliations,
  sync, os_crypt, policy/core/browser, prefs, pref_registry, favicon, metrics/ukm, device_reauth,
  safe_browsing/core/common, url_formatter, user_prefs, webauthn passkey_model, strings, and
  keyed_service/content (`core/browser/BUILD.gn:153-243`).
- `autofill/core/browser` is one big static library. It depends on, among others: optimization_guide,
  history/core, translate, language, payments, wallet, signin identity_manager, sync,
  sync_device_info, variations, feature_engagement, leveldb_proto, strike_database, consent_auditor,
  image_fetcher, security_state, webdata, os_crypt, and one_time_tokens (grep of
  `components/autofill/core/browser/BUILD.gn`).
  - There is no password-only Autofill target. PWM needs the whole of `autofill/core/browser`.
- Rough embedder code size, my estimate (not measured):
  - C++ glue: about 2,000 to 4,000 lines, covering the two clients, wiring, profile services and
    prediction forwarding. Add a custom `PasswordStoreBackend` (about 500 to 1,000 lines) if Go owns
    storage or on Android.
  - Native UI: the prompt, dropdown and generation popup on each platform.
  - The link cost is large. That means most of `components/autofill`, `password_manager`, `sync`,
    `signin` and `optimization_guide` cores, several hundred thousand source lines. I did not
    measure the binary-size delta.
- Google services: the crowdsourcing server, affiliation server, leak check and gstatic spec
  fetches all need Google API keys or endpoints. Chromium's policy is that Google API keys are not
  for redistribution in third-party builds (see the API keys doc below). Without keys, PWM still
  works on local heuristics and autocomplete. It loses server predictions, affiliation, leak check
  and server-learned generation specs. Whether a key would work in Project B is a policy question.
  Nothing in the code decides it.

---

## Docs (not fetched this session)

- Layered components: https://www.chromium.org/developers/design-documents/layered-components-design (cited by `components/password_manager/README.md`)
- Password form styles that Chromium understands: https://www.chromium.org/developers/design-documents/form-styles-that-chromium-understands/
- Create amazing password forms: https://www.chromium.org/developers/design-documents/create-amazing-password-forms/
- Sign-in form best practices: https://web.dev/articles/sign-in-form-best-practices
- Sign-up form best practices: https://web.dev/articles/sign-up-form-best-practices
- HTML autocomplete tokens: https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#autofilling-form-controls%3A-the-autocomplete-attribute (cited in `form_data_parser.cc:62`)
- Change-password URL: https://w3c.github.io/webappsec-change-password-url/
- Chromium API keys: https://www.chromium.org/developers/how-tos/api-keys/
- Digital Asset Links: https://developers.google.com/digital-asset-links
- Password Checkup protocol paper (leak check): https://www.usenix.org/conference/usenixsecurity19/presentation/thomas
- In-tree: `components/autofill/README.md` (submission, votes, embedders), `components/autofill/content/renderer/README.md`, `docs/security/autofill-across-iframes.md`
- Design-doc short links in code (possibly Google-internal): https://goo.gl/Mc2KRe (UFF two usernames), https://goo.gl/ERvoEN (password "likely" rules), go/autofill-metadata-upload (randomized encoder)

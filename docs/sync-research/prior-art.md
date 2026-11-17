# Prior art: end-to-end encrypted sync on storage the user controls

Survey for a browser sync design: Go core, macOS and Android, keys the user holds, storage the user controls (iCloud Drive, Google Drive, Dropbox, WebDAV, git, S3, LAN device-to-device, self-hosted). Researched 2026-11-17. Every claim carries an inline source link. Anything not confirmed from a source is marked **(unverified)**. The cross-product synthesis comes first, then the per-product detail in three parts: browsers, user-storage apps (part 1), and user-storage apps plus UX references (part 2).

## Synthesis across products

### At a glance

| Product | Storage | Root secret | Join a new device | Conflicts | Recovery when the secret is lost |
|---|---|---|---|---|---|
| Firefox Sync | Mozilla servers, or self-hosted syncstorage-rs | Account password, stretched to kB | Sign in, or pairing QR approved on an existing device ([pairing](https://mozilla.github.io/ecosystem-platform/explanation/pairing-flow-architecture)) | Bookmarks: two-way merge, newer item wins ([Dogear](https://mozilla.github.io/dogear/merging.html)). Others: three-way merge against a mirror ([app-services](https://mozilla.github.io/application-services/book/design/sync-overview.html)) | Reset drops kB unless the user saved a recovery key ([onepw](https://github.com/mozilla/fxa-auth-server/wiki/onepw-protocol), [Mozilla blog](https://blog.mozilla.org/services/2018/09/26/account-recovery-keys-in-firefox-accounts/)) |
| Chrome Sync | Google servers | Google account keys by default; optional custom passphrase ([proto](https://chromium.googlesource.com/chromium/src/+/main/components/sync/protocol/nigori_specifics.proto)) | Sign in, and type the passphrase if one is set ([Google blog](https://blog.google/technology/safety-security/google-password-manager-passkeys-update-september-2024/)) | Remote wins except deletes ([bridge](https://chromium.googlesource.com/chromium/src/+/main/components/sync/model/data_type_sync_bridge.cc)) | Resetting the passphrase wipes the server copy ([Chrome Help](https://support.google.com/chrome/answer/165139)) |
| Brave Sync v2 | Brave servers (Chromium engine) | Random 256-bit seed shown as BIP39 words ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)) | Enter or scan the chain code ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)) | Chromium engine | None. Unused chains are wiped after a year ([notice](https://community.brave.app/t/notice-unused-sync-accounts-will-now-expire-after-one-year-of-inactivity/534779)) |
| Orion | The user's iCloud ([Kagi](https://blog.kagi.com/orion-features)) | Apple account (unverified beyond the Kagi page) | Same Apple account (unverified) | Apple's (unverified) | Apple's (unverified) |
| Floccus | WebDAV, Nextcloud, Google Drive, git and more | Passphrase, which directly derives the content key ([Crypto.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/Crypto.ts)) | Retype backend settings and the passphrase | Three-way against the last synced tree, local wins ties, 20% safety stop ([Default.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts)) | None |
| Joplin | Dropbox, OneDrive, WebDAV, S3, Joplin Server | Random master keys wrapped by a password ([e2ee spec](https://joplinapp.org/help/dev/spec/e2ee)) | Retype the password | Losing copy goes to a Conflicts notebook ([conflict](https://joplinapp.org/help/apps/conflict)) | None |
| KeePass family | A .kdbx file on any cloud | Password and/or key file | Open the same file | Match by UUID, newer wins, loser kept in history, tombstones for deletes ([KeePass](https://keepass.info/help/v2/sync.html)) | None |
| Bitwarden | Bitwarden or a self-hosted server | Master password wraps a random user key ([KDF](https://bitwarden.com/help/kdf-algorithms/)) | Password, or Log in with device ([LWD](https://bitwarden.com/help/log-in-with-device/)) | Server revision check | Emergency access, org recovery ([emergency](https://bitwarden.com/help/emergency-access/)) |
| Cryptomator | Any cloud folder | Password wraps the vault keys ([architecture](https://docs.cryptomator.org/security/architecture/)) | Open the vault, type the password | The cloud makes conflict copies ([vault](https://docs.cryptomator.org/security/vault/)) | Recovery key, which can be shown again with the password ([docs](https://docs.cryptomator.org/desktop/password-and-recovery-key/)) |
| Anytype | any-sync nodes, self-hostable, or LAN | 12-word phrase ([key](https://doc.anytype.io/anytype/basics/key)) | Scan the phrase as a QR code | CRDT DAG of changes | The phrase |
| Syncthing | Peer devices, plus optional untrusted encrypted peers ([docs](https://docs.syncthing.net/users/syncing.html)) | Device certificates; a folder password for untrusted peers | Exchange device IDs, accept on both sides ([device IDs](https://docs.syncthing.net/dev/device-ids.html)) | Conflict copy files | Not applicable |
| Ente, Proton | Vendor servers | Password KEK wraps the master key; a 24- or 12-word recovery key ([Ente](https://ente.com/architecture/), [Proton](https://proton.me/support/recover-encrypted-messages-files)) | Sign in | Not applicable | The recovery key; a reset without one loses the data |

### Patterns that hold up

1. **Use a random data key, wrapped by whatever the user holds.**
   - Joplin, Bitwarden, Standard Notes, Ente, Cryptomator and Chromium's Nigori keybag wrap random data keys under a password- or seed-derived key ([Joplin](https://joplinapp.org/help/dev/spec/e2ee), [Bitwarden](https://bitwarden.com/help/kdf-algorithms/), [Ente](https://ente.com/architecture/), [Cryptomator](https://docs.cryptomator.org/security/architecture/), [Nigori](https://chromium.googlesource.com/chromium/src/+/main/components/sync/protocol/nigori_specifics.proto)). Changing the password then re-wraps a few kilobytes.
   - Floccus and xBrowserSync derive the content key straight from the passphrase, so a change means re-encrypting everything ([Floccus](https://github.com/floccusaddon/floccus/blob/develop/src/lib/Crypto.ts), [xBrowserSync](https://github.com/xbrowsersync/app/blob/master/src/modules/shared/crypto/crypto.service.ts)).
   - Cryptomator Hub adds a per-device layer: a device key unwraps the user key, so adding or removing a device never touches the vault keys ([Hub 1.3](https://cryptomator.org/blog/2023/12/01/hub-1.3.0)).
   - A random seed avoids weak password stretching. Firefox ran 1,000 PBKDF2 rounds until 2024 ([micropipes](https://micropipes.com/2024/10/28/mozilla-accounts-password-hashing-upgrades/)).

2. **Join a device by having an existing device approve it, over a blind channel.** Firefox pairing, Bitwarden Log in with device and Signal linking work the same way ([Firefox](https://mozilla.github.io/ecosystem-platform/explanation/pairing-flow-architecture), [Bitwarden](https://bitwarden.com/help/log-in-with-device/), [Signal](https://signal.org/blog/a-synchronized-start-for-linked-devices/)):
   - The new device makes a one-time key pair.
   - The QR code or a fingerprint phrase binds the two devices.
   - The existing device encrypts the data key to the new device's public key.
   - The relay carrying the exchange cannot read it, and requests expire. Bitwarden's expire after 15 minutes.

   Nothing in this needs a vendor server. A LAN link, a file in the user's own storage, or any dumb relay can carry it.

   There are two weaker alternatives. Anytype's QR code carries the master phrase itself ([Anytype](https://doc.anytype.io/anytype/basics/key)). Brave's chain code is both the join code and the key, and in its 2022 incident users who copied the sample codes from Brave's help page ended up sharing chains with strangers ([Brave](https://community.brave.app/t/sync-incident-affecting-a-small-number-of-users/328839)).

   QR-code phishing of Signal links in 2025 and 2026 shows the approval screen must name the requesting device ([GTIG](https://cloud.google.com/blog/topics/threat-intelligence/russia-targeting-signal-messenger)).

3. **On dumb storage, write many small objects, and give each device its own files.**
   - Cryptomator's flat directory-ID layout, Anytype's change DAG, Joplin's one-file-per-item layout and Firefox's one-record-per-device tabs all fit plain file storage ([Cryptomator vault](https://docs.cryptomator.org/security/vault/), [Firefox tabs TTL](https://bugzilla.mozilla.org/show_bug.cgi?id=1719392)).
   - Whole-file sync merged by modification time loses edits and leaves conflict copies. Logseq on iCloud, Syncthing's conflict files and KeePass on Android all show this ([Logseq](https://discuss.logseq.com/t/im-using-logseq-with-icloud-but-experiencing-data-loss-or-file-conflict-errors-whats-going-on/13393), [Syncthing](https://docs.syncthing.net/users/syncing.html), [ctrl.blog](https://www.ctrl.blog/entry/keepass-file-conflicts-android/)).
   - Where several writers share one file, the tools fall back on advisory lock files: Floccus refreshes its lock every 2 minutes and takes over a stale one after 15, and Joplin uses lock files plus a versioned target upgrade ([S3.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/S3.ts), [Joplin lock spec](https://joplinapp.org/help/dev/spec/sync_lock)).
   - Append-only per-device logs avoid needing compare-and-set on backends that lack it. This is a design inference, not something a surveyed product proves.

4. **Merge three ways against a stored base, choose the policy per data type, and never lose data silently.**
   - Firefox's Rust engines and Floccus keep the last synced state as the base ([app-services](https://mozilla.github.io/application-services/book/design/sync-overview.html), [Floccus](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts)).
   - KeePass's model fits passwords and bookmarks: match by UUID, newer wins, the loser goes into history, deletes become tombstones ([KeePass](https://keepass.info/help/v2/sync.html)).
   - Firefox's Dogear knowingly reverts some edits ([Dogear](https://mozilla.github.io/dogear/merging.html)).
   - Obsidian merges Markdown text automatically ([Obsidian](https://obsidian.md/help/sync/troubleshoot)). Joplin and Standard Notes keep the losing copy where the user can see it.
   - Floccus refuses a sync that would delete or add more than 20% of the bookmarks. Vivaldi's December 2024 outage, where empty new devices overwrote good copies, shows why a new or empty device must never win a reset ([Vivaldi](https://forum.vivaldi.net/post/802575)).

5. **Plan recovery from day one, and say plainly what a reset costs.**
   - Every user-storage app surveyed has no recovery beyond "keep your password or key".
   - Firefox drops kB on a password reset unless the user saved a recovery key. Chrome and Vivaldi wipe the server copy. Apple destroys the escrowed keychain after 10 failed attempts ([Apple escrow](https://support.apple.com/guide/security/escrow-security-for-icloud-keychain-sec3e341e75d/web)).
   - Ente and Proton show the better user experience: a recovery key at setup, viewable again from any signed-in device, and an honest warning that a reset without it loses the data ([Ente](https://ente.com/help/photos/faq/security-and-privacy), [Proton](https://proton.me/support/recover-encrypted-messages-files)).
   - Tailnet Lock's 10 break-glass secrets are a model for a printed emergency kit ([Tailscale](https://tailscale.com/kb/1226/tailnet-lock)).

6. **Decide early what the storage provider may see, and fail closed.**
   - Several designs leak metadata even with end-to-end encryption:
     - Firefox: its plaintext `meta/global` record and collection names ([overview](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html)).
     - Brave: plaintext parent IDs ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
     - Joplin: plaintext ids, parent ids and timestamps ([BaseItem.ts](https://cdn.jsdelivr.net/npm/@joplin/lib@3.5.1/models/BaseItem.ts)).
     - Obsidian Sync: deterministic hashes ([Obsidian security](https://obsidian.md/help/sync/security)).
     - Syncthing's untrusted mode: file sizes ([Syncthing](https://docs.syncthing.net/users/syncing.html)).
   - Anytype's February 2026 bug uploaded local-only spaces after a corrupted settings file, so a missing setting must mean "do not sync" ([post-mortem](https://community.anytype.io/t/port-mortem-local-only-mode-and-sync-bug/30158)).
   - Apple pulled Advanced Data Protection for new UK users in February 2025. A provider's encryption switch is not the user's key ([MacRumors](https://www.macrumors.com/2025/02/21/apple-pulls-encrypted-icloud-security-feature-uk/)).

### Gaps and open items

- I found no source for a 2023 Brave change that made sync chains depend on a Brave server. It is recorded as unverified in the Brave section. Brave is reported to be building an account-based Sync v3 (details in that section).
- Arc's help pages and Brave's Delete Sync Account page refused fetches, so those parts rest on search snippets.
- Whether Google has fixed the August 2026 synced-passkey flaws is unknown.
- Obsidian's scrypt parameters and xBrowserSync's QR code are unverified.
- No surveyed browser except Orion syncs through storage the user owns, and Orion works only on Apple devices. The closest real prior art for this design is therefore Floccus for the bookmark merge, KeePass for record merge semantics, Cryptomator for the encrypted file layout, and Bitwarden, Signal and Firefox for the join flow.

---

## Browsers

Every product below stores ciphertext on servers the browser vendor runs. Two exceptions sit close to our model: Orion syncs through the user's own iCloud, and Firefox and Brave let a user self-host the storage server. Each product uses the same six headings.

### Firefox Sync (Mozilla accounts)

**What syncs.** Bookmarks, history, open tabs, passwords, add-ons "and the like" ([syncstorage-rs client overview](https://mozilla-services.github.io/syncstorage-rs/sync-client/overview.html)). Clients store *records* (Basic Storage Objects, BSOs) inside *collections*. Each record carries public metadata such as last-modified time, so clients fetch only what changed ([Mozilla services docs](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html)). A BSO has `id`, `modified` and `payload`, and the payload is JSON `{ciphertext, IV, hmac}` ([storage format 5](https://mozilla-services.readthedocs.io/en/latest/sync/storageformat5.html)). Collection names beyond those listed (clients, forms, prefs, addresses, creditcards) come from my knowledge of the Firefox source and were not re-verified here (unverified).

**Where stored.** Mozilla's syncstorage servers. The server is described as a "dumb shared whiteboard". It stores data, tracks basic metadata and authenticates access, and has no notion of a sync session ([overview](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html)). Quota is 2.5 GB per collection per account in the current docs ([syncstorage-rs](https://mozilla-services.github.io/syncstorage-rs/sync-client/overview.html)), versus 2 GB in the archived docs ([readthedocs](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html)). Operators can see connection logs plus the types, number and rough size of items ([overview](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html)).
- *Self-hosting.* syncstorage-rs is Mozilla's Rust storage server, with Tokenserver as a second service. It supports MySQL 8, PostgreSQL or Spanner, and has docker targets. Users point Firefox at it with `identity.sync.tokenserver.uri` in about:config ([syncstorage-rs docs](https://mozilla-services.github.io/syncstorage-rs/), [repo](https://github.com/mozilla-services/syncstorage-rs)). The docs still describe devices as linked through a Mozilla account. Self-hosting therefore replaces storage but, by default, not identity or key management (whether a self-hosted FxA is practical is unverified).
- *TTLs.* Desktop tab records originally had a 7-day TTL, then [21 days](https://bugzilla.mozilla.org/show_bug.cgi?id=1319433) (604800 s raised to 1814400 s). [Bug 1719392](https://bugzilla.mozilla.org/show_bug.cgi?id=1719392) raised the clients and tabs TTL to 180 days in Firefox 105. The Rust tabs component also uses a 180-day TTL "same as CLIENTS_TTL" ([searchfox](https://searchfox.org/firefox-main/source/third_party/rust/tabs/src/storage.rs)). A 2020 report says iOS still expired tabs after 21 days ([bug 1646795](https://bugzilla.mozilla.org/show_bug.cgi?id=1646795)). The current iOS value is unverified.

**Keys.**
- *Made.* The onepw protocol runs in two stages. The client stretches the password with PBKDF2 using the email as salt, then derives `authPW` (the only password-derived value sent to the server) and `unwrapBKey` with HKDF. The server stretches `authPW` again with scrypt (64k/8/1) and a random 32-byte salt ([onepw](https://github.com/mozilla/fxa-auth-server/wiki/onepw-protocol)).
- *Two key classes.* kA is a random 32-byte "class A" key the server can recover. kB is a 32-byte "class B" key that the server holds only wrapped, as `wrap(wrap(kB))`. The client XORs `wrap(kB)` with `unwrapBKey` to get kB ([onepw](https://github.com/mozilla/fxa-auth-server/wiki/onepw-protocol)).
- *Sync Key Bundle.* This is HKDF-SHA256 over kB with info `identity.mozilla.com/picl/v1/oldsync`, giving 64 bytes: a 32-byte AES key and a 32-byte HMAC key. It decrypts the `crypto/keys` record, which holds a `default` bulk key pair plus optional per-collection key pairs. Records use AES-256-CBC with a random 16-byte IV plus HMAC-SHA256, and the client must verify the HMAC before it decrypts ([storage format 5](https://mozilla-services.readthedocs.io/en/latest/sync/storageformat5.html)).
- *meta/global.* An *unencrypted* `meta/global` record carries `storageVersion`, a global `syncID`, per-engine `{version, syncID}` and a `declined` list. A changed syncID tells clients to drop cached state ([storage format 5](https://mozilla-services.readthedocs.io/en/latest/sync/storageformat5.html)).
- *Shared to a new device.* The device signs in with the password and derives kB itself, or uses QR pairing (see "Joining a device" below). In pairing, the existing desktop encrypts a "scoped-key bundle" to the new phone's ephemeral public key (`keys_jwk` to `keys_jwe`). The keys never pass through web content ([pairing architecture](https://mozilla.github.io/ecosystem-platform/explanation/pairing-flow-architecture)).
- *Recovered.* A forgotten-password reset replaces `wrap(wrap(kB))` with random bytes, so "the old kB and all class-B data become unrecoverable". A normal password *change* re-wraps the same kB ([onepw](https://github.com/mozilla/fxa-auth-server/wiki/onepw-protocol)). Since 2018 there has been an optional **account recovery key**: generated in account settings, single-use, replaced after each use and never emailed ([Mozilla blog 2018](https://blog.mozilla.org/services/2018/09/26/account-recovery-keys-in-firefox-accounts/), [ghacks](https://www.ghacks.net/2018/09/27/how-to-create-firefox-account-recovery-keys/)). Without a key, Mozilla tells users to reset from a device that still has local data, which re-uploads it ([SUMO](https://support.mozilla.org/en-US/kb/how-change-or-reset-your-mozilla-account-password/revision/288945)).

**Conflicts/merge.**
- *Bookmarks.* These use **Dogear**, a Rust crate that does a *two-way* tree merge. It has no shared ancestor tree, so it cannot tell which fields changed ([Dogear merging](https://mozilla.github.io/dogear/merging.html), [docs.rs](https://docs.rs/dogear)). Items changed on both sides resolve per item, not per property, "picking the chronologically newer side". A newer local title change therefore reverts a remote URL change.
- *Bookmark deletions.* A non-folder edited on one side and deleted on the other is *revived*. A folder in the same situation stays deleted, and its new children move to the nearest surviving parent. Unsynced local items are deduped against incoming remote items by content ([Dogear merging](https://mozilla.github.io/dogear/merging.html)). Firefox ported this merger from JS to Rust in [bug 1482608](https://source-hub.etos-pro.ru/mozilla/gecko-dev/src/commit/c39c82922dd9e6d832cf38f4f5793581e027b902/third_party/rust/dogear).
- *Other engines.* Rust application-services components (logins, places, autofill, tabs) keep a *mirror* table of the last-known server record. They do a 3-way merge when a record changed both locally and remotely ([app-services sync overview](https://mozilla.github.io/application-services/book/design/sync-overview.html)).

**Joining a device.** The user picks "Connect another device" on desktop, which shows a QR code. In the v2 flow the code is shown on the Mozilla accounts page, and accounts with 2FA need a TOTP code first. The phone scans it, opens an encrypted channel, and shows the desktop's user agent, city and IP from the relay envelope. **Both devices show a confirm screen**: the phone confirms first, then the desktop approves. The channel key (32 bytes) travels only in the URL fragment. The relay runs TLS 1.3 PSK between the two devices and cannot read the traffic, and it allows only two connections per channel ([pairing architecture](https://mozilla.github.io/ecosystem-platform/explanation/pairing-flow-architecture), [FxA sitemap](https://mozilla.github.io/ecosystem-platform/fxa-sitemap/flow-pairing)). Users report "Device pairing failed" errors ([connect.mozilla.org](https://connect.mozilla.org/t5/discussions/the-quot-connect-another-device-quot-option-of-firefox-desktop/td-p/16996)).

**Weaknesses/incidents.**
- *Weak client-side stretching.* Client-side stretching was only 1,000 PBKDF2 iterations, so whoever runs the server sees `authPW` and can brute-force weak passwords offline to reach kB ([Palant 2018](https://palant.de/2018/03/13/can-chrome-sync-or-firefox-sync-be-trusted-with-sensitive-data), [bug 1444866](https://bugzilla.mozilla.org/show_bug.cgi?id=1444866)). Cure53 rated this high severity (FXA-01-014) in 2017 ([bug 1320222](https://bugzilla.mozilla.org/show_bug.cgi?id=1320222)). In 2024 Mozilla moved to **650,000** PBKDF2 iterations and random salts in place of email-as-salt, upgraded transparently at login ([micropipes, 2024-10-28](https://micropipes.com/2024/10/28/mozilla-accounts-password-hashing-upgrades/)).
- *Silent data loss on reset.* Forgotten-password resets silently lose data for users without a recovery key ([SUMO thread](https://support.mozilla.org/en-US/kb/how-change-or-reset-your-mozilla-account-password/revision/288626)).
- *Unencrypted metadata.* `meta/global` is plaintext. Collection names, record counts, sizes and timestamps are visible to the server ([overview](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html)).
- *No integrity on the wrapped key.* `wrap(kB)` has no MAC ([onepw](https://github.com/mozilla/fxa-auth-server/wiki/onepw-protocol)).

### Chrome Sync (Google)

**What syncs.** Bookmarks, passwords, addresses, payment methods, history, open tabs, settings and more ([Chrome Help](https://support.google.com/chrome/answer/165139)). Since 2024 Chrome splits *signing in* from *sync*. Signing in gives the account's passwords and addresses, and history and open tabs are a separate opt-in ([Chromium blog, Aug 2024](https://blog.chromium.org/2024/08/seamlessly-use-your-passwords-and.html)).

**Where stored.** Google's sync servers.

**Keys.** The account's "Nigori" node records a passphrase type ([nigori_specifics.proto](https://chromium.googlesource.com/chromium/src/+/main/components/sync/protocol/nigori_specifics.proto)):
- `IMPLICIT` (legacy, Gaia-based)
- `KEYSTORE` (default; the keybag is decrypted by a Google-served keystore key)
- `FROZEN_IMPLICIT`
- `CUSTOM` (user passphrase)
- `TRUSTED_VAULT` (key supplied by a trusted vault)

By default, "Google keeps the key in your account" ([Google Account Help](https://support.google.com/accounts/answer/11350823)). The data is encrypted but readable by Google.

*Custom passphrase.* Key derivation was PBKDF2-HMAC-SHA1 with 1003 iterations and a constant salt up to M69, and is scrypt N=8192, r=8, p=11 with a random salt since M70 ([proto](https://chromium.googlesource.com/chromium/src/+/main/components/sync/protocol/nigori_specifics.proto)). It encrypts all synced types, and Google Wallet payment methods and addresses are excluded. It **disables**:
- feed suggestions based on browsing
- passwords.google.com and Smart Lock
- full history sync (only typed URLs sync)

Changing or resetting the passphrase deletes the passphrase-encrypted data from Google's servers and signs out every device ([Chrome Help](https://support.google.com/chrome/answer/165139)). There is no recovery.

*Trusted vault / on-device encryption for Google Password Manager.*
- *Opt-in.* This is an opt-in layer for passwords and passkeys in which "only you can see your data". Google says it will "over time" be set up for everyone ([Google Account Help](https://support.google.com/accounts/answer/11350823)).
- *Unlocking on a new device.* The user unlocks with the Google password or a device screen lock.
- *No way back.* Once set up it "can't be removed". The only reset deletes all saved passwords ([same](https://support.google.com/accounts/answer/11350823)).
- *Passkeys (Sept 2024).* Desktop passkey sync arrived with a **Google Password Manager PIN**, six digits by default or alphanumeric, as the recovery factor on new devices alongside Android screen lock ([Google blog, 2024-09-19](https://blog.google/technology/safety-security/google-password-manager-passkeys-update-september-2024/)).
- *Recovery.* Recovery needs both the Google account sign-in and an existing device's screen lock. Secure enclaves enforce a guess limit that "is always 10 or less". A removed screen lock remains usable for recovery for up to 64 days ([Google Security Blog, 2022](https://security.googleblog.com/2022/10/SecurityofPasskeysintheGooglePasswordManager.html?m=1)).

**Conflicts/merge.** Chromium sync has one bridge per data type: `DataTypeSyncBridge`, formerly `ModelTypeSyncBridge`. Its default `ResolveConflict` returns *use local* if the remote is a deletion and *use remote* otherwise, so the server wins unless it is deleting ([data_type_sync_bridge.cc](https://chromium.googlesource.com/chromium/src/+/main/components/sync/model/data_type_sync_bridge.cc)). Per-type overrides exist, for example bookmarks and passwords (details unverified).

**Joining a device.** The user signs in to the Google account. With a custom passphrase they must type it on each device. With trusted vault they enter the GPM PIN or an existing device's screen lock ([Google blog](https://blog.google/technology/safety-security/google-password-manager-passkeys-update-september-2024/)).

**Weaknesses/incidents.**
- *Not end-to-end by default.* The default is not end-to-end. The custom passphrase degrades features, so few use it (adoption numbers unverified).
- *Same weak KDF before M70.* The old passphrase KDF had the same weakness Palant described for Firefox ([Palant 2018](https://palant.de/2018/03/13/can-chrome-sync-or-firefox-sync-be-trusted-with-sensitive-data)).
- *Synced-passkey flaws (Aug 2026).* Palo Alto Unit 42 reported flaws in synced passkeys, covered by [Privacy Guides, 2026-08-03](https://www.privacyguides.org/news/2026/08/03/multiple-flaws-in-googles-synced-passkey-implementation-allow-attackers-to-take-over-your-accounts.md):
  - malware on Windows can get signatures from the device identity key without a PIN
  - an attacker can replace a passkey and register an attacker key without proof it came from secure hardware
  - onboarding sends the device the master key protecting synced passkeys

  Fix status is unverified ([eSecurityPlanet](https://esecurityplanet.com/threats/news-google-password-manager-synced-passkey-attacks)).

### Brave Sync v2

**What syncs.** Chromium's data types. Desktop and Android have full sync, and iOS syncs bookmarks, history, passwords and open tabs ([Brave Sync v2 wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).

**Where stored.** Brave's own server, **go-sync**: Go, DynamoDB, wire-compatible with Chromium's sync protobufs (pinned to Chromium 116), with a single `POST /v2/command/` endpoint ([go-sync](https://github.com/brave/go-sync)). Self-hosting is possible by launching Brave with `--sync-url=...`, but only developer setup is documented ([go-sync](https://github.com/brave/go-sync)). No account or sign-in is needed, and no data goes to Google ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).

**Keys.**
- *Made.* The first device generates a random 32-byte seed, shown as a BIP39 phrase or QR code.
- *Authentication.* HKDF-SHA512 derives an Ed25519 key pair for server auth. The access token is a timestamp signed by that key plus the public key, expiring after 1 day.
- *Encryption.* Brave reuses Chromium's custom-passphrase feature with the BIP39 phrase as the passphrase, so the user never picks a weak one. That means scrypt N=2^13, r=8, p=11 with a server-stored salt, then AES-128-CTR-HMAC ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
- *Plaintext metadata.* The server sees device name and type, opaque item and parent IDs, versions and creation time ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
- *Shared to a new device.* By entering the code words or scanning the QR code. Since about 2022 the code shown is **24 seed words plus a 25th "time-limited" word that changes daily**, and a dev says it is "valid for one day" ([community, 2022](https://community.brave.app/t/sync-words-dont-work/418769/10), [community, 2023](https://community.brave.app/t/why-brave-code-25th-changes-every-day/520772)). The rationale given is that people were accidentally sharing phrases and QR codes ([FAQ thread](https://community.brave.com/t/psa-faq-in-progress/464018/36)). How the word is computed (epoch-day) is unverified.
- *Recovered.* There is no recovery service. The phrase is the key, and Brave says Sync is not a backup ([FAQ thread](https://community.brave.com/t/psa-faq-in-progress/464018/36)).

**Conflicts/merge.** Inherited from the Chromium sync engine, including per-type bridges and the server-side version. Brave-specific behaviour is unverified.

**Joining a device.** The user picks Sync, then "I have a sync code", then scans the QR code or types the words. The device list appears in settings, and a device can be removed or the whole chain deleted. A deleted chain cannot be recovered, and local data stays on each device ([Delete Sync Account FAQ](https://support.brave.app/hc/articles/12944495986957), page returned 403 to my fetch; text via search snippet, so partly unverified).

**Weaknesses/incidents.**
- **Jan 2022 incident.** Users set up Sync with the *sample codes* from Brave's help page, so strangers shared chains, including passwords. Brave estimated "fewer than 100 users" had identifiable data synced ([Brave community, 2022-01-19](https://community.brave.app/t/sync-incident-affecting-a-small-number-of-users/328839)). Brave's fixes:
  - replaced the sample QR codes with non-functional ones
  - added warnings
  - added **QR expiry**
  - **capped devices per chain**
- **Device cap.** A staff member in Aug 2023 cited a 50-device server cap. Users in 2024–25 report a 10-device limit ([Privacy Guides forum](https://discuss.privacyguides.net/t/is-there-a-chromium-based-privacy-browser-that-allows-greater-than-10-synced-devices/34311), [Brave community](https://community.brave.com/t/brave-sync-limit-10-devices-not-updating-after-removing-devices/570318)).
- **Code exposure.** A leaked code gives full access. Until self-delete existed (2022), the remedy was emailing Brave to delete the chain ([PSA, 2022-09-02](https://community.brave.app/t/psa-sync-security-all-device-types/428471)). Users note the code can be viewed in settings without re-authentication ([community](https://community.brave.app/t/i-was-just-hacked-why-is-sync-code-available-without-asking-for-password/641472)).
- **Inactivity expiry.** Since 2024-06-01, chains unused for 12 months are deleted server-side ([notice, 2024-03-01](https://community.brave.app/t/notice-unused-sync-accounts-will-now-expire-after-one-year-of-inactivity/534779)).
- **Reliability.** There are frequent "code expired" and broken-chain complaints ([community](https://community.brave.app/t/sync-code-expires-even-though-i-created-it-the-day-before/618768)).
- **Sync v3.** In Aug 2026 a Brave forum responder said Brave is building an **account-based "Sync v3"** (username and password) as an alternative ([community, 2026-08-14](https://community.brave.app/t/can-i-make-the-sync-code-last-forever/656988.md)). That is a retreat from code-only UX (staff status of the responder unverified).
- *Note on the "2023 change".* The brief asks about a change that made chains need a Brave server. I could not verify it. Chains have always needed a Brave-run (or self-run) go-sync server. The verifiable changes are the time-limited codes and device cap (2022) and inactivity expiry (2024).

### Safari / iCloud (Apple)

**What syncs.** Safari bookmarks, history, Tab Groups and iCloud Tabs. Passwords and passkeys sync through iCloud Keychain ([Apple iCloud data security](https://support.apple.com/en-us/102651)).

**Where stored.** Apple's iCloud (CloudKit / key-value storage) ([Secure keychain syncing](https://support.apple.com/guide/security/secure-keychain-syncing-sec0a319b35f/web)).

**Encryption status** ([Apple 102651](https://support.apple.com/en-us/102651)):
- *Safari history, Tab Groups, iCloud Tabs, Passwords/Keychain.* End-to-end encrypted under **Standard** Data Protection.
- *Safari Bookmarks.* Only E2E with **Advanced Data Protection (ADP)**. Otherwise Apple holds the keys.
- *iCloud Mail, Contacts, Calendars.* Never E2E, for interoperability (CalDAV/CardDAV, SMTP).

**Keys.**
- *Made.* Each device creates a P-384 "syncing identity". Devices keep a signed list of peer identities (a "circle"), stored in CloudKit ([Secure keychain syncing](https://support.apple.com/guide/security/secure-keychain-syncing-sec0a319b35f/web)).
- *Shared to a new device.* The new device is "sponsored" by an existing device, which adds its public key to the circle and issues a voucher, or it joins through iCloud Keychain recovery ([same](https://support.apple.com/guide/security/secure-keychain-syncing-sec0a319b35f/web)).
- *Recovered.* Escrow records sit in HSM clusters and are released by SRP over the iCloud security code (in practice the device passcode), plus SMS. The limit is **10 attempts**, after which the record is destroyed and "lost forever". HSM admin cards were destroyed, so firmware cannot be changed ([Escrow security](https://support.apple.com/guide/security/escrow-security-for-icloud-keychain-sec3e341e75d/web)).
- *ADP recovery.* ADP requires setting up a **recovery contact or a 28-character recovery key** before turning it on. Recovery is by device passcode, recovery contact or recovery key ([Apple 102651](https://support.apple.com/en-us/102651)). The 28-character length is from my knowledge of Apple docs, unverified on this page.

**Conflicts/merge.** Keychain: when two members update the same item, "one or the other is chosen, resulting in eventual consistency". No tiebreak rule is documented ([Secure keychain syncing](https://support.apple.com/guide/security/secure-keychain-syncing-sec0a319b35f/web)).

**Joining a device.** The user signs in with their Apple Account. The new device is approved by entering the passcode of an existing device, or an existing device sponsors it. This is the one mainstream design where *the user's existing device passcode is the recovery secret* (UI wording unverified).

**Weaknesses/incidents.**
- *UK withdrawal (Feb 2025).* Apple **withdrew ADP from the UK** in Feb 2025 after a Technical Capability Notice under the Investigatory Powers Act ([MacRumors, 2025-02-21](https://www.macrumors.com/2025/02/21/apple-pulls-encrypted-icloud-security-feature-uk/), [Privacy Guides](https://www.privacyguides.org/articles/2025/02/28/uk-forced-apple-to-remove-adp)).
- *New UK-only notice (Aug 2026).* A new notice targeting only UK users was reported in Aug 2026. Apple is challenging it at the Investigatory Powers Tribunal ([Malwarebytes, 2026-08-04](https://www.malwarebytes.com/blog/news/2026/08/apple-battles-it-out-again-with-uk-over-encrypted-icloud-access)). The outcome is unverified.
- *Bookmarks not E2E by default.* Bookmarks are not E2E unless ADP is on.
- *Apple-only.* There is no non-Apple client.

### Vivaldi Sync

**What syncs.** Bookmarks, Speed Dials, some settings, passwords, autofill, history, extensions, web apps, Reading List, open tabs and Notes (without attachments). "Sync is not meant to be used as a data backup service" ([Vivaldi Help](https://help.vivaldi.com/desktop/privacy/sync/)).

**Where stored.** Vivaldi's own servers in Iceland ([Vivaldi Help](https://help.vivaldi.com/desktop/privacy/sync/)). That it is a Chromium-protocol server is likely but unverified.

**Keys.** The account login password authenticates, and the server stores a hash of it. A **separate encryption password** of at least 12 characters encrypts data on the device and is never sent to Vivaldi ([Vivaldi Help](https://help.vivaldi.com/desktop/privacy/sync/)). The user can save a **backup encryption key file** instead of the password, and it stays valid until remote data is reset ([same](https://help.vivaldi.com/desktop/privacy/sync/)).
- *Recovered.* Only from another device holding the data, or from the key file. Otherwise "the synced data will unfortunately be lost when resetting the encryption password" ([Vivaldi Help](https://help.vivaldi.com/desktop/privacy/sync/), [Forgotten account credentials](https://help.vivaldi.com/article/forgotten-account-credentials/)).

**Conflicts/merge.** Not documented (unverified; presumably Chromium's engine).

**Joining a device.** The user logs in, then enters the encryption password or loads the key file.

**Weaknesses/incidents.** **21 Dec 2024.** After a sync server problem, users were asked to set a *new* encryption key and many lost synced data. In some cases an empty new device overwrote the cloud copy. The thread reached 251 posts and about 122k views ([Vivaldi forum](https://forum.vivaldi.net/post/802575), [thread](https://forum.vivaldi.net/topic/103964/21-12-24-sync-d-data-lost-new-encryption-key/184?page=11)). No official post-mortem was found (unverified root cause).

### Arc (The Browser Company)

The help pages returned 403 to my fetch, so the details below come from search snippets.
- **What syncs:** spaces, folders and tabs. History, passwords, extensions and profiles were "coming soon".
- **Encryption:** "end-to-end encrypted", with recovery through an **Arc Recovery Card** if the user forgets the password and has no prior device.
- **History:** Arc replaced its older iCloud-based sync, deprecated in macOS 1.48.0 on 2024-06-20, with its own E2E service ([Arc Help](https://resources.arc.net/hc/en-us/articles/20272860828823), [Arc Help](https://resources.arc.net/hc/en-us/articles/22370005382167)).
- **Server stack:** unverified.

### Zen

Zen rides on Firefox Sync (Mozilla account) for bookmarks, history, passwords and add-ons ([Zen docs](https://docs.zen-browser.app/llms.txt)). Workspaces did not sync until a Firefox Sync engine for spaces, folders and pinned tabs landed ([PR #12063](https://1clonegithubbyxjn.1934672944.workers.dev/zen-browser/desktop/pull/12063)). It shipped in 1.22.1b ([linuxcompatible](https://www.linuxcompatible.org/story/zen-browser-1221b-ships-with-crossdevice-sync-and-shareable-workspace-links/), [piunikaweb, 2026-07-09](https://piunikaweb.com/2026/07/09/zen-browser-device-sync-twilight/)). Version details are secondary-source, partly unverified. Lesson: a fork can add its own collection to an existing E2E sync service.

### Orion (Kagi)

Orion syncs bookmarks, tabs and passwords **through the user's own iCloud**, "same as Safari", with no Kagi account ([Kagi blog](https://blog.kagi.com/orion-features), [AppleInsider, 2025-11-26](https://appleinsider.com/articles/25/11/26/hands-on-kagis-orion-browser-is-a-surprisingly-good-alternative-to-safari)). Setup requires iCloud Drive to be enabled for Orion ([Macwelt](https://www.macwelt.de/article/3008144/orion-browser-test.html)). Whether that is CloudKit or files in iCloud Drive is unverified. Kagi declined to run a sync server, closing the request as "Nofix" because "Kagi doesn't want to handle our data". That leaves no sync on non-Apple platforms ([Orion feedback](https://orionfeedback.org/d/2940-browser-syncing-without-icloud-firefox-sync-or-orion-sync-via-kagi-servers-for-paid-users?page=2)). This is the nearest precedent for a user-owned-storage browser sync.

### Microsoft Edge (brief)

Edge syncs favorites, passwords, form-fill, settings, extensions, open tabs, history (v88+) and feature-usage data (v133+). Device name, make and model are uploaded too ([Microsoft Learn](https://learn.microsoft.com/deployedge/microsoft-edge-enterprise-sync)). For Entra accounts, data is protected with Azure Information Protection / Purview, using Microsoft-managed keys or **BYOK**. There is no user passphrase ([Microsoft Learn ESR](https://learn.microsoft.com/pl-pl/deployedge/microsoft-edge-enterprise-state-roaming), [password manager security](https://learn.microsoft.com/en-us/deployedge/microsoft-edge-security-password-manager-security)). Edge 142 (Nov 2025) added passkey sync for Microsoft accounts, gated by a **PIN with a maximum of 10 attempts**. The PIN can be reset from an existing device ([heise, 2025-11-04](https://heise.de/-11039627)). Users fix sync issues with "Reset sync data from Microsoft servers" ([Microsoft Learn](https://learn.microsoft.com/deployedge/microsoft-edge-enterprise-sync)).

### Browser takeaways

- **Two-tier keys are the norm.** A root secret wraps per-collection or bulk keys (Firefox kB, then the Sync Key Bundle, then `crypto/keys`; Chromium's Nigori keybag). Rotating bulk keys need not change the user's secret ([storage format 5](https://mozilla-services.readthedocs.io/en/latest/sync/storageformat5.html), [nigori proto](https://chromium.googlesource.com/chromium/src/+/main/components/sync/protocol/nigori_specifics.proto)).
- **Password-derived roots are the weak point.** Firefox ran 1,000 PBKDF2 rounds until 2024 ([Palant](https://palant.de/2018/03/13/can-chrome-sync-or-firefox-sync-be-trusted-with-sensitive-data), [micropipes](https://micropipes.com/2024/10/28/mozilla-accounts-password-hashing-upgrades/)). Brave avoids this with a random 256-bit seed but makes the seed the only credential ([wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
- **Pairing QR plus confirm-on-both is the best join UX here.** In Firefox v2 the old device encrypts keys to the new device's ephemeral public key over a relay that cannot read them. The 32-byte channel key travels in the QR fragment ([pairing architecture](https://mozilla.github.io/ecosystem-platform/explanation/pairing-flow-architecture)). LAN or any blind relay could carry the same exchange.
- **Long-lived shareable codes go wrong.** Brave's sample-code incident, leaked codes and inactivity wipes ([2022 incident](https://community.brave.app/t/sync-incident-affecting-a-small-number-of-users/328839), [2024 notice](https://community.brave.app/t/notice-unused-sync-accounts-will-now-expire-after-one-year-of-inactivity/534779)) argue for short-lived pairing codes, separate from any long-term recovery secret.
- **Recovery is where data is lost.** Forgotten-password resets silently drop kB (Firefox), wipe the server (Chrome passphrase, Vivaldi), or are capped at 10 HSM guesses (Apple, Google, Edge). Vivaldi's Dec 2024 outage shows an empty device "resetting" can overwrite the good copy ([Vivaldi forum](https://forum.vivaldi.net/post/802575)). Safeguards should stop an empty or new device from winning a reset.
- **Merges in practice.** Two-way, newer-wins at item level for trees (Dogear), with known revert-on-conflict loss ([Dogear](https://mozilla.github.io/dogear/merging.html)). Three-way merge with a stored mirror for records (Firefox Rust engines) ([app-services](https://mozilla.github.io/application-services/book/design/sync-overview.html)). Chromium's default is remote wins except on deletes ([bridge](https://chromium.googlesource.com/chromium/src/+/main/components/sync/model/data_type_sync_bridge.cc)). Keeping a mirror/base is what makes three-way possible.
- **Tabs are ephemeral, per-device records with a TTL.** Firefox uses one record per client with a 180-day TTL ([bug 1719392](https://bugzilla.mozilla.org/show_bug.cgi?id=1719392)). With user-controlled storage that becomes one file per device, garbage-collected by age.
- **Metadata leaks even with E2E.** Firefox's `meta/global`, collection names and counts, and Brave's plaintext parent IDs and versions are visible ([overview](https://mozilla-services.readthedocs.io/en/latest/sync/overview.html), [wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
- **Only Orion uses user-controlled storage.** It relies on iCloud and is Apple-only as a result ([Kagi](https://blog.kagi.com/orion-features)).

---

## User-controlled storage apps (part 1)

Scope: Floccus, xBrowserSync, Joplin, Obsidian (Sync and community options), the KeePass family, Bitwarden/Vaultwarden, Standard Notes. Researched 2026-11-17. Where a claim comes from source code, the link points at the file on the default branch as read on that date. "(unverified)" marks anything I could not confirm from a primary source.

### Floccus

**What syncs.** The browser's native bookmarks tree (one chosen folder or the whole tree), and optionally open tabs (marked experimental) ([floccus guides](https://floccus.org/guides)). Floccus is "a sync tool, not a bookmark manager": it syncs the browser's own bookmarks through a server that both sides can reach ([guides](https://floccus.org/guides)). It runs as a WebExtension in Firefox, Chrome, Edge, Brave, Vivaldi and Opera, plus Android/iOS apps; Safari is "not yet" supported ([README](https://github.com/floccusaddon/floccus)). The latest release is v5.13.0, published 2026-11-08 ([GitHub releases API, floccusaddon/floccus](https://github.com/floccusaddon/floccus/releases)).

**Where stored / file layout.** The backends are Nextcloud Bookmarks, Linkwarden, Karakeep, Google Drive, Dropbox, any Git server, S3-compatible buckets, and any WebDAV server ([README](https://github.com/floccusaddon/floccus), [guides](https://floccus.org/guides)). File-based backends (WebDAV, Google Drive, Dropbox, S3, Git) store one file, by default `bookmarks.xbel` ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts), [guides](https://floccus.org/guides)). The file is either XBEL (the XML bookmark exchange format) or Netscape bookmark HTML ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)). Each item carries a numeric id, and the file records a `<!--- highestId :N` comment so the next id can be allocated ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)). Items that have no id, for example in a hand-edited file, get an id derived from their content ([Xbel.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/serializers/Xbel.ts)). Git uses isomorphic-git on a single branch (default `main`) and commits the file ([Git.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/Git.ts)). Nextcloud Bookmarks, Linkwarden and Karakeep are server-side bookmark APIs, not files. Karakeep "cannot be used with end-to-end encryption" ([guides](https://floccus.org/guides)).

**Keys.** The passphrase is optional and per account, offered for Google Drive, Dropbox, WebDAV and S3. An empty field means the file is stored in plaintext ([guides](https://floccus.org/guides)). The source shows the following (all from [Crypto.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/Crypto.ts)):

- PBKDF2-SHA-256 with `iterations = 250000` derives an AES-GCM 256-bit key directly from the passphrase.
- Each encryption uses a random 16-byte IV, prefixed to the ciphertext and base64-encoded.

On every upload a fresh random 64-byte salt is generated and the file is written as JSON `{ciphertext, salt}` ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)). A legacy path, still accepted on read, used the bookmark file name as the salt ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)). There is no key hierarchy, no recovery and no key sharing: every device is configured with the same passphrase by hand. Profiles can be exported and imported between browsers, but Google Drive and Dropbox logins have to be entered again ([guides](https://floccus.org/guides)).

**Conflicts / merge algorithm.** Floccus does a three-way tree sync. It loads the local tree, the server tree, and a cached tree of the last synced state ([Default.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts)). Two Scanner runs diff local-vs-cache and server-vs-cache into CREATE, UPDATE, MOVE, REMOVE and REORDER actions. Persisted mappings translate local bookmark ids to server ids ([Default.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts), [Mappings.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/Mappings.ts)). Concurrent actions are then reconciled pairwise:

- concurrent removals against moves;
- concurrent creations of the same item, which are de-duplicated;
- hierarchy reversals, where A is moved into B while B is moved into A;
- concurrent reorderings.

When both sides change the same thing, `masterLocation` (set to LOCAL in the default strategy) "has precedence in case of conflicts" ([Default.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts)).

The user can choose between three strategies:

- Merge, the default ("Always merge local changes with changes of other browsers").
- One-way, where one device uploads and the others download and overwrite.
- Temporary "Push up once" or "Pull down once".

There is also a notify-only mode that is not available on mobile ([guides](https://floccus.org/guides)).

Concurrency is handled with an advisory lock file `bookmarks.xbel.lock`. It is refreshed every 2 minutes during a sync, and another client may take it over 15 minutes after its last refresh ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)). Google Drive keeps the same lock in file `appProperties` with the same 2 and 15 minute timers ([GoogleDrive.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/GoogleDrive.ts)). On S3, the lock is created with `If-None-Match: *`, and the bookmarks object is overwritten only if its ETag still matches, where the server supports this ([S3.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/S3.ts)). The WebDAV adapter also refuses a download whose byte size differs from the PROPFIND size, which guards against reading a partly uploaded file ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)). It re-uploads only when the tree hash changed ([WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts)).

A failsafe refuses any run that would delete or add more than 20% of the bookmarks, or more than 1,000 items, once the tree has at least 5 items. For tabs the threshold is 50% and at least 50 items ([Default.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts)). The user can disable it ([AMO review quoting E029](https://addons.mozilla.org/de/firefox/addon/floccus/reviews/2021278/)).

**Joining a device.** The user installs the extension or app, adds an account, chooses the backend, re-enters the URL, credentials and passphrase, and picks the local folder. The first sync merges ([guides](https://floccus.org/guides)). Each setup page tells the user to turn off the browser's native bookmark sync, because running both causes duplicates ([guides](https://floccus.org/guides), [FAQ](https://floccus.org/faq)).

**Weaknesses / incidents.**

- The design notes say the WebExtensions bookmarks API has no transactions and no compare-and-set. Firefox Sync can change GUIDs, and descriptions, tags and separators are not in the data model. The stated goal is therefore "eventual consistency" over preserving intent ([CONSIDERATIONS.md](https://github.com/floccusaddon/floccus/blob/develop/CONSIDERATIONS.md)).
- Users report losing all bookmarks while the folders survived, and deletions spreading even in one-way mode ([AMO 1-star reviews](https://firefox.marketplace.browsertotal.com/en-US/firefox/addon/floccus/reviews/?score=1)).
- Nextcloud users report extra and missing items ([Nextcloud forum](https://help.nextcloud.com/t/bookmarks-app-with-floccus-creates-extra-items-and-missing-others/124194)).
- WebDAV lock-file trouble is reported often, for example E019 404 on PUT after the lock was set ([issue digest](https://lightrun.com/answers/floccusaddon-floccus-webdav-sync-problems)).
- The passphrase is optional, so plaintext is the default.
- PBKDF2 runs directly on the passphrase with no separate data key, so changing the passphrase means rewriting the file. This follows from [Crypto.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/Crypto.ts) and [WebDav.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/WebDav.ts).

### xBrowserSync

**What syncs.** Bookmarks only, as a single compressed and encrypted blob per "sync" ([API README](https://github.com/xbrowsersync/api)).

**Where stored.** On an xBrowserSync API service: the official one or a self-hosted Node.js and MongoDB service ([API README](https://github.com/xbrowsersync/api)). The default limits and retention are:

- `maxSyncSize` is 512,000 bytes, and `dailyNewSyncsLimit` is 3 per IP. Client IPs are logged in `newsynclogs` to enforce this ([API README](https://github.com/xbrowsersync/api)).
- `maxSyncs` is 5,242 per server, and requests are throttled at 1,000 per 5 minutes ([API README](https://github.com/xbrowsersync/api)).
- Public services are advised to add a TTL index that deletes syncs not accessed for 3 weeks ([API README](https://github.com/xbrowsersync/api)).

A community re-implementation stores syncs in SQLite, PostgreSQL or MySQL ([GitHub topic xbs](https://github.com/topics/xbs)).

**Keys.** The user's password plus the 32-character sync ID go through PBKDF2-SHA-256 with 250,000 iterations to make an AES-GCM key ([crypto.service.ts](https://github.com/xbrowsersync/app/blob/master/src/modules/shared/crypto/crypto.service.ts), [v1.4.0 release notes](https://newreleases.io/project/github/xbrowsersync/app/release/v1.4.0)). The sync ID is the salt ([v1.4.0 release notes](https://newreleases.io/project/github/xbrowsersync/app/release/v1.4.0)). Data is compressed with lzutf8 and encrypted under a random 16-byte IV ([crypto.service.ts](https://github.com/xbrowsersync/app/blob/master/src/modules/shared/crypto/crypto.service.ts)). Sharing to a new device means copying the sync ID, the only identifier, and typing the password. There is no recovery: losing either loses the data.

**Conflicts.** There is no merge on the server. An update carries the client's last-known `lastUpdated`, and the server throws `SyncConflictException` if it differs ([bookmarks.service.ts](https://github.com/xbrowsersync/api/blob/master/src/services/bookmarks.service.ts)). The client must then pull and redo its change. This is optimistic concurrency on a whole-blob basis.

**Joining a device.** The user enters the service URL, the sync ID and the password (inferred from the scheme above). A QR code for the sync ID existed in the mobile app (unverified).

**Weaknesses / status.** The last app release is v1.5.2, from 2020-05-05, and the last API release is v1.1.13, from 2021-04-05 ([GitHub releases API](https://github.com/xbrowsersync/app/releases)). The repositories still received test and maintenance commits in March 2026 but no releases ([xbrowsersync/app commits](https://github.com/xbrowsersync/app/commits)). Store reviews call it "severely unmaintained" ([AMO reviews](https://firefox-stats.com/d/xbs/reviews)). The official Docker image was last updated about 5 years ago ([Docker Hub](https://hub.docker.com/r/xbrowsersync/api)). Because the whole blob is replaced, every edit re-uploads everything and a conflict forces a full refresh.

### Joplin

**What syncs.** Notes, notebooks, tags, note-tag links, resources (attachments), master keys and revisions ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee), [forum explanation of `type_`](https://discourse.joplinapp.org/t/question-about-syncing-directory-structure/6939)).

**Where stored / layout.** The sync targets are Dropbox, OneDrive, Nextcloud/WebDAV, S3, the file system, Joplin Server and Joplin Cloud ([Joplin sync docs](https://joplinapp.org/help/apps/sync/)) (target list as commonly documented; the page itself is not re-fetched here). The target holds the following:

- One file per item, `<32-hex-id>.md`, at the root ([forum](https://discourse.joplinapp.org/t/syncing-windows-installation-to-ipad-via-onedrive-loses-folders/41475)). Each file has a body plus `key: value` metadata, and its `type_` field says what it is: 1 note, 2 folder, 4 resource ([forum](https://discourse.joplinapp.org/t/question-about-syncing-directory-structure/6939)).
- Binary blobs in `.resource/`, plus `locks/` (older `.lock/`), `temp/` and `info.json` ([forum](https://discourse.joplinapp.org/t/syncing-windows-installation-to-ipad-via-onedrive-loses-folders/41475), [2020 sync-target upgrade news](https://joplinapp.org/news/20200906-172325)).
- `info.json` holds target-wide settings, including the E2EE state and master keys, so that all clients agree. Each property carries an `updatedTime` that resolves concurrent changes ([sync spec](https://joplinapp.org/help/dev/spec/sync)).
- The sync target is versioned (`{"version": 2}`, with `.sync/version.txt` kept for old clients) ([2020 news](https://joplinapp.org/news/20200906-172325)).

Upgrading the target needs an EXCLUSIVE lock. The client goes through SHOULD_UPGRADE, then MUST_UPGRADE, then IDLE, with restarts in between ([sync lock spec](https://joplinapp.org/help/dev/spec/sync_lock)). Locks are files named `<sync|exclusive>_<desktop|mobile|cli>_<clientId>.json`. Many SYNC locks can coexist, but only one EXCLUSIVE. If several exclusive locks exist, the oldest wins and ties go to the lowest client id. A client must refresh its lock every X seconds and stop if it cannot within Y seconds; the spec does not give X and Y ([sync lock spec](https://joplinapp.org/help/dev/spec/sync_lock)).

**Keys.**

- **Master keys.** A random master key is generated when E2EE is enabled, encrypted with the user's master password, and synced in `info.json`. Several master keys can exist, for when two offline clients both enabled E2EE, but only one is active. Each item's header names its master key ID ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee)).
- **Item format.** Encrypted items start with a `JED` header, a 2-hex version, the method ID and a 32-hex master key ID, followed by length-prefixed chunks ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee)).
- **Native encryption since v3.2.** v3.2 introduced `KeyV1`, `StringV1` and `FileV1` ([native encryption spec](https://joplinapp.org/help/dev/spec/e2ee/native_encryption)). They use AES-256-GCM with a 96-bit random IV and a 128-bit tag, and PBKDF2-HMAC-SHA512 with 220,000 iterations for KeyV1 ([native encryption spec](https://joplinapp.org/help/dev/spec/e2ee/native_encryption)). The project raised the master-key iterations from 10,000 to 220,000 ([GSoC final report](https://discourse.joplinapp.org/t/final-report-of-the-native-encryption-project/40171)).
- **What changed in v3.2.** The new methods replaced the SJCL methods: AES-CCM, AES-128 for data, a 64-bit tag and 10,000 PBKDF2 iterations ([parameters thread, 2024-06-24](https://discourse.joplinapp.org/t/parameters-for-the-new-encryption/38742)). In `@joplin/lib` 3.5.1 the defaults are `StringV1` (enum 10) with 64 KiB chunks, `FileV1` (9) with 128 KiB chunks, and `KeyV1` (8) for master keys ([EncryptionService.ts 3.5.1](https://cdn.jsdelivr.net/npm/@joplin/lib@3.5.1/services/e2ee/EncryptionService.ts)).
- **Plaintext metadata.** The fields `id, note_id, tag_id, parent_id, share_id, updated_time, deleted_time, type_` are deliberately left unencrypted, so the target can see the notebook tree shape and timestamps ([BaseItem.ts 3.5.1](https://cdn.jsdelivr.net/npm/@joplin/lib@3.5.1/models/BaseItem.ts)).
- **Sharing.** Each user has one RSA-style key pair. When a notebook is shared, the notebook's own key is encrypted to the recipient's public key, and the recipient re-encrypts it under their own master password. The key pair is created during sync if `info.json` lacks one, which prevents clients from generating competing pairs ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee)).
- **New device.** On its first sync the device downloads the master key, E2EE turns on automatically, and a persistent banner asks for the master password ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee)).
- **Recovery.** None. A forgotten password "cannot be recovered" ([user E2EE guide](https://joplinapp.org/help/apps/sync/e2ee)). A reset disables the old keys and regenerates the key pair, and data under the old keys becomes undecryptable ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee)). Changing the password re-wraps the keys but does not re-encrypt the notes ([E2EE spec](https://joplinapp.org/help/dev/spec/e2ee)).

**Conflicts.** Merging works item by item. If a note or attachment is changed in two places, the remote version takes the note's place and the local version is copied into a special "Conflicts" notebook for manual diffing. The pre-sync version can also be restored from note history ([conflict help](https://joplinapp.org/help/apps/conflict)). Local state per item is `sync_time` in the `sync_items` table ([sync spec](https://joplinapp.org/help/dev/spec/sync)). Deletions propagate: a note deleted on one device is "eventually deleted from each device too" ([sync spec](https://joplinapp.org/help/dev/spec/sync)).

**Joining a device.** The user configures the same target, syncs, and is asked for the master password. The docs warn: "Do not manually enable encryption on multiple devices in parallel." Enabling E2EE forces a full re-upload that can run "over night" ([user E2EE guide](https://joplinapp.org/help/apps/sync/e2ee)).

**Weaknesses / incidents.**

- The old SJCL scheme was criticised as weak: AES-CCM with a 64-bit tag, 10,000 iterations and AES-128 ([forum thread "Insecure encryption method in Joplin"](https://discourse.joplinapp.org/t/insecure-encryption-method-in-joplin-source-code/34644), [parameters thread](https://discourse.joplinapp.org/t/parameters-for-the-new-encryption/38742)).
- The item-per-file layout means thousands of small files, and generic cloud APIs like OneDrive have lost folders or been slow ([forum](https://discourse.joplinapp.org/t/syncing-windows-installation-to-ipad-via-onedrive-loses-folders/41475)).
- Users report notes lost after a sync ([forum](https://discourse.joplinapp.org/t/lost-my-joplin-notes-after-sync/29024)).
- The plaintext metadata above leaks the notebook structure.

### Obsidian

**What syncs.** A vault: a folder of Markdown files, attachments and `.obsidian/` settings. Selective Sync and "Vault configuration sync" toggles are set per device ([Sync setup](https://obsidian.md/help/sync/setup)).

**Where stored.**

- **Obsidian Sync** is the paid first-party service. Standard has 1 GB, 1 month of version history, 1 vault and 5 MB per file. Plus has 10 to 100 GB, 12 months of history, 10 vaults and 200 MB per file ([plans page](https://obsidian.md/help/sync/plans)).
- **Alternatives:** iCloud Drive, with the vault at `iCloud Drive/Obsidian/<vault>` on iOS ([sync notes help](https://obsidian.md/help/sync-notes)), or community plugins (below). Obsidian advises against syncing one vault through several services at once ([sync notes help](https://obsidian.md/help/sync-notes)).

**Keys (Obsidian Sync).**

- **Encryption.** Remote vaults default to "End-to-end encryption" with a user-chosen vault encryption password. Standard mode instead stores the key on Obsidian's servers. Encryption is AES-256-GCM, and the key comes from the password with scrypt and a salt ([Sync security](https://obsidian.md/help/sync/security)). The scrypt parameters are not published (unverified).
- **Visible metadata.** Even with E2EE the server sees which device uploaded or deleted a file, upload timestamps, and the mapping from encrypted path to encrypted content ([Sync security](https://obsidian.md/help/sync/security)).
- **Deduplication.** Hashing is deterministic, so identical content gives identical ciphertext hashes, allowing confirmation-of-file attacks by a server that can force uploads ([Sync security](https://obsidian.md/help/sync/security)).
- **Recovery.** None: a lost password means a new remote vault and reconnecting every device ([Sync security](https://obsidian.md/help/sync/security)).
- **Audits.** Cure53 audited Sync in October 2024, and Trail of Bits audited the Sync API, server and cryptography in December 2025 ([obsidian.md/security](https://obsidian.md/security)).

**Conflicts (Obsidian Sync).** Each file type is handled differently ([troubleshoot](https://obsidian.md/help/sync/troubleshoot)):

- Markdown is merged with Google's diff-match-patch.
- Other files, including canvases, are last-modified-wins.
- Settings JSON is merged by "appl[ying] keys from the local JSON on top of the remote JSON".

Since 1.9.7, users can choose "Create conflict file" instead of auto-merge, and the setting is per device. The conflict file is named `name (Conflicted copy <device> YYYYMMDDHHMM).md`, and the original keeps the remote version. The docs admit that auto-merge "may sometimes create duplicate text" ([troubleshoot](https://obsidian.md/help/sync/troubleshoot)).

**Joining a device (Sync).** The user logs in to the Obsidian account (with 2FA if set), selects "Connect" on a remote vault, enters the encryption password, names the local vault, and adjusts selective sync before pressing "Start syncing" ([Sync setup](https://obsidian.md/help/sync/setup)).

**Community, user-storage options.**

- **Self-hosted LiveSync** syncs through CouchDB (self-hosted, Fly.io or Cloudant), or through object storage (S3, R2 or MinIO) that is periodic rather than live. It can also sync WebRTC peer-to-peer, which still needs a signalling relay and both peers online ([README](https://github.com/vrtmrz/obsidian-livesync)).
  - It supports E2EE with a shared passphrase and optional "Path Obfuscation". Changing either requires rebuilding the local and remote databases ([settings doc mirror](https://avyl.ru/self-hosted/obsidian-livesync/src/commit/3d4ad4a3b446fa8f825b3989f0fca8c385cd3db1/docs/settings.md), [0.18.0 release](https://newreleases.io/project/github/vrtmrz/obsidian-livesync/release/0.18.0)).
  - It auto-merges simple conflicts ([README](https://github.com/vrtmrz/obsidian-livesync)).
  - A new device joins by pasting an encrypted "Setup URI" copied from a working device ([README](https://github.com/vrtmrz/obsidian-livesync)). This is the closest prior art to "copy one encrypted blob to join".
- **Remotely Save** backs onto S3-compatible storage, Dropbox, OneDrive (app folder only) and WebDAV for free, and Google Drive, Box, pCloud and others with PRO ([README](https://github.com/remotely-save/remotely-save)).
  - It offers optional E2EE in rclone crypt (recommended) or OpenSSL enc format, as of March 2024 ([encryption docs](https://github.com/remotely-save/remotely-save/blob/master/docs/encryption/README.md)). The vault name is not encrypted on Dropbox, OneDrive or WebDAV ([README](https://github.com/remotely-save/remotely-save)).
  - In the free tier, conflict handling is "keep newer or larger". The PRO "Smart Conflict" option merges small Markdown files and duplicates large or non-Markdown ones ([README](https://github.com/remotely-save/remotely-save)).
- **obsidian-git** does scheduled commit, pull and push with in-editor conflict resolution ([README](https://github.com/Vinzent03/obsidian-git)). On mobile it uses isomorphic-git, which the author calls "very unstable", with no SSH and limited repo size ([README](https://github.com/Vinzent03/obsidian-git)). The README does not mention encryption.

**Weaknesses.**

- iCloud Drive vaults are a common source of duplicates and missing files. A plugin guide advises "Keep Downloaded" to stop iCloud offloading notes ([jemzsync plugin page](https://community.obsidian.md/plugins/jemzsync)).
- Obsidian's zh-TW help warns that iCloud on Windows can duplicate or corrupt files ([zh-TW help](https://obsidian.md/zh-TW/help/sync-notes)).
- Forum users report duplicated content after syncs ([forum](https://forum.obsidian.md/t/note-contents-disappearing/119522)).

### KeePass family (KeePass 2, KeePassXC, KeePassDX, KeePass2Android, Strongbox, KeePassium) with a .kdbx in Dropbox/iCloud/etc.

**What syncs.** One encrypted file. Users sync it with any file-sync tool or open it over WebDAV, FTP or SFTP. For Dropbox, OneDrive and Google Drive, KeePass recommends the local file-system client over protocols ([KeePass sync help](https://keepass.info/help/v2/sync.html)).

**Format / keys.**

- **KDBX 4.1.** Version 0x00040001. Ciphers are AES-256-CBC or ChaCha20, and the KDF is AES-KDF, Argon2d or Argon2id. Argon2 should be version 0x13 with a 32-byte salt; the minimums are 1 iteration, 8 KiB memory and parallelism 1 ([KDBX 4.1 spec](https://keepass.info/help/kb/kdbx.html)).
- **Integrity and inner stream.** The header carries a SHA-256 hash and an HMAC-SHA-256 keyed from the master key. Protected fields use an inner ChaCha20 stream, which is the default ([KDBX 4.1 spec](https://keepass.info/help/kb/kdbx.html)).
- **Composite key.** The master key is a composite of password, key file and/or other providers ([KDBX 4.1 spec](https://keepass.info/help/kb/kdbx.html)).
- **No key distribution.** Each device needs the password, plus the key file if one is used, entered by hand. There is no recovery other than a copy of the key file kept elsewhere.

**Merge semantics.** These are the main prior art for "merge two encrypted replicas offline".

- **KeePass 2 "Synchronize".** Both files end up identical ("After synchronizing two files A and B, both A and B are up-to-date"). The merge works at entry level, not field level, and "mainly uses the last modification time". The losing version is kept as a history entry, not discarded ([sync help](https://keepass.info/help/v2/sync.html)). On Save, KeePass checks whether the file changed on disk or the server and offers overwrite or synchronize. This covers only "Save", not "Save As" ([sync help](https://keepass.info/help/v2/sync.html)).
- **KeePassXC merge modes.** `KeepNewer` merges history. `Synchronize`, the fallback default, merges history, keeps the most recent version as the top entry, and applies deletions ([Group.h](https://github.com/keepassxreboot/keepassxc/blob/develop/src/core/Group.h)). Entries are matched by UUID and ordered by `lastModificationTime`. History items are merged keyed by modification time, and the code warns when two history items with the same timestamp differ ("conflict resolution may lose data") ([Merger.cpp](https://github.com/keepassxreboot/keepassxc/blob/develop/src/core/Merger.cpp)). Deletions are carried in the database's `DeletedObjects` list (tombstones by UUID and time) ([Merger.cpp](https://github.com/keepassxreboot/keepassxc/blob/develop/src/core/Merger.cpp)).
- **KeePassXC save guard.** PR #3612 made KeePassXC refuse to save over a file whose on-disk changes have not been merged ([PR #3612](https://github.com/keepassxreboot/keepassxc/pull/3612)). Issue #5290 reports data loss when auto-reload was off ([issue #5290](https://github.hscsec.cn/keepassxreboot/keepassxc/issues/5290)).
- **Android clients** (tested on Android 11, 2021) ([ctrl.blog](https://www.ctrl.blog/entry/keepass-file-conflicts-android/)):
  - KeePassDroid, KPass and AuthPass silently overwrote remote changes.
  - KeePassDX detected the change but did not merge; saving after dismissing the warning overwrote remote changes.
  - KeePass2Android offered merge or overwrite, a pull-before-push, but the merge often crashed and the two buttons look alike.
- **KeePassium** "does not support database merging yet" and sends users to KeePassXC or KeePass 2 ([KeePassium KB](https://support.keepassium.com/kb/database-merge/)).
- **Strongbox** merges changes for Wi-Fi Sync between the user's own devices ([Strongbox KB](https://strongbox.reamaze.com/kb/sync/what-is-wi-fi-sync-and-how-do-i-use-it)). Its optional "Lazy Sync" opens a local cache first and "can lead to more sync/merge issues" ([Strongbox KB](https://strongbox.reamaze.com/kb/sync/what-is-lazy-sync)). Strongbox says it cannot guarantee integrity over third-party cloud sync ([Strongbox sync KB](https://strongbox.reamaze.com/kb/sync)).

**Joining a device.** The user points the app at the synced file and types the password, choosing the key file if one is used. Nothing is exchanged between devices.

**Weaknesses / incidents.** Dropbox makes "conflicted copy" files when a phone edits while an upload is pending. The user must then merge by hand with Database > Merge From Database ([panicvault guide](https://www.panicvault.org/cloud-sync/sync-conflicts/), [KeePassium user report](https://reddit.birdcat.cafe/r/KeePassium/comments/1kmpfbb/dropbox_sync_has_gone_haywire), [Dropbox forum](https://www.dropboxforum.com/discussions/101001014/mystery-conflicted-copies-of-a-keypass-file-shared-between-phone-and-macbook/855106/replies/855118)). Merging only works where both apps implement the UUID, mtime, history and tombstone rules; one naive client in the fleet loses data ([ctrl.blog](https://www.ctrl.blog/entry/keepass-file-conflicts-android/)). Last-modified-wins depends on device clocks (inferred from the [KeePass sync help](https://keepass.info/help/v2/sync.html)). Dropbox's own engineers wrote about clock skew breaking a password payload merge ([Dropbox tech blog](https://dropbox.tech/application/dropbox-passwords-clock-skew-payload-sync-merge)).

### Bitwarden and Vaultwarden (self-host)

**What syncs.** The vault: logins, cards, identities, notes, attachments, Sends, folders, collections and org items ([whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/)).

**Where stored.** On Bitwarden cloud or the self-hosted official server. Vaultwarden is "an alternative server implementation of the Bitwarden Client API, written in Rust", is "not associated with Bitwarden", and works with the official clients ([Vaultwarden README](https://github.com/dani-garcia/vaultwarden)). It defaults to SQLite and also supports MariaDB/MySQL and PostgreSQL ([Vaultwarden wiki](https://github.com/dani-garcia/vaultwarden/wiki)).

**Keys.**

- **Master key.** PBKDF2-SHA-256 runs on the master password with the email as salt, 600,000 iterations by default, giving a 256-bit master key. HKDF stretches this to a 512-bit "stretched master key". Neither key is stored or sent ([whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/), [KDF help](https://bitwarden.com/help/kdf-algorithms/)).
- **Argon2id option.** Current help lists the defaults as memory 32, iterations 6, parallelism 4 ([KDF help](https://bitwarden.com/help/kdf-algorithms/)) (the page gives memory without a unit beside it; MiB presumably). The 2026.2.1 release raised the minimum PBKDF2 iterations to 600,000 ([KDF help](https://bitwarden.com/help/kdf-algorithms/)).
- **User key.** A random 512-bit user (symmetric) key, wrapped by the stretched master key, is the "Protected Symmetric Key" stored on the server ([whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/)).
- **Authentication hash.** The master password hash is one more PBKDF2 over the master key. The server re-hashes it with 600,000 iterations ([whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/)).
- **Sharing keys.** An RSA key pair is created per account, and its private key is wrapped by the user key. Org keys are shared by RSA-OAEP to each member ([whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/)).
- **Changing the KDF** re-wraps the user key but does not re-encrypt the vault ([KDF help](https://bitwarden.com/help/kdf-algorithms/)). This is the separation a passphrase-only design like Floccus's lacks.

**New devices** can get the user key in three ways.

- **Master password.** The device derives the key locally ([whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/)).
- **"Log in with device".** This is the UX closest to QR pairing ([log in with device](https://bitwarden.com/help/log-in-with-device/)):
  - The new device sends an ephemeral auth-request public key and an access code.
  - An already signed-in device shows a per-request fingerprint phrase. When approved, it encrypts the user key to that public key.
  - Requests expire after 15 minutes, the browser extension waits up to 2 minutes, and the key pairs are purged afterwards.
- **Trusted devices (SSO).** The device keeps a local Device Key, an AES-256-CBC-HMAC 512-bit key that "never leaves the client" ([trusted devices](https://bitwarden.com/help/about-trusted-devices/)). The server keeps three blobs: the user key encrypted to the device's RSA-2048 public key, the device private key wrapped by the Device Key, and the public key wrapped by the user key for rotation ([trusted devices](https://bitwarden.com/help/about-trusted-devices/)). An untrusted device is approved by another device, by an admin (requests expire after 1 week), or by the master password ([trusted devices](https://bitwarden.com/help/about-trusted-devices/)).

**Recovery.**

- **Emergency access** (premium). The user key is encrypted to a trusted contact's RSA public key. The contact can view or take over after a wait time the grantor sets, and invitations are valid for 5 days ([emergency access](https://bitwarden.com/help/emergency-access/)).
- **Account recovery** (Enterprise). The member's user key is escrowed under the org public key, so an admin can set a new master password without seeing the old one ([account recovery](https://bitwarden.com/help/account-recovery/)).
- Otherwise, a lost master password means a lost vault.

**Conflicts.** The server keeps a per-item revision date. When a client sends its last-known revision date and the server's copy is newer, the server rejects the write: "The cipher you are updating is out of date". The user must sync and redo the edit ([community thread](https://community.bitwarden.com/t/cipher-out-of-date-error-bitwarden-password-manager/86851), [Vaultwarden commit](https://ark.sudovanilla.org/Infrastructure/vaultwarden/commit/cec1e87679cfd0e2f0bce9b7dc3256dbbd2effa8)). Vaultwarden returns "The client copy of this cipher is out of date. Resync the client and try again" ([Dec 2025 thread](https://community.bitwarden.com/t/edit-error-cypher-out-of-date-2025-12-0-clients-unofficial-third-party-server/91957)). This is server-arbitrated optimistic concurrency per item, with no merge.

**Joining a device.**

- The user enters an email, then the master password or "Log in with device", with 2FA if enabled.
- Since February/March 2025, accounts without 2FA must also enter an emailed one-time code on an unrecognized device. SSO logins are exempt, any 2FA method opts out, and users who lose email access must contact support ([new device verification](https://bitwarden.com/help/new-device-verification/), [community notice](https://community.bitwarden.com/t/security-update-new-device-verification-coming-march-4th/79925)).

**Weaknesses / incidents.**

- After the LastPass breach in January 2023, critics noted that Bitwarden's low client defaults and server-side iterations gave similar protection. Bitwarden first planned 350,000 iterations, then moved to 600,000 ([palant.de](https://palant.de/2023/01/23/bitwarden-design-flaw-server-side-iterations/), [ghacks](https://www.ghacks.net/?p=184418)). Existing accounts kept old values until prompted; the prompt now forces an upgrade at unlock ([KDF help](https://bitwarden.com/help/kdf-algorithms/)).
- The email salt and the hosted KDF parameters mean the server picks the KDF the client runs (inferred from the [whitepaper](https://bitwarden.com/help/bitwarden-security-white-paper/)).
- The email-code requirement caused lockout loops for users whose email password was stored in Bitwarden ([new device verification](https://bitwarden.com/help/new-device-verification/)).

### Standard Notes

**What syncs.** Notes, tags, files, settings, items keys and "smart views". Each record is an item with a UUID ([encryption whitepaper](https://standardnotes.com/help/security/encryption)).

**Where stored.** On Standard Notes' servers or a self-hosted Docker stack, which is the only supported self-host method ([self-hosting](https://standardnotes.com/help/self-hosting/getting-started)). The stack has an API gateway, syncing server and worker, auth and auth worker, a database, and Redis as cache and messaging ([self-hosting](https://standardnotes.com/help/self-hosting/getting-started)). Proton acquired Standard Notes on 2024-04-10. It stays open source and kept its free tier ([TechCrunch](https://techcrunch.com/2024/04/10/proton-standard-notes/), [TechRadar](https://techradar.com/pro/security/proton-has-acquired-encrypted-note-taking-app-standard-notes)).

**Keys (protocol 004).** All from the [whitepaper](https://standardnotes.com/help/security/encryption):

- **Root key.** The client makes a random 256-bit seed. The salt is the first 128 bits of `SHA256(identifier:seed)`. Argon2id runs with 64 MiB, 5 iterations and parallelism 1, giving a 512-bit output that is split into a `masterKey`, never sent, and a `serverPassword` used for login.
- **Two layers below it.** Random 256-bit **items keys** are encrypted by the master key and synced as ordinary items. Each item has its own random `item_key`, encrypted by the items key.
- **Cipher.** XChaCha20-Poly1305 with a 192-bit nonce. The format is `004:nonce:ciphertext:base64(authenticated_data)`. The authenticated data binds the UUID, the version and, for items keys, the key params.
- **Password change.** Only "a few KB" of items keys are re-encrypted, not all data.
- **Key recovery wizard.** If an items key was made under older key params, the client asks for the password as it was then, uses a temporary root key and discards it. An opt-in "strict sign-in" blocks protocol downgrade by the server.
- There is no recovery for a forgotten password. This is inferred: the whitepaper describes no escrow.

**Conflicts.** The server compares each incoming item's `updated_at_timestamp` with its own. A mismatch beyond a minimal interval returns a `ConflictingData` conflict instead of writing ([TimeDifferenceFilter.ts](https://github.com/standardnotes/server/blob/main/packages/syncing-server/src/Domain/Item/SaveRule/TimeDifferenceFilter.ts)). The client then applies one of these strategies per item type: `KeepBase`, `KeepApply`, `KeepBaseDuplicateApply`, `DuplicateBaseKeepApply` or `KeepBaseMergeRefs` ([ConflictStrategy.ts](https://github.com/standardnotes/app/blob/main/packages/models/src/Domain/Abstract/Item/Types/ConflictStrategy.ts)). Notes usually end up as a duplicate whose `conflict_of` field points to the original UUID, and the user resolves it by hand ([PayloadsByDuplicating.ts](https://github.com/standardnotes/app/blob/main/packages/models/src/Domain/Utilities/Payload/PayloadsByDuplicating.ts)). During server incidents, users saw bursts of conflicted copies ([Standard Notes blog, "microupdate: server status and conflicts"](https://standardnotes.org/blog/microupdate-server-status-and-conflicts)) (the page returned an error when fetched; title and snippet seen in search only, unverified).

**Joining a device.** The user signs in with email and password. The client fetches the key params, which is a public request unless 2FA is on, derives the root key and decrypts the items keys ([whitepaper](https://standardnotes.com/help/security/encryption)).

**Weaknesses.** The server sees the email, item UUIDs, content types, sizes and timestamps; the authenticated data is base64, not encrypted ([whitepaper](https://standardnotes.com/help/security/encryption)). The key params are publicly fetchable, which allows enumeration and offline guessing against the salt (inferred from the [whitepaper](https://standardnotes.com/help/security/encryption)). Self-hosted servers need a separate subscription arrangement for premium features ([self-hosting](https://standardnotes.com/help/self-hosting/getting-started), linked articles not read, unverified).

### Part 1 takeaways

1. **Three-way merge over a "last synced" snapshot works for trees on dumb storage, but needs failsafes.** Floccus diffs local and remote against a cached base, keeps id mappings, reconciles move/remove/create races, lets local win ties, and refuses runs that would delete or add more than 20% ([Default.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/strategies/Default.ts)). Even so, user reports of wiped or duplicated bookmarks persist ([AMO](https://firefox.marketplace.browsertotal.com/en-US/firefox/addon/floccus/reviews/?score=1)). A browser has the same API gaps that Floccus documents ([CONSIDERATIONS.md](https://github.com/floccusaddon/floccus/blob/develop/CONSIDERATIONS.md)). A Go core that owns the bookmark store can keep stable ids, record changes and batch them, which removes most of these gaps.
2. **Concurrency control on user storage is advisory locks plus conditional writes.**
   - Floccus uses a `.lock` file with a 2-minute refresh and 15-minute takeover, plus ETag `If-Match` on S3 ([S3.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/adapters/S3.ts)).
   - Joplin uses per-client lock files with SYNC and EXCLUSIVE types and a versioned target upgrade ([sync lock spec](https://joplinapp.org/help/dev/spec/sync_lock)).
   - xBrowserSync, Bitwarden and Standard Notes use server-checked revision timestamps ([xBS](https://github.com/xbrowsersync/api/blob/master/src/services/bookmarks.service.ts), [SN](https://github.com/standardnotes/server/blob/main/packages/syncing-server/src/Domain/Item/SaveRule/TimeDifferenceFilter.ts)).

   Backends without compare-and-set, such as plain WebDAV and some Drive APIs, need a design that tolerates lost lock races. Append-only per-device files avoid the problem altogether (this is a design inference).
3. **Use a key hierarchy, not passphrase-derived content keys.**
   - Floccus and xBrowserSync derive the content key straight from the passphrase ([Crypto.ts](https://github.com/floccusaddon/floccus/blob/develop/src/lib/Crypto.ts), [xBS crypto](https://github.com/xbrowsersync/app/blob/master/src/modules/shared/crypto/crypto.service.ts)).
   - Joplin, Bitwarden and Standard Notes wrap random data keys, so a password change re-wraps a few KB ([Joplin](https://joplinapp.org/help/dev/spec/e2ee), [BW](https://bitwarden.com/help/kdf-algorithms/), [SN](https://standardnotes.com/help/security/encryption)).
   - Joplin's sync-time key-pair creation and multiple master keys show the race when two offline devices both "enable encryption" ([Joplin spec](https://joplinapp.org/help/dev/spec/e2ee)).
4. **Device joining.** The best prior art is an ephemeral public key from the new device, a fingerprint phrase checked on an existing device, and the data key encrypted to the ephemeral key with a short TTL. This is Bitwarden's Log in with device: 15 minutes, keys purged afterwards ([BW](https://bitwarden.com/help/log-in-with-device/)). It does not depend on a server and maps onto QR plus a shared folder or LAN. The other designs make the user retype a passphrase on every device (Floccus, Joplin, KeePass, Obsidian Sync) or paste an encrypted setup blob (LiveSync Setup URI) ([LiveSync](https://github.com/vrtmrz/obsidian-livesync)).
5. **Conflicts: per-type policy, and never silent loss.** Obsidian merges Markdown with diff-match-patch, uses last-write-wins for binaries, and overlays keys for JSON, and it added an opt-out to conflict files after duplicate-text complaints ([Obsidian](https://obsidian.md/help/sync/troubleshoot)). The others keep the losing version somewhere visible ([Joplin](https://joplinapp.org/help/apps/conflict), [SN](https://github.com/standardnotes/app/blob/main/packages/models/src/Domain/Utilities/Payload/PayloadsByDuplicating.ts), [KeePass](https://keepass.info/help/v2/sync.html)):
   - Joplin moves it to a Conflicts notebook.
   - Standard Notes keeps a `conflict_of` duplicate.
   - KeePass and KeePassXC keep it as entry history, with a tombstone list for deletes.

   KeePass's UUID, mtime, history and deleted-objects model is the closest fit for passwords and bookmarks. One client that writes without merging destroys the guarantee ([ctrl.blog](https://www.ctrl.blog/entry/keepass-file-conflicts-android/)).
6. **Plaintext metadata leaks structure even when content is encrypted.**
   - Joplin keeps `id, parent_id, updated_time, deleted_time, type_` in the clear ([BaseItem.ts](https://cdn.jsdelivr.net/npm/@joplin/lib@3.5.1/models/BaseItem.ts)).
   - Obsidian Sync exposes devices, timestamps and deterministic hashes ([Obsidian](https://obsidian.md/help/sync/security)).
   - LiveSync makes path obfuscation an option ([LiveSync settings](https://avyl.ru/self-hosted/obsidian-livesync/src/commit/3d4ad4a3b446fa8f825b3989f0fca8c385cd3db1/docs/settings.md)).

   Decide early what the storage provider may see: file names, counts, sizes, timing.
7. **Recovery is the gap.** Every user-storage app here has no recovery beyond "keep your password or key file". Bitwarden alone offers escrow-style options, emergency access to a contact's RSA key and org account recovery, and both need its server ([emergency](https://bitwarden.com/help/emergency-access/), [recovery](https://bitwarden.com/help/account-recovery/)).

---

## User-controlled storage apps (part 2) and UX references

Researched 2026-11-17. Every claim links to its source. "(unverified)" marks claims I could not confirm from a primary source. "(secondary)" marks claims sourced only from press or aggregator coverage.

### Anytype (any-sync)

**What syncs.** Everything in a "Space": pages, databases, chats and files. A Space is "a cryptographic asset that encapsulates a collection of digital Objects, governed by an Access Control List (ACL)" ([sync.any.org concepts](https://sync.any.org/concepts/)). Search indexes are built locally, are not encrypted and never sync, so each device builds its own ([Anytype docs](https://doc.anytype.io/anytype-docs/data-and-security/how-we-keep-your-data-safe)).

**Where stored / layout.** Each Object is a DAG of content-addressed Changes. Changes with no successors are the object's "heads", and each Change records the ACL head that was current when it was made ([concepts](https://sync.any.org/concepts/)). Changes are signed ([any-sync README](https://github.com/anyproto/any-sync)). Some Changes are "probabilistically promoted to snapshots" so a client can skip replaying the full history ([concepts](https://sync.any.org/concepts/)). The network has four Go node types ([networking](https://sync.any.org/networking/), [README](https://github.com/anyproto/any-sync)):
- **Coordinator nodes** map channel IDs to sync nodes by consistent hashing, sign channel headers, and publish the "Subnet Configuration".
- **Sync nodes** are always-online replicas of space data.
- **File nodes** store blobs in an IPLD structure.
- **Consensus nodes** keep a single source of truth for ACLs, using RAFT to validate signed changes.

The nodes are a "blind messenger": backup nodes hold only the first-layer key, which groups changes by object, and cannot read content, which is AES in CFB mode under a second key ([Anytype docs](https://doc.anytype.io/anytype-docs/data-and-security/how-we-keep-your-data-safe)).

**Self-hosting.** Anytype maintains [any-sync-dockercompose](https://www.github.com/anyproto/any-sync-dockercompose). The community [any-sync-bundle](https://pkg.go.dev/github.com/grishy/any-sync-bundle) merges all the node types into one Go binary. It needs MongoDB and Redis, which its all-in-one image bundles, and S3 is optional (summarised from the [bundle's pkg.go.dev page](https://pkg.go.dev/github.com/grishy/any-sync-bundle)). One community guide reports ports 33010/tcp and 33020/udp and about 250–300 MB of RAM ([Codeberg guide](https://codeberg.org/Safecast/AnyType-VPS), secondary). To join a self-hosted network, the client logs out, picks "Self-hosted" on the onboarding gear menu, and uploads a network `.yml`. There is no in-place migration: you export, switch network and re-import, and "all your devices should be manually switched to the same self-hosted network" ([self-host docs](https://doc.anytype.io/anytype/data/sync-and-backup/self-host)).

**Local-only and LAN.** Peers find each other on the LAN with mDNS and fetch authorised channels directly. A device with internet can bridge a LAN-only peer to a sync node ([networking](https://sync.any.org/networking/)). Local-only mode turns off the backup node and is labelled experimental, with a data-loss warning ([local-only docs](https://doc.anytype.io/anytype/data/sync-and-backup/local-only)). Users report that mDNS never starts in airplane mode because no network interface is up ([forum](https://community.anytype.io/t/local-only-mode-images-and-media-fail-to-load-offline-on-android/30991)).

**Keys.** A 12-word BIP-39 seed derives Ed25519 keys by SLIP-10 ([access-control](https://sync.any.org/access-control/)):
- Master key at `m/44'/code'/index'`.
- Identity key at `.../0'`, which signs ACL record headers.
- A random per-device peer key, used for the P2P handshake.
- A metadata key.
- A symmetric AES **read key** per space, encrypted to each member's public key.

Every ACL change (add or remove a member, "Change Read Key") writes a new read key, encrypted for the current members, which also encrypts the previous key so members can still read old history. ACL changes need a consensus node ([access-control](https://sync.any.org/access-control/)).
- **New device:** you type the Key (the recovery phrase) on the new device. Or, on the old device, open Settings > Recovery Phrase, reveal the blurred QR code under the phrase and scan it with the new device ([Anytype docs](https://doc.anytype.io/anytype/basics/key)). The QR code simply carries the master secret.
- **Recovery:** Anytype holds no copy of the Key, so a lost Key means a lost vault ([docs](https://doc.anytype.io/anytype/basics/key)). On Mac, the phrase is stored in Keychain by default ([recovery phrase doc](https://doc.anytype.io/d/data-and-security/what-is-a-recovery-phrase)).

**Conflicts/merge.** CRDT. When a client rebuilds state from multiple heads, it topologically sorts Changes by hash for a deterministic order. The next local Change references every current head, which merges them ([concepts](https://sync.any.org/concepts/)).

**Weaknesses/incidents.** On 18 Feb 2026, around the 0.54.0 update, a crash while settings were being written left the settings file empty. The app fell back to its defaults, which flipped local-only users onto the Any Network and **uploaded their local-only spaces**. The data was E2EE, but files already in S3 could not be deleted. The fix was crash-safe settings writes with fsync, a backup and auto-restore ([post-mortem](https://community.anytype.io/t/port-mortem-local-only-mode-and-sync-bug/30158)). The lesson: a privacy-critical mode must default to safe when its config is missing.

### Logseq

**What syncs.** There are two products ([Logseq split announcement](https://logseq.io/p/e3YDyX5AYr)):
- The **file-based** app: a folder of Markdown or Org files plus assets.
- The **DB version**, which moves the graph into a database (SQLite, unverified detail).

**Logseq Sync (file version).** A paid beta for active Open Collective backers at $5 or $15 a month, for up to 10 graphs, chosen per graph. It works on desktop, Android and iOS, not on the web ([Logseq docs](https://raw.githubusercontent.com/logseq/docs/master/pages/Logseq%20Sync.md)). Encryption:
- Files and assets are encrypted with **age** under a per-graph password. File names and paths are encrypted too, and the ciphertext lives on AWS ([docs](https://raw.githubusercontent.com/logseq/docs/master/pages/Logseq%20Sync.md)).
- Logseq "does not store your password" ([forum](https://discuss.logseq.com/t/privacy-question-for-logseq-sync/18465)).
- Encryption covers only data leaving the device ([forum](https://discuss.logseq.com/t/deprecation-of-on-disk-encryption/12334)).
- The crypto is in Logseq's Rust `rsapi` (crate `lsq-encryption`, "file encryption based on age-encryption") ([rsapi](https://awesome.ecosyste.ms/projects/github.com%2Flogseq%2Frsapi)).

The docs say not to combine Sync with iCloud, Syncthing or Dropbox, and not to commit `logseq/graphs-txid.edn` to git ([docs](https://raw.githubusercontent.com/logseq/docs/master/pages/Logseq%20Sync.md)). The protocol is simple enough that a third party wrote a self-hosted S3 backend, still "somewhat implemented" ([bcspragu/logseq-sync](https://github.com/bcspragu/logseq-sync)).

**RTC sync (DB version).** The changelog for 16 Nov–6 Dec 2025 lists ([changelog](https://discuss.logseq.com/t/logseq-db-changelog/30013/33)):
- RSA key-pair generation for synced graphs.
- AES encryption of graph uploads, operations and assets.
- The E2EE password stored in the system keychain.
- A "forgot password" reset.
- Grant-access for encrypted shared graphs.
- 2,000-op incremental batches, with 10-second WebSocket and S3 timeouts.

A July 2026 update says the DB beta and RTC sync are open to all active monthly backers ([Open Collective](https://opencollective.com/logseq/updates), secondary summary). RTC is not open source ([forum](https://discuss.logseq.com/t/will-logseq-sync-be-open-source/31903)), and it is a paid Pro feature ([unofficial FAQ](https://logseq.io/page/e87c7359-51f7-44fe-87b3-4a0cd9f2dee3/695feeec-88be-4c5b-8bf2-572513c2f730)).

**Conflicts/merge.**
- **File-based Sync:** users report a "smart merge" overwriting content with empty entries when they edit before the sync has finished ([forum](https://discuss.logseq.com/t/logseq-sync-content-got-overwritten-when-edit-without-fully-synced/15041)).
- **iCloud, Drive or Syncthing:** Logseq leaves merging to the file provider. The team blames iCloud "sticking" on small-file uploads and advises: avoid editing on several devices at once, confirm the sync has finished before switching devices, and recover from `logseq/bak` ([official thread](https://discuss.logseq.com/t/im-using-logseq-with-icloud-but-experiencing-data-loss-or-file-conflict-errors-whats-going-on/13393)). Users report duplicated files and lost iPad edits ([forum](https://discuss.logseq.com/t/lost-2-hours-of-work-due-to-icloud-sync-issue-is-it-common/7451), [forum](https://discuss.logseq.com/t/data-loss-happened-twice-i-cant-trust-logseq-anymore/6896?page=2)).
- **Git:** the team points iOS users to git through Working Copy ([official thread](https://discuss.logseq.com/t/im-using-logseq-with-icloud-but-experiencing-data-loss-or-file-conflict-errors-whats-going-on/13393)).

**Joining a device.** With Sync, you log in, pick a remote graph and enter its password (inferred from the per-graph password design in the [docs](https://raw.githubusercontent.com/logseq/docs/master/pages/Logseq%20Sync.md), unverified UI detail).

**Weaknesses.** A whole-file text store on a generic file-sync provider gives last-writer-wins or duplicate files, with no merge. Logseq's own "smart merge" has produced overwrites. The company is moving to a closed, server-mediated CRDT/op-log design with E2EE.

### Cryptomator

**What syncs.** Nothing by itself. Cryptomator encrypts a vault folder and the cloud provider's client syncs the ciphertext: "synchronization is handled by the cloud providers app" ([community](https://community.cryptomator.org/t/how-sync-conflicts-works/10095)).

**Vault format 8 layout.**
- `vault.cryptomator` is a JWT signed with the 512-bit raw masterkey. It carries `format: 8`, `shorteningThreshold: 220`, and `cipherCombo` (`SIV_GCM` or `SIV_CTRMAC`). `kid` points at the masterkey ([architecture](https://docs.cryptomator.org/security/architecture/)).
- `masterkey.cryptomator` holds two 256-bit CSPRNG keys, encryption and MAC. They are wrapped with AES Key Wrap (RFC 3394) under a KEK derived with scrypt (N=32768, r=8). A `versionMac` HMAC guards against downgrade ([architecture](https://docs.cryptomator.org/security/architecture/)).
- **Files:** a 68-byte header (12-byte nonce, then a 40-byte AES-GCM payload holding the 32-byte per-file content key, then a 16-byte tag). Content follows in 32 KiB AES-GCM chunks, each +28 bytes. The AAD is the chunk number plus the header nonce ([vault docs](https://docs.cryptomator.org/security/vault/)).
- **Names:** AES-SIV over the NFC name, with the **parent directory ID as associated data** so a moved file is detected. The result is base64url plus `.c9r` ([vault docs](https://docs.cryptomator.org/security/vault/)).
- **Directories:** each directory has a random ID stored in `dir.c9r`. Its ciphertext path is `d/XX/YYYY…` from Base32(SHA-1(AES-SIV(dirId))). This gives a **flat tree**, so renaming a folder changes one small file, not a subtree ([vault docs](https://docs.cryptomator.org/security/vault/)).
- The optional `dirid.c9r` backs up the parent dir ID for repair. Names longer than 220 characters become a SHA-1 `.c9s` folder. The 220 limit leaves room for sync-provider conflict suffixes inside 255 ([vault docs](https://docs.cryptomator.org/security/vault/)).

**Keys / recovery.** The recovery key is a word list derived on the fly from the masterkey. It is never stored, and can be shown again later with the password. It can reset the password, and stays valid after a reset ([password and recovery key docs](https://docs.cryptomator.org/desktop/password-and-recovery-key/)). Desktop 1.18 (Nov 2025) can rebuild a lost or damaged masterkey file and vault config from the recovery key ([AlternativeTo](https://alternativeto.net/news/2025/11/cryptomator-1-18-desktop-now-lets-you-restore-lost-or-damaged-master-keys-and-vault-config), secondary).

**Joining a device.** You open the same synced folder on the new device and type the vault password. No pairing is needed, because the wrapped key travels inside the vault (implied by the [architecture](https://docs.cryptomator.org/security/architecture/)).

**Cryptomator Hub (teams).**
- Each user has an **Account Key** ("setup code"), which is needed to register each new device. A device key decrypts only the user key, which is stored per device as an ECDH-ES JWE, so adding a device never re-encrypts vault keys ([Hub 1.3.0 blog](https://cryptomator.org/blog/2023/12/01/hub-1.3.0), [access-vault docs](https://docs.cryptomator.org/hub/access-vault/)).
- Regenerating the Account Key keeps existing devices trusted ([your-account docs](https://docs.cryptomator.org/hub/your-account/)).
- Hub 1.4.0 (Apr 2025) added a Web of Trust and events for user-key changes ([blog](https://cryptomator.org/blog/2025/04/15/hub-1.4.0)).

**New format.** Hub is adding UVF (Universal Vault Format), "inspired by and partially based on Vault Format 8". Its metadata lives in `vault.uvf`, whose primary copy is held in Hub, and the format supports key rotation. Format 8 vaults will not be migrated ([Hub PR #274](https://www.github.com/cryptomator/hub/pull/274)). Release date unverified.

**Conflicts.** Cryptomator detects the provider's conflict copies and shows them with the provider's suffix added to the decrypted name. You resolve them by hand ([sync-conflicts docs](https://docs.cryptomator.org/desktop/sync-conflicts/)). Users report it resolves them lazily, when the folder is opened ([community](https://community.cryptomator.org/t/advice-to-resolve-file-conflict-in-vault/636/2)).

**Weaknesses.**
- File sizes and the directory count leak, a consequence of the design above.
- There is no merge.
- A lost or desynced `dir.c9r` orphans a subtree, which is why `dirid.c9r` exists ([vault docs](https://docs.cryptomator.org/security/vault/)).

### Syncthing

**What syncs.** Folders of files, peer to peer, over the Block Exchange Protocol (BEP) on **TLS 1.3+** ([BEP v1](https://docs.syncthing.net/specs/bep-v1.html)).

**Layout / protocol.**
- Files are split into blocks from 128 KiB to 16 MiB, using the smallest size that gives fewer than 2000 blocks.
- Each file carries a **version vector**: counters keyed by the first 64 bits of a device ID.
- Each folder has a random 64-bit Index ID plus a sequence number, so peers can exchange only index deltas.
- The Cluster Config message is sent first after the TLS handshake ([BEP v1](https://docs.syncthing.net/specs/bep-v1.html)).

Syncthing 2.0 (Aug 2025) moved the index database from LevelDB to SQLite and drops deletion tombstones after six months by default ([AlternativeTo](https://alternativeto.net/news/2025/8/syncthing-2-0-debuts-with-sqlite-migration-new-logs-platform-changes-and-much-more), [Neowin](https://www.neowin.net/news/syncthing-20-released-with-major-changes-switching-from-leveldb-to-sqlite/), secondary).

**Keys / identity.**
- On first start, Syncthing generates `key.pem` and `cert.pem`.
- The device ID is SHA-256 of the DER certificate, in base32, split into groups with check digits, for example `MFZWI3D-BONSGYC-…` (8×7 characters).
- **Both sides must add each other** ("needs to be done twice, once for each side").
- A PIN-based pairing was only ever proposed ([device IDs](https://docs.syncthing.net/dev/device-ids.html)).

**Introducer.** A device marked as an introducer pushes its device list to the devices it introduces (IDs, labels, addresses). Removals propagate on the next connection. Two devices must not be introducers of each other ([introducer docs](https://docs.syncthing.net/users/introducer.html)).

**Untrusted (encrypted) devices** (still "beta / testing only"). You set a per-folder password, and the untrusted peer uses the folder type "Receive Encrypted".
- **Crypto:** XChaCha20-Poly1305 plus AES-SIV, with a key derived from **password + folder ID** by scrypt.
- **Hidden:** names (as whole paths), times, hashes and tree structure.
- **Leaked:** folder ID, label and approximate file sizes.
- **Cost:** blocks cannot be deduplicated, so a rename means a full re-upload.
- **Recovery:** `syncthing decrypt` recovers plaintext ([untrusted docs](https://docs.syncthing.net/users/untrusted.html)).

This is the closest prior art for "a dumb storage peer that holds ciphertext".

**Conflicts.**
- The file with the **older mtime** loses and is renamed `<name>.sync-conflict-<date>-<time>-<modifiedBy>.<ext>`.
- On an mtime tie, the device with the larger first 63 bits of its device ID loses.
- When an edit conflicts with a deletion that wins, the edited copy survives as a conflict file.
- Conflict copies sync like normal files, so every device sees them ([syncing docs](https://docs.syncthing.net/users/syncing.html)).

Versioning (trashcan, simple, staggered, external, under `.stversions`) is **off by default** ([versioning](https://docs.syncthing.net/users/versioning.html)).

**Discovery.** Global discovery announcements go out every 30 minutes, and LAN broadcast or multicast every 30 seconds, carrying the device ID and port, so anyone on the LAN learns them. Relays learn device IDs but see only TLS traffic ([security docs](https://docs.syncthing.net/users/security.html)).

**Joining a device (UX).** You copy or scan the device ID on one side and accept the "new device wants to connect" prompt on the other, then accept each shared folder. That two-sided accept follows from the [device-ID flow](https://docs.syncthing.net/dev/device-ids.html); the UI wording is unverified.

**Weaknesses/incidents.** The official Android app was discontinued: announced Oct 2024, last release Dec 2024, repo archived 3 Dec 2024. The reasons given were Google Play publishing friction and no maintainer. A fork continues it ([forum](https://forum.syncthing.net/t/discontinuing-syncthing-android/23002), [repo](https://github.com/syncthing/syncthing-android)). Merging is whole-file only, by mtime.

### Apple Notes / CloudKit / CKSyncEngine

**What syncs and how it is encrypted.**
- Under Standard Data Protection, 15 categories are E2EE, including Keychain passwords. With **Advanced Data Protection (ADP)** this rises to **25**.
- Notes, iCloud Drive and Photos become E2EE only with ADP. Safari History, Tab Groups and iCloud Tabs are E2EE under both modes, while Bookmarks need ADP.
- Metadata (dates, sizes, checksums) stays under Apple-held keys even with ADP ([Apple support 102651](https://support.apple.com/en-us/102651)).

**Keys.**
- Each CloudKit container's private database has a key hierarchy rooted in a **CloudKit Service key** generated on a trusted device.
- E2EE service keys sync between devices through iCloud Keychain, even if the user doesn't sync passwords. Their private keys are never available to Apple's servers.
- "Available-after-authentication" keys, such as those for non-ADP Photos and Drive, sit in iCloud HSMs ([Platform Security](https://support.apple.com/guide/security/sec3cac31735/web)).

**Recovery.**
- **Keychain escrow:** HSM clusters check the iCloud security code by SRP. A majority of the cluster must agree before it releases the record. After **10 failed attempts**, "the HSM cluster destroys the escrow record and the keychain is lost forever" ([escrow](https://support.apple.com/guide/security/escrow-security-for-icloud-keychain-sec3e341e75d/web)).
- **Other routes:** Account Recovery Contacts and a Recovery Key. If all of these are lost, the E2EE CloudKit data is lost ([Platform Security](https://support.apple.com/guide/security/sec3cac31735/web)).

**CKSyncEngine** (WWDC23; iOS 17 / macOS 14 per a generated binding catalogue, [apisof.net](https://apisof.net/catalog/af5327761a0aa983cbc5327f47b88d2b), not Apple's page). From the [WWDC23 session](https://developer.apple.com/videos/play/wwdc2023/10188/):
- **What it handles:** scheduling (through the system task scheduler: network, battery), push subscriptions, batching through `nextRecordZoneChangeBatch`, and retries of transient errors.
- **What the app keeps:** the serialized engine state from each `stateUpdate` event.
- **Conflicts:** they arrive as `serverRecordChanged` in `failedRecordSaves`, carrying the server record. The app merges and re-queues.
- **Model:** zones are saved before records, with one engine per database.
- **Users:** Freeform, and `NSUbiquitousKeyValueStore`, which was rewritten on it.

`CKRecord.encryptedValues` gives per-field E2EE, with these limits:
- Only for new fields.
- Not in the public database.
- Encrypted fields cannot be indexed.
- `CKAsset` is encrypted by default, while `CKRecord.Reference` stays plaintext.
- A Keychain reset makes old data undecryptable, and CloudKit returns a specific error ([Apple "Encrypting user data"](https://developer.apple.com/documentation/cloudkit/encrypting-user-data.md)).

Whether Notes itself uses CKSyncEngine is unverified.

**Developer complaints (2026).**
- New installs replay old deletion tombstones for hours.
- There is no "initial sync" token.
- Several engines on one database clash.
- After `quotaExceeded`, the failed item is not re-queued.
- The sample's conflict handling is basic, though `ancestorRecord` would allow a three-way merge ([Michael Tsai roundup, 1 Apr 2026](https://mjtsai.com/blog/2026/04/01/cksyncengine/)).

**Joining a device.** You sign in with the Apple Account. The device then joins the keychain trust group by proving knowledge of an existing device's passcode, which is the "iCloud security code" step in [escrow](https://support.apple.com/guide/security/escrow-security-for-icloud-keychain-sec3e341e75d/web). The exact modern prompt is unverified.

**Incident.** Under a UK technical capability notice, Apple stopped offering ADP to new UK users from 21 Feb 2025 ([MacRumors](https://www.macrumors.com/2025/02/21/apple-pulls-encrypted-icloud-security-feature-uk/)). The UK reportedly dropped the demand in Aug 2025 ([TechRepublic](https://www.techrepublic.com/article/news-uk-drops-apple-backdoor/)). Whether ADP has been restored in the UK is unverified. The lesson: E2EE that depends on the provider's policy can be withdrawn by jurisdiction. User-held storage plus client-side keys avoids this.

### Tailscale (device pairing UX reference)

**Normal join.** The device signs in with an identity provider. **Device approval** (on every plan) can hold a new device "awaiting approval" until an Owner or Admin approves it on the Machines page. Until then it "cannot send or receive traffic", and it connects right after approval with no restart ([device approval](https://tailscale.com/kb/1099/device-approval)).

**Auth keys.** The types are one-off, reusable, ephemeral, pre-approved and tagged. A key lives 1–90 days, 90 by default. An expired key doesn't remove the devices it already added, and node keys default to 180 days ([auth keys](https://tailscale.com/kb/1085/auth-keys)).

**Tailnet Lock** (cryptographic trust that the server cannot forge).
- Each node has a Tailnet Lock key (TLK), whose private half never leaves the node.
- At least **2** and at most **20** signing nodes form the Tailnet Key Authority (TKA). "All new node keys must be signed by an existing Tailnet Lock key."
- Unsigned nodes show a **Locked out** badge. You sign one:
  - by "Sign this node" on a desktop signing node;
  - by **QR code on an iOS signing device**;
  - with a signing URL;
  - or with `tailscale lock sign`.
- Auth keys can be pre-signed for fleets.
- `lock init` prints **10 disablement secrets**, and any one of them disables the lock. If you lose them all and gave none to support, "the tailnet cannot be recovered".
- Revoking a key needs co-signatures from several signing nodes ([Tailnet Lock](https://tailscale.com/kb/1226/tailnet-lock)).

**Relevance.** Tailnet Lock is the template for "the server relays, existing devices authorise". It has a quorum of signing devices, a QR or URL to sign the newcomer, and printed break-glass secrets.

### Signal (device linking by QR)

**Mechanism** ([Signal blog, 27 Jan 2025](https://signal.org/blog/a-synchronized-start-for-linked-devices/)).
1. The new device registers a temporary provisioning address and shows a QR code holding that address and a fresh Curve25519 public key.
2. The primary phone scans it and sends a provisioning message encrypted to that key.
3. **History transfer (2025):** the phone builds one compressed archive of messages, reactions, receipts and call history, and encrypts it with a single-use 256-bit AES key carried inside the provisioning message.
4. The archive passes through Signal's servers, stored only temporarily, and the key is discarded after import.
5. Attachments travel as pointers, and since the server deletes attachments 45 days after upload, only the last 45 days of media come across.
6. The user chooses between transferring history and starting fresh.

**Limits.** Up to **5** linked devices. The phone must come online every **30 days**, and linked devices unlink after **45 days** inactive ([summarised from Signal support](https://support.signal.org/hc/en-us/articles/360007320551)). Signal's page returned 403 to my fetch, so these numbers come from search snippets of that page. Since Aug 2026 (Android 8.20, iOS 8.22), a second phone can be linked as well, not only tablets and desktops ([Android Headlines](https://www.androidheadlines.com/2026/08/signal-finally-fixes-a-major-limitation-with-android-device-linking.html)).

**Weaknesses/incidents.** In Feb 2025, Google Threat Intelligence reported Russia-aligned groups (UNC5792, UNC4221, APT44) phishing victims with **malicious linking QR codes** dressed as group invites. Signal also lacked central logging of new links ([GTIG](https://cloud.google.com/blog/topics/threat-intelligence/russia-targeting-signal-messenger), [Cyberscoop](https://cyberscoop.com/russia-threat-groups-target-ukraine-signal/)). In Sep 2026, Netzpolitik reported that German customs routinely use linked-device logins, coaching or phishing targets into scanning or typing linking codes. This has been an official option since Aug 2025 ([Privacy Guides, 30 Sep 2026](https://www.privacyguides.org/news/2026/09/30/german-police-are-using-linked-devices-to-read-signal-messages-without-cracking-the-encryption.md)). **Lesson:** a QR-code link is only as safe as the user's judgement about *which* screen they are scanning. Show the requesting device's name and location, ask for confirmation on both ends, and announce new links on every device.

### WhatsApp (multi-device)

- **Identity:** each companion device has **its own identity key**. Security codes cover all of a user's device identities, and Automatic Device Verification means users re-check codes only when an account re-registers ([Meta engineering, Jul 2021](https://engineering.fb.com/2021/07/14/security/whatsapp-multi-device/)).
- **Fan-out:** messages fan out from the client, encrypted separately for each recipient device.
- **History:** on linking, the primary sends an encrypted bundle of recent chats. Its key travels in an E2EE message and is deleted after import ([Meta engineering](https://engineering.fb.com/2021/07/14/security/whatsapp-multi-device/)).
- **Limit:** a phone plus up to **4** non-phone devices, which keep working while the phone is offline ([Meta engineering](https://engineering.fb.com/2021/07/14/security/whatsapp-multi-device/)).
- **Without a camera:** "Link with phone number instead" shows an **8-character code** on the web client, which you type on the phone under Linked Devices. It was in testing in 2023 ([Android Central](https://www.androidcentral.com/apps-software/whatsapp-link-with-phone-number)). Typed codes can be phished from fake pages (same Android Central source; [Privacy Guides](https://www.privacyguides.org/news/2026/09/30/german-police-are-using-linked-devices-to-read-signal-messages-without-cracking-the-encryption.md)).

### Ente (Photos / Auth) — recovery-key UX reference

**Keys** ([architecture](https://ente.com/architecture/)).
- A 256-bit **masterKey** generated on the client by `crypto_secretbox_keygen`.
- A **keyEncryptionKey** from the password by Argon2id (`crypto_pwhash`). It starts at the SENSITIVE ops and memory limits; when a device runs short of memory, the memory limit halves and the ops limit doubles.
- The masterKey is stored server-side wrapped by the keyEncryptionKey. A **recoveryKey** is also wrapped by the masterKey, and the masterKey is also stored wrapped by the recoveryKey.
- An X25519 key pair for sharing. A collectionKey per album and a fileKey per file.
- Ciphers: XSalsa20-Poly1305 for wrapping and XChaCha20-Poly1305 secretstream for file data.
- The "Verification ID" is SHA-256 of the public key, rendered as a BIP-39 mnemonic.

**New device.** Email OTP, then the server returns the encryptedMasterKey, then the password unwraps it locally. A wrong password simply fails to decrypt ([architecture](https://ente.com/architecture/)).

**Recovery.**
- The recovery key is **24 words**, entered in order.
- An email code cannot stand in for it.
- A signed-in device can show it again under Settings > Account > Recovery key ([help](https://ente.com/help/photos/faq/security-and-privacy)).
- Losing both password and recovery key is unrecoverable unless "Legacy" was set up. Legacy wraps the recovery key to a trusted contact's public key; that contact can reset the password and the 2FA and passkeys ([help](https://ente.com/help/photos/faq/security-and-privacy), [architecture](https://ente.com/architecture/)).
- Losing a passkey or 2FA factor can be fixed with the recovery key ([passkeys help](https://ente.com/help/photos/features/account/passkeys)).

**Audits.** Cure53 with Symbolic Software reviewed the cryptography in Feb–Mar 2023 ([Ente blog](https://ente.io/blog/cryptography-audit), [report](https://cure53.de/audit-report_ente-crypto.pdf)). A CERN-sponsored Cure53 audit followed in Oct 2025: 5 researchers over about 2 weeks, server code and infrastructure. They found 15 issues, 12 fixed during the audit, and "no data integrity issues" ([Ente blog, 3 Nov 2025](https://ente.com/blog/cern-audit), [report PDF](https://ente.io/reports/Cure53-Audit-Report-Oct-2025.pdf)).

**Relevance.** A clean pattern:
- a password-wrapped master key held on an untrusted server;
- a recovery key that double-wraps the same master key;
- an optional social-recovery escrow;
- server-side checks that never see a plaintext key.

### Proton (Pass / Drive) — recovery reference

- **Reset is not recovery.** A password reset brings back sign-in but not the data. After a reset, encrypted data, explicitly including **all Proton Pass items**, shows as undecryptable unless a *data* recovery method was set up **before** the reset ([Proton support](https://proton.me/support/recover-encrypted-messages-files)).
- **Data recovery methods:**
  - **Device data backup**, on by default in the web apps: a device that stays signed in can re-decrypt.
  - A 12-word **recovery phrase**.
  - A downloadable **recovery file**.
  - The **old password** ([Proton support](https://proton.me/support/recover-encrypted-messages-files)).
- **Recovery phrase details:** it is generated on the client and never stored on the server. It resets the password and recovers data in one step, and it can bypass 2FA, so it is treated as a second password ([Proton blog](https://proton.me/blog/data-recovery-end-to-end-encryption), [recovery phrase](https://proton.me/support/recovery-phrase)).
- **Signed-in reset:** if you are still signed in, you can reset the password without losing data ([account recovery](https://proton.me/support/set-account-recovery-methods)).
- **Single use:** some methods work only once and must be renewed afterwards ([support](https://proton.me/support/recover-encrypted-messages-files)).

**Relevance.** Proton's UI keeps "account access" and "data recovery" separate. That distinction is what an E2EE browser with user-held keys must explain: a lost key means lost data, however much account help is available.

### Local-first sync libraries (brief, relevant to a Go core)

- **Automerge 3.0** (2025) keeps the compressed format in memory, cutting memory use by more than 10x, with the same file format as 2.x ([Automerge blog](https://automerge.org/blog/automerge-3/)). Official **automerge-go** bindings exist (cgo over the Rust core, [pkg.go.dev](https://pkg.go.dev/github.com/automerge/automerge-go@v0.0.0-20240213171625-b7d9d510d501)). Whether they track 3.0 is unverified.
- **Loro 1.0** (23 Oct 2024) has a stable encoding and history GC, with bindings for Rust, JS/WASM and Swift but no Go ([Loro blog](https://loro.dev/blog/v1.0)). It was at 1.11 by 2026 ([jsDelivr](https://cdn.jsdelivr.net/npm/loro-crdt@1.11.0/README.md)).
- **Evolu:** SQLite plus CRDT, E2EE, with keys from a **12-word mnemonic**. Relays are stateless message buffers that can be self-hosted, and sync is split per "owner". It is still pre-release ([Evolu docs](https://evolu.vercel.app/docs/privacy), [overview](https://evolu.vercel.app/docs/overview)). It is the closest library analogue to "mnemonic + dumb relay".
- **cr-sqlite:** current maintenance status unverified ([repo](https://github.com/vlcn-io/cr-sqlite)). I did not research Yjs, Ditto or Jazz in depth.

### Part 2 takeaways

1. **The seed phrase is the root of trust in almost every design.** Anytype (12-word BIP-39 to SLIP-10 Ed25519), Evolu (12 words), Proton (12-word recovery phrase), Ente (24-word recovery key) and the Cryptomator recovery key are all words that wrap or derive one master secret. Anytype's "show a QR code of the phrase on the old device" is the cheapest join UX, but the QR code *is* the master secret ([Anytype](https://doc.anytype.io/anytype/basics/key)).
2. **Wrap a master key; don't derive data keys from the password.** Ente (Argon2id KEK wraps the master key, recovery key double-wraps it), Cryptomator (scrypt KEK plus RFC 3394 wrap) and Cryptomator Hub (device key unwraps a user key, so adding a device never re-encrypts vault keys) let the password change and devices come and go cheaply ([Ente](https://ente.com/architecture/), [Cryptomator](https://docs.cryptomator.org/security/architecture/), [Hub](https://cryptomator.org/blog/2023/12/01/hub-1.3.0)).
3. **On dumb file storage, use many small, immutable, content-addressed objects, never one mutable file.** Cryptomator's flat dir-ID tree, Anytype's change DAG and Syncthing's untrusted-device layout all hold up on any backend. Logseq and Syncthing, merging whole files by mtime, produce lost edits and conflict copies ([Cryptomator](https://docs.cryptomator.org/security/vault/), [Logseq](https://discuss.logseq.com/t/im-using-logseq-with-icloud-but-experiencing-data-loss-or-file-conflict-errors-whats-going-on/13393), [Syncthing](https://docs.syncthing.net/users/syncing.html)).
4. **Device joining should be "an existing device approves".** Signal's provisioning (QR code with an ephemeral public key, history key inside the encrypted provisioning message) and Tailnet Lock (a quorum of signing devices, QR or URL signing, 10 break-glass secrets) are the best templates. Signal's 2025–26 incidents show the UI must name the requesting device and warn about scanning codes others send you ([Signal](https://signal.org/blog/a-synchronized-start-for-linked-devices/), [Tailscale](https://tailscale.com/kb/1226/tailnet-lock), [GTIG](https://cloud.google.com/blog/topics/threat-intelligence/russia-targeting-signal-messenger)).
5. **Privacy modes must fail closed.** Anytype's Feb 2026 bug uploaded local-only spaces when the settings file was corrupted. The UK ADP withdrawal shows that provider-controlled E2EE can be switched off. Keep the "where does data go" choice in crash-safe state, and default to *not syncing* when it is missing ([Anytype post-mortem](https://community.anytype.io/t/port-mortem-local-only-mode-and-sync-bug/30158), [MacRumors](https://www.macrumors.com/2025/02/21/apple-pulls-encrypted-icloud-security-feature-uk/)).
6. **Explain "reset vs. recover" in the UI.** Proton and Ente both state that a reset without a data-recovery method loses the data, and Apple destroys the keychain after 10 failed escrow attempts. Show the recovery key at setup, let the user see it again from a signed-in device (Ente, Cryptomator), and treat any signed-in device as a recovery path (Proton) ([Proton](https://proton.me/support/recover-encrypted-messages-files), [Ente](https://ente.com/help/photos/faq/security-and-privacy), [Apple escrow](https://support.apple.com/guide/security/escrow-security-for-icloud-keychain-sec3e341e75d/web)).

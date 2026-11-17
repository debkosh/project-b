# Sync

A proposal for discussion with the owner, 2026-11-17. Nothing here is built. The owner's ask:
"support multiple ways to sync... where and how to sync... innovative ways to sync using what the
users control". The research behind it, with a source for each fact, is in
[sync-research/](sync-research/): [places.md](sync-research/places.md) (where data can live),
[prior-art.md](sync-research/prior-art.md) (how browsers and other apps do it) and
[merge-and-keys.md](sync-research/merge-and-keys.md) (merging and encryption). Mocks:
[mocks.html](screenshots/sync/mocks.html), pictures linked below.

## The idea

**Sync is a folder of sealed files that only ever grows, and each device writes only its own
files.** A place to sync through is anything that can hold that folder: list it, read a file,
write a new file, delete a file of its own. Nothing else is asked of the place.

From that one rule:

- **Any place works**, the dumbest included. No two devices ever write the same file, so a
  synced folder never makes a conflict copy and no place needs locks or conditional writes.
  iCloud Drive and Google Drive can't do a safe overwrite between devices
  ([places.md](sync-research/places.md#summary-table-facts-from-above)); with this rule they
  never have to.
- **Several places at once are safe.** Two copies of the folder merge by taking every file in
  either. A Mac on iCloud and the local network carries the phone's changes up to iCloud, so
  the phone syncs with an iCloud Mac although Android can't reach iCloud.
- **The place sees nothing.** Every file is sealed with a key only the user's devices and their
  recovery words hold. File names are keyed hashes, so the place can't count bookmarks or read
  dates.
- **Backup and export are the same format.** A copy of the folder on a USB drive is a backup
  that any new device opens with the recovery words.

Each profile syncs on its own: a profile is the unit of data ([profiles.md](profiles.md)), so
Personal and Work can sync through different places, or one through none.

```
  MacBook (iCloud + nearby) <---- nearby ----> Pixel (nearby + Drive)
        |                                            |
   iCloud Drive                                Google Drive
        |                                            |
  iMac (iCloud) ............................. (any device on Drive)

  Every arrow carries the same sealed files; a device on two places copies between them.
```

## Where the data lives

The settings page lists the places ([mock](screenshots/sync/mock-s1.png)). Each is a small
Go type with four calls: list, read, write a new file, delete one of its own.

| Place | Mac | Phone | How we reach it | What the user gives |
|---|---|---|---|---|
| **A folder** | iCloud Drive, Dropbox, Google Drive for desktop, OneDrive, Syncthing, a USB drive: the provider's own app syncs the folder | A folder through Android's picker: Nextcloud's app, Syncthing-Fork, a USB drive. Google Drive's picker may give only single files, to test ([places.md](sync-research/places.md#2-google-drive-rest-v3)) | Files on disk | Picks the folder |
| **Google Drive** | yes | yes | Drive's REST API; `drive.file`, a non-sensitive scope, so no yearly security assessment ([places.md](sync-research/places.md#access-and-sign-in-1)) | Signs in |
| **Dropbox** | yes | yes | Dropbox's API, the App Folder (`Apps/Project B`), with a long-poll for changes and no server of ours ([places.md](sync-research/places.md#3-dropbox-api-v2)) | Signs in |
| **WebDAV or Nextcloud** | yes | yes | HTTP | Address, user, app password ([mock](screenshots/sync/mock-s8.png)) |
| **S3 bucket** | yes | yes | S3's API: AWS, Cloudflare R2 (no egress fees), Backblaze B2 ([places.md](sync-research/places.md#7-s3-style-buckets)) | Endpoint, bucket, keys |
| **Git repository** | yes | yes | A commit per sync, on a branch of its own; history for free. GitHub allows about 80 content writes a minute and repositories should stay under 1 GB ([places.md](sync-research/places.md#6-a-git-repository)) | Remote and a token |
| **Nearby devices** | yes | yes, while the app is open | Our own: Bonjour on the Mac, `NsdManager` on Android, TLS between paired devices | Turns it on; macOS asks for local network access once, Android 17 asks for `ACCESS_LOCAL_NETWORK` ([places.md](sync-research/places.md#8-device-to-device-on-the-local-network)) |

Left out, and why:

- **iCloud through CloudKit.** Mac-only like the folder, and CloudKit's web service, the only
  way in from Android, is off by default under Advanced Data Protection
  ([places.md](sync-research/places.md#1-icloud-drive-and-cloudkit)). The folder gives the Mac
  iCloud with none of that.
- **Google Drive's hidden app folder.** Hidden from the user and deleted when they remove the
  app from Drive ([places.md](sync-research/places.md#access-and-sign-in-1)). A visible
  `Project B` folder under `drive.file` is one the user can see, copy and delete: theirs.
- **A sync server of ours, and Firefox's.** A self-hoster runs WebDAV (Nextcloud) or an S3
  server, both above. Firefox's sync server still signs in through Mozilla accounts
  ([places.md](sync-research/places.md#9-self-hosted-server)); MinIO's repository is archived.
- **Email, IPFS, a passkey's storage.** Gmail's API needs restricted scopes; IPFS keeps data
  only while someone pins it; passkeys carry a key, not data
  ([places.md](sync-research/places.md#10-short-notes-on-other-options)).

**Signing in to Google and Dropbox is ours to set up once.** Google needs brand verification
even for non-sensitive scopes, and a test app is capped at 100 users with 7-day tokens. Dropbox
reviews an app before its 51st user ([places.md](sync-research/places.md)). Both are free.

## What is in the place

```
Project B/<sync id>/
  devices/<device>.card       each device's name, kind and public keys, signed by a device already in
  keys/<epoch>.ring           the data key of this epoch, wrapped once per device and once for the words
  log/<device>/<n>            a device's changes, sealed, numbered; only that device writes here
  snap/<device>/<n>           a device's full merged state up to a point, so new devices start fast
  tabs/<device>               a device's open tabs, replaced as they change; only that device writes it
  inbox/<device>/<random>     a tab sent to that device; it opens it and deletes the file
  pair/<random>               a join in progress, gone in ten minutes
```

`<device>` and every other name are keyed hashes, never readable ids or dates. A segment holds
many changes, so the place sees only how many segments there are and roughly how big each is,
padded with Padmé to hide exact sizes ([merge-and-keys.md](sync-research/merge-and-keys.md#b4-metadata-leakage-on-user-storage)).

## Keys

**One random data key seals everything, and each holder gets it wrapped.** Joplin, Bitwarden,
Cryptomator and Chrome do the same; Floccus and xBrowserSync derive the key from a passphrase,
so changing it means sealing everything again ([prior-art.md](sync-research/prior-art.md#patterns-that-hold-up)).

- **A device** makes its own key pair when it joins, kept in the Keychain on the Mac and under
  an Android Keystore key on the phone. The data key is wrapped to it with age's post-quantum
  hybrid recipients (ML-KEM-768 with X25519, age 1.3), since the files sit in someone else's
  cloud for years ([merge-and-keys.md](sync-research/merge-and-keys.md#b1-file-and-record-encryption-choices)).
- **Recovery words**, 24 of them, are one more holder: a key that never syncs
  ([mock](screenshots/sync/mock-s5.png)). Shown when sync starts, printable, and again from any
  device in the set. The words and the place open everything on a new device. Without them
  and with no device left, nobody can, us included, and the page says so, as Ente and Proton
  do.
- **A segment** is sealed with the epoch's data key, with its device, number and epoch bound in,
  so the place can't swap, replay or move files between devices. Each device also signs its own
  segments, so one device can't write as another.
- **Later, a passkey** can be one more wrap through WebAuthn's PRF, never the only one: a
  native app needs our domain as the passkey's site, and native Android's support isn't
  confirmed ([merge-and-keys.md](sync-research/merge-and-keys.md#b3-recovery)).

## Joining a device

The device already in approves the new one; the code carries no key
([mocks](screenshots/sync/mock-s3.png), [the phone](screenshots/sync/mock-s4.png),
[the check](screenshots/sync/mock-s3b.png)).

1. On the Mac, Add a device shows a QR code and the same as a typed code, `K7QX-4M2P-9WHD`. It
   names the places, holds a one-time secret, and lasts ten minutes and one device.
2. The phone scans it, or the code is typed. The two devices agree on a key over the place's
   `pair/` file or the local network, by SPAKE2 for a typed code, as magic-wormhole does
   ([merge-and-keys.md](sync-research/merge-and-keys.md#b2-device-keys-joining-removal-and-rotation)).
3. Both show the joining device's name and the same six digits. Yes on the Mac wraps the data
   key to the phone. Naming the device matters: Signal's linking codes were phished in 2025 and
   2026 ([prior-art.md](sync-research/prior-art.md#patterns-that-hold-up)).

A code that is the key, as Brave's chain code is, can be shared by mistake for good: users who
copied the sample codes from Brave's help page shared their sync with strangers
([prior-art.md](sync-research/prior-art.md#patterns-that-hold-up)).

**A new device never wins.** What it has is merged in as new changes; it never replaces what is
in the place. Vivaldi's December 2024 outage had empty new devices overwrite good copies. And a
missing or broken sync setting means "don't sync", after Anytype uploaded local-only data from
a corrupted settings file in February 2026 ([prior-art.md](sync-research/prior-art.md#patterns-that-hold-up)).

## Removing a device

Remove… makes a new data key, wraps it to the devices that stay and the words, and every later
segment uses it ([mock](screenshots/sync/mock-s2.png)). The removed device keeps what it already
had and can read nothing new. It may still hold the place's sign-in, so the page says where to
revoke it (an app password, an S3 key) when the place has one. New recovery words work the same
way: the next key leaves the old words out.

## Merging, by kind of data

Every change carries a hybrid logical clock: wall time, a counter and the device, so changes
order the same on every device even with clocks a little apart
([merge-and-keys.md](sync-research/merge-and-keys.md#a2-crdt-per-data-type)).

| Data | How two devices' changes merge | What Go's store needs |
|---|---|---|
| **Bookmarks** | Each change is a move: create, edit, move to a folder, delete (a move to the bin). Applied in clock order; a move that would put a folder inside itself is skipped. Kleppmann's tree, written in Go: the Go libraries for it are a cgo wrapper with no release since 2024 or one-author ports | A random id, a parent id and a modified time per bookmark and folder: the onboarding session gives folders these in its slice 2 |
| **History** | Visits only add up. Clearing is a record of its own: "this site before this time", "everything since". The page chooses how far back syncs: 30 days, a year, all | Visits with their times. `core/history` keeps a count and the last visit today; the importer brings visits, so both want the change |
| **Passwords** | The newer whole login wins, and the other is kept as its previous version, as KeePass does. Field by field could pair one device's username with another's password | A random id and a modified time per login, which the daily driver's proposal for `core/passwords` now has. Passwords stay in the Keychain at rest; in the place they are sealed like everything else |
| **Open tabs** | Not merged. Each device replaces its own tabs file, and the others show it: in Standard's box ([mock](screenshots/sync/mock-s6.png)) and the phone's new tab ([mock](screenshots/sync/mock-s7.png)). Send to a device drops a file in its inbox | The session as it is |
| **Workspaces** | The newer name, colour and place win, per workspace. Their cookies and site storage never leave the device | Has ids; a modified time per workspace |
| **Settings** | The newer value wins, per key, for a list that syncs: the colour, the theme, the search engine, the keymap. The shell, the tab layout and the address bar's place stay per device | Nothing new |
| **Extensions** | The list of ids, on or off, between Macs; each Mac installs from the Web Store while its flag is on. Phones have none | The installed list |

Each store gains a change event naming what changed, so sync writes it to the log without
comparing lists. Deletes stay plain removals in the stores; sync keeps its own record of them.

**A large delete asks first.** A sync that would remove more than a fifth of the bookmarks or
passwords stops and asks, as Floccus does ([prior-art.md](sync-research/prior-art.md#floccus)).

## Keeping the folder small

Each device now and then writes a snapshot of everything it knows, with how far it has read of
each device's log, and deletes its own segments that every active device has read. A device
quiet for 90 days stops counting as active; when it returns, it starts from the newest
snapshot and adds what it changed meanwhile as new changes. Without that, one phone left in a
drawer would keep every segment forever
([merge-and-keys.md](sync-research/merge-and-keys.md#a2-crdt-per-data-type)).

## When it syncs

- **Mac:** a few seconds after a change, and on each place's own signal: the folder's file
  events, Dropbox's long-poll, Drive's changes list polled every minute or so, a nearby peer's
  push.
- **Phone:** while the app is open, and every 15 minutes or more in the background, Android's
  shortest interval for periodic work
  ([places.md](sync-research/places.md#8-device-to-device-on-the-local-network)). Nearby only
  while open.

## What the place can see

How many devices there are, when each writes, and roughly how much. Not what: no titles,
addresses, passwords, folder names, counts or dates. Firefox, Brave and Joplin all leave some of
these readable to their server; this design doesn't
([prior-art.md](sync-research/prior-art.md#patterns-that-hold-up)).

## What it takes

In Go, a `core/sync` package: the format, the keys, the merges, and one type per place. The
stores change as the table says. The engine and the shells don't change, beyond the settings
page's Sync section and the other devices' tabs in Standard's box and the Mobile new tab.
Android adds a camera view for the QR code and the folder picker. The Google and Dropbox apps
are registered once, by us.

**Testing it needs one Mac.** Two profiles pointed at the same folder are two devices, so every
merge and every join runs in the existing test groups without a phone or a cloud.

Slices, each a release of its own:

1. The format, keys, recovery words and joining, through **a folder**, for bookmarks,
   workspaces and settings. Mac only.
2. **Google Drive and Dropbox**, the phone, and **open tabs** with Send to a device.
3. **History and passwords**, once `core/passwords` and history's visits are in.
4. **WebDAV, S3, git and nearby devices**, and several places at once.
5. Extensions, and a passkey as another way in.

## For the owner

To decide, a few at a time:

1. **The rule:** each device writes only its own sealed files, so every place works and
   several at once are safe. Recommended.
2. **Which places in the first release:** a folder only (Mac), then Drive and Dropbox for the
   phone. Recommended. Or all of them from the start.
3. **Recovery words shown at setup**, with "nobody can recover without them" said plainly.
   Recommended, as no one holds a copy for the user.
4. **History:** how far back syncs by default. Recommended: a year.

Later:

5. Which settings sync, and which stay per device (the list above).
6. Send to a device, in the first phone slice or later.
7. Nearby devices as a place: our own protocol, the one piece here with nothing to reuse.

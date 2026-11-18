# Sync

A profile's bookmarks, history and typed addresses, the same on every device a person links,
through a sync server that holds them sealed. **Built in Go, 2026-11-19;** the Settings screens
and the deep link's native side come next, on the functions below. The protocol and the library
are [synckosh](https://github.com/sudiptadeb/synckosh): its SPEC.md is the protocol, and the core
uses its Go `client` package. The store it works over is [store.md](store.md).

## His decisions

- 2026-11-18: "sync should always be with a passphrase"
- 2026-11-18: "yes keychain default, passphrase lock over it... and enterprises can later force
  passphrases via policy". The keychain is built; the passphrase lock over it is not.
- 2026-11-18: "lets not go the app only level for now... since we cannot give the same level in
  windows and linux": the device key is an ordinary keychain item, not one only this app's
  signature can read.
- 2026-11-18: "profiles should be as isolated as possible... we would have to keep the dek in
  profile and kek in keyring"
- 2026-11-18: "moving profiles should work, just the passwords would not"
- 2026-11-19: "individual visits are not synced"
- 2026-11-19: "host sync server inside memd for now... users can sync by first configuring sync
  bucket url and token... then sync start asks for passphrase"
- 2026-11-19: "we should build some deep link which we can click from the memd page to set the
  sync server details on the app in mac and android"
- Agreed in the discussion of 2026-11-19: local storage is ours, and synckosh is given its storage
  interface over it; sealing on disk is ours, field by field; history and bookmarks stay plain on
  disk, sealed only for the wire, by synckosh; the sync key is apart from the profile key.

## What syncs

| Collection | One item per | What it carries |
|---|---|---|
| `bookmarks` | bookmark or folder, by its id | the folder it is in, its kind, title, address, when it was added and last changed, its place |
| `history` | address | the address, its title, how many visits, the last visit |
| `typed` | address typed | the address, how many times, the last time |

**Not synced:** each visit (the `visits` table), the sites (`sites`, made from history on each
device), the folder a bookmark was last saved in, and everything kept in JSON files (prefs,
workspaces, pins, the session, downloads).

Everything an item carries is in its private part, which synckosh seals with the sync key before
it leaves the device. Its public part is empty. The server learns the collections' names, each
item's id, when it changed and how big it is, and nothing else. A history or typed item's id is
made from its address with the sync key, so the same address on two devices is one item, and the
server can't tell from the id which address it is.

**When two devices change one item:**

- **History and typed** take the larger count and the later time, and the title of the version
  visited later (a title over none, then the later visit, then the greater title, so every device
  picks the same).
- **Bookmarks:** the newest change wins, by when it was made (the entry's `modified`).

A delete is an item too. Clearing all history is one item for each of `history` and `typed`,
synckosh's delete-before, which the server applies at once; clearing a time range deletes each
address's item.

## The keys

| Key | What it is | Where it lives |
|---|---|---|
| Device key | 32 random bytes, one for the app on this device | The Mac's login keychain, a generic password: service `Project B`, account `device key`. On Android, `files/device.key` in the app's folder, wrapped by an AES key of the Android Keystore, `project_b_device_key` |
| Profile key | 32 random bytes per profile, made the first time something is sealed | `profile.key` in the profile folder, wrapped by the device key, the profile's id bound in |
| Sync key | synckosh's data key, one per bucket, the same on every device in it | The profile's `secrets` table, sealed with the profile key. Joining devices get it from the server's sealed copy, opened with the passphrase |

The keychain and the Keystore are asked only once something is sealed in the profile, which today
is sync turned on: a profile that never syncs never touches them.

**Which key wraps a profile's key.** A profile in the profile list uses the device key. A folder
opened as it is with `--user-data-dir`, as every test and driver run is, never touches the keychain:
its profile key is wrapped by `folder.key`, a key in that same folder. So tests need no keychain,
and a listed profile's folder copied and opened with `--user-data-dir` gets nothing from the
keychain. Android's profile is the one folder Android gives the app, its only profile, and uses
the device key.

**Moving a profile.** History, bookmarks and the rest move with the folder. What was sealed does
not open on another device, whose device key differs, nor under another id: the profile key
is then *locked*.

**Never rekeyed silently.** When the device key doesn't open the profile key (a moved folder, a
lost or replaced keychain item), sealing and opening fail with a plain error, nothing is
overwritten, sync is off and its status says why. One call, `StartFresh`, deletes what was sealed
and makes a new profile key. History and bookmarks are untouched. A device key is made only when
a new profile key needs one, never in place of one that is there.

**Sealing on disk** (`core/keys`): XChaCha20-Poly1305, with the table, the row's id, the column
and any other values the row binds in as additional data, so a sealed value can't be moved to
another row or column. A fresh random nonce each time; the stored value is the nonce, then the
sealed bytes. The `secrets` table holds named sealed values; sync keeps its address, token and
sync key there.

## The address

A bucket on the memd server is reached two ways, and both are taken as they are:

- `https://<host>/sync`, with the token sent as `Authorization: Bearer <token>`;
- `https://<host>/sync/<token>`, the one address, with no token apart.

Only `https`, with a host, no user, query or fragment. An address whose path ends in `/sync` needs
its token given; one that goes on past it carries its token. Status shows the host alone.

**The deep link** the memd page offers is

```
project-b://sync?address=<the https address, percent-encoded>
```

It only fills in what Settings would connect to: it never connects by itself. Settings shows the
host and asks for the passphrase (and the token, for an address without one), and only then
connects.

## Turning it on

1. The address, from the deep link or typed: `Propose` checks it.
2. `Inspect` looks at the bucket without changing it: an empty bucket is a new one, so the page
   asks for a new passphrase twice; one holding items asks for its passphrase once.
3. `Connect` makes the bucket's sync key with synckosh's `Create` when it is empty, with a sealed
   copy under the passphrase, or opens the sealed copy with `Join` when it holds items. A wrong
   passphrase changes nothing. It keeps the address, the token and the sync key sealed, hands every
   row of the three collections to synckosh once, and syncs.

The passphrase is never kept: the device holds the sync key, so changing the passphrase needs no
old one.

## How it runs (`core/sync`)

**Local storage is ours.** `core/sync` is synckosh's `client.Local` over `store.db`:

| Table | What it holds |
|---|---|
| `sync_items` | synckosh's bookkeeping per item: its sequence, base, timestamp, pending, sent, and the row it is (the feature's key, the address or the bookmark's id). An item's content is the feature's own row, never a second copy. Only what has no row is kept whole: a deleted item, without its content; one sealed under a key this device doesn't hold, as it came; the sealed copies of the sync key; a delete-before |
| `sync_meta` | synckosh's values: its clock, cursor, store id and last pull |
| `sync_changes` | the rows changed here since the last run: collection, the row's key, deleted |
| `sync_state` | one row: whether changes are marked, which is whether sync is on |

Each synced feature gives a mapping (`sync.Mapping`): its collection and datatype, its table and
the column that names a row, the row as an item's private part, and the private part back as the
row; deleting a row; and `Join`, the row after taking in another version, which never loses what
the row holds that is newer. History's and typed's `Join` is their merger too.

**Writing first, then telling sync.** Features write their rows as before. While sync is on,
triggers on their tables write a mark in `sync_changes`, in the same transaction; with it off,
none. Clearing all history replaces its marks with one, which becomes the delete-before. A run:

1. Drains the marks into synckosh: a delete-before first, then a changed row as `Replica.Put`, a
   deleted one as `Replica.Delete`, which finds the item by its bookkeeping though the row is gone.
2. `Replica.Sync`: pull, push.
3. A version from the server is joined into its row (`Join`), on the store's worker, with the
   triggers quiet. When the row then differs from it, the row holds something newer, and is
   marked to go out. A row with a delete waiting is left alone. A deleted item takes its row. The
   features that changed are told on the main thread: the bookmarks read their tree again, and
   tell the shells.

synckosh keeps deleted items (`KeepDeleted`), so it never removes an entry for a delete, and a
removal (a device away past the server's cleanup starts over) only forgets the bookkeeping: the
rows stay, and go out again.

**When it runs:** at start when sync is on, within ten seconds of a change (it looks for marks
that often), every five minutes, and on `SyncNow`. On a goroutine of its own, never the main
thread; one run at a time. A failed run waits 30 seconds, then twice as long each time, up to half
an hour, and its error is in the status. `SyncNow` doesn't wait.

**A damaged store.db**, which the store moves aside and makes again ([store.md](store.md)), takes
the secrets and the bookkeeping with it: sync is off, and is connected again with the passphrase.

**A server reset** (the server gives the bucket a new store id and forgets its items): synckosh
pushes back everything this device holds, read from the rows, deletes included.

## The functions (`core/sync`)

Every answer comes on the main thread, where the calls are made. Only `Propose`, `Link`, `Status`
and `OnStatus` answer at once; the rest run on sync's goroutine, after what was asked before them.

| Function | What it does |
|---|---|
| `Link(link) (address, error)` | the address in a `project-b://sync?address=` link |
| `Propose(address) (Proposal, error)` | checks an address and says what would be connected: `Address`, `Host`, `NeedsToken`. Changes nothing |
| `Inspect(address, token, answer func(isNew bool, err error))` | whether the bucket is empty, so new. Changes nothing |
| `Connect(address, token, passphrase, done func(error))` | starts the bucket or joins it, keeps its details sealed, and syncs. `errors.Is(err, sync.ErrPassphrase)` for a wrong passphrase |
| `ChangePassphrase(passphrase, done func(error))` | puts a sealed copy under the new passphrase and deletes the others |
| `Disconnect(done func(error))` | stops syncing and forgets the address, token and sync key; every row stays |
| `SyncNow()` | a run now |
| `StartFresh(done func(error))` | for a locked profile key: `keys.StartFresh`, so sync is forgotten and off |
| `Status() Status`, `OnStatus(func(Status))` | `on`, `syncing`, `host`, `lastSynced` (Unix ms, 0 for never), `error`, `locked` |

**The settings page's messages** (`core/app/sync_settings.go`). Go sends `{"t": "sync", "sync":
<Status>}` to every settings page and shell when it changes, and both snapshots carry `sync`. The
page sends:

| Message | Go answers |
|---|---|
| `syncPropose {address}`, or `{link}` for a deep link's | `syncProposal {address, host, needsToken}` or `{error}` |
| `syncInspect {address, token}` | `syncBucket {new}` or `{error}` |
| `syncConnect {address, token, passphrase}` | `syncConnected {wrongPassphrase}`, with `error` when it failed |
| `syncChangePassphrase {passphrase}` | `syncPassphraseChanged {}` or `{error}` |
| `syncDisconnect` | `syncDisconnected {}` or `{error}` |
| `syncNow` | the status, as it changes |
| `syncStartFresh` | `syncStartedFresh {}` or `{error}` |

## Not built

- The passphrase lock over the keychain, and a policy forcing it.
- Saved logins, and any sealed field but the secrets.
- A token per device: every device uses the bucket's token.
- A backup code (synckosh's `BackupCode`).
- The Settings screens and the deep link's native side.

## Tests

`core/keys`: sealing binds its table, row, column and bound values; a folder named outright uses
its own key and never a device key; a profile key that doesn't open is locked, overwrites nothing,
and `StartFresh` makes a new one; the Mac's keychain, under a test-only service, deleted after.
`core/history`: joining two versions of a page, whichever is the row.
`core/sync`: two profiles syncing bookmarks, history and typed addresses through synckosh's server
in-process, in both address forms; one address visited on both, one row with the larger count; a
bookmark deleted on one gone from the other; clearing history reaching the other; no visit and no
readable title or address on the server; a wrong passphrase; joining; a changed passphrase, which a
third profile joins with while the old one fails; Disconnect keeping the rows; a server reset
refilled; a locked profile key turning sync off; sync on at start running with what was kept.
`core/app`: the settings page's status, a deep link only proposed. `core/cmd/lib`: a folder named
outright gets no device key.

**Not tested here:** the Android Keystore piece (`DeviceKey.java`, `go_table.c`) compiles against
the SDK and the NDK but has not run on a phone; the Mac's keychain test skips where the shell's
keychain list has no login keychain, as an agent's shell over ssh has.

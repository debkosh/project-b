# Sync model: a draft for discussion

**Where it went, 2026-11-17.** The owner chose a small sync server over files in a place the user
owns: "type buckets with items inside... latest item wins... custom mergers for specific data
types... default merger takes newest item", with the spec and a reference Go library in a public
repo that other projects can use. That is [synckosh](https://github.com/sudiptadeb/synckosh):
its [SPEC.md](https://github.com/sudiptadeb/synckosh/blob/main/SPEC.md) is the protocol, and
Project B will use its Go package. What it takes from this draft: the item shape, the
passphrase-sealed key as in keykosh, the timestamp rules, and order kept on the child. His later
points are in it too: guids are UUIDv7, so they sort by when they were made, and order keys may
repeat, with a list sorted by order key and then guid. The rest of this page, the files in a
place, is the earlier option.

**The protocol as built** is synckosh's SPEC.md, not this page. Its words differ from the ones
below: a bucket is one synced space, a collection is a kind of data in it, a datatype is what an
item is, and the timestamp is a plain number of milliseconds. Read this page only as the history of
how the design started.

A draft for the owner, from three asks of his: one item shape, with a guid, a type and a
timestamp, and the type owning parent and child; a public and a private part, sealed with a
passphrase as keykosh does; and no server, kept fast, with import and export easy. Nothing here
is built. The merge research behind it is [sync-research/merging.md](sync-research/merging.md).
## The mental model

Everything is an item. An item is `{guid, type, ts, deleted, data}`. For each guid, the item with
the newest `ts` wins. Sync never reads `data`; the type does.

Syncing, backing up, exporting, importing and passing data between nearby devices are one thing:
**exchanging sealed files of items, and merging them by that rule.** A sync place is a folder of
such files. An export is one such file. An import from Chrome writes items the same way.

## Two halves, as in keykosh

keykosh keeps a public key in plain view (`.kosh/recipient`) and the private key sealed with the
passphrase (`.kosh/identity.age`, with age's scrypt passphrase recipient). Clone the folder, type
the passphrase, and everything opens ([keykosh](https://github.com/sudiptadeb/keykosh),
`kosh.go`: `writeIdentity`, `identity`).

Sync does the same:

- **The public half:** what anyone who can see the folder sees. The format version, the public
  key, and a header on each file: which device wrote it, its number, and for each item a hashed
  guid, the timestamp and the deleted flag.
- **The private half:** the item's type and data, and the private key itself, sealed with the
  passphrase.

Joining a device: point it at the place and type the passphrase. The device then keeps the
private key in the Keychain or the Android Keystore, so the passphrase is typed once per device.
A QR code from a device already in can carry the passphrase, so nothing is typed.

### The place

```
Project B Sync/
  sync.json               public: format, public keys, scrypt settings
  identity.age            private keys, sealed with the passphrase
  log/mac1/000042.pbs     a batch of edits from the Mac
  log/pixel9/000107.pbs   a batch from the phone
  snap/mac1/000040.pbs    everything the Mac has written, folded into one file
```

Each device writes only its own folder under `log/` and `snap/`, as in [sync.md](sync.md).

`sync.json`:

```json
{
  "format": 1,
  "recipient": "age1qx3k...",
  "verify": "ed25519:Hk2p...",
  "created": "2026-11-17"
}
```

`identity.age`, once the passphrase opens it:

```json
{
  "seal": "AGE-SECRET-KEY-1...",
  "sign": "ed25519-private:...",
  "hash": "base64 key for hashing guids and history ids"
}
```

### One file of items, `.pbs`

```
PBS1
{"device":"mac1","n":42,"items":[
  {"id":"hK3v...","ts":[1763376127123,0,"mac1"],"deleted":false},
  {"id":"Qm9a...","ts":[1763376127123,1,"mac1"],"deleted":true}]}
<age-sealed body>
<signature over everything above>
```

- The first two lines are the public half. `id` is a keyed hash of the guid, so the folder shows
  neither guids nor setting names, nor which pages were visited.
- The body is sealed to `recipient` with age. Opened, it is the full items:

```json
[
  {"guid":"bm_7Hq2...","type":"bookmark","ts":[1763376127123,0,"mac1"],"deleted":false,
   "data":{"title":"Go documentation","url":"https://go.dev/doc/","parent":"root:toolbar","pos":"a1V~mac1"}},
  {"guid":"lg_K9w...","type":"login","ts":[1763376127123,1,"mac1"],"deleted":true,"data":null}
]
```

- The signature uses the private `sign` key, and any device checks it with `verify`. Sealing
  alone only needs the public key, which is in plain view, so without the signature anyone who
  can write to the folder could slip items in.

What the storage provider learns from the public half: how many items each device changed, and
when, and how many were deletes. Not what they are. That is the price of a public half, and it
buys the three things below.

**What the public half is for:**
1. **Merging before decrypting.** A device reads the headers, works out which items it lacks or
   which are newer, and opens only those bodies.
2. **Cleaning up without keys.** A helper with no passphrase, such as a NAS or an old phone left
   on, can see which files are fully replaced by newer ones and delete them.
3. **Relaying without keys.** A nearby device or a helper can pass files on without being able to
   read them.

## The item, and each type's data

The timestamp is `[wall clock ms, counter, device]`. A device never writes one lower than any it
has read, and the device breaks ties
([merging.md, rule 1](sync-research/merging.md#guidelines-we-can-adopt)).

A new item gets a new random guid. An existing one keeps the guid it synced with. Things that are
"the same on every device" get fixed guids.

| Type | Guid | `data` |
|---|---|---|
| `bookmark` | random, `bm_...` | `{"title","url","parent","pos","added"}` |
| `folder` | random, `fd_...`; the roots fixed: `root:toolbar`, `root:other`, `root:mobile` | `{"title","parent","pos"}` |
| `visit` | keyed hash of URL, time and device | `{"url","title","at","device","from"}` |
| `login` | random, `lg_...`; matched on origin and username when merging | `{"origin","username","password","note","previous":[{"password","until"}]}` |
| `setting` | fixed, `setting:theme` | `{"value":"dark"}` |
| `workspace` | random, `ws_...` | `{"name","color"}` |
| `tabs` | fixed per device, `tabs:mac1` | `{"name":"MacBook Pro","windows":[{"tabs":[{"url","title","at"}]}]}` |

Parent and position live on the child, never as a list in the folder, so two devices adding to
one folder both survive. `pos` is a fractional key with the device on the end. An item whose
parent is gone, or that ended up inside itself, shows in one fixed folder. That is the bookmark
type's job, not sync's.

If the owner chooses per-field merging for bookmarks and logins, only `data` changes; sync's rule
doesn't:

```json
{"title":{"v":"Go docs","ts":[1763376127123,0,"mac1"]},
 "url":{"v":"https://go.dev/doc/","ts":[1763370000000,0,"pixel9"]},
 "place":{"parent":"root:toolbar","pos":"a1V~mac1","ts":[1763376127123,0,"mac1"]}}
```

## No server, and fast

The place is only storage; the devices do all the work.

- **Local first.** Each device keeps the merged items in a local database and reads from that.
  Opening the browser or a bookmark never waits on the place.
- **Batches, not items.** Edits within a few seconds go out as one `.pbs` file.
- **Read only what's new.** A device remembers the last file number it read from each other
  device, lists the folders, and fetches only the newer files. The work follows what changed,
  not how much there is.
- **Snapshots.** Now and then a device folds its own files into one snapshot of its live items
  and deletes the older files. A new device reads one snapshot per device, then the newer batches.
- **History by month.** Visits go in monthly snapshots, so a phone can fetch only the last few
  months.
- **Being told, not polling.** On the Mac a watched folder, Drive's changes feed or Dropbox's
  long-poll; on the phone, on open and every 15 minutes or more in the background.
- **Nearby devices** send each other the same `.pbs` files over the LAN at once, so the Mac and
  the phone see each other's edits in a second, and the place catches up later. Same files, a
  different path.
- **A file by hand.** AirDrop, email or a USB stick: export writes one `.pbs` snapshot, and
  importing it on another device merges it by the same rule. A device with no place at all can
  still be kept in step by hand.

## SQLite: yes on the device, no as the synced file

**On each device, yes.** Sync needs a local store of the merged items, the last file read from
each device, the clock, and each item's last synced copy. SQLite does that well, and history needs
it anyway at a few hundred thousand visits. The Go core has no SQLite today, and whether a
pure-Go SQLite works in the Android build still has to be checked.

**As the synced file, no.**
- One database file in Dropbox or iCloud written by two devices becomes a conflicted copy, and
  merging those back is the problem we are solving.
- Every change uploads the whole file.
- SQLite warns that copying a database in the middle of a transaction, or without its `-wal` or
  `-journal` file, can corrupt it ([SQLite, How to corrupt, 1.2 and
  1.4](https://www.sqlite.org/howtocorrupt.html)). A provider's sync app does just that.
- SQLite's session extension records changes as changesets, but applying one assumes the target
  holds the old values; a conflict means skipping, aborting or overwriting
  ([SQLite sessions](https://www.sqlite.org/sessionintro.html)). That is not newest-wins per guid.
  It also ties the format to our table layout.

**For export, both.** The `.pbs` snapshot is the sealed export and import. For someone who wants
their data in a standard file, a plain SQLite export is easy: `VACUUM INTO` writes a consistent
copy even while the database is in use ([SQLite, How to corrupt,
1.2](https://www.sqlite.org/howtocorrupt.html)).

## The passphrase

The private key sits in the folder sealed only by the passphrase, so anyone who copies the folder
can try passphrases offline, as long as they like. age's scrypt makes each guess slow, but a
weak passphrase still falls. Two ways to handle that:

- **Suggest a generated passphrase:** the recovery words from [sync.md](sync.md), typed once per
  device or carried by the QR code. The user may choose their own, with a strength check.
- **Add a second secret,** as 1Password does with its Secret Key: a long code saved on each
  device and printed in the recovery kit
  ([1Password security design](https://1passwordstatic.com/files/security/1password-white-paper.pdf)).
  Stronger, but one more thing to keep.

The first keeps the keykosh model as it is.

## Removing a device

A removed device still has the private key. So removing makes a new key: a device still in writes
a new `identity.age` and `sync.json`, and new snapshots sealed to the new key. The other devices
pick up the new key with the passphrase they already hold, or the owner changes the passphrase
too. The removed device can still read what it had, but nothing new.

## Compared with sync.md

This replaces sync.md's per-device key wraps and epochs with one key pair behind a passphrase,
as keykosh does. Joining is "pick the place, type the passphrase". It replaces the per-type merge
table with one rule, plus each type's own handling inside `data`. Places, own-files-only, the
recovery words and the slices stay.

## For the owner

1. **The public half:** hashed guid, timestamp and deleted flag in plain view, for merging before
   decrypting and for keyless cleanup and relays. Or everything sealed, and the provider sees
   only file sizes and times.
2. **The passphrase:** suggested generated words, or a second secret like 1Password's.
3. **Bookmarks and logins:** newest wins for the whole item, or per field inside `data`.
4. **SQLite on the device:** for sync's state and history, if pure-Go SQLite works on Android.

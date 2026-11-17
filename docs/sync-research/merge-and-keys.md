# Sync over dumb storage: merging and user-held keys

Research notes, 2026-11-17. Every claim has a source link. **(unverified)** marks claims that rest only on a secondary or vendor source, or on my own inference. **(design note)** marks my recommendations, which are reasoning, not sourced fact.

---

## A. Merging edits from several devices over dumb storage

### A1. "Each device appends only to its own log" (single-writer logs, no shared mutable file)

The pattern: every device owns one folder or prefix (`devices/<device-id>/…`) and only ever writes new, immutable, content-named files there. Readers list every device's folder and merge. Writes never contend, so you need no conditional writes and no locks, and a synced folder never makes a conflict copy, because no two devices write the same path.

Who does this, or something close to it:

- **Automerge storage model.** Chunks are keyed `[documentId, chunkType, chunkId]`. An incremental chunk is named by the hash of its bytes, and a snapshot by the document heads. Each process deletes only the incremental chunks it has itself loaded into a snapshot. The design exists to avoid needing transactions, which the docs call "very fiddly" on a plain directory ([Automerge storage docs](https://automerge.org/docs/reference/under-the-hood/storage/)). `saveIncremental` output can be appended to a save file or fed to another replica ([saveIncremental API](https://automerge.org/automerge/api-docs/js/functions/saveIncremental.html)).
- **Hypercore / Autobase (Holepunch).** Each writer has its own append-only Hypercore. Autobase linearises the writers' logs into one deterministic view using causal (vector-clock) metadata, and can reorder entries as new causal information arrives ([Autobase docs](https://docs.pears.com/building-blocks/autobase), [alpha README](https://socket.dev/npm/package/autobase/overview/1.0.0-alpha.2)). Trust and authorisation of writers sit outside Autobase ([Autobase docs](https://docs.pears.com/building-blocks/autobase)).
- **Ink & Switch, "Local-first software" (April 2019, Onward! 2019).** It sets out seven ideals, including "The network is optional", "The Long Now", "Security and privacy by default" and "You retain ultimate ownership and control". It names a "sneakernet" approach of copying files around with Dropbox and USB keys among those tried, and argues that CRDTs can sync over any channel ([essay](https://www.inkandswitch.com/essay/local-first/)).
- **Actual Budget ("CRDTs for mortals").** It keeps one table of messages `(timestamp, dataset, row, column, value)`. Each message sets one cell, and the newest timestamp wins, so this is LWW per cell under a hybrid logical clock. A Merkle trie of message timestamps finds where two clients diverge. The system rejects clock drift beyond about one minute ([James Long, "Using CRDTs in the wild"](https://jlongster.com/using-crdts-in-the-wild)). It runs through a relay server, not dumb storage, but the log-plus-trie idea carries over.
- **Evolu.** Local SQLite. The relay is "generic, stateless" and replaceable, and it sees only OwnerId and Timestamp, with everything else encrypted ([Evolu relay docs](https://evolu.vercel.app/docs/evolu-relay)). Owner keys come from an OwnerSecret via SLIP-21, and restore uses a BIP39-style mnemonic ([Evolu Owner API](https://evolu.vercel.app/docs/api-reference/common/local-first/interfaces/Owner), [Evolu overview](https://evolu.vercel.app/docs/overview)). It needs a relay; it is not built for dumb storage.
- **cr-sqlite (vlcn).** A SQLite extension with per-column LWW, fractional index and observe-remove sets; counters and rich text were still in progress. MIT licence ([repo](https://github.com/vlcn-io/cr-sqlite)). The last default-branch push was about 2024-10-25 per a third-party index, so treat it as stalled **(unverified)** ([ecosyste.ms](https://awesome.ecosyste.ms/projects/github.com%2Fvlcn-io%2Fcr-sqlite)). It is C/Rust with no Go binding listed ([repo](https://github.com/vlcn-io/cr-sqlite)).
- **Joplin (one file per item).** It is not per-device-log: every note is one file on the sync target, so two devices write the same file. A conflict happens "when one note or one attachment is modified in two different places" ([Joplin conflict help](https://joplinapp.org/help/apps/conflict)). It relies on `updated_time` inside the item, because non-Joplin targets report unreliable timestamps ([Joplin file-api.ts](https://cdn.jsdelivr.net/npm/@joplin/lib@3.5.1/file-api.ts)). Encrypted items carry a plaintext header with the method and master-key ID ([Joplin E2EE spec](https://joplinapp.org/help/dev/spec/e2ee/)). This is the cautionary example: item-per-file gives you per-item conflicts and file names that leak item counts.
- **KeePass / KeePassXC merge.** Entries are matched by UUID. The newer modification time wins, and the loser goes into the entry's history instead of being lost ([KeePassXC Merger.cpp](https://git.hackliberty.org/Git-Mirrors/keepassxc/src/commit/79ca00604aab9721a741c848940998b3cdf36708/src/core/Merger.cpp)). The merge is entry-level, not field-level **(unverified, secondary)** ([panicvault](https://www.panicvault.org/cloud-sync/sync-conflicts/)). This is whole-file state-based merge of a single database file, which works over a synced folder because the merge reconciles conflict copies.
- **Replicache and Electric (not a fit).** Replicache is free and open-sourced, but in maintenance mode, with work moved to Zero (Apache-2) ([Zero open source](https://zero.rocicorp.dev/docs/open-source), [Replicache source access](https://doc.replicache.dev/howto/source-access)). Electric was rebuilt in July 2024 as read-path sync of Postgres "shapes", with writes going through your own API ([Electric Next post](https://electric-sql.com/blog/2024/07/17/electric-next), [shapes guide](https://electric-sql.com/docs/guides/shapes)). Both assume a server that you run, so neither fits dumb storage.

**What the storage backends add.** S3 supports `If-None-Match: *` (create only if absent) and `If-Match: <ETag>` (compare-and-swap) on PutObject. The If-Match addition was announced 2024-11-25 ([AWS what's new](https://aws.amazon.com/about-aws/whats-new/2024/11/amazon-s3-functionality-conditional-writes), [S3 conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html)). The Dropbox API has `WriteMode.update(rev)`, which fails or autorenames to "conflicted copy" on a rev mismatch ([dropbox-sdk WriteMode](https://docs.rs/dropbox-sdk/0.14.1/dropbox_sdk/files/enum.WriteMode.html)).

**(design note)** Use the per-device-log pattern as the baseline, because it needs only put/get/list. Treat conditional writes as an optimisation only, for example to make a compaction marker atomic on S3. A synced folder that makes conflict copies is then harmless, since no two devices ever write the same name.

### A2. CRDT per data type

| Data | Suggested CRDT | Notes and sources |
|---|---|---|
| **Bookmarks tree with moves** | Kleppmann et al. move-op tree CRDT, with a list or fractional-index CRDT for sibling order | See below. |
| **History visits** | Grow-only set of `(url-id, visit-time, transition, device)` | Firefox Sync stores a per-URL record with a `visits` array of `{date, type}` ([object formats](https://mozilla-services-docs.readthedocs.io/en/latest/sync/objectformats.html)). That design deduped only the most recent 20 visits (fixed in Fx 59, [bug 1423399](https://bugzilla.mozilla.org/show_bug.cgi?id=1423399)), and it cannot delete individual visits, because an absent visit doesn't imply it never existed ([bug 1164660](https://bugzilla.mozilla.org/show_bug.cgi?id=1164660)). **(design note)** For "clear history", add a deletion-range record ("visits to URL X before time T removed") rather than per-visit tombstones. |
| **Passwords** | Whole-record LWW register (by HLC), keeping the losing version in per-record history | This mirrors KeePassXC, where the newer entry wins and the older goes to history ([Merger.cpp](https://git.hackliberty.org/Git-Mirrors/keepassxc/src/commit/79ca00604aab9721a741c848940998b3cdf36708/src/core/Merger.cpp)). **(design note)** Per-field LWW can splice one device's username onto another's password. Use field-level LWW only for independent fields such as notes. |
| **Open tabs** | Per-device snapshot, no merge | Firefox: each client writes one tabs record, and tabs "are not merged like other data" ([syncstorage data types](https://mozilla-services.github.io/syncstorage-rs/data-types.html), [object formats](https://mozilla-services-docs.readthedocs.io/en/latest/sync/objectformats.html)). This is exactly the own-folder pattern: `devices/<id>/tabs` is overwritten by its owner only. |
| **Prefs** | LWW map per key (HLC timestamp plus device-id tiebreak) | The same idea as cr-sqlite per-column LWW ([cr-sqlite](https://github.com/vlcn-io/cr-sqlite)) and Actual's per-cell LWW ([jlongster](https://jlongster.com/using-crdts-in-the-wild)). |

**Move-op tree CRDT** (Kleppmann, Mulligan, Gomes, Beresford, IEEE TPDS 33(7), 2021/22, [paper PDF](https://martin.kleppmann.com/papers/move-op.pdf), [overview](https://martin.kleppmann.com/2021/10/07/crdt-tree-move-operation.html)):

- Every operation is a `Move(t, parent, meta, child)`. Create is a first move, and delete is a move to a trash node.
- To apply an op with timestamp t, a replica undoes all logged ops with a timestamp above t, does the op, then redoes them. The tree ends up as if every op had been applied in timestamp order.
- A move that would create a cycle (the child is an ancestor of the new parent) is skipped ([paper §3](https://martin.kleppmann.com/papers/move-op.pdf)).
- Creation ops can skip the undo/redo and the log, under stated assumptions. Deletion ops cannot ([paper §3.6](https://martin.kleppmann.com/papers/move-op.pdf)).
- **Log truncation:** ops with timestamp ≤ t can be discarded once t is causally stable. Trashed nodes can be garbage-collected once their trash-move is stable. Stability needs a known replica set, monotonic per-replica timestamps and FIFO delivery; the threshold is the minimum of the latest timestamp seen from each replica ([paper §3.7](https://martin.kleppmann.com/papers/move-op.pdf)).
- **Numbers:** a local op takes about 1–2 µs in handwritten code. Remote ops saturate at 5,700 ops/s, about 200 undo/redos per op at peak, across California, Ireland and Singapore. A leader-based state machine reached 22,000 ops/s but needs a 145–176 ms round trip per op ([paper §5](https://martin.kleppmann.com/papers/move-op.pdf)). Bookmark edit rates are far below either.
- **Sibling order:** keep a list CRDT (RGA or Logoot) per branch node ([paper §3.7](https://martin.kleppmann.com/papers/move-op.pdf)). Loro implements this movable tree with fractional indexes ([Loro movable-tree blog](https://loro.dev/blog/movable-tree)).
- A follow-up paper claims a 68.19× speedup for remote moves by using one compensation op instead of undo/redo ([Anjana et al., arXiv 2203.10285](https://arxiv.org/abs/2203.10285)).

**Hybrid logical clocks.** Kulkarni et al., "Logical Physical Clocks", OPODIS 2014. An HLC tracks causality like a Lamport clock while staying close to NTP time, and fits a 64-bit NTP-format timestamp ([paper PDF](https://www.cse.buffalo.edu//~demirbas/publications/hlc.pdf)). In practice a timestamp is `(wall ms, counter, node-id)`, and Actual rejects drift beyond about one minute ([jlongster](https://jlongster.com/using-crdts-in-the-wild)). **(design note)** HLC gives a total order for LWW and the move-op log, and makes "now" meaningful for TTLs and compaction.

**Tombstones, compaction and devices offline for months:**

- **Automerge** keeps full history by design. Version 3.0 (July 2025) cut memory by over 10× but still does not discard history. Its Moby Dick example dropped from 700 MB to 1.3 MB ([Automerge 3.0](https://automerge.org/blog/automerge-3/)).
- **Yjs** GC drops the content of deleted items but always keeps their metadata ([Yjs forum](https://discuss.yjs.dev/t/should-size-of-binary-ydoc-be-monotonically-increasing/2325), [Yjs forum 2](https://discuss.yjs.dev/t/clear-document-history-and-reject-old-updates/945)).
- **Loro** has shallow snapshots, like a git shallow clone. Peers can sync only if they hold versions after the shallow point ([Loro shallow snapshots](https://www.loro.dev/docs/advanced/shallow_snapshot)).
- **(design note)** The move-op stability rule ([paper §3.7](https://martin.kleppmann.com/papers/move-op.pdf)) shows the core problem: one device offline for months pins the stability threshold, so nothing can be truncated. The usual way out:
  1. Each device periodically writes a full **snapshot** plus its "seen up to" vector into its own folder.
  2. Any device may delete *its own* log segments that every *active* device has acknowledged.
  3. A device silent for N days is marked **stale**. When it returns, it must load the latest snapshot and rebase its unsynced local ops as new ops with fresh HLC timestamps, instead of replaying them under their old timestamps.
  4. Removing a device from the device list (see B2) also removes it from the stability computation.

### A3. Go libraries (maturity and licence)

| Library | Kind | Licence | Status (as found) |
|---|---|---|---|
| [automerge/automerge-go](https://github.com/automerge/automerge-go) | cgo wrapper around automerge-rs / automerge-c | MIT ([repo](https://github.com/automerge/automerge-go)) | Only a pseudo-version, **v0.0.0-20241030…**, dated 2024-10-30, and no tagged releases ([pkg.go.dev versions](https://pkg.go.dev/github.com/automerge/automerge-go?tab=versions)). Rebuilding deps "must be done on an Apple Silicon mac with docker" ([README](https://raw.githubusercontent.com/automerge/automerge-go/main/README.md)). Android targets are not mentioned; you would have to cross-build the Rust static libs yourself **(unverified)**. The wrapper looks stale against Automerge 3.0 (July 2025). |
| [aholstenson/loro-go](https://github.com/aholstenson/loro-go) | cgo/FFI wrapper of Loro (listed by loro-ffi) | MIT | Pre-1.0, "API may change". Tracks Loro 1.13.6. Prebuilt for macOS, Linux and Windows; **no Android** ([repo](https://github.com/aholstenson/loro-go), [loro-ffi](https://docs.rs/crate/loro-ffi/latest/source/README.md)). |
| [Deln0r/loro-go](https://pkg.go.dev/github.com/Deln0r/loro-go) | **Pure Go**, byte-compatible with the loro-crdt 1.17.1 wire format, no cgo | MIT | v1.0.0 published 2026-10-28. Map, List, Text, MovableList, **Tree** and Counter. No shallow snapshots, slow on long concurrent branches, and "not affiliated with loro-dev" ([pkg.go.dev](https://pkg.go.dev/github.com/Deln0r/loro-go)). A single author with a very new project, so maturity is **unverified**. |
| [Deln0r/ygo](https://github.com/Deln0r/ygo) | **Pure Go** Yjs port, gomobile bindings for iOS/Android | MIT | "Stable CRDT core, with known gaps". Targets yjs 13.6.33 with 319 cross-language fixtures ([repo](https://github.com/Deln0r/ygo)). Same single-author caveat. |
| Loro itself (Rust) | | MIT ([loro repo](https://github.com/loro-dev/loro), [crate](https://docs.rs/crate/loro/1.13.6/source/Cargo.toml)) | 1.0 released 2024-10-23 ([Loro 1.0](https://loro.dev/blog/v1.0)). |
| [vlcn-io/cr-sqlite](https://github.com/vlcn-io/cr-sqlite) | SQLite extension | MIT | Appears stalled since late 2024 **(unverified)**. No Go binding. |

**(design note)** The data here is small and record-shaped (bookmarks, visits, credentials, prefs), not collaborative text. LWW maps, a G-set and the move-op tree are each a few hundred lines of pure Go, which avoids cgo and Rust cross-builds on Android. Use Loro (pure-Go port) only if you want its tree and movable-list for free, and accept a young dependency.

### A4. How Firefox and Chromium merge bookmarks

**Firefox: dogear (Rust).** Despite the usual description, it is a **two-way** merge of a local tree against a remote tree, with no stored shared parent. Each item's `needs_merge` flag is the only change signal, so the merger knows *that* an item changed but not *how* ([dogear merging docs](https://mozilla.github.io/dogear/merging.html)).
- It walks both trees from the roots and gives every item one of seven merge states (local, remote, or "with new structure", which means apply and reupload).
- When both sides changed an item, the chronologically newer side wins. Conflicts are resolved per item, not per field, and the docs admit this is a form of data loss.
- Change versus delete: a non-folder item is revived. A folder stays deleted, and its new children move to the nearest surviving ancestor. Roots are always revived.
- New remote items are deduped against unsynced local items with similar content.

All of this is from the [dogear merging docs](https://mozilla.github.io/dogear/merging.html).

**Chromium: server-ordered with UniquePosition.**
- Each bookmark has a GUID, a parent GUID and a `unique_position`, an ordinal string with a client-tag suffix for a stable total order ([Unique Position design doc](https://chromium.org/developers/design-documents/sync/unique-positions), [parent_guid_preprocessing.cc](https://chromium.googlesource.com/chromium/src/+/913eaefa55beab765aa1d99da170db474d8c63ca/components/sync_bookmarks/parent_guid_preprocessing.cc)).
- On a conflict between a pending local change and a remote update, **the server wins** in general. The exceptions are a remote-only delete (local wins and re-uploads), a local-only delete (the item is restored from the server), and a missing parent (local wins) ([bookmark_remote_updates_handler.cc, 145.0.7632.45](https://chromium.googlesource.com/chromium/src/+/refs/tags/145.0.7632.45/components/sync_bookmarks/bookmark_remote_updates_handler.cc)).
- The initial merge matches by GUID, then by title and URL ([bookmark_model_merger.h](https://chromium.googlesource.com/chromium/src/+/refs/tags/145.0.7632.45/components/sync_bookmarks/bookmark_model_merger.h)).
- **(design note)** "Server wins" needs an authoritative server, which dumb storage does not have. That is why a CRDT (or dogear-style newest-wins plus revive rules) fits better here.

---

## B. Keys and encryption the user holds

### B1. File and record encryption choices

**age (filippo.io/age):**
- **Format** ([C2SP age spec](https://www.github.com/C2SP/C2SP/blob/main/age.md)):
  - A random 16-byte file key is wrapped once per recipient in a "stanza".
  - The header is MACed with HMAC-SHA-256 under a key derived from the file key.
  - The payload is STREAM ChaCha20-Poly1305 in 64 KiB chunks.
  - An X25519 stanza is a 32-byte ephemeral share plus a 32-byte wrapped key, about 100 bytes of header per recipient.
- **Post-quantum status:**
  - age **v1.3.0** (27 Dec, year inferred as 2025) added native hybrid PQ recipients: HPKE with **MLKEM768-X25519 (X-Wing)**, `age1pq1…` recipients and `AGE-SECRET-KEY-PQ-1…` identities, generated with `age-keygen -pq` ([release notes](https://github.com/FiloSottile/age/releases)). v1.3.2 was published 2026-08-29 ([pkg.go.dev](https://pkg.go.dev/filippo.io/age)).
  - The PQ stanza's encapsulated key is **1120 bytes** ([C2SP spec](https://www.github.com/C2SP/C2SP/blob/main/age.md)), and the recipient strings are "~2000 characters long" ([pkg.go.dev README](https://pkg.go.dev/filippo.io/age)).
  - A `HybridRecipient` carries a "postquantum" label and **cannot be mixed** with X25519 recipients in one file ([pkg.go.dev](https://pkg.go.dev/filippo.io/age)).
  - v1.3.0 also added `age1tag`/`age1tagpq` P-256 recipients for hardware plugins (Secure Enclave, YubiKey, TPM). Their 4-byte tag reveals which recipient a stanza targets ([C2SP spec](https://www.github.com/C2SP/C2SP/blob/main/age.md), [release notes](https://github.com/FiloSottile/age/releases)).
  - A scrypt (passphrase) recipient must be the only recipient of a file ([pkg.go.dev](https://pkg.go.dev/filippo.io/age)).
- **Go standard library:** Go 1.26 added `crypto/hpke`, including MLKEM768X25519 ([pkg.go.dev crypto/hpke](https://pkg.go.dev/crypto/hpke@go1.26.6), [Go 1.26 notes](https://tip.golang.org/doc/go1.26)). `crypto/mlkem` sizes: ML-KEM-768 encapsulation key 1184 B, ciphertext 1088 B ([crypto/mlkem source](https://tip.golang.org/src/crypto/mlkem/mlkem768.go?m=text)).

**libsodium secretstream (XChaCha20-Poly1305):**
- A single-key stream with tags MESSAGE, PUSH, REKEY and FINAL. It detects truncation, reordering, duplication and removal ([libsodium docs](https://doc.libsodium.org/secret-key_cryptography/secretstream)).
- It has **no multi-recipient** mechanism, and in Go it needs cgo or a port. age's STREAM gives the same integrity properties natively in Go.

**Single symmetric data key wrapped per device** (the "keyring" model): one random 256-bit data encryption key (DEK) encrypts all records, and a small `keyring` file holds the DEK wrapped to each device's public key, to the recovery key and to the passkey PRF.
- Precedents: Firefox Sync's `crypto/keys` record holds a default key pair plus per-collection keys under AES-256-CBC with HMAC-SHA256 ([storage format 5](https://mozilla-services.readthedocs.io/en/latest/sync/storageformat5.html)). Keybase per-user keys are re-encrypted for every remaining device ([Keybase PUK](https://book.keybase.io/docs/teams/puk)).
- **(design note)** This is the better fit. Encrypting every small record as an age file to N recipients costs about 100 B (X25519) or about 1.2 KB (PQ) of header per recipient per record ([C2SP spec](https://www.github.com/C2SP/C2SP/blob/main/age.md)). Wrapping one DEK costs that once.
- **(design note)** Recommended shape:
  - Use **age (or HPKE) to wrap the DEK in the keyring**, with PQ-hybrid recipients from day one. Harvest-now-decrypt-later matters for data stored for years in a third-party cloud.
  - Encrypt **log segments and snapshots with AEAD under the epoch DEK**, for example XChaCha20-Poly1305 or an age-style STREAM with the DEK-derived key. Bind `(device-id, segment-seq, epoch)` as associated data so the storage cannot swap or replay files.

### B2. Device keys, joining, removal and rotation

- **Per-device key pair:** each device generates a hybrid KEM key pair (for receiving wrapped DEKs) and a signing key (for signing its own log segments, so others can reject forged segments). The device list is itself a signed, append-only record, which is how Keybase does it with a sigchain ([Keybase PUK](https://book.keybase.io/docs/teams/puk)).
- **Joining by QR code:** a QR code can carry a high-entropy one-time secret plus the existing device's public-key fingerprint. That is already enough to authenticate a key exchange; a PAKE is needed only when the user must type a **short** code **(design note)**.
- **Joining by short code (PAKE):**
  - **magic-wormhole:** the code is a number plus words. The number picks a mailbox on an untrusted rendezvous server, and the words are the SPAKE2 password. The session key then encrypts everything, and neither the mailbox nor the transit relay needs to be trusted ([Wikipedia](https://en.wikipedia.org/wiki/Magic_Wormhole), [Warner on CFRG](https://mailarchive.ietf.org/arch/msg/cfrg/BBQ2gwCECu5ouTJjE_CE6d9Rg-0)). The Go implementation, wormhole-william (MIT), is protocol-compatible ([pkg.go.dev](https://beta.pkg.go.dev/github.com/psanford/wormhole-william)).
  - **SPAKE2:** RFC 9382, September 2023, Informational ([RFC 9382](https://www.rfc-editor.org/rfc/rfc9382.html)). Go: backkem/spake2-go, tested against the RFC vectors ([ecosyste.ms](https://awesome.ecosyste.ms/projects/github.com%2Fbackkem%2Fspake2-go)).
  - **CPace:** the CFRG-selected balanced PAKE, still an Internet-Draft (-21, 23 Apr 2026; datatracker shows "In IESG Review") ([datatracker](https://datatracker.ietf.org/doc/draft-irtf-cfrg-cpace/)). Go: filippo.io/cpace is experimental and was last pushed in 2021 ([pkg.go.dev](https://pkg.go.dev/filippo.io/cpace)). mendsec/cpace tracks draft-21 vectors (v0.1.0) **(unverified maturity)** ([pkg.go.dev](https://pkg.go.dev/github.com/mendsec/cpace)).
  - **OPAQUE:** RFC 9807, July 2025, Informational ([RFC 9807](https://www.rfc-editor.org/rfc/rfc9807.html)). It is an *augmented* PAKE for client-server password login, so it is the wrong tool for device-to-device pairing **(design note)**.
  - **Rendezvous:** with dumb storage, the PAKE messages can travel through the user's own storage (`pairing/<random-id>/a`, `/b`), the way magic-wormhole uses a mailbox, or over the LAN **(design note)**.
- **Removing a device and rotating keys:**
  - Keybase on revoke: the revoking device makes a new per-user key, encrypts it for the remaining devices, and records it with the revocation ([Keybase PUK](https://book.keybase.io/docs/teams/puk)). Device long-term keys never rotate; only the shared keys do ([Keybase ephemeral](https://book.keybase.io/docs/chat/ephemeral)).
  - **(design note)** Same model here: on removal, generate DEK epoch n+1, wrap it to the remaining devices and the recovery paths, and write all new segments under it. The removed device keeps whatever it already downloaded, which cannot be helped. Re-encrypting old data is optional and costly, and is worth it only if the storage credentials also leaked.
  - **(design note)** Also revoke the removed device's *storage* credentials where the backend allows it (S3 IAM key, WebDAV app password). Otherwise it can still delete or garble files, even though it cannot read new ones.

### B3. Recovery

- **Printed key or words (Brave):**
  - Brave Sync v2 makes a 32-byte random seed, BIP39-encoded, and new devices join by typing the words or scanning a QR code. The words are stretched with scrypt (N = 2^13, r = 8, p = 11), with a random salt stored on the server ([Brave Sync v2 wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2), last edited 2023-09-01).
  - The server sees device name and type plus item version and creation time ([same wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
  - Brave now uses 24 words plus a rotating time-based 25th word ([Brave support](https://support.brave.app/hc/en-us/articles/360047642371)). The payload cipher is AES128-CTR-HMAC via Chromium's custom-passphrase path **(unverified, community sources)** ([Privacy Guides thread](https://discuss.privacyguides.net/t/how-private-and-secure-is-brave-sync/225)).
  - Evolu uses the same idea: a 12-word mnemonic is the only restore path ([Evolu overview](https://evolu.vercel.app/docs/overview)).
- **Passphrase with Argon2id:**
  - Go: `golang.org/x/crypto/argon2.IDKey` ([pkg.go.dev](https://pkg.go.dev/golang.org/x/crypto/argon2)).
  - RFC 9106 settings: 2 GiB / t=1 / p=4, or **64 MiB / t=3 / p=4** when memory is limited. The Go docs lagged with t=1 ([golang/go#57065](https://golang.org/issue/57065)).
  - The OWASP minimum is reportedly 19 MiB / t=2 / p=1 **(unverified, secondary)** ([mojoauth](https://mojoauth.com/security-guides/argon2-in-go), [OWASP issue](https://github.com/OWASP/CheatSheetSeries/issues/1183)).
  - **(design note)** On phones, use the 64 MiB setting. A high-entropy generated recovery key (BIP39) needs no stretching at all and is safer than a user passphrase.
- **Passkeys with the WebAuthn PRF extension (wrapping the DEK):**
  - **Who uses it:** Bitwarden unlocks the vault with a PRF passkey (the PRF output decrypts the account key) in the web app and in Chromium-based extensions. On macOS this needs a Chromium browser ([Bitwarden help](https://bitwarden.com/help/login-with-passkeys)). Dashlane uses it too **(vendor-reported)** ([Corbado](https://www.corbado.com/blog/passkeys-prf-webauthn)). 1Password exposes PRF *as a passkey provider* to other sites, but its own passkey unlock uses a separately stored device key, not PRF, as of its July 2024 post ([1Password blog](https://1password.com/blog/encrypt-data-saved-passkeys)).
  - **Platform support** (Corbado matrix, updated 2026-09-29, a vendor source; [Corbado](https://www.corbado.com/blog/passkeys-prf-webauthn)):
    - Apple: iCloud Keychain PRF on macOS 15+/iOS 18+ (Safari 18+, Chrome 132+, Firefox 139+), with security-key PRF from Safari 26.4 (March 2026).
    - Android: Chrome, Edge and Samsung Internet with Google Password Manager passkeys, which "all include PRF by default"; Firefox for Android from 149.
    - Windows Hello: from the Feb 2026 update KB5077181.
  - **Native APIs:** Apple's AuthenticationServices has `ASAuthorizationPublicKeyCredentialPRFAssertionInput` and the matching registration input (iOS 18 / macOS 15) ([Apple docs](https://developer.apple.com/documentation/authenticationservices/asauthorizationpublickeycredentialprfassertioninput-swift.struct.md)). Apple fixed a hybrid-vs-local PRF mismatch in iOS 18.4 / macOS 15.4 ([Apple forums](https://developer.apple.com/forums/thread/764730)). Android Credential Manager takes raw WebAuthn JSON in `requestJson` ([Android docs](https://developer.android.com/reference/androidx/credentials/GetPublicKeyCredentialOption)); whether native Android returns `prf.results` is **unverified**.
  - **(design note) Key caveat:** a native passkey needs an RP ID domain with an associated-domain `webcredentials` file on Apple ([Apple supporting passkeys](https://developer.apple.com/documentation/authenticationservices/supporting-passkeys.md)) and Digital Asset Links on Android. So the browser vendor's domain becomes the RP, which is fine because no server is involved in PRF. Treat PRF as *one more* wrap of the DEK, never the only one; Corbado also advises this ([Corbado](https://www.corbado.com/blog/passkeys-prf-webauthn)).
- **At-rest device keys:**
  - **Secure Enclave:** classical keys are P-256 only, with no Curve25519 **(secondary)** ([swift-security notes](https://tessl.io/registry/dpearson2699/swift-ios-skills/files/skills/swift-security/references/secure-enclave.md)). iOS/macOS 26 add `SecureEnclave.MLKEM768` and `MLKEM1024` ([Apple docs](https://developer.apple.com/documentation/cryptokit/secureenclave/mlkem768.md)).
    - **(design note)** You can keep X25519/X-Wing keys in the Keychain wrapped by a Secure Enclave P-256 key, or use age's `age1tag` P-256 recipients via age-plugin-se.
  - **Android StrongBox:** only RSA-2048, AES-128/256, ECDSA/ECDH P-256, HMAC-SHA256 and 3DES, and it is "slower, more resource-constrained" ([Android Keystore docs](https://developer.android.com/privacy-and-security/keystore)). Curve25519 arrived with KeyMint 2 / Android 13 in the TEE, not StrongBox **(unverified, secondary)** ([Comviva](https://www.comviva.com/blog/safeguarding-cryptographic-keys-implementing-tee-and-strongbox-in-android-applications/)).
    - **(design note)** Use an AES-256 Keystore key, StrongBox if present, to wrap the Go core's private keys on disk.
  - **iCloud Keychain:** E2EE by default, even without Advanced Data Protection ([Apple iCloud data security](https://support.apple.com/102651)). That makes it a reasonable place to escrow a device-independent recovery wrap on Apple devices.
  - **Android Block Store / Restore Credentials:** these carry a small app token to a new device, E2EE "when possible" ([Block Store](https://developer.android.com/identity/block-store), [Android blog](https://android-developers.googleblog.com/2024/11/maintain-strong-user-relationships-with-restore-credentials.html)).

### B4. Metadata leakage on user storage

What a dumb store sees even with perfect content encryption: **file names, sizes, mtimes, counts, and who-writes-what-when** from access patterns.

Precedents:
- **Chromium sync:** `client_tag_hash` is base64(SHA1(client_tag)), and the source warns it is "exposed to the server" ([sync_entity.proto](https://chromium.googlesource.com/chromium/src.git/+/HEAD/components/sync/protocol/sync_entity.proto)). An unkeyed hash of a guessable tag such as a URL can be dictionary-attacked **(design note)**.
- **Brave:** item version and creation time are in plaintext ([Brave wiki](https://github.com/brave/brave-browser/wiki/Brave-Sync-v2)).
- **Joplin:** a plaintext item header ([Joplin spec](https://joplinapp.org/help/dev/spec/e2ee/)).
- **Cryptomator:** encrypts names with AES-SIV and flattens the directory tree to hide hierarchy ([Cryptomator vault docs](https://docs.cryptomator.org/security/vault/)), but chunked content still leaks approximate size **(inference)**.

Mitigations:
- **Keyed ids:** derive every on-storage name and record id with HMAC-SHA256(K_names, natural-id), where K_names comes from the DEK via HKDF. Never use a plain hash **(design note)**.
- **Batching hides record count:** with per-device logs, records live *inside* encrypted segments, so the store sees only `devices/<hmac(device)>/<seq>`, never per-bookmark files. This avoids Joplin's file-per-item count leak **(design note)**.
- **Padding:** Padmé pads to a limited-precision float length, with overhead at most 12%, and cuts length leakage from O(log M) to O(log log M) bits. In the paper's data, the share of Ubuntu packages identifiable by size fell from 83% to 3% ([Nikitin et al., PoPETs 2019](https://petsymposium.org/popets/2019/popets-2019-0056.php), [arXiv](https://arxiv.org/pdf/1806.03160)). **(design note)** Apply Padmé to segments and snapshots.
- **Timestamps:** the store's mtime is outside your control. Keep the real HLC times inside the ciphertext, and don't put dates in file names **(design note)**.
- **Residual:** write timing and frequency still show when each device is active. Constant-rate dummy writes are possible but rarely worth the cost **(design note)**.

### B5. Is MLS or Signal-style group keying worth it? (brief)

**(design note)** Probably not. MLS (RFC 9420, July 2023, Standards Track) gives continuous group key agreement whose cost grows with the log of group size, for groups of two to thousands ([RFC 9420](https://www.rfc-editor.org/rfc/rfc9420.html)). But:

1. **It needs ordering.** It requires that "Applications MUST have an established way to resolve conflicting Commit messages for the same epoch" ([RFC 9420 §14](https://www.rfc-editor.org/rfc/rfc9420.html)). Over dumb storage there is no sequencer, so concurrent device adds or removals would fork the group.
2. **Its main benefits don't apply.** Its post-compromise security and per-message forward secrecy are aimed at long-lived chat. Sync data must stay decryptable by every current device *and* by a recovery key that works years later, so old keys cannot be deleted anyway.
3. **Groups are tiny.** A user has perhaps 2–10 devices, and "re-wrap one DEK to N device keys on removal" is O(N) bytes, which is trivial.

The Keybase-style per-user-key-per-epoch model ([Keybase PUK](https://book.keybase.io/docs/teams/puk)) gives the property that matters: future data is safe from a removed device. It does this with one signed device list and one keyring file. Revisit MLS only if sharing between *different users* (family folders) becomes a goal.

---

## Short list of what was not verified

- Native Android Credential Manager returning PRF results. Browser support is documented; native support is not confirmed.
- automerge-go builds for Android.
- cr-sqlite maintenance status.
- Maturity of the Deln0r pure-Go loro-go and ygo ports. Both are single-author and very recent.
- mendsec/cpace maturity.
- Brave's exact payload cipher.
- Curve25519 in Android TEE / KeyMint 2.
- The Secure Enclave curve list, which comes from a secondary source.
- The age v1.3.0 release year, inferred as 2025 from v1.3.2 being 2026-08.
- The Corbado PRF matrix, which is a vendor source.

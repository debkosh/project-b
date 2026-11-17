# How production systems merge concurrent edits

Research for the browser sync design (bookmarks, history, passwords, open tabs, settings). The model under review: every item is `{guid, type, timestamp, deleted, data}`, the newest timestamp wins per guid, parent/child links and order live inside `data`, new items get new guids, and devices write only their own files to dumb storage with no server.

Every fact below has a URL. Where something is not documented, it says so. Where a point is my inference rather than a vendor statement, it says so.

---

## Figma

Granularity is one property of one object. The server keeps the latest value any client sent for each (object, property) pair. Two clients changing different properties of the same object do not conflict. Source: https://www.figma.com/blog/how-figmas-multiplayer-technology-works/

The rule is last writer wins, but with no timestamps. The blog says it is like a last-writer-wins register except "we don't need a timestamp because the server can define the order of events." Values are atomic, so two people editing the same text property get one person's whole value, never a blend. Same source.

Create and delete: object existence is treated like a last-writer-wins boolean. When an object is deleted the server drops its data, and the deleting client keeps the properties in its undo buffer so undo can bring it back. Object IDs include the client's unique ID, so offline creation never collides. Same source.

Order: each child has a fractional position between 0 and 1, and an insert takes the average of its neighbours. The parent link and the position are stored together as one property on the child, so a move updates both atomically. Reordering edits only the moved object's position. Sources: https://www.figma.com/blog/how-figmas-multiplayer-technology-works/ and https://www.figma.com/blog/realtime-editing-of-ordered-sequences/

Concurrent inserts at the same spot: the server gives the second insert its own unique position. Source: https://www.figma.com/blog/realtime-editing-of-ordered-sequences/

Cycles: parent links live on the child, and the server rejects a parent change that would make a cycle. A client can briefly see a cycle before the rejection arrives; it hides the affected objects until then. Source: https://www.figma.com/blog/how-figmas-multiplayer-technology-works/

Offline: on reconnect the client downloads a fresh copy and reapplies its offline edits on top. While a local change is unacknowledged, conflicting incoming values for that property are ignored to prevent flicker. What the user sees: nothing; the losing value just disappears. Same source.

## Linear

Linear publishes little about its merge rules. Its own post on scaling the sync engine is a talk introduction without merge details: https://linear.app/blog/scaling-the-linear-sync-engine

The most detailed description is a reverse-engineered write-up that Linear's CTO, Tuomas Artman, publicly called "correct": https://github.com/wzhudev/reverse-linear-sync-engine . According to it, the server gives every transaction a database-wide, incrementing sync ID, so all transactions have a total order. Local edits apply to in-memory models at once, but the local IndexedDB copy only holds server-approved state. Property updates use last writer wins, in server order. When a delta arrives, pending local transactions are rebased, which the author likens to OT. A rejected transaction rolls back its local change. Order uses fractional indexing in a `sortOrder` field.

Rich text is the exception. Since December 2023, issue descriptions and documents are edited collaboratively in real time: https://linear.app/changelog/2023-12-06-editor-improvements . The Yjs README lists Linear among Yjs users: https://github.com/yjs/yjs . That Linear's descriptions use Yjs specifically is my inference from those two sources, not a Linear statement.

What the user sees on a property conflict: the later value, silently.

## Notion

Edits are operations that create or update one record (a block), batched into transactions that the server commits or rejects as a unit. Pending transactions wait in a local queue in IndexedDB or SQLite. A block holds its children as an ordered array of block IDs ("content"), and a separate parent pointer is used only for permissions. Source: https://www.notion.com/blog/data-model-behind-notion

That post does not say how two concurrent edits to the same block or the same content array are merged. Not documented.

For offline mode, pages marked available offline "are dynamically migrated to our new CRDT data model for conflict-resolution," and the team names rich-text conflict resolution as a hard problem it solved, without details. Source: https://www.notion.com/blog/how-we-made-notion-available-offline . Which CRDT, and how block moves and splits merge, is not documented by Notion.

## Google Docs, Google Drive, Google Keep

Google Docs stores a document as a revision log of three kinds of change: insert text, delete text, apply a style to a range. Editors send changes to the server and then to other editors, and "each editor transforms incoming changes so that they make sense relative to the local version of the document." This is operational transformation, with the server ordering changes. Changes that do not conflict, such as two different style changes, need no transform. Source: https://drive.googleblog.com/2010/09/whats-different-about-new-google-docs_22.html (series overview: https://drive.googleblog.com/2010/09/whats-different-about-new-google-docs.html ). Granularity is per character. The user sees both edits; there is no conflict dialog.

Google Drive for plain files: I found no official Google page on how Drive for desktop handles a file changed in two places. Third-party guides say it makes a copy marked "[Conflict]": https://filerev.com/find-resolve-conflicted-google/ . Treat as unconfirmed.

Google Keep: the help page says that if you edit the same note on two devices and one is offline, you may see a "Conflicting edits found" notification: https://support.google.com/keep/answer/6102239?hl=en . A 2014 report describes Keep asking the user to pick: https://www.androidpolice.com/2014/04/04/neat-as-of-the-latest-update-google-keep-will-let-you-resolve-conflicting-edits/ . How Keep merges list items is not documented.

## Apple: CloudKit, CKSyncEngine, Notes, Reminders, iCloud Drive

CloudKit uses optimistic concurrency per record. Each record carries a server-issued change tag. With the default save policy, a save proceeds only if your record's change tag matches the server's; the other policies save only changed keys or all keys without the check. Source: https://developer.apple.com/documentation/cloudkit/ckmodifyrecordsoperation/recordsavepolicy

When the tag does not match, the save fails with `serverRecordChanged`, and the error carries three copies: the client record, the server record, and the ancestor record. Apple says the app "needs to merge all changes into the record for the `CKRecordChangedErrorServerRecordKey` key and attempt a new save using that record." That is a three-way merge, done by the app. Source: https://developer.apple.com/documentation/cloudkit/ckerror/code/serverrecordchanged

CKSyncEngine does not resolve this for you: "if you try to save a record and get a `serverRecordChanged`, you need to handle that error yourself." It retries only transient errors. Source: https://developer.apple.com/documentation/cloudkit/cksyncengine-5sie5 . Apple's sample merges the server record into the local object and re-queues the save: https://github.com/apple/sample-cloudkit-sync-engine/blob/main/SyncEngine/SyncedDatabase.swift . The clock is the server's change tag, not a device timestamp.

Apple Notes: Apple has not documented its merge. Reverse engineering of the Notes format found CRDT structures: a text structure with per-character clocks and tombstones, last-writer-wins registers and dictionaries, and ordered sets of UUIDs for table rows and columns, where two concurrent moves can leave duplicates that Apple cleans up. Source: https://github.com/dunhamsteve/notesutils/blob/master/notes.md (pointer: https://simonwillison.net/2021/Dec/9/notes-on-notesapp/ ). The user sees merged text, no conflict.

Reminders: not documented.

iCloud Drive files: when two computers save the same file at once, "one file is chosen as the current version and any other versions are tagged as being in conflict," and apps are expected to resolve them. Source: https://developer.apple.com/documentation/foundation/nsfileversion . In the Finder and in Pages and Numbers, the user gets a Resolve Conflict window, picks which versions to keep, and versions not kept are deleted everywhere. Sources: https://support.apple.com/guide/mac-help/mh40780 and https://support.apple.com/guide/pages-icloud/resolve-document-conflicts-gil07d27350d/icloud

## Dropbox

Granularity is the whole file. If two people change the same file at once, or one edits offline, Dropbox keeps both and adds the editor's name, "conflicted copy" and the date to one file name. The user must compare and merge by hand. Source: https://help.dropbox.com/organize/conflicted-copy

Nucleus, the rewritten sync engine, keeps three trees: Remote, Local, and Synced. "You can think of each node in the Synced Tree as a merge base," which lets it tell an add on one side from a delete on the other. Source: https://dropbox.tech/infrastructure/-testing-our-new-sync-engine . That is a three-way merge against the last agreed state.

For concurrent folder moves that would make a cycle, Nucleus keeps the original directories and "the final order depends on which sync engine uploads their move first"; the old engine duplicated the folders. Files and folders have globally unique IDs, and moves are atomic regardless of subtree size. Source: https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine

## CouchDB and PouchDB

Granularity is the whole document. Each document keeps a revision tree. On one node, a write with a stale revision gets a 409 and the app must retry. Across replicas, both edits survive as branches of the tree. Source: https://docs.couchdb.org/en/stable/replication/conflicts.html

Every replica picks the same winner with no coordination: "The revision with the longest revision history list becomes the winning revision. If they are the same, the _rev values are compared in ASCII sort order, and the highest wins." Source: https://guide.couchdb.org/editions/1/en/conflicts.html . No timestamps are involved. The losers are hidden, not lost, and `?conflicts=true` lists them. Source: https://pouchdb.com/guides/conflicts.html

Deletes: a delete is a new leaf marked `_deleted`. You resolve a conflict by writing the merged result on one leaf and deleting the others. Source: https://docs.couchdb.org/en/stable/replication/conflicts.html . The docs warn that a non-newest winner can hide information, using the example of an old phone number winning over a new one. Same source. The user sees nothing unless the app shows the conflicts. PouchDB also suggests avoiding conflicts by never updating, only adding new documents: https://pouchdb.com/guides/conflicts.html

## MongoDB Realm / Atlas Device Sync

Now end-of-life, but its rules are the clearest published set. Source for all of this paragraph: https://www.mongodb.com/docs/atlas/app-services/sync/details/conflict-resolution/

- It uses operational transformation on the server to converge.
- Deletes always win: a deleted object stays deleted even if the other side edited it later.
- Last update wins per property.
- Concurrent inserts at the same list position are ordered by insert time, so two appends both survive.
- Same primary key on two sides means the same object.
- Counters have an increment operation, so two +1s give +2, not +1.
- Strings are whole values, not merged per character.
- Removing a dictionary key counts as an update, so last update wins, not deletes win.
- For custom rules, turn the property into a list, let each side append, and resolve in the data model.

The user sees nothing.

## Firebase Firestore and Realtime Database

Firestore: offline writes are queued and pushed on reconnect, and "for multiple changes to the same document, it's last write wins." Source: https://firebase.google.com/docs/firestore/manage-data/enable-offline . An `update()` writes only the named fields, `set()` without merge replaces the document, and there are field operations such as increment and array union. Source: https://firebase.google.com/docs/firestore/manage-data/add-data . So in practice it is per field when apps use updates. Transactions "will fail when the client is offline" and are retried on concurrent edits. Source: https://firebase.google.com/docs/firestore/manage-data/transactions . The clock is the server's order of arrival.

Realtime Database: the write queue is persisted to disk, but "transactions are not persisted across app restarts." Source: https://firebase.google.com/docs/database/android/offline-capabilities . It offers server-generated timestamps: https://firebase.google.com/docs/database/web/offline-capabilities . The merge rule beyond the server applying writes as they arrive is not documented on those pages.

## Replicache and Zero (Rocicorp)

Replicache: "the server is authoritative." Changes are named mutators, plain JavaScript, that run locally at once and run again on the server when pushed, where they may produce a different result. On pull, the client rewinds to the last server state, applies the server patch, and "replays any pending mutations on top." Conflict policy is whatever the mutator code does, for example recording a failed booking instead of overwriting someone else's. Source: https://doc.replicache.dev/concepts/how-it-works . So the granularity is the intent (the operation), not the record.

Zero: the same idea. The server runs each mutator in a database transaction, pending local effects are rolled back when the server's rows arrive, and a rejected mutation reverts the optimistic change. Source: https://zero.rocicorp.dev/docs/writing-data . A general last-writer-wins rule is not documented; it follows from the mutator code.

## PowerSync and ElectricSQL

PowerSync: client changes go into an ordered upload queue that the developer's backend drains. In the simplest backend "the behavior of the overall system will be per-field Last-Write-Wins," where the last update received by the server wins per field. The backend can add its own rules and validation. Source: https://docs.powersync.com/usage/lifecycle-maintenance/handling-update-conflicts

ElectricSQL (current version): sync is read-path only, from Postgres to clients. Writes, and therefore conflicts, are the app's job; the guide offers patterns from online-only writes to a local database with a change log, and notes that blunt strategies are often enough. Source: https://electric.ax/docs/guides/writes . The older Electric with rich CRDTs is no longer documented at a working URL; I could not fetch it.

## Automerge and Yjs

Automerge: concurrent assignments to the same property get a winner that is the same on every node, chosen by operation ID, not wall-clock time. The losers are kept and can be read with `getConflicts`. Lists and text keep all inserts and deletes, and concurrent inserts at the same spot get the same order everywhere. Changes to different properties merge cleanly. Source: https://automerge.org/docs/reference/documents/conflicts/ . Delete versus concurrent update is not covered on that page.

Yjs: shared types (Map, Array, Text) "automatically merge without merge conflicts," the order updates arrive in does not matter, and no central source of truth is needed. Source: https://docs.yjs.dev/ . The specific rule for two concurrent sets of one Map key is not stated there. Shipped users listed in the README include Evernote, Gitbook, JupyterLab, AFFiNE, AppFlowy, Proton Docs, Nextcloud, AWS SageMaker and Linear. Source: https://github.com/yjs/yjs . In all of these the user sees both edits, no dialog.

## Firefox Sync bookmarks (dogear) and Chrome Sync

Firefox (dogear) does a three-way merge of the local tree and the remote tree. From the merge code comments: https://raw.githubusercontent.com/mozilla/dogear/master/src/merge.rs

- Value conflict on an item changed on both sides: the newer side wins by timestamp.
- Child order: if local is newer, merge local children first, then append remaining remote children; and the reverse if remote is newer. No child is dropped.
- Move conflict: if only one side changed a child's parent, keep the new parent; if both did, the newer wins.
- "Remote non-folder change wins over local deletion," so an edited bookmark comes back.
- "Local folder deletion wins over remote change," but children added inside it on the other device are moved to the merged parent. This "avoids data loss if the user adds a bookmark to a folder on another device, and deletes that folder locally."
- New local items are deduplicated against remote items by content (title and URL for bookmarks; content and position for separators).

Overview: https://github.com/mozilla/dogear

Chrome Sync: per entity. By default, when an entity has a pending local change and a remote update arrives, "the remote version overwrite[s] the local version unless the remote version is a tombstone, in which case the local version wins." The docs warn that because tombstones are not passed to the initial merge, a deletion made elsewhere while sync was off can revive the entity. Source: https://www.chromium.org/developers/design-documents/sync/model-api/ . Bookmark order uses unique positions stored on each entity, made unique across clients by a client tag hash, so moving one item commits only that item. Source: https://chromium.org/developers/design-documents/sync/unique-positions

## git and Wikipedia

git: a three-way merge against the common ancestor, by default with rename detection. Non-overlapping changes are taken as is; when both sides change the same area, git "cannot randomly pick one side over the other" and writes conflict markers for a human, optionally with the base text (diff3, zdiff3). Source: https://git-scm.com/docs/git-merge

Wikipedia (MediaWiki): a diff3-style merge, so a conflict happens only if two people edit "the same few lines." When it fails, the editor sees the current page in one box, their own text in another, and a diff between them, and merges by hand. Source: https://en.wikipedia.org/wiki/Help:Edit_conflict

## Things 3, Todoist, Trello, Jira

Things: Cultured Code named the exact hard cases, such as a to-do checked off on one device and deleted on another, and reordering on one device while adding or deleting on another, and said the merge algorithm resolves them. They did not publish the rules. Source: https://culturedcode.com/things/blog/2010/12/state-of-sync-part-1/ . Their support page says that after logging back in you may be offered merge options, recommending "Keep all to-dos": https://culturedcode.com/things/support/articles/2803590/

Trello (Android offline): changes are stored as per-field deltas with before and after values, run in order. The team expected conflicts to be the top problem and found them very low on the list, with last writer wins good enough in most cases. Sources: https://www.atlassian.com/blog/atlassian-engineering/syncing-changes (deltas) and https://www.atlassian.com/engineering/sync-failure-handling (conflicts ranked "very low"; "in most cases, last-writer wins is good enough")

Todoist: the Sync API takes batches of commands with UUIDs and client temp IDs that the server maps to real IDs: https://developer.todoist.com/sync/v9 . How concurrent edits resolve is not documented.

Jira: not documented.

---

## Cross-system table

| System | Granularity | Rule | Delete vs edit | Order and trees | Clock | User sees |
|---|---|---|---|---|---|---|
| Figma | Property of object | LWW, server order | Existence is an LWW flag | Fractional index plus parent on the child, stored together; server rejects cycles | Server arrival order | Nothing |
| Linear | Property (text: collaborative) | LWW, server order, rebase pending | Not documented | Fractional `sortOrder` | Server sync ID | Nothing |
| Notion | Block record; CRDT offline | Server-committed transactions; CRDT for offline pages | Not documented | Children array in the parent | Server | Not documented |
| Google Docs | Character | OT, server order | Transform of deletes | Text positions transformed | Server revision log | Both edits |
| Google Keep | Note | Unclear | Not documented | Not documented | Not documented | "Conflicting edits found" |
| CloudKit | Record (changed keys optional) | Optimistic check, app does three-way merge | App decides | App decides | Server change tag | App decides |
| Apple Notes | Character and register | CRDT (reverse-engineered) | Tombstones | Ordered UUID sets for tables | Per-replica clocks | Nothing |
| iCloud Drive | File | Keep both | n/a | n/a | n/a | Resolve Conflict window |
| Dropbox | File | Keep both; three-way vs Synced Tree | Merge base tells add from delete | Atomic moves; cycle keeps originals | Server | Conflicted copy |
| CouchDB/PouchDB | Document | Deterministic winner, losers kept | Delete is a leaf; live branch shows | App's problem | Revision depth and hash | Nothing unless app shows |
| Realm Device Sync | Property | OT, last update wins | Deletes win | Same-spot inserts by time; counters merge | Server-side op order | Nothing |
| Firestore | Document; fields via update | LWW | Not documented | Not documented | Server arrival | Nothing |
| Replicache/Zero | Operation (mutator) | Server re-runs, client rebases | App code | App code | Server | Optimistic, then server state |
| PowerSync | Field | Per-field LWW at backend | Backend decides | Backend | Server arrival | Nothing |
| ElectricSQL | n/a (read path only) | App | App | App | App | App |
| Automerge | Property, list element, character | CRDT, deterministic winner, losers kept | Not stated on page | List CRDT | Operation IDs | Both edits; `getConflicts` |
| Yjs | Map key, array element, character | CRDT | Tombstones | List CRDT | Client ID and clock | Both edits |
| Firefox dogear | Bookmark item and children list | Three-way merge, newer wins per conflict | Edit beats delete; folder delete wins but children rescued | Children merged, none dropped; newer parent wins | Device timestamps plus server-modified | Nothing |
| Chrome Sync | Entity | Remote wins | Local edit beats remote tombstone | Unique position on each entity | Server version | Nothing |
| git | Line, with base | Three-way merge | Modify/delete conflict for a human | Renames detected | Commit graph | Conflict markers |
| Wikipedia | Lines | diff3 | n/a | n/a | Base revision | Two boxes, merge by hand |
| Trello | Field | LWW | Not documented | Not documented | Server | Nothing |

---

## What this says about the owner's model

The model is sound in its frame. A guid per item, a tombstone flag, and deterministic newest-wins is what Chrome, Realm, Firestore and CouchDB all do at their core. Devices writing only their own files is better than most: there is never a write race on storage, which is the case that produces Dropbox and iCloud conflicted copies. Every device reads all files and computes the same answer, which is the CouchDB property of choosing the same winner everywhere.

Where it differs from the industry is in two things. Almost nobody runs whole-record newest-wins on records that people edit field by field. And nearly every system here has a server to order events; this model has none, so the timestamp is the only clock, and it has to be a good one.

### Where it fits as is

History visits fit perfectly if each visit is its own record. Records are only ever added, never edited, so there is nothing to conflict. This is PouchDB's "accountants don't use erasers" advice: https://pouchdb.com/guides/conflicts.html

Open tabs fit if each device's tab set is a record only that device writes. One writer means no conflict at all.

Settings fit if each setting is its own record with a stable guid made from the setting's name. Then it is per-key last writer wins, the same thing Realm and PowerSync do per field.

### Where whole-record newest-wins loses something users would notice

**1. Two devices change different fields of one record.** Phone changes a saved password; laptop, offline, edits the username or a note on the same login. Or one device renames a bookmark while another fixes its URL. Whole-record newest-wins drops one change completely. Figma, Realm, PowerSync, Trello and Linear all resolve per field for exactly this reason. Firefox's dogear also compares per item against a base. The fix is a timestamp per field inside `data`, or a three-way merge against the last synced copy, which CloudKit hands you as the ancestor record.

**2. Two devices add to the same folder, with the child list in the folder's record.** If order lives in the parent, as in Notion's content array, then adding X on one device and Y on another both rewrite the folder record, and the older add vanishes from the folder. Figma, Linear and Chrome avoid this by storing the parent and position on the child, so an add never touches the parent. Dogear keeps the list in the parent but merges both lists and drops no child. This is the most important design choice in the model: "order lives in the data" must mean the child's data, not the parent's.

**3. Two devices reorder the same folder.** With positions on the children, two moves of two different bookmarks both survive. Two inserts in the same gap can produce the same fractional key; Chrome makes positions unique with a per-client suffix, and Figma's server assigns a unique one. With no server, add the device ID as a suffix.

**4. Crossing moves make a cycle.** Phone moves folder A into B; laptop moves B into A. Each move is valid alone; together they detach both from the tree. Figma's server rejects the second. Dropbox keeps the original locations. Dogear takes the newer parent. With no server, every device must apply the same rule when it reads, for example: drop the newer of the moves in the cycle and put that folder back where it was.

**5. Delete on one device, edit on another.** Newest-wins makes the outcome depend on whose clock is later. The industry splits. Realm says deletes always win. Chrome and dogear say an edit beats a delete, so the bookmark comes back. CouchDB keeps both branches. Things named this exact case and kept the rule private. For bookmarks and passwords, edit-beats-delete is the safer choice: an unwanted bookmark returning is a small annoyance, while a lost password is not.

**6. Folder deleted on one device, bookmark added inside it on another.** The folder's tombstone wins, and the new bookmark points to a parent that no longer exists. Dogear keeps the deletion but moves the new children up to the nearest surviving folder, to avoid "data loss if the user adds a bookmark to a folder on another device, and deletes that folder locally." The model needs the same rule for orphans, applied on read.

**7. A wrong device clock.** A device whose clock runs an hour behind edits a record it just received, and its edit loses to the older value. Every server-backed system here sidesteps this by using the server's order (Figma, Linear, Firestore, CloudKit's change tag). Automerge and CouchDB use no wall clock at all. Without a server, use a hybrid logical clock: take the larger of wall time and the highest timestamp seen, plus one. Then an edit always sorts after anything the device had already seen. Break ties by device ID. Paper: https://cse.buffalo.edu/tech-reports/2014-04.pdf

**8. The same thing created on two devices.** The user saves the same login, or bookmarks the same page, on two devices before they sync. "New item, new guid" gives two records and a duplicate. Realm treats matching primary keys as the same object. Dogear deduplicates new items by content. For passwords, derive the guid from origin and username, or match by content on first merge.

**9. Counts.** Visit counts and password use counts kept as a number inside a record lose increments under newest-wins. Two +1s give +1. Realm added a special counter type for this. Storing each visit or use as its own record avoids it.

**10. Tombstones dropped too early.** If a tombstone is purged before an offline device returns, that device's old copy comes back. Chrome documents this reviving effect. CouchDB keeps revision IDs after compaction for the same reason. Keep tombstones at least as long as a device may stay offline.

**11. The user never sees the loss.** Every LWW system here loses silently. The systems that care about the loser keep it: CouchDB's hidden conflicts, Automerge's `getConflicts`, Dropbox's conflicted copy, iCloud's Resolve Conflict window, Keep's notice. For passwords, keeping the losing value as a previous password, rather than discarding it, costs little and protects the case users would notice most.

### Net

Newest-wins per guid is the right base. Four additions turn it into what the industry ships: a per-field timestamp (or three-way merge) for records edited field by field; parent and position stored on the child, with a device suffix on positions; a hybrid logical clock with a device-ID tiebreak; and fixed read-time rules for delete versus edit, orphans, cycles, and duplicate creates. None of these needs a server.

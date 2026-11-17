# Merging edits from several devices: what the industry does

Research for [sync.md](../sync.md), 2026-11-17. The question: when two or more devices edit, add,
delete, move and reorder the same data at once, how do shipped systems merge it, and what does
that say about the owner's model? The detail, with a source for every fact, is in two files:
[merging-systems.md](merging-systems.md) covers twenty products, from Figma, Linear and Google
Docs to CouchDB, Realm, Dropbox and git. [merging-theory.md](merging-theory.md) covers the papers
and the known pitfalls.

The owner's model: every synced item is `{guid, type, timestamp, deleted, data}`. The newest
timestamp wins for each guid. Parent, child and order live in `data` and belong to the data type.
A device creates a guid when it creates something, and otherwise uses the synced guid.

## The short answer

The model is sound. In the papers' terms it is a map from guid to a last-writer-wins register,
merged from every device's state, which always converges whatever order the files are read in
([Shapiro et al.](https://hal.sorbonne-universite.fr/inria-00609399v1)). Chrome, Realm,
Firestore and CouchDB all have this at their core. Each device writing only its own files is
better than most: it never races another device on storage, which is what makes Dropbox's and
iCloud's conflicted copies.

The industry differs from it in two places. Almost no one applies newest-wins to a whole record
that people edit field by field. And nearly every system has a server to put events in order; we
have none, so the timestamp has to be a good one.

## The ways to merge, from simple to heavy

- **Newest wins, whole record.** Chrome Sync, Firestore by default. Simple, and loses one side
  silently.
- **Newest wins, per field.** Figma, Realm, PowerSync, Trello, Linear for issue fields. Two edits
  to different fields both survive.
- **Keep both.** Dropbox and iCloud Drive (a conflicted copy), CouchDB (one winner everywhere, the
  loser kept), Google Keep (a notice). The loser can be got back.
- **Three-way merge against the last common version.** Firefox's bookmark merge, CloudKit's
  ancestor record, Dropbox's engine, git. Tells "I changed this" from "they changed this".
- **Merge each keystroke.** Google Docs with operational transforms on a server; Yjs, Automerge and
  Apple Notes with CRDTs. Needed for text that two people type at once. A browser has none.
- **Ask the person.** git's conflict markers, Wikipedia's two boxes. Right when the content is
  theirs and a wrong guess is costly.

Server order and replaying the edit on the server (Linear, Replicache, Zero) need a server, so
they're out.

## Where newest-wins per guid loses something a user would notice

From [merging-systems.md](merging-systems.md#what-this-says-about-the-owners-model), with what
other systems do:

1. **Two fields of one record edited on two devices.** The password changed on the phone, the
   username fixed on the laptop: one change is lost. Systems with per-field merge keep both.
2. **Two bookmarks added to the same folder, if the folder holds its child list.** One add drops
   out. Figma, Linear and Chrome keep the parent and position on the child, so an add never
   touches the folder.
3. **Two inserts into the same gap** get the same position. Chrome appends a per-device suffix.
4. **Crossing folder moves**, A into B on one device and B into A on the other, make a cycle.
   Without a server, every device has to apply the same repair when it reads.
5. **A delete on one device and an edit on another.** Realm lets the delete win. Chrome and
   Firefox let the edit win.
6. **A bookmark added inside a folder another device deleted.** Firefox moves it up to the nearest
   folder that survives.
7. **A device clock running behind.** Its new edit loses to older data. Servers avoid this with
   their own order; we need a clock that never goes behind what the device has already read.
8. **The same login or bookmark saved on two devices before they sync** gives two guids, a
   duplicate.
9. **Counts** kept in one record, such as visit counts, lose increments.
10. **Tombstones dropped too early.** A device that comes back after that brings deleted items
    back. Cassandra calls them zombies.
11. **The loss is silent.** That matters most for passwords.

## Guidelines we can adopt

From [merging-theory.md](merging-theory.md#guidelines-we-can-adopt), where each rule has its
sources. None needs a server, a CRDT library or text merging.

1. **The timestamp is a hybrid logical clock:** the larger of the wall clock and the highest
   timestamp read from any file, then a counter, then the device id to break ties. An edit made
   after seeing another always sorts after it, whatever the local clock says
   ([Kulkarni et al.](https://cse.buffalo.edu/tech-reports/2014-04.pdf)). Jepsen measured
   newest-wins losing about 28 percent of concurrent writes to the same value in Cassandra
   ([Kingsbury](https://aphyr.com/posts/294-jepsen-cassandra)).
2. **Cap timestamps far in the future**, say more than a day ahead, so one wrong clock can't win
   every conflict.
3. **Parent and position live on the child, never as a list in the parent.** The position is a
   fractional index with the device id on the end
   ([Figma](https://www.figma.com/blog/realtime-editing-of-ordered-sequences/)). This is the one
   rule the data types must follow for the owner's split to work.
4. **The tree is computed after merging, with fixed repairs:** an item whose parent is missing or
   in a cycle goes to one fixed folder, the same on every device. The repair isn't written back
   unless the user then edits the item.
5. **Deleting a folder marks only the folder.** Its children are hidden, so a bookmark added there
   on another device can still be found.
6. **Delete against edit: the newer wins**, and with the clock in rule 1, "newer" means "made after
   seeing the other". That is edit-wins when the edit came later.
7. **Tombstones are kept for a fixed time, say 90 days.** A device away longer than that takes the
   others' state first, then reapplies its own changes since it last synced.
8. **Each device keeps its last synced copy**, so it can tell its own changes from others'.
9. **Singletons get fixed guids:** `setting:theme`, the bookmark roots, each device's tabs.
10. **History visits get guids made from their content**, a hash of URL, time and device, so a
    re-import doesn't duplicate them.
11. **Logins match on site and username**, and a login changed on two devices keeps the losing
    password as a previous version, with a quiet note. Everything else resolves silently.

## Where the model is enough as is

Open tabs (only their own device writes them), settings (one guid per setting), and history
visits (only added, never edited). The envelope itself doesn't change: the rules above decide
what goes into `timestamp` and how `data` is split.

## The open question for the owner

Point 1 in the losses list, two fields of one record. The industry merges per field. In the
owner's model that can stay the data type's job: a type that needs it keeps a timestamp per field
group inside `data`, for bookmarks title, URL, and parent with position, and merges the two
versions itself. Sync's own rule doesn't change. The alternative is to keep whole-record
newest-wins and accept the rare lost edit, with passwords protected by rule 11.

# Profile Lock

A passphrase over a profile's sealed data: saved logins and passkeys when they are built, and
sync's own details today. **Not built: a design to agree.** It sits over the keys in
[sync.md](sync.md#the-keys) and the store in [store.md](store.md).

## His decisions

- 2026-11-18: "yes keychain default, passphrase lock over it... and enterprises can later force
  passphrases via policy"
- 2026-11-18: "why not just wrap the dek with passphrase.. so keyring is always the one
  decrypting the dek.. passphrase only adds another layer to it"
- 2026-11-19, its name and its modes: Profile Lock; "lock automatically after inactive for n
  minutes, or on browser close, or only when I do it myself"
- 2026-11-19: "the app does not know about the sealing of the passwords. we have one store
  manager which is the interface we use to store stuff like history, bookmarks etc... when it
  needs a passphrase or something it tells that to the shell, which needs to show the prompt and
  give that back, while the call to show a password or fill it waits for a promise or something"
- 2026-11-19: "so a locked profile works but just cannot touch sealed stuff... locking the UI
  itself is a different feature which we can build later"
- 2026-11-19: "now for a simple mental model... when the profile is locked, the memory or the
  profile folder no longer hold a key which can be decrypted by the keychain to get the DEK"
- 2026-11-19: no Apple developer licence for now, so no team id: the device key stays an
  ordinary login keychain item, and Touch ID can't be tied to it.

## The model

The profile key seals everything a profile seals. Today one file holds it, `profile.key`, which
the device key alone opens. Profile Lock adds a second file and one rule.

| File | Opens with | When it is there |
|---|---|---|
| `profile.key` | the device key | Profile Lock off: always. Profile Lock on: only in the mode "when I lock it", and only while unlocked |
| `profile.lock` | the device key, then the passphrase | Profile Lock on: always. It never changes at a lock or an unlock |

**Locked** means what he said: nothing in memory and nothing in the folder gets to the profile
key with the device key alone. **Unlocked** means the profile key is in memory.

- **Lock:** forget the key in memory, and delete `profile.key` if it is there.
- **Unlock:** the passphrase opens `profile.lock`, and the key is in memory. In the mode "when I
  lock it", `profile.key` is written too, so the next start is unlocked.
- **Turning it on:** write `profile.lock`. **Turning it off:** write `profile.key`, delete
  `profile.lock`.

`profile.lock` is the profile key sealed with a key made from the passphrase (argon2id, its salt
and settings at the head of the file), and that sealed again with the device key, the profile's
id bound into both. The device key stays the outer layer, as he asked: a folder copied to
another device doesn't open there even with the passphrase, and guessing the passphrase needs
the device key first.

## What a locked profile does

Everything that isn't sealed works as ever: tabs, pages, history, bookmarks, downloads,
settings. Only sealed data waits for the passphrase.

- **Something a person asked for** (fill, show, copy, save or change a login; use a passkey;
  turn sync on; Unlock): the shell shows the unlock Dialog over the tab that asked, and the call
  waits. The right passphrase runs it. Cancel answers it "locked", and the feature does no more
  than say so where the person is looking.
- **Something nobody asked for** (sync's own timer): never raises a Dialog. It is answered
  "locked" and runs again when the profile is unlocked.
- One Dialog at a time for a profile. Calls that arrive while it shows wait on the same one.
- A call that waits can be taken back by whoever made it: the page's request timed out, the
  person cancelled it there, or the tab closed or went elsewhere. It is answered "cancelled",
  and the Dialog closes when no call waits on it any more.
- A wrong passphrase leaves the Dialog up with "That passphrase isn't right". Each try costs
  the half second argon2id takes, which is the only brake: a count-out would stop nobody who has
  the files.

**Sync stops while locked, all of it.** The sync key and the token are sealed, and they have to
be: with them, anyone could fetch every synced login from the server and open it. So a locked
profile doesn't send or receive bookmarks and history either. Settings' Sync says "Paused:
profile locked" with Unlock.

## The modes

The modes only choose when Lock happens by itself. Lock in the app menu and in Settings works in
all three.

| Mode | Locks | Kept on disk while unlocked |
|---|---|---|
| When I lock it | only at Lock | `profile.key`, so a restart stays unlocked |
| When the browser closes | at quit; every start is locked | nothing |
| After n minutes without use | after n minutes, and at quit | nothing |

- "Without use" is no key, pointer or touch in any window of the profile. The Mac going to sleep
  or locking its screen locks at once. On Android the app in the background counts as not in use.
- n is 5, 15, 30 or 60, and starts at 15.
- The mode it starts in: **when the browser closes**.

## Turning it on, changing it, losing it

- **On:** Settings, Profile Lock: the passphrase twice, and the mode. At least 8 characters,
  with Suggest one (four words). Shorter than sync's 12 because it is typed often and the device
  key stands in front of it; sync's passphrase is all that guards the server's copy.
- **Changing the passphrase or the mode, or turning it off,** asks for the current passphrase
  even while unlocked, so a person at an unlocked browser can't take the lock away.
- **Forgotten:** there is no way back in. Start fresh deletes what was sealed and the lock;
  history and bookmarks stay. With sync on, saved logins return from the server once sync is
  set up again with sync's passphrase.
- It is apart from sync's passphrase. A person may choose the same words; neither is made from
  the other.

## What it protects, and what it doesn't

Protects sealed data, while locked, from a person at the unlocked Mac or phone, from another
program that can read the keychain and the folder, and from a copy of the folder.

It does not protect:

- **While unlocked.** The key is in memory, and a program running as the same person can read
  it.
- **"When I lock it" is the weakest mode.** A backup or a snapshot taken while unlocked holds
  `profile.key`, and deleting a file doesn't scrub a disk. A passphrase typed that rarely is
  also the one most often forgotten.
- **Memory.** Go can't promise a key is gone from memory once forgotten. The key is kept in one
  buffer that is overwritten at Lock, and that is as far as it goes.
- **The window.** A locked profile still shows its tabs and history. Locking the UI is another
  feature, later.

## Where it lives in the code

Sealing moves behind the store, so no feature holds a key or knows whether the profile is
locked. A feature gives the store a job that needs sealed data, and says whether a person asked
for it. The store opens the key, asking the shell through one hook when it must, and then runs
the job; a job nobody asked for is answered "locked". `core/keys` becomes the store's own part,
and sync, the first user, moves to it.

The unlock Dialog is one more kind in the list Go already sends every shell for a page's prompt
and sign-in (`core/app/dialogs.go`), so each shell draws it with the Dialog it has, on the Mac
and on the phone.

Two names change. Today's code calls a profile whose key the device key doesn't open "locked"
(`keys.ErrLocked`, a folder moved from another device). That becomes "can't be opened here", and
"locked" is Profile Lock's alone.

## The screens

To mock, and to ux before him:

1. Settings, Profile Lock: off; the turn-on Dialog; on, with its mode, Lock now, Change
   passphrase and Turn off.
2. The unlock Dialog in a shell, on the Mac and on a phone with the keyboard up, and its wrong
   passphrase.
3. Settings' Sync while locked: "Paused: profile locked", Unlock.
4. Forgotten: Start fresh, from the unlock Dialog.

## Not in it

- Locking the window.
- Touch ID or a fingerprint in place of the passphrase: with no team id a keychain item can't be
  tied to them. A passkey's own check per use is apart ([passkeys.md](passkeys.md)).
- A policy that forces Profile Lock or a mode.
- Sealing cookies or history.

## Tests

- Go: each mode's files through on, unlock, Lock, quit and start; a wrong passphrase; Cancel;
  several calls on one Dialog; a job nobody asked for never asks; a copied folder; Start fresh.
- The shells on the mock engine: the unlock Dialog in every shell, by the shells' contract.
- Settings on the mock engine: every screen above, both layouts and a phone.
- The Android driver: on, quit, a start that is locked, unlock with the real keyboard.
- Every new action and message, by name (`docs/action-coverage.md`).

# Passwords

The logins Project B keeps for a profile. **The first slice, built 2026-11-20:** they come
over from another browser or a file, are kept sealed, and are listed in Settings, where one
can be shown, copied and deleted. **Since 2026-11-20 they sync,** and a phone has the list
too. **Since 2026-11-22 they fill into sign-in pages, and a sign-in with a new login or a
new password offers to save it** ([password-autofill.md](password-autofill.md)). The keys
under it are in [sync.md](sync.md#the-keys); bringing them over is in
[onboarding.md](onboarding.md).

![The list, a login opened and its password shown](screenshots/passwords/desktop-shown.png)

## His words

- 2026-11-18: "username is not that critical": the site and the username are plain, the
  password is sealed.
- 2026-11-18, on bringing them over: "and password import as a second option under imports
  settings... so we have improt from browsers and imports from file.. like exported bookmarks
  password etc". A login that clashes with one saved: "both stays".
- 2026-11-20: "i need to be able to import passwords", from a browser Project B doesn't know
  by name, through its folder.
- 2026-11-20, to release: "i want the sync fixed so that i can take everything cleanly to my
  phone", his passwords with it: they sync, and the phone lists them.

## What is kept (`core/passwords`)

One table of the profile's store, `logins`: an id, the site's origin (`scheme://host[:port]`),
the username, the password, when it was saved, last edited and last used.

- **The password is sealed** with the profile key (XChaCha20-Poly1305, `keys.Seal`), bound to
  its row's id and its origin: copied to another row or another site, it doesn't open.
- **The origin and the username are plain,** so the list is read with no key, and asks the
  person nothing.
- **Start fresh deletes them** (`keys.OnStartFresh`): a new profile key opens none.

## Sync

Collection `logins`, an item per login (`Synced` in `core/passwords/sync.go`,
[sync.md](sync.md)). The item's private part is every column in the clear, the password
among them, inside synckosh's sealing under the sync key: the server reads no site, no
username and no password. Read opens a row's password and Write seals it under this
profile's own key, in sync's job, and asks the person nothing: only showing or copying one
does. A row that doesn't open is set aside, with why on Sync details, and the rest syncs.
The one changed last wins.

**What makes two devices' logins one is the site, the username and the password**, not the
row's id, which each device makes for itself. The same login brought to two devices ends as
one entry on both, under the id of the one that synced first; a second password for a site
and username stays a second entry, as bringing one in keeps both.
- A profile whose key doesn't open on this device keeps what it has, shows none of it, and
  takes nothing new.

| Call | What it does |
|---|---|
| `Import(logins) (added, already, skipped)` | keeps what an import read. Already: the same origin, username and password. A login for an origin and username saved with another password is added beside it. Skipped: an origin that isn't a web page's, or no password; all of them when the profile's key doesn't open |
| `List()`, `Total()`, `Get(id)` | the logins without their passwords, by site and then username; how many; one |
| `Password(ctx, id)` | the password, after the device has checked the person (`Verify`): Touch ID or the Mac's password, the phone's screen lock. `ErrNone` for a login that isn't there asks nothing |
| `Delete(id)` | deletes one |

## Settings, Privacy, Passwords

A row under Privacy, over Passkeys: "Passwords", "The logins Project B keeps for your sites",
with how many there are. It opens a list page. A phone has it too: it has no import, as
Android keeps each app's data from the others, and its logins come by sync.

- **Over the list:** "Sealed on this device for this profile. Showing or copying a password
  asks for Touch ID or your Mac's password, then not again for 5 minutes while this page
  stays open." (On a phone: "your screen lock".) While sync is on the first sentence is
  "Sealed for this profile; your sync server can't read them.": they are on the server too,
  sealed.
- **Search passwords:** keeps the logins whose site or username has what was typed. Escape
  empties it. None matching: "No saved password matches “…”".
- **One row a login, on a Mac and on a phone alike:** its site's mark, the site, the
  username ("No username" when it has none), and at its end three icon buttons: Show, Copy
  and a menu. Sudipta, 2026-11-20: "too many clicks to see a password.. first expand then
  copy... also the copy show can be icons", "one condensed row whic the username showing and
  the password only when we click show", "both on android and on desktop". Hovering a row
  says when it was saved.
  - **Show** (an eye) puts the password on the row, under the username, in monospace, and
    becomes Hide (an eye struck through). It stays until it is hidden, the window or the
    page is left, or the check that showed it holds no more: nothing still shows it on
    return.
  - **Copy** puts the password on the clipboard in Go, so the page never has it for a copy,
    and marked as a secret (`system.CopySecret`): on the Mac the pasteboard's concealed and
    transient types, which clipboard managers and Universal Clipboard's history leave out;
    on Android the clip's sensitive flag, which also hides the system's preview of it from
    Android 13.
  - **The menu** (⋯) has Copy username, copied plainly and with no check, and Delete…,
    which asks: "Delete this password?", "Your saved password for **site** (username) is
    deleted from this profile. Your account on site isn't changed." (while sync is on,
    "…is deleted here and on your synced devices.", as the delete goes round), Cancel and
    Delete, the danger.
  - A line under the username answers the last press for two seconds: "Password copied",
    "Username copied". With no screen lock on the device, in `--danger`: "Set a screen lock
    on this device to show passwords". A check the person refuses says nothing. A Linux
    keyring that gave no key says the keyring's line (docs/sync.md); Show or Copy again asks it again.
- **The device checks the person once for the page.** Sudipta, 2026-11-20: "its asking for
  each and everyone... we should ask it once while on the screen and ask again either after
  sometime say 5mins or if that place is closed". The first Show or Copy brings the
  device's check; while that page stays open it holds for five minutes
  (`passwordCheckHolds`), for every login on it, through a switch to another app and back.
  It ends when the five minutes are up, when the page is left (`passwordsClosed`) and when
  it opens anew (`passwordsList` with `fresh`); Go holds it, by the page's tab, so no page
  can say it is checked. Firefox's saved logins hold a check five minutes and Chrome's
  settings one; both ask again once the page is opened anew.
- **Under the list:** "3 saved".
- **None:** "No saved passwords yet. Bring them from another browser under Bring your things
  over." On a phone: "No saved passwords yet. With sync on, the ones on your
  other devices come here."

## Messages

| The page sends | Go answers |
|---|---|
| `passwordsList`, with `fresh` as the page opens | `passwords {passwords, never}`: each login `{id, site, origin, username, created}`, never a password, and `never`, the sites never offered to save on, each `{origin, site}`. Sent again to every Settings page after a change |
| `passwordShow {id}` | `passwordShown {id, password, heldUntil}` to the asking page alone, `heldUntil` being when the check holds until, Unix milliseconds, for the page to hide what it shows then; or `{id, error}`: `denied`, `noScreenLock`, `notHere`, `gone`, `failed` |
| `passwordCopy {id, what}` | `passwordCopied {id, what}`, with `error` as above. `what` is `password`, after the device's check, put on the clipboard as a secret, or `username`, with no check, plainly. Go writes the clipboard |
| `passwordDelete {id}` | the list and the count again |
| `passwordNeverRemove {origin}` | the list again, without that site under Never saved |
| `passwordsClosed` | nothing: the page was left, and its check holds no more |

The snapshot has `passwordCount`, and `passwordCount {passwordCount}` follows each change.

## Not in it yet

- A generator.
- Adding or editing one by hand, and exporting them.
- Clearing the clipboard a while after a password was copied.
- Profile Lock, a passphrase over them: designed, on hold.

## Tests

- `core/passwords`: import, the list and a password after the check, with the store's file
  holding no password plain; brought again, and a clash keeping both; a password bound to
  its row; delete; Start fresh; a profile whose key doesn't open.
- `core/app` (`TestSettingsPasswords`): the five messages; the device's check at the first
  Show, holding for that page through a Copy and another login's Show; asked again after
  `passwordsClosed`, after the page opens anew, and once the hold has passed, and not when
  the list is only read again; none for a username; a password copied as a secret and a
  username plainly; and each refusal.
- `shell/e2e/password-settings.cjs`, in the one-page, Sections and phone layouts: the row
  and its count, the list, the search, a row's three icon buttons, Show and Hide on the
  row, Copy and another login's Show with no second check, the username from the menu, the
  check asked again once the page was left, refused and with no screen lock, Delete from
  the menu, and none left.
- `tools/drive_android.py passwords_on_the_phone`, the real app on an emulator: another
  device (`core/cmd/syncdevice`, a profile of its own) saves a login and turns sync on with
  a bucket on a test server; the phone joins it with the passphrase, Privacy's row counts
  the login, the page lists it, Show says to set a screen lock when there is none, and with
  a PIN Android's own check shows the password, and Copy then copies it with no second
  check.
- `core/sync` (`TestSavedLoginsSync`, `TestTheSameLoginOnTwoDevicesIsOne`): logins brought
  to one profile are on the other, each password opening there; the server reads nothing of
  them; a delete goes round; the same login on two devices is one row on both and one item
  on the server, with a second password for it kept.
- `shell/e2e/imports.cjs`: passwords brought from a browser and from a file are counted on
  the row and listed on the page.
- `tools/vm/check.py passwords`, the real app in a VM of its own, where macOS's own sheet is
  answered with the account's password: a passwords file comes over, Passwords lists it,
  Show waits for the sheet, Copy then brings none, the copied password is on the pasteboard
  with `org.nspasteboard.ConcealedType` and `TransientType` beside it, and a username with
  neither. Passed 2026-11-20, 8 of 8, with the earlier page, where Copy asked again; not
  yet run with the row's icons and the check that holds.

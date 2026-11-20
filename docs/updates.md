# Updates

An installed copy keeps itself current. It looks at the project's release manifest every
minute, downloads a newer release by itself, checks it, and says once that it is ready, with
Relaunch and Later ([The notice](#the-notice)). Settings' About shows what it is doing all
along: the download on its way, the release ready, a failure. Nothing else shows.

## Pieces

| Where | What |
|---|---|
| `core/update` | The manager: the check loop, the download, the checks, the swap, the relaunch. `CheckInterval` is the one minute, in one place. |
| `core/relaunch` | `relaunch.Copies`: the other copies on the root, told to quit over their sockets (`core/instance`) and waited for, and each profile started again after the swap. |
| [amarit](https://github.com/sudiptadeb/amarit) | The manifest (`releases.json`), its rules (channels, no downgrade, `min_version`, a rejected version is not offered again) and the download, checked against the manifest's size and sha256 as it streams. Used through `amarit.Client`, the half of amarit for a program that installs itself. |
| `shell/src/apps/shell/standard/UpdateNotice.vue`, `mobile/UpdateBar.svelte` | A ready release's one-time notice ([The notice](#the-notice)). The notice is inside the page's box, so it sits in the page's corner in both tab layouts and takes no room; `v-shell` makes it shell, so a click on it does not reach the page. |
| `amarit.json`, `VERSION` | The project name, the manifest URL, the asset pattern; the version being built. |
| `tools/release.sh` | Publishes a release. |

The app is a signed folder (two binaries, a library, resources), not the one binary amarit
swaps in place, so the swap is ours. `amarit.Client` was added to amarit for this, and released in
amarit v0.7.0.

## The update folder

Updates are the app's, not a profile's. Every copy on a root shares its `update` folder,
beside the profile folders (`~/.project-b/update`):

- `lock`: one copy at a time holds it while it checks and downloads. A round of a copy that
  can't take it does nothing more; Check for updates waits for it.
- `stage/`: the release a copy downloaded and checked. Every copy reads it each round, so
  each offers the relaunch.
- `settings.json`: the automatic-updates setting, `{"autoUpdate": false}` when off, on when
  there is none. Each copy reads it each round and when its settings page opens, so a
  change in one holds in the others from then.
- `download/`, `previous/` and `amarit/`, amarit's state.

A copy started with `--user-data-dir` keeps all of this in `updates` in its own folder, and
takes no part in the restart of every copy below: tests and ChromeDriver run their copies so,
and never touch the copies on the root or the installed app those relaunch.

## What happens

1. Two seconds after start, then every `CheckInterval`, the manager reads the staged
   release, and unless automatic updates are off in Settings or another copy holds the lock,
   fetches
   `https://github.com/sudiptadeb/project-b/releases/latest/download/releases.json`. That is
   a plain web redirect to the latest release's asset, not GitHub's REST API, so a check a
   minute is not rate limited (the response carries no rate-limit headers).
2. If the install location isn't writable (a copy still on its DMG, or in a folder the user
   doesn't own), About says so and nothing is downloaded.
3. A newer `stable` version for `darwin/arm64` is downloaded into the update folder
   (`~/.project-b/update/download`), size and sha256 checked by amarit.
4. The zip is unpacked with `ditto` beside it, and the bundle inside must say it
   is that version (`CFBundleShortVersionString`) and pass `codesign --verify --deep
   --strict` and `codesign --verify -R='certificate leaf = H"<sha1>"'` with the SHA-1 of our
   signing certificate, compiled into the main binary. These are the two checks
   `tools/package.sh` makes before a build ships. A bundle that fails them is refused for
   good on that install: `amarit.Client.Reject` records the version, so a bad release is
   downloaded once, not every minute, by the automatic checks. Check for updates in
   settings tries it again (`amarit.Client.Unreject`): a person asking means "try it". When
   the signature check fails, `codesign --verbose=4`'s answer goes to the timeline, naming
   each file that differs from the seal. A download that fails (network, or bytes that don't
   match the manifest) is shown, and tried again next minute. A failure stays on show while
   the channel still points at that release; once it points elsewhere, it goes.
   A bundle that passes is moved into `update/stage`, so the stage only ever holds a
   checked release.
5. The notice, and About, offer relaunch, in every copy. Until one is pressed, the installed
   app is untouched.
6. Relaunch in one copy tells every other running copy to quit: each running copy holds
   its folder's `lock` (a picker copy, the root's), and the asking copy sends each one's
   socket `{"quit": true}`, the handover's request with a field of its own. Each answers
   `{"pid": <its process>}` and quits, and an open picker quits too. Once the kernel says
   each of those processes has ended, the asking copy renames the installed bundle to
   `update/previous/Project B.app`, renames the staged one into its place, leaves a
   `/bin/sh` behind for each profile that was running, its own among them, which waits for
   this process to exit and then starts the new release on that profile
   (`--root=<root> --profile=<id>`), and closes the window. A picker isn't started again.
   Each copy comes back with a new tab. Unix lets the running processes keep using the
   moved bundle, so the helpers never notice.

A failed automatic check of the manifest itself (offline) is logged, not shown: nothing was
happening for the user to see. A build running from a build directory, or with no version in its
bundle, never checks. A release staged and not relaunched into stays staged, and is offered
again at the next start; a half-finished download is cleared when the next one begins.

## States, and how each ends

What a person sees is `update.State`: its phase, and beside it how a check they asked for
went. No state lasts unless the person leaves it there: every progress has Retry and Cancel
while it runs, as the UX guide (`docs/ux-guide.md`) asks of every progress, and stopping is the
manager's, whatever the download is doing.

| State | Shown | Ends by |
|---|---|---|
| Idle | the version alone | a check finding a newer release (Downloading), or one asked for (Checking) |
| Checking (asked for) | Checking… | Up to date, Downloading, or Couldn't check, with Try again; it waits at most 15 s (`lockWait`) for the folder's lock, held by another copy or a download still stopping, then says so |
| Downloading | Downloading X… N%, with Cancel and Retry | the release checked and staged (Ready); no bytes for 30 s (`stalledAfter`), which is Failed with Try again; Retry, which stops it and checks again at once; Cancel, which stops it and shows Idle |
| Ready | Relaunch to update to X, with Relaunch | Relaunch |
| Failed | Couldn't update to X, with Try again | Try again (Retry); the channel moving off X |

Retry and Cancel stop the download by its context's cause, so the download's own late
answer is never shown: the state is the one Retry or Cancel set. A release the person
cancelled is left alone by the automatic checks for the rest of the run, so it doesn't
start again a minute later; Check for updates fetches it. The 30 s is awake time: the
Mac's sleep doesn't count, so a download that slept through the night is given up 30 s after
the wake, unless bytes arrive. Every other progress the app shows (downloads, imports) ends
on the same 30 s. Nothing watches the system's wake or network: Retry is the person's.

Sudipta, 2026-11-18, after 0.2.76 stuck at "Downloading 0.2.85… 38%" overnight with nothing to
press: "just have retry or cancel button whenever we show a progress, seems like a good
starting rule to evolve from".

## The notice

A release ready to relaunch into says so once, the owner's ask (2026-11-20: "when update become
available we can show a one time popup"). In Standard, a card over the page's bottom-left
corner, "Project B 0.3.7 is ready", with Later and Relaunch. In the Mobile shell, a bar over the
toolbar in the crash bar's form, with Later and Install. Either button records the version in
the profile's prefs, `updateNoticed`, through `setPref`, so the notice shows once a release in
a profile, not at every start, and goes in every window at once. After Later, Settings' About,
and on a phone the menu's Install update, offer it. Standard had a bubble in the page's corner
for every state; with the notice it went (2026-11-20: "since we have the popup we can remove
the bubble all together"), and a download on its way shows in About alone. The other shells
keep their own bubble or line.

## Settings

Settings' About ([settings.md](settings.md#about)) shows the running version with what the
manager is doing and its one button, the automatic updates switch, and the release notes.

- **Check for updates** runs one check at once, whatever the switch says (`CheckNow`).
  While a release downloads, Retry and Cancel stand in its place, as they do after the
  percentage in Fox's update bar; a failure there has Try again. The
  page shows what the manager says: checking, up to date, downloading a version, ready to
  relaunch, or the error it hit, including a manifest it couldn't read. A release it finds
  goes the way an automatic one does, and About's Relaunch (Install update on a phone) applies
  it as the notice's does. Automatic checks never set the panel's check result. The menu
  bar's Check for Updates… opens Settings at About and checks.
- **Updates from**, Stable | Dev, is the channel every copy follows, `channel` in
  `update/settings.json` beside `autoUpdate`, Stable when absent ([release-channels.md](release-channels.md)).
  A change checks the new channel at once; each check makes amarit's client again when the
  folder names another channel, so another copy's change holds from its next round. On a
  Dev build with Stable chosen, the hint says it stays on that build until a newer Stable.
- **Automatic updates** is the app's, `autoUpdate` in `update/settings.json`, on by
  default, and every copy shows the same. Off, the next round of every copy checks nothing;
  on again here, the timer starts as it does at launch, and the other copies check from
  their next round.

## Publishing

    tools/release.sh       # Stable
    tools/release.sh dev   # Dev: main after Go's tests and the mock suite

Every copy reads `releases/download/channels/releases.json`, the manifest of both channels
on the pre-release tagged `channels` ([release-channels.md](release-channels.md)). A Stable
release creates its numbered release first, then `amarit publish` adds it to the manifest on
`channels`, and the same manifest goes on the numbered release, where copies from before
channels read it. A Dev build takes its version from `amarit next-version -pre dev` (the next
patch with the UTC minute), writes it into `VERSION` for the build and puts it back after;
`amarit publish -prune` puts its DMG, zip and APK on `channels` beside the manifest and drops
the Dev build it replaces, from the manifest and the tag. Both give the APK's versionCode from
`amarit version-code`.

Bump `VERSION` and commit first. The script makes the release build (`RELEASE=1
OUT=out/rel`), packages and signs it, writes `Project-B-<v>.dmg` (for people) and
`Project-B-<v>-darwin-arm64.zip` (for the updater) into `build/dist`, downloads the last
release's `releases.json` so earlier releases stay listed, has `amarit release` add this
one, tags `v<v>` and creates the GitHub release with all of it. The first updater-enabled
version has to be installed by hand from its DMG, once; from then on the app updates itself.

The same release carries the Android app. `tools/build.sh android arm64` starts the Linux VM,
copies the files git tracks in `core`, `engine` and the build script, and the built shell, into
its container, builds the APK there incrementally and stops the VM; `tools/package.sh android` signs it on the Mac. The
container mounts the Android worktree, so the release's own files are copied in, and the build
rewrites a file in the tree it builds only when its content changes, so moving between
checkouts never leaves ninja trusting an older file. The APK goes into `build/dist` twice:
`Project-B-<v>.apk` for people, and `Project-B-<v>-android-arm64.zip` for the updater, the name
`amarit.json`'s one asset pattern gives the `android/arm64` target (an APK is a zip). How a phone
takes it is in [android.md](android.md#updates-on-a-phone).

## Checked

- `core/update` tests: a newer release is staged and relaunch swaps the bundle and keeps the
  previous one; a corrupted download is refused and tried again; a bundle of another
  version, or one that fails the signature check, is refused once; an unwritable location
  is reported without downloading; the same version shows nothing.
- `core/update` tests also cover a check asked for (up to date, a manifest that can't be
  read, a newer release) and the timer stopping and starting with the switch.
- `core/update` tests of the shared folder: two managers on one folder, one checks while
  the other's round does nothing, then reads the staged release without downloading it;
  the setting read the same by every manager; and a relaunch with other copies quits them
  before the swap and starts each profile after it. `core/instance` checks the quit request
  and its answer, and `core/relaunch` that `QuitOthers` returns only once real processes
  owning a profile and the root have ended.
- `tools/drive_first_app.py relaunch` runs it in the real app: copies on two profiles and a
  picker on a root of the run's own, a clone of the app under test as the installed app,
  and a staged bundle shaped like the update tests' fixtures whose executable records how
  it was started.
- `shell/e2e/settings.cjs` checks About's version, Check and switch against a local release
  server.
- `shell/e2e/update-notice.cjs` stages a release from a local server and checks the notice in
  Standard and the Mobile shell, Later recording it so a restart doesn't show it again, and
  About with this release's notes on a desktop and a phone.
- `shell/e2e/first-app.cjs` runs the real manager against a local release server with a
  slow asset, through the notice in both tab layouts, its Relaunch to the relaunch helper
  starting the new bundle, and a corrupted download refused, About saying why.
- The signature check runs only in the real app (`core/cmd/lib`); the mock has no signing
  key and checks the version alone.

## Not done

- A swap that fails after the other copies quit leaves them closed; the copy asked shows the
  failure.
- No automatic rollback: the previous release is kept, nothing starts it. A refused download
  never reaches the install, which is the case that matters.
- Only `stable`, only Apple silicon and arm64 Android, no delta downloads, no progress percentage.
- A manifest signature (amarit's enterprise tier) isn't implemented in amarit; the bundle's
  code signature against the pinned key is the trust here.

# Updates

An installed copy keeps itself current, through amarit's updater. It looks at the project's
release manifest every minute, downloads a newer release by itself, checks it, and says once
that it is ready, with Relaunch and Later ([The notice](#the-notice)); on a Mac the Dock icon
shows the download's progress and a notification offers Relaunch. Settings' About shows what
it is doing all along: the download on its way, the release ready, a failure and why.

## Pieces

| Where | What |
|---|---|
| [amarit](https://github.com/sudiptadeb/amarit) `appupdate` | The updater ([amarit's docs/appupdate.md](https://github.com/sudiptadeb/amarit/blob/main/docs/appupdate.md)): the rounds, the download with progress, Cancel and Retry, the release's checks, the stage, the install, the settings every copy shares, and the rollback of a release that won't start. It draws nothing. |
| amarit `macapp` | A Mac bundle: unpacked with `ditto`, refused unless it says it is the manifest's version and its signature, checked in the app through the Security framework, is ours (`macapp.SignedBy` of our signing certificate's SHA-1, compiled in); staged, swapped in with one `renamex_np`, the old one kept in `previous/`. `macapp.Present` puts a progress bar on the Dock icon and posts a notification with Relaunch when a release is ready. |
| amarit `relaunch` | Starting the new release once this process has gone; waiting for the other copies to end. |
| `core/update` | What the pages see of it: amarit's state in project-b's words (`words`, `why`), and the commands they send. `CheckInterval` is the one minute. |
| `core/relaunch` | `relaunch.Copies`: the other copies on the root, told to quit over their sockets (`core/instance`) and started again on each profile after the swap. |
| `core/cmd/lib/update_<os>.go` | The installer: `macapp.Installer` on the Mac, `appupdate.File` handing the APK to Android's package installer. |
| `shell/src/apps/shell/standard/UpdateNotice.vue`, `mobile/UpdateBar.svelte` | A ready release's one-time notice ([The notice](#the-notice)). |
| `amarit.json`, `VERSION` | The project, the manifest URL, `auto_update: true` (copies check on their own unless the person turns it off), the asset pattern; the version being built. |
| `tools/release.sh` | Publishes a release. |

## The update folder

Updates are the app's, not a profile's. Every copy on a root shares its `update` folder,
beside the profile folders (`~/.project-b/update`): the lock one copy at a time holds while
it checks and downloads, `stage/` (a checked release, which every copy offers),
`previous/`, `download/`, and `amarit/`, amarit's state with `settings.json`: automatic
updates and the channel, which every copy reads. A copy started with `--user-data-dir`
keeps all of this in `updates` in its own folder, and takes no part in the restart of every
copy: tests and ChromeDriver run their copies so.

## What happens

amarit's docs/appupdate.md has the whole of it; what is project-b's:

- **The checks.** Two seconds after start, then every `CheckInterval`, with automatic
  updates on, a copy reads `releases/download/channels/releases.json` for its channel. A
  newer release is downloaded, checked and staged; About and the notice offer it.
- **A refusal is the release's own answer** (another version, or a signature broken or not
  ours): it is rejected and not downloaded again until a person checks. A check that
  couldn't be made (a file that couldn't be read) is tried again.
- **Relaunch** in one copy tells every other running copy to quit (`{"quit": true}` on each
  one's socket; each answers with its pid and ends), swaps the bundle, and starts the new
  release on every profile that was running once this process has gone. A picker isn't
  started again.
- **Rollback.** Each start is counted early (`update.Manager.Settle`); the shell's first
  `shown` says the release works (`Healthy`). A release that didn't get that far three
  times is put back by the one before it, rejected, and the previous one started.
- **Programs the app starts** get its own limit on open files (`core/cmd/lib/filelimit.go`),
  not the 256 Go leaves them under launchd: they inherit the app's open files, and ditto and
  codesign once failed for want of one.

A failed automatic check of the manifest is logged, not shown. A build with no version (a
build directory) never checks, but keeps the settings.

## States, and how each ends

What a person sees is `update.State`: its phase, and beside it how a check they asked for
went. Every progress has Cancel and Retry while it runs (`docs/ux-guide.md`).

| State | Shown | Ends by |
|---|---|---|
| Idle | the version alone | a check finding a newer release (Downloading), or one asked for (Checking) |
| Checking (asked for) | Checking… | Up to date, Downloading, or Couldn't check, with why |
| Downloading | Downloading X… N%, with Cancel and Retry (and while the release is checked after it) | the release staged (Ready); nothing arriving for 30 s (Failed); Retry; Cancel, which shows Idle and leaves that release to a person's check |
| Ready | Relaunch to update to X, with Relaunch | Relaunch |
| Failed | Couldn't update to X, why on its own line, and Try again where it can help (`State.Retry`); a refused release offers Check for updates | Try again; Check for updates; a newer release |

Why, in words (`core/update.why`): the download failed (…); the download stopped arriving;
the download arrived damaged: it didn't match its checksum; its signature couldn't be checked
(…); it isn't signed with Project B's key, or its signature is broken, so it won't be tried
again; Project B can't be replaced where it is: move it to Applications; another download is
still stopping; couldn't put the new app in place (…).

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
  amarit's settings beside automatic updates (`update/amarit/settings.json`), Stable when absent ([release-channels.md](release-channels.md)).
  A change checks the new channel at once; each check makes amarit's client again when the
  folder names another channel, so another copy's change holds from its next round. On a
  Dev build with Stable chosen, the hint says it stays on that build until a newer Stable.
- **Automatic updates** is the app's, in `update/amarit/settings.json`, on by
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

- amarit's own tests cover the updater, the bundle, the signature verdicts, the swap, the
  rollback and the relaunch (amarit's repo).
- `core/update` tests: each failure's words and whether Try again is offered; a newer
  release found, staged and handed to the installer; a check on the same version says up to
  date; the channel and automatic updates shared by two managers on one folder, and heard;
  a Dev build older than the Stable running never taken; a development build keeps its
  settings and doesn't update.
- `core/app` tests: Settings' update buttons by their messages, and the settings page's
  switch and channel saved in the update folder.
- `tools/drive_first_app.py relaunch` runs a relaunch in the real app, with copies on two
  profiles and a picker.
- `shell/e2e/settings.cjs`, `update-notice.cjs` and `first-app.cjs` run the real updater
  against a local release server, through the mock's unsigned bundle: About, the notice,
  Relaunch starting the new bundle, and a corrupted download refused with why.
- The signature check runs only in the real app (`core/cmd/lib`); the mock's bundle isn't
  signed, so it is asked for no signature.

## Not done

- A swap that fails after the other copies quit leaves them closed; the copy asked shows the
  failure.
- Only Apple silicon and arm64 Android, no delta downloads.
- A manifest signature (amarit's enterprise tier) isn't implemented in amarit; the bundle's
  code signature against the pinned key is the trust here.

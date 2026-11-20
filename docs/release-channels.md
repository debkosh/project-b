# Release channels

Stable and Dev updates. Stable is what a copy follows unless Dev is chosen: a release made
by [releasing.md](releasing.md)'s full run. Dev is each finished
branch, out as soon as it lands, after Go's tests and the mock suite alone, for Sudipta
to try before the full run makes it Stable.

## The manifest

One `releases.json`, as amarit's spec has it, with both channels:

    "channels": { "stable": "0.3.8", "dev": "0.3.9-dev.202611201415" }

It lives on a GitHub pre-release with the fixed tag `channels`, at
`releases/download/channels/releases.json`, the address the app reads from the release
that brings channels on. `releases/latest/download/` is no use for it: it skips
pre-releases, and the stable release that is latest changes with each one.

Every publish, Stable or Dev, downloads that manifest, adds its version, points its
channel at it, and uploads it back with `--clobber`. A Stable release also uploads the
same file to its own release as today, which is what copies from before channels read.

## Names

- **Stable**: `VERSION`, as now: `0.3.8`.
- **Dev**: the next patch with a pre-release tag of the build's UTC time, minutes:
  `0.3.9-dev.202611201415` while `VERSION` is `0.3.8`. Semver puts it after 0.3.8 and
  before 0.3.9, so a Mac on Dev takes each new Dev build, and takes 0.3.9 itself when it
  ships as Stable. amarit's `0.3.8+g<sha>` doesn't work here: build metadata doesn't
  order, and the Mac's updater takes only a newer version.
- **Android**: the versionCode is the next patch's, 3009 here, the same for every Dev
  build of it and for the Stable 0.3.9 after. An equal code with a new name is an update
  on a phone (amarit's spec, section 3).
- One signing key for both, as now: the Mac's identity and the APK's key don't change.

## Moving between channels

The switch says which channel the copy follows; the updater never goes back a version.
Dev to Stable leaves the copy on its Dev build until Stable passes it, which the next
Stable release does, since a Dev build is always before the next version. Stable to Dev
takes the newest Dev build at the next check.

## A Dev build

1. A finished branch lands on `main`, rebased, as it does for a release now.
2. `tools/release.sh dev` on `main`: Go's tests and `tools/test.sh mock` first, and it
   stops at a failure; then the release build and package, the APK, the manifest as above,
   and the Dev build's DMG, zip and APK as assets of the `channels` pre-release, named
   with the version. `amarit release -prune` drops the Dev builds no channel points at,
   from the manifest and from the pre-release's assets.
3. The link to the Dev build goes to the orchestrator.

No tag and no test report. Its notes are written for it (`tools/dev_notes.py`): the `Notes:`
line of each commit since the Dev build before it that has one, what changed for a person in
their words ("Notes: Google sign-in works again"); a commit with none is left out. The commit
the build is made from is kept in a comment, where the next Dev build starts. The
build carries them as its own notes, What's new in it, and they are published beside the
manifest as `notes-<version>.md`, as a Stable release's are. Stable stays as it is: the full run, `VERSION` bumped, its own
tag and release, and its report.

## The switch

Settings, About Project B, in the card with the version and Automatic updates: a row
**Updates from**, Stable | Dev. Go keeps it beside `autoUpdate`, as `channel` in
`update/settings.json`, `stable` when absent; the page sends `{t:'setChannel', channel}`
and the settings snapshot carries it. A change takes effect at once: the updater's
client is made again for the new channel, and a check runs. A copy running a Dev build
names it in its version line, `0.3.9-dev.202611201415`, so the page says nothing more.

## Not in this

Rollout percentages, `min_version`, a Dev channel on a separate key, and anything for
the copies that predate channels beyond the manifest they already read.

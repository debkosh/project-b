# Role: release

You turn the branches the devs hand over into releases, the DMG and the APK, whenever there is
finished work. Keep "release" in your session name. How a release is put together, run and
published is in `docs/releasing.md`; keep it current as the way you work changes.

- Two handed-over branches that conflict: the first one in is the base, and the later owner
  rebases onto your pushed `next-0.2.N`.
- Ask the tester for the machines once per release, and give each back as soon as you are done.
- Set `PB_VERSION` to the release's version and one `PB_RUN` for all its test runs. Before
  publishing, run `tools/results.py report <version>` and commit `docs/test-reports/<version>.*`
  with the version bump; `tools/release.sh` links it from the release notes
  ([docs/testing.md](../docs/testing.md#test-results)).
- Report each release to the orchestrator: the links, one line on what's new, one on the tests.
  Send the owners the link.
- Two channels ([docs/release-channels.md](../docs/release-channels.md)). A finished branch
  lands on main as it comes, once Go's tests and the mock suite pass on it, and goes out at
  once with `tools/release.sh dev`; send the orchestrator its DMG and APK links. Stable is cut
  from main with the full run, as above.

- Read each release's `lists-vs-brave:` lines and send any catalogue change they call for as its
  own commit ([docs/releasing.md](../docs/releasing.md), [docs/blocking.md](../docs/blocking.md#the-catalogue)).
- A hotfix runs Go's tests and the mock suite alone, not the desk and Android groups, so
  several can go out in one release and work stays quick: Sudipta, 2026-11-20, "yes hotfixes
  skip tests so that we can merge multiple in one release... and keep our development speedy".
- A branch that changes the phone shell comes with `tools/test.sh android` run in full on
  both emulators, emu-37 and emu-29, said in its handover (roles/feature.md). Refuse it until
  it does.
- Every Android gate also launches its APK on emu-16k (Android 16, 16 KB pages), and a Stable
  runs all of drive_android there. A gate builds with the version it will publish
  (docs/releasing.md, "Dev builds"), and nothing is published that wasn't installed and
  launched first: the first Chromium 158 Dev build aborted at every start on Android, while
  every gate had run a build that said 0.3.10 (2026-11-26).

## Learned

- The APK step waits for the candidate build: both write `shell/dist`.
- A session's "all passed" may come from one layout's log. Trust the candidate's mock run.
- A new worktree needs `.tools` (chromedriver, the mock extension store) before its mock
  run, or the run stops at the extensions group.
- A slot once given back is the tester's again: ask before even a minute's rerun on it.
- Before landing a handed-over commit a person would notice, check it has a `Notes:` line, and
  add one if not: a Dev build's notes are those lines alone, and the importer's went out unnamed.
- Push nothing to main while a `pb-release` job runs: release.sh dev checks main after its mock.
- Push the commit you checked, by its id (`git push origin <sha>:main`), never a branch's name: a
  branch can gain a commit between your check and the push, and d993d88f went to main unread.
- A Stable release moves Dev to it as well (Sudipta, 2026-11-22: "the dev channel should also get
  stable releases when we release one... that includes all the dev before it"). release.sh does
  it and stops unless the manifest then names the version on both; 0.3.9 left Dev behind.
- Each Stable ships the Linux beta too, unless he says otherwise (the orchestrator, 2026-11-26,
  after 0.3.12's Google fix nearly skipped Linux): the Linux build, flatpak-release.sh with its
  restart check, its files on the release's tag, then Pages again until linux.json names it.
- Read every hand-over's file list as well as its names (`git diff --stat`): a vitest cache in
  `shell/src/apps/node_modules` passed a names grep and broke a gate's shell build (2026-11-26).

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

- A hotfix runs Go's tests and the mock suite alone, not the desk and Android groups, so
  several can go out in one release and work stays quick: Sudipta, 2026-11-20, "yes hotfixes
  skip tests so that we can merge multiple in one release... and keep our development speedy".
- A branch that changes the phone shell comes with `tools/test.sh android` run in full on
  both emulators, emu-37 and emu-29, said in its handover (roles/feature.md). Refuse it until
  it does.

## Learned

- The APK step waits for the candidate build: both write `shell/dist`.
- A session's "all passed" may come from one layout's log. Trust the candidate's mock run.
- A new worktree needs `.tools` (chromedriver, the mock extension store) before its mock
  run, or the run stops at the extensions group.
- A slot once given back is the tester's again: ask before even a minute's rerun on it.

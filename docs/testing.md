# Testing

How to run the tests is in `tools/test.sh` (its header lists the groups) and in
[vm-testing.md](vm-testing.md) for the macOS VM. This page is what every run leaves behind.

## Test results

Every test that `tools/test.sh` runs goes through `tools/results.py run`, which passes its
output through unchanged and keeps its result on this Mac. Nothing else changes for whoever
runs it. `PB_RESULTS=off` turns it off.

### Where

`$PB_RESULTS` (default `~/PW_TOOLS/project-b-results`)`/<version>/<run>/`:

- `run.json`: the run.
- `tests/NN-<name>.json` and `tests/NN-<name>.log`: each test, its result and its whole output.

The version is `PB_VERSION`, else the bundle `PB_BIN` runs from, else `VERSION`. A release
candidate still carries the last version until it is published, so release runs set
`PB_VERSION`. The run is `PB_RUN`: `tools/test.sh` makes one per call (the UTC time and the
groups) unless it is set, so a release sets it once and its groups share one folder.

`tools/results.py run [--group G] [--name N] [--rerun yes] -- <command>` wraps any driver on
its own, into the same run when `PB_RUN` is set. With `--rerun yes`, it reruns some parts of
the test G, N, and the report takes those parts' results in place of the earlier ones.

### run.json

| Field | |
|---|---|
| `schema` | 1 |
| `version`, `start` | as above; times are UTC, `2026-11-18T07:30:00Z` |
| `commit`, `dirty` | the checkout's HEAD, and whether it had changes |
| `tested` | the commit of the build under test, which can differ: `app` from `build.sh`'s `commit` note beside the binary or in the bundle's Resources, `apk` from the APK's name |
| `app` | the `PB_BIN`, `PB_DMG` and `PB_APK` used |
| `machine` | model, OS, arch, memory in GB, swap use, and each emulator's qemu memory in MB at the start |

### A test's JSON

| Field | |
|---|---|
| `group`, `name` | as `tools/test.sh` names it (`direct` and the command, run on its own) |
| `status`, `exit` | `passed` (0), `did_not_run` (2) or `failed` (anything else) |
| `rerun` | true when it reruns parts of an earlier result |
| `tested`, `app` | as in run.json, for this test: a run's groups can test different builds |
| `start`, `seconds` | |
| `ok`, `fail`, `failures` | the lines that start `ok ` and `FAIL`, every driver's way of saying a check passed or failed |
| `parts` | split at each `--- part <name>` line, which the drivers print before each part (`run_parts`, `drive_android.py`); one part, `all`, for a test without them. Each part has `seconds`, `ok`, `fail`, `failures` and `processes` |
| `processes` | per kind of process, over the part or the whole test: `samples`, `peakMB`, `avgMB`, `peakCPU`, `avgCPU` (% of one core) and `grows` |
| `hogs` | any process on the Mac over 4 GB during the test, the app's or not, with the time |

Kinds: `browser` (the app, Go and the browser process), `gpu`, `renderer` (all of them,
summed), `utility` (the network service), `helper`; on a phone, `android-app`,
`android-renderer` and `android-gpu`, with `@<serial>`. Only the test's own processes count,
those under its command, since other sessions run the app at the same time; on a phone, only
the test's emulator, `ADB_SERIAL`. Samples come every 2 s on the Mac (one `ps`), every 8 s on a
phone (one `top`). A VM run's app is inside the VM, so it has none.

`grows` is memory that climbs through a part and doesn't come back: the lowest of the last
third of the samples is above the highest of the first third, by 100 MB and a quarter at least.

### The release's report

`tools/results.py report <version>` merges every run of that version, the latest result of
each test, with reruns replacing only their parts, into `docs/test-reports/<version>.json` and
`.md`. Each test is `passed`, `failed`, `known` or `did_not_run`: `known` when every failure
it had matches `tools/known-failures.json` (the basics, which fail on purpose, and bugs we
know of), so the headline is the count of new failures. It compares each group's checks, each
test's status and each kind's peak memory with the previous version's JSON in that folder.

The repo copy keeps nothing of this machine but its model, OS and memory: home folders, temp
folders, the host and user names and local addresses are replaced. `tools/release.sh` links the
report from the GitHub release when the file is there.

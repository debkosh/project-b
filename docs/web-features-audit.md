# Web features: the audit of 2026-11-24

What the registry (`core/webfeatures/registry.json`) says against what a real page gets in the
real app, row by row, on the Mac, Linux and Android. No code was changed for it. The plan at the
end waits for his yes.

## How it was checked

- **Mac:** the Dev candidate at 8c7b9782 (2026-11-24), `drive_first_app.py webapis`,
  `check_basics.py`, and an audit probe that keeps each call's real answer (its value or its
  error), on load and again after a real click, with the state of 31 permissions.
- **Android:** the APK of release's Dev publish of 2026-11-24, `drive_android.py web_apis` with
  the same answers kept, on emu-37 (Android 17) and emu-29 (Android 10). The two agree.
- **Linux:** 0.3.10, the published Flatpak (Dev has no Linux bundle), on the Ubuntu VM ub3,
  the same audit probe. Read its rows as 0.3.10's.
- **Code:** every row read against `engine/`, `core/` and `oskit/` today.
- **Chromium 158:** `web_features_bump.py` on 158.0.8127.89. Lists 1 and 3 are empty. List 2 has
  14 new delegate methods. Of those, a page can see only the service worker's clipboard (off by
  content's default, as in Chrome) and XSLT (off by default in 158, in Chrome too). So 158 needs
  no new row. On the 158 Mac build (chromium-upgrade, 9e9e7ded) every call gave the same answer
  as on 155, after a click too, every permission's state is the same, and the basics are the
  same 32 of 41. Android on 158 waits for its build.

## Why it drifted

- **The probes can't fail a stale row.** In `tools/web_features.py`, `quiet` and `partly` accept
  resolved, rejected or pending: anything but a missing API or a killed renderer. A feature that
  starts working still passes as `quiet`. All 42 calls passed on all three platforms while half
  the rows were wrong.
- **The basics never meet the rows.** A row's `basics:N` is only checked for its spelling.
  Basics 10, 12, 13, 14, 15, 16, 17, 18, 22 and 41 pass, under rows that say `quiet` or
  `absent`. Release's full runs do run basics, and its failures (the URL password, a tab
  closing without asking) were reported each time as known, never raised as bugs.
- **The rows aren't touched when a feature lands.** Dialogs, downloads, the file chooser, full
  screen, camera and microphone, notifications, print, H.264 and password filling all landed
  without their row changing. The doc asks for it, but nothing fails when it is missed.
- **Linux has no column.** `Platform()` answers "mac" on Linux, so Settings on Linux shows the
  Mac's answers, and nothing probes Linux.

## What a person would notice, worst first

Settings, Web features, puts the first group in red under "Fails quietly". Today that group
lists alerts, sign-in prompts, certificate warnings, downloads, file upload, full screen and the
camera, all of which work.

### Rows that are wrong

| Row | Registry (Mac / Android) | Real: Mac / Linux / Android | Evidence |
|---|---|---|---|
| camera-mic | quiet / quiet | works / partly / works | Mac: the site's prompt (Block, Always allow, Allow this time), basics 17; Android: `page_camera`. Linux: no camera portal in the Flatpak yet, so the microphone only (code; the run's prompt was hidden behind another) |
| notifications | quiet / quiet | works / works / **quiet** | Mac and Linux: the prompt after a click, basics 16, oskit/notify (the portal on Linux). Android: no notification service, so `new Notification` throws and `showNotification` fails |
| downloads | quiet / quiet | works / works / works | basics 15; Android `downloads` |
| file-input | quiet / unknown | works / not run / works | basics 14; Android `file_inputs`; on Linux it needs the file portal, not run |
| js-dialogs | quiet / quiet | works / works / works | basics 13; Android `page_dialogs` |
| http-auth | quiet / quiet | works / works / works | basics 12 |
| cert-error | quiet / quiet | works / works / works | basics 10 |
| fullscreen | quiet / quiet | works / works / works | basics 18; the probe's click on the Mac and Linux; Android `page_full_screen` |
| beforeunload | quiet / quiet | **partly** everywhere | Leaving a page asks; **closing a tab doesn't** (basics 1 fails: "the tab closed without asking") |
| print | quiet / quiet | works / **quiet** / works | Mac panel and Android sheet; on Linux `ShowPanel` only logs |
| codecs-h264 | absent / absent | works / **absent** / works | canPlayType "probably", basics 41 plays an MP4. On Linux canPlayType answers "" for H.264 **and AAC**: the build leaves the codecs out, so most MP4 video and audio won't play |
| eme | absent / absent | partly everywhere | ClearKey answers, Widevine is refused (`absent` means the call throws, and it doesn't) |
| autofill | absent / absent | partly everywhere | Saved passwords fill and save; no addresses or cards |
| context-menu | partly / works | works / works / works | Every right-click gets Go's menu; the row was never right |
| client-cert | unknown / unknown | partly everywhere | An optional certificate goes on without one (`clientcert` check); none can be picked |
| color-input | quiet / quiet | desktop UNSURE / works on Android | Android has the colour sheet |
| client-hints | quiet / quiet | works everywhere | Chromium's in-memory delegate keeps a site's hints for the session |
| pip-document | unknown / absent | **opens as a tab** on the Mac | After a click, the window came as a second tab |
| fedcm, web-otp | unknown / quiet | quiet (Mac seen) | Rejected, timed out |
| bluetooth | unknown / quiet | quiet / **absent** / quiet | `getAvailability()` false; on Linux `navigator.bluetooth` isn't there |
| background-fetch | unknown / unknown | quiet everywhere | Go denies its permission |
| speech-synthesis | unknown / works | Mac and Linux: no voices seen / works (92 voices) | To settle with `voiceschanged` |
| pdf | unknown / unknown | absent (`pdfViewerEnabled` false, Mac and Android) | What Open on a downloaded PDF shows is still to see |

### Rows right in status, with a wrong reason

credentials-password ("there is no password manager": there is one, it just doesn't answer this
API), orientation-lock ("needs full screen": on the desktop no orientation exists, as in Chrome),
pointer-lock ("the hook reaches Go": nothing in Go answers it; the page gets "UnknownError: If you
see this error we have a bug"), protocol-handler (the same), ai-prompt (the Mac and Linux get
content's test echo, which tells a page a model "can be downloaded"; nothing turns it off).

### Not rows, found on the way

- **The address bar shows a URL's password** (basics 8: `http://user:secret@…` shown as typed).
- Pop-ups open without a click (basics 6, the popup-block row already says partly).

## The plan, for his yes

Each fix is one commit with a test that would have caught it, and goes to release.

**1. Make the checks able to fail** (tests only, no Notes line):
- A probe states what its call gives on each platform (`"expect": {"mac": "rejected:NotAllowedError"}`),
  and the run compares the real answer, not only resolved or rejected. A row that starts working
  then fails its probe until its row is changed.
- A `basics:N` result agrees with its row: a pass under a row that isn't `works`, or a fail
  under one that is, fails `webapis`.
- Rows name the checks that already prove them: Android's `page_camera`, `page_dialogs`,
  `downloads`, `file_inputs`, `page_full_screen`, and the Mac's `clientcert` and `filling`.
- `webapis` and the audit probe run on Linux, against the platform they run on.

**2. A Linux column** (a Notes line: "On Linux, Settings' Web features says what Linux gives"):
`status.linux` on every row, `Platform()` answering "linux", and Settings showing it.

**3. Correct the rows** above, each with the probe or check that now holds it: one commit per row,
or per cause where several rows changed in one landing (dialogs and sign-in, for one). Settings
changes, so each carries a Notes line.

**4. Product bugs the audit found**, each his call, each its own branch:
1. Closing a tab skips the page's "Leave site?", so unsaved work is lost.
2. Android: a page allowed notifications shows none.
3. Linux: print does nothing; no H.264 or AAC, so most MP4 video and audio; no camera.
4. PDFs: no viewer; check what a PDF link and Open do.
5. The address bar shows a URL's password.
6. The Mac and Linux expose content's echo model as built-in AI. Turn it off, as its row says.
7. Document picture-in-picture opens as a tab.
8. mailto: and tel: links do nothing (the external-protocol row).

**5. Keep it honest after this:**
- At each Chromium bump, beside `web_features_bump.py`, diff Blink's `runtime_enabled_features.json5`
  for the rows' features and run the probes on the new build (a step in docs/chromium-upgrade.md).
- Later, as a spike: run web-platform-tests' directories for these features against our binary
  through ChromeDriver. Their testdriver grants permissions and drives WebAuthn and Bluetooth
  without a person, which tells works from quiet better than our own calls can. The first step
  is to prove that ChromeDriver attaches to our build.

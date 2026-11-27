# UX guide

How Project B behaves, on the Mac and on a phone. How it looks is
[design-system.md](design-system.md). Each rule says when the owner decided it, in his words
where he gave them, and grows as he decides more. A feature's own doc has the detail; this
guide has the rules every feature follows.

## Anything that takes time

- **Every progress has Retry and Cancel** while it runs, wherever it shows: a bar or a
  percentage, for the updater, a download, an import or anything else. A progress that stops
  moving for long enough ends as failed, with Try again. 2026-11-18, after the updater sat at
  "Downloading 0.2.85… 38%" overnight with nothing to press: "just have retry or cancel button
  whenever we show a progress, seems like a good starting rule to evolve from".
- **A download has Pause and Cancel** while it runs, and Retry only once it is cancelled or
  has failed. It can pause, and a silent one fails by itself after 30 seconds, so Retry on a
  running one only threw away what had arrived. 2026-11-27: "We only have pause and cancel
  and retry only when canceled or error".
- The buttons are quiet text buttons beside the progress, in place of what can't be pressed
  while it runs (Settings' Check for updates, for one).
- **Stalled means 30 seconds with nothing arriving**, the same for the updater, downloads and
  imports. The updater and an import read "Stopped: nothing arrived for 30 seconds". A
  download is watched by the engine itself, which tries a new connection first, then fails
  saying why in plain words ("Failed: the connection stopped"). A paused download never
  stalls ([daily-driver.md](daily-driver.md)).
- **Waiting on the person is not stalling:** while we wait for their answer, macOS's keychain
  question during an import, for one, no clock runs, the line says what it waits for, and
  Cancel still works.

## Settings

- **Only what works is shown.** A setting that needs work the engine can't do yet is not drawn
  as a row that does nothing; it waits in [todo.md](todo.md). A row a device can't use (Ask
  where to save, Tab layout, the keyboard shortcuts on a phone; Import on a phone, which can't
  read another app's data) isn't drawn there.
- **Sections, one showing at a time**, with search, and a phone reaches every part. 2026-11-18:
  "build both and a flag to switch… once i use it more i can be more sure"; 2026-11-20, the
  one page dropped: "i ahve used them and feel sections make better sense". Each section's name
  stays on one line on a phone. [settings.md](settings.md).
- **Where a choice lives:** each new place a feature shows itself is a switch in Settings, with
  a sensible default, rather than one place forced on everyone. 2026-11-18, on the four places
  bookmarks can show: build them all, with a setting for choosing.
  [bookmark-folders.md](bookmark-folders.md#where-bookmarks-show).
- **Sites by site:** stored data is listed and cleared by site (example.com, with its hosts
  inside), as Safari and Chrome do, since the engine erases cookies by site; clearing one host
  alone would sign the person out of its sister hosts.

## Panels, sheets and confirming

- **A panel from a button in the address field** (the star's, the key's) hangs from that
  button, at the star panel's width and padding. Enter saves; Escape or a press outside closes
  it. [bookmark-folders.md](bookmark-folders.md#what-the-person-sees).
- **One primary, on the right.** A choice of Save, Not now and Never has Save filled, Not now
  beside it, and Never as a quiet text button at the left.
- **Danger is red at the last step only:** "Clear all site data…" is an outlined button; the
  red fill is on the confirm in its dialog.
- On a phone the same panel is a sheet (Sheet.svelte), with the same fields and words.

## The phone

- **Within the thumb's reach:** what a person taps most sits in the screen's lower two thirds;
  the top third holds what is read, not pressed.
- **The bars hide on scroll** and the page never shows the frame colour where they were
  ([shells.md](shells.md#bars-that-hide-on-scroll)): they slide away as the page scrolls down
  and come back on a scroll up, as phone browsers do. 2026-11-19, over always shown and hiding
  once the page loads: "ok with you".
- **Our pages are tabs**, decided 2026-11-14 ([windows.md](windows.md#our-own-pages-are-tabs-decided-2026-11-14)).
- **Our own pages pinch, and never zoom by themselves.** A person who needs bigger words can
  pinch Settings and the rest as they pinch a web page, and Android's display size makes the
  whole app bigger. The app doesn't follow Android's font size yet; when it does, and our
  pages hold at 1.3 and 2.0, their scale can be fixed, as the phone shell's is. Until then
  the one thing stopped is Android zooming a page in around a field that takes focus while
  the keyboard is up, as a Dialog's next field does ([fragility.md](fragility.md)): a field
  takes no pinch (`design.css`), and Chromium doesn't zoom in around such a field. Decided
  2026-11-25.

## The first start

- **Show what we have built**: the shells as they really look, workspaces and the keys.
  2026-11-18: "it should show the stuff we have built so shell and keyboard stays".
  [onboarding.md](onboarding.md).

## Mocks

- Two to four options, each with why it is good and what it costs, and a recommendation.
- Line icons from the design system, never emoji.
- Example sites and names only (example.com, News, Recipes): no real company in a mock, a test or a commit.

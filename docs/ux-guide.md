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
- Retry throws away what has arrived, so it sits last and a little apart from Pause and
  Cancel, where a slip doesn't land on it.

## Settings

- **Only what works is shown.** A setting that needs work the engine can't do yet is not drawn
  as a row that does nothing; it waits in [todo.md](todo.md). A row a device can't use (Ask
  where to save, Tab layout, the keyboard shortcuts on a phone; Import on a phone, which can't
  read another app's data) isn't drawn there.
- **One page by task, or sections**, chosen in Look's Settings row and switched live; one page
  is the default. 2026-11-18: under Look, not in Flags. Both have search, and a phone reaches every part. 2026-11-18: "build
  both and a flag to switch… once i use it more i can be more sure". [settings.md](settings.md).
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
  ([shells.md](shells.md#bars-that-hide-on-scroll)). Whether the bars should instead stay fixed,
  or float over the page, is with the owner.
- **Our pages are tabs**, decided 2026-11-14 ([windows.md](windows.md#our-own-pages-are-tabs-decided-2026-11-14)).

## The first start

- **Show what we have built**: the shells as they really look, workspaces and the keys.
  2026-11-18: "it should show the stuff we have built so shell and keyboard stays".
  [onboarding.md](onboarding.md).

## Mocks

- Two to four options, each with why it is good and what it costs, and a recommendation.
- Line icons from the design system, never emoji.
- Example sites and names only (example.com, News, Recipes): no real company in a mock, a test or a commit.

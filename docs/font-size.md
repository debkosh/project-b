# Android's font size

Android's font size (Settings > Display > Font size) reaches pages as Chrome's own Android
layer passes it. Sudipta, 2026-11-25, chose Chrome's way over Android WebView's.

## How Chrome does it, on Chromium 158

- Chrome reads `Configuration.fontScale` at each foreground start and when it changes, and
  sets the page preference `font_scale_factor` to it
  (`components/browser_ui/accessibility/android/.../FontSizePrefs.java`,
  `chrome_content_browser_client.cc`). Blink keeps it as `accessibilityFontScaleFactor`.
- Blink grows text by it only on a page that asks, with
  `<meta name="text-scale" content="scale">`. There the default font size, and so `rem`,
  grows by the scale (snapped to Android's steps), and `env(preferred-text-scale)` gives it.
  A page that doesn't ask stays as it is, and pinch zoom goes further.
- The old text autosizer, which grew text on every page, is gone from 158. Page zoom's
  adjustment for the OS font size shipped off (`AdjustForOSLevel=false`). Android WebView
  instead grows all text on a page without the tag (`scale_all_fonts_if_no_meta_text_scale_tag`).
  We don't: it breaks sites that size their layout in px.

## What we do

- **Engine.** `BrowserWindow` hands the engine `fontScale` when it starts, before the
  first page, and again from `BrowserActivity.onConfigurationChanged`, which takes
  `fontScale` without the activity starting again. `Engine::SetFontScale` has every page,
  shell and tabs, work out its preferences again, and
  `ContentBrowserClient::OverrideWebPreferences` sets `font_scale_factor` from it. Desktop
  platforms leave it at 1.
- **Our pages** ask, with the meta tag, and size their text in `rem`: every page Android
  shows (Settings, Welcome, History, Bookmarks, Downloads), the picker with them though
  only the desktop shows it, the shells Android runs (Standard and the phone's), the
  shared `ui/` and `extensions/` components and `design.css`. A size is written `calc(13rem / 16)` for 13px, which reads as the size it
  was and is 13px at the normal font size. Line heights given in px are written the same
  way. Shells only the desktop runs keep px.
- **The bars are held at 1.3 times**, and icons beside text grow up to that much, through
  `var(--capped-rem)`: ux's call, 2026-11-25, as chrome in fixed space crowds for little gain.
  Pages, sheets and dialogs grow all the way. The root keeps the browser's medium, with the
  14px base on `body`, so 1rem is 16px at the normal size.
- **Boxes around text** grow with it: a row's height is a `min-height`, or padding, not a
  fixed `height`, wherever text sits in it. A summary that would end in an ellipsis wraps to
  two lines first. The rules for new pages are in
  [design-system.md](design-system.md#androids-font-size).
- **The native phone shell** sizes its text in `sp`, which Android scales itself.

## Checking it

`shell/e2e/font-scale.cjs` (in `tools/test.sh mock`) fails on a px font size or line height
in any of the files above. It then opens the phone's shell and Standard on a touch tablet
at 1, 1.3 and 2, as DevTools emulates Android's font size (`Emulation.setEmulatedOSTextScale`,
which Blink takes as it takes `font_scale_factor`). For the bars, Welcome, Settings, History,
Bookmarks, Downloads and the page dialogs, it checks that the root is 16px and our text 14px at 1,
that our text grew by the scale, that the address in the bar grew to 1.3 times at most,
and that no line of it is cut off by a box that hides what overflows it. A box that scrolls,
an ellipsis or a line clamp cuts on purpose. It leaves screenshots in
`build/e2e/font-*.png`. On a phone, `adb shell settings put system font_scale 1.3` sets
the size, and the page follows it without a restart.

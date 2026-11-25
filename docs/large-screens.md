# Android tablets, foldables and desktop mode

A spec, 2026-11-22, agreed 2026-11-25. Nothing here is built yet. It turns the section
"Android tablets, foldables and desktop mode" of [research/platforms.md](research/platforms.md)
into what we would build, what we would leave, and how we would test it.

The short version: the same APK. A narrow window keeps the Mobile shell; a wide one gets the
Standard shell, with the mouse, the touchpad and a hardware keyboard working as on the Mac.
No new build work, and no new system images: three new emulators made from the images and
device profiles this Mac already has.

## Where the app is today

| | Today, on `main` |
|---|---|
| Resizing | Already taken in place. The manifest lists the size, layout and density changes (`AndroidManifest.xml.jinja2`), so the activity isn't recreated, which would end the process. `BrowserWindow.onSizeChanged` tells Go the new size. Never tried on a window that changes size often |
| Which shells | Go knows it is on a phone at build time (`shells.OnPhone = GOOS == "android"`), lists only the `phone` shells, and loads Mobile even where the profile names Standard. The APK carries only the Mobile shell's pages (`engine/build_go.py`, `PHONE_SHELL`) |
| Touches | Go's regions decide: a finger down on a shell region goes to the shell, the rest to the page (`BrowserWindow.inShell`) |
| Mouse and touchpad | Not routed. Hover and the wheel go to whatever Android picks, which is the shell's view, since it covers the window |
| Hardware keyboard | Typing works in the page and the shell. Our shortcuts (`core/commands`) aren't wired: `set_menu` and `app_command` do nothing on Android |
| Windows | One. `onDestroy` ends the process |

## What we would build

### 1. The shell follows the window

**A narrow window shows the Mobile shell, a wide one the Standard shell**, decided again each
time the window settles on a new size.

- **Narrow** is a window under 600 dp wide or under 480 dp tall (Android's compact width and
  compact height); **wide** is the rest. The height keeps a phone turned sideways (891 by 412
  dp) on Mobile. A window being dragged in desktop mode switches only 40 dp past either line,
  so it doesn't flip back and forth at the edge (ux, 2026-11-24).
- **It switches once the size has settled**, half a second after the last change, so dragging
  a desktop window's corner doesn't reload the shell on every step.
- **Switching is the reload shells already do** ([shells.md](shells.md#choosing-a-shell)): the
  tabs keep running, the new shell says `ready` and gets the snapshot. A foldable opened mid-page
  keeps its page, scroll and form.
- So: a phone stays on Mobile, upright or sideways. A foldable is Mobile folded and Standard
  open. A tablet is Standard both ways round, so turning it never changes the browser. A
  desktop-mode window is whichever its size says.
- **In Go:** `shells.OnPhone` became `shells.OnAndroid`, the platform, and `shells.Narrow`
  the rule. Go already hears the window's size in dp (`WindowState`), so the engine is
  unchanged: `app.shellPick` decides from it, the first size at once and a later one once
  it has settled, and the windows load the shell it says. The APK carries the Standard
  shell's pages too (`ANDROID_PAGES` in `engine/build_go.py`, under 300 KB). The mock plays
  Android with `-platform=android`, and `shell/e2e/large-screens.cjs` resizes it across
  the line.

What Standard needs on Android, all of it already true of Standard in a Mac window at full
screen: no window buttons (the system's caption bar has its own in desktop mode, which fits
"the shell draws the window buttons only when the window has no title bar of its own",
2026-11-19), no drag areas, square corners. With no menu bar, Go's menus are reached from
Standard's own Menu button (`AppMenu.vue`), as on Linux and Windows with the menu strip off.

**Why Standard, of the desktop shells.** It is the reference: every Go feature lands there
first and its tests already run, so large screens add no shell to keep complete. Fox looks the
most like a tablet browser, tabs on top, but has no tab menu, pinned tabs or bookmarks bar, and
at 841 dp its address field covers Back. Curve adds spaces and a command bar; Terminal and
Minimal are keyboard-first. Mocks of all five: [the mocks](https://claude.ai/artifact/EdgkSnyUs1Xo8WcaemfTSJ).

**Standard's touch density** (ux, 2026-11-24): one switch on `(pointer: coarse)` with no fine
pointer, not a layout. Sidebar rows grow from 32 to 44 to 48 dp, toolbar buttons get a 44 dp
target with the icons the same size, the active tab's close button always shows, the shortcut
hints go, and a long press opens the menus a right click opens. A mouse brings today's density
back. It works the same on a Mac with a touch screen, so it is Standard's, not Android's.

### 2. The mouse and the touchpad

- **Hover and the wheel go by Go's regions**, as touches do: over a shell region to the shell,
  otherwise to the page. `ShellHolder` answers generic motion as it answers a finger.
- **A right click** opens the context menu the shell draws for a mouse, not the long-press
  sheet, by the event's source.
- **Pointer shapes** (the hand over a link, the text bar) come from content, as on the Mac.
- **Pinch on a touchpad** zooms the page, as content already does.

### 3. The hardware keyboard

- **Go's shortcuts run on Android**, with the Linux keymap (Ctrl, not Cmd), which `commands.OS`
  already picks for any OS that isn't the Mac or Windows. The activity asks Go about a key
  before the page sees it, as the Mac's menu does: a key that is a shortcut runs its command,
  any other goes on to the page or the shell.
- **Meta stays Android's.** Android's own shortcuts (Meta+Tab, Meta+/) are left alone.
- **Settings, Keyboard** shows on Android when a hardware keyboard is connected.
- The shell's menu items show their keys when a keyboard is connected, else not.

## What we wouldn't do

- No several windows for now: one window, as on a phone. New Window stays out of the menus on
  Android.
- No tablet layout of the Mobile shell, and no third shell: a window wide enough gets Standard.
- No other desktop shells offered on Android at first (Curve, Terminal, Fox, Minimal). They
  would work as Standard does; offering them is a list away once Standard works.
- No drawing into desktop mode's caption bar. Android 15 lets an app draw there, as our
  shell does beside the Mac's buttons; it can come later.
- No change to packaging, signing or updates.

## Testing

**Three emulators, from what is installed.** All from the Android 17 image
(`system-images;android-37.0;google_apis;arm64-v8a`) and device profiles the SDK ships; nothing
to download. The tester makes and owns them:

| AVD | Device profile | For |
|---|---|---|
| `project-b-tablet` | `pixel_tablet` | Landscape Standard, portrait in between, keyboard and mouse |
| `project-b-fold` | `pixel_fold` | Folding and unfolding mid-page (the emulator's fold control) |
| `project-b-desktop` | `pixel_tablet`, with desktop windowing switched on in its developer options | Freeform windows: resizing, the caption bar |

If the Android 17 image won't turn on desktop windowing, the `desktop_medium` profile with the
same image is next; a desktop system image would be a download, which this spec doesn't ask for.

**The driver.** `tools/drive_android.py` gets a large-screen part, run on these three only:

- the shell for each size: start wide, Standard; resize narrow, Mobile; wide again, Standard,
  with the same tabs and the page's scroll kept (window size by `adb shell wm size`, the fold by
  the emulator console's fold command, a desktop window by `am task resize`);
- mouse: hover a shell button and see its hover state, wheel over the page and see it scroll,
  wheel over the bar and see the page stay, right click a link and see the shell's menu
  (`adb shell input mouse` and `input scroll`);
- keyboard: Ctrl+T opens a tab, Ctrl+W closes it, Ctrl+L puts the caret in the address field,
  Ctrl+Tab moves on, typing in a page isn't taken (`input keyevent --meta`);
- Go's tests for the class rule (wide, narrow, in between, the settle delay), and
  `tools/test.sh mock` with the mock's window resized across the line.

The phone runs stay as they are. This changes the phone shell's choosing, so per
[roles/feature.md](../roles/feature.md) the full `tools/test.sh android` runs on emu-37 and
emu-29 before handover, with slots from the tester.

## Order and size

1. The shell follows the window, with the Standard shell in the APK: about 4 days.
2. Standard's touch density: about 2 days.
3. Mouse and touchpad: about 2 days.
4. Keyboard shortcuts: about 2 days.
5. The emulators and the driver's large-screen part, alongside: about 2 days.

## Decided

- **Standard on a wide window, for now** (2026-11-24: "ok for now lets go with stands"; "lets
  go with standard as an option whne the phone is wide"). An iPad, if we ever build for
  iPadOS, gets Standard too.
- **A sidebar on demand, on tablets and desktops** (2026-11-24: "a collapbsable side bar which
  open on some clikc or shortcuts and closes on any interactions or page or tool bar click...
  this is nice to have in desktop too"). The tabs stay in the sidebar, closed until the
  sidebar button or its shortcut opens it over the page; the next tap or click on the page or
  the toolbar closes it. Standard already has tabs on top as a setting, which he felt suits a
  tablet ("i thing top bar would make more sense on tablets").
- **Standard keeps its name; Glide, the Arc-like shell, is now Curve** (2026-11-24: "lets
  keep stamdard and change glide to  curve or aomething else meaning arc"; "curve it it").
- **Shell ids are code names, apart from the names** (2026-11-24: "lets name the ids like
  ... code name like crshell arshell fxshell pbshell mwshell mnshell"): `pbshell` Standard,
  `arshell` Curve, `tmshell` Terminal, `fxshell` Fox, `mnshell` Minimal, `mwshell` Mobile,
  `mkshell` Mobile (native), with each name and description written in the registry only.
  Done on its own branch, `shell-ids`, before this work; the shells below are named by
  those ids from then on.
- **The shell follows the window, by size** (2026-11-25: "rest is fine"), with Settings
  letting a profile pin one shell for every size.
- **`shell` is the wide window's shell, a new `phoneShell` the narrow one's**, with no
  migration: a phone on Mobile (native) shows Mobile once until chosen again. `oneShell`
  keeps one at every size, `phone` or `wide`; unset, the size chooses.
- **Standard is the one desktop shell on Android**, with the touch density.
- **No several windows for now** (2026-11-25: "no multi window for now").

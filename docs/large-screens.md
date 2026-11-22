# Android tablets, foldables and desktop mode

A spec for discussion, 2026-11-22. Nothing here is built or agreed. It turns the section
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

- **Wide** is a window at least 840 dp wide (Android's "expanded" width), or at least 600 dp
  with a mouse or touchpad connected. **Narrow** is less than 600 dp. Between the two, the
  shell already showing stays, so a window dragged about its edge doesn't flip back and forth.
- **It switches once the size has settled**, half a second after the last change, so dragging
  a desktop window's corner doesn't reload the shell on every step.
- **Switching is the reload shells already do** ([shells.md](shells.md#choosing-a-shell)): the
  tabs keep running, the new shell says `ready` and gets the snapshot. A foldable opened mid-page
  keeps its page, scroll and form.
- So: a phone stays on Mobile. An unfolded foldable gets Standard, folded Mobile. A tablet in
  landscape gets Standard; in portrait (about 800 dp) it keeps whichever it had, Mobile from
  a fresh start. A desktop-mode window is whichever its size says.
- **In Go:** `shells.OnPhone` stops being a build-time fact. The engine tells Go the window's
  class, wide or narrow, with its size; the shell manager picks the profile's shell for that
  class. The APK carries the Standard shell's pages too (about 1 MB).

What Standard needs on Android, all of it already true of Standard in a Mac window at full
screen: no window buttons (the system's caption bar has its own in desktop mode, which fits
"the shell draws the window buttons only when the window has no title bar of its own",
2026-11-19), no drag areas, square corners. With no menu bar, Go's menus are reached from
Standard's own Menu button (`AppMenu.vue`), as on Linux and Windows with the menu strip off.

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

### 4. Several windows (a second step)

A window of ours is one activity, in the one process, each with its own Go window.
"New Window" opens another activity as a new task (`FLAG_ACTIVITY_NEW_DOCUMENT`); closing one
closes its window, and the last one closing ends the process, as now. This is a week of its
own, and step 1 to 3 don't need it, so it waits until they are out.

## What we wouldn't do

- No tablet layout of the Mobile shell: a tablet wide enough gets Standard, and Mobile at
  600 to 840 dp is the phone layout, wider. Ux may want it centred or capped in width there.
- No other desktop shells offered on Android at first (Glide, Terminal, Fox, Minimal). They
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
| `project-b-desktop` | `pixel_tablet`, with desktop windowing switched on in its developer options | Freeform windows: resizing, the caption bar, several windows later |

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
2. Mouse and touchpad: about 2 days.
3. Keyboard shortcuts: about 2 days.
4. The emulators and the driver's large-screen part, alongside: about 2 days.
5. Several windows, after 1 to 4 are released: about a week.

## Questions for Sudipta

1. **Automatic, or the user's choice?** Recommended: automatic by size, as above, with Settings
   letting a profile pin one shell for every size. The other way is a setting only, Mobile by
   default.
2. **Which pref holds what?** A profile's `shell` pref today means "the desktop shell" on a Mac
   and "the phone shell" on Android. Recommended: `shell` is the wide window's (Standard, as
   the templates already say), and a new `phone_shell` the narrow one's (Mobile, or Mobile
   (native)). A phone that chose Mobile (native) would show Mobile once, until chosen again;
   no migration code.
3. **The line.** Recommended: Mobile below 600 dp, Standard from 840 dp, or from 600 dp with a
   mouse, and in between the one showing stays. Simpler: one line at 600 dp.
4. **Several windows** as a second step after 1 to 3 are released, or in the same branch?
   Recommended: a second step.

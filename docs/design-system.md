# Design system

How every piece of our own UI looks: the shell, the settings page, and any app added under
`shell/src/apps/`. Read this before drawing anything. Web pages are not ours and get none of it.

## Where it lives

| File | What it holds |
|---|---|
| `core/tones` | Every colour, made from the one frame colour Go owns, light and dark. Go sends both palettes with the colour (`tones`, beside `windowColor`), and each workspace's with it, so web and native shells paint the same tones |
| `shell/src/theme.js` | Puts the light or dark palette on the root as CSS variables, as the theme pref and the system say |
| `shell/src/design.css` | Type, the floating shadow, the focus ring, and the shared controls: `.icon-button`, `.text-button` with `.primary`, `.quiet` and `.danger`, `.text-field`, `select`, `.switch`, `.tag`, `.secondary`, `.text-field-error`, and their touch sizes |
| `shell/src/ui/tint.js` | The colours made up for an item, from gulal: one environment per background, `--bar` or `--surface`, made again each time the theme is painted |
| `shell/src/ui/hues.js` | The colours of what the user makes, a workspace or a group: the seven hues Go sends, `paint` for a kept colour, `next` for a new one |
| `shell/src/ui/customColour.js` | The Custom colour sheet a + opens on a phone, with no framework in it, so Vue and Svelte pages open the same one |
| `shell/src/ui/` | The shared parts that need behaviour: `Dialog`, `RowMenu`, `Field`, `Segmented`, `Swatches`, and `escape.js`, which gives Escape to the innermost thing open |
| `shell/src/apps/settings/SettingsApp.vue` | The settings page's own layout: `.nav-item`, `.card`, `.row`, `.label`, `h2` |
| `shell/src/extensions/ExtensionsToolbar.vue` | The toolbar's floating menu (`.pbx-menu`) |

Each app's `main.js` imports `@/design.css` before mounting. A class two apps need goes into
`design.css`, never into one app's global style, and a part with behaviour two sections
need goes into `shell/src/ui/`, never copied. That is how the install prompt once drew
browser-default buttons: it used `.text-button`, which only the settings page defined, and the
shell shows the prompt over a web page, where that class did not exist.

## The rules

From the owner:

- **Flat first.** Hairline borders, no gradients, no shadows except on what floats.
- **A 4px grid** for sizes and spacing. Radii are 4, 6 and 8 only, plus 50% for round things.
- **Minimal and slim.** The density is a desktop browser's: not zoomed in, not cramped.
- **No colours of ours on pages.** Web pages keep Chromium's and the system's defaults.
- **No dummy controls.** Every control on screen works.
- **No explanatory prose.** No helper sentences where the design has no place for them.
- **The frame colour is the one thing in full accent.** Icons take a deeper ink.
- **Don't copy Chrome by reflex.** Chrome's behaviour is a reference, not its look.

## Colour

The colours come from the profile's frame colour and theme, which are Go's prefs, not a
shell's ([shells.md](shells.md#colours)).

Never write a colour literal in a component. Use these variables, which Go's tones set:

| Variable | Use |
|---|---|
| `--frame` | The window frame and the empty strips of the shell. The colour the user picked |
| `--accent` | Primary buttons, a switch that is on, a pinned item, focus rings |
| `--on-accent` | Text or an icon on `--accent`: a primary button's words, an extension's badge |
| `--accent-soft` | Pressed or selected: the current settings section, an open popup's button |
| `--accent-tint` | Hover on any control, an open row's details |
| `--bar` | The toolbar and the settings page's background |
| `--surface` | Cards, menus, dialogs, text buttons |
| `--text` | Body text |
| `--muted` | Secondary text: section headings, versions, descriptions, quiet buttons |
| `--icon` | Icons |
| `--line` | Every hairline border and divider |
| `--hover`, `--press` | The toolbar's own hover and press |
| `--danger` | Errors, and a destructive answer or menu item |
| `--on-danger` | Text on `--danger`: a Delete button's words |

Text on `--accent` is `--on-accent`, never a fixed colour: the accent's lightness follows the
frame, so white reads on slate and not on amber, and the near-black ink the other way round.
`core/tones` asks [gulal](https://github.com/debkosh/gulal) for the accent's ink, the one of
the two that contrasts more **by APCA**, the perceptual contrast of WCAG 3's draft, and holds it
to **Lc 60**, the level we hold every label to (decided 2026-11-18): colour decisions are gulal's.
WCAG 2's ratio, used before, gave the ink on mid tones such as the Teal and Blue presets, where
white reads better to the eye (the owner, 2026-11-18: the Save button's label was wrong on
teal). Where neither reaches Lc 60 (the teal preset in dark) it moves the accent away from the
label it picked, a little at a time, until it does; so the accent can differ slightly from the
frame. `core/tones`'s tests and `shell/e2e/settings.cjs` check every preset, light and dark.

**A colour made up for an item** (a site's letter tile, a bookmark folder's mark, anything
coloured from a seed or its text) comes from [gulal](https://github.com/debkosh/gulal), through `shell/src/ui/tint.js`,
never a literal or a list of our own. Give gulal the place it sits in, from the tones: the
scheme, the `--bar` or `--surface` it sits on as its background, and the frame colour as its
accent, none for None. The same text then gets the same colour
in that place every time, in Go and in the shells, as strong as the theme, and its inks are held
to APCA Lc 60. Decided 2026-11-18, after the letter tiles' random colours sat badly with the
theme.

**gulal keeps away from `--danger`,** the theme's own, at gulal's 25° either side, so no
site, group or folder reads as an error: about hues 4° to 54° in OKLCH, the reds and
red-oranges. Nothing else is avoided: the accent is leaned toward, and `--danger` is our only
status colour. Every environment `tint.js` and Go make passes it as `avoid`. Decided by ux
2026-11-19, as the owner asked ux to.

**Made up, or picked.** The owner, 2026-11-19: what the user didn't make (a site, its letter
tile) is coloured from its text, as above. What the user makes and names (a workspace, a tab
group) takes the colour they pick: the picker offers gulal's `distinct(7)` for its place as
swatches, then a + for any colour, and a new one starts on `next(existing)`, the suggestion
farthest from those in use. A suggestion is kept as its hue and drawn by gulal for the theme,
so it is the same hue in light and dark; a colour the user chose with + is kept as they chose
it, never moved.

How it is built (`core/tones/hues.go`, `shell/src/ui/hues.js`):

- **The seven** are gulal's `distinct(7)` started from hue 0, not from the frame's hue as
  gulal's own start would be, away from both schemes' `--danger`: 0, 181, 90, 270, 135, 225,
  315, named Rose, Teal, Olive, Indigo, Green, Blue and Purple. Starting from 0 keeps them
  the same seven whatever the frame, so a kept hue is always one of the swatches.
- **Kept** as the hue in whole degrees, `"181"`, or a picked `"#rrggbb"`. Go refuses
  anything else for a group or a workspace.
- **Drawn by Go:** the tones carry `hues`, each `{hue, name, light, dark}`, gulal's
  `hue(h)` as a mark on `--bar` with the frame as its accent. `paintTheme` puts each on the
  root as `--hue-181`, and a shell paints a kept colour with `paint(c)`; the native shell
  reads the same list. The frame strengthens or quietens them; their hues never move.
- **Next:** the swatch used least, then the one whose hue is farthest from those in use,
  picked colours counting by their hue (`tones.Next`, and `next` in `hues.js` for a dialog
  that shows it before Go makes the item).

Check light and dark with four frame colours: the default (the logo's teal, `tones.Brand`), None (a grey frame with ink for the
accent), a dark one, a light one and a mid one such as `#4f9e9e`. `core/tones` must keep every pair readable.

## Type

One family, the system font, 14px with a line height of 1.4, set by `design.css`: the
family and line height on the root, the 14px on `body`.

| Size | Weight | Use |
|---|---|---|
| 16px | 600 | A page's title (settings' header) |
| 15px | 600 | A dialog's title |
| 14px | 400, 500 | Body; 500 for labels and buttons |
| 13px | 400, 500 | Menus, lists, details; section headings at 500 in `--muted` |
| 12px | 400 | Secondary lines: versions, descriptions |
| 9px | 600 | An extension's badge only |

No monospace in the UI. Code-like strings (a permission name, a host) are body text.

### Android's font size

Our pages follow it, as a site that asks Chrome to does ([font-size.md](font-size.md)):

- **Sizes in rem.** Every font size and line height is written in rem, as
  `calc(13rem / 16)` for 13px, never in px, which would stay put. The root keeps the
  browser's medium, so 1rem is 16px at the normal size. `shell/e2e/font-scale.cjs` fails on
  a px size.
- **Pages, sheets and dialogs grow all the way**, to twice the size at Android's largest.
- **The bars are held at 1.3 times**: the address bar, the tabs and sidebar, the bookmarks
  bar, the find bar and the notice bars, chrome in fixed space, as Chrome's own toolbar
  barely follows the font size. Their text is in `var(--capped-rem)`, 1rem held at 1.3 times
  16px, written `calc(13 * var(--capped-rem) / 16)`.
- **Icons beside text grow with it up to 1.3 times**, in `var(--capped-rem)` too, and a tile
  around one with it. Icons alone in a bar stay their size.
- **A row grows with its text.** Its height is a `min-height` or padding, never a fixed
  `height` around text. A line that ends in an ellipsis at the normal size, such as a row's
  summary, wraps to two lines first (`-webkit-line-clamp: 2`), so a larger size shows the
  words rather than fewer of them. A name or a title stays on one line.

## Sizes and spacing

| Thing | Size |
|---|---|
| Icon button | 28px square, radius 6, icons 16px |
| Text button | 30px high, 12px side padding, radius 6 |
| Settings nav item | 34px high, radius 6 |
| Menu item | 32px high, radius 4 |
| Settings row | at least 48px high, padding 8px 16px |
| Gaps | 4px between toolbar buttons, 8px between buttons in a group, 12px between an icon and its text in a header |
| Touch | buttons, menu rows and segmented controls grow to 44px; hover does nothing. Standard on a touch screen with no mouse does the same in its own bars ([large-screens.md](large-screens.md)) |
| A phone | every control a finger presses is at least 48px each way, Android's least (48dp; a CSS pixel is a dp on a phone). What it draws may be smaller inside its target, as a 32px chip in a 48px row, or a 40px button in a 48px target. The Mobile shell's tests check every control on each screen (`shell/e2e/shells/mwshell.cjs`) |

## Motion

The owner, 2026-11-14: animations are very quick, about 100 ms, and smooth.

- **Every transition** uses `var(--motion) var(--ease)` from `design.css`: 100 ms on
  `cubic-bezier(0.2, 0, 0, 1)`, which starts fast and settles. No other durations.
- **Animate only what helps you follow a change:** something opening or folding (the
  sidebar, a group, a switch's thumb), a cross-fade from the splash to the UI, the load
  progress. A menu or a dialog appears at once; hover and press colours change at once.
- **Opacity and transforms** over size where it can be, so nothing reflows while it moves.
- **Less motion** when the system asks for it (`prefers-reduced-motion`): `--motion` is 0,
  and things change at once.

## Surfaces and elevation

- **Flat:** the bar, the settings page. No border, no shadow.
- **Card:** `--surface`, a `--line` hairline, radius 8, rows divided by hairlines.
- **The page as a card in the frame** (Curve): its corners are concentric with the window's,
  the window's radius less the inset (26 − 8 = 18 on macOS 26), and 8 when the window's
  corners are square, as in full screen. Radius 8 everywhere else.
- **Floating:** a menu, the address bar's suggestions, a dialog. `--surface`, a `--line`
  hairline, radius 8, and `var(--shadow-float)`. A modal dialog's backdrop is `--bar` at 70%.
- **Focus:** `var(--focus-ring)` on `:focus-visible`, never the browser's outline. On a menu
  item or a row it is `var(--focus-ring-inset)`.

## Layers

What lies over what, in every shell:

1. **The page's area is a layer of its own** (`isolation: isolate` on its box: Standard's
   `.main`, Fox's `.page`, Terminal's pane `.body`, Mobile's and Minimal's `main`). What lies
   over the page, its notice (crashed, failed, blocked), the find bar, a page's dialog, the
   command box's scrim, stays inside it and never rises over the shell.
2. **The shell's bars** lie over the page's area.
3. **The shell's menus, panels and bubbles** (the ☰ menu, the site panel, the bookmark and
   password panels, the downloads bubble, right-click menus) lie over everything, wherever
   in the shell they are drawn.

Inside the page's area, the corner bubbles (the update bubble, the last run's crash) lie over
the page's notice. A z-index only orders things within its own layer, so a new overlay of the
page goes in the page's box, and a new popup anywhere in the shell. 2026-11-18, after the ☰
menu opened under a crashed tab's notice.

## Controls

Use these; never leave a `<button>`, `<select>`, `<input>` or radio to the browser's look.

| Need | Use | Not |
|---|---|---|
| An icon alone | `.icon-button` with an `aria-label` | |
| Words | `.text-button`; `.quiet` inside a row | |
| The one answer a dialog wants | `.text-button.primary` | a primary button on a row |
| A destructive answer (Delete, Discard, Remove) | `.text-button.danger`, in a dialog | |
| On or off | `.switch`, named by an `aria-label` bound to the words its label shows, with the line under it as `aria-describedby` | a checkbox; a name through `aria-labelledby` or a wrapping label. One way to name a switch, which the mobile walk checks; it also reads on an engine whose Android names from a label TalkBack doesn't speak, as 155's didn't (2026-11-19) |
| Picking some items of a list (which sparks to export) | a checkbox per row, its `accent-color` `--accent`, with an All box above them, mixed while only some are ticked | a switch, which says a setting is on |
| Two to four short choices | `Segmented` (Theme: System, Light, Dark) | radio buttons |
| Five or more choices, or long ones | `select` | |
| A colour | `Swatches`: gulal's suggestions, and with `custom` a + for any colour: the system's picker on a desktop, our Custom colour sheet on a phone ([On a phone](#on-a-phone)) | a bare colour input; Android's own picker |
| A name, a number, a value | `Field` (`.text-field`); empty means Off where Off makes sense | a section's own input style |
| A row's actions | `RowMenu`: a ⋯ button and its menu | a row of quiet buttons |
| A label on an item (Hidden, Locked, Not in this build) | `.tag`, a hairline outline | a filled pill |
| A link in a line of text that opens something on the page | `.link` | |
| A second line under a label | `.secondary` | a section's own class |
| A question | `Dialog` | `confirm()`, or a section's own `<dialog>` |
| A password or passphrase | a password `.text-field` with Show, a quiet text button inside its right end that turns to Hide | an eye icon outside the field |
| A line under a field that has an action (a strength, Suggest one) | `.secondary` on the left, one `.link` on its right | a button under the field |

## Patterns

From the settings audit of 2026-11-13 (owner approved). Every section follows them.

### Saving

- **Everything saves the moment it changes.** No Save buttons, and no select that waits for
  a button.
- **A text field saves on Enter or when you leave it** (`Field`). Escape puts the saved
  value back and leaves the field; it never closes the page from inside one.
- **A choice that throws something away goes inside the confirm dialog,** not in a select
  on the row.
- **What applies only after a relaunch** says so on its row, and one Relaunch button at
  the top of the section applies them all.

### Lists

- **A list is a `.card` of `.row`s.** A row: a leading mark (a colour dot, an icon), the
  name as text, an optional `.secondary` line, then at most one inline control (usually a
  switch) and a `RowMenu`. Names are never always-open fields.
- **The row's menu holds every action on it:** Edit…, Move up and Move down where order
  matters, Details, then a separator and the destructive one last, in `--danger`.
  Right-clicking the row opens the same menu.
- **A list of secrets is the exception: two quick actions on every row.** A saved password's
  row carries Show (an eye, struck through for Hide) and Copy (two squares) as icon buttons
  before its `RowMenu`, on a Mac and a phone alike, so one press shows or copies; the
  username's copy and Delete… stay in the menu. Shown, the secret goes under the username in
  monospace. The owner's ask, 2026-11-20: "too many clicks to see a password".
- **The device's check holds while its page is open.** Showing or copying a secret asks for
  Touch ID, the Mac's password or the screen lock at the first press, then not again for 5
  minutes while the page stays open; leaving the page ends it, and a shown secret hides when
  the hold ends. The page's intro says so.
- **Adding:** a "New …" button on the right of the list's heading row, in the same place
  for every list. Never a form row after the items.
- **Create and edit share one `Dialog`** with the item's fields, titled "New workspace" or
  "Edit workspace". Enter saves, Escape cancels.
- **A new item gets the next colour no other item has.** Names are unique in a list; a
  duplicate is refused under the field.
- **An empty list is one muted row:** "No extensions installed". The add button stays where
  it is.
- **A long name is one line with an ellipsis,** the full name in its tooltip.

### Rows and headings

- **A row** is at least 48px high, padding 8px 16px: the label on the left, the control on
  the right.
- **The nav names the section; the content never repeats it.** An `h2` names a group of
  rows, and a section with one group has none, unless it is a list whose heading row
  carries its New button (Workspaces). A heading row may carry one action on its
  right, and is 30px high with or without it.
- **More under a row:** the row is the button, with a chevron on its right that turns
  when it opens. The panel sits under the row on `--accent-tint`. Escape closes it.

### Context menus

The owner, 2026-11-14: a right-click or long-press menu is drawn by the shell, and Go
supplies what was pressed and the actions it can run. It is the floating menu `RowMenu`
draws, opened at the pointer, so the shells that want the default look share
`shell/src/ui/ContextMenu.vue` and its styles, `.ui-menu` and `.ui-menu-item`.

- **The surface:** `--surface`, a `--line` hairline, radius 8, padding 4,
  `var(--shadow-float)`; 220px wide at least and 320px at most, a long label ending in an
  ellipsis (selected text is quoted to about 24 characters).
- **An item:** 32px high (40 on touch), 12px side padding, radius 4, 13px `--text` in
  sentence case. Hover `--accent-tint`; keyboard focus `--focus-ring-inset`. An icon is
  optional, 16px in `--icon`, 10px from the label; when one item has one, the others keep
  its room so the labels line up. A key shows at the right, 12px `--muted`.
- **Disabled** items stay, at half opacity, skipped by the arrow keys. A **danger** item is
  `--danger`, last, after a separator.
- **Separators** group items: a `--line` rule with 4px above and below; never first, last,
  or two together.
- **A submenu** row ends in a chevron and opens to its right, top aligned with the row,
  after a short hover or at once on click or the right arrow; it flips left without room.
  A workspace's item carries its colour dot as its icon.
- **Placement:** at the pointer, flipping left or up to stay 4px inside the window. It is
  the shell's region while open, as it floats over the page.
- **Keyboard:** opened by the mouse, nothing is highlighted until a key or the pointer
  moves; opened from the keyboard, the first item has focus. Up and down wrap, Home and End
  jump, Enter and Space run, right opens a submenu and left closes it, a letter goes to the
  next item starting with it. Escape closes the innermost menu (`ui/escape.js`), Tab all
  of them; so do a press outside, the window losing focus, a scroll, or the tab changing.
  Running an item closes the menu first, and focus goes back to what had it.
- **Items** follow one order per context, the most used first: for a link, Open link in
  new tab, in a tab group, in a workspace, in a new window, then Copy link and Save link
  as; for a tab, New tab below, Duplicate, Pin, Add to tab group, Move to workspace, Move
  to new window, then the closing ones. A shell lists only what Go can run today.

### The address field's slot

The owner, 2026-11-14: one icon for the address field, attached to its start rather than
inside the white area, so it reads as the field's own. A shell with an address field
that wants the default look follows it.

- **The slot** is 34px wide at the field's start, inside its hairline: `--accent-tint`,
  parted from the white area by a `--line` rule, with a 16px line icon.
- **On a web page** it is the site's settings, in `--icon`, always there, and a button
  named Site settings: a press opens that site's one view, which holds everything about
  the site. Never a lock, and never a second icon beside it.
- **When the page isn't secure** (an http page, or a certificate passed with Open anyway)
  the icon itself changes, a warning glyph in `--danger`, and the words "Not secure" follow
  it in `--danger`, 12px at 500, as Chrome and Edge write them (owner approved, item 7 of the
  UX comparison). The slot grows to fit them, 10px either side, and keeps the field's
  height. It stays one button, named "Not secure · Site settings": the site's view says why,
  on its first row, above the site's settings.
- **With no site** (a new tab) and **while the field is typed in**,
  it is a search icon in `--muted`, and no button: it says what Enter will do.
- **On one of our own pages** it is that page's own icon, and the white area shows the
  page's title with a close control, with nothing to type.
- Hover is `--accent-soft`; keyboard focus is `--focus-ring-inset`.

### Destructive actions

- **Confirm whatever deletes the user's data or more than one of their changes** in a
  `Dialog` that says what is lost: "Discard 4 changed shortcuts?". The button names the
  action (Delete, Discard, Remove), never OK.
- **A small edit that is easily redone** (one key, one switch) needs no confirm.

### On a phone

The owner, 2026-11-19: phone overlays were big and bulky, and felt bolted on over a page that
has little room. A phone shows as much page as it can.

- **News is a one-line bar above the toolbar,** 48px: a download started, finished or failed,
  in words and at most one action (Open, Retry). It hides itself after 2 s; a swipe down or
  sideways puts it away sooner, and a press keeps it while held. A question (Allow
  notifications) stays until it is answered, or 8 s. News never goes in the address field.
- **A sheet of actions has a one-line head** (the name, the address under it in `--muted`),
  then the most used actions, five at most, as a row of tiles 64px high (a 20px icon over an
  11.5px label), then the rest as 48px rows. It is as tall as what it holds.
- **A list is one 56px row per item:** an icon or thumbnail, the name, one `--muted` line;
  a tap does the main thing (Open), and everything else is in its ⋮. No row of buttons under
  an item. A failed one carries Retry in its row. Long lists group by day under a 12px
  `--muted` heading.
- **An action that repeats on every row is a 40px icon button** with an `aria-label`
  (Download, ⋯), not a text button.
- **A colour's + opens the Custom colour sheet,** not Android's picker (the owner,
  2026-11-19: "on android the custom color picker is baaad"). A one-line head with Done;
  a preview of the item as it will look, its dot and its chip; Hue, a full-width slider, a
  28px track in a 48px target with a 36px thumb; Shade, five 40px swatches of that hue from
  light to deep at the theme's strength; and Hex, a field for an exact colour. Each change
  shows at once; Done, Back or a tap outside keep it.
- **A Dialog with a field keeps its buttons in sight over the keyboard.** Its title and its
  buttons stay put and only its body scrolls between them, and while the keyboard is up the
  bars under the page (the toolbar, the crash and download bars) step aside, so the Dialog
  has the room down to the keyboard; so they do for a field in the page. The toolbar stays
  only while its own address field, or the find field in its place, has the keyboard. 2026-11-19, after a tall keyboard on Android 17 hid the
  sync passphrase's Turn on sync under the toolbar and a crash bar.
- **The page is dimmed only behind what must be answered or picked:** a dialog, a sheet of
  actions. A panel used beside the page, as a spark's, leaves it as it is.

### Errors and status

- **Check a value where the rule is known** (required, unique, min, max, pattern, a key in
  use). The error shows under the field in `.text-field-error`, the typed text stays, and Enter
  keeps focus there.
- **A system error is one plain sentence** in a row of the card it is about, with the
  technical detail in its tooltip. Never a raw Go or system message.
- **A status that asks the user to act carries that action on its own row.** An update
  that is ready has Relaunch there; one that failed has Try again; a key in use links to
  the action that has it.

### Keyboard and focus

- **One focus ring,** `--focus-ring`; on what fills its container (a menu item, a row),
  the same ring inside, `--focus-ring-inset`.
- **A group of choices** (`Segmented`, `Swatches`) is one Tab stop, and the arrow keys move
  in it.
- **When an action removes what had focus,** focus goes to a stated place: the next row, or
  the button that opened it. A `Dialog` gives focus back to what opened it.
- **Escape closes the innermost thing open:** a menu, then a dialog, then a field's edit or
  an open row, then the page (`ui/escape.js`). A field that has the keyboard is the
  innermost while it has it, so a find bar open beside it never takes its Escape. In an
  address field the first Escape closes the suggestions and drops a completion, leaving
  what was typed, and the second puts the page's address back and gives the page the
  keyboard, as every browser does.
- **Settings and Quit have their platform's keys.** On the Mac, Cmd+, and Cmd+Q. On Linux
  and Windows, Ctrl+, for Settings, as GNOME's Preferences and most apps there have it, and
  Ctrl+Shift+Q for Quit, as Chrome has it: Ctrl+Q sits beside Ctrl+W and Ctrl+Tab, and a
  slip would close every window. Quit asks nothing first, on every platform. 2026-11-20.
- **Back on our pages** closes what is open first, as Escape does, then goes to the place
  before in the page, then leaves it ([shells.md](shells.md)). A page's own ← to its parent
  ("Back to Privacy") is Up: it goes back when the place before is that parent, and otherwise
  takes the parent in place of the current entry, so it never adds one. 2026-11-19, after
  the phone's back left Settings from a sub-page.

### Copy

- **Sentence case** everywhere: "New workspace", not "New Workspace".
- **An ellipsis** when the button asks for more before it acts ("New workspace…", "Load
  unpacked…"), none when it acts at once.
- **Plain words for errors.**
- **A site or a server is named by its host, in bold,** in a dialog's title or words, so it
  reads first: "Sync with **example.com**?", as a sign-in and a passkey name theirs.
- **An address that isn't being edited shows its host first:** the host in `--text`, the
  scheme and the path in `--muted`. Editing it shows it whole, in one colour.

### The look ahead

From the owner's reference designs, for the sidebar and toolbar work that follows: soft
surfaces tinted from the frame colour rather than grey; thin line icons, 16px at a 1.5px
stroke; colour carrying identity, a workspace or a tab group as a coloured pill or dot, a
profile as a pill on the toolbar; room to breathe, sidebar rows 36px; the address field a
rounded surface with the page's own actions inside its right end.

### The window's own buttons and menus

The owner, 2026-11-19. On the Mac the window keeps its own buttons and the system's menu
bar. On Linux and Windows the frame is ours, so the shell draws them:

- **Minimise, maximise and close at the right end of the top row:** 28px icon buttons, 32px
  wide, in `--icon`; close turns `--danger` with white on hover. Each platform's habit; the
  Mac's circles there would read as a copy.
- **Every empty part of the top row drags the window,** a double-click maximises it, and a
  few pixels at each edge resize it. Nothing depends on knowing where the window is, as
  Wayland never says.
- **A menu strip, off by default:** Settings' "Show menu bar" puts Go's menus (File, Edit,
  View, History, Bookmarks, Tab, Window, Help) in a 30px strip on `--bar` above the top row,
  with the window's buttons moved into it. While it is off, Alt shows it until a menu
  closes and F10 opens the first menu. The menus are the context menus' surface, with
  shortcuts written as the platform does (Ctrl+R, F11).

## Shells with a character of their own

A shell may keep a look of its own where it is the point of the shell. What is listed here
is deliberate, not drift; everything else on this page still holds for it, colours,
contrast and focus rings first.

**Terminal** (`shell/src/apps/shell/tmshell/`):
- Monospace, 12px on a 16px line, set on `.app`, for the whole shell.
- Square corners on what floats (the suggestions) and on its marks (a workspace's square,
  a pane's square in the strip); controls keep radius 4.
- The strip's chip per window: a number in `--muted`, a name, and an 8px square per pane,
  outlined in `--muted`, filled in `--text` for the pane the window shows, in `--accent`
  while its page loads.
- Dense bars: the strip 32px, a pane's title bar 20px, the prompt line 28px. Its controls
  are 20px square with 12px icons at a 1.2px stroke, and its suggestion rows 24px.
- The `>` prompt mark, in `--icon`, `--accent` while the field has focus: the field's only
  focus mark besides the caret.
- The active pane's title bar in full accent, its one focus cue, with `--on-accent` on it.
- Status words on a pane (Blocked, Crashed, Failed) in `--on-danger` on `--danger`.

## Adding UI

1. Use the classes and variables above. If something is missing, add it to `design.css` (or
   to `core/tones` for a colour, in both palettes) and to this page, then use it.
2. Prefix a feature's own classes (`pbx-` for extensions) and keep them `scoped`.
3. Look at it in the real app, light and dark, and zoom in on the pixels: seams and default
   controls show there first.

# The daily driver

What a browser needs before someone can use it all day: right-click, downloads, file upload and
the page's own dialogs. Built in this order, one branch each, in Standard first, then Glide, then
the other shells through their owners. Mac first, then Android where it applies.

One rule for all four. **Go owns each request a page makes:** what is asked, the choices, how long
it lives, and what the answer does. The shell only draws it and sends back the choice. Each live
request has an id and sits in the state the SDK keeps; the shell answers by that id, and Go ignores
an answer to one it has dropped. What the OS already draws stays native: the file chooser and the
Save panel.

## Right-click

A right-click in a page, or a long press on a phone, reaches Go through content's
`HandleContextMenu`, which Go answers as handled. Go works out what can run there and sends it to
the window's shell as `contextMenu`. The shell designs and draws the menu, and sends back the
action chosen. Go runs it.

**The context.** `state.contextMenu` is `null`, or:

| Field | What it is |
|---|---|
| `id` | This menu's number. A newer right-click replaces it |
| `tab`, `x`, `y` | The tab, and where in its page, in the page's CSS pixels |
| `link`, `linkText` | The link pressed on, and its words |
| `src`, `media` | An image's address, or another medium's, and which: `image`, `video`, `audio` |
| `selection` | The text selected |
| `editable` | The press was in a field, for cut, copy, paste and select all |
| `actions` | What can run here: `{id, title, group, enabled}`, and `actions` of its own for a submenu |

**The actions.** Go lists only what it can run here and now. Ids never change once released, so a
shell may give an id an icon, or leave it out.

| Group | Actions |
|---|---|
| `link` | `link.newTab` Open Link in New Tab, `link.workspace` Open Link in Workspace (a submenu, `link.workspace.<id>` per other workspace), `link.newWindow` Open Link in New Window, `link.copy` Copy Link |
| `image` | `image.newTab` Open Image in New Tab, `image.copyAddress` Copy Image Address |
| `selection` | `selection.copy` Copy, `selection.search` Search for "…" |
| `edit` | `edit.undo`, `edit.redo`, `edit.cut`, `edit.copy`, `edit.paste`, `edit.selectAll` |
| `page` | `page.back` Back, `page.forward` Forward, `page.reload` Reload |

Back and Forward come disabled when there is nowhere to go. A press on a link in a field gets both
groups. Save Link As and Save Image As come with downloads, Copy Image and Inspect when the engine
has them, and Print and View Source later.

**The messages.** The shell sends `contextMenuChoose {id, item}` to run an action, or
`contextMenuClose {id}` when the menu goes without one. Go then sends `contextMenu` with `null`.
Go also drops the menu, and sends `null`, when its tab closes, navigates or stops being the active
one. Copying is Go's, through the engine's clipboard, so no shell needs `navigator.clipboard`.

**The design is the shell's.** It may order, group and leave out Go's actions, and add actions of
its own (Glide's Pin, say). Separators are its choice; `group` is there to help. In Standard it is
the design system's menu ([design-system.md](design-system.md)), at the press, kept inside the
window.

**A shell that draws nothing** shows no menu: content's own on the Mac is none. The contract's
rule 16 ([shells.md](shells.md#what-every-shell-does)) is what catches it.

**Android.** Go sends only presses on a link or an image. A long press on text or in a field keeps
Android's own selection bar, which already copies, pastes and selects. The Mobile shell's link
sheet opens on the same message, and still builds its own items from the press.

## Downloads

Today the engine has no `DownloadManagerDelegate`, so content refuses every download. The
engine gets one; Go decides where each file goes and keeps the list; the shell draws the list;
the Save panel is the system's own.

**The flow.** A page starts a download, or a menu's Save Link As does. The engine asks Go
where it goes (`PB_EVENT_DOWNLOAD_TARGET`: the download's id, its tab, address, suggested
name and type). Go answers with a path, with whether to show the Save panel at it first, or
with nothing to refuse it, as policy says. By default a file goes to `~/Downloads` under its
own name, with ` (1)`, ` (2)` added when that name is taken, and no panel; Save Link As and
Save Image As show the panel. The engine then reports each change
(`PB_EVENT_DOWNLOAD_UPDATED`: progress, done, cancelled, failed, paused), and Go keeps the
list.

**The list.** `state.downloads` is this profile's, newest first, each `{id, name, url,
path, state, received, total, paused}`, `state` being `progress`, `done`, `cancelled` or
`failed`. Go keeps the finished ones in the profile's `downloads.json`, the last 100, so the
list is there after a restart; one still running when the app quit comes back `failed`.
The shell asks for one action at a time, `downloadAction {id, action}`: `open` the file,
`reveal` it in Finder, `cancel`, `pause`, `resume`, or `remove` it from the list (the file
stays). `clearDownloads` removes every finished one.

**The shell.** A control named Downloads shows once the list has one, and opens the list:
each download a `listitem` by its file name, its progress while it runs, Open and Show in
Finder once done, Cancel while it runs. Standard puts the control in its toolbar and shows a
download's progress on it. The contract's rule 17 checks it, by role and name.

**Android.** There is no Save panel: a download goes where Go says, the profile's own
Downloads folder. Once done, the engine copies it into the phone's shared Downloads through
MediaStore, where the Files app and other apps find it, and removes the app's own copy. Open
opens the shared copy with the app the phone picks, and Show opens the phone's own list of
downloads. What each file became is kept in the app, so Open still works after a restart.

## File upload

The panel is the system's own, so no shell draws anything. A page's `<input type=file>`
reaches the engine (`RunFileChooser`), which asks Go first (`PB_EVENT_FILE_CHOOSER`: the tab,
whether it picks one file, several, a folder or a file to save, and the types it accepts).
Go lets it open while the page's address is one policy allows, and names the folder: the one
files were last chosen from in this run, or the system's choice. The engine then shows the
open panel as a sheet on the page's window, limited to the accepted types with All Files
still offered, and hands the page what was chosen; a folder upload gets every file under
it. The engine tells Go where they came from (`PB_EVENT_FILES_CHOSEN`) for the next panel.
Cancelling, or a refusal, answers the page with nothing, as a cancelled panel does.

**Android.** The phone's own picker already opens through content, without Go. Bringing it
through Go's policy comes with the Android pass.

## JavaScript dialogs

`alert`, `confirm`, `prompt` and the leave-page question. The engine's
`JavaScriptDialogManager` tells Go of each (`PB_EVENT_DIALOG_OPENED`: its id, tab, kind,
words, a prompt's text, the asking frame's origin, and whether a leave-page question is for a
reload) and holds the page until Go answers (`answer_dialog`). Go keeps them in
`state.dialogs`, oldest first, each with the title and the buttons' words every shell shows:
"example.com says" with OK and Cancel (OK alone for an alert), or "Leave site?" with Leave and
Cancel ("Reload site?" with Reload). When content gives up on one, as when its tab navigates or
closes, the engine says so (`PB_EVENT_DIALOG_CLOSED`) and it leaves the list unanswered.

A shell shows its active tab's, over that page only, so the tabs and the toolbar still work;
another tab shown hides it and the page keeps waiting. It answers with `answerDialog {id, ok,
text}`; Escape answers no. In Standard it is the design system's dialog over the page. The
contract's rule 15 holds every shell to it.

**Android.** The same on a phone: the engine gives every dialog and sign-in to Go, and the
Mobile shell draws them, its back key answering Cancel.

## Find in page

Find…, Find Next and Find Previous are Go's actions, `find.open`, `find.next` and
`find.previous` (Cmd+F, Cmd+G and Shift+Cmd+G on the Mac), in the Edit menu's Find submenu.
Each tab keeps its own search while its bar is open. Go runs it through the engine's `find`
(content's `WebContents::Find`) and hears the count from content's `FindReply`; it sends the
window's shell the active tab's search as `state.find`, `{tab, text, matches, active}`, or
`null`, and `focusFind` when the find key is pressed. The shell draws the bar and sends
`findText` as the text changes, `findNext` for Enter and Shift+Enter, and `findClose`, each
naming the tab whose bar it is, so a key typed as another tab comes forward goes nowhere; closing
keeps the match shown selected. A bar opened afresh starts with the last text searched for,
and a page that loads again is searched again. Contract rule 20 holds every desktop shell to
it. In Standard the bar hangs from the page's top right.

## Video full screen

A page's `requestFullscreen`, as a video's full screen button calls, reaches Go through
content's `EnterFullscreenModeForTab`. Go puts the window in full screen, unless it is already,
and tells its shell, `state.pageFullScreen`, the tab, or 0; the engine then has the page sized
again, and content asks Go (`IsFullscreenForTabOrPending`) whether the page has the screen,
which settles the page's promise. The shell lays that page across the whole window, with no
band and nothing of its own. It ends when the page calls `exitFullscreen`, when Escape is
pressed (Go takes it before the page, and asks the engine's `exit_page_full_screen`), when
another tab shows or the tab closes, and when the window leaves full screen through its own
button. The window then goes back to how it was. Contract rule 21 holds every desktop shell to
it.

**Android.** A phone's window already fills the screen, so a page in full screen there is the
window without the system's status and navigation bars (a swipe from an edge shows them for a
moment), and the phone's back key ends it before it goes back in the page's history.

## Sign-in to a site

A site or a proxy that answers 401 or 407 asks for a username and password. Content asks the
embedder (`CreateLoginDelegate`), and the engine makes it one of the page's dialogs: Go hears
it as `PB_EVENT_DIALOG_OPENED` with the kind `signin`, the site's realm as its words, and the
origin asking; `state.dialogs` carries it titled "Sign in", with "example.com asks for a
username and password." (or "The proxy …"), the realm as `realm`, and the Sign in and Cancel
buttons. The shell asks for the username and the password and answers with `answerDialog {id,
ok, text, password}`, the username as `text`; Go hands them to the engine's `answer_sign_in`,
which answers content outside its own call, as content asks. When content drops the request,
as when its tab navigates, the dialog goes. Contract rule 22 holds every shell that draws
dialogs to it. Nothing is remembered yet: saved passwords are a later step.

## Search engines and one suggestion source

Two things that go together: which search engine a typed text goes to, and one list of
suggestions for every place text is typed: the address field, Glide's Command Bar, and the
command box a new tab shows in the page's place.

**Search engines.** Go has a short built-in list (`core/search`): DuckDuckGo, Google, Bing,
Brave Search and Ecosia, each an id, a name and an address with the words' place in it. The
profile's `searchEngine` pref names one; DuckDuckGo until it is set. Everything that turns
words into a search uses it: the address field's Enter, the right-click menu's Search for,
and the suggestion list's search row. The settings page has a Search engine row, a select
of the list. The engine's name comes in the snapshot (`searchEngine: {id, name}`), for a
field's placeholder, "Search DuckDuckGo or type a URL". Adding one's own engine is a later
step.

**The suggestion source.** `core/suggest` answers a query with one ranked list, at most ten
rows, each tagged with where it comes from:

| `kind` | What it is | It carries |
|---|---|---|
| `address` | What was typed, read as an address | `url` |
| `search` | "Search DuckDuckGo for …" | `url`, the search |
| `tab` | A tab open in any window and workspace of the profile, matched by title or address | `tab`, `url`, `workspace` |
| `history` | A page visited before | `url` |
| `action` | An action that can run now, matched by its title | `action` |

Every row has `kind`, `title` and `detail`, a second line: the address's host, the tab's
workspace, or an action's key. The order: first what Enter does, the `address` row when the
text reads as one and the `search` row otherwise; then up to three tabs, five from history and
three actions, best first in each; and the `search` row last when an address came first. With
nothing typed, as in a new tab's box, the list is the pages visited most.

**The messages.** As today, `suggest {id, text}` and `suggestions {id, items}`, with two
additions. The shell names the kinds it draws, `kinds: ["tab", "history", …]`; without it the
list is history alone, as it was, so a shell that hasn't changed still works. And a shell runs
a row with `chooseSuggestion {row, newTab}`: Go loads the address or the search in the active
tab, or a new one, shows the tab, in whichever window it is, or runs the action. The SDK has
`suggest(text, kinds)` and `chooseSuggestion(row, newTab)`. A shell with a split or a mode of
its own may still act on a row's `url` itself.

**Speed.** A key must not scan and lowercase everything again: at 100,000 visits that took
30 to 38 ms a key. History keeps each entry's lowercased address and title from when it is
stored, and the search picks its best few without sorting every hit: about 8.5 ms a key at
100,000 visits. The actions are few, so they are matched as they are. A benchmark in
`core/suggest` measures it.

**Not in this step.** Suggestions fetched from the search engine as one types: they send every
keystroke to the engine, so they are a setting to decide, off by default if added. Bookmarks
join the list when bookmarks exist.

**The new tab.** A tab with nothing in it shows the shell's own command box where the page
would be, drawing this list; the look is the standard shell session's mock. Glide's Command
Bar moves onto this list, in place of its own search of tabs and actions.

## Permissions

**The interface.** The owner, 2026-11-14: "we should have an interface for it and not just
follow contents stuff... keep that decoupled behind the engine interface". So a page's
request crosses into Go in our own terms, and every request feature after this one follows
the same shape:

- The engine interface (`core/engine`, `pb_abi.h`) names things as the web platform does,
  never as content does: a kind is the Permissions API's name (`camera`, `microphone`,
  `notifications`, `clipboard-write`, `geolocation`...), a status is ours (`ask`, `granted`,
  `denied`, numbered 0, 1, 2 by us). Another engine could implement it unchanged.
- The C++ adapter, `engine/browser/permissions.cc`, is the only code that knows content's
  permission types and statuses and its camera path; it translates both ways
  and decides nothing.
- Go's sites manager, `core/sites`, is the one place a decision is made. Every kind a page
  can ask for reaches it, as a request or a read, even those answered without a prompt:
  clipboard write is granted there, and a kind it doesn't know is denied there.
- The kinds are one table in Go (`sites.Kinds`): its title, whether a page asks for it,
  whether it can be asked about, its answer otherwise, whether its prompt needs a press, and whether it offers Allow this time.
  Adding geolocation is one row there and the adapter's name for it.
- The shell only draws what Go sends and sends back the choice.

**Where per-site settings live.** The owner, 2026-11-14: "we might have other site level
permissons or configs that we might want later and go core should own those... like down
the line we can have an option to let extensions content script run on a page or some other
injected script based feature for the page and that time the config to diable that would
live in that domain and should ideall sit with the site permissions". So `core/sites` is the
home of every setting kept per site, not a copy of the web's permission list. A kind is ours:
some are what a page asks for (camera, microphone, notifications, clipboard), and a later
kind may be a switch of our own that no page asks for, which is one more row in the table
with `Requested` off. The store is per site (`sites.json`: origin, then kind, then value),
the messages and the settings section speak of a site's settings with Go's title for each,
and nothing in them assumes a kind came from the engine. Only the kinds built here exist.

Before this, the engine's delegate granted clipboard write and denied everything else
itself, so `getUserMedia` failed with NotSupportedError and `Notification.requestPermission()`
answered `denied` (basics 16 and 17). The OS's own question, macOS asking once whether the
app may use the camera, stays the system's.

**What Go offers.** Three kinds can be asked about: `camera`, `microphone` and
`notifications`. `clipboard-write` is granted without asking; every other kind is denied
without asking until it gets a row. Content keeps these from insecure pages, so only
`https` pages and `localhost` ask. Our own pages, which are in no tab, get each kind's
answer with no prompt.

**The flow.** A page asks (`getUserMedia`, `Notification.requestPermission()`). The engine
sends `PB_EVENT_PERMISSION_REQUEST`: the request's id, its tab, the origin asking, the kinds,
and whether a user gesture led to it. Go answers later, with the engine call
`answer_permission(id, statuses)`, one status per kind. A page reading the state
(`navigator.permissions.query`, `Notification.permission`, a device's label) asks
synchronously, `PB_EVENT_PERMISSION_STATUS`: tab, origin and kind, answered with a status.
For `getUserMedia`, the adapter asks for `camera` and `microphone` the same way, then gives
the page the devices it named, or the system's first.

**What Go decides.** The person's decisions are the profile's, in `sites.json`, by origin
and kind: `allow` or `block`. A kind already decided is answered at once, with no
prompt. A kind whose prompt needs a press (`notifications`), asked without one, is answered
`ask` with no prompt, so a page can't put one up on load (the page sees `default`). Anything
else undecided becomes a prompt.

**The prompt.** `state.permissionPrompts` is every prompt open in the profile, one per tab at
most, each `{id, tab, origin, kinds, once}`; a second request from the same tab waits behind
the first. A shell shows the one on its active tab, and sends `permissionAnswer {id, choice}`:

| `choice` | What Go does |
|---|---|
| `allow` | Grants it and keeps `allow` for the origin |
| `once` | Grants it until the tab leaves the origin or closes; offered (`once`) when every kind asked offers it: camera and microphone |
| `block` | Denies it and keeps `block` |
| `dismiss` | Answers `ask`, keeping nothing: the prompt was closed |

Go drops a prompt, answering `dismiss`, when its tab closes or navigates to another
document, and ignores an answer to a prompt it dropped.

**The site settings page.** The owner: "we should have a page which shows either all site
settings or jsut the current pages permissions... like who chrome shows the site settings
icon in the omnibox.. but make it look like ours". One section of settings, Site settings,
with two views by its address: `sites`, every site with a setting, a row each saying how
each is by Go's title, which opens the site's view, and a menu to reset one or Remove them
all; and `sites/<origin>`, one site alone, every setting settings can set (Go's
`siteKinds`), each Ask, Allow or Block (`setSiteSetting`). The address is all the section
reads, so it shows the same once settings is a tab of its own. A shell's control named Site
settings sends `openSiteSettings`, and Go opens settings at the active tab's site. In
Standard it is an icon at the start of the address field, on every web page. The contract's
rule 31 checks it.

**Notifications** a page shows are the system's: the engine sends each to Go
(`PB_EVENT_NOTIFICATION_SHOW`, with its title, body, origin and tab), Go checks the origin
is allowed and asks the engine to post it to macOS's notification centre.
A click on it comes back to Go (`PB_EVENT_NOTIFICATION_CLICKED`), which brings the tab's
window to the front, makes the tab active and fires the page's `click` event. A packaged app
only: a bare binary can't post to the notification centre.

**The shell.** In Standard the prompt is the design system's floating surface, from the
address field's start: a glyph of what is asked, then "*site* wants to use your camera and
microphone". Where Go offers this time, its buttons are Block, Always allow and Allow this
time, the primary, as a quick press should give the least; otherwise Block and Allow. It is
a `dialog` named by that sentence; Escape dismisses it. It takes no focus, so a page can't
type into it, and follows the address field in tab order, so F6 then Tab reach it. The
contract's rule 30 checks it, by role and name, on the mock.

**The connection.** The owner: "clicking gives one list of everything site related... so the
button get gains the unsecure data and doenot loose the site settings data". Each tab says
how its page reached it, in our own terms: `tab.connection`, `secure` or `insecure`, absent
for our own pages and the blank page, and `tab.connectionNote`, why in one line. The adapter
maps content's security state (`PB_EVENT_CONNECTION_CHANGED`, 44: none, secure or insecure,
and a reason, `not-encrypted`, `certificate` or `mixed-content`), and Go words the note. A
shell draws any value other than `secure` as not secure: Standard's address-field slot
changes its icon, and its press opens the same one-site view, whose first row is the
connection of the site's open pages (`siteConnections`, by origin, insecure when any of
them is). The certificate warnings extend it: a page whose certificate the person went past
is `insecure` for `certificate`.

**The numbers.** Events 40 to 44: `PB_EVENT_PERMISSION_REQUEST`, `PB_EVENT_PERMISSION_STATUS`,
`PB_EVENT_NOTIFICATION_SHOW`, `PB_EVENT_NOTIFICATION_CLICKED`, `PB_EVENT_CONNECTION_CHANGED`.
Rules 30 and 31.

**Not yet expressed,** none needed now, each one engine call or one field when it is:

- **Telling an open page its setting changed.** A Reset or a change in Site settings
  doesn't fire the page's `permissionStatus.onchange`; it would be one engine call from Go,
  the origin and kind.
- **A permission's options.** Camera pan-tilt-zoom and MIDI with sysex are kinds of their
  own; any other option a page's request carries is dropped by the adapter.
- **Settings by the page a site is embedded in.** A setting is by the origin asking, not
  by that origin within another site.
- **Which camera or microphone.** The adapter gives the one the page named, else the
  system's first; Go has no say in it yet.

**Android.** The same messages, and the Mobile shell's prompt as a sheet. Once Go allows the
camera or microphone, the adapter asks Android for its own grant before the page gets a
device. Built after the Mac's.
## Error pages and certificate warnings

Today a page that fails to load, crashes or is blocked shows an empty page in the desktop
shells (basics 7), and a bad certificate does the same (basics 10): content refuses the
certificate and nothing says why. The Mobile shell draws a notice of its own over such a
page since 0.2.58. Now Go says what the notice is, and every shell draws it.

**The notice.** `tab.notice` is absent, or `{kind, title, detail, actions}` while the tab's
page has nothing to show: `kind` is `failed`, `crashed`, `blocked` or `certificate`; `title`
one plain sentence ("Can't reach example.com", "This page crashed", "example.com's
certificate has expired"); `detail` a second line where there is one, the error's own name
or what a certificate warning means; and `actions`, each `{id, title, primary}`, what the
person can do: `reload` (Try again, or Reload after a crash), `back` (Go back, when there
is a page to go back to) and `proceed` (Open anyway). The shell draws it over the tab's
page and sends `noticeAction {id, action}`, the tab's id; Go does it. The notice goes when
the tab loads anything.

**A bad certificate.** The engine asks Go about each certificate error, in our own terms
(`PB_EVENT_CERTIFICATE_ERROR`, 45: the tab, the address, why, one of `expired`, `untrusted`,
`wrong-host`, `revoked` or `invalid`, and whether it may be passed; answered at once, allow
or refuse). Go refuses, so the page doesn't load, and the tab's notice is the warning. Where
the error may be passed (content says not for a site that demands a valid certificate, as
HSTS sites do), the notice offers Open anyway: Go then allows that host's certificate
errors until the app quits, kept nowhere, and loads the page again. A page opened that way
is `insecure` for `certificate` ([the connection](#permissions)), so the address field's
slot and the site's view say so for as long as it shows.

**The shell.** In Standard the notice fills the page's area, on the bar's colour: an icon,
the title, the detail in the muted colour, and the actions as buttons, the primary one
first. It is an `alert`, its buttons by Go's titles. The contract's rule 32 checks it on the
mock: a load that fails brings an alert with Try again, which loads the page again. Mobile
keeps its own notice, drawn from the same tab, until it takes Go's.

**Android.** The same messages; the certificate question comes with the Mac's, as the
engine's client is shared.


## Tabs come back, and closed ones reopen

The owner, 2026-11-15: "default is continue where you left off... and something cimila to
crtl at t for getting back recently closed tabs windows etc...". Before this, nothing of a
window's tabs was kept: the app always came back with one empty tab (basics 32).

**What Go keeps.** `session.json` in the profile: each window, in front-to-back order, with
its frame on screen and its tabs in order, each tab's address, title, workspace and tab
group, the active tab, and the window's groups (name, colour, folded); and what was closed
recently, newest first. Go writes it when any of that changes, a new file renamed over the
old, and only when what it would write differs from the last, so a page's loading progress
writes nothing. Quitting keeps it as it was before the quit began, and so does closing the
last window.

**When it comes back.** At every start, by default: the setting On start, in General, is
Where you left off, or Start fresh: a window opened as the profile's shell opens one,
Standard on its command box and other shells on a new tab. After a crash or a kill the
windows come back whatever the setting says. Go opens each window where it was, its tabs in
their workspaces and groups, the active one active, in place of the usual empty start. Each
window's active tab loads at once; every other tab shows its address and title and loads
when it is first made active, so a start doesn't start a renderer per tab. A shell sees such
a tab as any other. A tab whose workspace was deleted since goes in the first one. Addresses
given at the start open beside the windows that came back.

**Recently closed.** Go keeps the last 25 tabs and windows the person closed, newest first,
across restarts: a tab with its window, place, workspace, address and title; a window with
everything a window keeps above. Reopen Closed Tab (Shift+Cmd+T, Ctrl+Shift+T elsewhere, in
every keymap template) brings back the newest: a tab into its window at its place, or into
the window in front when its own is gone, made active and loading; a window where it was,
with its tabs. The History menu lists them as Recently Closed, each by its title, a window
as "N tabs", and choosing one brings that one back. Our own pages, such as settings, aren't
kept, and nor is a tab closed by quitting.

**What doesn't come back.** A tab's back and forward history (it loads its address alone),
its scroll position and form fields, and a shell's own per-window data.

**The shell.** Nothing new: it hears the windows and tabs as it does at any start, and the
action and the menu list are Go's. No rule.

**Android.** The same Go, so a phone's tabs come back too, in its one window.

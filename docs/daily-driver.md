# The daily driver

What a browser needs before someone can use it all day: right-click, downloads, file upload and
the page's own dialogs. Built in this order, one branch each, in Standard first, then Glide, then
the other shells through their owners. Mac first, then Android where it applies.

One rule for all four. **Go owns each request a page makes:** what is asked, the choices, how long
it lives, and what the answer does. The shell only draws it and sends back the choice. Each live
request has an id and sits in the state the SDK keeps; the shell answers by that id, and Go ignores
an answer to one it has dropped. What the OS already draws stays native: the file chooser and the
Save panel.

## Where it stands (2026-11-16)

**Done**, on the Mac in every desktop shell and on Android in both Mobile shells unless noted:
right-click, downloads (with Retry and Copy link, and a stalled connection resumed after
sleep), file upload (Mac), the page's dialogs, find in page, video full screen, sign-in to a
site, search engines with one suggestion source, permissions, tabs coming back, History
and Bookmarks (desktop; Cmd+D bookmarks in every shell), error pages with certificate
warnings (Standard and the Mobile web shell), printing through the system's panel, and a
new tab page (Standard's box and the Mobile web shell, which reaches History and Bookmarks
from it). The list with owners is
[daily-use.md](daily-use.md).

Bringing bookmarks, history and the search engine over from Chrome, Edge, Brave, Arc,
Firefox and Safari (once the app has Full Disk Access), in the welcome and Settings' Import
([onboarding.md](onboarding.md#built)).

**Planned**, decided and not started ([todo.md](todo.md#daily-driver)): bookmarks in the
address bar's suggestions as you type.

**Needs discussion** ([todo.md](todo.md#daily-driver)), in this order: saved passwords and
autofill, engine-fetched search suggestions (off by default if built), and where a phone's
downloads go.

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
| `page` | `page.back` Back, `page.forward` Forward, `page.reload` Reload, `page.print` Print… |

Back and Forward come disabled when there is nowhere to go. A press on a link in a field gets both
groups. Save Link As and Save Image As come with downloads, Copy Image and Inspect when the engine
has them, and View Source later. `page.print` Print… follows Reload ([Printing](#printing)).

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
Save Image As show the panel, and so does every download with Settings' Ask where to save
each file on (a pref, `askWhereToSave`, off by default; not on a phone, whose engine has no
panel). The engine then reports each change
(`PB_EVENT_DOWNLOAD_UPDATED`: progress, done, cancelled, failed, paused), and Go keeps the
list.

**The list.** `state.downloads` is this profile's, newest first, each `{id, name, url,
path, state, received, total, paused}`, `state` being `progress`, `done`, `cancelled` or
`failed`. A file the app made itself and kept as a finished download, a saved crash report,
has no `url` and says what it is in `from` instead. Go keeps the finished ones in the profile's `downloads.json`, the last 100, so the
list is there after a restart; one still running when the app quit comes back `failed`.
The shell asks for one action at a time, `downloadAction {id, action}`: `open` the file,
`reveal` it in Finder, `cancel`, `pause`, `resume`, `retry`, or `remove` it from the list
(the file stays). `clearDownloads` removes every finished one.

**The shell.** A control named Downloads shows once the list has one, and opens the list:
each download a `listitem` by its file name, its progress while it runs, Open and Show in
Finder once done, Pause (or Resume), Cancel and Retry while it runs, and Retry once it failed
or was cancelled. Every progress has Retry and Cancel while it runs (the UX guide,
`docs/ux-guide.md`); Retry sits a little apart from Pause and Cancel, as it throws away what
has arrived: Go stops the engine's copy, whose late news it then ignores, and starts the
address over in the same row, under the same name. A running row has no remove button,
Cancel being there. Mobile's download bar has Cancel and Retry while one runs, and its
Downloads page has Pause too. Standard
puts the control in its toolbar and shows a download's progress on it; Open and Show in
Finder are icons there, so the name has the row. The contract's rule 17 checks it, by role
and name.

**Details** (owner, 2026-11-18). Each row of Standard's list and of the Downloads page opens
out at its Details control (`aria-expanded`): From, the whole address with Copy link
(`copyLink`), or for a file the app kept itself what it is (`from`, as Crash report); and
Saved in, the folder on the Mac or Downloads on a phone, where the file is. A closed row is
as it was, so the list stays short.

**A download you see start** (owner approved, item 9 of the UX comparison). When a tab of
the window in front begins a download, Go tells that window's shell, `downloadStarted {id}`,
after the `downloads` that lists it, and marks `download.started` on the timeline; a window
behind hears nothing. Go knows which of the app's windows is in front, not whether the app
is, so the shell also checks that its page is visible. Standard's Downloads list then opens
by itself under its button, the same floating list the button opens, so the newest is at
its top with its progress. It closes after five seconds, unless the pointer is over it or a
control in it has the keyboard, and at once on Escape or a press outside it. Unlike the list
the button opens, it has no cover: the page around it stays usable. It takes the keyboard
while it shows, so Escape reaches it and a press on the page, which takes the keyboard from
the shell, closes it, and it gives the keyboard back as it closes; a download begins from a
press, so no typing is lost. A press on the button makes it the person's list, which stays;
a list the person opened stays as it is.

**A connection that goes silent.** Across the Mac's sleep a download's connection can die
without a word, and content waits on it for good. The engine watches every running download,
but a paused one, which never times out: one that receives nothing for 30 seconds of waking
time, the same 30 as the updater's (updates.md), is interrupted as a network timeout,
which content resumes at once on a new connection from where the file got to. When resuming
fails too, the download shows as failed, with Retry and Copy link (`downloadAction` `retry`,
`copyLink`). Retry resumes one this run still has, or starts it over as a new download of its
address in the same row (`download_url` with tab 0, the browser's own). The app's own update
download is watched the same way: one silent for 30 seconds fails, Settings shows Try again,
and the next check downloads it again.

**The Downloads page.** A page of ours, in a tab of ours as History and Bookmarks are
(`downloads.show`, "Downloads", Shift+Cmd+J, in the Window menu): the same list, newest
first, with Pause or Resume and Cancel while one runs, Open (a tap on its name too) and Show
in Finder, or Show in Downloads on a phone, once done, Retry once it failed or was
cancelled, Details, a × to take one off the list, and Clear all. Go sends it the list in its snapshot and on each change, and takes
`downloadAction` and `clearDownloads` from it as from a shell. A phone's shells reach it from
their menu, always there, with how many are running beside it (owner, 2026-11-15).

**Android.** There is no Save panel: a download goes where Go says, the profile's own
Downloads folder. Once done, the engine copies it into the phone's shared Downloads through
MediaStore, where the Files app and other apps find it, and removes the app's own copy. Open
opens the shared copy with the app the phone picks, and Show opens the phone's own list of
downloads. What each file became is kept in the app, so Open still works after a restart.
Open waits for that copy, so it works the moment a download ends.

**A phone's notification.** Go shows each download where the system shows such things
(`ShowDownload`, the engine's `show_download`; nothing on the Mac): on Android, a
notification per download, quiet, its progress while it runs (a few updates a second at
most), "Downloaded" with the size once done, which opens the file at a tap as Open does,
and "Download failed"; a cancelled one goes. Android 13 and later ask the person to allow
notifications at the first download.

**A navigation that becomes a download** shows nothing instead, so the tab stays at the page
it shows: the engine ends it as a failure with no failure named, at that page's address, and
Go puts the tab's address back. Before, the tab kept the download's address, and a restart
loaded it and downloaded the file again.

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
| `bookmark` | A bookmark whose title or address holds the text, or with nothing typed one of the newest ([A new tab](#a-new-tab)) | `url` |
| `closed` | A tab closed lately, with nothing typed only ([A new tab](#a-new-tab)). Choosing it reopens that tab | `url` |

Every row has `kind`, `title` and `detail`, a second line: the address's host, the tab's
workspace, or an action's key. The order: first what Enter does, the `address` row when the
text reads as one and the `search` row otherwise; then up to three tabs, five from history,
three bookmarks and three actions, best first in each (the bookmarks newest first); and the
`search` row last when an address came first. A page shows once: history and the bookmarks
leave out what a tab already lists. With nothing typed, the list is what a new tab offers
([A new tab](#a-new-tab)).

**Inline completion,** all local, as the owner approved (item 5 of the UX comparison). When
the text has no space and the bare address (no scheme, no `www.`) of a page from history or
a bookmark starts with it, that row comes first, before what Enter would
otherwise do, and carries `inline`: the rest of the address after what was typed, which the
field shows selected after the caret. The page visited most wins, then the newest bookmark.
While the text is still inside the host it completes to the host alone, as Chrome does:
`wikip` gives `edia.org`, on the row of the host's own page when it was visited, else on an
`address` row for the host, in the scheme and `www.` the page had. Past the host (`docs.test/gu`),
or for a shell that asks for no `address` rows, it completes to the whole address, in its own
case. A typed scheme or `www.` stays (`https://www.wiki` gives `pedia.org`). Only web
addresses complete, and a shell that names no kinds never gets one. A completed page open in
a tab is that tab's row, listed once, so Enter shows the tab rather than loading the page a
second time. Enter takes the first row, so it goes where the field shows.

Standard's address field shows the completion as Chrome and Safari do: the rest selected
after the caret. A key that matches its next letter carries it on at once, without waiting
for Go; Backspace takes the selected part away first, and the next answer then completes
nothing, the completing row moving a place down so Enter is what was typed; the arrow keys
replace it with the row they choose; the first Escape drops it with the list. The row has
no heading over it, as it is what the field shows. Bookmark rows carry the star.
Enter takes the first row, or the completion, only while it belongs to the text in the
field now: the completion shown is that row's rest after what was typed, or the list
answers what is typed now. Before Go's answer for the latest text, as after fast typing,
Enter takes what was typed, never a row of the list for an earlier text; a row the arrow
keys or a click chose is taken as always. Standard's command box takes the same rule from
0.2.76, and Mobile's field takes a row only at a tap, so its Enter is always what was typed.

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
keystroke to the engine, so they are a setting to decide, off by default if added; the owner
keeps them off.

**The new tab.** A tab with nothing in it shows the shell's own command box where the page
would be, drawing this list; the look is the standard shell session's mock. Glide's Command
Bar moves onto this list, in place of its own search of tabs and actions. In Standard, New Tab
over a page opens the box over it: the page shows faintly through a scrim of the bar's tint
at 80%, the owner's choice on 2026-11-15 ("a partial opacity backdrop will do", raised "so as
to not clutter the interface"), with no blur,
since the page is a view under the shell. A window with no tabs keeps the plain tint.

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
changes its icon and writes "Not secure" beside it, and its press opens the same one-site view, whose first row is the
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

A page that fails to load, crashes or is blocked, or has a bad certificate, has nothing to
show. Go says what the tab shows instead, a notice, and every shell draws it: Standard since
0.2.65, the Mobile web shell from its own notice to Go's after 0.2.74.

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
mock: a load that fails brings an alert with Try again, which loads the page again, and a bad
certificate one with Open anyway. In the Mobile web shell it is the same, sized for a thumb:
48 px buttons in a row, the others quiet, and Share report beside Reload after a crash. Glide
and the native Mobile shell are still to take it.

**Android.** The same messages; the certificate question comes with the Mac's, as the
engine's client is shared.


## Printing

The owner, 2026-11-16: "yes system print is fine... under file is fine", and then "print can
be under file in system menu.. but for app menu it can be direct". The page prints through
the system's own print panel, which has its preview, the printers, the page range and Save
as PDF. There is no print preview of ours.

**How it prints.** Chromium's printing component, unpatched, renders the page to a PDF: a
printing helper in each frame's renderer, and in the browser a print manager per tab
(headless Chrome's, which only makes PDFs) and, where a page's frames of other sites run in
processes of their own, the print compositor in a utility process, which puts them together.
The engine hands the PDF to the system: on the Mac the print panel as a sheet on the tab's
window, its preview that PDF; on Android the print sheet (`PrintManager`, with a
`PrintDocumentAdapter` that writes the PDF). The page is drawn on the paper the default
printer uses on the Mac, and on the locale's usual paper, Letter or A4, on a phone, whose
sheet fits it to the paper chosen there. Margins are Chromium's default; no header or footer
and no backgrounds, as Chrome prints by default; a page's own `@page` size wins.

**Four ways in.** All end in the engine's `print(tab)`.

- **Print…, Cmd+P** (`page.print`), Chrome's key, in the File menu of the menu bar, enabled
  while the active tab shows a page.
- **The shell's menu** (Standard's ☰, the phone's ⋯ sheet) shows it at its top level, just
  after File: the menu tree marks it `"shell": "top"`
  ([menus-shortcuts.md](menus-shortcuts.md#the-shells-menu)).
- **A right-click** on a page, with nothing under the press: Print… in the `page` group,
  after Reload. A phone's long press has no page group, so the ⋯ sheet is its way in.
- **The page's own `window.print()`.** Chromium's helper hands it to the browser instead
  of printing (its delegate overrides it), on our `FrameHost` interface, by the frame that
  asked. The engine asks Go (`PB_EVENT_PRINT_REQUESTED`, 46, answered at once), which lets
  it print while its tab is the one its window shows; a tab behind waits for no panel. The
  frame that asked is what prints, as in Chrome. The call returns at once, and the page
  gets `beforeprint` and `afterprint` once, as its PDF is drawn.

**Checked** on the mock by Go's tests (`TestPrintingAPage`: the menu, the right-click, a
page's own call, and not from a tab behind). In the real app on the Mac, 2026-11-17: Cmd+P,
☰ › Print… and a page's `window.print()` each open the panel as a sheet with the page in
its preview, Cancel closes it, and the page's `afterprint` runs; deskrig has no right click,
so the right-click's Print… is the tests'. On an Android 16 emulator, ⋯ › Print… opens the
print sheet with the page, on Letter there by the locale.

## A new tab

The owner, 2026-11-16, approved the mocks: on a phone "tiles, recent bookmarks, then All
bookmarks and History", the keyboard staying down; and in Standard's box "yes to frequent
sites and latest bookmarks or stuff with a cap on how many to list total".

**One list from Go.** What a new tab offers is Go's suggestion list for nothing typed: the
sites visited most, as `history` rows, then the newest bookmarks not among them, as
`bookmark` rows, eight rows at most. A site is a host without www.; its visits are its pages'
added up, and its row is its page visited most (`history.TopSites`). Asked for both kinds,
the sites keep up to five rows when there are bookmarks to show, the bookmarks fill the rest,
and the sites any rows the bookmarks leave. Asked for one kind, it has all eight. Choosing a
row loads it as any row does.

**Open and closed tabs, for a phone's field as it is tapped** (ux-comparison.md item 4).
A shell that asks for `tab` with nothing typed gets the window's other tabs first, the most
recently used first, its active tab left out, three at most, as `tab` rows; one that asks for
`closed` gets the tabs closed lately next, the newest first, three at most, as `closed` rows
`{kind, title, detail: the host, url}`. Then come the sites and the bookmarks in the rows
left, eight in all, each address once. Choosing a `closed` row reopens the newest closed tab
at its address, the way History > Recently Closed does (`reopen` in `core/app/session.go`,
the span `tab.reopen`). A shell that asks for neither kind gets what it got before: Standard's
command box asks for `history` and `bookmark` alone.

**Standard.** New Tab opens the command box ([shells.md](shells.md#a-shells-own-actions)),
which now shows the list before anything is typed, under Frequently visited and Bookmarks,
so Cmd+T and Enter goes to the top site. Typing replaces it with the usual list, and
emptying the box brings it back.

**The Mobile web shell.** A tab with no page yet, a new tab, shows the new tab page over it
instead of a blank page: the sites as tiles, four to a row, each a letter on a colour taken
from its host, with the page's title under it; the top bookmark folder as tiles while
`bookmarksNewTab` is on ([bookmark-folders.md](bookmark-folders.md#where-bookmarks-show));
then All bookmarks and History, which open those pages of ours (`bookmarks.show`,
`history.show`). It asks Go for the sites alone. The keyboard stays down, so nothing covers the tiles; a tap
on the address field starts typing. A long press on a tile shows a × on each, which takes
that site out of history, every page of it (`forgetSite {url}`); a tap elsewhere ends that.
The page goes as soon as the tab starts loading anything.

**Checked** on the mock by the mobile and Standard suites and the contract, and on an
Android 16 emulator, 2026-11-17: a new tab shows the tiles by host and the newest bookmark
with the keyboard down, a long press shows each tile's ×, and All bookmarks and History open
those pages, which the APK now carries as it carries Settings and Downloads.

**Not built:** pinned or edited tiles, and a background picture. Glide and the other shells
keep what they had.

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

## History and Bookmarks

Two pages of ours, each in a tab of ours, one per window, as Settings is: `chrome://history/`
and `chrome://bookmarks/`, the apps `shell/src/apps/history` and `shell/src/apps/bookmarks`.
Every page of ours in a tab talks to Go over one channel, `PB_EVENT_PAGE_MESSAGE` with the
page's name one way and the engine's `send_to_page` the other; Go hands each message to that
page's manager.

**History** shows the pages visited, the latest first and grouped by day, a hundred at a time
with Show more, and a search. A row opens its page in a new tab beside, can be removed, and
Clear history takes away the last hour, today, or everything. Go keeps one entry per address;
a removal is written to the history file as a line that takes it out, so it stays gone after a
restart. Show All History (Cmd+Y) is in the History menu.

**Bookmarks** keep folders, a tree in the profile's `bookmarks.json`
([bookmark-folders.md](bookmark-folders.md)). The address bar's star and Bookmark This Page…
(Cmd+D, in every shell) bookmark the page shown into the folder last saved in, and Standard and
the Mobile web shell show a panel to change its name and folder, or remove it. Bookmark
Manager (Option+Cmd+B) shows the folders on its left and the one chosen on its right, with a
search in every folder; entries move by a drag or Move to…, and each can be renamed,
re-addressed or deleted, a folder with what it holds; an address bookmarked twice is
refused. Import is a later step.

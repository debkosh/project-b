# The first-run onboarding

What a new user sees on the first start, and how they bring their things over from another
browser. Item 13 of the [UX comparison](ux-comparison.md#what-to-do-ranked). **Proposed,
2026-11-17: mocks for the owner, nothing built yet.**

The owner, 2026-11-17: "if we are missing features then we add those but the onboarding needs to
be nice and thats not shell its our internal page". So it is a page of ours in a tab, as Settings
is: the app `shell/src/apps/welcome`, at `chrome://welcome/`. Every shell shows it the same way,
and it opens once, on the first start. Every step can be skipped, and everything it sets is in
Settings later.

The owner on the first mocks, 2026-11-17: "nice mocks... the only thing is not to choose a default
color i donot like the current green thing... also first time the welcome should come as full app
screen... with a nice splash screen and stuff". So:

- **No colour is chosen for the user.** The welcome and a new profile are neutral (a light grey
  frame, ink for the accent) until a colour is picked; "None" is the first swatch and the one
  selected. The dusty teal default (`#4f9e9e`) goes from the profile templates and the presets.
- **The welcome takes the whole window,** with no browser UI around it: a splash (the mark draws
  itself, the name fades in), then the welcome, with a window behind the words that changes colour
  as it waits. The browser's UI comes in around the page at Start browsing. The phone's welcome
  takes the whole screen in the same way.

## The pages

The mocks are drawn in [screenshots/onboarding/mocks.html](screenshots/onboarding/mocks.html)
(`?m=d0` plays the splash into the welcome; `d0s` to `d6`, `p0` to `p2` are the stills); the
splash as a [video](screenshots/onboarding/splash.webm).

**Desktop**

1. **Welcome:** the splash ([mock](screenshots/onboarding/mock-d0s.png)), then the mark, the
   name, one line, Get started and Skip setup over a window that cycles through the colours
   ([mock](screenshots/onboarding/mock-d1.png)).
2. **Bring your things over:** each browser and profile found on the Mac, a row with what it
   holds. Turned on, the row opens to what to bring: bookmarks (with their folders), history,
   passwords, open tabs, the search engine, extensions ([mock](screenshots/onboarding/mock-d2.png)).
   Safari's row asks for Full Disk Access once ([mock](screenshots/onboarding/mock-d2b.png)): the
   page opens that pane of System Settings and goes on by itself when access is given. Then
   what came over ([mock](screenshots/onboarding/mock-d2c.png)). Open tabs come over as a
   workspace named after the browser.
3. **Pick your style:** a picture of the browser beside the choices: tabs on the side, on top,
   or Glide; the colour, None first; theme; keys like Chrome's or Project B's. The picture and
   the page itself take each choice at once ([none](screenshots/onboarding/mock-d3.png),
   [blue](screenshots/onboarding/mock-d3b.png)).
4. **Protection:** the ad blocker, Safe Browsing, Links to other sites, the search engine,
   automatic updates ([mock](screenshots/onboarding/mock-d4.png)).
5. **You're set:** three things worth knowing (the command box, the site panel, workspaces),
   Make default browser, which macOS confirms with its own question, and Start browsing, which
   turns the Welcome tab into a new tab ([mock](screenshots/onboarding/mock-d5.png)).

**Phone** ([splash](screenshots/onboarding/mock-p0.png), [mocks](screenshots/onboarding/mock-p1.png), [more](screenshots/onboarding/mock-p2.png)).
No import: Android keeps each app's data from the others.

1. Welcome.
2. Make it yours: the address bar at the foot or on top (`addressBarPosition`), and the colour,
   None first.
3. The search engine, the ad blocker and Safe Browsing.
4. Default browser, through Android's own sheet (`RoleManager`, `ROLE_BROWSER`).
5. Download notifications asked here, with a reason, instead of in the middle of the first
   download as today (`Downloads.java`).
6. Start browsing.

**Settings** gets an Import section, the same rows with Import… on each, and Run setup again,
which opens the Welcome tab ([mock](screenshots/onboarding/mock-d6.png)).

## For the owner

- **"How you work" left out.** The outline had a step for the profile template, starter
  workspaces and the keymap. A template only sets the frame colour and layout, which step 3
  sets already; workspaces come from import (a browser's open tabs); the keymap is one row in
  step 3. Recommended: leave the step out, five steps on the desktop.
- **Which shells step 3 offers.** Drawn: Standard with tabs on the side, on top, and Glide.
  Fox, Minimal and Terminal stay in Settings, as the focus is Standard.
- **Safe Browsing** is a feature of its own. Google's Update API keeps a list of hash
  prefixes on the Mac and asks Google only on a prefix match, with an API key of ours. The
  row is shown only once it is built.

## What it needs that we don't have

| Feature | Where | Slice |
|---|---|---|
| A first-run marker, the Welcome page, Run setup again | Go (`core/app`), `shell/src/apps/welcome`, settings | 1 |
| The welcome over the whole window, the splash, the UI coming in at the end | the window's layout gives the welcome tab all of it, with no shell regions, until Start browsing; on the phone, the Mobile shells hide their bar for it | 1 |
| No colour until one is picked | a neutral frame in `core/tones` (light grey, ink accent, and its dark twin), the templates' and presets' teal gone | 1 |
| Make default browser | Mac: the app can't be a browser yet. Its Info.plist (`tools/package.sh`) declares no `CFBundleURLTypes` for http and https nor `CFBundleDocumentTypes` for html, and nothing in the engine takes a link macOS hands over (`application:openURLs:`), as Android's `PBOpenURL` does. Then `NSWorkspace`'s `setDefaultApplicationAtURL:toOpenURLsWithScheme:`, which asks the user itself. Android: the manifest already takes http and https links, so `RoleManager`'s `ROLE_BROWSER` request is all | 1 |
| Download notifications asked on the phone's step | an engine call that asks for `POST_NOTIFICATIONS` and reports whether it was given. Today `Downloads.java` asks the moment a download starts, so Android's prompt covers the page; that ask goes, or stays only for a user who skipped setup | 1 |
| Preview pictures of each shell | made from the mock engine by a tool, kept in the app | 1 |
| Bookmark folders | `core/bookmarks`, the Bookmarks page | 2 |
| Importers: the Chromium family (Chrome, Edge, Brave, Arc), Firefox | Go, a package of its own; Settings' Import | 3 |
| History import | `core/history` | 3 |
| Safari, behind Full Disk Access | the same package | 4 |
| Passwords | the store is the daily driver session's (`core/passwords`, `Import([]Login)`); the importers hand logins to it. Chromium's are sealed with the Keychain's "<Browser> Safe Storage" key, which macOS asks about once per browser; Firefox's with `key4.db` | 5 |
| Extensions reinstalled by id | the Web Store install path, while their flag is on | 5 |
| Safe Browsing | its own item | later |

Each slice goes out through a release of its own.

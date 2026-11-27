# The built-in sparks

Three of the owner's sparks, cleaned up, that come with the app to give a feel of what sparks
do (docs/site-features.md, "The built-in sparks"). They are system sparks
(`core/sparks/builtin.go`): the app runs these files as they are in the app, never a copy in
the profile, so a fix here reaches every install with the next update. The profile keeps only
whether each is on, by its file's name (`video-downloader`); each starts off. Settings ›
Sparks lists them under Built in, to turn on, view, export or Duplicate into the person's own
copy to edit; they can't be edited or deleted. A change here changes the spark for everyone
who has it on, so keep each one's `@name` (an import matches by it) and its storage keys as
they are. Each stands alone: a floating
overlay in a shadow root of its own, with its own look in light and dark, and nothing from the
browser. His originals, which the runtime still runs unchanged, are in
`core/sparks/testdata/site-sparks/`.

## Video downloader

Finds the videos on a page so you can save one. It runs on every https site and shows nothing
until it finds a video. Then a small round button sits at the bottom right (on a phone, a
little higher, clear of a site's own bottom bar). The button opens "Videos on this page",
headed with how many it found ("2 videos on this page"): a small panel above the button on
the desktop; on a phone, a sheet along the bottom that leaves the page undimmed, as a panel
used beside the page does (docs/design-system.md, On a phone), with a 56px row per video, its
one muted line the host or what the row says (a row that can't be saved keeps its host over
what it says, as on the desktop), and its Download a round icon button.

- **What it finds**, as the owner's original does: the page's `<video>` and `<source>`
  elements (first the players it knows, such as Fluid Player), links and media attributes
  that look like video files, and video URLs in the page's own scripts. It looks again
  whenever the page adds a player, and on "Look again". `refreshVideoDownloadSources()` in
  the console does the same.
- **Streams** (docs/video-streams.md). Most sites play a stream through their own script,
  so the video's source is a `blob:` address and its playlist is named nowhere in the page.
  It finds every playlist (HLS's `.m3u8`, DASH's `.mpd`) the page loaded, from the page's resource timing
  (`performance.getEntriesByType('resource')`, and a buffered `PerformanceObserver` for
  those loaded later, as by a player that waits for play), and the page's own description
  of its video: `og:video`, a `VideoObject`'s `contentUrl`, and playlist addresses in
  player settings written into the page. A player that fetches from a worker isn't in the
  page's timing, and is missed. It fetches each playlist itself, as the page's player did,
  and reads it (`parsePlaylist` for HLS, `parseManifest` for DASH, `readStream`): a master
  playlist is one row, named by its best quality ("Video · 1080p", or "Video" where it has
  none: a stream is a video to the person, never a "stream"), and the playlists it names get
  no rows of their own; a DASH playlist, which holds every quality itself, is a row by its
  best picture.
  A live stream's row says "Live: can't be saved yet" and a protected one's "Protected:
  can't be saved", with no Download. One whose playlist can't be read keeps its Download,
  and the browser says what it is.
- **Their order:** what the page really loaded comes before what it only links. The
  script's own fetch of a playlist it found in a link shows in the page's timing too, and
  doesn't count as the page loading it.
- **Each row** is one video: its type and height where the page says them ("MP4 · 720p"),
  else "Video 1", and the host it comes from. Download is a plain link to the file with
  `#pb-download` on its end, which Project B saves as a download from the page, another
  site's file too, so the browser's own Downloads takes it over, with its Pause, Cancel and
  Retry; a right click or a long press on it has the browser's link menu. A tap turns it to
  a tick in the accent, the row saying "Downloading: see Downloads". A stream's Download
  is the same plain link, to its playlist: Project B saves a playlist as one MP4. For a
  master playlist it is the best quality's own playlist, and the row's menu lists each
  quality, best first, "Download 720p", each a plain link to that quality's playlist; when
  the stream's sound is a playlist of its own, a quality's playlist alone would be silent,
  so Download is the master playlist, which Project B saves at its best with its sound, and
  each lesser quality in the menu is the master playlist asked for at that height,
  `#pb-download=<title>&q=<height>`, of which Project B saves the closest with its sound
  (docs/video-streams.md, "A quality asked for"). A DASH stream's Download is its one
  playlist, saved at its best, and its menu's lesser qualities ask for their heights the
  same way. One of each height is listed. The row's menu has Copy link and Open in new tab.
- **Several at once.** Where two or more rows have a Download, the panel's menu has
  "Download all (4)" and "Select". Download all asks once, in a bar under the head,
  "Download 4 videos?", with Cancel and Download. Select puts a tick box on each row in
  place of its actions; the head is an All box where the rows' boxes stand, mixed while
  only some are ticked, then "3 selected", and Done in place of the close button; the bar
  has Download, and Done or Escape goes back to the list. A live or protected row, one
  that can't be saved and one already downloading can't be ticked and aren't counted.
  Under the bar's buttons, while more than one would be saved, a muted line says "Your
  browser will ask once to allow several downloads from this site.": the script can't
  tell whether the site is allowed already, so it says so each time (ux, 2026-11-27).
  **The quality** shows in the bar only where it is a choice, when one of the videos to
  save is a stream of more than one quality, or one whose playlist the script couldn't
  read, as on a video server that sends no `Access-Control-Allow-Origin`: Project B can
  read it, so it may hold several, and its link carries `&q=` (`hasQualities`); with none, each is saved as
  it is, whatever the site has kept. It is Best, 1080p, 720p, 480p or 360p: each
  stream is saved at the quality closest to that height, above or below, the smaller of
  two as close (`savedAt`). Where each quality has a playlist of its own, the link is that
  one's; where one address holds them all (sound apart, DASH) or the playlist couldn't be
  read, it is the address with `&q=<height>`, and Project B chooses by the same rule. A
  plain file is saved as it is. The downloads start one after another, 400 ms apart
  (`BATCH_GAP`), each a click on a plain link as a row's Download is (sent to the link as
  an event, never by `click()`, which a site's script may have replaced), named from its title
  the same way, its row turning to "Downloading: see Downloads". Only the first has the
  person's click behind it, so Project B asks "*site* wants to download several files"
  before the rest, as for any page.
- **A video's title** rides in its link, `#pb-download=<title>`, for Project B to name the
  file after when the file's own name says nothing, an id or `master` (its ending stays the
  file's own): the video's `aria-label` or `title`, then its `figcaption`, then the nearest
  heading above it, then the page's `og:title` (`titleNear`). A stream its player loaded,
  which no element names, goes by the page's largest video's; one the page only links, by
  the link's own words, which are nearer. With none, the link is the
  plain `#pb-download`.
- **A page of episodes** names or loads several videos, and each takes the one title near
  the player, the show's, so the files came out "Show", "Show (1)". The row in use, where
  that can be told and its title is another row's too, takes the page's own title, which
  says the episode (`inUse`, `pageTitle`). In use is, in order: the row whose address is
  the source of the one video that is playing or has played, which is certain; for a
  player that feeds the video itself, whose source is a `blob:`, the one row whose
  playlist is in the folder the page last fetched a piece of video from (resource timing,
  kept as it comes in `state.pieces`), since a stream loaded ahead stops after its
  playlist while the one playing goes on asking for pieces; and failing both, the one
  playlist the page loaded where it only names the rest. Two rows' playlists in one
  folder, pieces kept elsewhere, or nothing loaded: it can't tell, and every row keeps
  its title. The page's title loses its site's name where that ends it after a dash or
  a bar, matched to `og:site_name` or the host; any other last part stays. The panel
  reads the page again each time it opens.
- **Its data:** one value in the site's own `localStorage`, as Page cleanup keeps its own:
  `video_downloader_quality`, the quality last chosen there for saving several (`1080`,
  `720`, `480` or `360`; removed for Best), so each site keeps its own and can read or
  clear it. Nothing else, there or anywhere.
  "Hide on this page", in the panel's menu, removes it until the page loads again.
- **To change:** the `@match` line (which sites it runs on), the lists of player selectors
  and attributes in `collectSources`, `looksVideoLike` (what counts as a video URL),
  `STREAM_NOTES` (what a stream with no Download says), `QUALITIES` (the heights offered
  for several) and `BATCH_GAP` (the wait between their starts), and the colours at the top
  of `STYLES`.
- **Its test:** `node shell/e2e/video-downloader.cjs` runs it in a browser on pages served
  for it, with core/streams' test streams, saving several among them (the links followed,
  their quality, title and spacing); the real app's is `tools/drive_first_app.py
  videodl streams`.

## Screen time limit

`screen-time-limit.user.js`, on every site. It counts the time spent on each site, a day at a
time, only while its tab is in view. After 30 minutes it covers the page and pauses its videos:
"Daily limit reached" the first time, "Limit reached again" once more time has been added, and
"It's late" between 10 PM and 6 AM. "More time…" asks a sum, then "How much more?" offers
5 minutes, 15 minutes, 30 minutes and 1 hour, with "Rest of today" under More options the first
time by day. Each time more is added the sum gets harder and the choices get shorter; at night they
stop at 30 minutes. In the last 5 minutes a small countdown, "4:59 left", sits at the top right.

**Its data** is in the site's own `localStorage`, one set of keys for each host:
`screentime_seconds_<host>` (seconds today), `screentime_date_<host>` (the day they are for;
a new day starts again at 0), `screentime_snooze_<host>` (when the added time ends, in ms) and
`screentime_snooze_count_<host>` (how many times time was added today). Clearing the site's
data clears them.

**What to change**: the `CONFIG` object at the top: `TIME_LIMIT_MINUTES` (30),
`NIGHT_START_HOUR` (22) and `NIGHT_END_HOUR` (6), `NIGHT_MAX_SNOOZE` (30 minutes) and
`PUZZLE_ENABLED` (true; false goes straight to "How much more?"). Its `@match` is
`<all_urls>`; narrow it, or switch the spark off for a site, to limit only some sites.

## Page cleanup

`page-cleanup.user.js`, on every site. Hides any part of a page the person doesn't want to see,
and keeps it hidden on that site's later visits.

- **Using it:** a 40px button docked to the right edge, in the lower half of the screen (above
  a phone page's own bottom bar), opens a small menu: a floating panel on the desktop, a bottom
  sheet on a phone. "Hide parts of this page" outlines what is under the pointer or finger, and
  a tap or click hides it at once, with "Hidden. Undo" for 5 seconds. Clicks then don't reach
  the page. Done, or Escape, ends it. The page as a whole (its `html`, `body` and `main`) can't be
  hidden. "Show hidden parts (N)" shows when there are some, and brings them all back.
- **Its data:** the site's own `localStorage`. `page_cleaner_hidden_<host>` is a list of CSS
  selectors, one for each part hidden; `page_cleaner_fab_position` is where the button sits
  (`{"edge":"right","y":100}`, `y` in pixels from the bottom). A hidden element carries
  `data-pc-hidden`. Clearing the site's data shows everything again.
- **What the person can change:** drag the button up or down, or to the left edge. In the
  source: `@match` to run it on fewer sites, the colours at the top of `STYLE`, and
  `DISALLOWED_TAGS` for what may never be hidden.


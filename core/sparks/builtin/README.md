# The built-in sparks

Three of the owner's sparks, cleaned up, that come with the app to give a feel of what sparks
do (docs/site-features.md, "The built-in sparks"). A profile is given them once, off, by
`AddBuiltins` (`core/sparks/builtin.go`); from then on each is the person's own spark, to turn
on, edit, export or delete. An update never changes them. Each stands alone: a floating
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
one muted line the host or what the row says, and its Download a round icon button.

- **What it finds**, as the owner's original does: the page's `<video>` and `<source>`
  elements (first the players it knows, such as Fluid Player), links and media attributes
  that look like video files, and video URLs in the page's own scripts. It looks again
  whenever the page adds a player, and on "Look again". `refreshVideoDownloadSources()` in
  the console does the same.
- **Each row** is one video: its type and height where the page says them ("MP4 · 720p"),
  else "Video 1", and the host it comes from. Download is a plain link to the file with
  `#pb-download` on its end, which Project B saves as a download from the page, another
  site's file too, so the browser's own Downloads takes it over, with its Pause, Cancel and
  Retry; a right click or a long press on it has the browser's link menu. A tap turns it to
  a tick in the accent, the row saying "Downloading: see Downloads". A stream
  (`.m3u8`, `.mpd`) can't be saved as one file and has no Download. The row's menu has
  Copy link and Open in new tab.
- **Its data:** none. It keeps nothing in the site's storage or anywhere else.
  "Hide on this page", in the panel's menu, removes it until the page loads again.
- **To change:** the `@match` line (which sites it runs on), the lists of player selectors
  and attributes in `collectSources`, `looksVideoLike` (what counts as a video URL), and
  the colours at the top of `STYLES`.

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


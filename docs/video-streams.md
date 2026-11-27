# Video streams

Saving a streamed video (HLS, `.m3u8`) as one MP4, from any page, whatever player the page
uses. Agreed with the owner on 2026-11-27. DASH (`.mpd`) comes next, on the same pieces;
DRM-protected video is never saved.

## How a stream works

A streaming site doesn't send one file. It cuts the video into pieces of a few seconds and
lists them in a playlist, a text file starting `#EXTM3U`. A master playlist names the
qualities (each `#EXT-X-STREAM-INF` with its `RESOLUTION` and `BANDWIDTH`), each quality has
a media playlist of pieces, and the player fetches the pieces in turn. The pieces are MPEG-TS
(`.ts`) or, in newer streams, fragments of MP4 with one header piece named by `#EXT-X-MAP`.
A quality may carry its sound in a separate media playlist (`#EXT-X-MEDIA:TYPE=AUDIO` with a
`URI`). Pieces may be locked with AES-128 (`#EXT-X-KEY:METHOD=AES-128`), whose key the
playlist names and the page can fetch. A playlist with no `#EXT-X-ENDLIST` is live.

Most sites play a stream through their own script and Media Source, so the video element's
source is a `blob:` address and the playlist's address is nowhere in the page; the network
is where it shows.

## The three parts

1. **The video downloader spark finds the videos and offers them** (`core/sparks/builtin`).
   It stays in the page's own world, as every spark does.
2. **The app turns a download that is an HLS playlist into one MP4** (Go, `core/streams`,
   with one engine primitive below it). It is reached only through doors that already exist:
   a person's click or long press on a playlist link, a spark's `#pb-download` link
   (docs/site-features.md), and an extension's `chrome.downloads.download`. There is no API
   of our own for it, for sparks or anyone: a spark runs in the page's world, so whatever a
   spark can call the page can call too, and the rule is that the capability is safe for any
   page to start.
3. **Extensions get `chrome.downloads`** (`download` first), answered in Go like `tabs`
   (docs/extensions.md), so any extension that downloads a playlist gets the MP4.

## Finding videos (the spark)

The spark keeps what it finds today, and adds streams:

- **Every playlist the page loaded,** read from the page's resource timing
  (`performance.getEntriesByType('resource')` and a buffered `PerformanceObserver`), whose
  entries include the player's own fetches. So a stream loaded before anyone presses play is
  found. A player that fetches from a worker isn't in the page's timing: the spark misses
  those, and the main player libraries are checked for it first (hls.js, video.js, Shaka,
  JW Player).
- **The page itself:** each video element's real source and its `<source>` children, links to
  playlists and video files, the page's own description of its video (`og:video`, a
  `VideoObject`'s `contentUrl`), and playlist or video addresses in player settings written
  into the page.
- **Players that load nothing until play** show up once someone plays them; there is no Find.
- **For each stream** the spark fetches the playlist itself (the page's player already did,
  so the site allows it). A master playlist becomes one row with its qualities, best first;
  the row's Download takes the best, and its menu lists each quality. A live playlist's row
  says "Live: can't be saved yet", and a protected one's "Protected: can't be saved", with no
  Download.
- **Download** is a plain link to the chosen quality's media playlist with `#pb-download`,
  as a file's is today.

## Saving a stream (the app)

- **When.** A download whose type is `application/vnd.apple.mpegurl` or
  `application/x-mpegurl`, or whose address ends in `.m3u8`, or whose first bytes are
  `#EXTM3U`, is a stream. The app reads the playlist instead of keeping the text file.
- **Which quality.** A media playlist is saved as it is. A master playlist is saved at its
  best quality: the highest resolution, then the highest bandwidth.
- **Refused, plainly, in the download's row:**
  - Live (no `#EXT-X-ENDLIST`): "Live streams can't be saved yet".
  - Protected: any key method other than `NONE` or `AES-128`, or a `KEYFORMAT` other than
    `identity` (FairPlay, Widevine, PlayReady): "This video is protected and can't be
    saved". DRM is out for good.
- **Fetching.** Every playlist, piece and key is fetched through Chromium's own network, in
  the tab's storage partition, as from the page: its origin as the initiator, its site as
  the site for cookies, its address as the referrer. Chromium decides which cookies go to
  which server and keeps `SameSite` cookies off cross-site requests; Go never sees or handles
  a cookie. A few pieces at a time (4), each written to a temporary file.
- **Only the internet, from the internet.** Pieces and keys may be `http` or `https` only. A
  playlist from a public address may not fetch from a private or local one (a router, a
  service on this computer): the request carries a client security state that makes the
  network service refuse it, as Chromium's local network access does for pages.
- **Unlocking.** AES-128 pieces are decrypted with the key the playlist names, the IV from the
  playlist or the piece's sequence number, as the HLS spec says.
- **Repacking.** The pieces are repacked into one MP4 without re-encoding, sound and picture
  together (a separate sound playlist is fetched and joined). It is a regular MP4 with its
  index at the front, so the Mac's player, galleries and phones play it. MPEG-TS pieces and
  MP4 fragments are both handled.
- **Libraries first.** Playlists: Eyevinn's `hls-m3u8`. TS pieces: bluenviron's
  `mediacommon` (MIT). MP4: Eyevinn's `mp4ff` (MIT), which also reads DASH's MP4 pieces
  later. A part none of them covers well is written as a separate module of ours, with its
  own tests.
- **Its name** is the server's or the playlist's, with `.mp4`, never one the page chose, so a
  page can't pass a file off under another name.
- **In Downloads** it is a download like any other: its row by its name, progress counted in
  pieces received, Pause, Resume, Cancel and Retry, Open and Show in Finder once done. On
  Android it keeps going with no screen, as a download does (docs/daily-driver.md), and it
  lands in the shared Downloads.

## A page starting downloads

Today nothing limits the downloads a page starts by itself. Chrome lets a site's first
download go and asks before more go without a click ("This site is trying to download
multiple files", Allow or Block), and so does the app now
(`WebContentsDelegate::CanDownload`). A download started by a click goes, as it does. The
answer holds for the site until the app quits. This comes first, as its own change, and
holds for every download, streams among them. The app's own question reads as its other
prompts do, "*site* wants to download several files", with Block and Allow (ux, 2026-11-27).

## Guards

- Go unit tests over small streams made for the tests (a synthetic picture and tone, TS and
  MP4 fragments, AES-128, separate sound, master playlists, live, protected), checking the
  quality chosen, the refusals, and that the MP4 has every frame in order, its length and
  both tracks.
- The fetch's rules: a test that a piece on a private address from a public playlist is
  refused, and that only `http` and `https` are fetched.
- The page-download limit: a test of first, second and clicked downloads.
- Real-app checks: drive_first_app and drive_android save a local test stream from a page
  through the spark, and from a plain link, and play the MP4's length back.

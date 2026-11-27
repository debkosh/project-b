# Video streams

Saving a streamed video (HLS, `.m3u8`, and DASH, `.mpd`) as one MP4, from any page,
whatever player the page uses. Agreed with the owner on 2026-11-27; DASH, on the same
pieces, the same day ([DASH](#dash), below). DRM-protected video is never saved.

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
- **As built, where the sound is separate.** A quality's media playlist alone has no sound
  when the master playlist names the sound as a playlist of its own (`EXT-X-MEDIA` with a
  `URI`), and a plain link can name only one playlist. So for such a stream the row's
  Download is the master playlist, which the app saves at its best quality with its sound,
  and the menu offers no other quality. Offering the lesser ones with their sound needs a
  way for a link to say which quality of a master is meant: open, for the owner.

## Saving a stream (the app)

- **When.** A download whose type is `application/vnd.apple.mpegurl` or
  `application/x-mpegurl`, or whose address ends in `.m3u8`, or whose first bytes are
  `#EXTM3U`, is a stream. The app reads the playlist instead of keeping the text file.
  A plain click on a link to an HLS playlist is a download only when its server says so
  (`Content-Disposition: attachment`): otherwise the link opens in the tab and Chromium's
  own player plays it there, as a link to any video file does, and the stream is saved from
  the spark's Download or the link's menu. A DASH playlist's link always downloads, since
  Chromium plays none.
- **Which quality.** A media playlist is saved as it is. A master playlist is saved at its
  best quality: the highest resolution, then the highest bandwidth.
- **Refused, plainly, in the download's row,** worded as any failed download is (ux,
  2026-11-27), and in a phone's bar as "Couldn't download *name*: live videos can't be
  saved yet" and so on:
  - Live (no `#EXT-X-ENDLIST`): "Failed: live videos can't be saved yet". It keeps Retry,
    for when the stream has ended.
  - Protected: any key method other than `NONE` or `AES-128`, or a `KEYFORMAT` other than
    `identity` (FairPlay, Widevine, PlayReady): "Failed: this video is protected", with no
    Retry. DRM is out for good.
  - A stream of pieces the app can't repack: "Failed: this video's format can't be saved
    yet", with no Retry.
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

## DASH

The other common kind of stream (the owner, 2026-11-27: "DASH (.mpd), on the same pieces as
HLS"). Its playlist is one XML file, an MPD, that names every quality: each a
`Representation` in an `AdaptationSet`, the picture's and the sound's apart, each cut into
MP4 fragments with a header piece.

- **When.** A download whose type is `application/dash+xml`, or whose address ends in
  `.mpd`, or whose body is XML with an `MPD` root, is a stream, as an HLS playlist is.
- **Which quality.** The best picture, the `Representation` of the greatest height and
  then the greatest bandwidth, and the best sound: the greatest bandwidth of the first
  `AdaptationSet` with sound, the stream's own first language. Only those in MP4; WebM
  ones are passed over. The two are joined into one regular MP4 with its index first.
- **Its pieces** are named in any of DASH's ways: a `SegmentTemplate` with `$Number$` or
  `$Time$` (and `$RepresentationID$`, `$Bandwidth$`, a width as `$Number%05d$`), with one
  length for every piece or a `SegmentTimeline`; a `SegmentList`; or a `SegmentBase`, one
  file whose own index, a `sidx` box at the byte range the playlist gives, says where each
  piece is, each then fetched by its byte range. What a `Representation` leaves out, its
  `AdaptationSet` and its `Period` give, `BaseURL`s too. Each `Period` goes on where the
  one before ended.
- **Refused, in the row,** in the same words: live (`type="dynamic"`), "Failed: live videos
  can't be saved yet"; protected, any `ContentProtection` element anywhere, or pieces whose
  header says they are encrypted, "Failed: this video is protected", with no Retry.
- **The same fetch rules** as HLS: as from the page, only `http` and `https`, and nothing
  on this computer or its network for a playlist from the internet.
- **The spark** offers a `.mpd` stream as it does an HLS one: found from the page's
  resource timing and its own description, read by the spark, its row named by its best
  picture, live and protected ones with no Download. One address holds every quality, so
  the row offers the best only, as for an HLS stream whose sound is separate.
- **Read with** Eyevinn's `dash-mpd` (MIT); the index and the fragments with `mp4ff`.
- **Guards:** core/streams' tests on small streams made with ffmpeg's DASH writer, each
  MP4 held frame by frame to what ffmpeg makes of the same pieces: a template by number,
  a timeline, the same by `$Time$`, a `SegmentList` of byte ranges and a `SegmentBase`
  with its index; the choice of quality and each way of naming pieces; live, protected by
  the playlist and by the pieces themselves; and the fetch's rules.

## How it is built

**The fetch** (`fetch` and `cancel_fetch` in `pb_abi.h`, `engine/browser/fetch.cc`, Go's
`core/fetch`). Go asks the engine to fetch one address into a file, as a page at an address
in a workspace would, and hears how it ended (`PB_EVENT_FETCHED`: done, refused, the
connection, or the server, with the HTTP status, the address space of the server that
answered, the address after redirects, the type and the server's name for the file). It is
asked by the page's address and its workspace, not by a tab, so a stream goes on when its tab
closes, as a download does. The request goes through the workspace's own network
(`GetURLLoaderFactoryForBrowserProcess`, a `SimpleURLLoader` writing the file) with the
page's origin as its initiator, the page's site as its site for cookies and its isolation
info, and the page's address as its referrer under the web's default policy, with
credentials included and no CORS asked, as a page's own request for another site's file. So
the cookies are Chromium's to choose, and none reaches Go.

It is made from an address space, the one the playlist that named the address came from
(this computer, its network, or the internet; a download says which server it came from,
`PB_EVENT_DOWNLOAD_UPDATED`'s last value). The request carries that as its client security
state with the policy that blocks, so the network service refuses a server on a more private
address at the connection, a redirect's too, as it does for a page
(`LocalNetworkAccessChecker`). The person's own request, with no page, carries none. Go's
`core/fetch` holds both rules on its side as well: an address that isn't `http` or `https`
never reaches the engine, and an answer from a more private address than was asked from is
thrown away.

**Which download is a stream** (`core/downloads/streams.go`). When the engine asks where a
download goes, its type or its address may already say it is a playlist: it then goes to a
folder of the app's own in the profile, never the Downloads folder, with no Save panel, and
its row shows the MP4's name from the start. One that only its first bytes give away is
known once it is all here, and its file is taken out of the Downloads folder again. So that
this works on a phone too, the engine no longer hands a finished download to the system by
itself: Go asks for each one it keeps (`publish_file`), and never for a playlist. Either
way the playlist's text is read, its file removed, the engine's download dropped, and the
row goes on as the stream's, under the same id, so a phone's notification for it carries
on and the app stays alive behind it with no screen, exactly as for a running download:
Android counts a download as running from what Go shows of it (`show_download`), and Go
shows the stream's row as running until its MP4 is made.

**The job** (`core/streams/job.go`). It reads the playlist (`hls.go`, with hls-m3u8): a
master's best quality, whose own playlist it fetches, and its separate sound's; live and
protected ones end there. A DASH playlist (`dash.go`, with dash-mpd) gives its picture's
and its sound's pieces at once, but for a file that keeps its own index, which is fetched
and read first. Then the pieces, 4 at a time, each to a file in the job's folder,
picture and sound turn about, at most 8 ahead of what is repacked; a key or a header piece
(`EXT-X-MAP`, DASH's initialization) is fetched once. A piece that fails is asked for 3 times. Each playlist's
pieces are asked from where that playlist's server is. Pause stops the fetches under way
and keeps what is in; Resume asks for them again; Cancel and Retry throw the folder away,
and Retry starts over from the playlist's address. Progress is the pieces in, of how many;
the row shows it in bytes, the bytes in and what the whole comes to at that rate, so the
bar moves by pieces and the sizes mean something.

**Repacking** (`ts.go`, `work.go`). Everything becomes one working file of MP4 fragments,
picture and sound side by side as they arrive. A piece that is already a fragment is added
as it is. An MPEG-TS piece is read with mediacommon (H.264 or H.265, and AAC) and written
as fragments with mp4ff, each sample with the piece's own stamps; a picture's last sample
waits for the next piece to say how long it lasts; other parameter sets after a
discontinuity get a sample entry of their own. At the end the times are set, with nothing
re-encoded: each run between discontinuities starts where the last ended, every track moved
by the same amount, the stream's own edits honoured (a picture's reordering delay, a wait
before a track), and the earliest track starting at nothing. mp4ff's `Defragment` then
writes the regular MP4, its index first and 64-bit offsets when it is large, copying the
samples as they are, and the job's folder goes.

**Not yet:** sound other than AAC in MPEG-TS (AC-3, MP3), sound-only pieces that are bare
AAC or MP3 files, MP4 fragments whose header changes its codec partway, DASH in WebM, and a
DASH index of indexes: each fails as "This video's format can't be saved yet" (a stream
with nothing in MP4, as "This video has nothing to save"). With Ask where to save on, a stream's MP4 still
goes to the Downloads folder.

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

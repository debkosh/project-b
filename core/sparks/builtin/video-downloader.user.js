// ==UserScript==
// @name         Video downloader
// @description  Finds the videos on a page so you can save one
// @version      1.4
// @namespace    https://github.com/sudiptadeb/project-b
// @match        https://*/*
// @run-at       document-end
// ==/UserScript==

// Kept in the site's own localStorage: video_downloader_quality, the quality last chosen
// there for saving several videos at once (1080, 720, 480 or 360; nothing for Best).

(function () {
  'use strict';

  const ROOT_ID = 'custom-video-downloader-widget';

  // Re-running the script replaces the widget instead of duplicating it.
  const previous = document.getElementById(ROOT_ID);
  if (previous) {
    previous.dispatchEvent(new Event('vd-replaced'));
    previous.remove();
  }

  const VIDEO_EXT_RE = /\.(mp4|m4v|webm|ogv|ogg|mov|m3u8|mpd)(?:[?#]|$)/i;
  const URL_LIKE_RE = /https?:(?:\\?\/|\\u002f){2}[^\s"'<>]+/gi;
  // A playlist of a streamed video, HLS's or DASH's, by its address's path.
  const PLAYLIST_RE = /\.(m3u8|mpd)(?![a-z0-9])/i;
  // A piece of a streamed video, by its address's path.
  const PIECE_RE = /\.(ts|m4s|m4v|m4a|mp4|aac|cmfv|cmfa)$/i;

  const state = {
    sources: [],
    // What each stream's playlist says, by its address: see readStream.
    streams: new Map(),
    // Playlists the page loaded, from its resource timing, in the order they came.
    loaded: new Set(),
    // The timing lists the script's own fetches too, each playlist it reads: fetched counts
    // every fetch of a playlist seen there (seen keeps each entry from counting twice), and
    // own the script's, so one the page loaded is one fetched more often than the script did.
    fetched: new Map(),
    own: new Map(),
    seen: new Set(),
    // When the page last fetched a piece of video from each folder, by the folder's address:
    // the stream in use is the one still being fetched from (inUse).
    pieces: new Map(),
    open: false,
    menu: null, // { trigger } while a menu is open
    shownKey: null, // what the list shows now
    pending: null, // rows to show once the open menu closes
    downloading: new Set(),
    rows: [], // what the list shows, as describe gives it
    // 'select' while rows are ticked to save several, 'confirm' while Download all asks.
    mode: null,
    picked: new Set(), // the ticked rows' addresses
    batch: [], // the timers of downloads still to start
  };

  // The qualities several videos can be saved at, as heights: 0 is each one's best.
  const QUALITIES = [0, 1080, 720, 480, 360];
  const QUALITY_KEY = 'video_downloader_quality';
  // How long after one download of several the next starts.
  const BATCH_GAP = 400;

  // The quality last chosen on this site, kept in its own storage; Best where the site
  // keeps none or has none to keep it in.
  let quality = 0;
  try {
    const kept = Number(localStorage.getItem(QUALITY_KEY));
    if (QUALITIES.includes(kept)) quality = kept;
  } catch {
    // No storage here.
  }

  function chooseQuality(height) {
    quality = QUALITIES.includes(height) ? height : 0;
    try {
      if (quality) localStorage.setItem(QUALITY_KEY, String(quality));
      else localStorage.removeItem(QUALITY_KEY);
    } catch {
      // Chosen until the page goes, then.
    }
  }

  let refreshTimer = null;
  let observer = null;
  let loads = null;

  // ---- Finding videos (as the original) ----

  function normalizeUrl(raw) {
    if (!raw || typeof raw !== 'string') return '';

    let url = raw
      .trim()
      .replace(/\\u002f/gi, '/')
      .replace(/\\\//g, '/')
      .replace(/&amp;/g, '&')
      .replace(/[)"'<>;,]+$/g, '');

    if (!url) return '';

    if (url.startsWith('//')) {
      url = location.protocol + url;
    }

    // Ignore blob: and data: URLs, not downloadable outside the page session.
    if (url.startsWith('blob:') || url.startsWith('data:')) {
      return '';
    }

    try {
      return new URL(url, location.href).href;
    } catch {
      return '';
    }
  }

  function looksVideoLike(url) {
    if (!url) return false;

    const lower = url.toLowerCase();

    return (
      VIDEO_EXT_RE.test(lower) ||
      lower.includes('.mp4') ||
      lower.includes('.m3u8') ||
      lower.includes('/video/') ||
      lower.includes('video.mp4') ||
      lower.includes('manifest') ||
      lower.includes('playlist')
    );
  }

  // The address Project B saves as a download from this page: the file's, its fragment
  // swapped for #pb-download, which no server sees. With a title, #pb-download=<title>:
  // Project B names the file after it when the file's own name says nothing, an id or
  // "master" (its ending stays the file's own). With a height, &q=<height> after the title:
  // the quality wanted of a playlist that holds several, of which Project B saves the
  // closest (docs/video-streams.md, "A quality asked for").
  function savedFrom(url, title, height) {
    try {
      const u = new URL(url, location.href);
      u.hash = title || height ? `pb-download=${encodeURIComponent(title || '')}${height ? `&q=${height}` : ''}` : 'pb-download';
      return u.href;
    } catch {
      return url;
    }
  }

  function textOf(node) {
    return (node?.textContent || '').replace(/\s+/g, ' ').trim();
  }

  // The nearest heading before an element: among what comes before it, then before each
  // of the elements it is in.
  function headingAbove(el) {
    for (let node = el; node && node !== document.documentElement; node = node.parentElement) {
      for (let prev = node.previousElementSibling; prev; prev = prev.previousElementSibling) {
        if (prev.id === ROOT_ID) continue;
        const heading = /^H[1-6]$/.test(prev.tagName) ? prev : [...prev.querySelectorAll('h1,h2,h3,h4,h5,h6')].pop();
        if (heading && textOf(heading)) return textOf(heading);
      }
    }
    return '';
  }

  // The title a video goes by on its page: its own label or title, then its caption, then
  // the nearest heading above it, then the page's og:title. '' when the page gives none.
  // A link's own words are its title: nearer than anything about the page's video.
  function titleNear(video) {
    const tries = [];
    if (video && video.tagName === 'A') {
      tries.push(video.getAttribute('aria-label'), textOf(video), video.getAttribute('title'));
    } else if (video) {
      tries.push(video.getAttribute('aria-label'), video.getAttribute('title'));
      const by = video.getAttribute('aria-labelledby');
      if (by) tries.push(by.split(/\s+/).map((id) => textOf(document.getElementById(id))).join(' '));
      tries.push(textOf(video.closest('figure')?.querySelector('figcaption')));
      tries.push(headingAbove(video));
    }
    tries.push(document.querySelector('meta[property="og:title"], meta[name="og:title"]')?.getAttribute('content'));
    for (const text of tries) {
      const title = (text || '').replace(/\s+/g, ' ').trim();
      if (title) return title.slice(0, 150);
    }
    return '';
  }

  // The page's main video: the largest on it, for a source no element names, as a stream
  // its player loaded.
  function mainVideo() {
    let best = null;
    let size = -1;
    document.querySelectorAll('video').forEach((video) => {
      if (video.closest?.(`#${ROOT_ID}`)) return;
      const rect = video.getBoundingClientRect();
      if (rect.width * rect.height > size) {
        best = video;
        size = rect.width * rect.height;
      }
    });
    return best;
  }

  function guessFilename(url) {
    try {
      const parsed = new URL(url, location.href);
      const name = parsed.pathname.split('/').filter(Boolean).pop();
      return name && name.includes('.') ? name : 'video';
    } catch {
      return 'video';
    }
  }

  // video is the element the source is of, when one is: its video, or the link to it.
  function addSource(map, rawUrl, label, priority, requireVideoLike = true, video = null) {
    const url = normalizeUrl(rawUrl);

    if (!url) return;
    if (requireVideoLike && !looksVideoLike(url)) return;

    const key = url;

    if (!map.has(key)) {
      map.set(key, {
        url,
        labels: new Set([label]),
        priority,
        video,
      });
    } else {
      const existing = map.get(key);
      existing.labels.add(label);
      existing.priority = Math.max(existing.priority, priority);
      existing.video = existing.video || video;
    }
  }

  function collectFromVideo(map, video, label, priority) {
    if (!video || video.closest?.(`#${ROOT_ID}`)) return;

    addSource(map, video.currentSrc, `${label}: currentSrc`, priority + 5, false, video);
    addSource(map, video.src, `${label}: src`, priority + 4, false, video);

    video.querySelectorAll('source[src]').forEach((source) => {
      addSource(map, source.src || source.getAttribute('src'), `${label}: <source>`, priority + 3, false, video);
    });

    [
      'data-src',
      'data-video',
      'data-video-src',
      'data-file',
      'data-url',
      'data-mp4',
      'data-hls',
      'data-dash',
    ].forEach((attr) => {
      addSource(map, video.getAttribute(attr), `${label}: ${attr}`, priority + 1, true, video);
    });
  }

  function collectSources() {
    const map = new Map();

    // 1. Known/high-priority locations.
    [
      ['#vpVid', 'known #vpVid', 120],
      ['#video-id', 'known #video-id', 115],
      ['video#vpVid', 'known video#vpVid', 120],
      ['video#video-id', 'known video#video-id', 115],
      ['.fluid_video_wrapper video', 'Fluid Player video', 110],
      ['video.js-fluid-player', 'Fluid Player class', 108],
      ['video[id*="video" i]', 'video with video-like id', 100],
    ].forEach(([selector, label, priority]) => {
      document.querySelectorAll(selector).forEach((video) => {
        collectFromVideo(map, video, label, priority);
      });
    });

    // 2. Any video/source elements.
    document.querySelectorAll('video').forEach((video) => {
      collectFromVideo(map, video, 'generic <video>', 80);
    });

    document.querySelectorAll('source[src]').forEach((source) => {
      if (source.closest?.(`#${ROOT_ID}`)) return;
      addSource(map, source.src || source.getAttribute('src'), 'generic <source>', 75, false);
    });

    // 3. Links that look like video files/playlists.
    document.querySelectorAll('a[href]').forEach((a) => {
      if (a.closest?.(`#${ROOT_ID}`)) return;
      addSource(map, a.href || a.getAttribute('href'), 'page link', 55, true, a);
    });

    // 4. Common media-related attributes.
    const attrs = [
      'src',
      'href',
      'data-src',
      'data-video',
      'data-video-src',
      'data-file',
      'data-url',
      'data-mp4',
      'data-hls',
      'data-dash',
      'data-source',
    ];

    document.querySelectorAll('*').forEach((el) => {
      if (el.closest?.(`#${ROOT_ID}`)) return;

      attrs.forEach((attr) => {
        const value = el.getAttribute?.(attr);
        if (value) {
          addSource(map, value, `attribute ${attr}`, 45, true);
        }
      });
    });

    // 5. Inline scripts / player configs.
    document.querySelectorAll('script').forEach((script) => {
      const text = script.textContent || '';
      const matches = text.match(URL_LIKE_RE) || [];

      matches.forEach((url) => {
        addSource(map, url, 'inline script/player config', 35, true);
      });
    });

    // 6. The page's own description of its video: og:video, and a VideoObject's contentUrl.
    document.querySelectorAll('meta[property^="og:video"], meta[name^="og:video"]').forEach((meta) => {
      const what = meta.getAttribute('property') || meta.getAttribute('name');
      if (what === 'og:video' || what === 'og:video:url' || what === 'og:video:secure_url') {
        addSource(map, meta.getAttribute('content'), 'og:video', 60, true);
      }
    });
    document.querySelectorAll('script[type="application/ld+json"]').forEach((script) => {
      let data;
      try {
        data = JSON.parse(script.textContent || '');
      } catch {
        return;
      }
      const walk = (node, depth) => {
        if (!node || typeof node !== 'object' || depth > 6) return;
        if (Array.isArray(node)) {
          node.forEach((item) => walk(item, depth + 1));
          return;
        }
        const type = [].concat(node['@type'] || []);
        if (type.includes('VideoObject') && typeof node.contentUrl === 'string') {
          addSource(map, node.contentUrl, 'VideoObject contentUrl', 60, true);
        }
        Object.values(node).forEach((value) => walk(value, depth + 1));
      };
      walk(data, 0);
    });

    // 7. Every playlist the page loaded, its player's own fetches among them: most sites
    // play a stream through their own script, and its address is nowhere in the page.
    noteLoaded(performance.getEntriesByType('resource'));
    state.loaded.forEach((url) => addSource(map, url, 'loaded by the page', 90, false));

    // A stream's playlist is read, to say what it is.
    map.forEach((item) => {
      if (isPlaylist(item.url)) readStream(item.url);
    });

    const main = mainVideo();
    state.sources = [...map.values()]
      .map((item) => ({
        url: item.url,
        labels: [...item.labels],
        priority: item.priority,
        title: titleNear(item.video || main),
      }))
      .sort((a, b) => b.priority - a.priority);

    updateUI();
  }

  // ---- Streams ----

  function isPlaylist(url) {
    try {
      return PLAYLIST_RE.test(new URL(url).pathname);
    } catch {
      return false;
    }
  }

  function isPiece(url) {
    try {
      return PIECE_RE.test(new URL(url).pathname);
    } catch {
      return false;
    }
  }

  // The folder an address is in: the address up to its path's last slash.
  function folderOf(url) {
    try {
      const u = new URL(url);
      return u.origin + u.pathname.slice(0, u.pathname.lastIndexOf('/') + 1);
    } catch {
      return '';
    }
  }

  // noteLoaded keeps the playlists among the resources the page loaded, and when a piece of
  // video was last fetched from each folder.
  function noteLoaded(entries) {
    entries.forEach((entry) => {
      const url = normalizeUrl(entry.name);
      const key = `${entry.name} ${entry.startTime}`;
      // A piece of video says which folder the page's player is fetching from now.
      const folder = url && isPiece(url) ? folderOf(url) : '';
      if (folder && !(state.pieces.get(folder) > entry.startTime)) state.pieces.set(folder, entry.startTime);
      if (!url || !isPlaylist(url) || state.seen.has(key)) return;
      state.seen.add(key);
      state.fetched.set(url, (state.fetched.get(url) || 0) + 1);
    });
    // Not the script's own fetch of a playlist it found elsewhere, as in a link: that is no
    // sign the page loaded it.
    let added = false;
    state.fetched.forEach((count, url) => {
      if (count > (state.own.get(url) || 0) && !state.loaded.has(url)) {
        state.loaded.add(url);
        added = true;
      }
    });
    return added;
  }

  // A tag's attributes, as HLS writes them: NAME=value or NAME="value", comma separated.
  function attributes(line) {
    const out = {};
    const re = /([A-Z0-9-]+)=("[^"]*"|[^,]*)/g;
    let m;
    while ((m = re.exec(line.slice(line.indexOf(':') + 1)))) {
      out[m[1]] = m[2].replace(/^"|"$/g, '');
    }
    return out;
  }

  // A key the browser never opens: any method but NONE and AES-128, or a key format other
  // than the plain one (FairPlay, Widevine, PlayReady).
  function lockedBy(line) {
    const a = attributes(line);
    const method = (a.METHOD || 'NONE').toUpperCase();
    if (method !== 'NONE' && method !== 'AES-128') return true;
    return !!a.KEYFORMAT && a.KEYFORMAT.toLowerCase() !== 'identity';
  }

  // parsePlaylist reads an HLS playlist fetched from base. A master playlist gives its
  // qualities, best first (the largest picture, then the greatest bandwidth), each with its
  // own playlist's address, whether its sound is a playlist of its own, and every playlist
  // it names; a media playlist says whether it is live. Either says whether it is protected.
  function parsePlaylist(text, base) {
    const lines = text.replace(/^\uFEFF/, '').split(/\r?\n/).map((line) => line.trim());
    if (!lines[0] || !lines[0].startsWith('#EXTM3U')) return null;
    const out = { master: false, live: false, protected: false, qualities: [], names: [] };
    const resolve = (ref) => {
      try {
        return new URL(ref, base).href;
      } catch {
        return '';
      }
    };
    const sound = new Set();
    let ended = false;
    let pieces = 0;
    lines.forEach((line, i) => {
      if (line.startsWith('#EXT-X-KEY:') || line.startsWith('#EXT-X-SESSION-KEY:')) {
        out.protected = out.protected || lockedBy(line);
      } else if (line.startsWith('#EXT-X-STREAM-INF:')) {
        out.master = true;
        const a = attributes(line);
        const next = lines.slice(i + 1).find((l) => l && !l.startsWith('#'));
        const url = next ? resolve(next) : '';
        if (!url) return;
        const size = (a.RESOLUTION || '').toLowerCase().split('x').map(Number);
        const quality = { url, width: size[0] || 0, height: size[1] || 0, bandwidth: Number(a.BANDWIDTH) || 0, audio: a.AUDIO || '' };
        out.qualities.push(quality);
        out.names.push(url);
      } else if (line.startsWith('#EXT-X-MEDIA:') || line.startsWith('#EXT-X-I-FRAME-STREAM-INF:')) {
        out.master = true;
        const a = attributes(line);
        if (a.URI) out.names.push(resolve(a.URI));
        if (a.URI && a.TYPE === 'AUDIO' && a['GROUP-ID']) sound.add(a['GROUP-ID']);
      } else if (line.startsWith('#EXTINF:')) {
        pieces++;
      } else if (line.startsWith('#EXT-X-ENDLIST')) {
        ended = true;
      }
    });
    if (out.master) {
      out.qualities.sort((a, b) => b.width * b.height - a.width * a.height || b.bandwidth - a.bandwidth);
      const seen = new Map();
      out.qualities.forEach((q) => {
        q.split = sound.has(q.audio);
        q.label = q.height ? `${q.height}p` : q.bandwidth ? `${Math.round(q.bandwidth / 1000)} kbps` : 'Video';
        q.short = q.label;
        seen.set(q.label, (seen.get(q.label) || 0) + 1);
      });
      // Two of one height are told apart by their bandwidth.
      out.qualities.forEach((q) => {
        if (seen.get(q.label) > 1 && q.height && q.bandwidth) q.label += ` · ${(q.bandwidth / 1e6).toFixed(1)} Mbps`;
      });
    } else {
      out.live = pieces > 0 && !ended;
    }
    return out;
  }

  // parseManifest reads a DASH playlist, an MPD: whether it is live (type="dynamic") or
  // protected (any ContentProtection), and its pictures, best first: the greatest height
  // and then the greatest bandwidth, which is what the browser saves, with the best sound.
  // One address holds every quality (whole).
  function parseManifest(text) {
    let doc;
    try {
      doc = new DOMParser().parseFromString(text, 'application/xml');
    } catch {
      return null;
    }
    const root = doc.documentElement;
    if (!root || root.localName !== 'MPD') return null;
    const out = { master: true, live: root.getAttribute('type') === 'dynamic', protected: false, qualities: [], names: [], whole: true };
    const all = [...root.getElementsByTagName('*')];
    out.protected = all.some((node) => node.localName === 'ContentProtection');
    all.filter((node) => node.localName === 'Representation').forEach((rep) => {
      const set = rep.parentElement;
      const type = (rep.getAttribute('mimeType') || set.getAttribute('mimeType') || '').toLowerCase();
      const picture = set.getAttribute('contentType') === 'video' || type.startsWith('video/');
      if (!picture || (type && !type.includes('mp4'))) return;
      const height = Number(rep.getAttribute('height') || set.getAttribute('height') || set.getAttribute('maxHeight')) || 0;
      const bandwidth = Number(rep.getAttribute('bandwidth')) || 0;
      out.qualities.push({ height, bandwidth, label: height ? `${height}p` : 'Video', short: height ? `${height}p` : 'Video' });
    });
    out.qualities.sort((a, b) => b.height - a.height || b.bandwidth - a.bandwidth);
    return out;
  }

  // readStream fetches a stream's playlist, once, as the page's player did, and keeps what
  // it says in state.streams: { status, qualities, names, save }. status is 'checking' until
  // it is read; then 'ok', 'live' or 'protected', or 'unknown' when it couldn't be read, in
  // which case the browser finds out on Download. save is the address Download saves. Each
  // quality is { label, short, height, url } and, where one address holds them all (whole),
  // q, the height its link asks for. A master playlist's best quality is read too, since
  // only that says whether it is live.
  function readStream(url) {
    if (state.streams.has(url)) return;
    const info = { status: 'checking', qualities: [], names: [], save: url, whole: false };
    // The qualities of a playlist whose one address holds them all: the best, which the
    // address alone saves, and then one of each other height, which the address asks for.
    const byHeight = (qualities) => {
      const heights = new Set();
      return qualities.filter((q, i) => {
        if (i && (!q.height || heights.has(q.height))) return false;
        heights.add(q.height);
        return true;
      }).map((q, i) => ({ label: q.short, short: q.short, height: q.height, url, q: i ? q.height : 0 }));
    };
    state.streams.set(url, info);
    // A playlist that doesn't answer in a while is left to the browser to find out.
    const read = (address) => {
      state.own.set(address, (state.own.get(address) || 0) + 1);
      return fetch(address, { signal: AbortSignal.timeout?.(8000) });
    };
    const readList = (address) => read(address).then((response) => {
      if (!response.ok) throw new Error(String(response.status));
      return response.text().then((text) => parsePlaylist(text, response.url || address) || parseManifest(text));
    });
    const settle = (status) => {
      info.status = status;
      updateUI();
    };
    readList(url).then((list) => {
      if (!list) return settle('unknown');
      if (list.protected) return settle('protected');
      if (list.whole) {
        // DASH: one playlist for every quality, which the browser saves at its best, or at
        // the height its link asks for.
        info.whole = true;
        info.qualities = byHeight(list.qualities);
        return settle(list.live ? 'live' : 'ok');
      }
      if (!list.master) return settle(list.live ? 'live' : 'ok');
      info.names = list.names;
      if (!list.qualities.length) return settle('unknown');
      const best = list.qualities[0];
      // With sound in a playlist of its own, only the whole stream's playlist joins the two:
      // the browser saves its best quality with its sound, or the one closest to the height
      // its link asks for. Otherwise each quality's own playlist is a whole video, and each
      // can be saved.
      if (list.qualities.some((q) => q.split)) {
        info.whole = true;
        info.qualities = byHeight(list.qualities);
      } else {
        info.qualities = list.qualities.map((q) => ({ label: q.label, short: q.short, height: q.height, url: q.url }));
        info.save = best.url;
      }
      return readList(best.url).then((media) => {
        settle(media && media.protected ? 'protected' : media && media.live ? 'live' : 'ok');
      }, () => settle('ok'));
    }).catch(() => settle('unknown'));
  }

  function copyText(text) {
    navigator.clipboard?.writeText(text).catch(() => {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      textarea.remove();
    });
  }

  // ---- What a row says ----

  const TYPE_NAMES = { mp4: 'MP4', m4v: 'M4V', webm: 'WebM', ogv: 'OGV', ogg: 'OGG', mov: 'MOV' };
  const HEIGHTS = '144|240|360|480|540|576|720|1080|1440|2160';
  const URL_QUALITY_RE = new RegExp(`(?:^|[^0-9a-z])(${HEIGHTS})p(?:[^0-9a-z]|$)`, 'i');
  const ATTR_QUALITY_RE = new RegExp(`(?:^|[^0-9])(${HEIGHTS})(?:p|[^0-9]|$)`, 'i');
  const QUALITY_ATTRS = ['size', 'res', 'label', 'title', 'data-res', 'data-quality', 'data-label'];

  function isStream(url) {
    return /\.(m3u8|mpd)(?:[?#]|$)/i.test(url) || url.toLowerCase().includes('.m3u8');
  }

  // What a stream's row says under its name when it has no Download.
  const STREAM_NOTES = {
    checking: 'Checking the video',
    live: "Live: can't be saved yet",
    protected: "Protected: can't be saved",
  };

  function typeName(url) {
    try {
      const ext = new URL(url).pathname.match(/\.([a-z0-9]+)$/i);
      return ext ? TYPE_NAMES[ext[1].toLowerCase()] || '' : '';
    } catch {
      return '';
    }
  }

  function hostOf(url) {
    try {
      return new URL(url).host;
    } catch {
      return '';
    }
  }

  // Heights the page states for a URL: a <source>'s size or label, or a
  // playing video's own height.
  function qualityHints() {
    const hints = new Map();

    document.querySelectorAll('source[src]').forEach((source) => {
      const url = normalizeUrl(source.src || source.getAttribute('src'));
      if (!url || hints.has(url)) return;
      for (const attr of QUALITY_ATTRS) {
        const m = (source.getAttribute(attr) || '').match(ATTR_QUALITY_RE);
        if (m) {
          hints.set(url, `${m[1]}p`);
          return;
        }
      }
    });

    document.querySelectorAll('video').forEach((video) => {
      const url = normalizeUrl(video.currentSrc);
      if (url && video.videoHeight && !hints.has(url)) hints.set(url, `${video.videoHeight}p`);
    });

    return hints;
  }

  function qualityOf(url, hints) {
    if (hints.has(url)) return hints.get(url);
    try {
      const m = new URL(url).pathname.match(URL_QUALITY_RE);
      return m ? `${m[1]}p` : '';
    } catch {
      return '';
    }
  }

  function describe(sources) {
    const hints = qualityHints();
    // A playlist a master playlist names is that stream's quality or its sound, not a video
    // of its own: the master's row offers it.
    const named = new Set();
    state.streams.forEach((info) => info.names.forEach((url) => named.add(url)));
    const rows = sources.filter((source) => !named.has(source.url)).map((source, index) => {
      const stream = isStream(source.url);
      // A stream is a video to the person, as a file is: "Video", with its quality when known.
      const type = stream ? 'Video' : typeName(source.url);
      const info = state.streams.get(source.url);
      const quality = info && info.qualities.length ? info.qualities[0].short : qualityOf(source.url, hints);
      const name = type || `Video ${index + 1}`;
      const row = {
        url: source.url,
        label: quality && quality !== 'Video' ? `${name} · ${quality}` : name,
        host: hostOf(source.url),
        stream,
        title: source.title,
      };
      if (info) {
        // A playlist (HLS or DASH), which the browser saves as one MP4: its Download saves
        // the best quality, and its menu each of the others, where each has an address.
        row.status = info.status;
        row.save = info.save;
        row.qualities = info.qualities;
        row.whole = info.whole;
      }
      return row;
    });
    // A page of episodes loads or names several videos, and each takes the one title near
    // the player, the show's: the files come out "Show", "Show (1)". The one in use, where
    // that can be told, takes the page's own title instead, which says the episode. A row
    // whose title is its own alone keeps it.
    const used = inUse(rows);
    const page = used ? pageTitle() : '';
    const row = page && rows.find((r) => r.url === used);
    if (row && rows.some((other) => other !== row && other.title === row.title)) row.title = page;
    return rows;
  }

  // The address of the row whose video the page is playing, or '' where that can't be told.
  // In order: (1) the one video on the page that is playing, or the one that has played,
  // has a row's address as its own source, which is certain; a player that feeds the video
  // itself (Media Source) leaves it a blob: address, so (2) the folder the page last
  // fetched a piece of video from holds one row's playlist, the stream still being fetched:
  // a stream only loaded ahead stops after its playlist or a first piece, while the one
  // playing goes on asking for pieces, and a folder holding two rows' playlists says
  // nothing; and (3) the page loaded one of the rows' playlists and only names the rest.
  function inUse(rows) {
    const played = [...document.querySelectorAll('video')].filter((video) =>
      !video.closest?.(`#${ROOT_ID}`) && (video.currentTime > 0 || (!video.paused && video.readyState > 2)));
    const playing = played.filter((video) => !video.paused);
    const video = playing.length === 1 ? playing[0] : played.length === 1 ? played[0] : null;
    const source = video ? normalizeUrl(video.currentSrc) : '';
    if (source && rows.some((row) => row.url === source)) return source;

    const playlists = rows.filter((row) => isPlaylist(row.url));
    let folder = '';
    let last = -1;
    state.pieces.forEach((at, where) => {
      if (at > last && playlists.some((row) => folderOf(row.url) === where)) {
        folder = where;
        last = at;
      }
    });
    const within = playlists.filter((row) => folderOf(row.url) === folder);
    if (within.length === 1) return within[0].url;
    if (within.length > 1) return '';

    const loaded = playlists.filter((row) => state.loaded.has(row.url));
    return loaded.length === 1 && playlists.length > 1 ? loaded[0].url : '';
  }

  // The page's own title, without its site's name where that ends it after a dash or a bar:
  // only what matches og:site_name or the host is taken off, so a title's own last part
  // stays.
  function pageTitle() {
    let title = (document.title || '').replace(/\s+/g, ' ').trim();
    const flat = (text) => (text || '').toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '');
    const host = location.hostname.replace(/^www\./, '');
    const site = [document.querySelector('meta[property="og:site_name"], meta[name="og:site_name"]')?.getAttribute('content'), host, host.split('.')[0]].map(flat).filter(Boolean);
    const parts = title.match(/^(.*\S)\s+[-\u2013\u2014|\u00b7]\s+(\S.*)$/);
    if (parts && site.includes(flat(parts[2]))) title = parts[1];
    return title.slice(0, 150);
  }

  // Whether a row has a Download: not a stream that can't be saved, live or protected, one
  // still being read, or one already downloading.
  function saveable(row) {
    if (row.stream && !row.status) return false;
    return !STREAM_NOTES[row.status] && !state.downloading.has(row.url);
  }

  // The address that saves a row at the quality wanted, a height, or at its best for 0. A
  // file is saved as it is. A stream is saved at the quality closest to the height, the
  // smaller of two as close: that quality's own playlist where each is a whole video, and
  // where one address holds them all, or the playlist couldn't be read, the address with
  // the height, for Project B to choose by the same rule.
  function savedAt(row, height) {
    if (!height || !row.status) return savedFrom(row.save || row.url, row.title);
    if (row.whole || row.status === 'unknown') return savedFrom(row.url, row.title, height);
    let pick = null;
    row.qualities.forEach((q) => {
      if (!q.height) return;
      const by = Math.abs(q.height - height);
      const than = pick ? Math.abs(pick.height - height) : Infinity;
      if (by < than || (by === than && q.height < pick.height)) pick = q;
    });
    return savedFrom(pick ? pick.url : row.save || row.url, row.title);
  }

  // ---- The widget ----

  const ICONS = {
    download: '<path d="M12 4v11"/><path d="m7 10 5 5 5-5"/><path d="M5 20h14"/>',
    check: '<path d="m5 12 5 5 9-10"/>',
    more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
    close: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>',
    open: '<path d="M14 4h6v6"/><path d="M20 4 10 14"/><path d="M19 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h5"/>',
    refresh: '<path d="M20 12a8 8 0 1 1-2.3-5.7L20 8.5"/><path d="M20 4v4.5h-4.5"/>',
    select: '<rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8 12 3 3 5-6"/>',
    chevron: '<path d="m6 9 6 6 6-6"/>',
    hide: '<path d="M9.9 5.2A9.6 9.6 0 0 1 12 5c5 0 8.5 4.5 9.5 7a14 14 0 0 1-2.4 3.4"/><path d="M6.6 6.6C4.5 8 3 10.2 2.5 12c1 2.5 4.5 7 9.5 7a9.4 9.4 0 0 0 5.4-1.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/><path d="m3 3 18 18"/>',
  };

  function icon(name) {
    return `<svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">${ICONS[name]}</svg>`;
  }

  const STYLES = `
    :host {
      position: fixed;
      top: 0;
      left: 0;
      width: 0;
      height: 0;
      display: block;
      z-index: 2147483647;
    }
    .ui {
      --surface: #ffffff;
      --text: #1f1f1f;
      --muted: #5c6270;
      --line: rgba(15, 20, 25, 0.1);
      --hover: #f1f2f4;
      --accent: #23272f;
      --on-accent: #ffffff;
      --danger: #d93025;
      --shadow: 0 8px 28px rgba(16, 24, 40, 0.14), 0 1px 3px rgba(16, 24, 40, 0.08);
      font: 14px/1.4 -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
      color: var(--text);
      text-align: left;
    }
    @media (prefers-color-scheme: dark) {
      .ui {
        --surface: #2a2c30;
        --text: #e8eaed;
        --muted: #9aa0a6;
        --line: rgba(255, 255, 255, 0.12);
        --hover: #2b2f38;
        --accent: #e7eaf2;
        --on-accent: #1f1f1f;
        --danger: #f28b82;
      }
    }
    *, *::before, *::after { box-sizing: border-box; }
    .ui [hidden] { display: none; }
    svg {
      width: 16px;
      height: 16px;
      flex: none;
      fill: none;
      stroke: currentColor;
      stroke-width: 2;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
    button { font: inherit; color: inherit; margin: 0; }
    button, a { -webkit-tap-highlight-color: transparent; }
    :focus { outline: none; }
    :focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }

    .fab {
      position: fixed;
      right: 20px;
      bottom: 20px;
      width: 40px;
      height: 40px;
      padding: 0;
      border: 0;
      border-radius: 50%;
      background: var(--accent);
      color: var(--on-accent);
      box-shadow: var(--shadow);
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
    }

    .panel {
      position: fixed;
      right: 20px;
      bottom: 68px;
      width: min(360px, calc(100vw - 40px));
      max-height: min(480px, calc(100vh - 96px));
      display: flex;
      flex-direction: column;
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      overflow: hidden;
    }
    .head {
      display: flex;
      align-items: center;
      gap: 2px;
      padding: 6px 6px 6px 16px;
      border-bottom: 1px solid var(--line);
    }
    .title { flex: 1; margin: 0; font-size: 14px; font-weight: 600; }
    .icon-btn {
      width: 32px;
      height: 32px;
      padding: 0;
      border: 0;
      border-radius: 8px;
      background: transparent;
      color: var(--muted);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: none;
      cursor: pointer;
    }
    .icon-btn[aria-expanded="true"] { background: var(--hover); color: var(--text); }
    /* Only where a pointer hovers: a finger's tap leaves :hover on what it touched. */
    @media (hover: hover) {
      .icon-btn:hover { background: var(--hover); color: var(--text); }
      .btn.quiet:hover { background: var(--hover); }
      .menu button:hover, .menu a:hover { background: var(--hover); }
    }

    .list { margin: 0; padding: 0; list-style: none; overflow: auto; }
    .row { display: flex; align-items: center; gap: 6px; padding: 10px 6px 10px 16px; }
    .row + .row { border-top: 1px solid var(--line); }
    .text { flex: 1; min-width: 0; }
    .label, .sub { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .label { font-weight: 500; }
    .sub, .note { font-size: 12px; color: var(--muted); }
    .btn {
      height: 32px;
      padding: 0 12px;
      border: 0;
      border-radius: 8px;
      background: var(--accent);
      color: var(--on-accent);
      display: inline-flex;
      align-items: center;
      gap: 6px;
      flex: none;
      font-weight: 500;
      text-decoration: none;
      white-space: nowrap;
      cursor: pointer;
    }
    .btn.quiet { background: transparent; color: var(--text); border: 1px solid var(--line); }
    .btn { transition: background-color 150ms ease-out, color 150ms ease-out; }
    .btn.quiet:active { background: var(--accent); color: var(--on-accent); border-color: transparent; }
    /* A status, not a button: the quiet button's outline, its tick and word in the accent. */
    .btn.started { background: transparent; color: var(--accent); border: 1px solid var(--accent); cursor: default; }
    .btn:disabled { opacity: 0.5; cursor: default; }
    .head { min-height: 45px; }
    .head .btn { height: 28px; padding: 0 10px; margin-right: 4px; }
    /* The All box stands where each row's box does. */
    .tick.pick-every { margin-right: 10px; }
    .empty { padding: 16px; }

    /* Saving several: the bar under the head, with the quality and what it asks, and a tick
       box on each row. */
    .bar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px;
      padding: 8px 6px 8px 16px;
      border-bottom: 1px solid var(--line);
    }
    .ask { flex: 1 0 100%; margin: 0; font-weight: 500; }
    .gap { flex: 1; }
    .hint { flex: 1 0 100%; margin: 0; padding-right: 10px; font-size: 12px; color: var(--muted); }
    .quality { min-width: 0; display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); }
    .choice { position: relative; display: inline-flex; align-items: center; }
    .choice select {
      appearance: none;
      -webkit-appearance: none;
      height: 32px;
      margin: 0;
      padding: 0 30px 0 10px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--surface);
      color: var(--text);
      font: inherit;
      font-size: 14px;
      cursor: pointer;
    }
    .choice svg { position: absolute; right: 8px; color: var(--muted); pointer-events: none; }
    .pick { flex: 1; min-width: 0; display: flex; align-items: center; gap: 12px; padding-right: 10px; cursor: pointer; }
    .tick { width: 18px; height: 18px; margin: 0; flex: none; accent-color: var(--accent); cursor: pointer; }
    .tick:focus-visible { outline-offset: 3px; }
    .tick:disabled { cursor: default; }
    .tick:disabled + .text .label { color: var(--muted); }
    .empty p { margin: 0 0 12px; color: var(--muted); }

    .menu {
      position: fixed;
      min-width: 184px;
      padding: 4px;
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
    }
    .menu button, .menu a {
      width: 100%;
      height: 32px;
      padding: 0 10px;
      border: 0;
      border-radius: 6px;
      background: transparent;
      color: inherit;
      display: flex;
      align-items: center;
      gap: 10px;
      text-align: left;
      text-decoration: none;
      white-space: nowrap;
      cursor: pointer;
    }
    .menu button svg, .menu a svg { color: var(--muted); }
    .menu button:focus-visible, .menu a:focus-visible { background: var(--hover); outline: none; }

    /* A phone (docs/design-system.md, On a phone): a sheet beside the page, which it leaves
       undimmed; a one-line head; a 56px row per video with one muted line, its Download a
       round icon button, and its other actions under its menu. */
    @media (pointer: coarse), (max-width: 599px) {
      .fab { right: 16px; bottom: calc(96px + env(safe-area-inset-bottom, 0px)); width: 48px; height: 48px; }
      .panel {
        left: 0;
        right: 0;
        bottom: 0;
        width: auto;
        max-height: 75vh;
        border-width: 1px 0 0;
        border-radius: 16px 16px 0 0;
        padding-bottom: env(safe-area-inset-bottom, 0px);
        animation: vd-sheet-in 180ms ease-out;
      }
      .head { padding: 4px 4px 4px 16px; }
      .icon-btn { width: 48px; height: 48px; border-radius: 50%; }
      .icon-btn svg, .row .btn svg { width: 20px; height: 20px; }
      .btn, .head .btn { height: 44px; padding: 0 16px; }
      .head { min-height: 57px; }
      .head .btn { padding: 0 12px; margin-right: 8px; }
      .bar { padding: 4px 8px 8px 16px; }
      .choice select { height: 44px; }
      .pick { min-height: 48px; }
      .tick { width: 20px; height: 20px; }
      .row { min-height: 56px; padding: 4px 4px 4px 16px; gap: 0; }
      .row + .row { border-top: 0; }
      /* One muted line: what the row says in place of its host, but for a row that can't
         be saved, which keeps its host as on the desktop. */
      .row:not(.unsaved) .text:has(.note) .sub { display: none; }
      .row .btn {
        width: 48px;
        height: 48px;
        padding: 0;
        border: 0;
        border-radius: 50%;
        background: transparent;
        color: var(--text);
        justify-content: center;
      }
      .icon-btn:active { background: var(--hover); }
      .row .btn:active { background: var(--accent); color: var(--on-accent); }
      .row .btn.started { background: transparent; color: var(--accent); }
      .row .btn .btn-text { display: none; }
      .menu button, .menu a { height: 44px; padding: 0 12px; }
    }
    @media (prefers-reduced-motion: reduce) {
      .panel { animation: none; }
    }
    @keyframes vd-sheet-in {
      from { transform: translateY(100%); }
      to { transform: none; }
    }
  `;

  const host = document.createElement('div');
  host.id = ROOT_ID;
  const shadow = host.attachShadow({ mode: 'open' });
  shadow.innerHTML = `
    <style>${STYLES}</style>
    <div class="ui">
      <button class="fab" type="button" aria-label="Videos on this page" title="Videos on this page"
        aria-haspopup="dialog" aria-expanded="false" hidden>${icon('download')}</button>
      <div class="panel" role="dialog" aria-label="Videos on this page" tabindex="-1" hidden>
        <div class="head">
          <input class="tick pick-every" type="checkbox" aria-label="All" title="All" hidden>
          <h2 class="title" aria-live="polite">Videos on this page</h2>
          <button class="btn quiet pick-done" type="button" hidden>Done</button>
          <button class="icon-btn panel-more" type="button" aria-label="More actions"
            aria-haspopup="menu" aria-expanded="false">${icon('more')}</button>
          <button class="icon-btn panel-close" type="button" aria-label="Close">${icon('close')}</button>
        </div>
        <div class="bar" hidden>
          <p class="ask" hidden></p>
          <label class="quality">Quality
            <span class="choice"><select class="quality-choice" aria-label="Quality">${QUALITIES.map((height) => `<option value="${height}">${height ? `${height}p` : 'Best'}</option>`).join('')}</select>${icon('chevron')}</span>
          </label>
          <span class="gap"></span>
          <button class="btn quiet bar-cancel" type="button" hidden>Cancel</button>
          <button class="btn bar-go" type="button">Download</button>
          <p class="hint" hidden>Your browser will ask once to allow several downloads from this site.</p>
        </div>
        <ul class="list"></ul>
        <a class="batch-link" tabindex="-1" aria-hidden="true" hidden></a>
      </div>
      <div class="menu" role="menu" hidden></div>
    </div>
  `;

  const fab = shadow.querySelector('.fab');
  const title = shadow.querySelector('.title');
  const panel = shadow.querySelector('.panel');
  const list = shadow.querySelector('.list');
  const menu = shadow.querySelector('.menu');
  const panelMore = shadow.querySelector('.panel-more');
  const panelClose = shadow.querySelector('.panel-close');
  const pickEvery = shadow.querySelector('.pick-every');
  const pickDone = shadow.querySelector('.pick-done');
  const qualityLabel = shadow.querySelector('.quality');
  const hint = shadow.querySelector('.hint');
  const bar = shadow.querySelector('.bar');
  const ask = shadow.querySelector('.ask');
  const qualityChoice = shadow.querySelector('.quality-choice');
  const barCancel = shadow.querySelector('.bar-cancel');
  const barGo = shadow.querySelector('.bar-go');
  const batchLink = shadow.querySelector('.batch-link');

  function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
  }

  function moreButton(name) {
    const button = el('button', 'icon-btn row-more');
    button.type = 'button';
    button.setAttribute('aria-label', name);
    button.setAttribute('aria-haspopup', 'menu');
    button.setAttribute('aria-expanded', 'false');
    button.innerHTML = icon('more');
    return button;
  }

  // What a row with no Download says under its name.
  function noteOf(row) {
    // A stream whose address isn't a playlist's the script reads.
    if (row.stream && !row.status) return "This video can't be saved";
    if (row.status && STREAM_NOTES[row.status]) return STREAM_NOTES[row.status];
    return state.downloading.has(row.url) ? 'Downloading: see Downloads' : '';
  }

  // A row while several are chosen: a tick box, the row its label, and no actions. One with
  // no Download can't be ticked.
  function renderPick(row) {
    const li = el('li', 'row');
    const pick = el('label', 'pick');
    const tick = el('input', 'tick');
    tick.type = 'checkbox';
    tick.setAttribute('aria-label', `Select ${row.label}`);
    tick.disabled = !saveable(row);
    tick.checked = !tick.disabled && state.picked.has(row.url);
    tick.addEventListener('change', () => {
      if (tick.checked) state.picked.add(row.url);
      else state.picked.delete(row.url);
      drawHead();
    });
    const text = el('div', 'text');
    text.append(el('div', 'label', row.label), el('div', 'sub', row.host));
    if (noteOf(row)) text.append(el('div', 'note', noteOf(row)));
    if (tick.disabled && !state.downloading.has(row.url)) li.classList.add('unsaved');
    pick.append(tick, text);
    li.append(pick);
    return li;
  }

  function renderRow(row) {
    if (state.mode === 'select') return renderPick(row);
    const li = el('li', 'row');
    const text = el('div', 'text');
    const sub = el('div', 'sub', row.host);
    text.append(el('div', 'label', row.label), sub);
    li.append(text);

    const saves = row.save || row.url;
    if (!saveable(row) && !state.downloading.has(row.url)) {
      text.append(el('div', 'note', noteOf(row)));
      li.classList.add('unsaved');
    } else if (state.downloading.has(row.url)) {
      text.append(el('div', 'note', 'Downloading: see Downloads'));
      // The button turns to a tick in the accent, a status rather than a button, so the tap
      // is seen to have taken.
      const started = el('span', 'btn started');
      started.setAttribute('role', 'img');
      started.setAttribute('aria-label', `Downloading ${row.label}`);
      started.innerHTML = icon('check');
      started.append(el('span', 'btn-text', 'Downloading'));
      li.append(started);
    } else {
      // A plain link, so a right click or a long press has the browser's own menu for it.
      // Its address ends in #pb-download, which Project B saves as a download from this
      // page, another site's file too: a browser ignores `download` for another site's
      // file and opens it in a tab, where a site that serves its files only to its own
      // pages refuses it (docs/site-features.md).
      const link = el('a', 'btn quiet');
      link.href = savedFrom(saves, row.title);
      link.download = guessFilename(saves);
      link.setAttribute('aria-label', `Download ${row.label}`);
      link.innerHTML = icon('download');
      // A phone shows the icon alone, its name the link's label.
      link.append(el('span', 'btn-text', 'Download'));
      link.addEventListener('click', () => {
        state.downloading.add(row.url);
        // Let the browser start the download before the row changes.
        setTimeout(() => {
          const next = renderRow(row);
          li.replaceWith(next);
          next.querySelector('.row-more').focus();
        }, 0);
      });
      li.append(link);
    }

    const more = moreButton(`Actions for ${row.label}`);
    more.addEventListener('click', () => {
      // A stream's qualities, each a plain link as Download is.
      const qualities = row.status === 'ok' && !state.downloading.has(row.url) && row.qualities.length > 1 ? row.qualities : [];
      // (A stream whose playlist couldn't be read has none to list.)
      toggleMenu(more, [
        ...qualities.map((quality) => ({
          icon: 'download',
          label: `Download ${quality.label}`,
          // Where one address holds every quality, it asks for this one's height.
          href: savedFrom(quality.url, row.title, quality.q),
          run: () => {
            state.downloading.add(row.url);
            setTimeout(() => {
              if (li.isConnected) li.replaceWith(renderRow(row));
            }, 0);
          },
        })),
        {
          icon: 'copy',
          label: 'Copy link',
          run: () => {
            copyText(row.url);
            sub.textContent = 'Link copied';
            setTimeout(() => { sub.textContent = row.host; }, 1500);
          },
        },
        {
          icon: 'open',
          label: 'Open in new tab',
          run: () => window.open(row.url, '_blank', 'noopener'),
        },
      ]);
    });
    li.append(more);
    return li;
  }

  function renderList(rows) {
    list.replaceChildren();
    if (!rows.length) {
      const li = el('li', 'empty');
      li.append(el('p', '', 'No videos found yet. Play the video, then look again'));
      const again = el('button', 'btn quiet');
      again.type = 'button';
      again.innerHTML = icon('refresh');
      again.append('Look again');
      again.addEventListener('click', () => collectSources());
      li.append(again);
      list.append(li);
    } else {
      rows.forEach((row) => list.append(renderRow(row)));
    }
  }

  function updateUI() {
    if (!host.isConnected) return;

    fab.hidden = !state.sources.length && !state.open;

    const rows = describe(state.sources);
    state.rows = rows;
    // A ticked row that has gone, or has no Download any more, is no longer chosen; with
    // nothing left to save, nothing is being chosen or asked.
    const left = new Set(rows.filter(saveable).map((row) => row.url));
    state.picked.forEach((url) => {
      if (!left.has(url)) state.picked.delete(url);
    });
    if (!left.size) state.mode = null;
    drawHead();
    const key = JSON.stringify([state.mode, rows.map((row) => [row.url, row.label, row.status, row.save, row.qualities, row.title])]);
    if (key === state.shownKey) return;
    state.shownKey = key;
    // Don't pull a row out from under an open menu: show the new list when it closes.
    if (state.menu) {
      state.pending = rows;
    } else {
      renderList(rows);
    }
  }

  // The rows a press of the bar's Download would save: every one with a Download while
  // Download all asks, the ticked ones while they are chosen.
  function chosen() {
    return state.rows.filter((row) => saveable(row) && (state.mode === 'confirm' || state.picked.has(row.url)));
  }

  // Whether the quality is a choice for some of the rows: a stream of more than one, or one
  // whose playlist couldn't be read, as where its server lets no page's script read it.
  // That one may hold several, and Project B, which can read it, saves the closest.
  function hasQualities(rows) {
    return rows.some((row) => row.status === 'unknown' || (row.status && row.qualities.length > 1));
  }

  // The head, and the bar under it while several are being saved: an All box, mixed while
  // only some are ticked, "3 selected" and Done, with Download; or "Download 3 videos?"
  // with Cancel. The quality shows where it is a choice, and a line says what the browser
  // will ask before more than one is saved: the script can't tell whether the site is
  // allowed already, so it says so each time.
  function drawHead() {
    const rows = state.rows;
    const all = rows.filter(saveable);
    const selecting = state.mode === 'select';
    const asking = state.mode === 'confirm';
    const saving = state.mode ? chosen() : [];
    const picked = selecting ? saving.length : 0;
    title.textContent = selecting ? `${picked} selected`
      : rows.length === 1 ? '1 video on this page' : rows.length ? `${rows.length} videos on this page` : 'Videos on this page';
    pickEvery.hidden = !selecting;
    pickEvery.checked = picked > 0 && picked === all.length;
    pickEvery.indeterminate = picked > 0 && picked < all.length;
    pickDone.hidden = !selecting;
    panelMore.hidden = selecting;
    panelClose.hidden = selecting;
    bar.hidden = !state.mode;
    ask.hidden = !asking;
    ask.textContent = all.length === 1 ? 'Download 1 video?' : `Download ${all.length} videos?`;
    qualityLabel.hidden = !hasQualities(saving);
    barCancel.hidden = !asking;
    barGo.disabled = selecting && !picked;
    hint.hidden = saving.length < 2;
    qualityChoice.value = String(quality);
  }

  // Draws the list again, as when the way it is shown changes.
  function redraw() {
    state.shownKey = null;
    updateUI();
  }

  function setMode(mode) {
    closeMenu(false);
    state.mode = mode;
    state.picked.clear();
    redraw();
  }

  // Leaves choosing or asking, back to the plain list.
  function leaveMode() {
    setMode(null);
    if (state.open) panelMore.focus();
  }

  // Starts each row's download, one after another a moment apart, at the quality chosen:
  // each a click on a plain link, as a row's own Download is. After a page's first, Project
  // B asks whether the site may download several files.
  function saveSeveral(rows) {
    // With no quality to choose among them, each is saved as it is.
    const height = hasQualities(rows) ? quality : 0;
    rows.forEach((row, i) => {
      const start = () => {
        batchLink.href = savedAt(row, height);
        batchLink.download = guessFilename(row.save || row.url);
        // A click sent to the link, which the browser follows as it does a person's: not
        // click(), which a site's own script may have replaced, as one that opens an
        // advertisement on every click does, and then nothing is followed.
        batchLink.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, composed: true, view: window }));
        state.downloading.add(row.url);
        redraw();
      };
      if (i) state.batch.push(setTimeout(start, i * BATCH_GAP));
      else start();
    });
  }

  function firstControl() {
    return list.querySelector('a, button') || shadow.querySelector('.panel-close');
  }

  function openPanel() {
    // Which video is in use may have changed since the page was last read.
    collectSources();
    state.open = true;
    panel.hidden = false;
    fab.setAttribute('aria-expanded', 'true');
    firstControl().focus();
  }

  function closePanel(returnFocus) {
    if (!state.open) return;
    closeMenu(false);
    if (state.mode) setMode(null);
    state.open = false;
    panel.hidden = true;
    fab.setAttribute('aria-expanded', 'false');
    fab.hidden = !state.sources.length;
    if (returnFocus && !fab.hidden) fab.focus();
  }

  function toggleMenu(trigger, items) {
    const same = state.menu && state.menu.trigger === trigger;
    closeMenu(same);
    if (same) return;

    menu.replaceChildren();
    menu.setAttribute('aria-label', trigger.getAttribute('aria-label'));
    items.forEach((item) => {
      // An item with an address is a plain link, so the browser follows it itself, and a
      // right click or a long press has the browser's own menu for it.
      const button = el(item.href ? 'a' : 'button');
      if (item.href) button.href = item.href;
      else button.type = 'button';
      button.setAttribute('role', 'menuitem');
      button.innerHTML = icon(item.icon);
      button.append(item.label);
      button.addEventListener('click', () => {
        closeMenu(true);
        item.run();
      });
      menu.append(button);
    });

    menu.hidden = false;
    const rect = trigger.getBoundingClientRect();
    const width = menu.offsetWidth;
    const height = menu.offsetHeight;
    const left = Math.max(8, Math.min(rect.right - width, innerWidth - width - 8));
    let top = rect.bottom + 4;
    if (top + height > innerHeight - 8) top = Math.max(8, rect.top - height - 4);
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;

    state.menu = { trigger };
    trigger.setAttribute('aria-expanded', 'true');
    menu.querySelector('button, a').focus();
  }

  function closeMenu(returnFocus) {
    if (!state.menu) return;
    const { trigger } = state.menu;
    state.menu = null;
    menu.hidden = true;
    trigger.setAttribute('aria-expanded', 'false');
    if (state.pending) {
      renderList(state.pending);
      state.pending = null;
    } else if (returnFocus && trigger.isConnected) {
      trigger.focus();
    }
  }

  fab.addEventListener('click', () => {
    if (state.open) closePanel(true);
    else openPanel();
  });
  panelClose.addEventListener('click', () => closePanel(true));
  pickDone.addEventListener('click', () => leaveMode());
  panelMore.addEventListener('click', () => {
    // Saving several, where there are several to save.
    const count = state.rows.filter(saveable).length;
    toggleMenu(panelMore, [
      ...(count > 1 ? [
        {
          icon: 'download',
          label: `Download all (${count})`,
          run: () => {
            setMode('confirm');
            barGo.focus();
          },
        },
        {
          icon: 'select',
          label: 'Select',
          run: () => {
            setMode('select');
            (list.querySelector('.tick:not(:disabled)') || pickDone).focus();
          },
        },
      ] : []),
      { icon: 'refresh', label: 'Look again', run: () => collectSources() },
      { icon: 'hide', label: 'Hide on this page', run: () => destroyWidget() },
    ]);
  });
  list.addEventListener('scroll', () => closeMenu(false));

  // All ticks every row that can be saved, and unticks them once they all are.
  pickEvery.addEventListener('change', () => {
    const all = state.rows.filter(saveable);
    const every = all.every((row) => state.picked.has(row.url));
    state.picked.clear();
    if (!every) all.forEach((row) => state.picked.add(row.url));
    redraw();
  });
  qualityChoice.addEventListener('change', () => chooseQuality(Number(qualityChoice.value)));
  barCancel.addEventListener('click', () => leaveMode());
  barGo.addEventListener('click', () => {
    const rows = chosen();
    leaveMode();
    saveSeveral(rows);
  });

  // A tap anywhere else in the widget closes an open menu.
  shadow.addEventListener('click', (event) => {
    if (!state.menu) return;
    const path = event.composedPath();
    if (!path.includes(menu) && !path.includes(state.menu.trigger)) closeMenu(false);
  });

  menu.addEventListener('keydown', (event) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    const items = [...menu.querySelectorAll('button, a')];
    const at = items.indexOf(shadow.activeElement);
    const step = event.key === 'ArrowDown' ? 1 : -1;
    items[(at + step + items.length) % items.length].focus();
  });

  // A click outside the widget closes it, as the original's popup.
  function onDocumentClick(event) {
    if (event.composedPath().includes(host)) return;
    closeMenu(false);
    closePanel(false);
  }

  // Escape closes the innermost: the menu, then choosing several or what Download all
  // asks, then the panel.
  function onKeydown(event) {
    if (event.key !== 'Escape' || (!state.menu && !state.open)) return;
    event.preventDefault();
    event.stopPropagation();
    if (state.menu) closeMenu(true);
    else if (state.mode) leaveMode();
    else closePanel(true);
  }

  function onResize() {
    closeMenu(false);
  }

  document.addEventListener('click', onDocumentClick);
  document.addEventListener('keydown', onKeydown, true);
  window.addEventListener('resize', onResize);

  function teardown() {
    observer?.disconnect();
    loads?.disconnect();
    clearTimeout(refreshTimer);
    state.batch.forEach((timer) => clearTimeout(timer));
    document.removeEventListener('click', onDocumentClick);
    document.removeEventListener('keydown', onKeydown, true);
    window.removeEventListener('resize', onResize);
    if (window.refreshVideoDownloadSources === collectSources) {
      delete window.refreshVideoDownloadSources;
    }
  }

  // "Hide on this page": gone until the page loads again.
  function destroyWidget() {
    teardown();
    host.remove();
  }

  host.addEventListener('vd-replaced', teardown);

  (document.body || document.documentElement).appendChild(host);
  collectSources();

  // Re-scan when dynamic players inject video/source tags after page load.
  observer = new MutationObserver(() => {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(collectSources, 500);
  });

  observer.observe(document.documentElement, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: [
      'src',
      'href',
      'data-src',
      'data-video',
      'data-video-src',
      'data-file',
      'data-url',
      'data-mp4',
      'data-hls',
      'data-dash',
    ],
  });

  // A playlist the page loads later, as a player that waits for play does, shows up as it
  // loads; those loaded before the script ran come at once (buffered).
  try {
    loads = new PerformanceObserver((list) => {
      if (noteLoaded(list.getEntries())) {
        clearTimeout(refreshTimer);
        refreshTimer = setTimeout(collectSources, 200);
      }
    });
    loads.observe({ type: 'resource', buffered: true });
  } catch {
    // No resource timing here: the page's own elements and scripts are still read.
  }

  // Optional manual refresh from console:
  window.refreshVideoDownloadSources = collectSources;
})();

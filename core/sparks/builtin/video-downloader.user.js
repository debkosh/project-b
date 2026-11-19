// ==UserScript==
// @name         Video downloader
// @description  Finds the videos on a page so you can save one
// @version      1.0
// @namespace    https://github.com/sudiptadeb/project-b
// @match        https://*/*
// @run-at       document-end
// ==/UserScript==

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
  const URL_LIKE_RE = /https?:\\?\/\\?\/[^\s"'<>]+/gi;

  const state = {
    sources: [],
    open: false,
    menu: null, // { trigger } while a menu is open
    shownKey: null, // what the list shows now
    pending: null, // rows to show once the open menu closes
    downloading: new Set(),
  };

  let refreshTimer = null;
  let observer = null;

  // ---- Finding videos (as the original) ----

  function normalizeUrl(raw) {
    if (!raw || typeof raw !== 'string') return '';

    let url = raw
      .trim()
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

  function guessFilename(url) {
    try {
      const parsed = new URL(url, location.href);
      const name = parsed.pathname.split('/').filter(Boolean).pop();
      return name && name.includes('.') ? name : 'video';
    } catch {
      return 'video';
    }
  }

  function addSource(map, rawUrl, label, priority, requireVideoLike = true) {
    const url = normalizeUrl(rawUrl);

    if (!url) return;
    if (requireVideoLike && !looksVideoLike(url)) return;

    const key = url;

    if (!map.has(key)) {
      map.set(key, {
        url,
        labels: new Set([label]),
        priority,
      });
    } else {
      const existing = map.get(key);
      existing.labels.add(label);
      existing.priority = Math.max(existing.priority, priority);
    }
  }

  function collectFromVideo(map, video, label, priority) {
    if (!video || video.closest?.(`#${ROOT_ID}`)) return;

    addSource(map, video.currentSrc, `${label}: currentSrc`, priority + 5, false);
    addSource(map, video.src, `${label}: src`, priority + 4, false);

    video.querySelectorAll('source[src]').forEach((source) => {
      addSource(map, source.src || source.getAttribute('src'), `${label}: <source>`, priority + 3, false);
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
      addSource(map, video.getAttribute(attr), `${label}: ${attr}`, priority + 1, true);
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
      addSource(map, a.href || a.getAttribute('href'), 'page link', 55, true);
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

    state.sources = [...map.values()]
      .map((item) => ({
        url: item.url,
        labels: [...item.labels],
        priority: item.priority,
      }))
      .sort((a, b) => b.priority - a.priority);

    updateUI();
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
    return sources.map((source, index) => {
      const stream = isStream(source.url);
      const type = stream ? 'Stream' : typeName(source.url);
      const quality = qualityOf(source.url, hints);
      const name = type || `Video ${index + 1}`;
      return {
        url: source.url,
        label: quality ? `${name} · ${quality}` : name,
        host: hostOf(source.url),
        stream,
      };
    });
  }

  // ---- The widget ----

  const ICONS = {
    download: '<path d="M12 4v11"/><path d="m7 10 5 5 5-5"/><path d="M5 20h14"/>',
    more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
    close: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>',
    open: '<path d="M14 4h6v6"/><path d="M20 4 10 14"/><path d="M19 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h5"/>',
    refresh: '<path d="M20 12a8 8 0 1 1-2.3-5.7L20 8.5"/><path d="M20 4v4.5h-4.5"/>',
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
      .menu button:hover { background: var(--hover); }
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
    .empty { padding: 16px; }
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
    .menu button {
      width: 100%;
      height: 32px;
      padding: 0 10px;
      border: 0;
      border-radius: 6px;
      background: transparent;
      display: flex;
      align-items: center;
      gap: 10px;
      text-align: left;
      white-space: nowrap;
      cursor: pointer;
    }
    .menu button svg { color: var(--muted); }
    .menu button:focus-visible { background: var(--hover); outline: none; }

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
      .btn { height: 44px; padding: 0 16px; }
      .row { min-height: 56px; padding: 4px 4px 4px 16px; gap: 0; }
      .row + .row { border-top: 0; }
      .text:has(.note) .sub { display: none; }
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
      .row .btn:active, .icon-btn:active { background: var(--hover); }
      .row .btn .btn-text { display: none; }
      .menu button { height: 44px; padding: 0 12px; }
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
          <h2 class="title">Videos on this page</h2>
          <button class="icon-btn panel-more" type="button" aria-label="More actions"
            aria-haspopup="menu" aria-expanded="false">${icon('more')}</button>
          <button class="icon-btn panel-close" type="button" aria-label="Close">${icon('close')}</button>
        </div>
        <ul class="list"></ul>
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

  function renderRow(row) {
    const li = el('li', 'row');
    const text = el('div', 'text');
    const sub = el('div', 'sub', row.host);
    text.append(el('div', 'label', row.label), sub);
    li.append(text);

    if (row.stream) {
      text.append(el('div', 'note', "Streams can't be saved as one file"));
    } else if (state.downloading.has(row.url)) {
      text.append(el('div', 'note', 'Downloading: see Downloads'));
    } else {
      // Another site's file: the browser ignores `download` for it and opens it in a tab,
      // where the person can save it.
      let elsewhere = false;
      try {
        elsewhere = new URL(row.url).origin !== location.origin;
      } catch {}
      if (elsewhere) text.append(el('div', 'note', 'Opens in a new tab, where you can save it'));
      const link = el('a', 'btn quiet');
      link.href = row.url;
      link.download = guessFilename(row.url);
      // The browser may ignore `download` for another site's file; then it opens in a tab.
      link.target = '_blank';
      link.rel = 'noopener';
      link.setAttribute('aria-label', `Download ${row.label}`);
      link.innerHTML = icon('download');
      // A phone shows the icon alone, its name the link's label.
      link.append(el('span', 'btn-text', 'Download'));
      link.addEventListener('click', () => {
        if (elsewhere) return;
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
      toggleMenu(more, [
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
    title.textContent = rows.length === 1 ? '1 video on this page' : rows.length ? `${rows.length} videos on this page` : 'Videos on this page';
    const key = JSON.stringify(rows.map((row) => [row.url, row.label]));
    if (key === state.shownKey) return;
    state.shownKey = key;
    // Don't pull a row out from under an open menu: show the new list when it closes.
    if (state.menu) {
      state.pending = rows;
    } else {
      renderList(rows);
    }
  }

  function firstControl() {
    return list.querySelector('a, button') || shadow.querySelector('.panel-close');
  }

  function openPanel() {
    state.open = true;
    panel.hidden = false;
    fab.setAttribute('aria-expanded', 'true');
    firstControl().focus();
  }

  function closePanel(returnFocus) {
    if (!state.open) return;
    closeMenu(false);
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
      const button = el('button');
      button.type = 'button';
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
    menu.querySelector('button').focus();
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
  shadow.querySelector('.panel-close').addEventListener('click', () => closePanel(true));
  panelMore.addEventListener('click', () => {
    toggleMenu(panelMore, [
      { icon: 'refresh', label: 'Look again', run: () => collectSources() },
      { icon: 'hide', label: 'Hide on this page', run: () => destroyWidget() },
    ]);
  });
  list.addEventListener('scroll', () => closeMenu(false));

  // A tap anywhere else in the widget closes an open menu.
  shadow.addEventListener('click', (event) => {
    if (!state.menu) return;
    const path = event.composedPath();
    if (!path.includes(menu) && !path.includes(state.menu.trigger)) closeMenu(false);
  });

  menu.addEventListener('keydown', (event) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    const items = [...menu.querySelectorAll('button')];
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

  // Escape closes the innermost: the menu, then the panel.
  function onKeydown(event) {
    if (event.key !== 'Escape' || (!state.menu && !state.open)) return;
    event.preventDefault();
    event.stopPropagation();
    if (state.menu) closeMenu(true);
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
    clearTimeout(refreshTimer);
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

  // Optional manual refresh from console:
  window.refreshVideoDownloadSources = collectSources;
})();

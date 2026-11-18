// ==UserScript==
// @name         dhd
// @description  
// @match        https://*/*
// @run-at       document-end
// ==/UserScript==

(function () {
  'use strict';

  const ROOT_ID = 'custom-video-downloader-widget';
  const STYLE_ID = 'custom-video-downloader-style';

  // Re-running the script refreshes the widget instead of duplicating it.
  document.getElementById(ROOT_ID)?.remove();
  document.getElementById(STYLE_ID)?.remove();

  const VIDEO_EXT_RE = /\.(mp4|m4v|webm|ogv|ogg|mov|m3u8|mpd)(?:[?#]|$)/i;
  const URL_LIKE_RE = /https?:\\?\/\\?\/[^\s"'<>]+/gi;

  const state = {
    sources: [],
    selectedIndex: 0,
    popupOpen: false,
  };

  let refreshTimer = null;
  let observer = null;

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

    // Ignore blob: and data: URLs — not downloadable outside the page session.
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

    if (state.selectedIndex >= state.sources.length) {
      state.selectedIndex = 0;
    }

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

  function destroyWidget() {
    observer?.disconnect();
    clearTimeout(refreshTimer);
    delete window.refreshVideoDownloadSources;
    document.getElementById(STYLE_ID)?.remove();
    document.getElementById(ROOT_ID)?.remove();
  }

  function injectStyles() {
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = `
      #${ROOT_ID} {
        position: fixed;
        right: 10px;
        bottom: 10px;
        z-index: 999999;
        font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        color: #111;
      }

      #${ROOT_ID} * {
        box-sizing: border-box;
      }

      #${ROOT_ID} .vd-main-row {
        display: flex;
        align-items: stretch;
        justify-content: flex-end;
        gap: 2px;
      }

      #${ROOT_ID} .vd-download,
      #${ROOT_ID} .vd-toggle,
      #${ROOT_ID} .vd-kill {
        border: 0;
        background: #e53e3e;
        color: #fff;
        font-size: 14px;
        font-weight: 700;
        cursor: pointer;
        box-shadow: 0 2px 8px rgba(0,0,0,.3);
      }

      #${ROOT_ID} .vd-download {
        display: inline-flex;
        align-items: center;
        min-height: 38px;
        padding: 8px 14px;
        border-radius: 6px 0 0 6px;
        text-decoration: none;
      }

      #${ROOT_ID} .vd-toggle {
        width: 38px;
        border-radius: 0;
      }

      #${ROOT_ID} .vd-kill {
        width: 38px;
        border-radius: 0 6px 6px 0;
        background: #822;
      }

      #${ROOT_ID} .vd-download:hover,
      #${ROOT_ID} .vd-toggle:hover {
        background: #c53030;
      }

      #${ROOT_ID} .vd-kill:hover {
        background: #611;
      }

      #${ROOT_ID} .vd-popup {
        display: none;
        position: absolute;
        right: 0;
        bottom: 48px;
        width: min(620px, calc(100vw - 20px));
        max-height: min(520px, 70vh);
        overflow: auto;
        background: #fff;
        color: #111;
        border-radius: 10px;
        border: 1px solid rgba(0,0,0,.12);
        box-shadow: 0 8px 28px rgba(0,0,0,.35);
      }

      #${ROOT_ID}.open .vd-popup {
        display: block;
      }

      #${ROOT_ID} .vd-header {
        position: sticky;
        top: 0;
        background: #fff;
        z-index: 1;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
        padding: 10px 12px;
        border-bottom: 1px solid rgba(0,0,0,.1);
      }

      #${ROOT_ID} .vd-title {
        font-size: 14px;
        font-weight: 800;
      }

      #${ROOT_ID} .vd-actions {
        display: flex;
        gap: 6px;
      }

      #${ROOT_ID} .vd-small-btn {
        border: 1px solid rgba(0,0,0,.18);
        background: #f7f7f7;
        color: #111;
        border-radius: 5px;
        padding: 4px 8px;
        font-size: 12px;
        cursor: pointer;
      }

      #${ROOT_ID} .vd-small-btn:hover {
        background: #eee;
      }

      #${ROOT_ID} .vd-empty {
        padding: 14px 12px;
        font-size: 13px;
        color: #555;
      }

      #${ROOT_ID} .vd-list {
        padding: 8px;
      }

      #${ROOT_ID} .vd-item {
        border: 1px solid rgba(0,0,0,.1);
        border-radius: 8px;
        padding: 8px;
        margin-bottom: 8px;
        background: #fafafa;
      }

      #${ROOT_ID} .vd-item.primary {
        border-color: #e53e3e;
        background: #fff5f5;
      }

      #${ROOT_ID} .vd-meta {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
        margin-bottom: 6px;
        font-size: 12px;
        color: #555;
      }

      #${ROOT_ID} .vd-badge {
        display: inline-block;
        border-radius: 999px;
        padding: 2px 7px;
        font-size: 11px;
        font-weight: 700;
        color: #fff;
        background: #e53e3e;
      }

      #${ROOT_ID} .vd-url {
        width: 100%;
        border: 1px solid rgba(0,0,0,.16);
        border-radius: 5px;
        padding: 6px;
        font-size: 12px;
        background: #fff;
        color: #111;
      }

      #${ROOT_ID} .vd-row-actions {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-top: 7px;
      }
    `;
    document.head.appendChild(style);
  }

  function createUI() {
    injectStyles();

    const root = document.createElement('div');
    root.id = ROOT_ID;

    root.innerHTML = `
      <div class="vd-main-row">
        <a class="vd-download" href="#" target="_blank" rel="noopener">⬇ Download</a>
        <button class="vd-toggle" type="button" title="Show video sources">▾</button>
        <button class="vd-kill" type="button" title="Remove widget">✕</button>
      </div>

      <div class="vd-popup">
        <div class="vd-header">
          <div class="vd-title">Video sources</div>
          <div class="vd-actions">
            <button class="vd-small-btn vd-refresh" type="button">Refresh</button>
            <button class="vd-small-btn vd-close" type="button">Close</button>
          </div>
        </div>
        <div class="vd-list"></div>
      </div>
    `;

    document.body.appendChild(root);

    root.querySelector('.vd-toggle').addEventListener('click', (event) => {
      event.preventDefault();
      event.stopPropagation();
      state.popupOpen = !state.popupOpen;
      updateUI();
    });

    root.querySelector('.vd-kill').addEventListener('click', (event) => {
      event.preventDefault();
      event.stopPropagation();
      destroyWidget();
    });

    root.querySelector('.vd-refresh').addEventListener('click', (event) => {
      event.preventDefault();
      collectSources();
    });

    root.querySelector('.vd-close').addEventListener('click', (event) => {
      event.preventDefault();
      state.popupOpen = false;
      updateUI();
    });

    root.querySelector('.vd-download').addEventListener('click', (event) => {
      const selected = state.sources[state.selectedIndex];

      if (!selected) {
        event.preventDefault();
        state.popupOpen = true;
        updateUI();
        return;
      }

      // Browser may ignore `download` for cross-origin URLs.
      event.currentTarget.href = selected.url;
      event.currentTarget.download = guessFilename(selected.url);
    });

    document.addEventListener('click', (event) => {
      if (!document.getElementById(ROOT_ID)) return;
      if (!root.contains(event.target)) {
        state.popupOpen = false;
        updateUI();
      }
    });

    return root;
  }

  function updateUI() {
    const root = document.getElementById(ROOT_ID);
    if (!root) return;

    root.classList.toggle('open', state.popupOpen);

    const selected = state.sources[state.selectedIndex];
    const download = root.querySelector('.vd-download');
    const list = root.querySelector('.vd-list');

    if (selected) {
      download.href = selected.url;
      download.download = guessFilename(selected.url);
      download.textContent = `⬇ Download`;
      download.title = selected.url;
    } else {
      download.href = '#';
      download.removeAttribute('download');
      download.textContent = '⬇ No video found';
      download.title = 'No video source found yet';
    }

    if (!state.sources.length) {
      list.innerHTML = `
        <div class="vd-empty">
          No video sources found yet. Try playing the video, then click Refresh.
        </div>
      `;
      return;
    }

    list.innerHTML = '';

    state.sources.forEach((source, index) => {
      const item = document.createElement('div');
      item.className = `vd-item ${index === state.selectedIndex ? 'primary' : ''}`;

      const labelText = source.labels.slice(0, 3).join(', ');
      const moreLabels = source.labels.length > 3 ? ` +${source.labels.length - 3} more` : '';

      item.innerHTML = `
        <div class="vd-meta">
          <span>${index + 1}. ${labelText}${moreLabels}</span>
          ${index === state.selectedIndex ? '<span class="vd-badge">primary</span>' : ''}
        </div>

        <input class="vd-url" readonly value="">

        <div class="vd-row-actions">
          <button class="vd-small-btn vd-use" type="button">Use as main</button>
          <button class="vd-small-btn vd-open" type="button">Open</button>
          <button class="vd-small-btn vd-copy" type="button">Copy</button>
        </div>
      `;

      item.querySelector('.vd-url').value = source.url;

      item.querySelector('.vd-use').addEventListener('click', () => {
        state.selectedIndex = index;
        updateUI();
      });

      item.querySelector('.vd-open').addEventListener('click', () => {
        window.open(source.url, '_blank', 'noopener');
      });

      item.querySelector('.vd-copy').addEventListener('click', (event) => {
        copyText(source.url);
        event.currentTarget.textContent = 'Copied';
        setTimeout(() => {
          event.currentTarget.textContent = 'Copy';
        }, 900);
      });

      list.appendChild(item);
    });
  }

  createUI();
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

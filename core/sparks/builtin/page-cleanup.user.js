// ==UserScript==
// @name         Page cleanup
// @description  Hide any part of a page you don't want to see
// @version      1.0
// @namespace    https://github.com/sudiptadeb/project-b
// @match        <all_urls>
// @run-at       document-end
// ==/UserScript==

// A button docked to the edge of every page opens a small menu. "Hide parts of this page"
// outlines what is under the pointer or finger, and a tap hides it at once, with Undo.
// What is hidden stays hidden on that site: its selectors are kept in the site's own
// localStorage under page_cleaner_hidden_<host>, and the button's place under
// page_cleaner_fab_position. "Show hidden parts" brings them all back.

(function () {
    'use strict';

    const STORAGE_KEY = 'page_cleaner_hidden_' + location.hostname;
    const FAB_POS_KEY = 'page_cleaner_fab_position';
    const HIDDEN_ATTR = 'data-pc-hidden';
    const DISALLOWED_TAGS = ['HTML', 'BODY', 'MAIN'];
    const MAX_SELECTOR_LENGTH = 200;
    // The screen time limit's own parts are never hidden, and taps on them pass through.
    const OTHER_SPARK_UI = '#screentime-blocker, #screentime-puzzle, #screentime-sheet, #screentime-backdrop, #screentime-warning';
    const HOST_ID = 'pb-page-cleanup';
    const TAP_SLOP = 10;
    const CANT_HIDE = "This part can't be hidden";

    /* ---------------------------------------------------------------- storage */

    function getHiddenSelectors() {
        try {
            const raw = localStorage.getItem(STORAGE_KEY);
            const list = raw ? JSON.parse(raw) : [];
            return Array.isArray(list) ? list : [];
        } catch {
            return [];
        }
    }

    function setHiddenSelectors(list) {
        try {
            if (list.length) {
                localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
            } else {
                localStorage.removeItem(STORAGE_KEY);
            }
        } catch { }
    }

    /* ------------------------------------------------------ hiding page parts */

    const savedDisplay = new WeakMap();

    function hideEl(el) {
        if (el.hasAttribute(HIDDEN_ATTR)) {
            return;
        }
        savedDisplay.set(el, [el.style.getPropertyValue('display'), el.style.getPropertyPriority('display')]);
        el.style.setProperty('display', 'none', 'important');
        el.setAttribute(HIDDEN_ATTR, '1');
    }

    function showEl(el) {
        const prev = savedDisplay.get(el);
        el.style.removeProperty('display');
        if (prev && prev[0]) {
            el.style.setProperty('display', prev[0], prev[1]);
        }
        savedDisplay.delete(el);
        el.removeAttribute(HIDDEN_ATTR);
    }

    function matches(el, selector) {
        try {
            return el.matches(selector);
        } catch {
            return false;
        }
    }

    function applyHidden() {
        for (const selector of getHiddenSelectors()) {
            let found;
            try {
                found = document.querySelectorAll(selector + ':not([' + HIDDEN_ATTR + '])');
            } catch {
                continue;
            }
            found.forEach(hideEl);
        }
    }

    function unhideSelector(selector) {
        const rest = getHiddenSelectors().filter(s => s !== selector);
        setHiddenSelectors(rest);
        document.querySelectorAll('[' + HIDDEN_ATTR + ']').forEach(el => {
            if (matches(el, selector) && !rest.some(s => matches(el, s))) {
                showEl(el);
            }
        });
    }

    function showAll() {
        setHiddenSelectors([]);
        document.querySelectorAll('[' + HIDDEN_ATTR + ']').forEach(showEl);
    }

    function isDisallowed(el) {
        if (!el || !(el instanceof Element)) {
            return true;
        }
        if (DISALLOWED_TAGS.includes(el.tagName)) {
            return true;
        }
        if (el === document.body || el === document.documentElement || el === host) {
            return true;
        }
        return !!el.closest(OTHER_SPARK_UI);
    }

    // The owner's way of naming an element so it can be found again on the next visit.
    function buildElementSelector(el) {
        // 1. ID selector (most reliable)
        if (el.id && !el.id.startsWith('pc-')) {
            return '#' + CSS.escape(el.id);
        }

        // 2. aria-label (usually unique)
        const aria = el.getAttribute('aria-label');
        if (aria) {
            const escapedAria = aria.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
            return el.tagName.toLowerCase() + '[aria-label="' + escapedAria + '"]';
        }

        // 3. role attribute
        const role = el.getAttribute('role');
        if (role) {
            const roleSelector = el.tagName.toLowerCase() + '[role="' + role + '"]';
            try {
                if (document.querySelectorAll(roleSelector).length <= 3) {
                    return roleSelector;
                }
            } catch { }
        }

        // 4. data-* attribute
        const dataAttr = [...el.attributes].find(a => a.name.startsWith('data-') && !a.name.startsWith('data-_pc'));
        if (dataAttr) {
            const escapedValue = dataAttr.value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
            const dataSelector = el.tagName.toLowerCase() + '[' + dataAttr.name + '="' + escapedValue + '"]';
            try {
                if (document.querySelectorAll(dataSelector).length <= 3) {
                    return dataSelector;
                }
            } catch { }
        }

        // 5. Classes, if unique enough
        const classes = [...el.classList].filter(c => c.length < 30 && !c.startsWith('pc-'));
        if (classes.length) {
            const sel = el.tagName.toLowerCase() + '.' + classes.map(c => CSS.escape(c)).join('.');
            try {
                if (sel.length <= MAX_SELECTOR_LENGTH && document.querySelectorAll(sel).length <= 3) {
                    return sel;
                }
            } catch { }
        }

        // 6. A path up to the nearest ancestor with an ID
        const path = [];
        let current = el;
        let foundAnchorId = false;

        while (current && current !== document.body && path.length < 5) {
            let segment = current.tagName.toLowerCase();

            if (current.id && !current.id.startsWith('pc-')) {
                segment = '#' + CSS.escape(current.id);
                path.unshift(segment);
                foundAnchorId = true;
                break;
            }

            const currentClasses = [...current.classList].filter(c => c.length < 20 && !c.startsWith('pc-'));
            if (currentClasses.length > 0) {
                segment += '.' + currentClasses.slice(0, 2).map(c => CSS.escape(c)).join('.');
            }

            if (current.parentElement) {
                const siblings = [...current.parentElement.children].filter(s => s.tagName === current.tagName);
                if (siblings.length > 1) {
                    segment += ':nth-of-type(' + (siblings.indexOf(current) + 1) + ')';
                }
            }

            path.unshift(segment);
            current = current.parentElement;
        }

        if (foundAnchorId && path.length > 1) {
            const selector = path.join(' > ');
            if (selector.length <= MAX_SELECTOR_LENGTH) {
                return selector;
            }
        }

        // 7. The same path, from the body
        if (path.length > 0) {
            const selector = path.join(' > ');
            if (selector.length <= MAX_SELECTOR_LENGTH) {
                return selector;
            }
        }

        // 8. A child of a parent named by its ID or classes
        const parent = el.parentElement;
        if (parent && parent !== document.body) {
            const childIndex = [...parent.children].indexOf(el) + 1;
            let parentSel = null;
            if (parent.id && !parent.id.startsWith('pc-')) {
                parentSel = '#' + CSS.escape(parent.id);
            } else {
                const parentClasses = [...parent.classList].filter(c => c.length < 20 && !c.startsWith('pc-'));
                if (parentClasses.length > 0) {
                    parentSel = parent.tagName.toLowerCase() + '.' + parentClasses.slice(0, 2).map(c => CSS.escape(c)).join('.');
                }
            }
            if (parentSel) {
                const selector = parentSel + ' > :nth-child(' + childIndex + ')';
                try {
                    if (document.querySelectorAll(selector).length <= 3) {
                        return selector;
                    }
                } catch { }
            }
        }

        return null;
    }

    /* --------------------------------------------------------------------- UI */

    const ICON_EYE_OFF = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10.7 5.1A10.7 10.7 0 0 1 21.9 11.7a1 1 0 0 1 0 .7 10.7 10.7 0 0 1-1.4 2.5"/><path d="M14.1 14.2a3 3 0 0 1-4.2-4.2"/><path d="M17.5 17.5A10.7 10.7 0 0 1 2.1 12.3a1 1 0 0 1 0-.7 10.7 10.7 0 0 1 4.4-5.1"/><path d="m2 2 20 20"/></svg>';
    const ICON_EYE = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2.1 12.3a1 1 0 0 1 0-.7 10.7 10.7 0 0 1 19.8 0 1 1 0 0 1 0 .7 10.7 10.7 0 0 1-19.8 0"/><circle cx="12" cy="12" r="3"/></svg>';

    const STYLE = `
        :host {
            --surface: #ffffff; --text: #1f1f1f; --muted: #5c6270; --line: rgba(15, 20, 25, 0.1);
            --accent: #23272f; --on-accent: #ffffff; --danger: #d93025;
            --hover: rgba(0, 0, 0, .05); --scrim: rgba(0, 0, 0, .32);
            --shadow: 0 8px 28px rgba(16, 24, 40, 0.14), 0 1px 3px rgba(16, 24, 40, 0.08);
            font: 400 14px/1.4 -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif;
            color: var(--text);
            letter-spacing: normal; word-spacing: normal; text-transform: none;
            text-align: left; text-indent: 0; white-space: normal; font-style: normal;
        }
        @media (prefers-color-scheme: dark) {
            :host {
                --surface: #2a2c30; --text: #e8eaed; --muted: #9aa0a6; --line: rgba(255, 255, 255, 0.12);
                --accent: #e7eaf2; --on-accent: #1f1f1f; --danger: #f28b82;
                --hover: rgba(255, 255, 255, .06); --scrim: rgba(0, 0, 0, .5);
                --shadow: 0 8px 24px rgba(0, 0, 0, .4);
            }
        }
        *, *::before, *::after { box-sizing: border-box; }
        button {
            font: inherit; color: inherit; background: none; border: 0; margin: 0; padding: 0;
            cursor: pointer; -webkit-tap-highlight-color: transparent;
        }
        button:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
        svg {
            width: 16px; height: 16px; flex: none; fill: none; stroke: currentColor;
            stroke-width: 2; stroke-linecap: round; stroke-linejoin: round;
        }

        .fab {
            position: fixed; right: 0; width: 40px; height: 40px;
            display: flex; align-items: center; justify-content: center;
            background: var(--surface); color: var(--text);
            border: 1px solid var(--line); border-right: 0; border-radius: 8px 0 0 8px;
            box-shadow: var(--shadow); touch-action: none;
        }
        .fab.left {
            left: 0; right: auto; border-right: 1px solid var(--line); border-left: 0;
            border-radius: 0 8px 8px 0;
        }
        .fab:hover { color: var(--accent); }

        .scrim { position: fixed; inset: 0; background: var(--scrim); }

        .panel {
            position: fixed; width: 248px; padding: 6px;
            background: var(--surface); color: var(--text);
            border: 1px solid var(--line); border-radius: 12px; box-shadow: var(--shadow);
        }
        .title { font-size: 12px; color: var(--muted); padding: 6px 10px 4px; }
        .row {
            display: flex; align-items: center; gap: 10px; width: 100%; min-height: 36px;
            padding: 0 10px; border-radius: 8px; text-align: left;
        }
        .row:hover { background: var(--hover); }

        .outline {
            position: fixed; pointer-events: none; border: 2px solid var(--accent);
            border-radius: 2px; box-shadow: 0 0 0 1px var(--surface);
        }

        .done {
            position: fixed; right: 16px; bottom: calc(16px + env(safe-area-inset-bottom, 0px));
            height: 40px; min-width: 80px; padding: 0 18px; border-radius: 8px;
            background: var(--accent); color: var(--on-accent); font-weight: 600;
            box-shadow: var(--shadow);
        }

        .toast {
            position: fixed; left: 50%; transform: translateX(-50%);
            bottom: calc(16px + env(safe-area-inset-bottom, 0px));
            max-width: calc(100vw - 32px); min-height: 40px; padding: 0 16px;
            display: flex; align-items: center; gap: 8px;
            background: var(--accent); color: var(--on-accent);
            border-radius: 12px; box-shadow: var(--shadow);
        }
        .toast.has-undo { padding-right: 6px; }
        .undo {
            min-height: 32px; padding: 0 10px; border-radius: 8px;
            font-weight: 600; text-decoration: underline; text-underline-offset: 2px;
        }
        .undo:focus-visible { outline-color: var(--on-accent); outline-offset: -2px; }

        @media (pointer: coarse), (max-width: 599px) {
            .fab { width: 44px; height: 44px; }
            .panel {
                left: 0; right: 0; bottom: 0; width: auto;
                padding: 8px 8px calc(8px + env(safe-area-inset-bottom, 0px));
                border-bottom: 0; border-radius: 12px 12px 0 0;
                transform: translateY(100%); transition: transform 180ms ease-out;
            }
            .panel.in { transform: none; }
            .title { padding: 8px 12px 6px; }
            .row { min-height: 48px; padding: 0 12px; gap: 12px; }
            .done { height: 44px; min-width: 88px; }
            .toast { bottom: calc(72px + env(safe-area-inset-bottom, 0px)); min-height: 44px; }
            .undo { min-height: 44px; }
        }
        @media (prefers-reduced-motion: reduce) {
            .panel { transition: none; }
        }

        [hidden] { display: none; }
    `;

    // A run again replaces the last one.
    const old = document.getElementById(HOST_ID);
    if (old) {
        if (typeof old.pcDispose === 'function') {
            old.pcDispose();
        }
        old.remove();
    }

    const host = document.createElement('pb-page-cleanup');
    host.id = HOST_ID;
    host.style.cssText = 'position:fixed;top:0;left:0;width:0;height:0;margin:0;padding:0;border:0;display:block;z-index:2147483647;';
    const root = host.attachShadow({ mode: 'open' });
    root.innerHTML = `
        <style>${STYLE}</style>
        <div class="outline" hidden></div>
        <button class="fab" type="button" aria-label="Page cleanup" aria-haspopup="dialog" aria-expanded="false">${ICON_EYE_OFF}</button>
        <div class="scrim" hidden></div>
        <div class="panel" role="dialog" aria-label="Page cleanup" hidden>
            <div class="title" aria-hidden="true">Page cleanup</div>
            <button class="row" type="button" data-act="hide">${ICON_EYE_OFF}<span>Hide parts of this page</span></button>
            <button class="row" type="button" data-act="show">${ICON_EYE}<span class="show-label"></span></button>
        </div>
        <button class="done" type="button" hidden>Done</button>
        <div role="status" aria-live="polite">
            <div class="toast" hidden><span class="toast-text"></span><button class="undo" type="button" hidden>Undo</button></div>
        </div>
    `;

    const $ = s => root.querySelector(s);
    const fab = $('.fab');
    const scrim = $('.scrim');
    const panel = $('.panel');
    const hideRow = $('[data-act="hide"]');
    const showRow = $('[data-act="show"]');
    const showLabel = $('.show-label');
    const done = $('.done');
    const outline = $('.outline');
    const toast = $('.toast');
    const toastText = $('.toast-text');
    const undoBtn = $('.undo');
    const sheetQuery = window.matchMedia('(pointer: coarse), (max-width: 599px)');

    const ours = e => e.composedPath().includes(host);

    /* --------------------------------------------------------- the button */

    let fabPos = { edge: 'right', y: 100 };
    try {
        const saved = JSON.parse(localStorage.getItem(FAB_POS_KEY) || 'null');
        if (saved && typeof saved === 'object') {
            fabPos = { ...fabPos, ...saved };
        }
    } catch { }

    function placeFab() {
        // On a phone it stays above the page's own bottom bar.
        const min = sheetQuery.matches ? 80 : 16;
        const max = Math.max(min, window.innerHeight - (fab.offsetHeight || 44) - 16);
        const y = Math.max(min, Math.min(max, Number(fabPos.y) || 100));
        fab.classList.toggle('left', fabPos.edge === 'left');
        fab.style.bottom = y + 'px';
    }

    let drag = null;
    let dragged = false;

    fab.addEventListener('pointerdown', e => {
        if (e.button !== 0) {
            return;
        }
        drag = { id: e.pointerId, x: e.clientX, y: e.clientY, bottom: parseFloat(fab.style.bottom) || 100, moved: false };
        fab.setPointerCapture(e.pointerId);
    });
    fab.addEventListener('pointermove', e => {
        if (!drag || e.pointerId !== drag.id) {
            return;
        }
        const dx = e.clientX - drag.x;
        const dy = e.clientY - drag.y;
        if (!drag.moved && Math.abs(dx) < TAP_SLOP && Math.abs(dy) < TAP_SLOP) {
            return;
        }
        if (!drag.moved) {
            drag.moved = true;
            closePanel(false);
        }
        fabPos.y = drag.bottom - dy;
        fabPos.edge = e.clientX < window.innerWidth / 2 ? 'left' : 'right';
        placeFab();
    });
    fab.addEventListener('pointerup', e => {
        if (!drag || e.pointerId !== drag.id) {
            return;
        }
        if (drag.moved) {
            dragged = true;
            fabPos.y = parseFloat(fab.style.bottom);
            try {
                localStorage.setItem(FAB_POS_KEY, JSON.stringify(fabPos));
            } catch { }
        }
        drag = null;
    });
    fab.addEventListener('pointercancel', () => {
        drag = null;
    });
    fab.addEventListener('click', () => {
        if (dragged) {
            dragged = false;
            return;
        }
        if (panelOpen) {
            closePanel(true);
        } else {
            openPanel();
        }
    });

    /* --------------------------------------------------------- the menu */

    let panelOpen = false;

    function openPanel() {
        const count = getHiddenSelectors().length;
        showRow.hidden = count === 0;
        showLabel.textContent = 'Show hidden parts (' + count + ')';
        panelOpen = true;
        fab.setAttribute('aria-expanded', 'true');
        panel.classList.remove('in');
        panel.style.left = panel.style.right = panel.style.bottom = '';
        panel.hidden = false;
        if (sheetQuery.matches) {
            scrim.hidden = false;
            void panel.offsetHeight;
        } else {
            const r = fab.getBoundingClientRect();
            const gap = fab.offsetWidth + 8 + 'px';
            const bottom = Math.min(window.innerHeight - r.bottom, window.innerHeight - panel.offsetHeight - 8);
            panel.style.bottom = Math.max(8, bottom) + 'px';
            if (fabPos.edge === 'left') {
                panel.style.left = gap;
            } else {
                panel.style.right = gap;
            }
        }
        panel.classList.add('in');
        hideRow.focus();
    }

    function closePanel(restoreFocus) {
        if (!panelOpen) {
            return;
        }
        panelOpen = false;
        panel.hidden = true;
        scrim.hidden = true;
        panel.classList.remove('in');
        fab.setAttribute('aria-expanded', 'false');
        if (restoreFocus) {
            fab.focus();
        }
    }

    scrim.addEventListener('click', () => closePanel(true));
    hideRow.addEventListener('click', startHiding);
    showRow.addEventListener('click', () => {
        showAll();
        closePanel(true);
    });

    /* --------------------------------------------------------- toast */

    let toastTimer = 0;
    let undoAction = null;

    function showToast(text, undo) {
        clearTimeout(toastTimer);
        toastText.textContent = text;
        undoAction = undo || null;
        undoBtn.hidden = !undo;
        toast.classList.toggle('has-undo', !!undo);
        toast.hidden = false;
        toastTimer = setTimeout(hideToast, undo ? 5000 : 3000);
    }

    function hideToast() {
        clearTimeout(toastTimer);
        toast.hidden = true;
        undoAction = null;
    }

    undoBtn.addEventListener('click', () => {
        const action = undoAction;
        hideToast();
        if (action) {
            action();
        }
    });

    /* --------------------------------------------------------- hiding mode */

    let hiding = false;
    let outlined = null;
    let touch = null;

    function drawOutline() {
        if (!outlined || !outlined.isConnected) {
            outline.hidden = true;
            return;
        }
        const r = outlined.getBoundingClientRect();
        const vw = document.documentElement.clientWidth;
        const vh = document.documentElement.clientHeight;
        const left = Math.max(0, r.left);
        const top = Math.max(0, r.top);
        const width = Math.min(vw, r.right) - left;
        const height = Math.min(vh, r.bottom) - top;
        if (width <= 0 || height <= 0) {
            outline.hidden = true;
            return;
        }
        outline.style.left = left + 'px';
        outline.style.top = top + 'px';
        outline.style.width = width + 'px';
        outline.style.height = height + 'px';
        outline.hidden = false;
    }

    function setOutline(el) {
        outlined = el && !isDisallowed(el) ? el : null;
        drawOutline();
    }

    function tryHide(el) {
        const selector = isDisallowed(el) ? null : buildElementSelector(el);
        if (!selector) {
            showToast(CANT_HIDE);
            return;
        }
        const list = getHiddenSelectors();
        if (!list.includes(selector)) {
            list.push(selector);
            setHiddenSelectors(list);
        }
        hideEl(el);
        applyHidden();
        setOutline(null);
        showToast('Hidden.', () => {
            unhideSelector(selector);
            if (el.hasAttribute(HIDDEN_ATTR)) {
                showEl(el);
            }
        });
    }

    function passes(e) {
        return ours(e) || (e.target instanceof Element && e.target.closest(OTHER_SPARK_UI));
    }

    // In hiding mode nothing the person does on the page reaches the page.
    function guard(e) {
        if (passes(e)) {
            return;
        }
        e.stopImmediatePropagation();
        // Not on touchstart or pointerdown, so the page still scrolls under a finger.
        if (e.cancelable && e.type !== 'touchstart' && e.type !== 'pointerdown' && e.type !== 'touchend') {
            e.preventDefault();
        }
        if (e.type === 'touchstart' && e.touches.length === 1) {
            const t = e.touches[0];
            touch = { x: t.clientX, y: t.clientY, moved: false };
            setOutline(document.elementFromPoint(t.clientX, t.clientY));
        } else if (e.type === 'click') {
            const wasMoved = touch && touch.moved;
            touch = null;
            if (!wasMoved) {
                tryHide(e.target);
            }
        }
    }

    function onTouchMove(e) {
        if (!touch || passes(e)) {
            return;
        }
        const t = e.touches[0];
        if (Math.abs(t.clientX - touch.x) > TAP_SLOP || Math.abs(t.clientY - touch.y) > TAP_SLOP) {
            touch.moved = true;
            setOutline(null);
        }
    }

    function onPointerMove(e) {
        if (e.pointerType !== 'mouse' && e.pointerType !== 'pen') {
            return;
        }
        setOutline(ours(e) ? null : e.target);
    }

    const GUARDED = ['pointerdown', 'pointerup', 'mousedown', 'mouseup', 'touchstart', 'touchend', 'click', 'auxclick', 'dblclick', 'contextmenu'];

    function startHiding() {
        if (hiding) {
            return;
        }
        closePanel(false);
        hiding = true;
        GUARDED.forEach(type => window.addEventListener(type, guard, { capture: true, passive: false }));
        window.addEventListener('touchmove', onTouchMove, { capture: true, passive: true });
        window.addEventListener('pointermove', onPointerMove, true);
        fab.hidden = true;
        done.hidden = false;
        done.focus();
    }

    function stopHiding() {
        if (!hiding) {
            return;
        }
        hiding = false;
        GUARDED.forEach(type => window.removeEventListener(type, guard, { capture: true }));
        window.removeEventListener('touchmove', onTouchMove, { capture: true });
        window.removeEventListener('pointermove', onPointerMove, true);
        touch = null;
        setOutline(null);
        done.hidden = true;
        fab.hidden = false;
        fab.focus();
    }

    done.addEventListener('click', stopHiding);

    /* --------------------------------------------------------- page events */

    function onKeyDown(e) {
        if (e.key !== 'Escape') {
            return;
        }
        if (panelOpen) {
            closePanel(true);
        } else if (hiding) {
            stopHiding();
        } else {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
    }

    function onOutsidePointer(e) {
        if (panelOpen && !ours(e)) {
            closePanel(false);
        }
    }

    function onResize() {
        placeFab();
        drawOutline();
    }

    window.addEventListener('keydown', onKeyDown, true);
    window.addEventListener('pointerdown', onOutsidePointer, true);
    window.addEventListener('resize', onResize);
    window.addEventListener('scroll', drawOutline, { capture: true, passive: true });

    // Parts the page adds later are hidden as they come.
    let applyPending = false;
    const observer = new MutationObserver(() => {
        if (applyPending) {
            return;
        }
        applyPending = true;
        requestAnimationFrame(() => {
            applyPending = false;
            applyHidden();
        });
    });

    host.pcDispose = () => {
        stopHiding();
        hideToast();
        observer.disconnect();
        window.removeEventListener('keydown', onKeyDown, true);
        window.removeEventListener('pointerdown', onOutsidePointer, true);
        window.removeEventListener('resize', onResize);
        window.removeEventListener('scroll', drawOutline, { capture: true });
    };

    applyHidden();
    observer.observe(document.body, { childList: true, subtree: true });
    document.documentElement.appendChild(host);
    placeFab();
})();

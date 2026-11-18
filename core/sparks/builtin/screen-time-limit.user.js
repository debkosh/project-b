// ==UserScript==
// @name        Screen time limit
// @description Stops each site after 30 minutes a day, until you ask for more
// @version     1.0
// @namespace   https://github.com/sudiptadeb/project-b
// @match       <all_urls>
// @run-at      document-end
// ==/UserScript==

// Counts the seconds spent on each site, a day at a time, while its tab is in view. At the
// limit it covers the page; "More time…" asks a sum, then "How much more?". Each time more is
// asked for, the sum gets harder and the choices get shorter; at night they stop at
// NIGHT_MAX_SNOOZE minutes. The last 5 minutes show a countdown at the top right.
//
// Kept in the site's own localStorage:
//   screentime_seconds_<host>       seconds spent today
//   screentime_date_<host>          the day those seconds are for (a new day starts at 0)
//   screentime_snooze_<host>        when the more time asked for ends (ms since 1970)
//   screentime_snooze_count_<host>  how many times more time was asked for today

'use strict';

const CONFIG = {
    TIME_LIMIT_MINUTES: 30,   // each site's limit a day
    NIGHT_START_HOUR: 22,     // night starts (24-hour clock)
    NIGHT_END_HOUR: 6,        // night ends
    NIGHT_MAX_SNOOZE: 30,     // the most more time at night, in minutes
    PUZZLE_ENABLED: true,     // a sum before more time
};

const HOST = location.hostname;
const LIMIT = CONFIG.TIME_LIMIT_MINUTES * 60;
const WARNING_SECONDS = 5 * 60;

const KEYS = {
    TIME: `screentime_seconds_${HOST}`,
    DATE: `screentime_date_${HOST}`,
    SNOOZE: `screentime_snooze_${HOST}`,
    SNOOZE_COUNT: `screentime_snooze_count_${HOST}`,
};

/* ---------------------------------------------------------------- the day's time */

const getToday = () => new Date().toDateString();
const isSnoozed = () => Date.now() < (Number(localStorage.getItem(KEYS.SNOOZE)) || 0);
const isNightTime = () => {
    const hour = new Date().getHours();
    return hour >= CONFIG.NIGHT_START_HOUR || hour < CONFIG.NIGHT_END_HOUR;
};

function getSnoozeCount() {
    if (localStorage.getItem(KEYS.DATE) !== getToday()) {
        return 0;
    }
    return Number(localStorage.getItem(KEYS.SNOOZE_COUNT)) || 0;
}

function loadTime() {
    if (localStorage.getItem(KEYS.DATE) !== getToday()) {
        localStorage.setItem(KEYS.DATE, getToday());
        localStorage.setItem(KEYS.TIME, '0');
        localStorage.removeItem(KEYS.SNOOZE);
        localStorage.setItem(KEYS.SNOOZE_COUNT, '0');
        return 0;
    }
    return Number(localStorage.getItem(KEYS.TIME)) || 0;
}

function snooze(mins) {
    const until = mins === 'all'
        ? new Date().setHours(23, 59, 59, 999)
        : Date.now() + mins * 60000;
    localStorage.setItem(KEYS.SNOOZE, until);
    localStorage.setItem(KEYS.SNOOZE_COUNT, getSnoozeCount() + 1);
    removeBlocker();
    startTimer();
}

// The more time on offer: shorter each time it is asked for, and at most
// NIGHT_MAX_SNOOZE minutes at night.
function getSnoozeOptions() {
    const count = getSnoozeCount();
    const night = isNightTime();
    const maxCap = night ? CONFIG.NIGHT_MAX_SNOOZE : Infinity;
    const allOptions = [5, 15, 30, 60].filter(v => v <= maxCap);

    let primaryMax, extendedValue;
    if (count >= 3) { primaryMax = 5; extendedValue = 15; }
    else if (count >= 2) { primaryMax = 15; extendedValue = 30; }
    else if (count >= 1) { primaryMax = 30; extendedValue = 60; }
    else { primaryMax = 60; extendedValue = null; }

    const primary = allOptions.filter(v => v <= primaryMax);
    const extended = [];
    if (extendedValue && allOptions.includes(extendedValue)) {
        extended.push(extendedValue);
    }
    if (count === 0 && !night) {
        extended.push('all');
    }
    return { primary, extended };
}

// The sum gets harder each time more time is asked for.
function generatePuzzle() {
    const count = getSnoozeCount();
    const n = (from, span) => Math.floor(Math.random() * span) + from;
    if (count >= 2) {
        const a = n(2, 9), b = n(10, 11);
        return { question: `${a} × ${b}`, answer: a * b };
    }
    if (count === 1) {
        const a = n(2, 9), b = n(2, 9);
        return { question: `${a} × ${b}`, answer: a * b };
    }
    const a = n(5, 20), b = n(5, 20);
    return { question: `${a} + ${b}`, answer: a + b };
}

/* ---------------------------------------------------------------- words */

function minutesText(mins) {
    const unit = (v, one) => `${v} ${one}${v === 1 ? '' : 's'}`;
    if (mins < 60) {
        return unit(mins, 'minute');
    }
    const h = Math.floor(mins / 60), m = mins % 60;
    return unit(h, 'hour') + (m ? ' ' + unit(m, 'minute') : '');
}

function hourText(h) {
    return `${h % 12 || 12} ${h < 12 ? 'AM' : 'PM'}`;
}

// One message for each case: the night, the first time, and again.
function getMessage() {
    const count = getSnoozeCount();
    if (isNightTime()) {
        return {
            title: "It's late",
            body: [`More time is capped at ${minutesText(CONFIG.NIGHT_MAX_SNOOZE)} until ${hourText(CONFIG.NIGHT_END_HOUR)}.`],
        };
    }
    if (count === 0) {
        return {
            title: 'Daily limit reached',
            body: [`You've spent ${minutesText(Math.floor(seconds / 60))} on `, el('b', {}, HOST), ' today.'],
        };
    }
    return {
        title: 'Limit reached again',
        body: [`You've added more time ${count === 1 ? 'once' : count + ' times'} today.`],
    };
}

/* ---------------------------------------------------------------- the view */

const CSS = `
:host {
    --surface: #ffffff; --text: #1f1f1f; --muted: #5c6270; --line: rgba(15, 20, 25, 0.1);
    --accent: #23272f; --on-accent: #ffffff; --danger: #d93025;
    --shadow: 0 8px 28px rgba(16, 24, 40, 0.14), 0 1px 3px rgba(16, 24, 40, 0.08); --scrim: rgba(0,0,0,.32);
    display: block; position: static; z-index: 2147483647;
    font: 14px/1.4 -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Arial, sans-serif; color: var(--text);
    text-align: left; letter-spacing: normal; text-transform: none;
}
@media (prefers-color-scheme: dark) {
    :host {
        --surface: #2a2c30; --text: #e8eaed; --muted: #9aa0a6; --line: rgba(255, 255, 255, 0.12);
        --accent: #e7eaf2; --on-accent: #1f1f1f; --danger: #f28b82;
        --shadow: 0 8px 24px rgba(0,0,0,.48); --scrim: rgba(0,0,0,.56);
    }
}
:host([data-state="block"]) { position: fixed; inset: 0; }
* { box-sizing: border-box; }
h1, h2, p { margin: 0; }
button, input { font: inherit; color: inherit; margin: 0; }
svg { flex: none; }

.cover {
    position: fixed; inset: 0; display: flex; flex-direction: column;
    align-items: center; justify-content: center; gap: 24px;
    padding: max(24px, env(safe-area-inset-top)) 16px max(24px, env(safe-area-inset-bottom));
    background: var(--surface); text-align: center;
    overflow: hidden; overscroll-behavior: contain; touch-action: none;
}
.msg { display: flex; flex-direction: column; align-items: center; gap: 8px; max-width: 360px; }
.badge {
    display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 8px;
    border: 1px solid var(--line); border-radius: 50%; color: var(--muted);
}
h1 { font-size: 20px; font-weight: 600; line-height: 1.3; }
.muted { color: var(--muted); }
.msg b { color: var(--text); font-weight: 600; }

.btn {
    display: inline-flex; align-items: center; justify-content: center; gap: 8px;
    min-height: 36px; padding: 0 16px; border: 1px solid var(--line); border-radius: 8px;
    background: var(--surface); color: var(--text); font-weight: 500; cursor: pointer;
}
.btn.primary { border-color: var(--accent); background: var(--accent); color: var(--on-accent); }
.btn.plain { border-color: transparent; background: transparent; }
@media (hover: hover) {
    .btn:hover, .btn.plain:hover { background: color-mix(in srgb, var(--text) 6%, var(--surface)); }
    .btn.primary:hover { background: color-mix(in srgb, var(--accent) 86%, var(--surface)); }
}
.btn:focus-visible, input:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }

.cover:focus, .panel:focus { outline: none; }
.scrim { position: fixed; inset: 0; background: var(--scrim); touch-action: none; }
.panel {
    position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%);
    display: flex; flex-direction: column; gap: 8px;
    width: min(320px, calc(100vw - 32px)); max-height: calc(100vh - 32px);
    overflow: auto; overscroll-behavior: contain; padding: 20px;
    border-radius: 12px; background: var(--surface); box-shadow: var(--shadow);
}
.panel h2 { margin-bottom: 4px; font-size: 16px; font-weight: 600; }
.panel .btn { width: 100%; }
.sum { padding: 8px 0; font-size: 28px; font-weight: 600; text-align: center; font-variant-numeric: tabular-nums; }
input {
    width: 100%; min-height: 36px; padding: 0 12px; border: 1px solid var(--line); border-radius: 8px;
    background: var(--surface); font-size: 16px; text-align: center;
}
input[aria-invalid="true"] { border-color: var(--danger); outline-color: var(--danger); }
.err { color: var(--danger); font-size: 12px; text-align: center; }
.err[hidden], .extra[hidden] { display: none; }
form, .opts, .extra { display: flex; flex-direction: column; gap: 8px; }

.pill {
    position: fixed; top: 12px; right: 12px; z-index: 2147483647;
    display: flex; align-items: center; height: 24px; padding: 0 10px;
    border: 1px solid var(--line); border-radius: 12px; background: var(--surface);
    box-shadow: var(--shadow); font-size: 12px; font-weight: 600; font-variant-numeric: tabular-nums;
}
.pill.soon { color: var(--danger); }
.tip {
    position: absolute; top: calc(100% + 6px); right: 0; display: none; padding: 6px 8px;
    border-radius: 8px; background: var(--accent); color: var(--on-accent);
    box-shadow: var(--shadow); font-size: 12px; font-weight: 400; white-space: nowrap;
}
.pill.tip-on .tip, .pill:hover .tip { display: block; }

@media (pointer: coarse), (max-width: 599px) {
    .cover { justify-content: flex-end; }
    .msg { flex: 1; justify-content: center; }
    .cover > .btn { width: 100%; max-width: 480px; }
    .btn, input { min-height: 44px; }
    .panel {
        top: auto; right: 0; bottom: 0; left: 0; transform: none; width: auto; max-height: 85vh;
        padding: 20px 16px max(16px, env(safe-area-inset-bottom));
        border-radius: 12px 12px 0 0; animation: up 180ms ease-out;
    }
}
@keyframes up { from { transform: translateY(100%); } to { transform: none; } }
@media (prefers-reduced-motion: reduce) { .panel { animation: none; } }
`;

const CLOCK = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" '
    + 'stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'
    + '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>';

function el(tag, props, ...children) {
    const node = document.createElement(tag);
    for (const [k, v] of Object.entries(props)) {
        if (k === 'html') node.innerHTML = v;
        else if (k.startsWith('on')) node.addEventListener(k.slice(2), v);
        else node.setAttribute(k, v);
    }
    node.append(...children.flat(Infinity));
    return node;
}

document.getElementById('screentime-blocker')?.remove();
const host = document.createElement('screentime-limit');
host.id = 'screentime-blocker';
const root = host.attachShadow({ mode: 'open' });
root.append(el('style', {}, CSS));

let pill = null, cover = null, moreBtn = null, panel = null, scrim = null;

function mount() {
    if (!host.isConnected) document.documentElement.appendChild(host);
}

function unmountIfEmpty() {
    if (!pill && !cover) host.remove();
}

const stop = e => e.preventDefault();

/* The countdown in the last minutes before the page is covered. */

function showWarningTimer(remaining) {
    if (cover) return;
    if (!pill) {
        pill = el('div', { class: 'pill tip-on', role: 'timer', 'aria-label': "This site's limit ends soon" },
            el('span', { class: 'time' }),
            el('span', { class: 'tip', role: 'tooltip' }, "This site's limit ends soon"));
        root.append(pill);
        mount();
        const tipOff = () => pill?.classList.remove('tip-on');
        document.addEventListener('scroll', tipOff, { once: true, passive: true });
        document.addEventListener('click', tipOff, { once: true });
    }
    const m = Math.floor(remaining / 60), s = remaining % 60;
    pill.firstChild.textContent = `${m}:${String(s).padStart(2, '0')} left`;
    pill.classList.toggle('soon', remaining <= 10);
}

function hideWarningTimer() {
    pill?.remove();
    pill = null;
    unmountIfEmpty();
}

/* The cover, and the panels over it. */

function createBlocker() {
    if (cover) {
        mount();
        return;
    }
    hideWarningTimer();
    const message = getMessage();
    document.body.style.overflow = 'hidden';
    document.querySelectorAll('video').forEach(v => v.pause());

    moreBtn = el('button', { type: 'button', class: 'btn', onclick: () => CONFIG.PUZZLE_ENABLED ? showPuzzle() : showSheet() },
        'More time…');
    cover = el('div', { class: 'cover', role: 'dialog', 'aria-modal': 'true', 'aria-label': message.title, tabindex: '-1' },
        el('div', { class: 'msg' },
            el('span', { class: 'badge', html: CLOCK }),
            el('h1', {}, message.title),
            el('p', { class: 'muted' }, message.body)),
        moreBtn);
    cover.addEventListener('touchmove', stop, { passive: false });
    cover.addEventListener('wheel', stop, { passive: false });
    root.append(cover);
    host.dataset.state = 'block';
    mount();
    cover.focus();
}

function removeBlocker() {
    closePanel();
    cover?.remove();
    cover = moreBtn = null;
    delete host.dataset.state;
    unmountIfEmpty();
    document.body.style.overflow = '';
}

function openPanel(label, ...children) {
    closePanel();
    scrim = el('div', { class: 'scrim', onclick: () => closePanel(true) });
    scrim.addEventListener('touchmove', stop, { passive: false });
    scrim.addEventListener('wheel', stop, { passive: false });
    panel = el('div', { class: 'panel', role: 'dialog', 'aria-modal': 'true', 'aria-label': label, tabindex: '-1' },
        el('h2', {}, label), children);
    root.append(scrim, panel);
    return panel;
}

function closePanel(refocus) {
    scrim?.remove();
    panel?.remove();
    scrim = panel = null;
    if (refocus) moreBtn?.focus();
}

function showPuzzle() {
    const puzzle = generatePuzzle();
    const input = el('input', { type: 'text', inputmode: 'numeric', autocomplete: 'off', 'aria-label': 'Answer' });
    const err = el('p', { class: 'err', role: 'alert', hidden: '' }, 'Not quite. Try again');
    input.addEventListener('input', () => {
        err.hidden = true;
        input.removeAttribute('aria-invalid');
    });
    const form = el('form', {
        onsubmit: e => {
            e.preventDefault();
            if (parseInt(input.value, 10) === puzzle.answer) {
                showSheet();
                return;
            }
            input.value = '';
            input.setAttribute('aria-invalid', 'true');
            err.hidden = false;
            input.focus();
        },
    }, input, err, el('button', { type: 'submit', class: 'btn primary' }, 'Check'));
    openPanel('Solve this for more time', el('p', { class: 'sum' }, `${puzzle.question} = ?`), form);
    input.focus();
}

function showSheet() {
    const { primary, extended } = getSnoozeOptions();
    const option = v => el('button', { type: 'button', class: 'btn', onclick: () => snooze(v) },
        v === 'all' ? 'Rest of today' : minutesText(v));
    const parts = [el('div', { class: 'opts' }, primary.map(option))];
    if (extended.length) {
        const extra = el('div', { class: 'extra', hidden: '' }, extended.map(option));
        const more = el('button', {
            type: 'button', class: 'btn plain',
            onclick: () => {
                more.remove();
                extra.hidden = false;
                extra.querySelector('button').focus();
            },
        }, 'More options');
        parts.push(more, extra);
    }
    openPanel('How much more?', parts).focus();
}

root.addEventListener('keydown', e => {
    if (e.key === 'Escape' && panel) {
        e.preventDefault();
        e.stopPropagation();
        closePanel(true);
    }
});

// Focus stays on the cover while the page is covered.
document.addEventListener('focusin', e => {
    if (cover && e.target !== host) {
        (panel || cover).focus();
    }
});

/* ---------------------------------------------------------------- the timer */

let seconds = loadTime();
let timer = null;

const isLimitReached = () => seconds >= LIMIT;

function startTimer() {
    stopTimer();
    timer = setInterval(() => {
        const snoozeUntil = Number(localStorage.getItem(KEYS.SNOOZE)) || 0;
        const snoozed = Date.now() < snoozeUntil;
        const remaining = snoozed ? Math.ceil((snoozeUntil - Date.now()) / 1000) : LIMIT - seconds;

        if (remaining > 0 && remaining <= WARNING_SECONDS) {
            showWarningTimer(remaining);
        } else {
            hideWarningTimer();
        }
        if (snoozed) {
            return;
        }

        seconds++;
        localStorage.setItem(KEYS.TIME, seconds);
        localStorage.setItem(KEYS.DATE, getToday());
        if (isLimitReached()) {
            stopTimer();
            hideWarningTimer();
            createBlocker();
        }
    }, 1000);
}

function stopTimer() {
    clearInterval(timer);
    timer = null;
}

// The cover comes back if the page takes it away.
let mutationPending = false;
new MutationObserver(() => {
    if (mutationPending) return;
    mutationPending = true;
    requestAnimationFrame(() => {
        mutationPending = false;
        if (!isSnoozed() && isLimitReached() && !(cover && host.isConnected)) {
            createBlocker();
        }
    });
}).observe(document.documentElement, { childList: true, subtree: true });

document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        stopTimer();
    } else if (!isSnoozed() && !isLimitReached()) {
        startTimer();
    }
});

if (!isSnoozed() && isLimitReached()) {
    createBlocker();
} else {
    startTimer();
}

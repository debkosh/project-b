// ==UserScript==
// @name         Limit Screen Time
// @description  Set daily website time limit, 30 min by default.
// @match        <all_urls>
// @run-at       document-end
// ==/UserScript==

/**
 * ─────────────────────────────────────────────────────────────────────────────
 * HOW IT WORKS
 * ─────────────────────────────────────────────────────────────────────────────
 *  1. Timer tracks seconds spent on each domain (per-site, resets daily)
 *  2. Timer pauses when tab is hidden, resumes when visible
 *  3. When TIME_LIMIT_MINUTES is reached, a blocker overlay appears
 *  4. User taps "Ignore limit" → puzzle gate (if enabled)
 *  5. On success, a bottom sheet shows snooze options
 *  6. Snooze options decrease with repeated use to encourage disengagement
 *
 * ─────────────────────────────────────────────────────────────────────────────
 * FEATURES
 * ─────────────────────────────────────────────────────────────────────────────
 *
 *  MATH PUZZLE GATE (optional, toggle via PUZZLE_ENABLED)
 *    - Adds friction before granting more time
 *    - Progressive difficulty:
 *        • 1st snooze: Addition (5-24 + 5-24)
 *        • 2nd snooze: Multiplication (2-10 × 2-10)
 *        • 3rd+ snooze: Harder multiplication (2-10 × 10-20)
 *    - Wrong answers shake the input with red highlight
 *
 *  PROGRESSIVE LIMITS
 *    - 1st snooze: 5, 15, 30, 60 min visible | "Ignore limit for today" in More Options
 *    - 2nd snooze: 5, 15, 30 min visible | 60 min in More Options
 *    - 3rd snooze: 5, 15 min visible | 30 min in More Options
 *    - 4th+ snooze: 5 min visible | 15 min in More Options
 *
 *  NIGHT-TIME STRICTNESS (10 PM - 6 AM)
 *    - Maximum snooze capped at 30 minutes
 *    - No extended options (More Options hidden)
 *    - Special messaging encouraging sleep
 *
 *  CONTEXT-AWARE MESSAGING
 *    - Different titles/subtitles based on:
 *      • First block vs. repeat blocks
 *      • Day vs. night time
 *      • Number of snoozes used
 *
 *  UI/UX
 *    - Light/Dark mode (follows system preference)
 *    - Responsive layout (portrait/landscape)
 *    - Smooth animations (fade, scale, shake, pulse, bounce)
 *    - Bottom sheet with drag-to-dismiss
 *    - "More options" reveals extended snooze durations
 *    - Scroll blocking prevents background interaction
 *
 * ─────────────────────────────────────────────────────────────────────────────
 * CONFIGURATION
 * ─────────────────────────────────────────────────────────────────────────────
 *  Adjust the CONFIG object below:
 *    TIME_LIMIT_MINUTES  - Daily limit per site (default: 30 minutes)
 *    NIGHT_START_HOUR    - Night mode start (default: 22 / 10 PM)
 *    NIGHT_END_HOUR      - Night mode end (default: 6 / 6 AM)
 *    NIGHT_MAX_SNOOZE    - Max snooze at night (default: 30 minutes)
 *    PUZZLE_ENABLED      - Require puzzle before snooze (default: true)
 *
 * ─────────────────────────────────────────────────────────────────────────────
 * STORAGE
 * ─────────────────────────────────────────────────────────────────────────────
 *  Uses localStorage with per-domain keys (prefixed with "screentime_"):
 *    screentime_seconds_{host}      - Seconds spent today
 *    screentime_date_{host}         - Current date (for daily reset)
 *    screentime_snooze_{host}       - Snooze expiry timestamp
 *    screentime_snooze_count_{host} - Number of snoozes used today
 *
 * ═══════════════════════════════════════════════════════════════════════════
 */

// ignorei18n_start

(function () {
    'use strict';

    /*=========================================================================
     * CONFIGURATION
     *=========================================================================*/
    const CONFIG = {
        TIME_LIMIT_MINUTES: 30,          // Daily time limit per site (in minutes)
        NIGHT_START_HOUR: 22,            // Night mode starts (24h format, 22 = 10 PM)
        NIGHT_END_HOUR: 6,               // Night mode ends (6 / 6 AM)
        NIGHT_MAX_SNOOZE: 30,            // Max snooze at night (minutes)
        PUZZLE_ENABLED: true,            // Require solving a puzzle before snooze
    };

    const HOST = location.hostname;

    const Z_INDEX = {
        BACKDROP: 2147483646,
        TOP: 2147483647
    };

    /*=========================================================================
     * THEME
     *=========================================================================*/
    const isDarkMode = () => window.matchMedia('(prefers-color-scheme: dark)').matches;

    const THEME = {
        dark: { bg: '#000', text: '#fff', subtext: '#aaa', sheet: '#1c1c1e', btn: '#2c2c2e', primary: '#0a84ff', backdrop: 'rgba(0,0,0,.6)' },
        light: { bg: '#f2f2f7', text: '#000', subtext: '#555', sheet: '#fff', btn: '#f2f2f7', primary: '#007aff', backdrop: 'rgba(0,0,0,.35)' }
    };

    const getTheme = () => isDarkMode() ? THEME.dark : THEME.light;
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    /*=========================================================================
     * LOCAL STORAGE KEYS
     *=========================================================================*/
    const KEYS = {
        TIME: `screentime_seconds_${HOST}`,
        DATE: `screentime_date_${HOST}`,
        SNOOZE: `screentime_snooze_${HOST}`,
        SNOOZE_COUNT: `screentime_snooze_count_${HOST}`
    };

    /*=========================================================================
     * HELPER FUNCTIONS
     *=========================================================================*/
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

    /*=========================================================================
     * PROGRESSIVE LIMITS
     *=========================================================================*/
    function getSnoozeOptions() {
        const count = getSnoozeCount();
        const night = isNightTime();
        const maxCap = night ? CONFIG.NIGHT_MAX_SNOOZE : Infinity;

        const allOptions = [
            [5, 'Remind me in 5 minutes'],
            [15, 'Remind me in 15 minutes'],
            [30, 'Remind me in 30 minutes'],
            [60, 'Remind me in an hour']
        ].filter(([v]) => v <= maxCap);

        let primaryMax, extendedValue;
        if (count >= 3) { primaryMax = 5; extendedValue = 15; }
        else if (count >= 2) { primaryMax = 15; extendedValue = 30; }
        else if (count >= 1) { primaryMax = 30; extendedValue = 60; }
        else { primaryMax = 60; extendedValue = null; }

        const primary = allOptions.filter(([v]) => v <= primaryMax);
        const extended = [];

        if (extendedValue && extendedValue <= maxCap) {
            const opt = allOptions.find(([v]) => v === extendedValue);
            if (opt) {
                extended.push(opt);
            }
        }

        if (count === 0 && !night) {
            extended.push(['all', 'Ignore limit for today']);
        }

        return { primary, extended };
    }

    /*=========================================================================
     * CONTEXT-AWARE MESSAGING
     *=========================================================================*/
    function getMessage() {
        const count = getSnoozeCount();
        const night = isNightTime();

        if (night) {
            return count === 0
                ? { title: "Time to wind down.", sub: "This is a good moment to call it a night." }
                : { title: "Still up?", sub: "Staying longer means sleeping less." };
        }

        if (count === 0) {
            return { title: "Daily Limit Reached", sub: `You've reached your limit on ${HOST}` };
        } else if (count === 1) {
            return { title: "Back already?", sub: "Looks like you've spent a while here. Time to step away." };
        } else if (count === 2) {
            return { title: "Again so soon?", sub: "This site is pulling you back. Consider taking a break." };
        } else {
            return { title: "Time to step back a little", sub: "You've extended your time limit multiple times. Consider ending this session." };
        }
    }

    /*=========================================================================
     * CSS STYLES (deferred)
     *=========================================================================*/
    let stylesInjected = false;
    function injectStyles() {
        if (stylesInjected) {
            return;
        }
        stylesInjected = true;

        const styles = document.createElement('style');
        styles.id = 'screentime-styles';
        styles.textContent = `
            /* Reset all inherited styles */
            #screentime-blocker,
            #screentime-blocker *,
            #screentime-puzzle,
            #screentime-puzzle *,
            #screentime-sheet,
            #screentime-sheet *,
            #screentime-backdrop {
                all: revert;
                box-sizing: border-box;
            }

            /* Blocker overlay */
            #screentime-blocker {
                position: fixed !important; inset: 0 !important;
                z-index: ${Z_INDEX.TOP} !important;
                display: flex !important; flex-direction: column; align-items: center; justify-content: center;
                visibility: visible !important;
                font-family: -apple-system, system-ui, sans-serif;
                text-align: center;
                overflow: hidden; touch-action: none; overscroll-behavior: contain;
                padding: env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
            }

            /* Blocker content */
            #screentime-blocker .screentime-content {
                display: flex; flex-direction: column; align-items: center; justify-content: center;
                padding: 20px; max-width: 320px; width: 100%;
            }
            #screentime-blocker .screentime-icon { font-size: 3.5rem; margin-bottom: 16px; line-height: 1 }
            #screentime-blocker h1 { margin: 0 0 8px; font-size: 1.625rem; font-weight: 600; line-height: 1.2 }
            #screentime-blocker p { margin: 0 0 20px; font-size: 1rem; line-height: 1.4 }
            #screentime-blocker .screentime-link {
                font-size: 1.0625rem; background: none; border: none; padding: 10px;
                font-weight: 500; cursor: pointer;
                -webkit-tap-highlight-color: transparent; outline: none;
            }

            /* Puzzle overlay */
            #screentime-puzzle {
                position: fixed !important; inset: 0 !important;
                z-index: ${Z_INDEX.TOP} !important;
                display: flex !important; align-items: center; justify-content: center;
                visibility: visible !important;
                font-family: -apple-system, system-ui, sans-serif;
                overflow: hidden; touch-action: none; overscroll-behavior: contain;
            }
            #screentime-puzzle .screentime-card { padding: 24px 32px; border-radius: 16px; text-align: center; line-height: 1.4 }
            #screentime-puzzle .screentime-label { margin: 0 0 4px; font-size: 0.8125rem; line-height: 1.4 }
            #screentime-puzzle .screentime-question { margin: 0 0 16px; font-size: 2rem; font-weight: 600; line-height: 1.2 }
            #screentime-puzzle .screentime-submit {
                display: block; width: 100%; margin-top: 12px; padding: 14px;
                font-size: 1.0625rem; border: none; border-radius: 12px;
                cursor: pointer; font-weight: 500; line-height: 1.2;
            }

            /* Backdrop */
            #screentime-backdrop {
                position: fixed !important; inset: 0 !important;
                width: 100vw !important; height: 100vh !important;
                width: 100dvw !important; height: 100dvh !important;
                z-index: ${Z_INDEX.BACKDROP} !important;
                display: block !important; visibility: visible !important;
            }

            /* Bottom sheet */
            #screentime-sheet {
                position: fixed !important; left: 0 !important; right: 0 !important; bottom: 0 !important;
                border-radius: 16px 16px 0 0;
                padding: 12px 20px calc(30px + env(safe-area-inset-bottom));
                z-index: ${Z_INDEX.TOP} !important;
                display: block !important; visibility: visible !important;
                font-family: -apple-system, system-ui, sans-serif;
                touch-action: none;
            }
            #screentime-sheet .screentime-handle { width: 36px; height: 5px; background: #888; border-radius: 3px; margin: 0 auto 12px }
            #screentime-sheet h3 { text-align: center; margin: 0 0 12px; font-size: 1.125rem; font-weight: 600 }
            #screentime-sheet .screentime-grid { display: grid; gap: 10px }
            #screentime-sheet .screentime-btn { padding: 14px; font-size: 1.0625rem; border-radius: 12px; border: none; cursor: pointer }
            #screentime-sheet .screentime-more {
                display: block; width: 100%; margin-top: 10px; padding: 12px; font-size: 0.9375rem;
                background: none; border: none; cursor: pointer;
                -webkit-tap-highlight-color: transparent; outline: none;
            }
            #screentime-sheet .screentime-more-options { display: none; margin-top: 10px }
            #screentime-sheet .screentime-more-options .screentime-btn { display: block; width: 100%; margin-bottom: 10px }
            #screentime-sheet .screentime-more-options .screentime-btn:last-child { margin-bottom: 0 }

            /* Puzzle input */
            #screentime-puzzle .screentime-puzzle-input {
                width: 100%; padding: 14px; font-size: 1.0625rem; text-align: center;
                border: 2px solid #888; border-radius: 12px; outline: none;
                background: transparent; color: inherit;
                -webkit-appearance: none; -moz-appearance: textfield;
            }
            #screentime-puzzle .screentime-puzzle-input:focus { border-color: var(--screentime-primary, #007aff) }
            #screentime-puzzle .screentime-puzzle-input.error { border-color: #ff3b30; animation: screentime-shake 0.5s ease-out }

            /* Transitions */
            .screentime-overlay { transition: opacity .3s ease }
            .screentime-content { transition: opacity .3s ease, transform .3s ease }
            .screentime-card { transition: opacity .3s ease, transform .3s ease }
            .screentime-sheet { transition: transform .35s cubic-bezier(.32,.72,0,1) }
            .screentime-btn { transition: transform .1s ease, opacity .2s ease, background .2s ease }
            .screentime-btn:active:not(:disabled) { transform: scale(0.95) }
            .screentime-btn:disabled { opacity: 0.5; cursor: not-allowed }

            /* Animations */
            @keyframes screentime-shake {
                0%, 100% { transform: translateX(0) }
                20% { transform: translateX(-8px) }
                40% { transform: translateX(8px) }
                60% { transform: translateX(-4px) }
                80% { transform: translateX(4px) }
            }
            .screentime-shake { animation: screentime-shake 0.5s ease-out }

            @keyframes screentime-pulse {
                0%, 100% { transform: scale(1) }
                50% { transform: scale(1.1) }
            }
            .screentime-pulse { animation: screentime-pulse 2s ease-in-out infinite }

            @keyframes screentime-bounce {
                0% { transform: translateY(0) }
                50% { transform: translateY(-8px) }
                100% { transform: translateY(0) }
            }
            .screentime-bounce { animation: screentime-bounce 0.3s ease-out }

            /* Landscape adjustments */
            @media (orientation: landscape) {
                #screentime-blocker .screentime-content {
                    flex-direction: row;
                    align-items: flex-start;
                    justify-content: center;
                    gap: 24px;
                    max-width: 90vw;
                    text-align: left;
                }
                #screentime-blocker .screentime-icon { font-size: 3rem; margin: 0; flex-shrink: 0; line-height: 1.2 }
                #screentime-blocker .screentime-text-group { flex: 0 1 auto }
                #screentime-blocker h1 { font-size: 1.375rem; margin: 0 0 4px }
                #screentime-blocker p { margin: 0 0 8px; font-size: 0.9375rem }
                #screentime-blocker .screentime-link { margin: 0; padding: 8px 0 }
            }

            @media (orientation: landscape) and (max-height: 500px) {
                #screentime-sheet { padding: 10px 20px 20px; max-height: 90vh; overflow-y: auto }
                #screentime-sheet .screentime-grid { display: flex; flex-wrap: wrap; gap: 8px }
                #screentime-sheet .screentime-btn { flex: 1 1 auto; min-width: 100px; padding: 10px 12px; font-size: 0.9375rem }
            }

            /* Warning countdown timer - Liquid Glass design */
            #screentime-warning {
                all: revert;
                position: fixed !important;
                top: 12px !important;
                right: 12px !important;
                z-index: ${Z_INDEX.BACKDROP} !important;
                display: flex !important;
                align-items: center;
                justify-content: center;
                padding: 5px 10px;
                border-radius: 14px;
                font-family: -apple-system, system-ui, sans-serif;
                font-size: 0.6875rem;
                font-weight: 600;
                backdrop-filter: blur(40px) saturate(180%);
                -webkit-backdrop-filter: blur(40px) saturate(180%);
                background-image: linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.02));
                transition: opacity 0.3s ease, transform 0.3s ease;
            }
            #screentime-warning .screentime-warning-time {
                font-variant-numeric: tabular-nums;
                letter-spacing: 0.3px;
            }
            @keyframes screentime-warning-pulse {
                0%, 100% { transform: scale(1); }
                50% { transform: scale(1.05); }
            }
            #screentime-warning.screentime-warning-urgent {
                background: rgba(255, 59, 48, 0.15);
                border-color: rgba(255, 59, 48, 0.3);
                color: #ff3b30;
                animation: screentime-warning-pulse 0.6s ease-in-out infinite;
            }

            /* Tooltip */
            #screentime-warning .screentime-tooltip {
                position: absolute;
                top: calc(100% + 6px);
                right: 0;
                padding: 6px 10px;
                border-radius: 8px;
                font-size: 0.625rem;
                font-weight: 500;
                white-space: nowrap;
                background: rgba(0, 0, 0, 0.75);
                backdrop-filter: blur(8px);
                -webkit-backdrop-filter: blur(8px);
                border: 1px solid rgba(255, 255, 255, 0.08);
                color: #fff;
                opacity: 0;
                transform: translateY(-4px);
                pointer-events: none;
                transition: opacity 0.2s ease, transform 0.2s ease;
            }
            #screentime-warning .screentime-tooltip::after {
                content: '';
                position: absolute;
                top: -4px;
                right: 12px;
                border-left: 5px solid transparent;
                border-right: 5px solid transparent;
                border-bottom: 5px solid rgba(0, 0, 0, 0.75);
            }
            #screentime-warning .screentime-tooltip.screentime-tooltip-visible {
                opacity: 1;
                transform: translateY(0);
            }
        `;
        document.head.appendChild(styles);
    }

    /*=========================================================================
     * BLOCKER UI
     *=========================================================================*/
    function createBlocker() {
        if (document.getElementById('screentime-blocker')) {
            return;
        }

        injectStyles();
        const theme = getTheme();
        const message = getMessage();

        document.body.style.overflow = 'hidden';
        document.querySelectorAll('video').forEach(v => v.pause());

        const blocker = document.createElement('div');
        blocker.id = 'screentime-blocker';
        blocker.className = 'screentime-overlay';
        blocker.style.cssText = `background:${theme.bg};color:${theme.text};opacity:0`;

        blocker.addEventListener('touchmove', e => e.preventDefault(), { passive: false });
        blocker.addEventListener('wheel', e => e.preventDefault(), { passive: false });

        const content = document.createElement('div');
        content.className = 'screentime-content screentime-shake';
        content.style.cssText = 'opacity:0;transform:scale(0.95)';

        const icon = document.createElement('div');
        icon.className = 'screentime-icon screentime-pulse';
        icon.textContent = '⏰';

        const textGroup = document.createElement('div');
        textGroup.className = 'screentime-text-group';

        const h1 = document.createElement('h1');
        h1.textContent = message.title;

        const p = document.createElement('p');
        p.style.color = theme.subtext;
        p.textContent = message.sub;

        const button = document.createElement('button');
        button.id = 'screentime-ask-btn';
        button.className = 'screentime-link';
        button.style.color = theme.primary;
        button.textContent = 'Ignore limit';

        textGroup.appendChild(h1);
        textGroup.appendChild(p);
        textGroup.appendChild(button);

        content.appendChild(icon);
        content.appendChild(textGroup);
        blocker.appendChild(content);

        document.body.appendChild(blocker);

        requestAnimationFrame(() => {
            blocker.style.opacity = '1';
            content.style.cssText += ';opacity:1;transform:scale(1)';
        });

        button.onclick = () => {
            const sheet = document.getElementById('screentime-sheet');
            const nextAction = CONFIG.PUZZLE_ENABLED ? showPuzzle : showSheet;
            if (sheet) {
                removeSheet();
                setTimeout(nextAction, 350);
            } else {
                nextAction();
            }
        };
    }

    /*=========================================================================
     * PUZZLE
     *=========================================================================*/
    function generatePuzzle() {
        const count = getSnoozeCount();
        let a, b, op, answer;

        if (count >= 2) {
            a = Math.floor(Math.random() * 9) + 2;
            b = Math.floor(Math.random() * 11) + 10;
            op = '×';
            answer = a * b;
        } else if (count === 1) {
            a = Math.floor(Math.random() * 9) + 2;
            b = Math.floor(Math.random() * 9) + 2;
            op = '×';
            answer = a * b;
        } else {
            a = Math.floor(Math.random() * 20) + 5;
            b = Math.floor(Math.random() * 20) + 5;
            op = '+';
            answer = a + b;
        }

        return { question: `${a} ${op} ${b}`, answer };
    }

    function showPuzzle() {
        if (document.getElementById('screentime-puzzle')) {
            return;
        }

        const theme = getTheme();
        const puzzle = generatePuzzle();

        // Set CSS variable for input focus color
        document.documentElement.style.setProperty('--screentime-primary', theme.primary);

        const overlay = document.createElement('div');
        overlay.id = 'screentime-puzzle';
        overlay.className = 'screentime-overlay';
        overlay.style.cssText = `background:${theme.backdrop};opacity:0`;

        const card = document.createElement('div');
        card.className = 'screentime-card';
        card.style.cssText = `background:${theme.sheet};color:${theme.text};opacity:0;transform:scale(0.9)`;

        const snoozeNum = getSnoozeCount() + 1;

        const label = document.createElement('p');
        label.className = 'screentime-label';
        label.style.color = theme.subtext;
        label.textContent = `Daily Puzzle ${snoozeNum}. Solve to unlock more time.`;

        const question = document.createElement('p');
        question.className = 'screentime-question';
        question.textContent = `${puzzle.question} = ?`;

        const input = document.createElement('input');
        input.type = 'number';
        input.inputMode = 'numeric';
        input.className = 'screentime-puzzle-input';
        input.autocomplete = 'off';

        const submitBtn = document.createElement('button');
        submitBtn.id = 'screentime-puzzle-submit';
        submitBtn.className = 'screentime-submit';
        submitBtn.style.cssText = `background:${theme.primary};color:#fff`;
        submitBtn.textContent = 'Solve';

        card.appendChild(label);
        card.appendChild(question);
        card.appendChild(input);
        card.appendChild(submitBtn);

        overlay.appendChild(card);
        document.body.appendChild(overlay);

        function submitAnswer() {
            const userAnswer = parseInt(input.value, 10);

            if (userAnswer === puzzle.answer) {
                overlay.style.opacity = '0';
                card.style.transform = 'scale(0.9)';
                setTimeout(() => {
                    overlay.remove();
                    showSheet();
                }, 300);
            } else {
                input.classList.remove('error');
                void input.offsetWidth;
                input.classList.add('error');
                input.value = '';
            }
        }

        requestAnimationFrame(() => {
            overlay.style.opacity = '1';
            card.style.cssText += ';opacity:1;transform:scale(1)';
        });

        input.onkeydown = e => {
            if (e.key === 'Enter') {
                e.preventDefault();
                submitAnswer();
            }
        };

        submitBtn.onclick = submitAnswer;

        overlay.onclick = e => {
            if (e.target === overlay) {
                overlay.style.opacity = '0';
                card.style.transform = 'scale(0.9)';
                input.onkeydown = null;
                submitBtn.onclick = null;
                setTimeout(() => overlay.remove(), 300);
            }
        };
    }

    function removeBlocker() {
        const blocker = document.getElementById('screentime-blocker');
        if (blocker) {
            blocker.style.opacity = '0';
            setTimeout(() => blocker.remove(), 300);
        }
        removeSheet();
        document.body.style.overflow = '';
    }

    /*=========================================================================
     * BOTTOM SHEET
     *=========================================================================*/
    function showSheet() {
        if (document.getElementById('screentime-sheet')) {
            return;
        }

        const theme = getTheme();
        const { primary, extended } = getSnoozeOptions();
        const count = getSnoozeCount();
        const night = isNightTime();

        const backdrop = document.createElement('div');
        backdrop.id = 'screentime-backdrop';
        backdrop.className = 'screentime-overlay';
        backdrop.style.cssText = `background:${theme.backdrop};opacity:0`;

        const sheet = document.createElement('div');
        sheet.id = 'screentime-sheet';
        sheet.className = 'screentime-sheet';
        sheet.style.cssText = `background:${theme.sheet};color:${theme.text};transform:translateY(100%)`;

        let sheetTitle = 'A few more minutes?';
        if (night) {
            sheetTitle = 'Just a bit longer?';
        }
        if (count >= 2) {
            sheetTitle = 'Are you sure?';
        }

        const hasExtended = extended.length > 0;

        const handle = document.createElement('div');
        handle.className = 'screentime-handle';

        const h3 = document.createElement('h3');
        h3.textContent = sheetTitle;

        const grid = document.createElement('div');
        grid.className = 'screentime-grid';

        primary.forEach(([value, label]) => {
            const btn = document.createElement('button');
            btn.className = 'screentime-btn';
            btn.dataset.value = value;
            btn.style.cssText = `background:${theme.btn};color:${theme.text}`;
            btn.textContent = label;
            btn.onclick = () => snooze(value);
            grid.appendChild(btn);
        });

        sheet.appendChild(handle);
        sheet.appendChild(h3);
        sheet.appendChild(grid);

        if (hasExtended) {
            const moreBtn = document.createElement('button');
            moreBtn.id = 'screentime-more-btn';
            moreBtn.className = 'screentime-more';
            moreBtn.style.color = theme.primary;
            moreBtn.textContent = 'More options...';

            const moreOptions = document.createElement('div');
            moreOptions.id = 'screentime-more-options';
            moreOptions.className = 'screentime-more-options';

            extended.forEach(([value, label]) => {
                const btn = document.createElement('button');
                btn.className = 'screentime-btn';
                btn.dataset.value = value;
                btn.style.cssText = `background:${theme.btn};color:${theme.text}`;
                btn.textContent = label;
                btn.onclick = () => snooze(value === 'all' ? 'all' : value);
                moreOptions.appendChild(btn);
            });

            moreBtn.onclick = () => {
                moreOptions.style.display = 'block';
                moreBtn.style.display = 'none';
            };

            sheet.appendChild(moreBtn);
            sheet.appendChild(moreOptions);
        }

        let startY, offset = 0;

        sheet.onpointerdown = e => {
            startY = e.clientY;
            sheet.style.transition = 'none';
            sheet.setPointerCapture(e.pointerId);
        };

        sheet.onpointermove = e => {
            if (!sheet.hasPointerCapture(e.pointerId)) {
                return;
            }
            const delta = e.clientY - startY;
            offset = delta < 0 ? delta * 0.2 : delta;
            sheet.style.transform = `translateY(${offset}px)`;
        };

        sheet.onpointerup = e => {
            sheet.releasePointerCapture(e.pointerId);
            sheet.style.transition = '';

            if (offset > 80) {
                removeSheet();
            } else if (offset < -10) {
                sheet.classList.add('screentime-bounce');
                setTimeout(() => sheet.classList.remove('screentime-bounce'), 300);
                sheet.style.transform = 'translateY(0)';
            } else {
                sheet.style.transform = 'translateY(0)';
            }
            offset = 0;
        };

        document.body.append(backdrop, sheet);
        requestAnimationFrame(() => {
            backdrop.style.opacity = '1';
            sheet.style.transform = 'translateY(0)';
        });
    }

    function removeSheet() {
        const sheet = document.getElementById('screentime-sheet');
        const backdrop = document.getElementById('screentime-backdrop');

        if (sheet) {
            sheet.style.transform = 'translateY(100%)';
            setTimeout(() => sheet.remove(), 350);
        }
        if (backdrop) {
            backdrop.style.opacity = '0';
            setTimeout(() => backdrop.remove(), 300);
        }
    }

    /*=========================================================================
     * WARNING COUNTDOWN TIMER (last 2 minutes)
     *=========================================================================*/
    const WARNING_SECONDS = 120;  // Show warning for last 2 minutes

    function showWarningTimer(remainingSeconds) {
        // Don't show if blocked
        if (document.getElementById('screentime-blocker')) {
            return;
        }

        injectStyles();
        const dark = isDarkMode();
        let warning = document.getElementById('screentime-warning');

        // Dynamic colors based on theme
        const bgColor = dark ? 'rgba(60, 60, 67, 0.7)' : 'rgba(255, 255, 255, 0.7)';
        const textColor = dark ? '#fff' : '#333';
        const shadowColor = dark ? 'rgba(0, 0, 0, 0.3)' : 'rgba(0, 0, 0, 0.08)';

        if (!warning) {
            warning = document.createElement('div');
            warning.id = 'screentime-warning';
            warning.style.cssText = `
                opacity: 0;
                transform: translateY(-20px);
                background-color: ${bgColor};
                color: ${textColor};
                border: 1px solid rgba(255, 255, 255, 0.08);
                box-shadow: 0 8px 32px ${shadowColor}, inset 0 1px 0 rgba(255,255,255,0.08);
            `;

            const timeSpan = document.createElement('span');
            timeSpan.className = 'screentime-warning-time';

            const tooltip = document.createElement('span');
            tooltip.className = 'screentime-tooltip';
            tooltip.textContent = 'Screen time limit ends soon';

            warning.appendChild(timeSpan);
            warning.appendChild(tooltip);
            document.body.appendChild(warning);

            // Show tooltip initially
            requestAnimationFrame(() => {
                tooltip.classList.add('screentime-tooltip-visible');
            });

            // Dismiss tooltip on click or scroll
            let tooltipDismissed = false;
            const dismissTooltip = () => {
                if (tooltipDismissed) {
                    return;
                }
                tooltipDismissed = true;
                tooltip.classList.remove('screentime-tooltip-visible');
            };

            // Store handler reference for cleanup
            warning._dismissTooltip = dismissTooltip;
            warning.addEventListener('click', dismissTooltip);
            document.addEventListener('scroll', dismissTooltip, { once: true, passive: true });
            document.addEventListener('click', (e) => {
                if (!warning.contains(e.target)) {
                    dismissTooltip();
                }
            }, { once: true });

            // Animate in
            requestAnimationFrame(() => {
                warning.style.opacity = '1';
                warning.style.transform = 'translateY(0)';
            });
        }

        // Format as MM:SS
        const mins = Math.floor(remainingSeconds / 60);
        const secs = remainingSeconds % 60;
        const timeStr = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;

        const timeSpan = warning.querySelector('.screentime-warning-time');
        if (timeSpan) {
            timeSpan.textContent = timeStr;
        }

        // Add urgent animation for last 10 seconds
        if (remainingSeconds <= 10) {
            warning.classList.add('screentime-warning-urgent');
        } else {
            warning.classList.remove('screentime-warning-urgent');
        }
    }

    function hideWarningTimer() {
        const warning = document.getElementById('screentime-warning');
        if (warning) {
            // Clean up event listener
            if (warning._dismissTooltip) {
                warning.removeEventListener('click', warning._dismissTooltip);
            }
            warning.style.opacity = '0';
            warning.style.transform = 'translateY(-20px)';
            setTimeout(() => warning.remove(), 300);
        }
    }

    /*=========================================================================
     * TIMER
     *=========================================================================*/
    let seconds = loadTime();
    let timer = null;
    const LIMIT = CONFIG.TIME_LIMIT_MINUTES * 60;

    const isLimitReached = () => seconds >= LIMIT;

    function startTimer() {
        stopTimer();

        timer = setInterval(() => {
            const snoozeUntil = Number(localStorage.getItem(KEYS.SNOOZE)) || 0;
            const snoozed = Date.now() < snoozeUntil;

            // Calculate remaining seconds until block (either snooze expiry or limit reached)
            const remaining = snoozed
                ? Math.ceil((snoozeUntil - Date.now()) / 1000)
                : LIMIT - seconds;

            // Show warning for last 2 minutes
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

    /*=========================================================================
     * ENFORCEMENT & INITIALIZATION
     *=========================================================================*/
    let mutationPending = false;
    new MutationObserver(() => {
        if (mutationPending) {
            return;
        }
        mutationPending = true;
        requestAnimationFrame(() => {
            mutationPending = false;
            if (!isSnoozed() && isLimitReached()) {
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

    mediaQuery.addEventListener('change', () => {
        if (document.getElementById('screentime-blocker')) {
            removeBlocker();
            createBlocker();
        }
        // Update warning timer colors on theme change
        const warning = document.getElementById('screentime-warning');
        if (warning) {
            const dark = isDarkMode();
            const bgColor = dark ? 'rgba(60, 60, 67, 0.7)' : 'rgba(255, 255, 255, 0.7)';
            const textColor = dark ? '#fff' : '#333';
            const shadowColor = dark ? 'rgba(0, 0, 0, 0.3)' : 'rgba(0, 0, 0, 0.08)';
            warning.style.backgroundColor = bgColor;
            warning.style.color = textColor;
            warning.style.boxShadow = `0 8px 32px ${shadowColor}, inset 0 1px 0 rgba(255,255,255,0.08)`;
        }
    });

    if (!isSnoozed() && isLimitReached()) {
        createBlocker();
    } else {
        startTimer();
    }
})();

// ignorei18n_end

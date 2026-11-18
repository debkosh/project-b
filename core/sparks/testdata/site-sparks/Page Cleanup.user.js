// ==UserScript==
// @name         Page Cleanup
// @description  Clean up pages by hiding elements with a simple tap.
// @match        <all_urls>
// @run-at       document-end
// ==/UserScript==

/**
 * ─────────────────────────────────────────────────────────────────────────────
 * HOW IT WORKS
 * ─────────────────────────────────────────────────────────────────────────────
 *  1. A draggable FAB (Floating Action Button) appears on every page
 *  2. Tap FAB to open menu with "Start Cleaning" and "Restore All" options
 *  3. In cleaning mode, tap any element to select it for hiding
 *  4. Confirm via bottom sheet → element is hidden and selector saved
 *  5. Hidden elements persist per-domain using localStorage
 *  6. Use "Restore All" to unhide all elements and reset
 *
 * ─────────────────────────────────────────────────────────────────────────────
 * FEATURES
 * ─────────────────────────────────────────────────────────────────────────────
 *
 *  DRAGGABLE FAB
 *    - Drag to reposition anywhere on screen
 *    - Snaps to left/right edge with partial hide effect
 *    - Position persists across page loads
 *    - Pulses gently when idle to indicate availability
 *
 *  CLEANING MODE
 *    - Blue overlay indicates active selection mode
 *    - Tap any element to highlight and select
 *    - Bottom sheet confirms hide action
 *    - Exit via checkmark button (fixed bottom-right)
 *
 *  SMART SELECTOR GENERATION
 *    - Prefers ID selectors when available
 *    - Falls back to aria-label, data-* attributes, or classes
 *    - Validates selector length to avoid overly specific selectors
 *
 *  SAFETY GUARDS
 *    - Cannot hide HTML, BODY, or MAIN elements
 *    - Cannot hide the FAB or its UI components
 *    - Cannot hide Screen Time UI elements (cross-script compatibility)
 *
 *  UI/UX
 *    - Light/Dark mode (follows system preference)
 *    - Liquid glass effect (backdrop blur + transparency)
 *    - Smooth animations (fade, scale, drag)
 *    - Bottom sheet with drag-to-dismiss
 *    - Toast notifications for feedback
 *
 * ─────────────────────────────────────────────────────────────────────────────
 * STORAGE
 * ─────────────────────────────────────────────────────────────────────────────
 *  Uses localStorage with per-domain keys:
 *    page_cleaner_hidden_{host}   - Array of CSS selectors for hidden elements
 *    page_cleaner_fab_position    - FAB position { edge: 'left'|'right', y: number }
 *
 * ═══════════════════════════════════════════════════════════════════════════
 */

// ignorei18n_start

(function () {
    'use strict';

    /*=========================================================================
     * CONSTANTS & STATE
     *=========================================================================*/
    const STORAGE_KEY = `page_cleaner_hidden_${location.hostname}`;
    const FAB_POS_KEY = 'page_cleaner_fab_position';
    const HIDDEN_ATTR = 'data-pc-hidden';
    const DISALLOWED_TAGS = ['HTML', 'BODY', 'MAIN'];
    const MAX_SELECTOR_LENGTH = 200;
    const Z_INDEX = {
        OVERLAY: 2147483640,
        FAB: 2147483641,
        TOP: 2147483642
    };

    let isCleaningModeActive = false;
    let selectedElement = null;
    let fabElement = null;
    let containerElement = null;
    let cleaningHandlers = null;

    /*=========================================================================
     * THEME
     *=========================================================================*/
    const isDarkMode = () => window.matchMedia('(prefers-color-scheme: dark)').matches;

    const THEME = {
        dark: { text: '#fff', sheet: '#1c1c1e', btn: '#2c2c2e', primary: '#0a84ff', backdrop: 'rgba(0,0,0,.6)' },
        light: { text: '#000', sheet: '#fff', btn: '#f2f2f7', primary: '#007aff', backdrop: 'rgba(0,0,0,.35)' }
    };

    const getTheme = () => isDarkMode() ? THEME.dark : THEME.light;
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    const BROOM_ICON = '🧹';

    function applyThemeVars() {
        const t = getTheme();
        const root = document.documentElement;
        root.style.setProperty('--pc-text', t.text);
        root.style.setProperty('--pc-sheet', t.sheet);
        root.style.setProperty('--pc-btn', t.btn);
        root.style.setProperty('--pc-primary', t.primary);
        root.style.setProperty('--pc-backdrop', t.backdrop);
        const dark = isDarkMode();
        root.style.setProperty('--pc-menu-bg', dark ? 'rgba(30, 30, 32, 0.65)' : 'rgba(255, 255, 255, 0.7)');
        root.style.setProperty('--pc-shadow', dark ? 'rgba(0,0,0,0.40)' : 'rgba(0,0,0,0.12)');
    }

    applyThemeVars();
    mediaQuery.addEventListener('change', applyThemeVars);

    /*=========================================================================
     * CSS STYLES
     *=========================================================================*/
    let stylesInjected = false;

    function injectStyles() {
        if (stylesInjected) {
            return;
        }
        stylesInjected = true;

        const styles = document.createElement('style');
        styles.id = 'pc-styles';
        styles.textContent = `
            /* Reset all inherited styles */
            #pc-fab-container,
            #pc-fab-container *,
            #pc-exit-btn,
            #pc-overlay,
            #pc-toast,
            #pc-sheet,
            #pc-sheet *,
            #pc-backdrop {
                all: revert;
                box-sizing: border-box;
            }

            /* Toast notification */
            #pc-toast {
                position: fixed !important;
                bottom: calc(24px + env(safe-area-inset-bottom)) !important;
                left: 50% !important;
                transform: translateX(-50%) translateY(20px) !important;
                z-index: ${Z_INDEX.TOP} !important;
                max-width: calc(100vw - 40px);
                padding: 14px 20px;
                border-radius: 14px;
                font-size: 0.9375rem;
                font-weight: 500;
                font-family: -apple-system, system-ui, sans-serif;
                text-align: center;
                line-height: 1.4;
                -webkit-tap-highlight-color: transparent;
                backdrop-filter: blur(40px) saturate(180%);
                -webkit-backdrop-filter: blur(40px) saturate(180%);
                background-color: var(--pc-menu-bg);
                background-image: linear-gradient(180deg, rgba(255,255,255,0.1), rgba(255,255,255,0.02));
                border: 1px solid rgba(255, 255, 255, 0.12);
                color: var(--pc-text);
                box-shadow: 0 8px 32px var(--pc-shadow), inset 0 1px 0 rgba(255,255,255,0.1);
                opacity: 0;
                transition: opacity .3s ease, transform .3s cubic-bezier(.32,.72,0,1);
            }
            #pc-toast.pc-visible {
                opacity: 1;
                transform: translateX(-50%) translateY(0) !important;
            }
            #pc-toast.pc-hiding {
                opacity: 0;
                transform: translateX(-50%) translateY(10px) !important;
            }

            /* Cleaning overlay */
            #pc-overlay {
                position: fixed !important;
                top: 0 !important;
                left: 0 !important;
                width: 100vw !important;
                width: 100dvw !important;
                height: 100vh !important;
                height: 100dvh !important;
                background: rgba(10, 132, 255, 0.08);
                border: 3px solid var(--pc-primary);
                box-shadow: inset 0 0 30px 10px rgba(10, 132, 255, 0.12);
                z-index: ${Z_INDEX.OVERLAY} !important;
                pointer-events: none;
                opacity: 0;
                transition: opacity .3s ease;
            }
            #pc-overlay.pc-visible { opacity: 1 }

            /* Backdrop */
            #pc-backdrop {
                position: fixed !important;
                top: 0 !important;
                left: 0 !important;
                width: 100vw !important;
                width: 100dvw !important;
                height: 100vh !important;
                height: 100dvh !important;
                z-index: ${Z_INDEX.FAB} !important;
                display: block !important; visibility: visible !important;
                background: var(--pc-backdrop);
                opacity: 0;
                transition: opacity .3s ease;
            }
            #pc-backdrop.pc-visible { opacity: 1 }

            /* Bottom sheet */
            #pc-sheet {
                position: fixed !important;
                left: 0 !important;
                right: 0 !important;
                bottom: 0 !important;
                bottom: env(safe-area-inset-bottom, 0) !important;
                border-radius: 16px 16px 0 0;
                padding: 12px 20px calc(30px + env(safe-area-inset-bottom));
                max-height: 80vh;
                max-height: 80dvh;
                z-index: ${Z_INDEX.TOP} !important;
                display: block !important; visibility: visible !important;
                font-family: -apple-system, system-ui, sans-serif;
                touch-action: none;
                transform: translateY(100%);
                transition: transform .35s cubic-bezier(.32,.72,0,1);
                background: var(--pc-sheet);
                color: var(--pc-text);
            }
            #pc-sheet.pc-visible { transform: translateY(0) }
            #pc-sheet .pc-sheet-handle { width: 36px; height: 5px; background: #888; border-radius: 3px; margin: 0 auto 12px }
            #pc-sheet .pc-sheet-title { text-align: center; margin: 0 0 12px; font-size: 1.125rem; font-weight: 600 }
            #pc-sheet .pc-sheet-buttons { display: grid; gap: 10px }
            #pc-sheet .pc-btn {
                display: block; width: 100%; padding: 14px; font-size: 1.0625rem;
                border-radius: 12px; border: none; cursor: pointer;
                -webkit-tap-highlight-color: transparent;
                touch-action: manipulation;
                transition: transform .1s ease, opacity .2s ease, background .2s ease;
            }
            #pc-sheet .pc-btn:active:not(:disabled) { transform: scale(0.95) }
            #pc-sheet .pc-btn:disabled { opacity: 0.5; cursor: not-allowed }
            #pc-sheet .pc-btn-primary { background: var(--pc-primary); color: #fff }
            #pc-sheet .pc-btn-danger { background: #ff3b30; color: #fff }
            #pc-sheet .pc-btn-secondary { background: var(--pc-btn); color: var(--pc-text) }

            /* FAB Container */
            #pc-fab-container {
                position: fixed !important;
                z-index: ${Z_INDEX.FAB} !important;
                font-family: -apple-system, system-ui, sans-serif;
                -webkit-tap-highlight-color: transparent;
                touch-action: none;
            }
            #pc-fab-container.pc-dragging { transition: none !important }
            #pc-fab-container.pc-edge-right { transition: transform .3s ease }
            #pc-fab-container.pc-edge-left { transition: transform .3s ease }
            #pc-fab-container.pc-edge-right:not(.pc-open):not(.pc-selection-active) { transform: translateX(18px) }
            #pc-fab-container.pc-edge-left:not(.pc-open):not(.pc-selection-active) { transform: translateX(-18px) }
            #pc-fab-container.pc-open, #pc-fab-container.pc-selection-active { transform: translateX(0) !important }

            /* FAB Button */
            #pc-fab {
                width: 50px; height: 50px;
                border-radius: 50%;
                display: flex; align-items: center; justify-content: center;
                font-size: 1.75rem; line-height: 1;
                cursor: grab;
                touch-action: none;
                -webkit-tap-highlight-color: transparent;
                user-select: none; -webkit-user-select: none;
                backdrop-filter: blur(40px) saturate(180%);
                -webkit-backdrop-filter: blur(40px) saturate(180%);
                background-color: var(--pc-menu-bg);
                background-image: linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.02));
                border: 1px solid rgba(255, 255, 255, 0.08);
                color: var(--pc-text);
                box-shadow: 0 8px 32px var(--pc-shadow), inset 0 1px 0 rgba(255,255,255,0.08);
                transition: box-shadow .3s ease, border-color .3s ease, border-radius .2s ease, background-color .3s ease;
            }
            #pc-fab-container.pc-dragging #pc-fab { cursor: grabbing }
            #pc-fab-container:not(.pc-open):not(.pc-dragging):not(.pc-selection-active) #pc-fab { animation: pc-fab-pulse 1.8s ease-in-out infinite }
            #pc-fab-container.pc-open #pc-fab { font-size: 1.625rem }
            /* Hide FAB when in selection mode */
            #pc-fab-container.pc-selection-active #pc-fab { display: none }
            
            /* Exit button for selection mode - fixed at bottom right */
            #pc-exit-btn {
                display: none;
                position: fixed !important;
                bottom: calc(25px + env(safe-area-inset-bottom)) !important;
                right: 20px !important;
                left: auto !important;
                width: 50px; height: 50px;
                border-radius: 50%;
                align-items: center; justify-content: center;
                font-size: 1.625rem; line-height: 1;
                cursor: pointer;
                -webkit-tap-highlight-color: transparent;
                font-family: -apple-system, system-ui, sans-serif;
                backdrop-filter: blur(40px) saturate(180%);
                -webkit-backdrop-filter: blur(40px) saturate(180%);
                background-color: var(--pc-menu-bg);
                background-image: linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.02));
                border: 1px solid rgba(255, 255, 255, 0.08);
                color: var(--pc-text);
                box-shadow: 0 8px 32px var(--pc-shadow), inset 0 1px 0 rgba(255,255,255,0.08);
                transition: transform .15s ease, box-shadow .15s ease, opacity .3s ease;
                z-index: ${Z_INDEX.TOP} !important;
                opacity: 0;
            }
            #pc-exit-btn:active { transform: scale(0.9) }
            #pc-exit-btn.pc-visible { display: flex; opacity: 1 }

            /* FAB Menu */
            #pc-menu {
                position: absolute;
                padding: 8px; border-radius: 14px; min-width: 180px;
                opacity: 0; transform: scale(0.9);
                pointer-events: none;
                transition: opacity .2s ease, transform .2s ease;
                backdrop-filter: blur(40px) saturate(180%);
                -webkit-backdrop-filter: blur(40px) saturate(180%);
                background-color: var(--pc-menu-bg);
                background-image: linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.02));
                border: 1px solid rgba(255, 255, 255, 0.08);
                color: var(--pc-text);
                box-shadow: 0 10px 30px var(--pc-shadow), inset 0 1px 0 rgba(255,255,255,0.08);
            }
            #pc-fab-container.pc-edge-right #pc-menu { right: 64px }
            #pc-fab-container.pc-edge-left #pc-menu { left: 64px }
            #pc-fab-container.pc-menu-top #pc-menu { top: 0; bottom: auto }
            #pc-fab-container.pc-menu-bottom #pc-menu { bottom: 0; top: auto }
            #pc-fab-container.pc-edge-right #pc-menu { transform: scale(0.9) translateX(10px) }
            #pc-fab-container.pc-edge-left #pc-menu { transform: scale(0.9) translateX(-10px) }
            #pc-fab-container.pc-open #pc-menu {
                opacity: 1; transform: scale(1) translateX(0);
                pointer-events: auto;
            }
            #pc-menu .pc-menu-item {
                padding: 14px 12px; cursor: pointer;
                font-size: 1.0625rem; border-radius: 8px;
                touch-action: manipulation;
                -webkit-tap-highlight-color: transparent;
                transition: background .15s ease, opacity .2s ease;
            }
            #pc-menu .pc-menu-item:active { opacity: 0.7 }

            /* Element highlight */
            .pc-highlight { outline: 2px solid var(--pc-primary) !important; outline-offset: 2px }

            /* Animations */
            @keyframes pc-fab-pulse {
                0%, 100% { box-shadow: 0 10px 26px rgba(0,0,0,0.30) }
                50% { box-shadow: 0 14px 34px rgba(0,0,0,0.40), 0 0 0 3px rgba(255,255,255,0.06) }
            }
            /* Landscape adjustments */
            @media (orientation: landscape) and (max-height: 500px) {
                #pc-sheet {
                    padding: 12px 20px 16px;
                    max-height: 90vh;
                    overflow-y: auto;
                }
                #pc-sheet .pc-btn {
                    padding: 12px;
                    font-size: 1rem;
                }
            }
        `;
        document.head.appendChild(styles);
    }

    /*=========================================================================
     * STORAGE
     *=========================================================================*/
    function getHiddenSelectors() {
        try {
            const raw = localStorage.getItem(STORAGE_KEY);
            return raw ? (JSON.parse(raw) || []) : [];
        } catch {
            return [];
        }
    }

    function saveHiddenSelector(selector) {
        const selectors = getHiddenSelectors();
        if (!selectors.includes(selector)) {
            selectors.push(selector);
            try {
                localStorage.setItem(STORAGE_KEY, JSON.stringify(selectors));
            } catch { }
        }
    }

    function clearHiddenSelectors() {
        localStorage.removeItem(STORAGE_KEY);
    }

    function restoreHiddenElements() {
        clearHiddenSelectors();
        location.reload();
    }

    /*=========================================================================
     * APPLY RULES
     *=========================================================================*/
    function applyHiddenElements() {
        const selectors = getHiddenSelectors();
        if (selectors.length === 0) {
            return;
        }

        const combined = selectors
            .map(s => s + ':not([' + HIDDEN_ATTR + '])')
            .join(',');

        try {
            const elements = document.querySelectorAll(combined);
            for (let i = 0; i < elements.length; i++) {
                elements[i].style.display = 'none';
                elements[i].setAttribute(HIDDEN_ATTR, '1');
            }
        } catch { }
    }

    applyHiddenElements();

    let applyPending = false;
    new MutationObserver(() => {
        if (applyPending) {
            return;
        }
        applyPending = true;
        requestAnimationFrame(() => {
            applyPending = false;
            applyHiddenElements();
        });
    }).observe(document.body, { childList: true, subtree: true });

    /*=========================================================================
     * UI HELPERS
     *=========================================================================*/
    function showToast(msg) {
        injectStyles();

        const existing = document.getElementById('pc-toast');
        if (existing) {
            existing.remove();
        }

        const t = document.createElement('div');
        t.id = 'pc-toast';
        t.textContent = msg;
        document.body.appendChild(t);

        requestAnimationFrame(() => {
            t.classList.add('pc-visible');
        });

        setTimeout(() => {
            t.classList.remove('pc-visible');
            t.classList.add('pc-hiding');
            setTimeout(() => t.remove(), 300);
        }, 2000);
    }

    function updateOverlaySize() {
        const o = document.getElementById('pc-overlay');
        if (!o) {
            return;
        }
        // Use visualViewport for accurate dimensions on iOS
        if (window.visualViewport) {
            o.style.width = window.visualViewport.width + 'px';
            o.style.height = window.visualViewport.height + 'px';
            o.style.top = window.visualViewport.offsetTop + 'px';
            o.style.left = window.visualViewport.offsetLeft + 'px';
        }
    }

    function showOverlay() {
        injectStyles();

        if (document.getElementById('pc-overlay')) {
            return;
        }

        const o = document.createElement('div');
        o.id = 'pc-overlay';
        document.body.appendChild(o);

        // Update size immediately and on viewport changes
        updateOverlaySize();
        if (window.visualViewport) {
            window.visualViewport.addEventListener('resize', updateOverlaySize);
            window.visualViewport.addEventListener('scroll', updateOverlaySize);
        }
        window.addEventListener('resize', updateOverlaySize);
        window.addEventListener('orientationchange', updateOverlaySize);

        requestAnimationFrame(() => {
            o.classList.add('pc-visible');
        });
    }

    function removeOverlay() {
        const o = document.getElementById('pc-overlay');
        if (o) {
            // Clean up viewport listeners
            if (window.visualViewport) {
                window.visualViewport.removeEventListener('resize', updateOverlaySize);
                window.visualViewport.removeEventListener('scroll', updateOverlaySize);
            }
            window.removeEventListener('resize', updateOverlaySize);
            window.removeEventListener('orientationchange', updateOverlaySize);

            o.classList.remove('pc-visible');
            setTimeout(() => o.remove(), 300);
        }
    }

    function highlight(el) {
        if (!el) {
            return;
        }
        el.classList.add('pc-highlight');
    }

    function unhighlight(el) {
        if (!el) {
            return;
        }
        el.classList.remove('pc-highlight');
    }

    /*=========================================================================
     * SAFETY
     *=========================================================================*/
    function isElementDisallowed(el) {
        if (!el) {
            return true;
        }
        if (DISALLOWED_TAGS.includes(el.tagName)) {
            return true;
        }
        if (el === document.body || el === document.documentElement) {
            return true;
        }
        // Exclude Page Cleaner UI
        if (el.closest('#pc-fab-container, #pc-exit-btn, #pc-sheet, #pc-toast, #pc-overlay, #pc-backdrop')) {
            return true;
        }
        // Exclude Screen Time UI
        if (el.closest('#screentime-blocker, #screentime-puzzle, #screentime-sheet, #screentime-backdrop, #screentime-warning')) {
            return true;
        }
        return false;
    }

    /*=========================================================================
     * SELECTOR BUILDER
     *=========================================================================*/
    function buildElementSelector(el) {
        // 1. ID selector (most reliable)
        if (el.id && !el.id.startsWith('pc-')) {
            return '#' + CSS.escape(el.id);
        }

        // 2. aria-label (accessibility attribute, usually unique)
        const aria = el.getAttribute('aria-label');
        if (aria) {
            const escapedAria = aria.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
            return el.tagName.toLowerCase() + '[aria-label="' + escapedAria + '"]';
        }

        // 3. role attribute (common in modern web)
        const role = el.getAttribute('role');
        if (role) {
            const roleSelector = el.tagName.toLowerCase() + '[role="' + role + '"]';
            try {
                if (document.querySelectorAll(roleSelector).length <= 3) {
                    return roleSelector;
                }
            } catch { }
        }

        // 4. data-* attribute (often unique identifiers)
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

        // 5. Class-based selector (if unique enough)
        const classes = [...el.classList].filter(c => c.length < 30 && !c.startsWith('pc-'));
        if (classes.length) {
            const sel = el.tagName.toLowerCase() + '.' + classes.map(c => CSS.escape(c)).join('.');
            try {
                if (sel.length <= MAX_SELECTOR_LENGTH && document.querySelectorAll(sel).length <= 3) {
                    return sel;
                }
            } catch { }
        }

        // 6. Build path from nearest ancestor with ID
        const path = [];
        let current = el;
        let foundAnchorId = false;

        while (current && current !== document.body && path.length < 5) {
            let segment = current.tagName.toLowerCase();

            // Check if this ancestor has an ID
            if (current.id && !current.id.startsWith('pc-')) {
                segment = '#' + CSS.escape(current.id);
                path.unshift(segment);
                foundAnchorId = true;
                break;
            }

            // Add classes for specificity
            const currentClasses = [...current.classList].filter(c => c.length < 20 && !c.startsWith('pc-'));
            if (currentClasses.length > 0) {
                segment += '.' + currentClasses.slice(0, 2).map(c => CSS.escape(c)).join('.');
            }

            // Add nth-of-type for uniqueness
            if (current.parentElement) {
                const siblings = [...current.parentElement.children].filter(s => s.tagName === current.tagName);
                if (siblings.length > 1) {
                    const index = siblings.indexOf(current) + 1;
                    segment += ':nth-of-type(' + index + ')';
                }
            }

            path.unshift(segment);
            current = current.parentElement;
        }

        // If we found an ancestor with ID, join with >
        if (foundAnchorId && path.length > 1) {
            const selector = path.join(' > ');
            if (selector.length <= MAX_SELECTOR_LENGTH) {
                return selector;
            }
        }

        // 7. Fallback: use nth-of-type from body
        if (path.length > 0) {
            const selector = path.join(' > ');
            if (selector.length <= MAX_SELECTOR_LENGTH) {
                return selector;
            }
        }

        // 8. Last resort: build a parent-scoped nth-child selector
        const parent = el.parentElement;
        if (parent && parent !== document.body) {
            const childIndex = [...parent.children].indexOf(el) + 1;
            // Try to identify parent uniquely
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

        // Cannot build a reliable selector
        return null;
    }

    /*=========================================================================
     * CONFIRM SHEET UI
     *=========================================================================*/
    function showConfirmSheet(el) {
        injectStyles();

        const backdrop = document.createElement('div');
        backdrop.id = 'pc-backdrop';

        const sheet = document.createElement('div');
        sheet.id = 'pc-sheet';

        const handle = document.createElement('div');
        handle.className = 'pc-sheet-handle';

        const title = document.createElement('div');
        title.className = 'pc-sheet-title';
        title.textContent = 'Hide this element?';

        const hideBtn = document.createElement('button');
        hideBtn.className = 'pc-btn pc-btn-primary';
        hideBtn.textContent = 'Yes';

        const cancelBtn = document.createElement('button');
        cancelBtn.className = 'pc-btn pc-btn-secondary';
        cancelBtn.textContent = 'No';

        const buttonsContainer = document.createElement('div');
        buttonsContainer.className = 'pc-sheet-buttons';
        buttonsContainer.appendChild(hideBtn);
        buttonsContainer.appendChild(cancelBtn);

        sheet.appendChild(handle);
        sheet.appendChild(title);
        sheet.appendChild(buttonsContainer);

        document.body.appendChild(backdrop);
        document.body.appendChild(sheet);

        // Handle viewport changes for iOS fullscreen
        function updateSheetPosition() {
            if (window.visualViewport) {
                const vv = window.visualViewport;
                backdrop.style.width = vv.width + 'px';
                backdrop.style.height = vv.height + 'px';
                backdrop.style.top = vv.offsetTop + 'px';
                backdrop.style.left = vv.offsetLeft + 'px';
                // Position sheet at bottom of visual viewport
                sheet.style.bottom = (window.innerHeight - vv.height - vv.offsetTop) + 'px';
            }
        }
        updateSheetPosition();
        if (window.visualViewport) {
            window.visualViewport.addEventListener('resize', updateSheetPosition);
            window.visualViewport.addEventListener('scroll', updateSheetPosition);
        }

        requestAnimationFrame(() => {
            backdrop.classList.add('pc-visible');
            sheet.classList.add('pc-visible');
        });

        function closeSheet() {
            // Clean up viewport listeners
            if (window.visualViewport) {
                window.visualViewport.removeEventListener('resize', updateSheetPosition);
                window.visualViewport.removeEventListener('scroll', updateSheetPosition);
            }
            sheet.classList.remove('pc-visible');
            backdrop.classList.remove('pc-visible');
            setTimeout(() => {
                sheet.remove();
                backdrop.remove();
            }, 350);
            unhighlight(el);
            selectedElement = null;
        }

        function handleHide(e) {
            e.preventDefault();
            if (e.type === 'touchend') {
                hideBtn.removeEventListener('click', handleHide);
            }
            const selector = buildElementSelector(el);

            // Check if we couldn't build a reliable selector
            if (!selector) {
                closeSheet();
                showToast('This element cannot be hidden.');
                return;
            }

            saveHiddenSelector(selector);
            el.style.display = 'none';
            closeSheet();
        }

        function handleCancel(e) {
            e.preventDefault();
            if (e.type === 'touchend') {
                cancelBtn.removeEventListener('click', handleCancel);
                backdrop.removeEventListener('click', handleCancel);
            }
            closeSheet();
        }

        hideBtn.addEventListener('touchend', handleHide, { passive: false });
        hideBtn.addEventListener('click', handleHide);
        cancelBtn.addEventListener('touchend', handleCancel, { passive: false });
        cancelBtn.addEventListener('click', handleCancel);
        backdrop.addEventListener('click', handleCancel);

        let startY = 0, offset = 0;
        sheet.addEventListener('pointerdown', function (e) {
            e.preventDefault();
            startY = e.clientY;
            sheet.style.transition = 'none';
            sheet.setPointerCapture(e.pointerId);
        });
        sheet.addEventListener('pointermove', function (e) {
            if (!sheet.hasPointerCapture(e.pointerId)) {
                return;
            }
            e.preventDefault();
            const delta = e.clientY - startY;
            offset = delta < 0 ? delta * 0.2 : delta;
            sheet.style.transform = 'translateY(' + offset + 'px)';
        });
        sheet.addEventListener('pointerup', function (e) {
            sheet.releasePointerCapture(e.pointerId);
            sheet.style.transition = '';
            if (offset > 80) {
                closeSheet();
            } else {
                sheet.style.transform = 'translateY(0)';
            }
            offset = 0;
        });
    }

    /*=========================================================================
     * CLEANING MODE
     *=========================================================================*/
    function updateFabIcon() {
        if (!fabElement || !containerElement) {
            return;
        }
        const exitBtn = document.getElementById('pc-exit-btn');
        if (isCleaningModeActive) {
            containerElement.classList.add('pc-selection-active');
            if (exitBtn) {
                exitBtn.classList.add('pc-visible');
            }
        } else if (containerElement.classList.contains('pc-open')) {
            fabElement.textContent = '\u00D7';
            containerElement.classList.remove('pc-selection-active');
            if (exitBtn) {
                exitBtn.classList.remove('pc-visible');
            }
        } else {
            fabElement.textContent = BROOM_ICON;
            containerElement.classList.remove('pc-selection-active');
            if (exitBtn) {
                exitBtn.classList.remove('pc-visible');
            }
        }
    }

    function startPageCleaning() {
        if (isCleaningModeActive) {
            return;
        }
        isCleaningModeActive = true;

        showOverlay();
        updateFabIcon();

        let touchStartX = 0, touchStartY = 0;
        let isTouchActive = false;
        const TAP_THRESHOLD = 15;

        function handleTouchStart(e) {
            if (!isCleaningModeActive) {
                return;
            }
            if (document.getElementById('pc-sheet')) {
                return;
            }

            isTouchActive = true;
            if (e.touches && e.touches.length > 0) {
                touchStartX = e.touches[0].clientX;
                touchStartY = e.touches[0].clientY;
            } else {
                touchStartX = e.clientX;
                touchStartY = e.clientY;
            }
        }

        function handleTap(e) {
            if (!isCleaningModeActive) {
                return;
            }

            if (document.getElementById('pc-sheet')) {
                return;
            }

            // Let Screen Time UI handle its own events
            if (e.target.closest('#screentime-blocker, #screentime-puzzle, #screentime-sheet, #screentime-backdrop')) {
                return;
            }

            const fabContainer = document.getElementById('pc-fab-container');
            const exitBtn = document.getElementById('pc-exit-btn');
            if (fabContainer && fabContainer.contains(e.target)) {
                return;
            }
            if (exitBtn && (e.target === exitBtn || exitBtn.contains(e.target))) {
                return;
            }

            let clientX, clientY;
            if (e.changedTouches && e.changedTouches.length > 0) {
                clientX = e.changedTouches[0].clientX;
                clientY = e.changedTouches[0].clientY;
            } else {
                clientX = e.clientX;
                clientY = e.clientY;
            }

            if (isTouchActive) {
                const dx = Math.abs(clientX - touchStartX);
                const dy = Math.abs(clientY - touchStartY);
                if (dx > TAP_THRESHOLD || dy > TAP_THRESHOLD) {
                    isTouchActive = false;
                    return;
                }
            }
            isTouchActive = false;

            e.preventDefault();
            e.stopPropagation();

            const overlay = document.getElementById('pc-overlay');
            if (overlay) {
                overlay.style.visibility = 'hidden';
            }
            const el = document.elementFromPoint(clientX, clientY);
            if (overlay) {
                overlay.style.visibility = '';
            }

            if (!el || isElementDisallowed(el)) {
                showToast('This element cannot be hidden.');
                return;
            }

            if (selectedElement) {
                unhighlight(selectedElement);
            }
            selectedElement = el;

            highlight(selectedElement);
            showConfirmSheet(selectedElement);
        }

        cleaningHandlers = { start: handleTouchStart, end: handleTap };
        document.addEventListener('touchstart', handleTouchStart, true);
        document.addEventListener('mousedown', handleTouchStart, true);
        document.addEventListener('touchend', handleTap, true);
        document.addEventListener('click', handleTap, true);
    }

    function stopPageCleaning() {
        if (!isCleaningModeActive) {
            return;
        }

        if (cleaningHandlers) {
            document.removeEventListener('touchstart', cleaningHandlers.start, true);
            document.removeEventListener('mousedown', cleaningHandlers.start, true);
            document.removeEventListener('touchend', cleaningHandlers.end, true);
            document.removeEventListener('click', cleaningHandlers.end, true);
            cleaningHandlers = null;
        }

        removeOverlay();
        if (selectedElement) {
            unhighlight(selectedElement);
        }
        selectedElement = null;
        isCleaningModeActive = false;
        updateFabIcon();
    }

    /*=========================================================================
     * FAB MENU UI
     *=========================================================================*/
    function createMenu() {
        if (document.getElementById('pc-fab-container')) {
            return;
        }

        injectStyles();

        let fabPos = { edge: 'right', y: 100 };
        try {
            const saved = localStorage.getItem(FAB_POS_KEY);
            if (saved) {
                fabPos = { ...fabPos, ...JSON.parse(saved) };
            }
        } catch { }

        const container = document.createElement('div');
        container.id = 'pc-fab-container';
        container.classList.add('pc-edge-' + fabPos.edge);
        container.dataset.edge = fabPos.edge;
        containerElement = container;

        function applyPosition() {
            if (fabPos.edge === 'right') {
                container.style.right = '0';
                container.style.left = 'auto';
            } else {
                container.style.left = '0';
                container.style.right = 'auto';
            }
            // Clamp y to ensure FAB stays visible within viewport
            const maxY = window.innerHeight - 80;
            fabPos.y = Math.max(20, Math.min(maxY, fabPos.y));
            container.style.bottom = fabPos.y + 'px';
        }
        applyPosition();

        // Re-clamp position on orientation/resize change
        window.addEventListener('resize', () => {
            applyPosition();
            try {
                localStorage.setItem(FAB_POS_KEY, JSON.stringify(fabPos));
            } catch { }
        });

        const fab = document.createElement('div');
        fab.id = 'pc-fab';
        fab.textContent = BROOM_ICON;
        fabElement = fab;

        let exitBtn = document.getElementById('pc-exit-btn');
        if (!exitBtn) {
            exitBtn = document.createElement('div');
            exitBtn.id = 'pc-exit-btn';
            exitBtn.textContent = '✓';
            document.body.appendChild(exitBtn);
        }

        const menu = document.createElement('div');
        menu.id = 'pc-menu';

        const startBtn = document.createElement('div');
        startBtn.className = 'pc-menu-item';
        startBtn.textContent = 'Start Cleaning';

        const restoreBtn = document.createElement('div');
        restoreBtn.className = 'pc-menu-item';
        restoreBtn.textContent = 'Restore All';

        menu.appendChild(startBtn);
        menu.appendChild(restoreBtn);

        function openFab() {
            const rect = container.getBoundingClientRect();
            const spaceBelow = window.innerHeight - rect.bottom;
            const menuHeight = 120;

            container.classList.remove('pc-menu-top', 'pc-menu-bottom');
            if (spaceBelow < menuHeight && rect.top > menuHeight) {
                container.classList.add('pc-menu-bottom');
            } else {
                container.classList.add('pc-menu-top');
            }

            container.classList.add('pc-open');
            updateFabIcon();
        }

        function closeFab() {
            container.classList.remove('pc-open');
            updateFabIcon();
        }

        let isDragging = false;
        let dragStartX = 0, dragStartY = 0;
        let startPosX = 0, startPosY = 0;
        let hasMoved = false;
        const DRAG_THRESHOLD = 10;

        function handlePointerDown(e) {
            hasMoved = false;
            dragStartX = e.clientX;
            dragStartY = e.clientY;

            if (container.classList.contains('pc-open')) {
                isDragging = false;
                e.preventDefault();
                return;
            }

            if (isCleaningModeActive) {
                isDragging = false;
                e.preventDefault();
                return;
            }

            isDragging = true;
            const rect = container.getBoundingClientRect();
            startPosX = rect.left;
            startPosY = rect.top;
            container.classList.add('pc-dragging');
            container.setPointerCapture(e.pointerId);
            e.preventDefault();
        }

        function handlePointerMove(e) {
            if (!isDragging) {
                return;
            }
            if (isCleaningModeActive) {
                return;
            }

            const dx = e.clientX - dragStartX;
            const dy = e.clientY - dragStartY;

            if (!hasMoved && Math.abs(dx) < DRAG_THRESHOLD && Math.abs(dy) < DRAG_THRESHOLD) {
                return;
            }
            hasMoved = true;

            const newX = startPosX + dx;
            const newY = startPosY + dy;

            const maxY = window.innerHeight - 80;
            const clampedY = Math.max(20, Math.min(maxY, newY));

            container.style.left = newX + 'px';
            container.style.right = 'auto';
            container.style.bottom = 'auto';
            container.style.top = clampedY + 'px';
        }

        function handlePointerUp(e) {
            // Handle tap on FAB when menu is open to close it
            if (container.classList.contains('pc-open') && (e.target === fab || fab.contains(e.target))) {
                closeFab();
                return;
            }

            if (!isDragging) {
                return;
            }
            isDragging = false;
            container.classList.remove('pc-dragging');

            // Release pointer capture - may throw if pointer was already released
            try {
                if (e.pointerId) {
                    container.releasePointerCapture(e.pointerId);
                }
            } catch { }

            if (!hasMoved) {
                if (isCleaningModeActive) {
                    stopPageCleaning();
                    return;
                }
                if (container.classList.contains('pc-open')) {
                    closeFab();
                } else {
                    openFab();
                }
                return;
            }

            const rect = container.getBoundingClientRect();
            const centerX = rect.left + rect.width / 2;
            const screenCenterX = window.innerWidth / 2;

            const yFromBottom = window.innerHeight - rect.top - rect.height;

            fabPos.edge = centerX > screenCenterX ? 'right' : 'left';
            fabPos.y = Math.max(20, Math.min(window.innerHeight - 100, yFromBottom));

            container.classList.remove('pc-edge-right', 'pc-edge-left');
            container.classList.add('pc-edge-' + fabPos.edge);
            container.dataset.edge = fabPos.edge;

            container.style.top = 'auto';
            applyPosition();

            updateFabIcon();

            try {
                localStorage.setItem(FAB_POS_KEY, JSON.stringify(fabPos));
            } catch { }
        }

        fab.addEventListener('pointerdown', handlePointerDown);
        document.addEventListener('pointermove', handlePointerMove);
        document.addEventListener('pointerup', handlePointerUp);

        function handleMenuStart(e) {
            e.preventDefault();
            e.stopPropagation();
            if (e.type === 'touchend') {
                startBtn.removeEventListener('click', handleMenuStart);
            }
            closeFab();
            startPageCleaning();
        }

        function handleMenuRestore(e) {
            e.preventDefault();
            e.stopPropagation();
            if (e.type === 'touchend') {
                restoreBtn.removeEventListener('click', handleMenuRestore);
            }
            closeFab();
            restoreHiddenElements();
        }

        startBtn.addEventListener('touchend', handleMenuStart, { passive: false });
        startBtn.addEventListener('click', handleMenuStart);
        restoreBtn.addEventListener('touchend', handleMenuRestore, { passive: false });
        restoreBtn.addEventListener('click', handleMenuRestore);

        function handleExitClick(e) {
            e.preventDefault();
            e.stopPropagation();
            stopPageCleaning();
        }
        exitBtn.addEventListener('touchend', handleExitClick, { passive: false });
        exitBtn.addEventListener('click', handleExitClick);

        function handleOutsideTap(e) {
            if (!isCleaningModeActive && container.classList.contains('pc-open') && !container.contains(e.target)) {
                closeFab();
            }
        }
        document.addEventListener('touchend', handleOutsideTap);
        document.addEventListener('click', handleOutsideTap);

        container.appendChild(menu);
        container.appendChild(fab);
        document.body.appendChild(container);
    }

    createMenu();

})();

// ignorei18n_end

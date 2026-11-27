// The blocker's watcher (docs/blocking.md, Parts of a page): most hide rules are by class or
// id on any site, so it reads the classes and ids in the document, and those a
// MutationObserver sees added later, at most once per animation frame, and asks the blocker
// about only the names it hasn't asked about before. The engine runs it at document start
// in a world of its own, where __pbBlockerAsk(json, callback) asks the blocker and calls
// back with a JSON array of selectors, and the answer's selectors are hidden by one style
// element of ours.
(() => {
  'use strict';
  if (typeof __pbBlockerAsk !== 'function') return;
  const asked = new Set();
  const hidden = new Set();
  let classes = [];
  let ids = [];
  let style = null;
  let due = false;

  const note = (el) => {
    if (el.nodeType !== 1) return;
    if (el.id && !asked.has('#' + el.id)) {
      asked.add('#' + el.id);
      ids.push(el.id);
    }
    const list = el.classList;
    for (let i = 0; list && i < list.length; i++) {
      const c = list[i];
      if (!asked.has('.' + c)) {
        asked.add('.' + c);
        classes.push(c);
      }
    }
  };

  const noteAll = (root) => {
    note(root);
    if (root.querySelectorAll) {
      for (const el of root.querySelectorAll('[id],[class]')) note(el);
    }
  };

  const hide = (selectors) => {
    let added = false;
    for (const s of selectors) {
      if (typeof s === 'string' && !hidden.has(s)) {
        hidden.add(s);
        added = true;
      }
    }
    if (!added) return;
    if (!style || !style.isConnected) {
      style = document.createElement('style');
      (document.head || document.documentElement).appendChild(style);
    }
    // A rule each: one selector the browser can't read would drop a whole list of them.
    style.textContent = [...hidden].map((s) => s + ' { display: none !important; }').join('\n');
  };

  const send = () => {
    due = false;
    if (!classes.length && !ids.length) return;
    const query = JSON.stringify({ classes, ids });
    classes = [];
    ids = [];
    __pbBlockerAsk(query, (answer) => {
      try {
        const selectors = JSON.parse(answer);
        if (Array.isArray(selectors)) hide(selectors);
      } catch (e) {}
    });
  };

  const soon = () => {
    if (due) return;
    due = true;
    requestAnimationFrame(send);
  };

  new MutationObserver((records) => {
    for (const r of records) {
      if (r.type === 'attributes') {
        note(r.target);
      } else {
        for (const n of r.addedNodes) noteAll(n);
      }
    }
    soon();
  }).observe(document, { childList: true, subtree: true, attributes: true, attributeFilter: ['class', 'id'] });

  if (document.documentElement) noteAll(document.documentElement);
  soon();
})();

// The form script (docs/password-autofill.md): the page side of the page protocol, run in
// each frame, in a world of its own that the page's scripts can't reach. It reports facts
// about the frame's fields, with what DuckDuckGo's matching takes each for, tells when the
// person presses one and when a form is sent or goes, and fills fields on command. It knows nothing of logins and decides nothing: Go
// does (core/forms).
//
// It speaks through a transport its carrier gives it: transport.send(type, payload) to Go,
// and transport.receive(handler) for Go's messages, handler(type, payload). The messages
// are core/forms' types: "fields", "pressed", "submitted", "gone" and "filled" out; "watch"
// and "fill" in. A carrier starts it with formScript(transport), or by leaving the
// transport at globalThis.__pbFormsTransport before this file runs.
(() => {
  // The inputs a role could be typed in. A password field shown as text is one of them.
  const TYPES = new Set(['', 'text', 'email', 'tel', 'url', 'number', 'password', 'search']);
  const MOST = 200; // fields of one frame
  const TEXT = 120; // letters of a label or of the text near a field

  function formScript(transport) {
    let next = 1;
    const refs = new WeakMap(); // element -> ref
    const elements = new Map(); // ref -> WeakRef of its element
    const wasPassword = new WeakSet();
    let watched = new Set();
    let sent = ''; // the last "fields", as JSON, so the same isn't sent twice
    let asked = 0; // when the page last sent a request, for "gone"

    const ref = (el, prefix) => {
      let r = refs.get(el);
      if (!r) {
        r = prefix + next++;
        refs.set(el, r);
        elements.set(r, new WeakRef(el));
      }
      return r;
    };
    const element = (r) => elements.get(r)?.deref();
    const clip = (s) => (s || '').replace(/\s+/g, ' ').trim().slice(0, TEXT);

    const isField = (el) => el instanceof HTMLInputElement && (TYPES.has((el.getAttribute('type') || '').toLowerCase()) || wasPassword.has(el));
    // Seen: drawn, of a size to be seen, on the page and not off its edge, and not hidden
    // from a screen reader, as a field kept only for a password manager to find often is.
    const visible = (el) => {
      if (!el.isConnected || !el.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true })) return false;
      if (el.closest('[aria-hidden="true"]')) return false;
      const box = el.getBoundingClientRect();
      if (box.width <= 2 || box.height <= 2) return false;
      return box.right + scrollX > 0 && box.bottom + scrollY > 0;
    };
    // Able to take the focus: drawn at all, switched on, and in the order of the Tab key.
    const focusable = (el) => el.isConnected && el.getClientRects().length > 0 && !el.disabled && el.tabIndex >= 0 && !el.closest('[inert]');

    // Every input of the frame, those in open shadow trees too, as a page built of its own
    // elements keeps its fields there; each tree found is watched for changes like the page.
    const trees = new WeakSet();
    function inputs(root, out) {
      for (const el of root.querySelectorAll('input')) out.push(el);
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT);
      for (let el = walker.nextNode(); el && out.length < MOST * 4; el = walker.nextNode()) {
        if (!el.shadowRoot) continue;
        if (!trees.has(el.shadowRoot)) {
          trees.add(el.shadowRoot);
          watcher.observe(el.shadowRoot, WATCHED);
        }
        inputs(el.shadowRoot, out);
      }
      return out;
    }
    // The element an event is really at, inside a shadow tree or not.
    const at = (e) => e.composedPath()[0] || e.target;

    // What the page calls a field: its label element, or the label it sits in, or what
    // aria-labelledby names.
    function label(el) {
      const by = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean).map((id) => el.getRootNode().getElementById?.(id)?.textContent || '');
      if (by.join('').trim()) return clip(by.join(' '));
      if (el.labels?.length) return clip([...el.labels].map((l) => l.textContent).join(' '));
      return '';
    }
    // The text just before a field, for one with no label: back through what comes before
    // it in its parent, then its parent's, a few steps.
    function nearby(el) {
      let at = el;
      for (let up = 0; up < 3 && at && at !== el.ownerDocument.body; up++) {
        for (let n = at.previousSibling; n; n = n.previousSibling) {
          if (n.nodeType === Node.COMMENT_NODE) continue;
          if (n.nodeType === Node.ELEMENT_NODE) {
            if (n.querySelector('input, select, textarea')) break;
            // Words a person can't read aren't what the field is called.
            if (n.matches('script, style, template, noscript') || !n.checkVisibility()) continue;
          }
          const text = clip(n.nodeType === Node.ELEMENT_NODE ? n.innerText : n.textContent);
          if (text) return text;
        }
        at = at.parentElement;
      }
      return '';
    }

    function fields() {
      const all = inputs(document, []).filter(isField).slice(0, MOST);
      // What each field is, and the element taken for its form, as DuckDuckGo's matching
      // reads the page (matching.gen.js, which comes before this file). It reads, and
      // changes nothing; only the kind it names leaves the page.
      let read = [];
      try {
        read = globalThis.__pbFormMatching?.(all) || [];
      } catch {
        read = [];
      }
      const forms = new Map();
      const out = all.map((el, i) => {
        if ((el.getAttribute('type') || '').toLowerCase() === 'password') wasPassword.add(el);
        const f = { ref: ref(el, 'f'), tag: 'input', visible: visible(el), focusable: focusable(el), empty: el.value === '' };
        if (el.form) {
          f.form = ref(el.form, 'F');
          forms.set(f.form, el.form);
        }
        const put = (key, value) => value && (f[key] = value);
        put('guess', read[i]?.kind);
        if (read[i]?.form) f.group = ref(read[i].form, 'G');
        put('type', (el.getAttribute('type') || '').toLowerCase());
        put('name', el.name);
        put('id', el.id);
        put('autocomplete', el.getAttribute('autocomplete') || '');
        put('placeholder', clip(el.placeholder));
        put('label', label(el));
        put('ariaLabel', clip(el.getAttribute('aria-label')));
        if (!f.label && !f.ariaLabel) put('nearby', nearby(el));
        if (el.maxLength > 0) f.maxLength = el.maxLength;
        if (f.type !== 'password' && wasPassword.has(el)) f.wasPassword = true;
        if (el.readOnly) f.readOnly = true;
        return f;
      });
      return {
        fields: out,
        forms: [...forms].map(([r, form]) => ({
          ref: r,
          action: form.getAttribute('action') || '',
          method: (form.getAttribute('method') || '').toLowerCase(),
          buttons: [...form.querySelectorAll('button, input[type=submit], input[type=button], [role=button]')].map((b) => clip(b.value || b.textContent || b.getAttribute('aria-label'))).filter(Boolean).slice(0, 8),
        })),
      };
    }

    // What is watched for, in the page and in each shadow tree found in it.
    const WATCHED = { childList: true, subtree: true, attributes: true, attributeFilter: ['type', 'hidden', 'style', 'class', 'autocomplete', 'disabled', 'readonly'] };
    const watcher = new MutationObserver(() => {
      report();
      gone();
    });

    // The fields are told when there are some, and again only when they differ: a moment
    // after the page changes, as it changes in bursts, and at once before anything that
    // names a field, so what is told of a field never comes before the field itself.
    let due = 0;
    function tellFields() {
      clearTimeout(due);
      const now = fields();
      const json = JSON.stringify(now);
      if (json === sent || (!now.fields.length && !sent)) return;
      sent = json;
      transport.send('fields', now);
    }
    function report() {
      clearTimeout(due);
      due = setTimeout(tellFields, 100);
    }

    // The values of the watched fields of a form, or of every watched field with no form.
    function values(form) {
      const out = {};
      for (const r of watched) {
        const el = element(r);
        if (el && (el.form || null) === (form || null)) out[r] = el.value;
      }
      return out;
    }
    const watchedIn = (form) => [...watched].some((r) => element(r) && (element(r).form || null) === (form || null));
    const tell = (type, form) => {
      const v = values(form);
      if (Object.keys(v).length) transport.send(type, { form: form ? ref(form, 'F') : '', values: v });
    };

    // While a list is offered at a field, its arrow keys, Enter and Escape are Go's: told,
    // and kept from the page. Enter only once an arrow has moved in the list; before that
    // it is the page's own, which sends its form. Nothing typed is ever told.
    let keysOf = '';
    let moved = false;
    document.addEventListener('keydown', (e) => {
      if (!keysOf || !e.isTrusted || element(keysOf) !== at(e)) return;
      const arrow = e.key === 'ArrowDown' || e.key === 'ArrowUp';
      if (!arrow && e.key !== 'Escape' && !(e.key === 'Enter' && moved)) return;
      moved = moved || arrow;
      e.preventDefault();
      e.stopImmediatePropagation();
      transport.send('key', { ref: keysOf, key: e.key });
    }, true);

    // A form is sent: its submit event, Enter in one of its fields, or a press on what
    // looks like its button. A page that sends with its own script fires no submit event,
    // so each of them tells, once for a press.
    let told = 0;
    function submitted(form) {
      if (!watchedIn(form) || Date.now() - told < 300) return;
      told = Date.now();
      asked = Date.now();
      tell('submitted', form);
    }
    document.addEventListener('submit', (e) => submitted(e.target instanceof HTMLFormElement ? e.target : null), true);
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && e.isTrusted && at(e) instanceof HTMLInputElement && refs.has(at(e))) submitted(at(e).form);
    }, true);
    document.addEventListener('click', (e) => {
      if (!e.isTrusted || !(at(e) instanceof Element)) return;
      const button = at(e).closest('button, input[type=submit], input[type=button], [role=button], a');
      if (!button) return;
      const form = button.closest('form');
      if (form) return submitted(form);
      // A button in no form sends the fields that are in none.
      if (button.tagName !== 'A') submitted(null);
    }, true);

    // The person presses or comes to a field, or the page puts the focus in one, as an
    // autofocus does: where it is, for the list to be drawn by it. A focus is the browser's
    // own event whoever asked for it, so the page can bring the list up; it can't pick from it.
    function pressed(e) {
      const el = at(e);
      if (!e.isTrusted || !(el instanceof HTMLInputElement) || !isField(el)) return;
      // The fields first, as they are now: a field focused as its page appears, an
      // autofocus or a quick press, was pressed before its page's fields were told, and
      // nothing was offered at it.
      tellFields();
      const box = el.getBoundingClientRect();
      // In the view this frame is drawn in: the frames above it that are of its own site
      // share its view, and each adds where it sits in the one above. A frame of another
      // site has a view of its own, whose place the carrier knows.
      let x = box.x;
      let y = box.y;
      try {
        for (let w = window; w !== w.parent && w.frameElement; w = w.parent) {
          const in_ = w.frameElement.getBoundingClientRect();
          x += in_.x + w.frameElement.clientLeft;
          y += in_.y + w.frameElement.clientTop;
        }
      } catch {
        // The frame above is of another site: this view ends here.
      }
      transport.send('pressed', { ref: ref(el, 'f'), box: { x, y, w: box.width, h: box.height } });
    }
    document.addEventListener('focusin', pressed, true);
    document.addEventListener('pointerdown', (e) => {
      const el = at(e);
      if (el === el.getRootNode().activeElement) return pressed(e);
      // A press on the page that is on no field says so, with no field named: a list
      // offered at a field goes when the person presses elsewhere.
      if (e.isTrusted && !(el instanceof HTMLInputElement && isField(el))) transport.send('pressed', { ref: '', box: { x: 0, y: 0, w: 0, h: 0 } });
    }, true);

    // A watched field that went, or went out of sight, soon after a form was sent: the
    // sign-in went through without the page leaving. Its value is still the element's.
    const here = new Map(); // ref -> whether it was seen last time
    function gone() {
      const lost = new Set();
      for (const r of watched) {
        const el = element(r);
        const now = !!el && visible(el);
        if (here.get(r) && !now && el && Date.now() - asked < 10000) lost.add(el.form || null);
        here.set(r, now);
      }
      for (const form of lost) tell('gone', form);
    }

    watcher.observe(document, WATCHED);
    document.addEventListener('input', (e) => {
      // A field that fills or empties changes what is told of it.
      if (at(e) instanceof HTMLInputElement && refs.has(at(e))) report();
    }, true);

    // A value is set as a person's typing sets it, so a page built on a framework that keeps
    // its own copy of a field hears of it.
    function set(el, value) {
      const own = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
      own.set.call(el, value);
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('change', { bubbles: true }));
    }

    transport.receive((type, payload) => {
      if (type === 'keys') {
        keysOf = payload.ref || '';
        moved = false;
      } else if (type === 'watch') {
        watched = new Set(payload.refs || []);
        here.clear();
        for (const r of watched) here.set(r, !!element(r) && visible(element(r)));
      } else if (type === 'fill') {
        // Only a field still there to be seen and used is filled.
        const done = [];
        for (const [r, value] of Object.entries(payload.values || {})) {
          const el = element(r);
          if (el && isField(el) && visible(el) && focusable(el) && !el.readOnly) {
            set(el, value);
            done.push(r);
          }
        }
        transport.send('filled', { refs: done });
        report();
      }
    });

    report();
    // A shadow tree given to an element already on the page is no change the watcher hears
    // of: the page is read again a few times as it settles.
    for (const ms of [500, 1500, 4000]) setTimeout(report, ms);
  }

  globalThis.formScript = formScript;
  if (globalThis.__pbFormsTransport) formScript(globalThis.__pbFormsTransport);
})();

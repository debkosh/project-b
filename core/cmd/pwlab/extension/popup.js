// What Go's rules made of this tab's fields, a frame at a time, with the test logins that
// match each sign-in form and a button to fill one. A test tool's popup: Project B itself
// draws its list outside the page, which an extension can't.
const out = document.getElementById('out');
const el = (tag, text, cls) => {
  const e = document.createElement(tag);
  if (text) e.textContent = text;
  if (cls) e.className = cls;
  return e;
};

async function show() {
  // The tab the popup hangs from; or, opened as a page of its own, the one named in its
  // address: popup.html?tab=<id>.
  const named = Number(new URLSearchParams(location.search).get('tab'));
  const tab = named ? { id: named } : (await chrome.tabs.query({ active: true, currentWindow: true }))[0];
  const state = await chrome.runtime.sendMessage({ ask: 'tab', tab: tab.id });
  out.textContent = '';
  out.className = '';
  if (state.error) return out.append(el('div', state.error, 'err'));
  if (!state.frames?.length) out.append(el('div', 'No fields a role could be typed in were told for this tab.', 'muted'));
  for (const frame of state.frames || []) {
    const box = el('div', '', 'frame');
    box.append(el('div', `${frame.stamp.top ? 'The page' : 'A frame'}: ${frame.stamp.origin}${frame.stamp.crossOrigin ? ' (under another site)' : ''}${frame.stamp.opaque ? ' (no origin of its own)' : ''}`, 'muted'));
    if (!frame.found.length) box.append(el('div', `${frame.fields} fields, none with a role.`, 'muted'));
    for (const found of frame.found) {
      box.append(el('div', found.kind, 'kind'));
      for (const role of found.roles) {
        const line = el('div');
        line.append(el('span', role.role, 'role'), el('span', ' is '), el('code', role.field));
        box.append(line);
      }
      for (const login of found.logins || []) {
        const b = el('button', `Fill ${login.username || '(no username)'}${login.note ? ` · ${login.note}` : ''}`);
        b.onclick = async () => {
          await chrome.runtime.sendMessage({ ask: 'fill', fill: { tab: frame.stamp.tab, frame: frame.stamp.frame, document: frame.stamp.document, form: found.form, login: login.id } });
          setTimeout(show, 300);
        };
        box.append(b);
      }
      if (found.refused) box.append(el('div', `Nothing is filled here: ${found.refused}`, 'muted'));
    }
    out.append(box);
  }
  if (state.log?.length) {
    out.append(el('h2', 'What happened'));
    out.append(el('div', state.log.join('\n'), 'log'));
  }
}
show();

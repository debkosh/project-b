// The carrier of the test extension (docs/password-autofill.md, Carriers): it gets each
// frame's messages to pwlab, the Go side, stamped with what the browser itself says of the
// frame, never with what the page says; and Go's messages back to that same document.
const LAB = 'http://127.0.0.1:__PORT__';
const ports = new Map(); // "tab:frame" -> the port of the document there now

async function stamp(sender) {
  const origin = sender.origin || 'null';
  let crossOrigin = false;
  // Any frame above it of another origin, as the browser tells its frames.
  try {
    const frames = await chrome.webNavigation.getAllFrames({ tabId: sender.tab.id });
    const byId = new Map(frames.map((f) => [f.frameId, f]));
    for (let at = byId.get(sender.frameId); at && at.parentFrameId >= 0; at = byId.get(at.parentFrameId)) {
      const up = byId.get(at.parentFrameId);
      if (up && new URL(up.url).origin !== origin) crossOrigin = true;
    }
  } catch {}
  return {
    tab: String(sender.tab.id),
    frame: String(sender.frameId),
    document: sender.documentId || '',
    origin,
    top: sender.frameId === 0,
    crossOrigin,
    opaque: origin === 'null' || !/^https?:/.test(origin),
    // An extension isn't told of a certificate that doesn't check; the engine's carrier is.
    secure: origin.startsWith('https://'),
  };
}

async function toLab(path, body) {
  try {
    const r = await fetch(LAB + path, { method: 'POST', body: JSON.stringify(body) });
    return await r.json();
  } catch (e) {
    return { error: `pwlab isn't answering at ${LAB}: ${e.message}` };
  }
}

chrome.runtime.onConnect.addListener((port) => {
  if (port.name !== 'forms' || !port.sender?.tab) return;
  const key = `${port.sender.tab.id}:${port.sender.frameId}`;
  ports.set(key, port);
  port.onDisconnect.addListener(async () => {
    if (ports.get(key) === port) ports.delete(key);
    // The document has gone: Go forgets what it held of it.
    toLab('/page', { stamp: await stamp(port.sender), type: 'left', payload: {} });
  });
  port.onMessage.addListener(async (m) => {
    const answer = await toLab('/page', { stamp: await stamp(port.sender), type: m.type, payload: m.payload });
    for (const out of answer.messages || []) port.postMessage(out);
  });
});

// The popup asks what Go made of a tab, and for a login to be filled.
chrome.runtime.onMessage.addListener((m, _sender, reply) => {
  (async () => {
    if (m.ask === 'tab') return reply(await toLab('/tab', { tab: String(m.tab) }));
    if (m.ask === 'fill') {
      const answer = await toLab('/fill', m.fill);
      const port = ports.get(`${m.fill.tab}:${m.fill.frame}`);
      // Only the document that sent the fields is filled.
      if (answer.message && port && port.sender.documentId === m.fill.document) port.postMessage(answer.message);
      return reply(answer.message ? { ok: !!port } : answer);
    }
  })();
  return true;
});

// The form script's transport in the test extension: a port to the service worker, which
// is the carrier here. Each frame has its own.
(() => {
  const port = chrome.runtime.connect({ name: 'forms' });
  globalThis.__pbFormsTransport = {
    send: (type, payload) => port.postMessage({ type, payload }),
    receive: (handler) => port.onMessage.addListener((m) => handler(m.type, m.payload)),
  };
})();

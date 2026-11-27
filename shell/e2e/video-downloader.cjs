// The built-in Video downloader spark finding streams (docs/video-streams.md,
// core/sparks/builtin/README.md), run in a real browser on pages served here, with
// core/streams' test streams: a playlist the page's player loaded and names nowhere, the
// page's own description of its video, a player that loads nothing until play, a master
// playlist's qualities, live and protected streams, which have no Download, and DASH.
// Run: node shell/e2e/video-downloader.cjs   (from the repo root)

const fs = require('fs');
const http = require('http');
const path = require('path');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '../..');
const streams = path.join(root, 'core/streams/testdata');
const spark = path.join(root, 'core/sparks/builtin/video-downloader.user.js');

let failed = 0;
function check(name, ok, detail) {
  if (!ok) failed++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${ok || detail === undefined ? '' : `: ${typeof detail === 'string' ? detail : JSON.stringify(detail)}`}`);
}

async function until(cond, ms = 8000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    try {
      if (await cond()) return true;
    } catch {}
    await new Promise((r) => setTimeout(r, 50));
  }
  return false;
}

const piece = '#EXTINF:1.0,\nts/piece0.ts\n';
const head = '#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n';
// Playlists written for the test, beside the streams' own.
const lists = {
  '/live.m3u8': head + piece,
  '/locked.m3u8': head + '#EXT-X-KEY:METHOD=SAMPLE-AES,URI="skd://key",KEYFORMAT="com.apple.streamingkeydelivery"\n' + piece + '#EXT-X-ENDLIST\n',
  // Each quality a whole video of its own: the smaller listed first.
  '/qualities.m3u8': '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=30000,RESOLUTION=96x54\nsmall/index.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=60000,RESOLUTION=160x90\nts/index.m3u8\n',
  '/live-master.m3u8': '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=60000,RESOLUTION=160x90\nlive-best.m3u8\n',
  '/live-best.m3u8': head + piece,
  '/live.mpd': '<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic"><Period id="0"><AdaptationSet contentType="video"><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90"/></AdaptationSet></Period></MPD>',
  '/locked.mpd': '<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"><Period id="0"><AdaptationSet contentType="video"><ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"/><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90"/></AdaptationSet></Period></MPD>',
  // Two pictures, the lesser first, and one in WebM, which isn't saved.
  '/two.mpd': '<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT3S"><Period id="0"><AdaptationSet contentType="video">' +
    '<Representation id="a" mimeType="video/mp4" bandwidth="30000" width="640" height="360"/><Representation id="b" mimeType="video/mp4" bandwidth="90000" width="1280" height="720"/>' +
    '<Representation id="c" mimeType="video/webm" bandwidth="900000" width="3840" height="2160"/></AdaptationSet></Period></MPD>',
};

const pages = (cdn) => ({
  // A player of the page's own: it fetches the playlists, and the video's source is a blob.
  '/player': `<!doctype html><title>Player</title><video id=v></video><script>
    document.getElementById('v').src = URL.createObjectURL(new Blob([]));
    // Its addresses are put together as it runs, so nothing written in the page names them.
    const load = (name) => fetch('${cdn}' + '/split/' + name + '.m3u' + '8').then((r) => r.text());
    load('master').then(() => load('video')).then(() => load('audio')).then(() => { document.title = 'loaded'; });
  </script>`,
  // The page's own description, and a player's settings written into the page.
  '/described': `<!doctype html><title>Described</title>
    <meta property="og:video" content="${cdn}/ts/index.m3u8">
    <script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"VideoObject","name":"A film","contentUrl":"${cdn}/fmp4/index.m3u8"}]}</script>
    <script>var player = {"sources":[{"file":"${cdn.replace(/\//g, '\\/')}\\/aes\\/index.m3u8"}]};</script>`,
  '/late': `<!doctype html><title>Late</title><button id=play onclick="fetch('${cdn}' + '/qualities.m3u' + '8').then((r) => r.text())">Play</button>`,
  // A DASH player: its playlist loaded by its script, and others linked.
  '/dash': `<!doctype html><title>DASH</title><video id=v></video><a href="${cdn}/live.mpd">live</a><a href="${cdn}/locked.mpd">locked</a><a href="${cdn}/two.mpd">two</a><script>
    fetch('${cdn}/dash/template/manifest.mpd').then((r) => r.text()).then(() => { document.title = 'loaded'; });
  </script>`,
  // A video's title, for the browser to name its file after: its own label, then its
  // caption, then the nearest heading above it, then the page's og:title.
  '/titled': `<!doctype html><title>Titled</title><meta property="og:title" content="The page's own title">
    <h1>Films</h1>
    <section><h2>Wrong: after its video</h2></section>
    <video src="${cdn}/labelled.mp4" aria-label="Big Buck Bunny: the trailer"></video>
    <figure><video src="${cdn}/captioned.mp4" title=""></video><figcaption>  A caption,
      on two lines </figcaption></figure>
    <article><h2>Sintel &amp; friends / part 2</h2><div><p>About it</p><div><video src="${cdn}/headed.mp4"></video></div></div></article>
    <script>fetch('${cdn}' + '/qualities.m3u' + '8').then((r) => r.text()).then(() => { document.title = 'loaded'; });</script>`,
  '/untitled': `<!doctype html><title>Untitled</title><video src="${cdn}/plain.mp4"></video>`,
  '/og': `<!doctype html><title>Og</title><meta property="og:title" content="Only the page says"><video src="${cdn}/plain.mp4"></video>`,
  // A stream the page's player loaded, beside a link to another's playlist, the link first
  // in the page.
  '/both': `<!doctype html><title>Both</title><video id=v aria-label="Sound and picture"></video>
    <a id=plain href="${cdn}/ts/index.m3u8" style="display:block;padding:12px">A playlist</a>
    <script>fetch("${cdn}" + "/split/master.m3u" + "8").then((r) => r.text()).then(() => { document.body.dataset.played = "yes" })</script>`,
  '/unsaved': `<!doctype html><title>Unsaved</title><a href="${cdn}/live.m3u8">live</a><a href="${cdn}/locked.m3u8">locked</a><a href="${cdn}/live-master.m3u8">live, by its master</a><a href="${cdn}/gone.m3u8">gone</a>`,
});

function serve(handler) {
  return new Promise((resolve) => {
    const server = http.createServer(handler);
    server.listen(0, '127.0.0.1', () => resolve(server));
  });
}

(async () => {
  // The streams' server, another site than the page's as a video host is, which lets any
  // page's script read it, as one a player fetches from does.
  const asked = [];
  const cdnServer = await serve((req, res) => {
    const url = new URL(req.url, 'http://x');
    asked.push(url.pathname);
    res.setHeader('Access-Control-Allow-Origin', '*');
    if (lists[url.pathname]) {
      res.setHeader('Content-Type', 'application/vnd.apple.mpegurl');
      res.end(lists[url.pathname]);
      return;
    }
    const file = path.join(streams, path.normalize(url.pathname));
    if (!file.startsWith(streams) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
      res.statusCode = 404;
      res.end();
      return;
    }
    res.setHeader('Content-Type', file.endsWith('.m3u8') ? 'application/vnd.apple.mpegurl' : file.endsWith('.mpd') ? 'application/dash+xml' : 'application/octet-stream');
    res.end(fs.readFileSync(file));
  });
  const cdn = `http://localhost:${cdnServer.address().port}`;
  const html = pages(cdn);
  const siteServer = await serve((req, res) => {
    const body = html[new URL(req.url, 'http://x').pathname];
    res.statusCode = body ? 200 : 404;
    res.setHeader('Content-Type', 'text/html');
    res.end(body || '');
  });
  const site = `http://127.0.0.1:${siteServer.address().port}`;

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));

  // The spark's widget, read through its shadow root: each row's words, its Download's
  // address, and its menu's items.
  const rows = () => page.evaluate(() => {
    const root = document.getElementById('custom-video-downloader-widget')?.shadowRoot;
    if (!root) return null;
    return [...root.querySelectorAll('.row')].map((row) => ({
      label: row.querySelector('.label')?.textContent,
      host: row.querySelector('.sub')?.textContent,
      note: row.querySelector('.note')?.textContent || '',
      download: row.querySelector('a.btn')?.href || '',
    }));
  });
  const open = async (where, ready) => {
    await page.goto(site + where);
    if (ready) await page.waitForFunction(ready);
    // As the browser runs it: once the page is read.
    await page.addScriptTag({ path: spark });
  };
  const settled = (n) => until(async () => {
    const r = await rows();
    return r && r.length === n && r.every((row) => row.note !== 'Checking the video');
  });
  const menuOf = async (n) => {
    await page.evaluate((i) => {
      const root = document.getElementById('custom-video-downloader-widget').shadowRoot;
      if (root.querySelector('.panel').hidden) root.querySelector('.fab').click();
      root.querySelectorAll('.row-more')[i].click();
    }, n);
    return page.evaluate(() => [...document.getElementById('custom-video-downloader-widget').shadowRoot.querySelectorAll('.menu [role=menuitem]')]
      .map((item) => ({ words: item.textContent, href: item.href || '' })));
  };

  // A stream the page's player loaded before the script ran, named nowhere in the page.
  await open('/player', "document.title === 'loaded'");
  check('a stream the player loaded, named nowhere in the page, is found from the page\'s resource timing', await settled(1), await rows());
  let r = (await rows()) || [];
  check('one row for the stream, by its best quality, not one for each playlist it names', r.length === 1 && r[0].label === 'Video · 90p' && r[0].host === new URL(cdn).host, r);
  check('its sound is a playlist of its own, so Download is the whole stream\'s playlist, which the browser saves at its best with its sound', r[0]?.download === `${cdn}/split/master.m3u8#pb-download`, r[0]?.download);
  let items = await menuOf(0);
  check('and its menu has no lesser quality, which would be saved silent', items.map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);
  check('the button shows once a stream is found', await page.evaluate(() => !document.getElementById('custom-video-downloader-widget').shadowRoot.querySelector('.fab').hidden));

  // The page's own description of its video, and its player's settings.
  asked.length = 0;
  await open('/described');
  check('og:video, a VideoObject\'s contentUrl and a playlist in the player\'s settings are each found', await settled(3), await rows());
  r = (await rows()) || [];
  const found = r.map((row) => row.download).sort();
  check('each a stream with a plain Download link to its playlist, ending #pb-download',
    JSON.stringify(found) === JSON.stringify([`${cdn}/aes/index.m3u8#pb-download`, `${cdn}/fmp4/index.m3u8#pb-download`, `${cdn}/ts/index.m3u8#pb-download`]) && r.every((row) => row.label === 'Video' && !row.note), r);
  check('the script fetched each playlist itself, once', asked.filter((p) => p.endsWith('.m3u8')).sort().join(' ') === '/aes/index.m3u8 /fmp4/index.m3u8 /ts/index.m3u8', asked);

  // A player that loads nothing until play.
  await open('/late');
  await new Promise((done) => setTimeout(done, 400));
  check('a page with no video shows nothing', await page.evaluate(() => document.getElementById('custom-video-downloader-widget').shadowRoot.querySelector('.fab').hidden));
  await page.click('#play');
  check('a stream loaded on play shows up as it loads', await settled(1), await rows());
  r = (await rows()) || [];
  check('a master playlist is one row, its Download the best quality\'s own playlist', r.length === 1 && r[0].label === 'Video · 90p' && r[0].download === `${cdn}/ts/index.m3u8#pb-download`, r);
  items = await menuOf(0);
  check('its menu lists each quality, best first, each a plain link to that quality\'s playlist',
    JSON.stringify(items.slice(0, 2)) === JSON.stringify([{ words: 'Download 90p', href: `${cdn}/ts/index.m3u8#pb-download` }, { words: 'Download 54p', href: `${cdn}/small/index.m3u8#pb-download` }]) &&
    items.slice(2).map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);

  // Live and protected streams, and one whose playlist can't be read.
  await open('/unsaved');
  check('live and protected streams are found', await settled(4), await rows());
  r = (await rows()) || [];
  const by = (name) => r.find((row) => row.download.includes(name));
  const notes = r.map((row) => row.note).sort();
  check('a live stream says "Live: can\'t be saved yet", by its own playlist or its master\'s best, and a protected one "Protected: can\'t be saved"',
    JSON.stringify(notes) === JSON.stringify(['', "Live: can't be saved yet", "Live: can't be saved yet", "Protected: can't be saved"]), r);
  check('and neither has a Download; one whose playlist can\'t be read keeps its Download, for the browser to find out',
    r.filter((row) => row.note).every((row) => !row.download) && r.filter((row) => !row.note).length === 1 && by('gone.m3u8')?.download === `${cdn}/gone.m3u8#pb-download=gone`, r);
  items = await menuOf(0);
  check('a row with no Download still has Copy link and Open in new tab', items.map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);

  // A video's title rides in its link, for the browser to name the file after.
  await open('/titled', "document.title === 'loaded'");
  await settled(4);
  r = (await rows()) || [];
  const titled = (name) => decodeURIComponent((r.find((row) => row.download.includes(name))?.download || '').split('#')[1] || '');
  check('a video\'s own label is the title its Download carries, after #pb-download=', titled('labelled.mp4') === 'pb-download=Big Buck Bunny: the trailer', r);
  check('with no label, its caption', titled('captioned.mp4') === 'pb-download=A caption, on two lines', titled('captioned.mp4'));
  check('with neither, the nearest heading above it', titled('headed.mp4') === 'pb-download=Sintel & friends / part 2', titled('headed.mp4'));
  check('the title is percent-encoded in the link', r.find((row) => row.download.includes('headed.mp4'))?.download === `${cdn}/headed.mp4#pb-download=Sintel%20%26%20friends%20%2F%20part%202`, r);
  check('a stream its player loaded goes by the page\'s main video\'s title', titled('index.m3u8').startsWith('pb-download=') && titled('index.m3u8').length > 'pb-download='.length, titled('index.m3u8'));
  items = await menuOf(r.findIndex((row) => row.download.includes('index.m3u8')));
  check('and each quality in its menu carries it too', items.slice(0, 2).every((i) => i.href.includes('#pb-download=')), items);
  await open('/og');
  await settled(1);
  check('a video with nothing of its own goes by the page\'s og:title', ((await rows()) || [])[0]?.download === `${cdn}/plain.mp4#pb-download=Only%20the%20page%20says`, await rows());
  await open('/untitled');
  await settled(1);
  check('and with no title anywhere, its link is the plain #pb-download', ((await rows()) || [])[0]?.download === `${cdn}/plain.mp4#pb-download`, await rows());

  // What the page really loaded comes before what it only links.
  await open('/both', "document.body.dataset.played === 'yes'");
  await settled(2);
  r = (await rows()) || [];
  check('a stream the page loaded comes before one it only links', r.length === 2 && r[0].download.startsWith(`${cdn}/split/master.m3u8#`) && r[1].download.startsWith(`${cdn}/ts/index.m3u8#`), r);
  check('the loaded one goes by the video\'s own label, and the linked one by its link\'s words',
    r[0]?.download === `${cdn}/split/master.m3u8#pb-download=Sound%20and%20picture` && r[1]?.download === `${cdn}/ts/index.m3u8#pb-download=A%20playlist`, r);

  // DASH: one playlist for every quality.
  await open('/dash', "document.title === 'loaded'");
  check('a DASH playlist the player loaded, and those the page links, are found', await settled(4), await rows());
  r = (await rows()) || [];
  const dash = (name) => r.find((row) => row.download.includes(name)) || {};
  check('a DASH stream\'s row is by its best picture, its Download a plain link to its playlist',
    dash('manifest.mpd').label === 'Video · 90p' && dash('manifest.mpd').download === `${cdn}/dash/template/manifest.mpd#pb-download` &&
    dash('two.mpd').label === 'Video · 720p', r);
  check('a live DASH stream and a protected one say so, with no Download',
    JSON.stringify(r.filter((row) => !row.download).map((row) => row.note).sort()) === JSON.stringify(["Live: can't be saved yet", "Protected: can't be saved"]), r);
  items = await menuOf(r.findIndex((row) => row.download.includes('two.mpd')));
  check('one address holds every quality, so its menu lists none', items.map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);

  check('no errors in the pages', errors.length === 0, errors);
  await browser.close();
  cdnServer.close();
  siteServer.close();
  console.log(failed ? `\n${failed} failed` : '\nall passed');
  process.exit(failed ? 1 : 0);
})();

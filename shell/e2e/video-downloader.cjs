// The built-in Video downloader spark finding streams (docs/video-streams.md,
// core/sparks/builtin/README.md), run in a real browser on pages served here, with
// core/streams' test streams: a playlist the page's player loaded and names nowhere, the
// page's own description of its video, a player that loads nothing until play, a master
// playlist's qualities, live and protected streams, which have no Download, and DASH; and
// saving several at once: Select and its tick boxes, Download all, and the quality chosen;
// streams whose server lets no script read them; and a page of episodes, whose one in use
// takes the page's title.
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
  // Four qualities, each a whole video: 1080, 720, 360 and 240.
  '/ladder.m3u8': '#EXTM3U\n' + [[1920, 1080], [1280, 720], [640, 360], [426, 240]].map(([w, h]) => `#EXT-X-STREAM-INF:BANDWIDTH=${h * 1000},RESOLUTION=${w}x${h}\nladder/${h}.m3u8\n`).join(''),
  '/ladder/1080.m3u8': head + piece + '#EXT-X-ENDLIST\n',
  '/tie.m3u8': '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=1706x960\ntie/960.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=300000,RESOLUTION=854x480\ntie/480.m3u8\n',
  '/tie/960.m3u8': head + piece + '#EXT-X-ENDLIST\n',
  // On a server that lets no page's script read it (the server below sends /closed/ no
  // Access-Control-Allow-Origin): a master of three qualities, which only the browser reads.
  '/closed/a/master.m3u8': '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=1920x1080\nhd.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=300000,RESOLUTION=1280x720\nmid.m3u8\n',
  '/closed/b/master.m3u8': '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=1920x1080\nhd.m3u8\n',
  // Episodes, each in a folder of its own, and two in one folder.
  ...Object.fromEntries(['/eps/1/index.m3u8', '/eps/2/index.m3u8', '/eps/3/index.m3u8', '/flat/ep1.m3u8', '/flat/ep2.m3u8'].map((name) => [name, head + piece + '#EXT-X-ENDLIST\n'])),
  ...Object.fromEntries(['/eps/2/seg1.ts', '/eps/3/seg1.ts', '/flat/ep1-1.ts'].map((name) => [name, 'a piece'])),
  '/live-master.m3u8': '#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=60000,RESOLUTION=160x90\nlive-best.m3u8\n',
  '/live-best.m3u8': head + piece,
  '/live.mpd': '<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic"><Period id="0"><AdaptationSet contentType="video"><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90"/></AdaptationSet></Period></MPD>',
  '/locked.mpd': '<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static"><Period id="0"><AdaptationSet contentType="video"><ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"/><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90"/></AdaptationSet></Period></MPD>',
  // Two pictures, the lesser first, and one in WebM, which isn't saved.
  '/two.mpd': '<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT3S"><Period id="0"><AdaptationSet contentType="video">' +
    '<Representation id="a" mimeType="video/mp4" bandwidth="30000" width="640" height="360"/><Representation id="b" mimeType="video/mp4" bandwidth="90000" width="1280" height="720"/>' +
    '<Representation id="c" mimeType="video/webm" bandwidth="900000" width="3840" height="2160"/></AdaptationSet></Period></MPD>',
};

// A page of episodes: its title, the playlists its settings name, and what its player does.
const episodes = (cdn, title, names, player) => `<!doctype html><title>${title}</title><meta property="og:site_name" content="Films"><meta property="og:title" content="${title}">
  <h1>A show</h1><video id=v></video><button id=next>Next</button>
  <script>var settings = {"episodes":[${names.map((name) => `"${`${cdn}/${name}.m3u8`.replace(/\//g, '\\/')}"`).join(',')}]};</script>
  <script>
    document.getElementById('v').src = URL.createObjectURL(new Blob([]));
    const load = (name) => fetch('${cdn}/' + name + '.m3u' + '8').then((r) => r.text());
    const piece = (name) => fetch('${cdn}/' + name + '.t' + 's').then((r) => r.text());
    ${player}
  </script>`;

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
  // Several to save at once: a file, a stream whose qualities are each a whole video, one
  // whose sound is a playlist of its own, a DASH one, and a live and a protected one.
  '/several': `<!doctype html><title>Several</title>
    <video src="${cdn}/film.mp4" aria-label="A file"></video>
    <a href="${cdn}/ladder.m3u8">A ladder</a>
    <a href="${cdn}/split/master.m3u8">Sound apart</a>
    <a href="${cdn}/two.mpd">Two in one</a>
    <a href="${cdn}/live.m3u8">Live</a>
    <a href="${cdn}/locked.m3u8">Locked</a>`,
  '/files': `<!doctype html><title>Files</title><video src="${cdn}/one.mp4" aria-label="One"></video><video src="${cdn}/two.mp4" aria-label="Two"></video>`,
  // Streams on a server that lets no script read them, and a file; and the page has put its
  // own click() on every link, as a script that opens an advertisement on each click does.
  '/closed': `<!doctype html><title>Closed</title><a href="${cdn}/closed/a/master.m3u8">One</a><a href="${cdn}/closed/b/master.m3u8">Two</a><video src="${cdn}/film.mp4" aria-label="A file"></video>
    <script>HTMLAnchorElement.prototype.click = function () { document.body.dataset.hijacked = 'yes'; };</script>`,
  // A page of episodes, as a site of short dramas is: its settings name every episode's
  // playlist, its player feeds the one video itself, and the heading over it is the show's.
  // The player loads episode 2 and plays it, and loads episode 3 ahead; Next goes on to 3.
  '/episodes': episodes(cdn, 'A show: episode 2 - Films', ['eps/1/index', 'eps/2/index', 'eps/3/index'], `
    load('eps/2/index').then(() => load('eps/3/index')).then(() => piece('eps/2/seg1')).then(() => { document.body.dataset.played = 'yes'; });
    document.getElementById('next').onclick = () => piece('eps/3/seg1').then(() => { document.title = 'A show: episode 3 - Films'; document.body.dataset.played = 'next'; });`),
  // Its player has loaded one episode's playlist and no piece yet.
  '/episodes-one': episodes(cdn, 'A show, episode 1 | Watch it free', ['eps/1/index', 'eps/2/index', 'eps/3/index'], `load('eps/1/index').then(() => { document.body.dataset.played = 'yes'; });`),
  // Nothing loaded: nothing says which is in use.
  '/episodes-idle': episodes(cdn, 'A show: episode 2 - Films', ['eps/1/index', 'eps/2/index', 'eps/3/index'], `document.body.dataset.played = 'yes';`),
  // Two episodes in one folder, both loaded: a piece from the folder says nothing.
  '/episodes-flat': episodes(cdn, 'A show: episode 1 - Films', ['flat/ep1', 'flat/ep2'], `load('flat/ep1').then(() => load('flat/ep2')).then(() => piece('flat/ep1-1')).then(() => { document.body.dataset.played = 'yes'; });`),
  // Two files, each a video's own source, under the show's heading: the second has played.
  '/episodes-files': `<!doctype html><title>A show: episode 2 - Films</title><meta property="og:site_name" content="Films"><h1>A show</h1>
    <video src="${cdn}/ep-one.mp4"></video><video id=second src="${cdn}/ep-two.mp4"></video>
    <script>Object.defineProperty(document.getElementById('second'), 'currentTime', { get: () => 12 });</script>`,
  '/tie': `<!doctype html><title>Tie</title><a href="${cdn}/tie.m3u8">A tie</a><a href="${cdn}/split/master.m3u8">Sound apart</a>`,
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
    if (!url.pathname.startsWith('/closed/')) res.setHeader('Access-Control-Allow-Origin', '*');
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
  // The widget's shadow root, for what runs in the page.
  await page.addInitScript(() => { window.widget = () => document.getElementById('custom-video-downloader-widget').shadowRoot; });

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

  // The panel's own menu, opened: its items' words.
  const panelMenu = async () => {
    await page.evaluate(() => {
      const root = widget();
      if (root.querySelector('.panel').hidden) root.querySelector('.fab').click();
      if (root.querySelector('.menu').hidden || root.querySelector('.panel-more').getAttribute('aria-expanded') !== 'true') root.querySelector('.panel-more').click();
    });
    return page.evaluate(() => [...widget().querySelectorAll('.menu [role=menuitem]')].map((item) => item.textContent));
  };

  // A stream the page's player loaded before the script ran, named nowhere in the page.
  await open('/player', "document.title === 'loaded'");
  check('a stream the player loaded, named nowhere in the page, is found from the page\'s resource timing', await settled(1), await rows());
  let r = (await rows()) || [];
  check('one row for the stream, by its best quality, not one for each playlist it names', r.length === 1 && r[0].label === 'Video · 90p' && r[0].host === new URL(cdn).host, r);
  check('its sound is a playlist of its own, so Download is the whole stream\'s playlist, which the browser saves at its best with its sound', r[0]?.download === `${cdn}/split/master.m3u8#pb-download`, r[0]?.download);
  let items = await menuOf(0);
  check('and its menu lists each quality, the best the playlist alone and a lesser one the playlist asked for at its height, which the browser saves with its sound',
    JSON.stringify(items.slice(0, 2)) === JSON.stringify([{ words: 'Download 90p', href: `${cdn}/split/master.m3u8#pb-download` }, { words: 'Download 54p', href: `${cdn}/split/master.m3u8#pb-download=&q=54` }]) &&
    items.slice(2).map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);
  check('a page with one video has nothing to save several of', (await panelMenu()).join('|') === 'Look again|Hide on this page', await panelMenu());
  await page.keyboard.press('Escape');
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
  check('one address holds every quality, so its menu lists each as the playlist asked for at that height, but for the best',
    JSON.stringify(items.slice(0, 2)) === JSON.stringify([{ words: 'Download 720p', href: `${cdn}/two.mpd#pb-download=two` }, { words: 'Download 360p', href: `${cdn}/two.mpd#pb-download=two&q=360` }]) &&
    items.slice(2).map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);
  await page.keyboard.press('Escape');

  // Saving several at once.
  await open('/several');
  check('six videos are found, two of them live or protected', await settled(6), await rows());
  // Every link the widget follows is noted, with when, and goes nowhere.
  await page.evaluate(() => {
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  const followed = () => page.evaluate(() => window.followed);
  const ui = () => page.evaluate(() => {
    const root = widget();
    const shown = (sel) => { const node = root.querySelector(sel); return !!node && !node.hidden && !root.querySelector('.bar').hidden; };
    return {
      title: root.querySelector('.title').textContent,
      bar: !root.querySelector('.bar').hidden,
      ask: shown('.ask') ? root.querySelector('.ask').textContent : '',
      cancel: shown('.bar-cancel'),
      go: !root.querySelector('.bar-go').disabled,
      // The All box: '' while there is none, else ticked, mixed or empty.
      every: ((box) => (box.hidden ? '' : box.indeterminate ? 'mixed' : box.checked ? 'ticked' : 'empty'))(root.querySelector('.pick-every')),
      everyFirst: root.querySelector('.head').firstElementChild === root.querySelector('.pick-every') && root.querySelector('.pick-every').nextElementSibling === root.querySelector('.title'),
      done: !root.querySelector('.pick-done').hidden && root.querySelector('.pick-done').textContent === 'Done',
      close: !root.querySelector('.panel-close').hidden,
      more: !root.querySelector('.panel-more').hidden,
      hint: shown('.hint') ? root.querySelector('.hint').textContent : '',
      qualityShown: shown('.quality'),
      quality: root.querySelector('.quality-choice').value,
      choices: [...root.querySelectorAll('.quality-choice option')].map((o) => o.textContent).join('|'),
      ticks: [...root.querySelectorAll('.row')].map((row) => { const t = row.querySelector('.tick'); return t ? (t.disabled ? 'x' : t.checked ? '1' : '0') : '-'; }).join(''),
      downloads: root.querySelectorAll('.row a.btn').length,
      panel: !root.querySelector('.panel').hidden,
    };
  });
  const press = (sel) => page.evaluate((s) => widget().querySelector(s).click(), sel);
  const pick = async (words) => {
    const items = await panelMenu();
    await page.evaluate((w) => [...widget().querySelectorAll('.menu [role=menuitem]')].find((item) => item.textContent.startsWith(w)).click(), words);
    return items;
  };
  r = (await rows()) || [];
  // The rows, by what each is, wherever it is in the list.
  const at = (name) => r.findIndex((row) => (row.download || row.note).includes(name) || false);
  const order = r.map((row) => (row.note.startsWith('Live') ? 'live' : row.note.startsWith('Protected') ? 'locked' : ['film.mp4', 'ladder', 'split/master', 'two.mpd'].find((n) => row.download.includes(n))));
  check('the page\'s rows are a file, three streams, a live one and a protected one', [...order].sort().join(' ') === 'film.mp4 ladder live locked split/master two.mpd', order);
  const tickOf = (name) => page.evaluate((i) => widget().querySelectorAll('.row')[i].querySelector('.tick').click(), order.indexOf(name));
  const expectTicks = (on) => order.map((name) => (name === 'live' || name === 'locked' ? 'x' : on.includes(name) ? '1' : '0')).join('');

  items = await pick('Select');
  check('the panel\'s menu has Download all, counting the rows with a Download, and Select', items.join('|') === 'Download all (4)|Select|Look again|Hide on this page', items);
  let u = await ui();
  check('Select puts a tick box on each row, none ticked, and those of a live and a protected video can\'t be', u.ticks === expectTicks([]) && u.downloads === 0, u);
  check('the head is an All box, empty, then "0 selected", with Done in place of the close button and the menu, and a bar whose Download has nothing to do yet',
    u.title === '0 selected' && u.every === 'empty' && u.everyFirst && u.done && !u.close && !u.more && u.bar && !u.ask && !u.cancel && !u.go, u);
  check('the All box stands where the rows\' boxes do', await page.evaluate(() => Math.abs(widget().querySelector('.pick-every').getBoundingClientRect().left - widget().querySelector('.row .tick').getBoundingClientRect().left) < 1));
  check('with nothing ticked there is no quality to choose, and nothing said of what the browser asks', !u.qualityShown && !u.hint, u);
  await tickOf('film.mp4');
  u = await ui();
  check('a file ticked alone has no quality to choose either, and the All box is mixed', u.title === '1 selected' && !u.qualityShown && !u.hint && u.every === 'mixed' && u.go, u);
  await tickOf('ladder');
  await tickOf('live');
  u = await ui();
  check('ticking a second counts them, and a live one\'s box stays as it was', u.title === '2 selected' && u.ticks === expectTicks(['ladder', 'film.mp4']) && u.every === 'mixed', u);
  check('a stream of several qualities among them brings the quality: Best, of Best, 1080p, 720p, 480p and 360p', u.qualityShown && u.quality === '0' && u.choices === 'Best|1080p|720p|480p|360p', u);
  check('and with two to save, a line says what the browser will ask', u.hint === 'Your browser will ask once to allow several downloads from this site.', u);
  await tickOf('film.mp4');
  u = await ui();
  check('a tick comes off again', u.title === '1 selected' && u.ticks === expectTicks(['ladder']) && u.qualityShown && !u.hint, u);
  await press('.pick-every');
  u = await ui();
  check('the All box ticks every row that can be saved, and is ticked', u.title === '4 selected' && u.ticks === expectTicks(['film.mp4', 'ladder', 'split/master', 'two.mpd']) && u.every === 'ticked', u);
  await press('.pick-every');
  u = await ui();
  check('and unticks them all again', u.title === '0 selected' && u.ticks === expectTicks([]) && u.every === 'empty' && !u.go, u);
  await page.keyboard.press('Escape');
  u = await ui();
  check('Escape leaves selecting, the panel still open with its rows\' Downloads and its close button', u.panel && !u.bar && u.title === '6 videos on this page' && u.downloads === 4 && u.more && u.close && !u.every && !u.done, u);
  await pick('Select');
  await press('.pick-done');
  u = await ui();
  check('and so does Done', u.panel && !u.bar && u.downloads === 4 && u.close, u);

  // The quality, and what each link asks for.
  await pick('Select');
  await press('.pick-every');
  await tickOf('two.mpd');
  await page.evaluate(() => { const c = widget().querySelector('.quality-choice'); c.value = '480'; c.dispatchEvent(new Event('change', { bubbles: true })); });
  check('the quality chosen is kept in the site\'s own storage', await page.evaluate(() => localStorage.getItem('video_downloader_quality')) === '480');
  check('nothing is followed until Download', (await followed()).length === 0, await followed());
  await press('.bar-go');
  check('Download starts the first at once', (await followed()).length === 1, await followed());
  await until(async () => (await followed()).length === 3);
  await new Promise((done) => setTimeout(done, 700));
  let got = await followed();
  const hrefs = (list) => list.map((f) => f.href);
  const apart = (list) => list.slice(1).map((f, i) => Math.round(f.at - list[i].at));
  check('the three ticked are saved in the list\'s order, the unticked one left', got.length === 3 && JSON.stringify(hrefs(got).map((h) => ['film.mp4', 'ladder', 'split/master', 'two.mpd'].find((n) => h.includes(n)))) === JSON.stringify(order.filter((n) => ['film.mp4', 'ladder', 'split/master'].includes(n))), hrefs(got));
  const link = (list, name) => hrefs(list).find((h) => h.includes(name));
  check('a file is saved as it is, under its title', link(got, 'film.mp4') === `${cdn}/film.mp4#pb-download=A%20file`, link(got, 'film.mp4'));
  check('at 480p a stream of 1080, 720, 360 and 240 is saved at 360, the closest, by that quality\'s own playlist', link(got, 'ladder') === `${cdn}/ladder/360.m3u8#pb-download=A%20ladder`, link(got, 'ladder'));
  check('and one whose sound is apart by its master playlist asked for at 480, for the browser to choose', link(got, 'split/master') === `${cdn}/split/master.m3u8#pb-download=Sound%20apart&q=480`, link(got, 'split/master'));
  check('one after another, a moment apart', apart(got).every((ms) => ms >= 300 && ms <= 1500), apart(got));
  u = await ui();
  r = (await rows()) || [];
  check('selecting is over, and each row saved says "Downloading: see Downloads"', !u.bar && r.filter((row) => row.note === 'Downloading: see Downloads').length === 3 && u.downloads === 1, r);
  check('with one left to save, the menu offers no more to save several of', (await panelMenu()).join('|') === 'Look again|Hide on this page', await panelMenu());
  await page.keyboard.press('Escape');

  // The quality is remembered on the site, and Download all asks once.
  await open('/several');
  await settled(6);
  await page.evaluate(() => {
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  await pick('Download all');
  u = await ui();
  check('Download all asks once in the panel, "Download 4 videos?", with Download and Cancel, the rows as they were',
    u.ask === 'Download 4 videos?' && u.cancel && u.go && u.downloads === 4 && u.title === '6 videos on this page' && u.close && !u.every, u);
  check('with the line saying what the browser will ask, and the quality, there being streams of several', u.hint === 'Your browser will ask once to allow several downloads from this site.' && u.qualityShown, u);
  check('at the quality last chosen on this site', u.quality === '480', u);
  await press('.bar-cancel');
  u = await ui();
  check('Cancel saves nothing', !u.bar && (await followed()).length === 0 && u.downloads === 4, u);
  await pick('Download all');
  await page.keyboard.press('Escape');
  u = await ui();
  check('nor does Escape, which leaves the panel open', !u.bar && u.panel && (await followed()).length === 0, u);
  await pick('Download all');
  await page.evaluate(() => { const c = widget().querySelector('.quality-choice'); c.value = '720'; c.dispatchEvent(new Event('change', { bubbles: true })); });
  await press('.bar-go');
  await until(async () => (await followed()).length === 4);
  await new Promise((done) => setTimeout(done, 700));
  got = await followed();
  check('Download saves all four, live and protected ones left out', got.length === 4 && !hrefs(got).some((h) => h.includes('live') || h.includes('locked')), hrefs(got));
  check('at 720p: the ladder\'s own 720, the DASH playlist and the master asked for at 720, the file as it is',
    link(got, 'ladder') === `${cdn}/ladder/720.m3u8#pb-download=A%20ladder` && link(got, 'two.mpd') === `${cdn}/two.mpd#pb-download=Two%20in%20one&q=720` &&
    link(got, 'split/master') === `${cdn}/split/master.m3u8#pb-download=Sound%20apart&q=720` && link(got, 'film.mp4') === `${cdn}/film.mp4#pb-download=A%20file`, hrefs(got));
  check('each a moment after the last', apart(got).length === 3 && apart(got).every((ms) => ms >= 300 && ms <= 1500), apart(got));

  // A tie goes to the smaller, and Best is each one's best.
  await open('/tie');
  await settled(2);
  await page.evaluate(() => {
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  await pick('Download all');
  check('the quality is this site\'s still', (await ui()).quality === '720', await ui());
  await press('.bar-go');
  await until(async () => (await followed()).length === 2);
  got = await followed();
  check('as close to 480 as to 960, a stream is saved at the smaller', link(got, 'tie/') === `${cdn}/tie/480.m3u8#pb-download=A%20tie`, hrefs(got));
  await open('/tie');
  await settled(2);
  await page.evaluate(() => {
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  await pick('Download all');
  await page.evaluate(() => { const c = widget().querySelector('.quality-choice'); c.value = '0'; c.dispatchEvent(new Event('change', { bubbles: true })); });
  check('Best keeps nothing in the site\'s storage', await page.evaluate(() => localStorage.getItem('video_downloader_quality')) === null);
  await press('.bar-go');
  await until(async () => (await followed()).length === 2);
  got = await followed();
  check('at Best each is its best, with no quality asked for', link(got, 'tie/') === `${cdn}/tie/960.m3u8#pb-download=A%20tie` && link(got, 'split/master') === `${cdn}/split/master.m3u8#pb-download=Sound%20apart`, hrefs(got));

  // Files alone: no quality to choose, though the site has one kept.
  await open('/files');
  await settled(2);
  await page.evaluate(() => {
    localStorage.setItem('video_downloader_quality', '480');
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  await pick('Download all');
  u = await ui();
  check('Download all of plain files shows no quality, only what it asks and what the browser will', u.ask === 'Download 2 videos?' && !u.qualityShown && !!u.hint, u);
  await press('.bar-go');
  await until(async () => (await followed()).length === 2);
  got = await followed();
  check('and each is saved as it is', JSON.stringify(hrefs(got).sort()) === JSON.stringify([`${cdn}/one.mp4#pb-download=One`, `${cdn}/two.mp4#pb-download=Two`]), hrefs(got));

  // On a phone, a row that can't be saved keeps its host, as on the desktop.
  await page.setViewportSize({ width: 390, height: 760 });
  await open('/several');
  await settled(6);
  await page.evaluate(() => widget().querySelector('.fab').click());
  const hosts = () => page.evaluate(() => [...widget().querySelectorAll('.row')].filter((row) => row.querySelector('.note')).map((row) => getComputedStyle(row.querySelector('.sub')).display !== 'none'));
  check('on a phone a live and a protected row show their host over what they say', JSON.stringify(await hosts()) === '[true,true]', await hosts());
  await pick('Select');
  check('and while selecting too', JSON.stringify(await hosts()) === '[true,true]', await hosts());
  await page.evaluate(() => {
    // The link followed goes nowhere, as above.
    widget().addEventListener('click', (event) => {
      if (event.composedPath().some((node) => node.tagName === 'A')) event.preventDefault();
    }, true);
    widget().querySelector('.row .tick:not(:disabled)').click();
    widget().querySelector('.bar-go').click();
  });
  await until(async () => (await hosts()).length === 3);
  check('a row that is downloading says so in place of its host, in its one line', JSON.stringify((await hosts()).sort()) === '[false,true,true]', await hosts());

  // A stream whose server lets no page's script read its playlist: the browser can, so it
  // may have qualities, and its link asks for the one chosen.
  await open('/closed');
  await settled(3);
  await page.evaluate(() => {
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  r = (await rows()) || [];
  check('a stream the script can\'t read is a row with no quality and a Download for its playlist as it is',
    r.filter((row) => row.label === 'Video' && !row.note && /\/closed\/[ab]\/master\.m3u8#pb-download=(One|Two)$/.test(row.download)).length === 2, r);
  items = await menuOf(r.findIndex((row) => row.download.includes('/closed/a/')));
  check('its menu can list no quality', items.map((i) => i.words).join('|') === 'Copy link|Open in new tab', items);
  await page.keyboard.press('Escape');
  await pick('Select');
  const closedOrder = r.map((row) => ['film.mp4', '/closed/a/', '/closed/b/'].find((n) => row.download.includes(n)));
  const tickClosed = (name) => page.evaluate((i) => widget().querySelectorAll('.row')[i].querySelector('.tick').click(), closedOrder.indexOf(name));
  await tickClosed('film.mp4');
  check('a file ticked alone still has no quality to choose', !(await ui()).qualityShown, await ui());
  await tickClosed('/closed/a/');
  check('a stream that couldn\'t be read, ticked, brings the quality: it may hold several', (await ui()).qualityShown, await ui());
  await page.keyboard.press('Escape');
  await pick('Download all');
  u = await ui();
  check('Download all with such streams among them has the quality too', u.ask === 'Download 3 videos?' && u.qualityShown, u);
  await page.evaluate(() => { const c = widget().querySelector('.quality-choice'); c.value = '720'; c.dispatchEvent(new Event('change', { bubbles: true })); });
  await press('.bar-go');
  await until(async () => (await followed()).length === 3);
  got = await followed();
  check('at 720p each such stream is its playlist asked for at 720, for the browser to choose, and the file as it is',
    link(got, '/closed/a/') === `${cdn}/closed/a/master.m3u8#pb-download=One&q=720` && link(got, '/closed/b/') === `${cdn}/closed/b/master.m3u8#pb-download=Two&q=720` &&
    link(got, 'film.mp4') === `${cdn}/film.mp4#pb-download=A%20file`, hrefs(got));
  await open('/closed');
  await settled(3);
  await page.evaluate(() => {
    window.followed = [];
    widget().addEventListener('click', (event) => {
      const link = event.composedPath().find((node) => node.tagName === 'A');
      if (!link) return;
      event.preventDefault();
      window.followed.push({ href: link.href, at: performance.now() });
    }, true);
  });
  await pick('Download all');
  await page.evaluate(() => { const c = widget().querySelector('.quality-choice'); c.value = '0'; c.dispatchEvent(new Event('change', { bubbles: true })); });
  await press('.bar-go');
  await until(async () => (await followed()).length === 3);
  got = await followed();
  check('each link is followed though the page has replaced click() on its links, which is never called', got.length === 3 && await page.evaluate(() => document.body.dataset.hijacked) === undefined, hrefs(got));
  check('and at Best it is the playlist with no quality asked for', link(got, '/closed/a/') === `${cdn}/closed/a/master.m3u8#pb-download=One`, hrefs(got));

  // A page of episodes: the one in use takes the page's title, the rest keep the show's.
  const named = async (n) => {
    await settled(n);
    return Object.fromEntries(((await rows()) || []).map((row) => [new URL(row.download).pathname, decodeURIComponent(row.download.split('#pb-download=')[1] || '')]));
  };
  const same = (got, want) => JSON.stringify(Object.entries(got).sort()) === JSON.stringify(Object.entries(want).sort());
  await open('/episodes', "document.body.dataset.played === 'yes'");
  let names = await named(3);
  check('of the episodes a page names, the one whose pieces its player is fetching takes the page\'s title, without the site\'s name, and the rest the show\'s, one loaded ahead too',
    same(names, { '/eps/1/index.m3u8': 'A show', '/eps/2/index.m3u8': 'A show: episode 2', '/eps/3/index.m3u8': 'A show' }), names);
  await page.click('#next');
  await page.waitForFunction("document.body.dataset.played === 'next'");
  await until(async () => (await named(3))['/eps/3/index.m3u8'] === 'A show: episode 3');
  names = await named(3);
  check('when the player goes on to the next, that one takes the page\'s new title and the last is the show\'s again',
    same(names, { '/eps/1/index.m3u8': 'A show', '/eps/2/index.m3u8': 'A show', '/eps/3/index.m3u8': 'A show: episode 3' }), names);
  await open('/episodes-one', "document.body.dataset.played === 'yes'");
  names = await named(3);
  check('with no piece fetched yet, the one playlist the page loaded is the one in use; a title\'s last part that isn\'t the site\'s name stays',
    same(names, { '/eps/1/index.m3u8': 'A show, episode 1 | Watch it free', '/eps/2/index.m3u8': 'A show', '/eps/3/index.m3u8': 'A show' }), names);
  await open('/episodes-idle', "document.body.dataset.played === 'yes'");
  names = await named(3);
  check('with nothing loaded, nothing says which is in use, and each keeps the show\'s title', same(names, { '/eps/1/index.m3u8': 'A show', '/eps/2/index.m3u8': 'A show', '/eps/3/index.m3u8': 'A show' }), names);
  await open('/episodes-flat', "document.body.dataset.played === 'yes'");
  names = await named(2);
  check('two episodes in one folder: a piece from it says nothing, and each keeps the show\'s title', same(names, { '/flat/ep1.m3u8': 'A show', '/flat/ep2.m3u8': 'A show' }), names);
  await open('/episodes-files');
  names = await named(2);
  check('of two files, the one that is the played video\'s own source takes the page\'s title', same(names, { '/ep-one.mp4': 'A show', '/ep-two.mp4': 'A show: episode 2' }), names);

  check('no errors in the pages', errors.length === 0, errors);
  await browser.close();
  cdnServer.close();
  siteServer.close();
  console.log(failed ? `\n${failed} failed` : '\nall passed');
  process.exit(failed ? 1 : 0);
})();

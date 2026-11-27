#!/usr/bin/env python3
"""Drives the real Android app on an emulator: real touches, real keys, real screenshots.

Where to touch comes from the app itself: Android's accessibility tree, which content
fills with the shell's controls by their labels, the settings page and the shown page,
each with its place on the screen. Nothing here is a fixed position, so a change to the
shell's layout needs no new numbers. A small local web server is the oracle for what the
page received, and the app's own store.db for what the managers kept. Results and
screenshots go to build/android-drive-content.

    amarit start project-b-emu          # the emulator; docs/android.md says how it was made
    tools/test.sh android               # or: python3 tools/drive_android.py
    python3 tools/drive_android.py links_to_other_sites   # only the parts named

ADB_SERIAL picks another device than emulator-5600, and PB_APK another APK than
build/android/project-b-content.apk. The APK is installed over what is there and the
app's data is cleared.
"""
import atexit
import http.server
import json
import mimetypes
import os
import re
import shlex
import shutil
import sqlite3
import subprocess
import sys
import threading
import struct
import time
import urllib.parse
import xml.etree.ElementTree as ET
import zipfile
import zlib

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import links_page  # noqa: E402
import mp4_length  # noqa: E402
import web_features  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# The small streams core/streams' tests save, served to the phone (stream_download).
STREAMS = os.path.join(ROOT, "core", "streams", "testdata")
SDK = os.path.expanduser("~/Library/Android/sdk")
ADB = [SDK + "/platform-tools/adb", "-s", os.environ.get("ADB_SERIAL", "emulator-5600")]
APK = os.environ.get("PB_APK") or ROOT + "/build/android/project-b-content.apk"
PACKAGE = "dev.projectb.browser"
ACTIVITY = ".BrowserActivity"
# The app's name, as the launcher and the recent apps show it.
APP_NAME = "Project B"
PROFILE = "app_project_b/profile"
OUT = os.environ.get("OUT", ROOT + "/build/android-drive-content")
FIELD = "android.widget.EditText"  # The address field is the shell's only text field.

PAGES = {
    "/a": "<title>Page A</title><a href=/b style='display:block;height:200px;font-size:40px'>to b</a>",
    # A page that starts two downloads by itself, the second a while after the first
    # (page_downloads_ask).
    "/two-files": "<title>Two files</title><script>location.href='/dl/pb-ask-one.zip';"
                  "setTimeout(function(){location.href='/dl/pb-ask-two.zip'},3000)</script>two files",
    "/b": "<title>Page B \U0001F642</title>b",
    "/c": "<title>Page C</title>c",
    "/tap": "<title>Tap</title><button style='position:fixed;left:0;top:0;width:100%;height:200px'"
            " onclick=\"fetch('/tapped')\">tap</button>",
    "/tall": "<title>Tall</title><div style='height:5000px'>tall</div>"
             "<script>onscroll=()=>fetch('/scrolled?y='+Math.round(scrollY))</script>",
    "/busy": "<title>Busy</title><button style='width:100%;height:200px' onclick='for(;;);'>busy</button>",
    "/form": "<title>Form</title><form method=post action=http://blocked.test/>"
             "<button style='width:100%;height:200px'>post</button></form>",
    "/input": "<title>Input</title><form action=/typed><input name=q autofocus "
              "style='width:100%;height:200px;font-size:40px'></form>",
    "/typed": "<title>Typed</title>typed",
    # Tells its scale as it changes, as a pinch changes it.
    "/pinch": "<meta name=viewport content='width=device-width, initial-scale=1'><title>Pinch</title>"
              "<p style='font-size:20px'>Pinch me</p>"
              "<script>visualViewport.onresize = () => fetch('/said?scale=' + visualViewport.scale.toFixed(2))</script>",
    "/words": "<title>Words</title><p style='font-size:40px'>Some words to hold a finger on, " + "words " * 200,
    "/to-blocked": None,
}

# Web APIs whose browser side is Chrome's: each call of the web-features registry
# (core/webfeatures/registry.json). When the browser has no binder for the interface one asks
# for, content kills the asking renderer for a bad message, as DuckDuckGo's search did on
# Android with getInstalledRelatedApps. The registry leaves out the ones that open a system
# picker over the app, since the picker would take the touches meant for the next part.
REGISTRY_APIS = web_features.probes()
# And some whose browser side is content's own, so they aren't in the registry: called so
# that a page using them is seen to keep its renderer too.
CONTENT_APIS = {
    "battery": "navigator.getBattery()",
    "topics": "document.browsingTopics()",
    "sharedStorage": "sharedStorage.set('a', 'b')",
    "adAuction": "navigator.runAdAuction({})",
    "interestGroup": "navigator.joinAdInterestGroup({})",
    "privateToken": "document.hasPrivateToken('https://example.com')",
    "launchQueue": "launchQueue.setConsumer(() => {})",
    "webgpu": "navigator.gpu.requestAdapter()",
    "pressure": "new PressureObserver(() => {}).observe('cpu')",
    "handwriting": "navigator.createHandwritingRecognizer({languages: ['en']})",
    "barcode": "new BarcodeDetector().detect(new ImageData(1, 1))",
    "face": "new FaceDetector().detect(new ImageData(1, 1))",
    "text": "new TextDetector().detect(new ImageData(1, 1))",
}
WEB_APIS = dict([(probe["name"], probe["call"]) for _, probe in REGISTRY_APIS] + list(CONTENT_APIS.items()))

# Calls every one of them when the page loads and again on a tap, which some need, waiting at most
# 2 s for each, then says what each gave; and says again a moment later, so a renderer killed
# after it answered still shows.
PAGES["/apis"] = ("<title>APIs</title><button style='position:fixed;inset:0;width:100%;height:100%'>apis</button><script>"
                  "const calls = {" + ",".join("%s: () => %s" % kv for kv in WEB_APIS.items()) + "};"
                  "const outcome = " + web_features.OUTCOME + ";"
                  "function run(when) {"
                  " const names = Object.keys(calls);"
                  " Promise.all(names.map(name => outcome(calls[name])))"
                  " .then(seen => fetch('/apis-ran?' + when + '&seen=' + encodeURIComponent(JSON.stringify("
                  "Object.fromEntries(names.map((name, i) => [name, seen[i]]))))))"
                  " .then(() => setTimeout(() => fetch('/apis-alive?' + when), 3000)) }"
                  "run('load'); document.querySelector('button').onclick = () => run('tap')</script>")

# A <usermedia> element, as Google's search page shows for its voice search, which Chromium
# 158 has on by default: Blink reads its words from the app's resources as it lays one out,
# and without them the renderer aborted ("Unable to find resource: 47513"). Says a moment
# later that it lives, and whether Blink knows the element.
PAGES["/usermedia"] = ("<title>User media</title><usermedia></usermedia>"
                       "<script>setTimeout(() => fetch('/usermedia-alive?' + typeof HTMLUserMediaElement), 3000)</script>")
# Images with no words of their own, which a screen reader asks Chromium to describe, from
# its accessibility strings; says a moment later that the page lives.
PAGES["/images"] = ("<title>Images</title>" + "".join(
    "<img width=160 height=120 src=\"data:image/svg+xml,%%3Csvg xmlns='http://www.w3.org/2000/svg'%%3E%%3Crect width='160' height='120' fill='%%23%s'/%%3E%%3C/svg%%3E\">" % c
    for c in ("c33", "3c3", "33c")) + "<script>setTimeout(() => fetch('/images-alive'), 3000)</script>")

served = []
# The /own/ files asked for from anywhere but their page.
refused = []
# Each request's host and path, for Links to other sites, which serves two sites.
seen = []
# Each request's user agent.
agents = []
results = []


class Server(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("http://"):
            # Asked as the phone's proxy, as blocking's test hosts are: the address's path.
            asked_for = urllib.parse.urlsplit(self.path)
            self.path = asked_for.path + ("?" + asked_for.query if asked_for.query else "")
        served.append(self.path)
        seen.append(self.headers.get("Host", "").split(":")[0] + self.path)
        agents.append(self.headers.get("User-Agent", ""))
        path = self.path.split("?")[0]
        if path in ("/leave", "/leave-redirect"):
            # Links to other sites (tools/links_page.py): the other site is this server as
            # 127.0.0.1, which isn't localhost's site.
            other = "http://127.0.0.1:%d" % self.server.server_port
            if path == "/leave-redirect":
                self.send_response(302)
                self.send_header("Location", other + "/r9")
                self.end_headers()
                return
            data = links_page.page(other).encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(data)
            return
        if path == "/signin" and self.headers.get("Authorization") != "Basic YWRhOnNlY3JldA==":  # ada:secret
            self.send_response(401)
            self.send_header("WWW-Authenticate", 'Basic realm="Staff only"')
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        if path == "/signin":
            body = "<title>Signed in</title><p>signed in"
            data = body.encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        # A file only for the press page, as a site's own files can be: asked for from
        # anywhere else, with no Referer or another, it is refused.
        if path.startswith("/own/"):
            if not self.headers.get("Referer", "").endswith("/press"):
                refused.append(path)
                self.send_response(403)
                self.send_header("Content-Length", "0")
                self.end_headers()
                return
            path = ("/pic/" if path.endswith(".png") else "/dl/") + path[5:]
        if path.startswith("/dl/"):
            data = b"Project B download check\n"
            self.send_response(200)
            # Its own name's type, as a server's is: the phone's shared Downloads takes a file
            # under a name that fits its type.
            self.send_header("Content-Type", mimetypes.guess_type(path[4:])[0] or "application/octet-stream")
            self.send_header("Content-Disposition", 'attachment; filename="%s"' % path[4:])
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        if path.startswith("/pic/"):
            data = PICTURE
            self.send_response(200)
            self.send_header("Content-Type", "image/png")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        # A big file that comes slowly, a quarter of a megabyte each half second, for a
        # download that runs long enough to be paused, resumed and cancelled.
        if path.startswith("/slow/"):
            chunk = b"x" * 250000
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Disposition", 'attachment; filename="%s"' % path[6:])
            self.send_header("Content-Length", str(len(chunk) * 240))
            self.end_headers()
            try:
                for _ in range(240):
                    self.wfile.write(chunk)
                    self.wfile.flush()
                    time.sleep(0.5)
            except OSError:
                pass  # Cancelled, or the app stopped.
            return
        # A streamed video, core/streams' test streams (docs/video-streams.md): /stream/ as
        # they are, /slowstream/ with each piece held back five seconds, so the stream runs
        # long enough to outlive the app's screen.
        if path.startswith("/stream/") or path.startswith("/slowstream/"):
            name = path.split("/", 2)[2]
            file = os.path.normpath(os.path.join(STREAMS, name))
            if not file.startswith(STREAMS) or not os.path.isfile(file):
                self.send_response(404)
                self.send_header("Content-Length", "0")
                self.end_headers()
                return
            with open(file, "rb") as f:
                data = f.read()
            if path.startswith("/slowstream/") and not file.endswith(".m3u8"):
                time.sleep(5)
            self.send_response(200)
            self.send_header("Content-Type", "application/vnd.apple.mpegurl" if file.endswith(".m3u8") else "application/octet-stream")
            if file.endswith(".m3u8"):
                # A name of its own, so the MP4 keeps it: one that says nothing, as "index",
                # is named after the page instead (docs/daily-driver.md, "Naming downloads").
                self.send_header("Content-Disposition", 'attachment; filename="pb-film.m3u8"')
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            try:
                self.wfile.write(data)
            except OSError:
                pass
            return
        if path == "/to-blocked":
            self.send_response(302)
            self.send_header("Location", "http://blocked.test/")
            self.end_headers()
            return
        body = PAGES.get(path)
        head = "<!doctype html><meta charset=utf-8><meta name=viewport content='width=device-width'>"
        data = (head + (body or "")).encode()
        self.send_response(200 if body else 404)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        # So that back and forward show up here instead of being answered from the cache.
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


def png(width, height):
    """A PNG of a colour ramp, width by height, for a page's picture."""
    rows = b"".join(b"\x00" + bytes(c for x in range(width) for c in (255 * x // width, 120, 255 * y // height))
                   for y in range(height))
    chunk = lambda kind, data: struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b""))


PICTURE = png(96, 64)

# A long press's targets: a link, a picture and a linked picture, each a third of the screen,
# and a field that says what is pasted in it.
PAGES["/press"] = ("<title>Press</title><style>body{margin:0;font-size:32px}a,img{display:block;height:25vh;width:100%}"
                   "a{line-height:25vh}img{object-fit:cover}#p{height:15vh;border:2px solid #888}</style>"
                   "<a href=/own/pb-press-link.txt>The press file</a>"
                   "<img src=/own/pb-press-pic.png alt='A picture'>"
                   "<a href=/b><img src=/pic/pb-press-linked.png alt='A linked picture'></a>"
                   "<div id=p contenteditable>paste here</div><script>"
                   "p.onpaste=e=>{const f=[...e.clipboardData.items].find(i=>i.kind=='file')?.getAsFile();"
                   "fetch('/pasted?type='+(f?f.type:'none')+'&size='+(f?f.size:0))}</script>")

# A page that asks each of its dialogs from a tap, and says what it got back.
# A checked switch and a checked checkbox, for what a screen reader hears of them.
PAGES["/switches"] = ("<title>Switches</title><h1>Switches</h1>"
                      "<button role=switch aria-checked=true aria-label='Ad blocker'>x</button> "
                      "<input type=checkbox aria-label='Box' checked>"
                      # Named three more ways a page names a control, each of which Android 16
                      # once got only as a supplemental description, which TalkBack never says.
                      "<p><label><input type=checkbox role=switch checked> Wrapped</label></p>"
                      "<p><span id=lb>By id</span> <button role=switch aria-checked=false aria-labelledby=lb>y</button></p>"
                      "<p><label for=f>Field</label> <input id=f></p>")
# Pop-ups (docs/site-features.md, Pop-ups): one the page opens on its own as it loads, and one
# a tap opens.
PAGES["/popups"] = ("<title>Popups</title><button id=tap style='display:block;width:100%;height:40vh;font-size:40px'>tap to open</button><script>"
                    "tap.onclick = () => window.open('/popped-tapped');"
                    "setTimeout(() => window.open('/popped-untapped'), 300)</script>")
PAGES["/popped-tapped"] = "<title>Popped</title><p>popped"
PAGES["/popped-untapped"] = "<title>Popped</title><p>popped"
# A page with unsaved work: its beforeunload asks before it goes, once a person has used it.
PAGES["/unsaved"] = ("<title>Unsaved</title><style>button{display:block;width:100%;height:60vh;font-size:40px}</style>"
                     "<button id=w>unsaved work</button><script>"
                     "w.onclick = () => fetch('/said?used=1');"
                     "onbeforeunload = e => e.preventDefault();</script>")
PAGES["/dialogs"] = ("<title>Dialogs</title><style>button{display:block;width:100%;height:30vh;font-size:40px}</style>"
                     "<button id=c>confirm</button><button id=p>prompt</button><button id=al>alert</button><script>"
                     "const say = (what, v) => fetch('/said?' + what + '=' + encodeURIComponent(String(v)));"
                     "c.onclick = () => say('confirm', confirm('Go on?'));"
                     "p.onclick = () => say('prompt', prompt('Name?', ''));"
                     "al.onclick = () => { alert('Hello'); say('alert', 'closed'); };</script>")


# Passkeys (docs/passkeys.md): make saves one for ada, use signs in with whichever the person
# chooses, and each tells the server how it ended.
PAGES["/passkey"] = ("<title>Passkey</title><style>button{display:block;width:100%;height:30vh;font-size:40px}</style>"
                     "<button id=m>make</button><button id=u>use</button><script>"
                     "const say = (what, p) => p.then((v) => v, (e) => 'refused ' + e.name).then((v) => fetch('/said?' + what + '=' + encodeURIComponent(v)));"
                     "m.onclick = () => say('make', navigator.credentials.create({publicKey: {challenge: new Uint8Array(16), rp: {name: 'Passkeys'},"
                     "user: {id: new TextEncoder().encode('ada'), name: 'ada@example.com', displayName: 'Ada'},"
                     "pubKeyCredParams: [{type: 'public-key', alg: -7}], authenticatorSelection: {residentKey: 'required'}}})"
                     ".then((c) => 'made ' + c.response.getTransports().join() + ' ' + c.response.getPublicKeyAlgorithm() + ' ' + (c.response.getPublicKey() ? 'key' : 'nokey')));"
                     "u.onclick = () => say('use', navigator.credentials.get({publicKey: {challenge: new Uint8Array(16)}})"
                     ".then((c) => 'used ' + new TextDecoder().decode(c.response.userHandle)));</script>")


# A sign-in (docs/password-autofill.md): an email and a password, big enough to tap, which
# tells the server what they hold once both hold something. Not at /signin, which is the
# server's own path for a sign-in over HTTP, answered before these pages; and with no "Sign in"
# among its words, which is how screen_parts knows the shell's own sign-in dialog.
PAGES["/login"] = ("<title>A login form</title><style>input{display:block;width:90%;height:12vh;font-size:40px;margin:4vh auto}</style>"
                    "<form onsubmit='event.preventDefault()'><input id=e name=email type=email><input id=p name=password type=password></form><script>"
                    "document.addEventListener('input', () => e.value && p.value && fetch('/said?filled=' + encodeURIComponent(e.value + ' ' + p.value)));</script>")
# The same sign-in, whose form goes through: its button first, clear of the keyboard, and a
# tap on it sends the form and takes the tab on to a page with no sign-in. Its path doesn't
# begin as /login does: the address field would complete /login to it once it was visited.
PAGES["/onward"] = ("<title>A login form that goes on</title><style>input,button{display:block;width:90%;height:9vh;font-size:40px;margin:3vh auto}</style>"
                       "<form onsubmit=\"event.preventDefault(); location.href='/signed-in'\"><button id=go>Go on</button>"
                       "<input id=e name=email type=email><input id=p name=password type=password></form>")
PAGES["/signed-in"] = "<title>Signed in</title><p>signed in"


# A page's camera (docs/media-permissions.md): a tap asks for it, and the page tells the server
# how it ended, the stream's track or the error's name.
PAGES["/camera"] = ("<title>Camera</title><button id=c style='position:fixed;inset:0;width:100%;height:100%;font-size:40px'>camera</button><script>"
                    "c.onclick = () => navigator.mediaDevices.getUserMedia({video: true}).then("
                    "(s) => { s.getTracks().forEach((t) => t.stop()); return 'ok'; }, (e) => e.name)"
                    ".then((v) => fetch('/said?camera=' + encodeURIComponent(v)));</script>")


# A page that fills the screen at a tap, as a video's full screen does, and says when it has
# and how big it is then, in device pixels. Its one colour is the screen's wherever it shows.
# A file input over the whole page, which says which file it got, or that the picker closed.
PAGES["/upload"] = ("<title>Upload</title><input type=file id=f style='position:fixed;inset:0;width:100%;height:100%;opacity:0'><script>"
                    "f.onchange = () => fetch('/said?file=' + encodeURIComponent([...f.files].map(x => x.name + ':' + x.size).join(',')));"
                    "f.oncancel = () => fetch('/said?cancelled');</script>")
PAGES["/full"] = ("<title>Full</title><button id=f style='position:fixed;inset:0;width:100%;height:100%;font-size:40px;border:0;background:#2e9d5b;color:#fff'>full</button><script>"
                  "const size = () => document.fullscreenElement && fetch('/full-size?' + Math.round(visualViewport.width * devicePixelRatio) + 'x' + Math.round(visualViewport.height * devicePixelRatio));"
                  "f.onclick = () => document.documentElement.requestFullscreen().then(() => fetch('/full-on')).then(size);"
                  "visualViewport.onresize = size;"
                  "document.onfullscreenchange = () => document.fullscreenElement || fetch('/full-off');</script>")


# The Downloads page with five files, as the phone drew it, kept as it rendered (its
# styles inlined): with multisampled raster on a phone it left tiles undrawn, its header's
# among them, though Chrome drew it whole.
PAGES["/paint"] = open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "android-paint-page.html")).read().replace("<!doctype html>", "")


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Server)
threading.Thread(target=server.serve_forever, daemon=True).start()
PORT = server.server_port
BASE = "localhost:%d" % PORT


def adb(*args):
    return subprocess.run(ADB + list(args), capture_output=True, text=True, errors="replace").stdout


def shot(name):
    with open("%s/%s.png" % (OUT, name), "wb") as f:
        subprocess.run(ADB + ["exec-out", "screencap", "-p"], stdout=f)


def until(condition, timeout=15):
    """Waits for condition to give something other than None or False, and returns it. When
    time runs out it returns None."""
    deadline = time.time() + timeout
    while True:
        value = condition()
        if value is not None and value is not False:
            return value
        if time.time() > deadline:
            return None
        time.sleep(0.1)


def still(find_node, timeout=20, hold=0.6):
    """Waits for a node to show and then to stay where it is, and returns it, or None. A bar
    over the foot of the page moves as the keyboard comes and goes, and a tap aimed at where a
    chip was a moment ago lands on the keyboard or the page."""
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        node = find_node()
        at = node.get("bounds") if node is not None else None
        if at is not None and at == last:
            return node
        last = at
        time.sleep(hold)
    return None


def asked(path, timeout=15):
    """Waits until the page has asked the local server for a path that starts this way."""
    return until(lambda: any(s.startswith(path) for s in served), timeout)


# What is on the screen, asked from the app. Content gives Android's accessibility tree one
# WebView per page it shows: the shell and the shown tab's page. Settings is a tab of ours, so
# while it is the shown tab its page is "settings" too, as the shell then offers Close Settings.
# A node's attributes are Android's: class, text, content-desc (an aria-label), focused,
# selected, and bounds in device pixels.
PASSKEY_TITLES = ("Create a passkey", "Set a screen lock first", "Already saved")

PARTS = {
    # The shell, or, while a page's dialog or the Custom colour sheet shows, it alone, as a
    # modal dialog hides the rest of the shell from the tree: a page dialog's title, Go's
    # words, ends in " says" or is Sign in. A modal sheet of the shell's own is known by a
    # label only it has: the Custom colour sheet by its Deepest shade, as Settings' frame
    # colours also have a "Custom colour" swatch.
    # A passkey request's card is a page dialog too, titled by its step.
    "shell": lambda labels: "Menu" in labels or "Deepest" in labels or any(l.endswith(" says") or l == "Sign in" or l in PASSKEY_TITLES or l.startswith("Sign in to ") for l in labels),
    "page": lambda labels: "Menu" not in labels,
}
# A WebView's own text is its document's title. Every shell's is "Project B"
# (shell/src/apps/shell/*/index.html) and no page of ours has it, so it tells the shell even
# while the soft keyboard has put the toolbar, and Menu with it, aside; and Settings on a phone
# is titled Settings, its Close Settings being in that toolbar.
SHELL_TITLE = "Project B"


def screen_parts():
    """The parts on screen now, by name. Empty while Android can't read the screen, as during
    an animation, or before content has built its tree."""
    out = adb("exec-out", "uiautomator", "dump", "/dev/tty")
    if "</hierarchy>" not in out:
        return {}
    root = ET.fromstring(out[:out.rindex("</hierarchy>") + len("</hierarchy>")])
    parts = {}
    for view in root.iter("node"):
        # A native shell's root view says so by its class (docs/native-shell.md); its
        # controls carry the same labels a shell page's do.
        if view.get("class") == "dev.projectb.browser.NativeShell":
            parts["shell"] = view
        elif view.get("class") == "android.webkit.WebView":
            labels = {n.get("content-desc") for n in view.iter("node")} | {n.get("text") for n in view.iter("node")}
            name = "shell" if view.get("text") == SHELL_TITLE else next(name for name, test in PARTS.items() if test(labels))
            # The shell under a page in full screen draws nothing, so it reads as a page too:
            # the page is the one with words in it, or failing that more in it, as a page with
            # only a field. Words under the view, not its own: its own text is its document's
            # title, which the shell has too.
            def weight(v):
                inside = list(v.iter("node"))[1:]
                said = {n.get("content-desc") for n in inside} | {n.get("text") for n in inside}
                return (not said <= {None, ""}, len(inside))
            if name in parts and weight(parts[name]) >= weight(view):
                continue
            parts[name] = view
    shell = parts.get("shell")
    if "page" in parts and shell is not None and (parts["page"].get("text") == "Settings" or any(n.get("content-desc") == "Close Settings" for n in shell.iter("node"))):
        parts["settings"] = parts["page"]
    return parts


def close_settings():
    """Closes the settings tab by the shell's close in the address's place, as any tab of ours."""
    press("Close Settings")
    until(lambda: "settings" not in screen_parts())


def find(part, **want):
    """The first node in a part of the screen whose attributes are all as wanted, or None.
    A wanted value is the attribute's text, or a function of it. label is content-desc,
    kind is class."""
    names = {"label": "content-desc", "kind": "class"}
    view = screen_parts().get(part)
    for node in view.iter("node") if view is not None else ():
        if all(v(node.get(names.get(k, k))) if callable(v) else node.get(names.get(k, k)) == v for k, v in want.items()):
            return node
    return None


def first(*finds):
    """The node the first of finds gives, each a function giving a node or None, or None. A node
    with no children is false, so `find(...) or find(...)` passes over the one it found."""
    for f in finds:
        node = f()
        if node is not None:
            return node
    return None


def see(part, timeout=20, **want):
    """Waits for such a node to show, and returns it."""
    node = until(lambda: find(part, **want), timeout)
    if node is None:
        raise TimeoutError("never showed in the %s: %s" % (part, want))
    return node


def box(node):
    """A node's place on the screen: left, top, right, bottom. A node that isn't there ends the
    part as a step whose screen never came, not as the driver's own crash."""
    if node is None:
        raise TimeoutError("a node the step needed isn't on the screen")
    return tuple(map(int, re.findall(r"\d+", node.get("bounds"))))


def middle(node):
    left, top, right, bottom = box(node)
    return (left + right) // 2, (top + bottom) // 2


def tap(point):
    adb("shell", "input", "tap", str(point[0]), str(point[1]))


def finger(point, ms=150):
    """A tap as a finger makes it, down for a moment before it is up. `input tap` is up at
    once, before anything the touch set off has moved what is under it: it filled from a chip
    that a finger missed nearly every time (docs/fragility.md, "A tap on a saved login's chip
    lost")."""
    adb("shell", "input", "swipe", str(point[0]), str(point[1]), str(point[0]), str(point[1]), str(ms))


def press(label):
    """Taps one of the shell's controls, found by its label."""
    tap(middle(see("shell", label=label)))


def menu(label):
    """Taps an item of the Mobile shell's menu sheet, which the ⋯ button opens. A tap the
    shell missed, as it can while it draws again after the GPU process died, is tried once
    more."""
    for _ in range(3):
        press("Menu")
        if until(lambda: find("shell", label=label), 5) is not None:
            break
    # The sheet slides up: an item found while it moves is tapped where it was, which can be
    # another item by then (Find in page opened Bookmarks). Its place twice the same first.
    where = lambda: (lambda n: n.get("bounds") if n is not None else None)(find("shell", label=label))
    last = None
    for _ in range(20):
        now = where()
        if now is not None and now == last:
            break
        last = now
        time.sleep(0.15)
    press(label)


def host(address):
    """What the Mobile shell's address pill shows of an address at rest: its host."""
    return re.sub(r"^https?://(www\.)?", "", address).split("/")[0]


def key(code):
    adb("shell", "input", "keyevent", str(code))


def type_text(text):
    adb("shell", "input", "text", text.replace(" ", "%s"))


def type_in_bar(text):
    """Taps the address field and types, waiting for the field to have the keyboard and
    then to show what was typed."""
    tap(middle(see("shell", kind=FIELD)))
    see("shell", kind=FIELD, focused="true")
    type_text(text)
    see("shell", kind=FIELD, text=lambda shown: shown.lower().startswith(text.lower()))


def go(text):
    type_in_bar(text)
    key(66)


def page_top():
    """A point just inside the top of the shown page, right under the shell's rows."""
    left, top, right, _ = box(see("page"))
    return (left + right) // 2, top + 10


def clear_data():
    """The app's data cleared, then its automatic updates turned off, as a person can in
    Settings: a test build older than the published release found that release at its first
    check and downloaded it mid-run, and its notification and update bar moved what the parts
    read (downloads' tap opened the group's summary; keyboard_bars and page_full_screen read
    the page 170 px short), 2026-11-26. update_on_the_phone and update_notification clear the
    data themselves (update_ready), so they update as a new profile does."""
    adb("shell", "pm", "clear", PACKAGE)
    adb("shell", "run-as", PACKAGE, "sh", "-c",
        "'mkdir -p app_project_b/profile/updates/amarit && echo {\\\"auto_update\\\":false} > app_project_b/profile/updates/amarit/settings.json'")


def launch(block=""):
    adb("shell", "am", "force-stop", PACKAGE)
    # A notification shade left pulled down, by a person or a probe, covers the app and takes
    # every touch: emu-37's whole run failed at its first step so, 2026-11-21.
    adb("shell", "cmd", "statusbar", "collapse")
    # Android takes the app zygote down a few seconds after a force stop. A start inside that window
    # reused the dying zygote once and hung for good, so wait it out. Killing it ourselves is worse:
    # the system never makes another, and no renderer starts.
    until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME"))
    # Tabs come back at start, and one of our pages (Downloads, Settings) has no address field
    # to type in: each part starts fresh, on a web tab, as it expects.
    adb("shell", "rm", "-f", "/data/data/%s/%s/session.json" % (PACKAGE, PROFILE))
    # The first start after the app's data was cleared would open the welcome over the
    # screen (docs/onboarding.md): each part starts past it, as its Skip setup does.
    args = ["shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY, "--ez", "skip-welcome", "true"]
    if block:
        args += ["--es", "block", block]
    adb(*args)
    # Content starts in the background, so the window shows before the shell does.
    see("shell", timeout=60, label="Menu")
    # The shell asked for is the one that loaded: the native one's root view is its own, the
    # web one's a WebView. Setting the wrong pref once left every native part on the web
    # shell, passing while testing the wrong one.
    native = until(lambda: screen_parts().get("shell")).get("class") == "dev.projectb.browser.NativeShell"
    if native != (asked_shell == "mkshell"):
        # A TimeoutError, as a screen that never came: the part fails and the run goes on.
        raise TimeoutError("asked for the %s shell, and the %s one loaded" % (asked_shell or "phone's own", "native" if native else "web"))
    # A run that ended in a kill leaves a notice over the top of the page. It is shell, so
    # it would take the touches meant for the page.
    # The Mobile shell's has Dismiss.
    dismiss = lambda: first(lambda: find("shell", label="Close"), lambda: find("shell", text="Dismiss"))
    notice = dismiss()
    if notice is not None:
        tap(middle(notice))
        until(lambda: dismiss() is None)


def processes():
    """The device's processes as (pid, name)."""
    return [tuple(line.split()) for line in adb("shell", "ps", "-A", "-o", "PID,NAME").splitlines() if len(line.split()) == 2]


def store_rows(query, *args):
    """The rows a query gives on the app's store.db (docs/store.md), none while there is no
    store: the database and its WAL are copied off the device and read here."""
    folder = os.path.join(OUT, "store")
    os.makedirs(folder, exist_ok=True)
    for suffix in ("", "-wal"):
        with open(os.path.join(folder, "store.db" + suffix), "wb") as f:
            subprocess.run(ADB + ["exec-out", "cat", "/data/data/%s/%s/store.db%s" % (PACKAGE, PROFILE, suffix)], stdout=f)
    # The index of the WAL is made again from the copy.
    if os.path.exists(os.path.join(folder, "store.db-shm")):
        os.remove(os.path.join(folder, "store.db-shm"))
    db = sqlite3.connect(os.path.join(folder, "store.db"))
    try:
        return db.execute(query, args).fetchall()
    except sqlite3.Error:
        return []
    finally:
        db.close()


def history():
    """Go's history, each address with its title."""
    return dict(store_rows("SELECT url, title FROM history"))


def visited(url):
    return url in history()


def history_title(url):
    return history().get(url)


def children():
    """The app's child processes as (pid, process name), from what the system knows it bound.
    Only live ones: the system can keep a service's record of a process long gone, such as one
    stopped and killed by a test, across later runs of the app."""
    alive = {pid for pid, _ in processes()}
    out = []
    pending = False
    for line in adb("shell", "dumpsys", "activity", "services", PACKAGE).splitlines():
        if "ServiceRecord{" in line:
            pending = (PACKAGE + "/" in line) and "ProcessService" in line
        elif pending and "app=ProcessRecord{" in line:
            pid, name = line.split("ProcessRecord{")[1].split()[1].split(":", 1)
            if pid in alive:
                out.append((pid, name.split("/")[0]))
            pending = False
    return out


def go_library_mappings(pid):
    """How many executable mappings of libcore.so a process has. Android either unpacks a library
    to a file or maps it straight from the APK, where it shows as a stretch of the APK."""
    start, end = go_library_span()
    count = 0
    for line in adb("shell", "cat", "/proc/%s/maps" % pid).splitlines():
        parts = line.split()
        if len(parts) < 6 or "r-xp" not in parts[1] or PACKAGE + "-" not in parts[5]:
            continue
        if parts[5].endswith("/libcore.so"):
            count += 1
        elif parts[5].endswith("base.apk") and start <= int(parts[2], 16) < end:
            count += 1
    return count


def go_library_span():
    """Where lib/arm64-v8a/libcore.so sits in the APK, from the zip's own directory."""
    with zipfile.ZipFile(APK) as z:
        info = z.getinfo("lib/arm64-v8a/libcore.so")
    # The stored data begins just after the local header, a few hundred bytes at most.
    return info.header_offset, info.header_offset + 4096 + info.file_size


def check(name, ok, detail=""):
    results.append((name, bool(ok), detail))
    print("%s  %s  %s" % ("ok  " if ok else "FAIL", name, detail), flush=True)


def status(url, word):
    """Waits for the address pill to show url's host, then says whether the shell marks it
    with this word: Blocked, Failed or Crashed."""
    see("shell", kind=FIELD, text=host(url))
    return until(lambda: find("shell", text=word) is not None)


def alive():
    return PACKAGE in [name for _, name in processes()]


def turn(rotation):
    """Turns the phone and waits for the page to have the new shape."""
    adb("shell", "settings", "put", "system", "accelerometer_rotation", "0")
    adb("shell", "settings", "put", "system", "user_rotation", str(rotation))

    def turned():
        page = find("page")
        if page is None:
            return False
        left, top, right, bottom = box(page)
        return (right - left > bottom - top) == (rotation == 1)

    return until(turned)


def url(path):
    return "http://%s%s" % (BASE, path)


def browsing():
    """Pages, tabs, settings, touches and the gate, in one sitting."""
    launch(block="blocked.test")
    shot("01-opens-empty")

    # Type a URL.
    go(BASE + "/a")
    check("typing a URL loads it", asked("/a"))
    check("the visit reached Go's history", until(lambda: visited(url("/a"))))
    # Chromium takes its version from the app's on Android unless the app hands it its own
    # (docs/fragility.md, "A Dev build that never started"): sites were told Chrome/0.0.0.0.
    major = open(os.path.join(ROOT, "CHROMIUM_VERSION")).read().split(".")[0]
    check("pages are told Chromium's own version, Chrome/%s" % major, agents and "Chrome/%s." % major in agents[-1], agents[-1:])
    shot("02-page-a")

    # A tap on the page belongs to the page.
    tap(middle(see("page", label="to b")))
    check("a tap on a link in the page reaches the page", asked("/b"))
    shot("03-link-tapped")

    # Back, forward, reload through the shell's menu sheet.
    served.clear()
    menu("Back")
    check("back returns to the page before", asked("/a"), str(served))
    served.clear()
    menu("Forward")
    check("forward returns to the page after", asked("/b"), str(served))
    served.clear()
    menu("Reload")
    check("reload asks the server again", asked("/b"), str(served))
    check("an emoji in a title reaches Go whole", until(lambda: history_title(url("/b")) == "Page B \U0001F642"),
          repr(history_title(url("/b"))))

    # Tabs: New Tab is in the menu, the count opens the switcher, a card per tab. A new tab
    # shows the new tab page with the keyboard down (docs/daily-driver.md, A new tab).
    menu("New Tab")
    see("shell", text="All bookmarks")
    check("a new tab shows the new tab page, the keyboard down", find("shell", kind=FIELD, focused="true") is None)
    go(BASE + "/c")
    check("New Tab in the menu opens a tab", asked("/c"))
    press("Tabs")
    check("the count opens the switcher, a card per tab", until(lambda: find("shell", label="Close Page C")) is not None and
          find("shell", label="Close Page B \U0001F642") is not None)
    shot("06b-switcher")
    served.clear()
    tap(middle(see("shell", text="Page B \U0001F642")))
    check("a tap on a card shows that tab", until(lambda: find("page", text="b")) is not None and find("shell", label="Close Page C") is None)

    # Settings is a tab of ours: its title and a close where the address goes.
    menu("Settings")
    check("the settings button opens settings as a tab, its title where the address goes",
          until(lambda: find("settings", text=SETTINGS_LIST)) is not None and find("shell", text="Settings") is not None and
          find("shell", label="Address", kind=FIELD) is None)
    shot("06c-settings")
    menu("Settings")
    check("settings again shows the same tab", until(lambda: find("settings", text=SETTINGS_LIST)) is not None)
    close_settings()
    check("its close closes the settings tab", "settings" not in screen_parts())

    # Suggestions come from history, shown over the page.
    go(BASE + "/c")
    asked("/c")
    type_in_bar("page")
    shot("07-suggestions")
    served.clear()
    tap(middle(see("shell", text=lambda shown: shown.startswith("Page A"))))
    check("a tap on a suggestion loads it", asked("/a"), str(served))
    shot("08-suggestion-tapped")

    # The phone's back key follows the same path as the shell's button.
    served.clear()
    key(4)
    check("the phone's back key goes back", asked("/c"), str(served))
    shot("09-back-key")

    # Touch routing: a tap and a scroll in the page area go to the page, not the shell above it.
    go(BASE + "/tap")
    asked("/tap")
    served.clear()
    tap(page_top())
    check("a tap just below the shell's rows reaches the page", asked("/tapped"), str(served))
    go(BASE + "/tall")
    asked("/tall")
    served.clear()
    left, top, right, bottom = box(see("page"))
    across, high = (left + right) // 2, bottom - top
    adb("shell", "input", "swipe", str(across), str(top + high * 3 // 4), str(across), str(top + high // 4), "300")
    check("a swipe scrolls the page under the shell", asked("/scrolled"), str(served[:3]))
    shot("10-scrolled")
    check("scrolling down, the shell's bars give the page their room", until(lambda: box(see("page"))[3] > bottom + 100) is not None,
          "%d then %d" % (bottom, box(see("page"))[3]))
    shot("10a-bars-away")
    adb("shell", "input", "swipe", str(across), str(top + high // 4), str(across), str(top + high * 3 // 4), "300")
    check("scrolling back up brings them back", until(lambda: box(see("page"))[3] == bottom) is not None, "%d then %d" % (bottom, box(see("page"))[3]))

    # Typing into a page goes through the keyboard to the page, not the shell.
    go(BASE + "/input")
    asked("/input")
    served.clear()
    tap(middle(see("page", kind=FIELD)))
    see("page", kind=FIELD, focused="true")
    type_text("hello")
    key(66)
    # Android 16's keyboard capitalises the first letter, as it would for a person.
    check("the keyboard types into a page", until(lambda: any("q=hello" in s.lower() for s in served)), str(served))
    shot("10b-typed-into-page")

    # Holding a finger on text starts a selection, and the app lives through it.
    go(BASE + "/words")
    asked("/words")
    left, top, _, _ = box(see("page", text=lambda shown: shown.startswith("Some words")))
    adb("shell", "input", "swipe", str(left + 100), str(top + 30), str(left + 100), str(top + 30), "900")
    check("selecting text in a page doesn't kill the app", alive())
    shot("10c-text-selected")
    key(4)

    # The phone turns: the page follows, and a tap in the new layout still lands.
    go(BASE + "/tap")
    asked("/tap")
    check("the page follows the phone into landscape", turn(1))
    shot("10d-landscape")
    served.clear()
    tap(page_top())
    check("a tap lands in the page in landscape", asked("/tapped"), str(served))
    turn(0)

    # A gate says no: for a typed URL, for a redirect and for a form's POST.
    go("blocked.test")
    check("a blocked host is marked Blocked and never joins history",
          status("https://blocked.test/", "Blocked") and not visited("https://blocked.test/"))
    shot("11-blocked")
    go(BASE + "/to-blocked")
    check("a redirect to a blocked host is stopped", status("http://blocked.test/", "Blocked") and not visited("http://blocked.test/"))
    shot("12-redirect-blocked")
    go(BASE + "/form")
    asked("/form")
    tap(page_top())
    check("a form posted to a blocked host is stopped", status("http://blocked.test/", "Blocked"))
    shot("13-post-to-blocked-host")

    # A load that fails stays out of history.
    go("https://nothing.invalid/")
    check("a failed load is marked Failed and never joins history",
          status("https://nothing.invalid/", "Failed") and not visited("https://nothing.invalid/"))
    shot("14-failed")


def long_press(point):
    adb("shell", "input", "swipe", str(point[0]), str(point[1]), str(point[0]), str(point[1]), "1000")


def small_targets():
    """The shell's controls on screen smaller than 48dp either way, Android's least for a
    finger, by their labels."""
    density = int(re.findall(r"\d+", adb("shell", "wm", "density"))[-1]) / 160
    view = screen_parts().get("shell")
    small = []
    for node in view.iter("node") if view is not None else ():
        if node.get("clickable") != "true":
            continue
        left, top, right, bottom = box(node)
        if min(right - left, bottom - top) < 48 * density - 2:
            small.append("%s %dx%ddp" % (node.get("content-desc") or node.get("text") or node.get("class"),
                                         (right - left) / density, (bottom - top) / density))
    return small


def mobile_shell():
    """The Mobile shell's own: the back key closes each of its sheets; a long press on a link
    opens its sheet, whose In group gathers the tabs in the strip behind the page being
    read; and each screen in light and dark, for the owner."""
    launch()
    go(BASE + "/a")
    asked("/a")
    small = small_targets()
    check("every control of the toolbar is 48dp or more each way", not small, ", ".join(small))
    press("Tabs")
    see("shell", label="Close Page A")
    small = small_targets()
    check("every control of the switcher is 48dp or more each way", not small, ", ".join(small))
    key(4)
    check("the back key closes the switcher", bool(until(lambda: find("shell", label="Close Page A") is None)))
    press("Menu")
    see("shell", label="Settings")
    key(4)
    check("the back key closes the menu", bool(until(lambda: find("shell", label="Settings") is None)))
    tap(middle(see("shell", kind=FIELD)))
    see("shell", kind=FIELD, focused="true")
    key(4)  # The keyboard goes first.
    key(4)
    check("the back key ends editing the address", bool(until(lambda: find("shell", kind=FIELD, focused="true") is None)))

    long_press(middle(see("page", label="to b")))
    item = see_named("shell", "In group")
    check("a long press on a link opens the shell's sheet, with In group", item is not None)
    shot("20-link-menu")
    key(4)
    check("the back key closes it", bool(until(lambda: named("shell", "In group") is None)))
    served.clear()
    long_press(middle(see("page", label="to b")))
    tap(middle(see_named("shell", "In group")))
    check("In group opens the link behind the page, and the strip shows the group",
          asked("/b") and until(lambda: find("shell", label=lambda l: (l or "").startswith("Group "))) is not None
          and find("page", label="to b") is not None)
    shot("21-strip")
    small = small_targets()
    check("every control of the group strip is 48dp or more each way", not small, ", ".join(small))
    tap(middle(see("shell", label="Go to Page B \U0001F642")))
    check("a tap on a favicon in the strip shows that tab", until(lambda: find("page", text="b")) is not None)

    # Each screen in light and dark, the theme chosen in settings.
    for scheme in ("light", "dark"):
        menu("Settings")
        settings_section("Appearance", "Theme")
        tap(middle(see("settings", text=scheme.capitalize())))
        close_settings()
        time.sleep(1)
        shot("30-page-" + scheme)
        press("Tabs")
        see("shell", label="New Tab")
        shot("31-switcher-" + scheme)
        key(4)
        press("Menu")
        see("shell", label="Settings")
        shot("32-menu-" + scheme)
        key(4)
        tap(middle(see("shell", kind="android.widget.Button", label=lambda l: (l or "").startswith("Group "))))
        see("shell", text="Ungroup")
        shot("33-group-" + scheme)
        key(4)
        type_in_bar("page")
        see("shell", text=lambda shown: (shown or "").startswith("Page"))
        shot("34-suggestions-" + scheme)
        key(4)
        key(4)
    menu("Settings")
    settings_section("Appearance", "Theme")
    tap(middle(see("settings", text="System")))
    close_settings()
    # The active favicon in the strip closes its tab at a tap; the group keeps its last tab, so
    # the strip stays where it is (docs/groups.md).
    closing = see("shell", label=lambda l: (l or "").startswith("Close Page"), kind="android.widget.Button")
    gone = closing.get("content-desc")
    tap(middle(closing))
    check("a tap on the active favicon in the strip closes its tab, and the group stays with its last tab",
          until(lambda: find("shell", label=gone) is None) is not None
          and find("shell", label=lambda l: (l or "").startswith("Group ")) is not None, gone)


def custom_colour():
    """The Mobile shell's group sheet on a phone: gulal's seven hues by name, then a +, which
    opens our Custom colour sheet, not Android's picker; a shade shows at once, and the back
    key closes the sheet keeping it, the group's sheet still open (docs/design-system.md, On
    a phone)."""
    launch()
    go(BASE + "/a")
    asked("/a")
    long_press(middle(see("page", label="to b")))
    tap(middle(see_named("shell", "In group")))
    tap(middle(see("shell", kind="android.widget.Button", label=lambda l: (l or "").startswith("Group "))))
    see("shell", text="Ungroup")
    names = ("Rose", "Teal", "Olive", "Indigo", "Green", "Blue", "Purple")
    check("the group's sheet offers the seven hues by name", all(find("shell", label=n) is not None for n in names),
          ", ".join(n for n in names if find("shell", label=n) is None))
    shot("40-group-sheet")
    tap(middle(see("shell", label="Custom colour")))
    check("its + opens the Custom colour sheet, with Hue, five shades and Hex",
          until(lambda: named("shell", "Hue")) is not None and named("shell", "Deepest") is not None and named("shell", "Hex") is not None,
          ", ".join(n for n in ("Hue", "Deepest", "Hex") if named("shell", n) is None))
    small = small_targets()
    check("every control of the Custom colour sheet is 48dp or more each way", not small, ", ".join(small))
    shot("41-custom-colour")
    tap(middle(see("shell", label="Deep")))
    time.sleep(0.5)
    key(4)
    check("the back key closes it, and the group's sheet stays",
          until(lambda: find("shell", label="Hue") is None) is not None and find("shell", text="Ungroup") is not None)
    shot("42-group-custom")
    key(4)
    check("and then closes the group's sheet", until(lambda: find("shell", text="Ungroup") is None) is not None)


def profile_file(name, data):
    """Writes one of the profile's files, while the app is stopped, as the app's own: the next
    start reads it."""
    adb("shell", "am", "force-stop", PACKAGE)
    folder = "/data/data/%s/%s" % (PACKAGE, PROFILE)
    if name == "bookmarks.json":
        # The store reads bookmarks.json only as it makes its tables, at the first start
        # without a store.db (docs/store.md): the store goes, history with it, so it does.
        adb("shell", "rm", "-f", folder + "/store.db", folder + "/store.db-wal", folder + "/store.db-shm")
    local = os.path.join(OUT, name)
    with open(local, "w") as f:
        json.dump(data, f)
    adb("shell", "mkdir", "-p", folder)
    adb("push", local, folder + "/" + name)
    owner = adb("shell", "stat", "-c", "%u:%g", "/data/data/" + PACKAGE).strip()
    adb("shell", "chown", "-R", owner, "/data/data/%s/app_project_b" % PACKAGE)
    # And its SELinux label, the app's categories: a push over a file kept the label of the
    # install that made it, and after an uninstall and install (an upgrade check's) the new
    # app was denied its own bookmarks.json and prefs.json, with no table made and nothing
    # bookmarked, until a reboot relabelled them (2026-11-23). The label is the app's data
    # folder's, which the system keeps right. restorecon looks the categories up instead, and
    # on emu-16k it gave an earlier install's: the app was denied the files it was given, and
    # bookmark_star's folders never came (2026-11-26). ls -Z, not stat -c %C: Android 10's
    # stat prints "?" for it, and chcon to that left the folder unlabeled and the app aborting.
    label = adb("shell", "ls", "-Zd", "/data/data/" + PACKAGE).split()[0]
    if not label.startswith("u:object_r:"):
        raise RuntimeError("the app's data folder has no label to copy: " + label)
    adb("shell", "chcon", "-R", label, "/data/data/%s/app_project_b" % PACKAGE)


def profile_json(name):
    """One of the profile's JSON files, or {} while there is none."""
    out = adb("shell", "cat", "/data/data/%s/%s/%s" % (PACKAGE, PROFILE, name))
    try:
        return json.loads(out)
    except ValueError:
        return {}


# The phone shell use_shell asked for, which launch checks loaded: '' for the phone's own.
asked_shell = ""


def use_shell(shell):
    """Makes shell the phone's at the next start, '' for its own. A phone-sized window loads
    the phoneShell pref, not shell, which is the wide window's (docs/large-screens.md)."""
    global asked_shell
    asked_shell = shell
    prefs = profile_json("prefs.json")
    prefs.pop("phoneShell", None)
    if shell:
        prefs["phoneShell"] = shell
    profile_file("prefs.json", prefs)


def named(part, name, **want):
    """The first node in a part whose label or text is name, as a web shell's button can be
    named by its words alone and a native one by its label."""
    return first(lambda: find(part, label=name, **want), lambda: find(part, text=name, **want))


def see_named(part, name, timeout=20):
    """Waits for a node named name to show, by its label or its text, and returns it."""
    node = until(lambda: named(part, name), timeout)
    if node is None:
        raise TimeoutError("never showed in the %s: %s" % (part, name))
    return node


def bookmark_star():
    """The phone's star, in both Mobile shells, the web one and the native one
    (docs/bookmark-folders.md): the menu's Bookmark bookmarks the page into the folder last
    saved in and shows its sheet; Enter in the name saves it with the folder chosen; the back
    key closes it as it was; on a bookmarked page the star is on, and Remove takes it away."""
    for shell in ("mwshell", "mkshell"):
        # Two folders, the second in the first, and the first the one last saved in.
        profile_file("bookmarks.json", {"version": 2, "last": "f1", "entries": [
            {"id": "f1", "parent": "", "kind": "folder", "title": "Reading", "added": 1, "modified": 1, "place": "a"},
            {"id": "f2", "parent": "f1", "kind": "folder", "title": "Recipes", "added": 1, "modified": 1, "place": "a"}]})
        use_shell(shell)
        try:
            launch()
            go(BASE + "/a")
            see("page", label="to b")
            menu("Bookmark")
            see("shell", text="Bookmarked")
            saved = lambda: [{"parent": parent, "title": title} for parent, title in
                             store_rows("SELECT parent, title FROM bookmarks WHERE url = ?", "http://" + BASE + "/a")]
            # The folder chosen, by its name; checked only in the native shell's tree, as the
            # tree a web page gives Android without a screen reader on carries no checked state.
            chosen = {"checked": "true"} if shell == "mkshell" else {"kind": "android.widget.RadioButton"}
            check(shell + ": the menu's Bookmark bookmarks the page into the folder last saved in, and shows its sheet",
                  until(lambda: [e["parent"] for e in saved()] == ["f1"]) is not None
                  and until(lambda: named("shell", "Reading", **chosen)) is not None, saved())
            shot("40-bookmark-" + shell)
            small = small_targets()
            check(shell + ": every control of the bookmark sheet is 48dp or more each way", not small, ", ".join(small))
            key(4)
            check(shell + ": the back key closes it, the bookmark kept",
                  until(lambda: find("shell", text="Bookmarked") is None) is not None and len(saved()) == 1)

            menu("Edit bookmark")
            see("shell", text="Edit bookmark")
            tap(middle(see_named("shell", "Bookmarks")))
            name = see("shell", kind=FIELD, text="Page A")
            tap(middle(name))
            see("shell", kind=FIELD, text="Page A", focused="true")
            key(123)  # The end of the name.
            type_text(" kept")
            see("shell", kind=FIELD, text="Page A kept")
            key(66)
            check(shell + ": on a bookmarked page the star is on, and Enter in the name saves the name and the folder chosen",
                  until(lambda: [(e["title"], e["parent"]) for e in saved()] == [("Page A kept", "")]) is not None
                  and until(lambda: find("shell", text="Edit bookmark") is None) is not None, saved())

            menu("Edit bookmark")
            tap(middle(see_named("shell", "Remove")))
            check(shell + ": Remove takes the bookmark away", until(lambda: not saved()) is not None, saved())
            press("Menu")
            check(shell + ": and the star is off again", see("shell", label="Bookmark") is not None)
            key(4)
        except TimeoutError:
            stopped_here("bookmark_star", "bookmark_star-" + shell)
            raise
        finally:
            use_shell("")


def settings_ahead():
    """Settings made ahead as the menu opens, in both Mobile shells, the web one and the native
    one (docs/windows.md): the menu opening starts the settings page out of sight, no tab to
    the person, neither counted, nor a card in the switcher, nor on screen; the menu's
    Settings then shows that page, already started."""
    for shell in ("mwshell", "mkshell"):
        use_shell(shell)
        try:
            launch()
            go(BASE + "/a")
            see("page", label="to b")
            timeline_marks = lambda: adb("logcat", "-d", "-s", "ProjectB")
            adb("logcat", "-c")
            press("Menu")
            see("shell", label="Settings")
            check(shell + ": the menu opening starts the settings page out of sight, none on screen",
                  until(lambda: "settings.prepare" in timeline_marks() and "settings.ready" in timeline_marks()) is not None
                  and "settings" not in screen_parts())
            key(4)
            until(lambda: find("shell", label="Settings") is None)
            press("Tabs")
            check(shell + ": the switcher has a card for the page's tab and none for settings",
                  until(lambda: find("shell", label="Close Page A")) is not None and find("shell", label="Close Settings") is None)
            key(4)
            until(lambda: find("shell", label="Close Page A") is None)
            menu("Settings")
            check(shell + ": the menu's Settings shows that page, its tab the one made ahead",
                  until(lambda: find("settings", text=SETTINGS_LIST)) is not None and "settings.open.prepared_ready" in timeline_marks())
            close_settings()
            check(shell + ": and its close closes it", "settings" not in screen_parts())
        except TimeoutError:
            stopped_here("settings_ahead", "settings_ahead-" + shell)
            raise
        finally:
            use_shell("")


def in_front():
    """Whether the app's window is the one Android shows."""
    return PACKAGE in adb("shell", "dumpsys", "window", "displays").split("mCurrentFocus", 1)[-1].split("\n", 1)[0]


def back_key_chain():
    """The phone's back key goes back in the page, then closes a tab with no history, the tab
    used before it showing, down to the last tab, whose closing closes the app."""
    launch()
    go(BASE + "/a")
    asked("/a")
    tap(middle(see("page", label="to b")))
    asked("/b")
    menu("New Tab")
    see("shell", text="All bookmarks")
    go(BASE + "/c")
    asked("/c")
    served.clear()
    # Back until the tab is gone: the first may only put the keyboard away, and the new tab's
    # blank page may be in its history.
    for _ in range(3):
        if until(lambda: find("page", text="b"), 2) is not None:
            break
        key(4)
    check("back on a tab with no history closes it, and the tab used before it shows",
          until(lambda: find("page", text="b")) is not None)
    served.clear()
    key(4)
    check("back in that tab goes back in its page", asked("/a"), str(served))
    key(4)
    check("back on the last tab, with no history, closes it, and so the app", until(lambda: not in_front()) is not None)


def leave():
    """Closing a tab whose page has unsaved work asks first, as Chrome does: its card's close
    in the tab switcher shows "Leave site?", Cancel keeps the tab, and Leave closes it. The
    close went straight to the engine, so the tab and its work went without a word."""
    launch()
    go(BASE + "/a")
    asked("/a")
    menu("New Tab")
    see("shell", text="All bookmarks")
    go(BASE + "/unsaved")
    asked("/unsaved")
    # A page's beforeunload asks only once a person has used it.
    tap(middle(see("page", text="unsaved work")))
    asked("/said?used=1")
    cards = lambda: [n.get("content-desc") for n in screen_parts().get("shell", ET.Element("x")).iter("node")
                     if (n.get("content-desc") or "").startswith("Close ")]
    press("Tabs")
    until(cards, 5)
    before = cards()
    press("Close Unsaved")
    shown = until(lambda: find("shell", text="Leave site?"), 10)
    shot("50-leave-site")
    check("closing a tab whose page has unsaved work asks Leave site? first", shown is not None, before)
    if shown is not None:
        tap(middle(see("shell", text="Cancel")))
        until(lambda: find("shell", text="Leave site?") is None, 10)
    time.sleep(1)
    if not cards():
        press("Tabs")
        until(cards, 5)
    check("and Cancel keeps the tab", shown is not None and "Close Unsaved" in cards(), (before, cards()))
    press("Close Unsaved")
    left = until(lambda: find("shell", text="Leave site?"), 10) is not None
    if left:
        tap(middle(see("shell", text="Leave")))
    if not cards():
        press("Tabs")
    check("and Leave closes it", left and until(lambda: cards() and "Close Unsaved" not in cards(), 10) is not None, cards())
    key(4)
    go(BASE + "/a")
    asked("/a")


def page_dialogs():
    """A page's confirm, prompt and alert show in the shell, and its answer reaches the page;
    the back key answers no; a site's sign-in takes a username and password."""
    launch()
    go(BASE + "/dialogs")
    asked("/dialogs")
    served.clear()
    tap(middle(see("page", text="confirm")))
    see("shell", text="Go on?")
    shot("23-confirm")
    tap(middle(see("shell", text="OK")))
    check("a confirm shows in the shell, and OK answers the page yes", asked("/said?confirm=true"), str(served))
    tap(middle(see("page", text="confirm")))
    see("shell", text="Go on?")
    key(4)
    check("the back key answers it no", asked("/said?confirm=false"), str(served))
    tap(middle(see("page", text="prompt")))
    see("shell", kind=FIELD, focused="true", label=lambda l: l != "Address")
    type_text("Ada")
    tap(middle(see("shell", text="OK")))
    check("a prompt takes text from the keyboard, already in its field", asked("/said?prompt=Ada"), str(served))
    tap(middle(see("page", text="alert")))
    see("shell", text="Hello")
    tap(middle(see("shell", text="OK")))
    check("an alert shows and OK lets the page go on", asked("/said?alert=closed"), str(served))
    go(BASE + "/signin")
    see("shell", text="Username")
    shot("24-sign-in")
    type_text("ada")
    tap(middle(see("shell", password="true")))
    type_text("secret")
    tap(middle(see("shell", text="Sign in", kind=lambda k: k != FIELD, clickable="true")))
    check("a site's sign-in takes a username and password, and the page loads",
          until(lambda: find("page", text="signed in")) is not None)
    # Leave a plain page, as the next part's start restores this tab, and a signed-in page
    # restored asks for the sign-in again.
    go(BASE + "/a")
    asked("/a")
def in_front_now():
    """The window Android shows in front: its package and activity."""
    return adb("shell", "dumpsys", "window", "displays").split("mCurrentFocus", 1)[-1].split("\n", 1)[0]


def in_recents():
    """Whether the app has a task in the recent apps. Android 10 writes a task's app as
    A=<package>, later ones as A=<uid>:<package>."""
    said = adb("shell", "dumpsys", "activity", "recents")
    return re.search(r"Recent #\d+: Task\w*\{[^\n]*A=(\d+:)?" + re.escape(PACKAGE) + r"[ }]", said) is not None


def recent_cards():
    """The cards in the recent apps on screen, each a (node, title) pair, the title None where
    the launcher names none, as Android 10's. Empty while the screen can't be read."""
    out = adb("exec-out", "uiautomator", "dump", "/dev/tty")
    if "</hierarchy>" not in out:
        return []
    root = ET.fromstring(out[:out.rindex("</hierarchy>") + len("</hierarchy>")])
    cards = []
    for node in root.iter("node"):
        rid = node.get("resource-id", "")
        if rid.endswith(":id/task_view_single") or (rid.endswith(":id/snapshot") and not cards_hold(cards, node)):
            titles = [t.get("text") or t.get("content-desc") for t in node.iter("node") if t.get("resource-id", "").endswith(":id/icon_title")]
            cards.append((node, titles[0] if titles else None))
    return cards


def cards_hold(cards, node):
    """Whether node lies inside one of the cards already found."""
    l, t, r, b = box(node)
    return any(cl <= l and ct <= t and r <= cr and b <= cb for cl, ct, cr, cb in (box(c) for c, _ in cards))


def swipe_away():
    """Swipes the app away from the recent apps, as a person closes it, and makes sure it went.
    Android 17's launcher may open the recent apps at their oldest end, with another app's card
    in the middle, a slow drag there springs back, and with other emulators busy its dismissal
    may be dropped: the swipe took another app's card, or none, and the app ran on in front of
    nothing, so a check of what it does once closed passed or failed for an app that was never
    closed. The recent apps are opened from Home, so that the app's card is its picture, not
    the app still drawing under it, whose dismissal Android 17 dropped again and again; its
    card is found by the app's name where the launcher names cards, else it is the one in the
    middle, as on Android 10, whose recent apps open on the app; it is flicked, and the whole
    is done again while the app is still among the recent apps."""
    for _ in range(4):
        adb("shell", "input", "keyevent", "KEYCODE_HOME")
        time.sleep(1)
        adb("shell", "input", "keyevent", "KEYCODE_APP_SWITCH")
        card = None
        for _ in range(12):
            # The recent apps still opening or scrolling: the cards move between two reads.
            cards = until(lambda: recent_cards(), 10) or []
            time.sleep(1)
            again = recent_cards()
            if not cards or [box(c) for c, _ in cards] != [box(c) for c, _ in again]:
                continue
            named = [c for c, title in again if title == APP_NAME]
            if named:
                card = named[0]
                break
            if all(title is None for _, title in again):
                card = next((c for c, _ in again if box(c)[0] < 540 < box(c)[2]), None)
                break
            # The newest card is at the end away from Clear all.
            adb("shell", "input", "swipe", "900", "1100", "100", "1100", "150")
        if card is None:
            continue
        x, y = middle(card)
        adb("shell", "input", "swipe", str(x), str(y), str(x), "50", "120")
        if until(lambda: not in_recents(), 6):
            return
    raise TimeoutError("the app's card never left the recent apps")


def download_outlives_screen():
    """A download outlives the app's screen, as Chrome's does: swiped away from the recent
    apps mid-download, the app runs on with no screen and the file still arrives; opened
    again meanwhile, it shows its tabs as they were; once the download is done, with no
    screen, the app ends (docs/daily-driver.md, Downloads). Swiping it away ended the
    process at once, and the download with it."""
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    launch()
    name = "pb-outlive-%d.bin" % int(time.time())
    go(BASE + "/a")
    asked("/a")
    go(BASE + "/slow/" + name)
    until(lambda: (notification_of(name) or "") != "", 15)
    swipe_away()
    time.sleep(5)
    pid = adb("shell", "pidof", PACKAGE).strip()
    check("swiped away mid-download, the app runs on with no screen", bool(pid) and "BrowserActivity" not in in_front_now(), in_front_now())
    adb("shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY)
    back = until(lambda: find("page", text="to b"), 20)
    check("opened again mid-download, it shows its tab as it was, in the same process",
          back is not None and adb("shell", "pidof", PACKAGE).strip() == pid, adb("shell", "pidof", PACKAGE).strip() + " was " + pid)
    swipe_away()
    stored = until(lambda: name in adb("shell", "ls", "/sdcard/Download/"), 150)
    check("swiped away again, the file still reaches the phone's Downloads", stored is not None)
    ended = until(lambda: not adb("shell", "pidof", PACKAGE).strip(), 30)
    check("and with the download done and no screen, the app ends", ended is not None)
    adb("shell", "rm", "-f", "/sdcard/Download/" + name)


def stream_download():
    """A streamed video saves as one MP4 on a phone (docs/video-streams.md): a playlist's
    address, gone to, lands in the phone's Downloads as an MP4 under the playlist's name,
    with picture and sound and its stream's length, and its notification says Downloaded;
    the playlist itself never reaches the phone's Downloads. And a stream outlives the app's
    screen as a running download does: swiped away while its pieces come, the app runs on
    with no screen, the MP4 still arrives, and then the app ends."""
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    files = lambda: adb("shell", "ls", "-1", "/sdcard/Download/").splitlines()
    for stale in [f for f in files() if f.startswith("pb-film")]:
        adb("shell", "rm", "-f", shlex.quote("/sdcard/Download/" + stale))
    launch()
    go(BASE + "/a")
    asked("/a")
    go(BASE + "/stream/ts/index.m3u8")
    check("a playlist's address keeps the page where it was", until(lambda: find("page", text="to b")) is not None)
    stored = until(lambda: "pb-film.mp4" in files(), 40)
    check("the stream reaches the phone's Downloads as one MP4, under its playlist's name", stored is not None, files()[-8:])
    check("and its playlist doesn't", not any(f.startswith("pb-film") and not f.endswith(".mp4") for f in files()), files()[-8:])
    said = until(lambda: (notification_of("pb-film.mp4") or "").startswith("Downloaded") or None, 15)
    check("its notification says it is downloaded", said is not None, str(notification_of("pb-film.mp4")))
    local = os.path.join(OUT, "stream-pb-film.mp4")
    adb("pull", "/sdcard/Download/pb-film.mp4", local)
    got = mp4_length.read(local) if os.path.isfile(local) else {}
    check("the MP4 has picture and sound, its index first, and is as long as its stream, 4 seconds",
          got.get("tracks") == ["vide", "soun"] and got.get("index_first") and abs(got.get("seconds", 0) - 4.0) < 0.15, got)
    menu("Downloads")
    row = until(lambda: find("page", text=lambda t: (t or "").startswith("pb-film.mp4")))
    check("the Downloads page lists it by the MP4's name", row is not None)
    press("Close Downloads")
    adb("shell", "rm", "-f", "/sdcard/Download/pb-film.mp4")

    # Its pieces coming slowly, about 20 seconds in all, longer than a swipe away takes: swiped
    # away meanwhile.
    launch()
    go(BASE + "/a")
    asked("/a")
    go(BASE + "/slowstream/fmp4/index.m3u8")
    # Its name may be numbered, as "pb-film (1).mp4", when Android still holds an earlier one.
    until(lambda: "pb-film" in adb("shell", "dumpsys", "notification", "--noredact"), 15)
    swipe_away()
    time.sleep(3)
    pid = adb("shell", "pidof", PACKAGE).strip()
    check("swiped away while a stream's pieces come, the app runs on with no screen", bool(pid) and "BrowserActivity" not in in_front_now(), in_front_now())
    stored = until(lambda: any(re.fullmatch(r"pb-film( \(\d+\))?\.mp4", f) for f in files()), 60)
    check("and the MP4 still reaches the phone's Downloads", stored is not None, files()[-8:])
    ended = until(lambda: not adb("shell", "pidof", PACKAGE).strip(), 30)
    check("and with the stream saved and no screen, the app ends", ended is not None)
    adb("shell", "rm", "-f", "/sdcard/Download/pb-film.mp4")


def notification_of(name):
    """The download notification for the file name, its title and text as Android keeps them,
    or None."""
    text = adb("shell", "dumpsys", "notification", "--noredact")
    for record in text.split("NotificationRecord(")[1:]:
        if "pkg=" + PACKAGE in record and "android.title=String (%s)" % name in record:
            said = re.search(r"android\.text=String \(([^)]*)\)", record)
            return said.group(1) if said else ""
    return None


def page_downloads_ask():
    """A page's downloads with no tap ask after its first (docs/video-streams.md), in both
    Mobile shells, the web one and the native one: the first goes; the second waits on a
    sheet, "site wants to download several files", and Allow lets it go."""
    files = lambda: adb("shell", "ls", "-1", "/sdcard/Download/").splitlines()
    # A file rm takes away keeps its row in Android's media store, so the next one of that
    # name lands as "name (1)": either counts.
    has = lambda stem: any(re.fullmatch(re.escape(stem) + r"( \(\d+\))?\.zip", f) for f in files())
    for shell in ("mwshell", "mkshell"):
        for stale in [f for f in files() if f.startswith("pb-ask-")]:
            adb("shell", "rm", "-f", shlex.quote("/sdcard/Download/" + stale))
        use_shell(shell)
        try:
            launch()
            go(BASE + "/two-files")
            first = until(lambda: has("pb-ask-one"), 20)
            check(shell + ": a page's first download with no tap goes", first is not None, files()[-6:])
            asks = until(lambda: find("shell", text=lambda t: "wants to download several files" in (t or "")) or
                         find("shell", label=lambda t: "wants to download several files" in (t or "")), 20)
            check(shell + ": its second asks first, on a sheet: site wants to download several files",
                  asks is not None and not has("pb-ask-two"), files()[-6:])
            shot("28-downloads-ask-" + shell)
            if asks is not None:
                tap(middle(see_named("shell", "Allow")))
            second = until(lambda: has("pb-ask-two"), 20)
            check(shell + ": Allow lets it go, and the sheet goes",
                  second is not None and until(lambda: find("shell", text=lambda t: "wants to download several files" in (t or "")) is None, 10) is not None, files()[-6:])
        finally:
            use_shell("")
            for stale in [f for f in files() if f.startswith("pb-ask-")]:
                adb("shell", "rm", "-f", shlex.quote("/sdcard/Download/" + stale))


def downloads():
    """A file the page downloads goes to the phone's Downloads, with a notification that
    says so and opens it at a tap; the menu's Downloads opens the Downloads page, which lists
    it, Open opens a zip in another app, and Show in Downloads opens the phone's Downloads app;
    a restart doesn't download it again; and Open shows a text file in a tab of the app's own,
    as the engine shows it itself."""
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    launch()
    name = "pb-check-%d.zip" % int(time.time())
    go(BASE + "/a")
    asked("/a")
    go(BASE + "/dl/" + name)
    check("a download keeps the page where it was", until(lambda: find("page", text="to b")) is not None)
    stored = until(lambda: name in adb("shell", "ls", "/sdcard/Download/"), 20)
    check("the file reaches the phone's Downloads", stored is not None, adb("shell", "ls", "/sdcard/Download/")[-200:])
    said = until(lambda: (notification_of(name) or "").startswith("Downloaded") or None, 10)
    check("a notification says it is downloaded", said is not None, str(notification_of(name)))
    menu("Downloads")
    row = until(lambda: find("page", text=lambda t: (t or "").startswith(name)))
    check("the menu's Downloads opens the Downloads page, which lists it by its name", row is not None)
    check("and the pill says Downloads, with a close", find("shell", label="Close Downloads") is not None)
    shot("25-downloads")
    # A tap on the row opens the file (docs/design-system.md, "On a phone").
    tap(middle(row))
    # A window other than ours, once Android has one in front: the chooser, or the app.
    opened = until(lambda: "Window{" in in_front_now() and "BrowserActivity" not in in_front_now(), 10)
    check("a tap on its row hands the file to another app", opened is not None, in_front_now())
    shot("26-downloads-open")
    key(4)
    until(lambda: "BrowserActivity" in in_front_now(), 10)
    # Everything else is in its ⋮, a sheet of actions.
    sheet_item = lambda label: (tap(middle(see("page", label="Actions for " + name))), see_named("page", label))[1]
    tap(middle(sheet_item("Show in Downloads")))
    shown = until(lambda: "documentsui" in in_front_now().lower() or "downloads" in in_front_now().lower(), 10)
    check("Show in Downloads, in its ⋮, opens the phone's Downloads app", shown is not None, in_front_now())
    shot("27-downloads-shown")
    key(4)
    until(lambda: "BrowserActivity" in in_front_now(), 10)
    tap(middle(sheet_item("Share")))
    shared = until(lambda: "chooser" in in_front_now().lower() or "intentresolver" in in_front_now().lower(), 10)
    check("and Share hands its copy in the phone's Downloads to the share sheet", shared is not None, in_front_now())
    shot("27b-downloads-share")
    key(4)
    until(lambda: "BrowserActivity" in in_front_now(), 10)
    press("Close Downloads")
    # A tap on the notification opens the file, as Open does.
    adb("shell", "cmd", "statusbar", "expand-notifications")
    time.sleep(1)
    dump = adb("exec-out", "uiautomator", "dump", "/dev/tty")
    at = re.search(r'text="%s"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"' % re.escape(name), dump)
    if at:
        a, b, c, e = map(int, at.groups())
        tap(((a + c) // 2, (b + e) // 2))
    opened = until(lambda: "Window{" in in_front_now() and "BrowserActivity" not in in_front_now() and "NotificationShade" not in in_front_now(), 10)
    check("a tap on the notification opens the file", at is not None and opened is not None, in_front_now())
    key(4)
    adb("shell", "cmd", "statusbar", "collapse")
    # The same name again, which the phone's Downloads has now: Android takes it only under
    # the next free name, and once refused it outright, which closed the app.
    until(lambda: "BrowserActivity" in in_front_now(), 10)
    go(BASE + "/dl/" + name)
    again = name.replace(".zip", " (1).zip")
    stored = until(lambda: again in adb("shell", "ls", "/sdcard/Download/"), 20)
    check("the same name again reaches the phone's Downloads as name (1), and the app keeps running",
          stored is not None and alive(), adb("shell", "ls", "/sdcard/Download/")[-200:])
    # The tab is still at the page it showed, so a restart loads that, not the download.
    served.clear()
    launch()
    time.sleep(3)
    check("a restart doesn't download it again", not any(s.startswith("/dl/") for s in served), str(served))
    # On the Downloads page again: Delete, from its ⋮, last, as it takes the file away.
    menu("Downloads")
    see("page", text=lambda t: (t or "").startswith(name))
    # Delete, asked first, deletes it, and only it: the same name's later copy, name (1),
    # stays (docs/fragility.md, "Delete on the older of two same-named downloads").
    tap(middle(sheet_item("Delete")))
    tap(middle(see("page", text="Delete", kind="android.widget.Button")))
    files = lambda: adb("shell", "ls", "-1", "/sdcard/Download/").splitlines()
    check("Delete deletes it from the phone's Downloads, and the list", until(lambda: name not in files() and find("page", label="Actions for " + name) is None, 10) is not None,
          [f for f in files() if f.startswith(name[:-len(".zip")])])
    check("and leaves the same name's later copy", again in files() and find("page", label="Actions for " + again) is not None,
          [f for f in files() if f.startswith(name[:-len(".zip")])])
    # A text file the engine shows itself: Open, a tap on its row, shows it in a tab here.
    press("Close Downloads")
    text = "pb-text-%d.txt" % int(time.time())
    go(BASE + "/dl/" + text)
    until(lambda: text in adb("shell", "ls", "/sdcard/Download/"), 20)
    menu("Downloads")
    tap(middle(see("page", text=lambda t: (t or "").startswith(text))))
    shown = until(lambda: find("page", text=lambda t: "Project B download check" in (t or "")), 15)
    page = screen_parts().get("page")
    check("Open shows a text file in a tab of the app's own", shown is not None and "BrowserActivity" in in_front_now(),
          (in_front_now(), sorted(screen_parts()), [t for t in (n.get("text") for n in (page.iter("node") if page is not None else ())) if t][:8]))


def page_camera():
    """A page's camera (docs/media-permissions.md): Go asks the site in its sheet, then
    Android for the app's own permission, the first time an allowed site needs it, and the
    page gets the emulator's camera; refused at Android's question, the sheet says so in
    Android's words, and Not now denies the page."""
    camera = "android.permission.CAMERA"
    adb("shell", "pm", "revoke", PACKAGE, camera)
    adb("shell", "pm", "clear-permission-flags", PACKAGE, camera, "user-set", "user-fixed")
    launch()
    go(BASE + "/camera")
    asked("/camera")
    tap(middle(screen_parts().get("page")))
    site = until(lambda: find("shell", text="Always allow"), 15)
    check("a page asking for the camera brings the site's question, as a sheet", site is not None and find("shell", text=lambda t: "wants to use your camera" in (t or "")) is not None)
    shot("50-camera-site")
    if site is None:
        return
    tap(middle(site))
    allow_ids = ["com.android.permissioncontroller:id/permission_allow_foreground_only_button",
                 "com.android.permissioncontroller:id/permission_allow_button"]
    question = until(lambda: next((n for n in (system_node(**{"resource-id": i}) for i in allow_ids) if n is not None), None), 15)
    check("allowed there, Android asks for the app's own camera permission", question is not None, in_front_now())
    shot("51-camera-android")
    if question is not None:
        tap(middle(question))
    check("and allowed there too, the page gets the camera", asked("/said?camera=ok", 20), str(served[-5:]))

    # Refused at Android's question, which Android 10 always asks again (it has no
    # permission flags to fix a refusal with): the sheet says why.
    adb("shell", "am", "force-stop", PACKAGE)
    adb("shell", "pm", "revoke", PACKAGE, camera)
    try:
        launch()
        go(BASE + "/camera")
        asked("/camera")
        tap(middle(screen_parts().get("page")))
        refused = lambda: find("shell", text=lambda t: (t or "").startswith("Android hasn't let Project B use your camera"))
        refuse_android_question(refused)
        said = until(refused, 15)
        check("Android refusing it, the sheet says so in Android's words, with Open settings and Not now",
              said is not None and find("shell", text="Open settings") is not None and find("shell", text="Not now") is not None)
        shot("52-camera-refused")
        if said is not None:
            tap(middle(find("shell", text="Not now")))
        check("and Not now denies the page", asked("/said?camera=NotAllowedError", 15), str(served[-5:]))
    except TimeoutError:
        stopped_here("page_camera")
        raise
    finally:
        # The next run sees Android's question again.
        for p in (camera, "android.permission.RECORD_AUDIO"):
            adb("shell", "pm", "clear-permission-flags", PACKAGE, p, "user-set", "user-fixed")
            adb("shell", "pm", "revoke", PACKAGE, p)


def settings_back():
    """The back key inside Settings goes back to where it was, not out of Settings
    (docs/fragility.md, "The back key out of Settings"): from the Sites list to Privacy and
    sites, the section it was opened from, then to the list of sections, and only from there out."""
    launch()
    menu("Settings")
    see("settings", text=SETTINGS_LIST)
    # Privacy and sites has Sites, which opens as a list of its own. A row that opens a list is named by
    # its label.
    settings_section("Privacy and sites", lambda t: t == "Clear browsing data…" or t == "Block ads")
    tap(middle(see_named("settings", "Sites")))
    check("Sites opens as a list of its own", until(lambda: find("settings", label=lambda l: (l or "").startswith("Back to"))) is not None)
    key(4)
    check("the back key goes back to the section it was opened from, still in Settings",
          until(lambda: "settings" in screen_parts() and find("settings", label="Back to Privacy and sites") is None and named("settings", "Sites") is not None) is not None)
    key(4)
    check("then to the list of sections", until(lambda: "settings" in screen_parts() and named("settings", "Sites") is None and find("settings", text=SETTINGS_LIST) is not None) is not None)
    key(4)
    check("and only then out of Settings", until(lambda: "settings" not in screen_parts()) is not None)


def notifications_allowed():
    return "android.permission.POST_NOTIFICATIONS: granted=true" in adb("shell", "dumpsys", "package", PACKAGE)


def forget_notifications():
    """Notifications not allowed and never asked about: Android's question asks again, and
    the download bar's offer, shown once, comes again (downloads.json's notificationsOffered)."""
    adb("shell", "am", "force-stop", PACKAGE)
    adb("shell", "pm", "revoke", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    adb("shell", "pm", "clear-permission-flags", PACKAGE, "android.permission.POST_NOTIFICATIONS", "user-set", "user-fixed")
    kept = profile_json("downloads.json")
    if kept.pop("notificationsOffered", None) is not None:
        profile_file("downloads.json", kept)


def notification_buttons(name):
    """The download notification for the file name: Android's id for it and its buttons'
    words, as Android keeps them, or (None, [])."""
    text = adb("shell", "dumpsys", "notification", "--noredact")
    for record in text.split("NotificationRecord(")[1:]:
        if "pkg=" + PACKAGE in record and "android.title=String (%s)" % name in record:
            return re.search(r" id=(\d+)", record).group(1), re.findall(r'\[\d+\] "([^"]+)" -> PendingIntent', record)
    return None, []


def press_notification_button(id, command):
    """Sends what a download notification's button sends, PBDownloadCommand command (1
    cancel, 2 pause, 3 resume): the shade can't be read while the notification moves, as
    uiautomator waits for a stillness it never has, so its buttons can't be found to tap."""
    adb("shell", "am", "broadcast", "-a", "dev.projectb.browser.DOWNLOAD_COMMAND", "-p", PACKAGE,
        "--el", "id", id, "--ei", "command", str(command))


def download_notifications():
    """In both Mobile shells: while notifications aren't allowed, Android doesn't ask by
    itself as a download starts; the first download's bar offers them, once, "Show progress
    in notifications?" with Allow, which asks Android. Then a running download's notification
    says how much of how much and the time left, with Pause and Cancel; Pause pauses it, with
    Resume; Resume runs it again; and Cancel takes it and its notification away. Before
    Android 13 an app needs no leave to show notifications, so there is nothing to offer:
    the bar shows the download with no offer, and the notification shows all the same."""
    # Android 13 (API 33) brought POST_NOTIFICATIONS, the leave the offer asks for.
    asks = int(adb("shell", "getprop", "ro.build.version.sdk").strip() or 0) >= 33
    for shell in ("mwshell", "mkshell"):
        forget_notifications()
        use_shell(shell)
        launch()
        go(BASE + "/a")
        asked("/a")
        name = "pb-slow-%s-%d.bin" % (shell, int(time.time()))
        go(BASE + "/slow/" + name)
        if not asks:
            said = until(lambda: find("shell", text=lambda t: (t or "").startswith("Downloading " + name)), 10)
            check("%s: before Android 13 the first download's bar shows it with no offer, as notifications need no leave" % shell,
                  said is not None and find("shell", text="Show progress in notifications?") is None,
                  said.get("text") if said is not None else None)
            adb("shell", "am", "force-stop", PACKAGE)
            continue
        # The question, the words and Allow from one reading of the screen, and Allow at
        # once: each reading takes a second or two, and the offer lasts 8.
        def offered():
            shell_part = screen_parts().get("shell")
            nodes = list(shell_part.iter("node")) if shell_part is not None else []
            ask = next((n for n in nodes if n.get("text") == "Show progress in notifications?"), None)
            allow = next((n for n in nodes if "Allow" in (n.get("text"), n.get("content-desc"))), None)
            words = next((n for n in nodes if (n.get("text") or "").startswith("Downloading pb-slow")), None)
            return (words, allow, in_front_now()) if ask is not None else None
        offer = until(offered, 10)
        words, allow, front = offer if offer is not None else (None, None, "")
        check("%s: the first download's bar offers notifications" % shell, offer is not None and words is not None,
              words.get("text") if words is not None else None)
        if allow is None:
            check("%s: the offer has Allow" % shell, False)
            continue
        tap(middle(allow))
        check("%s: and Android doesn't ask by itself" % shell, "permissioncontroller" not in front.lower(), front)
        question = until(lambda: system_node(**{"resource-id": "com.android.permissioncontroller:id/permission_allow_button"}), 10)
        check("%s: Allow asks Android" % shell, question is not None, in_front_now())
        shot("33-download-asked-" + shell)
        if question is not None:
            tap(middle(question))
        check("%s: and once allowed, the app may show them" % shell, until(notifications_allowed, 10) is not None)
        # Cancelled from the app, so the next shell starts with nothing running.
        adb("shell", "am", "force-stop", PACKAGE)
        shot("34-download-allowed-" + shell)
    # Shown once, the offer never comes again, taken up or not.
    adb("shell", "pm", "revoke", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    launch()
    go(BASE + "/a")
    asked("/a")
    name = "pb-again-%d.bin" % int(time.time())
    go(BASE + "/slow/" + name)
    said = until(lambda: find("shell", text=lambda t: (t or "").startswith("Downloading " + name)), 10)
    check("once shown, the offer never comes again", said is not None and "notifications" not in said.get("text", ""),
          said.get("text") if said is not None else None)
    # The notification: sizes and time left, with Pause and Cancel.
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    launch()
    go(BASE + "/a")
    asked("/a")
    name = "pb-notify-%d.bin" % int(time.time())
    go(BASE + "/slow/" + name)
    running = until(lambda: re.search(r" of 60 MB · \d+ (sec|secs|min|mins) left$", notification_of(name) or ""), 15)
    check("a running download's notification says how much of how much, and the time left", running is not None, str(notification_of(name)))
    id, buttons = notification_buttons(name)
    check("with Pause and Cancel", buttons == ["Pause", "Cancel"], str(buttons))
    adb("shell", "cmd", "statusbar", "expand-notifications")
    time.sleep(2)
    shot("35-download-notification")
    adb("shell", "cmd", "statusbar", "collapse")
    if id is not None:
        press_notification_button(id, 2)
        paused = until(lambda: (notification_of(name) or "").startswith("Paused · "), 10)
        check("Pause pauses it, and the notification says so", paused is not None, str(notification_of(name)))
        check("with Resume and Cancel", until(lambda: notification_buttons(name)[1] == ["Resume", "Cancel"], 5) is not None,
              str(notification_buttons(name)[1]))
        press_notification_button(id, 3)
        again = until(lambda: " of 60 MB" in (notification_of(name) or "") and not (notification_of(name) or "").startswith("Paused"), 10)
        check("Resume runs it again", again is not None, str(notification_of(name)))
        press_notification_button(id, 1)
        gone = until(lambda: notification_of(name) is None, 10)
        check("Cancel takes it and its notification away", gone is not None, str(notification_of(name)))
    use_shell("")


def start_as_it_is():
    """Starts the app as a person does, after its process has gone, leaving whatever notice
    it shows (launch dismisses the crash notice)."""
    until(lambda: not adb("shell", "pidof", PACKAGE).strip())
    adb("shell", "rm", "-f", "/data/data/%s/%s/session.json" % (PACKAGE, PROFILE))
    adb("shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY, "--ez", "skip-welcome", "true")
    see("shell", timeout=60, label="Menu")


def relaunch_on_the_phone():
    """Settings' Relaunch starts the app again on a phone, as Chrome restarts, through the
    activity (ProcessPhoenix): a flag's change is applied in a new process, in front, with no
    crash notice; twice, turning Show window regions on and back off. It ran /bin/sh with Go's
    os/exec, which Android 10 answers with SIGSYS, ending the app, and which a later Android,
    with no /bin/sh, refused, so the app quit and never came back."""
    launch()
    flag = "Show window regions"
    pending = lambda: find("settings", text=starts("Relaunch to apply"))
    for state in ("on", "off"):
        menu("Settings")
        see("settings", text=SETTINGS_LIST)
        # The row's words are the switch's label: a tap on them turns it.
        tap(middle(settings_section("Flags", flag)))
        see("settings", text=starts("Relaunch to apply"))
        before = adb("shell", "pidof", PACKAGE).strip()
        tap(middle(see_named("settings", "Relaunch")))
        now = until(lambda: (lambda p: p if p and p != before else None)(adb("shell", "pidof", PACKAGE).strip()), 30)
        see("shell", timeout=60, label="Menu")
        check("Relaunch with %s %s starts the app again, in a new process, in front" % (flag, state),
              now is not None and until(in_front, 10) is not None, "pid %s then %s" % (before, now))
        check("with no crash notice, and nothing left of the restart",
              find("shell", text=starts("Project B crashed last time")) is None
              and ":phoenix" not in adb("shell", "ps", "-A", "-o", "NAME"))
        menu("Settings")
        see("settings", text=SETTINGS_LIST)
        settings_section("Flags", flag)
        check("and the flag's change is applied, nothing left to relaunch", until(lambda: pending() is None, 5) is not None)
        close_settings()


def crash_notice():
    """The "crashed last time" notice comes only after a crash (docs/fragility.md, "A
    crash notice after runs that never crashed"): not after Android ends the app in the
    background, nor after it's swiped away or stopped from its app info, all of which leave
    its crash.log behind as a crash would; a native crash of the app's process still brings
    it. Android 11 and later say why the process ended; before 11 the app's crash log does."""
    said = lambda: find("shell", text=lambda t: (t or "").startswith("Project B crashed last time"))
    # The process went, so the check below is of a run that ended.
    ended = lambda: until(lambda: not adb("shell", "pidof", PACKAGE).strip(), 10) is not None
    launch()
    go(BASE + "/a")
    asked("/a")
    # In the background, ended as Android ends one it wants the memory of. am kill ends
    # only an app Android already ranks as in the background, which may take it a moment
    # after Home on a busy phone, and else does nothing: so again until it has gone.
    key(3)
    gone = until(lambda: adb("shell", "am", "kill", PACKAGE) is not None and not adb("shell", "pidof", PACKAGE).strip(), 10) is not None
    start_as_it_is()
    check("no crash notice after Android ended the app in the background", gone and until(said, 5) is None, "" if gone else "it never ended")
    # Stopped from its app info, Force stop.
    adb("shell", "am", "force-stop", PACKAGE)
    gone = ended()
    start_as_it_is()
    check("nor after Force stop", gone and until(said, 5) is None, "" if gone else "it never ended")
    # Swiped away from the recent apps: Android 14 and later stop it so from the shell.
    if "stop-app" in adb("shell", "am", "help"):
        adb("shell", "am", "stop-app", PACKAGE)
        gone = ended()
        start_as_it_is()
        check("nor after it was swiped away", gone and until(said, 5) is None, "" if gone else "it never ended")
    # A crash of the app's own process.
    adb("shell", "kill", "-SEGV", adb("shell", "pidof", PACKAGE).strip().split()[0])
    start_as_it_is()
    notice = until(said, 10)
    check("a native crash still brings the notice, naming no macOS report", notice is not None and "macOS" not in (notice.get("text") or ""),
          notice.get("text") if notice is not None else "no notice")
    shot("36-crash-notice")
    dismiss = named("shell", "Dismiss")
    if dismiss is not None:
        tap(middle(dismiss))
    # A second crash within a minute of this one, as a rerun of this part brings, has
    # Android put "Project B keeps stopping" over the app, and it stays across a reinstall.
    adb("shell", "am", "broadcast", "-a", "android.intent.action.CLOSE_SYSTEM_DIALOGS")


def screen_size():
    """The phone's screen in device pixels: width, height."""
    w, h = re.search(r"(\d+)x(\d+)", adb("shell", "wm", "size").split("Physical size:", 1)[1]).groups()
    return int(w), int(h)


def system_node(**want):
    """The first node on the whole screen whose attributes are as wanted, for another app's
    window, such as the system's file picker. label is content-desc."""
    out = adb("exec-out", "uiautomator", "dump", "/dev/tty")
    if "</hierarchy>" not in out:
        return None
    root = ET.fromstring(out[:out.rindex("</hierarchy>") + len("</hierarchy>")])
    names = {"label": "content-desc"}
    for node in root.iter("node"):
        if all(v(node.get(names.get(k, k)) or "") if callable(v) else node.get(names.get(k, k)) == v for k, v in want.items()):
            return node
    return None


def box_of(bounds):
    """A bounds attribute as left, top, right, bottom."""
    return tuple(map(int, re.findall(r"\d+", bounds)))


def picker_shows():
    return "documentsui" in in_front_now().lower()


def pick(name, several=False):
    """Picks the file name in the system's picker, from Downloads: a tap for one file, or a
    long press and Select where the page takes several."""
    if until(lambda: system_node(text=name), 5) is None:
        tap(middle(until(lambda: system_node(label="Show roots"))))
        tap(middle(until(lambda: system_node(text="Downloads"))))
    node = until(lambda: system_node(text=name))
    if several:
        long_press(middle(node))
        tap(middle(until(lambda: system_node(text=lambda t: t.lower() == "select"))))
    else:
        tap(middle(node))


def file_inputs():
    """The picker checks, then the profile cleared: the sparks they import would otherwise run
    on every page of the parts after, and one's floating button put a second "to" on
    find_in_page's page (its 1/2 flake)."""
    try:
        file_input_checks()
    except TimeoutError:
        stopped_here("file_inputs")
        raise
    finally:
        adb("shell", "am", "force-stop", PACKAGE)
        clear_data()


def file_input_checks():
    """A page's file input opens the phone's own picker, and the file picked reaches the page
    by its own name; Settings' Sparks Import… picks a zip of sparks there and imports them.
    Content said no to every file input on Android until the window asked Go and opened the
    picker, as the Mac's does."""
    sparks = os.path.join(ROOT, "core/sparks/testdata/site-sparks")
    local = os.path.join(OUT, "SiteSparks.zip")
    with zipfile.ZipFile(local, "w") as z:
        for name in sorted(os.listdir(sparks)):
            z.write(os.path.join(sparks, name), "SiteSparks/" + name)
            z.writestr("__MACOSX/SiteSparks/._" + name, b"\x00\x05\x16\x07junk")
    size = os.path.getsize(local)
    adb("push", local, "/sdcard/Download/SiteSparks.zip")
    adb("shell", "content", "call", "--uri", "content://media/", "--method", "scan_volume", "--arg", "external_primary")
    launch()
    go(BASE + "/upload")
    asked("/upload")
    def open_picker():
        """Taps the page's file input and waits for the phone's picker. An older Android asks
        for the camera first: the app declares it, for the QR scanner, and content's chooser
        asks so that it can offer the camera as a source. Refused, the picker opens all the
        same, which is what this checks; allowed, Android shows its own chooser before it."""
        tap(middle(screen_parts().get("page")))
        refuse_android_question(picker_shows)
        return until(picker_shows) is not None

    opened = open_picker()
    check("a page's file input opens the phone's picker", opened, in_front_now())
    if not opened:
        return
    shot("40-file-picker")
    key(4)
    check("and closing it tells the page", asked("/said?cancelled"), str(served))
    open_picker()
    pick("SiteSparks.zip")
    check("a file picked reaches the page by its own name", asked("/said?file=" + urllib.parse.quote("SiteSparks.zip:%d" % size)), str(served))

    menu("Settings")
    see("settings", text=SETTINGS_LIST)
    tap(middle(see("settings", kind=FIELD)))
    type_text("Sparks")
    # Android 10's keyboard holds the word it is composing, and the search reads only what is
    # put in: until then the whole list shows, its Sparks row under the keyboard, and a tap
    # there took a suggestion ("Spark's"). Putting the keyboard away puts the word in.
    matched = lambda shown: shown is not None and shown.startswith("1 setting matches")
    if until(lambda: find("settings", text=matched), 5) is None:
        key(4)
        see("settings", text=matched)
    # On Android 10 a tap after typing can only close the keyboard: the result is tapped
    # again, where it is then, until Sparks opens.
    for _ in range(3):
        result = find("settings", label="Sparks", kind="android.widget.Button")
        if result is not None:
            tap(middle(result))
        if until(lambda: find("settings", text="New", kind="android.widget.Button"), 5):
            break
    # The menu beside New: Android reads no name for it (it is NAF), so it is the button
    # after New on the heading line.
    new = see("settings", text="New", kind="android.widget.Button")
    tap(middle(see("settings", kind="android.widget.Button", bounds=lambda b: b is not None and
                   box_of(b)[0] >= box(new)[2] and box_of(b)[1] == box(new)[1])))
    tap(middle(see("settings", text="Import…")))
    refuse_android_question(picker_shows)
    check("Settings' Sparks Import… opens the phone's picker", until(picker_shows) is not None, in_front_now())
    pick("SiteSparks.zip", several=True)
    check("and a zip of sparks picked there imports them", see("settings", text="Added 3 sparks") is not None)
    shot("41-sparks-imported")
    tap(middle(see("settings", text="Done")))
    close_settings()
    # Through the media index, not rm: rm leaves its row, and the app's next download of that
    # name then died copying into Downloads on the stale row (Downloads.copyToShared).
    adb("shell", "content", "delete", "--uri", "content://media/external/downloads",
        "--where", "\"_display_name='SiteSparks.zip'\"")


def page_full_screen():
    """A page in full screen has the whole screen: Android's bars hide and the shell draws
    nothing; the back key ends it, and the page and the shell are as they were."""
    # Android's one-time note on how to leave full screen, already seen, as after a person's
    # first Got it.
    adb("shell", "settings", "put", "secure", "immersive_mode_confirmations", "confirmed")
    launch()
    go(BASE + "/full")
    asked("/full")
    before = box(see("page", text="full"))
    tap(middle(see("page", text="full")))
    check("a page's requestFullscreen is granted", asked("/full-on"), str(served))
    # Not by the page's box in Android's tree: on Android 10 uiautomator cuts every box at the
    # bottom of the screen less the navigation bar, even with the bar hidden and the page
    # drawn there. The page says its own size, and the screen's top and bottom rows show
    # its colour, not a bar's or the shell's.
    width, height = screen_size()
    sizes = lambda: [tuple(map(int, s.split("?")[1].split("x"))) for s in served if s.startswith("/full-size?")]
    whole = lambda: bool(sizes()) and all(abs(a - b) <= 2 for a, b in zip(sizes()[-1], (width, height)))
    green = lambda c: all(abs(a - b) < 10 for a, b in zip(c, (0x2e, 0x9d, 0x5b)))
    edges = lambda: [pixel_at(x, y) for x in (2, width // 2, width - 3) for y in (2, height - 3)]
    covers = until(lambda: find("shell", label="Menu") is None and whole() and all(map(green, edges())), 10)
    check("it covers the whole screen, with nothing of the shell or Android's bars on it", covers is not None,
          "the page says %s, the screen's corners and edges are %s, on a %dx%d screen"
          % (sizes()[-1:] or "nothing of its size", edges(), width, height))
    shot("30-full-screen")
    served.clear()
    key(4)
    check("the back key ends it and stays on the page", asked("/full-off") and find("page", text="full") is not None, str(served))
    check("and the window is as it was", until(lambda: find("shell", label="Menu") is not None and box(see("page", text="full")) == before) is not None,
          "%s, was %s" % (box(see("page", text="full")), before))


TALKBACK = "com.google.android.marvin.talkback/com.google.android.marvin.talkback.TalkBackService"


def talkback(on):
    """TalkBack on or off. On only to read the screen: with it on, a tap is TalkBack's to
    explore, not the app's. Its notification prompt is answered first, for good, so it never
    covers what is read."""
    if on:
        adb("shell", "pm", "revoke", "com.google.android.marvin.talkback", "android.permission.POST_NOTIFICATIONS")
        adb("shell", "pm", "set-permission-flags", "com.google.android.marvin.talkback",
            "android.permission.POST_NOTIFICATIONS", "user-fixed")
        adb("shell", "settings", "put", "secure", "enabled_accessibility_services", TALKBACK)
        adb("shell", "settings", "put", "secure", "accessibility_enabled", "1")
    else:
        adb("shell", "settings", "delete", "secure", "enabled_accessibility_services")
        adb("shell", "settings", "put", "secure", "accessibility_enabled", "0")


def talkback_states():
    """With TalkBack on, a page's switch and checkbox read named, checkable and as they are:
    Chromium gives a page the screen reader's tree only once it hears TalkBack runs, and the
    app once kept the answer it got first, so every switch read off (docs/fragility.md)."""
    if "talkback" not in adb("shell", "pm", "list", "packages", "com.google.android.marvin.talkback"):
        # An image without it, as the Android 10 one: nothing to read with, so no check here.
        print("note: talkback_states not checked: this emulator has no TalkBack", flush=True)
        return
    launch()
    go(BASE + "/switches")
    asked("/switches")
    see("page", text="Switches")
    try:
        talkback(True)
        time.sleep(8)
        page = until(lambda: find("page", label="Ad blocker"), 15)
        box = find("page", label="Box")
        said = lambda n: None if n is None else (n.get("class", "").rsplit(".", 1)[-1], n.get("checkable"), n.get("checked"))
        check("TalkBack hears the page's switch as named, checkable and on",
              page is not None and page.get("checkable") == "true" and page.get("checked") == "true", said(page))
        check("and its checkbox the same", box is not None and box.get("checkable") == "true" and box.get("checked") == "true", said(box))
        # The name where TalkBack reads it: the label, text, or a field's hint.
        # A node with no children is false to Python, so each find is tested against None.
        def named(name):
            for want in ({"label": name}, {"text": name, "checkable": "true"}, {"hint": name}):
                n = find("page", **want)
                if n is not None:
                    return n
            return None
        # A field's <label for> reaches TalkBack as its hint from Android 16 on; below, it goes as
        # the field's labeledBy, which uiautomator doesn't show, so it can't be read here there.
        sdk = int(adb("shell", "getprop", "ro.build.version.sdk").strip() or 0)
        ways = [("Wrapped", "by a label around it"), ("By id", "by aria-labelledby")]
        if sdk >= 36:
            ways.append(("Field", "by <label for>"))
        else:
            print("note: the <label for> field's name isn't checked below Android 16 (API %d)" % sdk, flush=True)
        for name, how in ways:
            # Each name waited for: the screen reader's tree fills in a moment after TalkBack
            # starts, a field's hint last (a flake on 5602).
            n = until(lambda: named(name), 10)
            check("and a control named %s says its name" % how, n is not None,
                  None if n is None else (n.get("class", "").rsplit(".", 1)[-1], n.get("text"), n.get("content-desc"), n.get("hint")))
        # With the screen reader on, pages whose words Chromium gives it from the app's
        # resources: a <usermedia> element's, and images' with none of their own. Opened by a
        # link from another app, as a tap is TalkBack's to read, not to press.
        for path, what in (("/usermedia", "a <usermedia> element"), ("/images", "images with no words of their own")):
            since = len(served)
            adb("shell", "am", "start", "-W", "-a", "android.intent.action.VIEW", "-d", url(path), PACKAGE)
            check("with TalkBack on, a page with %s keeps its renderer" % what,
                  until(lambda: any(s.startswith(path + "-alive") for s in served[since:]), 30) is not None,
                  str(served[since:][-4:]))
    finally:
        talkback(False)


def find_in_page():
    """Find in page from the menu: the bar takes the keyboard, a word typed is found and
    counted, and the back key closes the bar."""
    launch()
    go(BASE + "/a")
    asked("/a")
    menu("Find in page")
    field = until(lambda: find("shell", kind=FIELD, focused="true", label=lambda l: l != "Address"))
    check("Find in page in the menu opens the find bar with the keyboard in it", field is not None)
    # The word in the field before its count: input text can be lost while the keyboard is
    # still coming up (a flake on emu-37 in 0.2.99), so it types once more if it didn't land.
    typed = lambda: find("shell", kind=FIELD, text=lambda t: (t or "").lower() == "to")
    type_text("to")
    if until(typed, 5) is None:
        tap(middle(field))
        type_text("to")
    counted = until(lambda: find("shell", text="1/1")) is not None
    shown = find("shell", kind=FIELD, focused="true")
    # What count the bar shows instead, if any: the word was typed (0.2.100 held 'to') and
    # still no 1/1, so the count itself is what to look at next.
    counts = [n.get("text") for n in (screen_parts().get("shell") or ET.Element("x")).iter("node")
              if re.fullmatch(r"\d+/\d+|No results|0", n.get("text") or "")]
    if not counted:
        shot("31-find-not-counted")
    check("a word typed is found in the page and counted", counted,
          "the field holds %r, the bar shows %r" % (shown.get("text") if shown is not None else None, counts))
    shot("31-find")
    key(4)
    if find("shell", text="1/1") is not None:
        key(4)  # The first back may only put the keyboard away, as Android's back does.
    check("the back key closes the bar", until(lambda: find("shell", text="1/1") is None and find("shell", label="Menu") is not None) is not None)


def pixel_at(x, y):
    """The screen's colour at device pixel x, y, as (r, g, b)."""
    raw = subprocess.run(ADB + ["exec-out", "screencap"], capture_output=True).stdout
    width = int.from_bytes(raw[0:4], "little")
    at = 16 + (y * width + x) * 4
    return tuple(raw[at:at + 3])


def page_paints():
    """A page with SVG icons in a tall rounded box draws whole: the Downloads page's header
    row in the page's colour, not left undrawn as white, black, stale or magenta tiles."""
    launch()
    if find("shell", kind=FIELD) is None:
        menu("New Tab")
    go(BASE + "/paint")
    asked("/paint")
    time.sleep(2)
    left, top, right, _ = box(screen_parts()["page"])
    at = ((left + right) // 2, top + 60)
    near = lambda c, want: all(abs(a - b) < 10 for a, b in zip(c, want))
    drawn = until(lambda: near(pixel_at(*at), (0xe8, 0xf2, 0xf2)) or None, 5)
    check("a page with SVG icons in a rounded box draws its header row", drawn is not None, str(pixel_at(*at)))
    shot("35-paint")


def history_after_a_restart():
    launch()
    type_in_bar("page")
    check("history still suggests Page A after a restart",
          until(lambda: find("shell", text=lambda shown: shown.startswith("Page A"))) is not None and visited(url("/a")))
    shot("15-suggestions-after-restart")


def renderers():
    """The pids of the app's renderers, the shell's first: it starts first."""
    return sorted(int(pid) for pid, name in children() if "sandboxed" in name)


def where_go_runs():
    """Go is in the app's process and in none of its children. The page needs a renderer first."""
    launch()
    go(BASE + "/a")
    asked("/a")
    main_pid = next(pid for pid, name in processes() if name == PACKAGE)
    kids = children()
    go_size = go_library_mappings(main_pid)
    check("Go's library is mapped in the main process", go_size > 0, "%d executable mapping(s)" % go_size)
    kids += [(pid, "app:zygote") for pid, name in processes() if name == PACKAGE + "_zygote"]
    for pid, name in kids:
        rmaps = adb("shell", "cat", "/proc/%s/maps" % pid)
        check("Go's library is not in " + name.split(":")[1], len(rmaps) > 0 and go_library_mappings(pid) == 0, "pid " + pid)
    check("the shell and the page have separate renderers", len(renderers()) >= 2, str(kids))


def links_from_other_apps():
    """A link another app sends opens in the app, running or not."""
    view = lambda path: adb("shell", "am", "start", "-W", "-a", "android.intent.action.VIEW",
                            "-d", url(path), PACKAGE)
    launch()
    served.clear()
    view("/b")
    check("a link from another app opens while the app runs", asked("/b"), str(served))
    adb("shell", "am", "force-stop", PACKAGE)
    until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME"))
    served.clear()
    view("/a")
    check("a link from another app opens the app and the link", asked("/a", 60), str(served))
    shot("15b-link-from-another-app")


# A sync link as the memd page offers one (docs/sync.md, The link).
SYNC_LINK = "project-b://sync?address=https%3A%2F%2Fsync.example.test%2Fsync%2Fabc"
DEVICE_KEY = "/data/data/%s/files/device.key" % PACKAGE


def view_link(link):
    """Sends a link as another app does, quoted for the device's shell."""
    adb("shell", "am", "start", "-W", "-a", "android.intent.action.VIEW", "-d", "'%s'" % link, PACKAGE)


def settings_text(want):
    """A node of the settings page whose text is want, or a function of it."""
    return find("settings", text=want)


def starts(words):
    """A test of a node's text, that it starts with words."""
    return lambda t: (t or "").startswith(words)


# Settings on a phone opens at its list of sections (docs/settings.md). Its row Appearance
# marks the list: a phone always has that section, and only the list and the section itself
# show the word first, where Profile, which only holds a named profile on a phone, is not
# there at all.
SETTINGS_LIST = starts("Appearance")


def settings_section(label, inside):
    """Settings at the section label: the list's row for it tapped, unless the page is in it
    already, where the text inside shows. Returns that node."""
    until(lambda: find("settings", text=inside) or find("settings", text=starts(label)))
    if find("settings", text=inside) is None:
        tap(middle(see("settings", text=starts(label))))
    return see("settings", text=inside)


def sync_on_the_phone():
    """Sync on a phone (docs/sync.md). A project-b://sync link from another app opens Settings
    at Sync with the link's Dialog, its address filled in, whether the app runs or not. Turning
    sync on against a sync server on this Mac (core/cmd/syncserver, reached through adb reverse,
    its own CA put in the device's user CA folder, which Go reads, as sync takes only https)
    makes the device key, wrapped by the Android Keystore, and the profile's key; after a
    restart sync runs again with what was kept, so the Keystore opened the key; and oskit's
    Secret logs no exception."""
    stopped = lambda: (adb("shell", "am", "force-stop", PACKAGE), until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME")))
    linked = lambda: until(lambda: settings_text("https://sync.example.test/sync/abc"), 30)
    adb("logcat", "-c")
    launch()
    view_link(SYNC_LINK)
    check("a sync link from another app opens Settings at Sync with the link's Dialog, while the app runs",
          linked() is not None and settings_text(lambda t: (t or "").startswith("A link asked to set up sync")) is not None)
    # What a screen reader is given, which the page's own tests can't read: named by its
    # label, the address was told as "Server address" and never itself, and the title, its
    # host drawn in pieces, as "Sync with sync. example. test?" (docs/fragility.md, "A sync
    # link's address nobody heard").
    told = [n.get("text") for n in screen_parts().get("settings", ET.Element("none")).iter("node") if n.get("class", "").endswith("TextView")]
    check("a screen reader is told the Dialog's title as one sentence and its address as it is written",
          [t for t in told if t.startswith("Sync with")] == ["Sync with sync.example.test?"] and [t for t in told if "sync.example.test/" in t] == ["https://sync.example.test/sync/abc"], told)
    shot("sync-link-running")
    stopped()
    view_link(SYNC_LINK)
    check("and when the app wasn't running", linked() is not None)
    shot("sync-link-stopped")

    folder = os.path.abspath(os.path.join(OUT, "syncserver"))
    binary = os.path.abspath(os.path.join(OUT, "syncserver-bin"))
    # The server starts with nothing synced: what a run before left in its folder would have
    # the phone asked for that run's passphrase, where this one chooses a new one.
    shutil.rmtree(folder, ignore_errors=True)
    subprocess.run(["go", "build", "-o", binary, "./cmd/syncserver"], cwd=os.path.join(ROOT, "core"), check=True)
    srv = subprocess.Popen([binary, "-addr", "127.0.0.1:0", "-dir", folder], stdout=subprocess.PIPE, text=True)
    port = int(srv.stdout.readline().strip().rsplit(":", 1)[1])
    requests = []
    threading.Thread(target=lambda: [requests.append(line.strip()) for line in srv.stdout], daemon=True).start()
    try:
        adb("reverse", "tcp:%d" % port, "tcp:%d" % port)
        adb("shell", "mkdir", "-p", "/data/misc/keychain/certs-added")
        adb("push", os.path.join(folder, "ca.pem"), "/data/misc/keychain/certs-added/project-b-sync-test.0")
        stopped()
        view_link("project-b://sync?address=" + urllib.parse.quote("https://127.0.0.1:%d/sync/phone" % port, safe=""))
        tap(middle(see("settings", text="Connect", timeout=30)))
        passphrase = "river meadow lantern copper"
        see("settings", text="Choose a sync passphrase", timeout=30)
        fields = lambda: [n for n in screen_parts()["settings"].iter("node") if n.get("class") == FIELD]
        for i in (0, 1):
            tap(middle(fields()[i]))
            type_text(passphrase)
        # The keyboard is up, and on Android 17 it is tall enough that the Dialog scrolls its
        # buttons out of sight: the tap then lands beside them and only puts the keyboard away.
        # Turn on sync is tapped again, where it is then, until its Dialog goes.
        for _ in range(3):
            button = find("settings", text="Turn on sync")
            if button is not None:
                tap(middle(button))
            if until(lambda: find("settings", text="Choose a sync passphrase") is None, 5):
                break
        # The second step, for a bucket the phone just made: its backup code (docs/sync.md,
        # "Backup code"), in two lines of three groups of four.
        see("settings", text="Save your backup code", timeout=60)
        said = lambda: " ".join(w for n in screen_parts().get("settings", ET.Element("none")).iter("node") for w in (n.get("text"), n.get("content-desc")) if w)
        check("a bucket the phone just made goes on to its backup code, six groups of four, whole in what a screen reader reads",
              until(lambda: re.search(r"\b([0-9A-Z]{4}-){5}[0-9A-Z]{4}\b", said()) is not None, 30) is not None, said()[:300])
        shot("sync-backup-code")
        tap(middle(see("settings", text="I've saved it")))  # does: syncBackupKeep
        check("I've saved it closes it, and the Sync card has its Backup code row",
              until(lambda: find("settings", text="Save your backup code") is None and find("settings", text="Backup code") is not None, 30) is not None, said()[:300])
        pushed = until(lambda: any("/push" in r for r in requests), 60)
        shot("sync-on")
        key = adb("shell", "stat", "-c", "%s", DEVICE_KEY).strip()
        profile_key = adb("shell", "stat", "-c", "%s", "/data/data/%s/%s/profile.key" % (PACKAGE, PROFILE)).strip()
        check("turning sync on syncs, and makes the device key, wrapped by the Keystore, and the profile's key",
              pushed and key.isdigit() and int(key) > 32 and profile_key.isdigit(), (requests[-3:], key, profile_key))
        # Sync now, pressed, asks the server again: nothing else does within these seconds, as
        # sync's own runs come five minutes apart with nothing changed here.
        time.sleep(3)
        before = len(requests)
        tap(middle(see("settings", text="Sync now", timeout=30)))  # does: syncNow
        check("Sync now asks the server again", until(lambda: len(requests) > before, 15) is not None, requests[before:])
        # Sync details, from the card's second row: a page of its own, with the runs since the
        # app started, the one that turned sync on and the one just pressed, and never the token,
        # which this address carries as its last part.
        tap(middle(see_named("settings", "Sync details")))
        opened = until(lambda: find("settings", text="Runs since the app started"), 30)
        said = [n.get("text") or "" for n in screen_parts()["settings"].iter("node")] if opened is not None else []
        check("Sync details opens from the card: the state, the runs since the app started, the pressed one among them, and no token",
              opened is not None and "Sync is on" in said and any("Turning sync on" in t for t in said)
              and any("Sync now" in t and t != "Sync now" for t in said) and not any("/sync/phone" in t for t in said), said[:80])
        shot("sync-details")
        adb("shell", "input", "keyevent", "4")  # the back key: key is the device key's size here
        check("the back key goes back to Sync's card",
              until(lambda: find("settings", text="Turn off sync") is not None and find("settings", text="Runs since the app started") is None) is not None)
        stopped()
        before = len(requests)
        launch()
        check("after a restart sync runs again with what was kept: the Keystore opened the device key",
              until(lambda: len(requests) > before, 60) is not None, requests[before:])
        logcat = adb("logcat", "-d")
        trouble = [l for l in logcat.splitlines() if "oskit.Secret" in l and ("Exception" in l or "Error" in l)]
        check("oskit's Secret, the device key, logs no exception", not trouble, trouble[:5])
    finally:
        srv.terminate()
        adb("reverse", "--remove", "tcp:%d" % port) if "port" in locals() else None
        adb("shell", "rm", "-f", "/data/misc/keychain/certs-added/project-b-sync-test.0")


def passkeys_on_the_phone():
    """Our own passkeys on a phone (docs/passkeys.md, Our own passkeys), the only way there.
    With no screen lock, Save says to set one and the page is refused. With a PIN set, Save
    brings Android's own check, the PIN typed into it makes the passkey, and the page reads
    its transport, algorithm and key; signing in shows the passkey as a row, a tap and the
    PIN again sign in, and the page gets its user handle; a no to the check refuses the
    page. The PIN is cleared at the end, whatever happened: a lock left on stalls every
    other run."""
    pin = "1357"

    def locked():
        # With a PIN set, Android answers only to one who knows it.
        return "false" in adb("shell", "locksettings", "get-disabled", "--old", pin).lower()

    def check_person(title):
        """Android's own sheet for a check, titled title, and the PIN typed into it."""
        sheet = until(lambda: system_node(text=title), 20)
        if sheet is not None:
            # Android 10's check is a secure screen, whose picture comes back empty: its
            # words are kept beside it, as Android's tree has them.
            out = adb("exec-out", "uiautomator", "dump", "/dev/tty")
            words = [w for n in ET.fromstring(out[:out.rindex("</hierarchy>") + len("</hierarchy>")]).iter("node") for w in (n.get("text"), n.get("content-desc")) if w]
            with open("%s/passkey-check-%s.txt" % (OUT, title.split()[0].lower()), "w") as f:
                f.write("\n".join(words) + "\n")
            adb("shell", "input", "text", pin)
            key(66)
        return sheet is not None

    adb("shell", "locksettings", "clear", "--old", pin)
    launch()
    go(BASE + "/passkey")
    asked("/passkey")
    served.clear()
    try:
        tap(middle(see("page", text="make")))
        check("a page making a passkey is asked to save it in Project B", see("shell", text="Save a passkey for localhost in Project B?") is not None)
        shot("passkey-save")
        tap(middle(see("shell", text="Save")))
        check("with no screen lock, Save says to set one", see("shell", text="Set a screen lock first") is not None)
        shot("passkey-no-screen-lock")
        tap(middle(see("shell", text="Close")))
        check("and the page is refused", asked("/said?make=" + urllib.parse.quote("refused NotAllowedError")), str(served))

        adb("shell", "locksettings", "set-pin", pin)
        check("a PIN is set for the run", until(locked) is not None, adb("shell", "locksettings", "get-disabled", "--old", pin))
        served.clear()
        tap(middle(see("page", text="make")))
        tap(middle(see("shell", text="Save")))
        check("Save brings Android's own check, named for saving", check_person("Save your passkey"))
        check("the PIN makes the passkey, and the page reads its transport, algorithm and key", asked("/said?make=" + urllib.parse.quote("made internal -7 key")), str(served))

        served.clear()
        tap(middle(see("page", text="use")))
        row = until(lambda: first(lambda: find("shell", text=lambda t: t.startswith("Ada")), lambda: find("shell", label=lambda l: l.startswith("Ada"))), 20)
        check("signing in shows the passkey here as a row, under the site", row is not None and first(lambda: find("shell", text="Sign in to localhost"), lambda: find("shell", label="Sign in to localhost")) is not None)
        shot("passkey-own")
        tap(middle(row))
        check("a tap on it brings the check, named for using", check_person("Use your passkey"))
        check("and the PIN signs in: the page gets its user handle", asked("/said?use=" + urllib.parse.quote("used ada")), str(served))

        served.clear()
        tap(middle(see("page", text="use")))
        tap(middle(until(lambda: first(lambda: find("shell", text=lambda t: t.startswith("Ada")), lambda: find("shell", label=lambda l: l.startswith("Ada"))), 20)))
        until(lambda: system_node(text="Use your passkey"), 20)
        # Android 10 asks for the PIN on a screen with the keyboard up, and the first back
        # only puts the keyboard away.
        for _ in range(2):
            key(4)
            if until(lambda: system_node(text="Use your passkey") is None, 3):
                break
        check("the back key at the check refuses the page", asked("/said?use=" + urllib.parse.quote("refused NotAllowedError")), str(served))
    finally:
        adb("shell", "locksettings", "clear", "--old", pin)
        check("the PIN is cleared after the run", until(lambda: "true" in adb("shell", "locksettings", "get-disabled").lower()) is not None, adb("shell", "locksettings", "get-disabled"))
    go(BASE + "/a")
    asked("/a")


def passwords_on_the_phone():
    """Saved passwords on a phone (docs/passwords.md): they come by sync, as a phone has no
    import. Another device (core/cmd/syncdevice) saves a login and turns sync on with a bucket
    on this Mac's sync server, choosing the passphrase; the phone joins the bucket with it,
    and the Passwords section's Passwords row opens a page that lists the login by site and username, with
    no password, and counts it. With no screen lock Show, on the login's row, says to set
    one. With a PIN set, Show brings Android's own check and the PIN shows the password; Copy,
    the check holding for the page, brings none and says the password was copied. Then
    filling (docs/password-autofill.md): on a sign-in page of this Mac's test site, a tap on
    the password field brings the login sync brought for that site as a chip over the foot
    of the page, and a tap on the chip fills the page's fields, with no check of the person. The PIN is cleared at the end, whatever happened: a lock left on
    stalls every other run."""
    pin = "1357"
    site, username, password, passphrase = "mail.example", "asha", "copper-lantern-42", "river meadow lantern copper"
    stopped = lambda: (adb("shell", "am", "force-stop", PACKAGE), until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME")))

    def locked():
        return "false" in adb("shell", "locksettings", "get-disabled", "--old", pin).lower()

    def check_person():
        """Android's own sheet for the check, and the PIN typed into it."""
        sheet = until(lambda: system_node(text="Show your password"), 20)
        if sheet is not None:
            adb("shell", "input", "text", pin)
            key(66)
        return sheet is not None

    folder = os.path.abspath(os.path.join(OUT, "passwords-syncserver"))
    shutil.rmtree(folder, ignore_errors=True)
    binaries = {}
    for name in ("syncserver", "syncdevice"):
        binaries[name] = os.path.abspath(os.path.join(OUT, name + "-bin"))
        subprocess.run(["go", "build", "-tags", "sqlite_fts5", "-o", binaries[name], "./cmd/" + name], cwd=os.path.join(ROOT, "core"), check=True)
    srv = subprocess.Popen([binaries["syncserver"], "-addr", "127.0.0.1:0", "-dir", folder], stdout=subprocess.PIPE, text=True)
    port = int(srv.stdout.readline().strip().rsplit(":", 1)[1])
    threading.Thread(target=lambda: [None for _ in srv.stdout], daemon=True).start()
    address = "https://127.0.0.1:%d/sync/passwords" % port
    adb("shell", "locksettings", "clear", "--old", pin)
    try:
        # Two logins: one for a site to list in Settings, one for this Mac's test site to fill.
        fill_user, fill_password = "filler@example.org", "fill-me-in-7"
        other = subprocess.run([binaries["syncdevice"], "-address", address, "-ca", os.path.join(folder, "ca.pem"), "-passphrase", passphrase,
                                "-login", "https://%s|%s|%s" % (site, username, password),
                                "-login", "http://%s|%s|%s" % (BASE, fill_user, fill_password)], capture_output=True, text=True, timeout=120)
        check("another device saves two logins and turns sync on with the bucket", "synced" in other.stdout, other.stdout[-300:] + other.stderr[-600:])
        adb("reverse", "tcp:%d" % port, "tcp:%d" % port)
        adb("shell", "mkdir", "-p", "/data/misc/keychain/certs-added")
        adb("push", os.path.join(folder, "ca.pem"), "/data/misc/keychain/certs-added/project-b-sync-test.0")
        # A start past the welcome first, as each part's is; then the app, stopped, reads the
        # device's CA folder again when the link starts it.
        launch()
        stopped()
        view_link("project-b://sync?address=" + urllib.parse.quote(address, safe=""))
        tap(middle(see("settings", text="Connect", timeout=30)))
        see("settings", text="Enter your sync passphrase", timeout=30)
        # The other way in, a backup code, can be scanned on a phone (docs/sync.md, "Backup
        # code"): the scanner's own screen opens over the app, asking for the camera the
        # first time, and the back key leaves it with nothing read. What it reads from a
        # code held up to a real camera is not something an emulator can show.
        tap(middle(see("settings", text="Use backup code instead")))
        tap(middle(see("settings", label="Scan QR code")))  # does: syncBackupScan
        # The scanner's screen comes in front first and asks for the camera from there, so
        # Android's question covers it a moment later: the question is answered whenever it
        # is the one in front, and the scanner counts as open once the app has the camera.
        def scanner():
            front = in_front_now()
            if "GrantPermissions" in front:
                # Android 11 and later ask "While using the app"; Android 10 asks "Allow".
                allow = first(lambda: system_button("While using the app", 2), lambda: system_button("Allow", 2))
                if allow is not None:
                    tap(middle(allow))
                return None
            return "CaptureActivity" in front and "android.permission.CAMERA: granted=true" in adb("shell", "dumpsys", "package", PACKAGE)
        check("Scan QR code, at the field's end, opens the scanner's own screen, once Android's question about the camera is answered",
              until(scanner, 40) is not None, in_front_now())
        # The camera takes a moment to start: then its picture shows, under the scanner's frame.
        time.sleep(4)
        shot("passwords-scanner")
        # A back key pressed while the camera is still starting can be lost (emu-37, once in
        # three runs): once more if the scanner is still in front.
        key(4)
        if until(lambda: "CaptureActivity" not in in_front_now(), 6) is None:
            key(4)
        check("the back key leaves the scanner, with the join step as it was and nothing read",
              until(lambda: in_front() and find("settings", text="Use the passphrase instead") is not None, 20) is not None, in_front_now())
        tap(middle(see("settings", text="Use the passphrase instead")))
        see("settings", text="Enter your sync passphrase", timeout=30)
        # The field, once Android reads the Dialog: its tree is empty while the Dialog comes in.
        tap(middle(until(lambda: next((n for n in screen_parts().get("settings", ET.Element("none")).iter("node") if n.get("class") == FIELD), None), 20)))
        type_text(passphrase)
        # As in sync_on_the_phone: with the keyboard up the tap may only put it away.
        for _ in range(3):
            button = find("settings", text="Turn on sync")
            if button is not None:
                tap(middle(button))
            if until(lambda: find("settings", text="Enter your sync passphrase") is None, 5):
                break
        check("the phone joins the bucket with the passphrase", until(lambda: find("settings", text="Sync now"), 60) is not None)

        # The link opened Settings at Sync, and back from there leaves Settings: from the
        # menu it opens at its list of sections, and Passwords is one.
        key(4)
        until(lambda: "settings" not in screen_parts())
        menu("Settings")
        # A row that opens a page is one button to Android, named by its label, where a row
        # of words is text: both are looked at.
        words = lambda: [w for n in screen_parts().get("settings", ET.Element("none")).iter("node") for w in (n.get("text"), n.get("content-desc")) if w]
        node = lambda test: next((n for n in screen_parts().get("settings", ET.Element("none")).iter("node") if test(n.get("text") or "") or test(n.get("content-desc") or "")), None)
        tap(middle(until(lambda: node(starts("Passwords")), 30)))
        # The list's row for the section starts "Passwords" too, and so does the section's
        # heading: Passkeys shows once inside, and the row is the one that takes a tap.
        inside = until(lambda: node(starts("Passkeys")), 30)
        row = next((n for n in screen_parts().get("settings", ET.Element("none")).iter("node") if n.get("clickable") == "true" and ((n.get("text") or "").startswith("Passwords") or (n.get("content-desc") or "").startswith("Passwords"))), None)
        check("Passwords is a section on a phone, with its Passwords row", inside is not None and row is not None, words()[:60])
        shot("passwords-row")
        tap(middle(row))
        # The login sync brought: a run may still be bringing it as the page opens.
        listed = until(lambda: node(starts(site)), 60)
        said = words()
        check("its page lists the login sync brought, by its site and username, with no password, and counts it",
              listed is not None and any(username in t for t in said) and "2 saved" in said and not any(password in t for t in said) and any(t.startswith("Sealed for this profile; your sync server can't read them.") and "your screen lock" in t for t in said), said[:60])
        shot("passwords-list")
        show = "Show the password for " + site
        tap(middle(see_named("settings", show)))  # does: passwordShow
        check("with no screen lock, Show says to set one", until(lambda: settings_text("Set a screen lock on this device to show passwords"), 20) is not None)
        shot("passwords-no-screen-lock")

        adb("shell", "locksettings", "set-pin", pin)
        check("a PIN is set for the run", until(locked) is not None, adb("shell", "locksettings", "get-disabled", "--old", pin))
        tap(middle(see_named("settings", show)))
        check("with one, Show brings Android's own check", check_person())
        check("and the PIN shows the password", until(lambda: settings_text(password), 20) is not None)
        shot("passwords-shown")
        tap(middle(see_named("settings", "Copy the password for " + site)))  # does: passwordCopy
        copied = until(lambda: settings_text("Password copied"), 20)
        check("Copy, the check holding for the page, says the password was copied and brings no second check",
              copied is not None and system_node(text="Show your password") is None)

        # Filling, with the PIN still set: a fill asks for no check.
        launch()
        go(BASE + "/login")
        asked("/login")
        served.clear()
        inputs = until(lambda: [n for n in screen_parts().get("page", ET.Element("none")).iter("node") if n.get("class") == FIELD] or None, 20) or []
        check("the sign-in page shows its two fields", len(inputs) == 2, len(inputs))
        tap(middle(inputs[1]))
        chip = until(lambda: first(lambda: find("shell", text=fill_user), lambda: find("shell", label=fill_user)), 20)
        check("a tap on the password field brings the site's login as a chip, outside the page", chip is not None,
              [w for n in screen_parts().get("shell", ET.Element("none")).iter("node") for w in (n.get("text"), n.get("content-desc")) if w][:40])
        shot("passwords-fill-chip")
        # A tap in the bar's first half second is no pick; and the chip is tapped where it
        # is once the keyboard has stopped moving the bar, as a finger taps: down a moment,
        # in which the keyboard goes and the bar with it.
        time.sleep(0.8)
        chip = still(lambda: first(lambda: find("shell", text=fill_user), lambda: find("shell", label=fill_user)))
        finger(middle(chip))
        check("a tap on the chip fills the username and the password into the page", asked("/said?filled=" + urllib.parse.quote(fill_user + " " + fill_password)), str(served))
        check("and nothing asked for the PIN", system_node(text="Show your password") is None)
        shot("passwords-filled")

        # Saving: a sign-in typed with a login that isn't saved, on a page whose form goes
        # through. Once the tab has gone on, the shell's sheet asks; Save keeps the login,
        # which the sign-in page's chips then offer and fill.
        save_user, save_password = "saver@example.org", "save-me-in-9"
        fields = lambda: until(lambda: [n for n in screen_parts().get("page", ET.Element("none")).iter("node") if n.get("class") == FIELD] or None, 20) or []
        go(BASE + "/onward")
        asked("/onward")
        tap(middle(fields()[0]))
        until(keyboard_shown, 10)
        type_text(save_user)
        # The page may have moved under the keyboard: its fields are read again.
        tap(middle(fields()[1]))
        time.sleep(1)
        type_text(save_password)
        # Android 10's keyboard once made it "saver@example.orgs" as the focus moved on, and the
        # page saved what it held: the username is read back, and typed again from empty.
        if fields()[0].get("text") != save_user:
            tap(middle(fields()[0]))
            adb("shell", "input", "keyevent", "123", *["67"] * (len(save_user) + 10))
            type_text(save_user)
            tap(middle(fields()[1]))
            time.sleep(1)
        served.clear()
        tap(middle(see("page", text="Go on")))
        check("the sign-in's button sends its form, and the tab goes on", asked("/signed-in"), str(served))
        sheet = until(lambda: named("shell", "Save password?"), 30)
        words = [w for n in screen_parts().get("shell", ET.Element("none")).iter("node") for w in (n.get("text"), n.get("content-desc")) if w]
        check("once the page has loaded, the shell's sheet asks to save, with the username", sheet is not None and save_user in words, words[:40])
        check("and the typed password is nowhere in the shell", not any(save_password in w for w in words))
        shot("passwords-save-sheet")
        tap(middle(see("shell", text="Save")))  # does: passwordSaveAnswer
        check("Save takes the sheet away", until(lambda: named("shell", "Save password?") is None, 20) is not None)
        go(BASE + "/login")
        asked("/login")
        served.clear()
        tap(middle(fields()[1]))
        chip = until(lambda: first(lambda: find("shell", text=save_user), lambda: find("shell", label=save_user)), 20)
        check("the saved login is a chip at the site's sign-in", chip is not None,
              [w for n in screen_parts().get("shell", ET.Element("none")).iter("node") for w in (n.get("text"), n.get("content-desc")) if w][:40])
        time.sleep(0.8)
        chip = still(lambda: first(lambda: find("shell", text=save_user), lambda: find("shell", label=save_user)))
        shot("passwords-saved-chip")
        finger(middle(chip))
        check("and fills with the password that was saved", asked("/said?filled=" + urllib.parse.quote(save_user + " " + save_password)), str(served))
        shot("passwords-saved-filled")
    except TimeoutError:
        stopped_here("passwords_on_the_phone")
        raise
    finally:
        adb("shell", "locksettings", "clear", "--old", pin)
        check("the PIN is cleared after the run", until(lambda: "true" in adb("shell", "locksettings", "get-disabled").lower()) is not None, adb("shell", "locksettings", "get-disabled"))
        srv.terminate()
        adb("reverse", "--remove", "tcp:%d" % port)
        adb("shell", "rm", "-f", "/data/misc/keychain/certs-added/project-b-sync-test.0")
        # The scanner was allowed the camera: taken back, as a reinstall over the app can
        # keep the grant, and a later run would then see no question about it.
        adb("shell", "pm", "revoke", PACKAGE, "android.permission.CAMERA")


def the_page_renderer_dies():
    # With every app's log open to it, as adb grants on a phone we develop on, its report
    # holds the killed renderer's own lines, and its app sections the app's alone. Granted
    # before the start: Android ends an app that gets READ_LOGS, which gives it the log group.
    adb("shell", "pm", "grant", PACKAGE, "android.permission.READ_LOGS")
    launch()
    go(BASE + "/a")
    asked("/a")
    # The page shown, not only asked for: a new tab starts its renderer only as it loads, so
    # a kill between the request and the commit left a crashed tab with no address, which
    # Reload can't bring back (Android 10, 0.2.95's candidate).
    see("page", label="to b")
    # Every renderer but the shell's, the first: content may keep a spare one, and killing
    # only the newest then killed the spare, and the page never crashed (a flake on emu-37).
    before = renderers()
    for pid in before[1:]:
        adb("shell", "kill", "-9", str(pid))
    marked = status(url("/a"), "Crashed")
    check("a page whose renderer died is marked Crashed, and the app lives", marked and alive(),
          "marked %s, alive %s, renderers %s killed %s now %s" % (bool(marked), alive(), before, before[1:], renderers()))
    shot("16-after-page-renderer-kill")
    # Its report says how it ended: content's word for a renderer that died while in use, and
    # the page's origin, not its address. Before, it said "exited abnormally", next to the
    # whole address (Sudipta's page crash on Google, 2026-11-26).
    reports = lambda: set(re.findall(r"project-b-report-[\w-]+\.txt", adb("shell", "ls", "/sdcard/Download/")))
    had = reports()
    tap(middle(see("shell", text="Save report")))
    # From Android 13 an app that reads every app's log asks the person first, once a run.
    sdk = int(adb("shell", "getprop", "ro.build.version.sdk").strip() or 0)
    if sdk >= 33:
        consent = system_button("Allow one-time access", 10)
        if consent is not None:
            tap(middle(consent))
    saved = until(lambda: (reports() - had) or None, 20)
    report = until(lambda: adb("shell", "cat", "/sdcard/Download/" + sorted(saved)[-1]) or None, 10) if saved else None
    children = (report or "").split("== Processes that crashed (crashes.log)")[-1].split("\n==")[0]
    check("a page crash's report says the renderer ended while in use, with its page's origin only",
          any("renderer ended while in use" in l and l.endswith(", " + url("/a").rsplit("/", 1)[0]) for l in children.splitlines()),
          children.strip()[-400:] or "no report saved")
    section = lambda title: (report or "").split("== " + title)[-1].split("\n==")[0] if "== " + title in (report or "") else ""
    ended = section("The children that crashed in this run")
    killed = before[1:]
    check("with READ_LOGS granted, it has the killed renderer's own lines, by its pid",
          any(re.search(r"\(\s*%d\):" % pid, ended) for pid in killed), ended.strip()[:400] or "no such section")
    system = adb("shell", "pidof", "system_server").strip()
    own = section("The app's log, its end")
    check("and its app's log holds the app's lines alone, none of Android's own",
          own.strip() != "" and system != "" and not re.search(r"\(\s*%s\):" % system, own), "system_server is %s" % system)
    served.clear()
    menu("Reload")
    check("reload recovers after a crash", asked("/a"), str(served))
    shot("17-reloaded-after-crash")
    adb("shell", "pm", "revoke", PACKAGE, "android.permission.READ_LOGS")


def the_shell_renderer_dies():
    """It dies alone: the shell comes back, and the page stays."""
    launch()
    go(BASE + "/a")
    asked("/a")
    before = renderers()
    adb("shell", "kill", "-9", str(before[0]))
    # The shell's renderer gone and one there for it: content may hand the shell its spare,
    # already running, so a renderer new to the list isn't required (a flake on emu-37).
    again = until(lambda: before[0] not in renderers() and len(renderers()) > 0)
    check("the shell gets a new renderer, and the app lives", again and alive(),
          "new renderer %s, alive %s, renderers %s killed %s now %s" % (bool(again), alive(), before, before[0], renderers()))
    shot("17b-after-shell-renderer-kill")
    if not alive():
        return
    served.clear()
    menu("Reload")
    check("the shell works again after its renderer died", asked("/a"), str(served))


def the_gpu_process_dies():
    launch()
    go(BASE + "/a")
    asked("/a")
    privileged = lambda: {pid for pid, name in children() if "privileged" in name}
    before = privileged()
    for pid in before:
        adb("shell", "kill", "-9", pid)
    until(lambda: privileged() - before)
    shot("17c-after-gpu-kill")
    served.clear()
    menu("Reload")
    check("the app works after the GPU process died", asked("/a"), str(served))
    shot("17d-reload-after-gpu-kill")


def a_page_spins():
    """The shell and the page have separate renderers, so a page that spins leaves the shell working."""
    launch()
    go(BASE + "/busy")
    asked("/busy")
    tap(page_top())
    try:
        type_in_bar("xyz")
        check("the address field takes typing while a page spins", True)
    except TimeoutError as e:
        check("the address field takes typing while a page spins", False, e)
    shot("18-page-spinning")


def web_apis():
    """A page that calls each web API whose browser side is Chrome's keeps its renderer, and
    each call gives what the web-features registry says it gives on Android."""
    launch()
    adb("logcat", "-c")
    go(BASE + "/apis")
    for when in ("load", "tap"):
        if when == "tap":
            # The page's call to make a passkey asked to save one, and the dialog waits for a
            # person over the page: Cancel, or it takes the tap.
            cancel = until(lambda: first(lambda: find("shell", text="Cancel"), lambda: find("shell", label="Cancel")), 5)
            if cancel is not None:
                tap(middle(cancel))
            check("its call to make a passkey asks to save one, and Cancel takes the dialog away",
                  cancel is not None and until(lambda: find("shell", text="Save") is None and find("shell", label="Save") is None, 5) is not None)
            tap(middle(see("page")))
        lived = asked("/apis-alive?" + when, 30)
        # Content says which interface it found no binder for.
        killed = [line.split("Received bad user message: ", 1)[-1] for line in adb("logcat", "-d").splitlines()
                  if "bad Mojo message" in line]
        on = "a tap" if when == "tap" else "load"
        check("a page that calls %d web APIs on %s keeps its renderer" % (len(WEB_APIS), on),
              lived and not killed, "; ".join(killed) or ("" if lived else "no word from the page after the calls"))
        if not lived:
            return
        ran = next(s for s in served if s.startswith("/apis-ran?%s&seen=" % when))
        seen = json.loads(urllib.parse.unquote(ran.split("&seen=", 1)[1]))
        wrong = [web_features.mismatch(row, probe, "android", "killed" if killed else seen[probe["name"]], when)
                 for row, probe in REGISTRY_APIS]
        wrong = [w for w in wrong if w]
        check("and its %d calls of the web-features registry give what it says for Android, on %s" % (len(REGISTRY_APIS), on),
              not wrong, "; ".join(wrong))
        unknown = ["%s (%s): %s" % (row["id"], probe["name"], seen[probe["name"]])
                   for row, probe in REGISTRY_APIS if row["status"]["android"] == "unknown"]
        if unknown:
            print("  not yet known on Android, seen on %s: %s" % (on, "; ".join(unknown)), flush=True)
    shot("18b-web-apis")
    # A fresh start: the tap's calls can leave a dialog of theirs over the page, and the
    # address field out of reach.
    launch()
    since = len(served)
    go(BASE + "/usermedia")
    lived = until(lambda: any(s.startswith("/usermedia-alive") for s in served[since:]), 30)
    known = [s for s in served[since:] if s.startswith("/usermedia-alive")]
    check("a page with a <usermedia> element keeps its renderer, as Google's search page does", lived is not None and known[-1].endswith("?function"),
          known[-1] if known else "no word from the page")


def popups():
    """Pop-ups (docs/site-features.md, Pop-ups): with the site's Pop-ups at Block, as every
    site starts, a tab a page opens with no tap first closes as it opens, its page never asked,
    and the pill counts it; one a tap opens, opens. The site sheet's Open it opens the
    blocked one, and its Pop-ups set to Allow lets the next one open."""
    launch()
    del served[:]
    go(BASE + "/popups")
    asked("/popups")
    counted = until(lambda: find("shell", label="1 pop-up blocked"), 10)
    time.sleep(1)
    check("a pop-up a page opens with no tap closes as it opens, its page never asked, and the pill counts it",
          counted is not None and not asked("/popped-untapped", 0.5),
          "counted=%s served=%s" % (counted is not None, served[-4:]))

    button = until(lambda: first(lambda: find("page", text="tap to open"), lambda: find("page", label="tap to open")), 10)
    if button is not None:
        tap(middle(button))
    check("a tab a tap opens, opens", asked("/popped-tapped", 10), served[-4:])
    # Back to the page that opened it: the new tab's Back closes it, as a phone's tab does.
    press("Tabs")
    tap(middle(see("shell", label="Close Popped")))
    key(4)
    until(lambda: find("shell", label="1 pop-up blocked"), 10)

    count = find("shell", label="1 pop-up blocked")
    if count is not None:
        tap(middle(count))
    once = until(lambda: find("shell", text="Open it"), 5)
    if once is not None:
        tap(middle(once))
    check("the count opens the site sheet, whose Open it opens the pop-up in a tab",
          once is not None and asked("/popped-untapped", 10), served[-4:])
    shot("45-popups-open-once")

    go(BASE + "/popups?again")
    asked("/popups?again")
    count = until(lambda: find("shell", label="1 pop-up blocked"), 10)
    if count is not None:
        tap(middle(count))
    # The sheet's last Allow is Pop-ups': its row comes after Links to other sites.
    allows = lambda: [n for n in screen_parts().get("shell", ET.Element("x")).iter("node") if n.get("text") == "Allow"]
    allow = until(lambda: allows()[-1] if len(allows()) >= 2 else None, 5)
    if allow is not None:
        tap(middle(allow))
    sites = "/data/data/%s/%s/sites.json" % (PACKAGE, PROFILE)

    def kept():
        try:
            return json.loads(adb("shell", "cat", sites) or "{}").get("http://" + BASE, {}).get("popups")
        except ValueError:
            return None
    check("the site sheet's Pop-ups set to Allow, Go keeps it for the site", allow is not None and until(lambda: kept() == "allow", 10),
          "allow=%s kept=%s" % (allow is not None, kept()))
    key(4)
    del served[:]
    go(BASE + "/popups?allowed")
    check("with the site's Pop-ups at Allow, the pop-up opens", asked("/popped-untapped", 10), served[-4:])
    adb("shell", "am", "force-stop", PACKAGE)


def links_to_other_sites():
    """Links to other sites (docs/site-features.md#checking-it), set to Block from the site's
    sheet: each of the ten ways a page leaves for another site goes nowhere, the page staying
    with its count risen, one tab and nothing in history, and the address pill counting the tabs
    it closed; an address typed still goes; under Allow every way goes."""
    launch()
    go(BASE + "/leave")
    asked("/leave")
    other = "127.0.0.1"
    sites = "/data/data/%s/%s/sites.json" % (PACKAGE, PROFILE)
    origin = "http://" + BASE

    def kept():
        try:
            return json.loads(adb("shell", "cat", sites) or "{}").get(origin, {}).get("links", "allow")
        except ValueError:
            return "allow"

    def set_links(value):
        # The site button is named Site settings, and "Not secure · Site settings" on an
        # http page such as the test server's.
        tap(middle(see("shell", label=lambda label: (label or "").endswith("Site settings"))))
        tap(middle(see("shell", text=value.capitalize())))
        if until(lambda: kept() == value, 10) is None:
            raise TimeoutError("Go never kept links=%s for %s" % (value, origin))
        key(4)
        until(lambda: find("shell", text=value.capitalize()) is None)

    def shown(words):
        """The page's node with these words, as its text or its label. A node with no
        children is false, so each is tested against None."""
        node = find("page", text=words)
        return node if node is not None else find("page", label=words)

    def way(words):
        return until(lambda: shown(words))

    def reached():
        return [x for x in seen if x.startswith(other + "/")]

    def tabs():
        """How many tabs the switcher shows, by its cards' close buttons: the Tabs button's
        count isn't in the accessibility tree, its label is."""
        press("Tabs")
        closes = lambda: [n for n in screen_parts().get("shell", ET.Element("x")).iter("node")
                          if (n.get("content-desc") or "").startswith("Close ")]
        until(closes, 5)
        count = len(closes())
        key(4)
        until(lambda: find("shell", label=lambda l: (l or "").startswith("Close ")) is None, 5)
        return count

    set_links("block")
    shot("40-links-block-sheet")
    del seen[:]
    blocked = 0
    for n, words, opens, paths in links_page.WAYS:
        node = way(words)
        if node is None:
            raise TimeoutError("the page never showed %r" % words)
        tap(middle(node))
        alive = until(lambda: "localhost/alive?%d" % n in seen, 10)
        if opens:
            blocked += 1
            until(lambda: find("shell", label=links_page.popups_word(blocked)), 5)
        count = until(lambda: shown("clicks %d" % n), 3)
        check("under Block, %s goes nowhere: the page stays, its count risen, one tab, the other site never asked%s"
              % (words, ", and the pill says " + links_page.popups_word(blocked) if opens else ""),
              alive and count is not None and tabs() == 1 and not reached()
              and (not opens or find("shell", label=links_page.popups_word(blocked)) is not None),
              "alive=%s count=%s tabs=%s reached=%s" % (bool(alive), count is not None, tabs(), reached()))
    shot("41-links-blocked")
    check("under Block, nothing of the other site went into history", not any(other in u for u in history()))
    go("%s:%d/typed" % (other, PORT))
    check("under Block, an address typed still goes", until(lambda: "%s/typed" % other in seen, 15))

    go(BASE + "/leave")
    asked("/leave")
    set_links("allow")
    for n, words, opens, paths in links_page.WAYS:
        go(BASE + "/leave")
        node = way(words)
        if node is None:
            raise TimeoutError("the page never showed %r" % words)
        del seen[:]
        tap(middle(node))
        went = until(lambda: all(other + p in [x.split("?")[0] for x in seen] for p in paths), 10)
        check("under Allow, %s goes to the other site" % words, went, reached())


def the_real_web():
    """Over the office's 443-only egress."""
    launch()
    go("example.com")
    check("a real https page joins history", until(lambda: visited("https://example.com/"), 30))
    shot("19-real-web-example")


def secure_dns():
    """Secure DNS on the phone (docs/secure-dns.md): Secure only with a provider no one can
    reach, from prefs.json at the start, and a real page doesn't load, its name looked up
    nowhere else; set back to Off, the next one does. Over the office's 443-only egress, as
    the_real_web."""
    prefs = profile_json("prefs.json")
    try:
        profile_file("prefs.json", {**prefs, "secureDns": "secure", "dnsProvider": "custom",
                                    "dnsCustom": "https://127.0.0.1:9/dns-query{?dns}"})
        launch()
        go("example.net")
        time.sleep(15)
        check("Secure only, with a provider that can't be reached, a real page doesn't load", not visited("https://example.net/"),
              str(list(history())[-3:]))
        shot("19b-secure-dns-unreachable")
        profile_file("prefs.json", {**prefs, "secureDns": "off"})
        launch()
        go("example.org")
        check("set back to Off, the network's DNS looks it up and it loads", until(lambda: visited("https://example.org/"), 30))
    finally:
        profile_file("prefs.json", prefs)


# The block lists (docs/blocking.md), as tools/check_basics.py checks them on the desktop (28
# and 42 to 48). Our test lists' hosts are all under blocking.test, RFC 6761's .test. A phone
# takes no switches, so no --host-resolver-rules: while the test lists are checked, the phone's
# own proxy is this server, through the adb reverse, and the server answers every host. The
# browser goes to localhost direct, as it never proxies it, and Go fetches the lists itself,
# not through the browser, so the proxy touches neither.
LISTED = "blocking.test"
with open(os.path.join(ROOT, "core/blocking/catalog.json")) as _f:
    CATALOGUE = json.load(_f)
_BLOCKER = {g["id"]: g["blocker"] for g in CATALOGUE["groups"]}
# The person's choices, as prefs.json keeps them: Block ads and Warn before dangerous sites on,
# the test group's lists on and every other list off, so only ours are fetched.
TEST_LISTS = {"blockAds": True, "warnDangerous": True,
              "lists": {l["id"]: l["group"] == "test" for l in CATALOGUE["lists"]}}
# What each of them loads as: the network blocker's group test, and each safe list on its own.
TEST_UNITS = {"test"} | {l["id"] for l in CATALOGUE["lists"] if l["group"] == "test" and l["id"].endswith(("-malware", "-phishing"))}
# What a new profile's lists load as, leaving out those that go by the phone's languages:
# each network group with a list on, and each safe list on.
DEFAULT_NETWORK = {l["group"] for l in CATALOGUE["lists"] if _BLOCKER[l["group"]] == "network" and l.get("on") and not l.get("languages")}
DEFAULT_SAFE = {l["id"] for l in CATALOGUE["lists"] if _BLOCKER[l["group"]] == "safe" and l.get("on") and not l.get("languages")}
TIMELINE = "/data/data/%s/%s/timeline.log" % (PACKAGE, PROFILE)

PAGES["/listed-images"] = ("<title>Images</title>"
                           "<img width=96 height=64 src='http://ads.%s/pic/pb-test-ad.png'>"
                           "<img width=96 height=64 src='http://allowed.ads.%s/pic/pb-test-allowed.png'>" % (LISTED, LISTED))
PAGES["/hide"] = "<title>Hide</title><p class=pb-test-ad style='font-size:40px'>an ad<p style='font-size:40px'>not an ad"
PAGES["/unsafe"] = "<title>Unsafe</title><p style='font-size:40px'>a page the dangerous sites' test lists name"


def blocking_lines():
    """The lists manager's lines in this start's timeline (docs/logging.md), as (kind, text)."""
    out = adb("shell", "cat", TIMELINE)
    return [(m.group(1), m.group(2)) for m in re.finditer(r"^\s*[\d.]+ (\w+)\s+go\s+blocking: (.*)$", out, re.M)]


def every_two_seconds(condition, timeout):
    """until's, asking only every two seconds: each ask copies the whole timeline off the phone."""
    deadline = time.time() + timeout
    while True:
        value = condition()
        if value is not None and value is not False:
            return value
        if time.time() > deadline:
            return None
        time.sleep(2)


def lists_ready(units, timeout):
    """Waits for each of units to be compiled or loaded, as Go says in the timeline, and
    returns the lists manager's lines then. A TimeoutError names those that came, and the
    errors."""
    def ready():
        lines = blocking_lines()
        have = {m.group(1) for _, t in lines for m in [re.match(r"(\S+) (compiled from|loaded from) ", t)] if m}
        return lines if units <= have else None
    lines = every_two_seconds(ready, timeout)
    if lines is None:
        lines = blocking_lines()
        have = sorted({t.split()[0] for _, t in lines if " compiled from " in t or " loaded from " in t})
        errors = [t for k, t in lines if k == "error"]
        raise TimeoutError("the lists didn't load in %d s: wanted %s, loaded %s, errors %s" % (timeout, sorted(units), have, errors[-3:]))
    # Go hands the safe lists to the navigation gate once the pass that loaded them ends.
    time.sleep(1)
    return lines


def compiled(lines):
    """Each unit compiled, with its lists, time and engine's bytes, from the lines."""
    return [m.groups() for _, t in lines for m in [re.match(r"(\S+) compiled from (\d+) lists in (\S+), (\d+) bytes", t)] if m]


def app_memory():
    """The app's main process, from dumpsys meminfo, in kB: its total PSS, its native heap's
    PSS, and what malloc holds there (the native heap's Heap Alloc, the row's last but one on
    every Android)."""
    pid = next((p for p, name in processes() if name == PACKAGE), None)
    if pid is None:
        raise TimeoutError("the app isn't running to be measured")
    out = adb("shell", "dumpsys", "meminfo", pid)
    total = re.search(r"TOTAL(?: PSS)?:\s+(\d+)", out)
    native = re.search(r"Native Heap:\s+(\d+)", out)
    row = re.search(r"^\s*Native Heap((?:\s+\d+)+)\s*$", out, re.M)
    if not (total and native and row):
        raise TimeoutError("dumpsys meminfo %s said no totals: %s" % (pid, out[:300]))
    return {"pss": int(total.group(1)), "native": int(native.group(1)), "alloc": int(row.group(1).split()[-2])}


def said_memory(when, m):
    print("info: blocking memory, %s: total PSS %.1f MB, native heap PSS %.1f MB, malloc'd %.1f MB"
          % (when, m["pss"] / 1024, m["native"] / 1024, m["alloc"] / 1024), flush=True)


def lists_checked_again():
    """Waits for the first check of the lists after the start-up wait, which finds the
    manifest unchanged or takes a new one, so each measure is taken at the same point."""
    return every_two_seconds(lambda: any(t == "the manifest hasn't changed" or " compiled from " in t for _, t in blocking_lines()), 120)


def blocking():
    """The block lists on the phone. With our test lists on: an image from ads.blocking.test
    isn't fetched and the site's sheet counts it, while its exception's image is; on
    hide.blocking.test .pb-test-ad is hidden; malware.blocking.test is refused before it is
    asked, with the unsafe notice's Go back and Open anyway, and opened anyway, the address
    says Dangerous. Then, from a new profile's own lists, what they cost: the compile times,
    and the main process's memory with Warn before dangerous sites on and off, as info."""
    if adb("shell", "settings", "get", "secure", "enabled_accessibility_services").strip() not in ("", "null"):
        talkback(False)
    try:
        clear_data()
        adb("shell", "settings", "put", "global", "http_proxy", "127.0.0.1:%d" % PORT)
        try:
            test_lists_on_the_phone()
        finally:
            adb("shell", "settings", "put", "global", "http_proxy", ":0")
        default_lists_memory()
    except TimeoutError:
        # Kept before the data is cleared, which stops the app.
        STOPPED["blocking"] = keep_evidence("stopped-blocking", "blocking")
        raise
    finally:
        adb("shell", "settings", "put", "global", "http_proxy", ":0")
        adb("shell", "cmd", "statusbar", "collapse")
        # The lists, the test prefs and the app with them: the next run starts as main leaves it.
        clear_data()


def test_lists_on_the_phone():
    profile_file("prefs.json", TEST_LISTS)
    launch()
    lists_ready(TEST_UNITS, 180)

    del seen[:]
    go(BASE + "/listed-images")
    until(lambda: "localhost/listed-images" in seen)
    allowed = until(lambda: any(s.startswith("allowed.ads.%s/" % LISTED) for s in seen), 15)
    ads = [s for s in seen if s.startswith("ads.%s/" % LISTED)]
    check("with the test lists on, an image from ads.%s isn't fetched, and its exception's from allowed.ads.%s is" % (LISTED, LISTED),
          allowed and not ads, "allowed fetched %s, ads fetched %s" % (bool(allowed), ads))
    # The site's sheet says the tab's count. The site button is "Not secure · Site settings"
    # on the test server's http page.
    tap(middle(see("shell", label=lambda label: (label or "").endswith("Site settings"))))
    counted = lambda: (lambda m: int(m.group(1)) if m else 0)(re.search(r"(\d+) blocked on this page", shown_words("shell")))
    until(lambda: counted() >= 1, 10)
    shot("blocking-site-sheet")
    count = counted()
    check("the site's sheet counts the blocked image", count >= 1, "it says %d blocked" % count)
    key(4)
    until(lambda: "blocked on this page" not in shown_words("shell"), 5)

    # On the test server the same paragraph shows, so the tree is seen to carry it.
    go(BASE + "/hide")
    shows = until(lambda: find("page", text="an ad") is not None and find("page", text="not an ad") is not None, 15)
    check("on localhost, .pb-test-ad shows", shows, shown_words("page")[:200])
    del seen[:]
    go("http://hide.%s/hide" % LISTED)
    until(lambda: "hide.%s/hide" % LISTED in seen)
    hidden = until(lambda: find("page", text="not an ad") is not None and find("page", text="an ad") is None, 15)
    shot("blocking-hidden")
    check("on hide.%s, .pb-test-ad is hidden and the plain paragraph shows" % LISTED, hidden,
          "asked %s; the page says %s" % (seen[-3:], shown_words("page")[:200]))

    del seen[:]
    go(BASE + "/a")
    until(lambda: "localhost/a" in seen)
    del seen[:]
    unsafe = "malware.%s" % LISTED
    go("http://%s/unsafe" % unsafe)
    said = until(lambda: find("shell", text=lambda t: bool(t) and (unsafe + " may harm your device") in t), 15)
    back, anyway = named("shell", "Go back"), named("shell", "Open anyway")
    shot("blocking-unsafe-notice")
    reached = [s for s in seen if s.startswith(unsafe)]
    check("%s is refused before it is asked, with the unsafe notice, Go back and Open anyway" % unsafe,
          said is not None and back is not None and anyway is not None and not reached,
          "notice %s, Go back %s, Open anyway %s, the server was asked for %s; the shell says %s"
          % (said is not None, back is not None, anyway is not None, reached, shown_words("shell")[:240]))
    if anyway is None:
        return
    tap(middle(anyway))
    opened = until(lambda: any(s.startswith(unsafe + "/unsafe") for s in seen) and named("shell", "Open anyway") is None
                   and find("page", text=lambda t: bool(t) and "test lists name" in t) is not None, 15)
    marked = until(lambda: find("shell", label=lambda label: (label or "").startswith("Dangerous")), 10)
    shot("blocking-opened-anyway")
    check("Open anyway opens it, and the address says Dangerous", opened and marked is not None,
          "opened %s; the shell says %s" % (bool(opened), shown_words("shell")[:240]))


def default_lists_memory():
    """What a new profile's lists cost: no prefs, so Block ads and Warn before dangerous sites
    are on, with the catalogue's lists. Measured once they compiled, then from a start that
    loads them from disk, as every later start does, and from one with Warn before dangerous
    sites off, so the safe lists' share shows. Only errors fail."""
    clear_data()
    launch()
    lines = lists_ready(DEFAULT_NETWORK | DEFAULT_SAFE, 300)
    for name, lists, took, size in compiled(lines):
        print("info: blocking %s compiled from %s lists in %s, %.1f MB" % (name, lists, took, int(size) / 1048576), flush=True)
    time.sleep(5)
    said_memory("just compiled", app_memory())
    errors = [t for k, t in lines if k == "error"]

    launch()
    lines = lists_ready(DEFAULT_NETWORK | DEFAULT_SAFE, 60)
    if lists_checked_again() is None:
        print("note: the lists weren't checked again in 120 s; measured anyway", flush=True)
    time.sleep(5)
    on = app_memory()
    said_memory("loaded from disk, Warn before dangerous sites on", on)
    errors += [t for k, t in blocking_lines() if k == "error"]

    profile_file("prefs.json", {**profile_json("prefs.json"), "warnDangerous": False})
    launch()
    lists_ready(DEFAULT_NETWORK, 60)
    if lists_checked_again() is None:
        print("note: the lists weren't checked again in 120 s; measured anyway", flush=True)
    time.sleep(5)
    off = app_memory()
    said_memory("loaded from disk, Warn before dangerous sites off", off)
    lines = blocking_lines()
    errors += [t for k, t in lines if k == "error"]
    safe = sorted({t.split()[0] for _, t in lines if t.split()[0] in DEFAULT_SAFE and " loaded from " in t})
    print("info: blocking memory, the safe lists' share: total PSS %+.1f MB, native heap PSS %+.1f MB, malloc'd %+.1f MB%s"
          % ((on["pss"] - off["pss"]) / 1024, (on["native"] - off["native"]) / 1024, (on["alloc"] - off["alloc"]) / 1024,
             "; yet with it off these still loaded: %s" % safe if safe else ""), flush=True)
    check("a new profile's lists compile and load with no errors", not errors, "; ".join(errors[:3]))


def long_press_menu():
    """A long press on a link, a picture and a linked picture, in both Mobile shells, the web
    one and the native one: the dense sheet's tiles, Download putting the file in the phone's
    Downloads at once, Share opening Android's chooser, Copy image putting the picture itself on
    the clipboard for a page to paste, and a linked picture offering the link's tiles and the
    picture's."""
    held = lambda name: long_press(middle(see_named("page", name)))
    tiles = lambda *names: all(named("shell", n) is not None for n in names)
    # Android's own question at the first download would cover the sheet.
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    for shell in ("mwshell", "mkshell"):
        use_shell(shell)
        # Through the media index, as a plain rm leaves its rows, and a download of the same name
        # then can't be published.
        adb("shell", "content delete --uri content://media/external/downloads --where \"_display_name LIKE 'pb-press-%'\"")
        # The app's own list still has the first shell's downloads, so the second shell's of
        # the same name is "pb-press-link (1).txt" (core/downloads, free): either name will do.
        landed = lambda stem, ext: re.search(r"%s( \(\d+\))?\.%s" % (stem, ext), adb("shell", "ls", "/sdcard/Download/"))
        try:
            launch()
            go(BASE + "/press")
            see("page", text="The press file")

            held("The press file")
            check(shell + ": a long press on a link shows New tab, In group, Copy link, Download and Share as tiles, and Copy link text",
                  until(lambda: tiles("New tab", "In group", "Copy link", "Download", "Share", "Copy link text")) is not None)
            shot("47-long-press-link-" + shell)
            tap(middle(see_named("shell", "Download")))
            check(shell + ": Download puts the file in the phone's Downloads at once, from its page, as a site that serves its files only to its own pages wants",
                  until(lambda: landed("pb-press-link", "txt"), 20) is not None, "refused: %s" % refused)
            held("The press file")
            tap(middle(see_named("shell", "Share")))
            check(shell + ": Share opens Android's chooser",
                  until(lambda: "Window{" in in_front_now() and "BrowserActivity" not in in_front_now(), 10) is not None, in_front_now())
            key(4)
            until(lambda: "BrowserActivity" in in_front_now(), 10)

            held("A picture")
            check(shell + ": a long press on a picture shows New tab, Copy image, Download and Share, and Copy image address",
                  until(lambda: tiles("New tab", "Copy image", "Download", "Share", "Copy image address")) is not None
                  and named("shell", "In group") is None)
            time.sleep(1)  # Its picture, once the engine has fetched it.
            shot("48-long-press-picture-" + shell)
            tap(middle(see_named("shell", "Copy image")))
            until(lambda: named("shell", "Copy image") is None, 5)
            tap(middle(see("page", text="paste here")))
            time.sleep(1)
            before = len(served)
            key(279)  # Paste.
            check(shell + ": Copy image puts the picture itself on the clipboard, which a page pastes as a PNG",
                  until(lambda: any(re.match(r"/pasted\?type=image/png&size=[1-9]", x) for x in served[before:]), 10) is not None,
                  str([x for x in served[before:] if x.startswith("/pasted")]))
            key(4)  # The keyboard.
            held("A picture")
            tap(middle(see_named("shell", "Download")))
            check(shell + ": the picture's Download puts it in the phone's Downloads, from its page",
                  until(lambda: landed("pb-press-pic", "png"), 20) is not None, "refused: %s" % refused)
            held("A picture")
            tap(middle(see_named("shell", "Share")))
            check(shell + ": the picture's Share opens Android's chooser",
                  until(lambda: "Window{" in in_front_now() and "BrowserActivity" not in in_front_now(), 10) is not None, in_front_now())
            key(4)
            until(lambda: "BrowserActivity" in in_front_now(), 10)

            held("A linked picture")
            check(shell + ": a long press on a linked picture offers the link's tiles and the picture's",
                  until(lambda: tiles("In group", "Copy link", "Copy image", "Copy image address")) is not None)
            time.sleep(1)
            shot("49-long-press-linked-" + shell)
            small = small_targets()
            check(shell + ": every control of the sheet is 48dp or more each way", not small, ", ".join(small))
            key(4)
            check(shell + ": the back key closes it", until(lambda: named("shell", "Copy image") is None, 5) is not None)
        except TimeoutError:
            stopped_here("long_press_menu", "long_press_menu-" + shell)
            raise
        finally:
            use_shell("")


def keyboard_shown():
    """Whether the soft keyboard is up, as the input method service says."""
    return "mInputShown=true" in adb("shell", "dumpsys", "input_method")


def keyboard_bars():
    """While the soft keyboard is up the bars under the page step aside, in both Mobile shells
    (docs/design-system.md, On a phone): a tap in a page's field brings it up, the toolbar goes
    and the page takes its room; the keyboard going brings the toolbar back and the page gives
    the room back; and the address field in edit keeps the toolbar with the keyboard up."""
    for shell in ("mwshell", "mkshell"):
        use_shell(shell)
        try:
            launch()
            go(BASE + "/input")
            asked("/input")
            # The keyboard the address had, if it is still up: with it down, the back key
            # would go back in the tab, or leave the app.
            if until(lambda: not keyboard_shown(), 3) is None:
                key(4)
                until(lambda: not keyboard_shown(), 5)
            bottom = box(see("page"))[3]
            check(shell + ": with the keyboard down, the toolbar is under the page", named("shell", "Menu") is not None)
            tap(middle(see("page", kind=FIELD)))
            up = until(lambda: keyboard_shown() and named("shell", "Menu") is None, 10)
            check(shell + ": a tap in a page's field brings the keyboard up, and the toolbar steps aside", up is not None,
                  "keyboard %s, Menu %s" % (keyboard_shown(), named("shell", "Menu") is not None))
            time.sleep(1)
            shot("50-keyboard-bars-" + shell)
            page_bottom = box(see("page"))[3]
            view = screen_parts().get("shell")
            under = [n.get("content-desc") or n.get("text") or n.get("class") for n in (view.iter("node") if view is not None else ())
                     if n.get("clickable") == "true" and box(n)[1] >= page_bottom - 1]
            check(shell + ": nothing of the shell's lies between the page and the keyboard", not under, ", ".join(under))
            key(4)  # The keyboard goes.
            check(shell + ": the keyboard going brings the toolbar back, and the page gives the room back",
                  until(lambda: not keyboard_shown() and named("shell", "Menu") is not None and box(see("page"))[3] == bottom, 10) is not None,
                  "page ends at %d, %d before" % (box(see("page"))[3], bottom))
            tap(middle(see("shell", kind=FIELD)))
            see("shell", kind=FIELD, focused="true")
            check(shell + ": the address field in edit keeps the toolbar with the keyboard up",
                  until(lambda: keyboard_shown(), 10) is not None and find("shell", kind=FIELD, focused="true") is not None)
        except TimeoutError:
            stopped_here("keyboard_bars", "keyboard_bars-" + shell)
            raise
        finally:
            use_shell("")


# Whether the app was running when a part stopped, kept by the part itself: see keep_evidence.
STOPPED = {}


def stopped_here(part, label=None):
    """Keeps a stopped part's evidence while the app is as the step left it. Called from the
    part's own except, before its finally puts the shell back, clears the data, stops the
    app or revokes a permission, which Android stops the app for: kept after that, by the
    run, it always said "the app is gone"."""
    STOPPED[part] = keep_evidence("stopped-" + (label or part), part)


def keep_evidence(name, logname=None):
    """A shot as name, and every log buffer, as running- or gone-<logname>.logcat.txt: the app
    has died with nothing in the crash buffer, and has started without drawing its shell, each
    now and then. Says whether the app is running."""
    shot(name)
    running = alive()
    with open("%s/%s-%s.logcat.txt" % (OUT, "running" if running else "gone", logname or name), "w") as f:
        f.write(adb("logcat", "-b", "all", "-d"))
    return running


def open_welcome():
    """A new profile's welcome, past Get started at Pick your style. False when it never got
    there, which it says as a check."""
    clear_data()
    until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME"))
    adb("shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY)
    see("page", timeout=60, text="Get started")
    # On Android 10 the welcome can show in the page before it takes a tap, and the first
    # tap is lost: tap again while Get started is still there.
    def started():
        if find("page", text="Pick your style") is not None:
            return True
        button = find("page", text="Get started")
        if button is not None:
            tap(middle(button))
            time.sleep(2)
    opened = until(started, 30) is not None
    if not opened:
        # Kept as a stopped part keeps them: it failed once on emu-29 in a release's drive,
        # just after welcome_extras passed the same check, and left nothing to look at.
        keep_evidence("welcome-not-opened")
    check("Get started opens Pick your style", opened)
    return opened


def welcome_extras():
    """A new profile's welcome on a phone: after Pick your style, Extras for any page offers
    the built-in sparks, each off, by name and line; a switch turns one on in Go."""
    if not open_welcome():
        return
    tap(middle(see("page", text="Continue")))
    see("page", text="Extras for any page")
    names = ["Video downloader", "Screen time limit", "Page cleanup"]
    # Android reads a page's switch as a Switch, but not as checkable: whether it is on is
    # Go's, in sparks.json.
    switches = lambda: [n for n in screen_parts()["page"].iter("node") if n.get("class") == "android.widget.Switch"]
    kept = lambda: json.loads(adb("shell", "cat", "/data/data/%s/%s/sparks/sparks.json" % (PACKAGE, PROFILE)))
    ons = lambda: [s["on"] for s in kept()["sparks"] if s["id"] in kept()["builtins"]]
    shown = all(find("page", text=n) is not None for n in names)
    check("after Pick your style, Extras for any page offers the three built-in sparks, each off",
          shown and len(switches()) == 3 and ons() == [False, False, False], ons())
    shot("45-welcome-extras")
    tap(middle(switches()[1]))
    check("its switch turns Screen time limit on, in Go", until(lambda: ons() == [False, True, False], 5) is not None, ons())
    shot("46-welcome-extras-on")
    tap(middle(see("page", text="Skip setup")))


def refuse_android_question(shown, timeout=15):
    """Waits until shown() or Android's own permission question, and refuses the question
    when it is the one up: Deny on Android 10, Don't allow from 11. An older Android asks for
    the camera at any file input, as the app declares it (docs/media-permissions.md)."""
    until(lambda: shown() or "GrantPermissions" in in_front_now(), timeout)
    if "GrantPermissions" in in_front_now():
        no = first(lambda: system_button("Deny", 5), lambda: system_button("Don't allow", 5))
        if no is not None:
            tap(middle(no))


def system_button(text, timeout=10):
    """A button of Android's own on screen, outside our pages, by its words, as a sheet or a
    question the system shows has, any case, ’ read as '; None when none shows in time."""
    def look():
        out = adb("exec-out", "uiautomator", "dump", "/dev/tty")
        if "</hierarchy>" not in out:
            return None
        root = ET.fromstring(out[:out.rindex("</hierarchy>") + len("</hierarchy>")])
        said = lambda n: (n.get("text") or "").replace("\u2019", "'").lower()
        return next((n for n in root.iter("node") if said(n) == text.lower()), None)
    return until(look, timeout)


def browser_role_holders():
    """The packages Android holds as the default browser, one per line."""
    return adb("shell", "cmd", "role", "get-role-holders", "android.app.role.BROWSER").strip()


def welcome_last_step():
    """The welcome's last step on a phone asks Android through Go's core/system, not the engine
    (docs/layers.md): Make default browser (makeDefault) shows Android's sheet, and Allow
    (askNotifications) its question about notifications. Cancelled or refused, each row stays as it was,
    Go having heard no; asked once Android has said yes, Go hears yes at once and the row says
    so."""
    # does: makeDefault
    # does: askNotifications
    before = browser_role_holders()
    adb("shell", "cmd", "role", "remove-role-holder", "android.app.role.BROWSER", PACKAGE)
    try:
        if not open_welcome():
            return
        # Extras for any page when the profile has built-in sparks, then Protection, then the
        # last step.
        for _ in range(4):
            if find("page", text="You're set") is not None:
                break
            tap(middle(see("page", text="Continue")))
            time.sleep(1)
        see("page", text="You're set")
        shot("47-welcome-last-step")

        tap(middle(see("page", text="Make default browser")))
        sheet = until(lambda: "RequestRole" in in_front_now(), 10)
        check("Make default browser shows Android's sheet", sheet, in_front_now())
        tap(middle(system_button("Cancel")))
        check("cancelled, the row still offers it, Go having heard no",
              until(lambda: in_front() and find("page", text="Make default browser") is not None, 10)
              and PACKAGE not in browser_role_holders(), browser_role_holders())
        adb("shell", "cmd", "role", "add-role-holder", "android.app.role.BROWSER", PACKAGE)
        tap(middle(see("page", text="Make default browser")))
        check("the app already the default, it says so at once",
              until(lambda: find("page", text="This is your default browser"), 10) is not None)

        if int(adb("shell", "getprop", "ro.build.version.sdk").strip()) < 33:
            check("before Android 13 notifications are on, with nothing to ask",
                  find("page", text="On") is not None)
            return
        tap(middle(see("page", text="Allow")))
        asked_ = until(lambda: "GrantPermissions" in in_front_now(), 10)
        check("Allow shows Android's question about notifications", asked_, in_front_now())
        tap(middle(system_button("Don't allow")))
        check("refused, the row still offers it, Go having heard no",
              until(lambda: in_front() and find("page", text="Allow") is not None, 10) is not None)
        adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
        tap(middle(see("page", text="Allow")))
        check("allowed already, it says On at once", until(lambda: find("page", text="On"), 10) is not None)
        shot("48-welcome-last-step-answered")
    finally:
        adb("shell", "cmd", "role", "remove-role-holder", "android.app.role.BROWSER", PACKAGE)
        for holder in before.split():
            adb("shell", "cmd", "role", "add-role-holder", "android.app.role.BROWSER", holder)


# The kinds of web server and what each should show: tools/server_kinds.py.
from server_kinds import SERVER_KINDS  # noqa: E402

def shown_words(part):
    """Every text and label in a part of the screen, joined."""
    view = screen_parts().get(part)
    return " | ".join(n.get("text") or n.get("content-desc") for n in (view.iter("node") if view is not None else ()) if n.get("text") or n.get("content-desc"))


def web_server_kinds():
    """Each kind of web server in SERVER_KINDS shows as a person should see it: its page, or
    our error page with the right title, never an unexplained failure. A public page can be
    down or reset a connection now and then, so a miss is tried twice more, 20 s apart, before
    it fails, and the check names the site."""
    for link, want, words in SERVER_KINDS:
        said = ""
        for attempt in (1, 2, 3):
            if attempt > 1:
                time.sleep(20)
            launch()
            view_link(link)
            def outcome():
                page, shell = shown_words("page"), shown_words("shell")
                # A page with a "Sign in" link reads as the shell (PARTS), so its words are
                # looked for in both; the address pill shows only the host.
                if want == "page" and words.lower() in (page + " | " + shell).lower():
                    return "page"
                # An error page is judged by its own title, not by Failed alone, which shows
                # a moment before the title does (drive_first_app's, 0.3.10's run).
                if want == "error" and words.lower() in shell.lower():
                    return "error"
                if want == "page" and ("Failed" in shell.split(" | ") or "Open anyway" in shell):
                    return "error"
                return None
            got = until(outcome, 45)
            said = "the page: %s / the shell: %s" % (shown_words("page")[:200], shown_words("shell")[:240])
            ok = got == want and (want == "page" or words.lower() in shown_words("shell").lower())
            if ok:
                break
        shot("kinds-" + re.sub(r"[^a-z0-9]+", "-", link.split("//", 1)[1].lower()).strip("-"))
        check("%s shows %s" % (link, "its page" if want == "page" else "our error page: " + words), ok,
              "" if ok else "after three tries; " + said)


CHANNELS = "https://github.com/debkosh/project-b/releases/download/channels/releases.json"


def version_installed():
    """The versionName Android has for the app, or ''."""
    said = re.search(r"versionName=(\S+)", adb("shell", "dumpsys", "package", PACKAGE))
    return said.group(1) if said else ""


def update_ready(part, channel=None):
    """PB_UPDATE_FROM installed, an older signed APK whose updater is the one tested, followed
    in About to channel ("dev" or "stable"; its own when None) until the channel's newest
    (CHANNELS) is downloaded and About says it is ready. Returns (before, target), or None
    after checking what failed, or with nothing checked when PB_UPDATE_FROM isn't set."""
    old = os.environ.get("PB_UPDATE_FROM", "")
    if not old:
        print("note: %s needs PB_UPDATE_FROM, an older signed APK of a published channel; skipped" % part, flush=True)
        return None
    if "Success" not in adb("install", "-r", "-d", "--no-incremental", old):
        check("the older build installs to update from", False, old)
        return None
    adb("shell", "pm", "clear", PACKAGE)
    before = version_installed()
    channel = channel or ("dev" if "-dev." in before else "stable")
    out = subprocess.run(["curl", "-sSL", "--max-time", "30", CHANNELS], capture_output=True, text=True).stdout
    target = (json.loads(out or "{}").get("channels") or {}).get(channel, "")
    if not target or target == before:
        check("the %s channel has a newer build than %s to update to" % (channel, before), False, "the manifest names %r" % target)
        return None
    # Android asks once whether this app may install apps; a person allows it in Settings.
    adb("shell", "appops", "set", PACKAGE, "REQUEST_INSTALL_PACKAGES", "allow")
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    adb("logcat", "-b", "crash", "-c")
    launch()
    # Settings' About shows the update's state and its Install update button. A new
    # profile follows Stable whatever build it runs, so About's Updates from picks Dev.
    menu("Settings")
    see("settings", text=SETTINGS_LIST, timeout=30)
    w, h = screen_size()
    for _ in range(8):
        if find("settings", text=starts("About Project B")) is not None:
            break
        adb("shell", "input", "swipe", str(w // 2), str(h * 7 // 10), str(w // 2), str(h * 3 // 10), "300")
        time.sleep(1)
    settings_section("About Project B", "Updates from")
    if channel == "dev":
        tap(middle(see("settings", text="Dev")))
    # The app checks every minute (core/update) and downloads what it finds.
    ready = until(lambda: find("settings", text=target + " is ready"), 300)
    shot(part + "-ready")
    check("%s finds %s on %s and has it ready" % (before, target, channel), ready is not None,
          "About says: %s" % ", ".join(n.get("text") for n in screen_parts()["settings"].iter("node") if n.get("text"))[:300] if ready is None and "settings" in screen_parts() else "")
    return (before, target) if ready is not None else None


def update_on_the_phone():
    """An update on a phone, end to end, as tools/vm/check.py's update is on a Mac: the app
    PB_UPDATE_FROM names, an older published build of the same channel, signed with the same
    key, installed; it finds the channel's newest in the real manifest (docs/release-channels.md)
    and downloads it; the menu sheet's Install update opens Android's package installer in
    front, with the app not crashed; the installer's own button installs it; and the new
    version starts. A post-check of what is published: the updater tested is PB_UPDATE_FROM's,
    as an update runs the installed app's code, so give it the build just before the newest.
    Skipped without PB_UPDATE_FROM. Ends with PB_APK installed again, as it found it."""
    try:
        found = update_ready("update")
        if found is None:
            return
        before, target = found
        tap(middle(see("settings", text="Install update")))
        installer = lambda: re.search(r"(com\.(google\.)?android\.packageinstaller)", in_front_now())
        shown = until(installer, 30)
        shot("update-installer")
        crashed = PACKAGE in adb("logcat", "-b", "crash", "-d")
        check("Install update opens Android's package installer in front, and the app doesn't crash", shown and not crashed,
              "in front: %s%s" % (in_front_now().strip(), "; the app crashed" if crashed else ""))
        if not shown:
            return
        # It stages the APK first, then asks; its button is Update, or Install on Android 10.
        # Only the installer's own: our app's bar under it has an Install too.
        button = until(lambda: system_node(package=lambda p: p.endswith("packageinstaller"), text=lambda t: t in ("Update", "Install")), 60)
        shot("update-asks")
        check("the package installer asks to update the app", button is not None)
        if button is None:
            return
        tap(middle(button))
        installed = until(lambda: version_installed() == target, 120)
        check("the installer puts %s in place of %s" % (target, before), installed, "installed: %s" % version_installed())
        if installed:
            launch()
            check("the updated app starts and shows its shell", find("shell", label="Menu") is not None)
            shot("updated")
    except TimeoutError:
        stopped_here("update_on_the_phone")
        raise
    finally:
        adb("install", "-r", "-d", "--no-incremental", APK)
        clear_data()


def pinch():
    """Two fingers set down near the middle of the screen's upper half and moved apart, as a
    person pinches a page bigger: monkey's PinchZoom, `input` having one finger only."""
    w, h = screen_size()
    x, y = w // 2, h * 3 // 10
    script = os.path.join(OUT, "pinch.txt")
    with open(script, "w") as f:
        f.write("type= user\ncount= 1\nspeed= 1.0\nstart data >>\nPinchZoom(%d,%d,%d,%d,%d,%d,%d,%d,12)\n"
                % (x - 60, y - 60, x - 300, y - 300, x + 60, y + 60, x + 300, y + 300))
    adb("push", script, "/data/local/tmp/pinch.txt")
    adb("shell", "monkey", "-f", "/data/local/tmp/pinch.txt", "1")


def settings_keeps_its_size():
    """Settings keeps its size when a second field of a Dialog takes focus with the keyboard up,
    and still pinches (docs/ux-guide.md, "The phone"; docs/fragility.md, "Settings zoomed by
    itself on a phone"). That is the case in which Android asks a page to zoom in around a
    field, read from Chromium's code: a field asks for the keyboard while it is up already.
    The zoom itself was never made to happen here, on the app before the fix either, so this
    shows only that the case leaves Settings as it was, not that the zoom can't come. A pinch
    makes Settings bigger, as it does a web page: our pages pinch until the app follows
    Android's font size."""
    # A profile with no sync set up, whatever part ran before: after sync_on_the_phone sync is
    # on, and Settings has no Set up… to press.
    clear_data()
    launch()
    go(BASE + "/pinch")
    see("page", text="Pinch me")
    scales = lambda: [float(s.split("=", 1)[1]) for s in served if s.startswith("/said?scale=")]
    pinch()
    check("a pinch makes a web page in a tab bigger", until(lambda: any(v > 1.2 for v in scales()), 10) is not None, scales())
    # Settings at Sync, by its link, whose Dialog is put away: Set up… then opens the set-up
    # at a press, as a person does, its first field with the keyboard.
    view_link("project-b://sync?address=https://sync.example.test/sync/abc")
    until(lambda: settings_text("https://sync.example.test/sync/abc"), 30)
    tap(middle(see("settings", text="Cancel")))
    tap(middle(see("settings", text="Set up…")))
    fields = lambda: until(lambda: [n for n in screen_parts().get("settings", ET.Element("none")).iter("node") if n.get("class") == FIELD] or None, 20) or []
    check("Set up… opens the set-up with the keyboard", len(fields()) >= 2 and until(keyboard_shown, 10) is not None, len(fields()))
    # A button's width and height say the page's scale: the Dialog moves with the keyboard,
    # and its button stays as big as it was.
    def size():
        left, top, right, bottom = box(see("settings", text="Connect"))
        return right - left, bottom - top
    still(lambda: find("settings", text="Connect"))
    own = size()
    # As big as it was, to a few pixels: Android rounds a box as the Dialog moves, and the
    # zoom this looks for is of a third or more.
    same = lambda: all(abs(now - was) <= 4 for now, was in zip(size(), own))
    tap(middle(fields()[1]))
    time.sleep(4)
    check("Settings keeps its size when the Dialog's second field takes focus with the keyboard up",
          keyboard_shown() and same(), "%s, was %s; the keyboard is %s" % (size(), own, "up" if keyboard_shown() else "away"))
    shot("settings-second-field")
    key(4)
    until(lambda: not keyboard_shown(), 10)
    still(lambda: find("settings", text="Connect"))
    pinch()
    bigger = lambda: find("settings", text="Connect") is None or not same()
    check("and a pinch still makes Settings bigger", until(bigger, 10) is not None,
          "%s, was %s" % (size() if find("settings", text="Connect") is not None else "out of sight", own))
    shot("settings-pinched")


def update_notification():
    """The update's notification on a phone (docs/update-notifications.md): PB_UPDATE_FROM,
    following Dev, has the newest ready; its notification says so as About does, and its
    Install, tapped in the shade with the app in the background, opens Android's package
    installer in front. An app can't start an activity from the background since Android 10,
    which is how amarit's Install did nothing (0.3.9), and a notification's updates past a few
    a second are dropped, which kept 0.3.9's on Downloading. Its updater is PB_UPDATE_FROM's,
    as update_on_the_phone's is: before a release, the candidate built with a VERSION older
    than Dev's newest; after one, the published build before Dev's newest. Skipped without
    PB_UPDATE_FROM. Ends with PB_APK installed again, as it found it."""
    try:
        found = update_ready("notification", channel="dev")
        if found is None:
            return
        before, target = found

        def ours():
            dump = adb("shell", "dumpsys", "notification", "--noredact")
            recs = [r for r in dump.split("NotificationRecord(")[1:] if "pkg=" + PACKAGE in r]
            return " | ".join(re.findall(r"android\.title=String \(([^)]*)\)", " ".join(recs)))
        said = until(lambda: "is ready" in ours(), 90)
        check("the update's notification says %s is ready, as About does" % target, said is not None, "its title: %s" % ours())
        if said is None:
            return
        adb("logcat", "-c")
        key(3)  # home: the app in the background
        time.sleep(2)
        adb("shell", "cmd", "statusbar", "expand-notifications")
        time.sleep(2)
        # A collapsed notification hides its buttons: its chevron, at the row's right, opens it.
        title = until(lambda: system_node(text=lambda t: "is ready" in t), 10)
        install = lambda: system_node(text=lambda t: t in ("Install", "INSTALL"))
        if title is not None and install() is None:
            _, y1, _, y2 = box_of(title.get("bounds"))
            tap((screen_size()[0] - 110, (y1 + y2) // 2))
            time.sleep(1)
        button = until(install, 15)
        shot("notification-install")
        check("the shade has the update's notification with Install", button is not None)
        if button is None:
            return
        tap(middle(button))
        shown = until(lambda: re.search(r"packageinstaller", in_front_now()), 30)
        shot("notification-installer")
        blocked = [l for l in adb("logcat", "-d").splitlines() if re.search(r"Background activity|Abort background activity|BAL_BLOCK", l)][:3]
        check("Install in the notification, the app in the background, opens Android's package installer", shown is not None,
              "in front: %s%s" % (in_front_now().strip(), "; " + " / ".join(blocked) if blocked else ""))
    except TimeoutError:
        stopped_here("update_notification")
        raise
    finally:
        adb("shell", "cmd", "statusbar", "collapse")
        adb("install", "-r", "-d", "--no-incremental", APK)
        clear_data()


def main():
    os.makedirs(OUT, exist_ok=True)
    adb("root")
    # adbd restarts as root a moment after saying so, and drops what is sent meanwhile.
    if not until(lambda: adb("shell", "id", "-u").strip() == "0", 30):
        sys.exit("no rooted device answers as %s: start the emulator with `amarit start project-b-emu`" % ADB[-1])
    adb("reverse", "tcp:%d" % PORT, "tcp:%d" % PORT)
    # Removed however the run ends, an error outside a part's own too: runs that ended so
    # left their reverses on the emulator for the next session to find (2026-11-22).
    atexit.register(lambda: adb("reverse", "--remove", "tcp:%d" % PORT))
    # -d: an older APK goes over a newer one, as comparing releases needs.
    # Never incremental: adb picks it on some emulators, and the app then dies of SIGBUS in
    # libproject_b_content.so at every start, as its library pages arrive late.
    if "Success" not in adb("install", "-r", "-d", "--no-incremental", APK):
        sys.exit("can't install %s" % APK)
    clear_data()

    # Each part starts the app again, so one that the app doesn't survive costs only itself.
    every = (browsing, mobile_shell, custom_colour, settings_back, talkback_states, bookmark_star, settings_ahead, back_key_chain, page_dialogs, popups, leave, page_camera, downloads, crash_notice, relaunch_on_the_phone, download_notifications, download_outlives_screen, stream_download, file_inputs, page_full_screen, find_in_page, page_paints, history_after_a_restart, links_from_other_apps, sync_on_the_phone, passkeys_on_the_phone, passwords_on_the_phone, where_go_runs, the_page_renderer_dies, the_shell_renderer_dies,
             the_gpu_process_dies, a_page_spins, web_apis, links_to_other_sites, the_real_web, secure_dns, long_press_menu, keyboard_bars, page_downloads_ask, welcome_extras, welcome_last_step, web_server_kinds, update_on_the_phone, settings_keeps_its_size, update_notification, blocking)
    for part in [p for p in every if not sys.argv[1:] or p.__name__ in sys.argv[1:]]:
        # tools/results.py splits a run's memory, CPU and checks by these lines.
        print("--- part " + part.__name__, flush=True)
        try:
            part()
        except TimeoutError as e:
            # Something the next step needed never came on screen.
            # A part that puts its shell back stops the app on its way out, so its own
            # evidence, kept before that, says whether the app was running.
            running = STOPPED[part.__name__] if part.__name__ in STOPPED else keep_evidence("stopped-" + part.__name__, part.__name__)
            check("%s ran to its end" % part.__name__.replace("_", " "), False, "%s; the app is %s" % (e, "running" if running else "gone"))
    adb("shell", "am", "force-stop", PACKAGE)
    # Its own port forward goes with it, as a run leaves nothing behind on the emulator.
    adb("reverse", "--remove", "tcp:%d" % PORT)

    failed = [r for r in results if not r[1]]
    print("\n%d checks, %d failed" % (len(results), len(failed)))
    with open(OUT + "/results.json", "w") as f:
        json.dump([(name, ok, str(detail)) for name, ok, detail in results], f, indent=1)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())

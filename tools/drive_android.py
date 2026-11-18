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
import http.server
import json
import os
import re
import sqlite3
import subprocess
import sys
import threading
import time
import urllib.parse
import xml.etree.ElementTree as ET
import zipfile

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import links_page  # noqa: E402
import web_features  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SDK = os.path.expanduser("~/Library/Android/sdk")
ADB = [SDK + "/platform-tools/adb", "-s", os.environ.get("ADB_SERIAL", "emulator-5600")]
APK = os.environ.get("PB_APK") or ROOT + "/build/android/project-b-content.apk"
PACKAGE = "dev.projectb.browser"
ACTIVITY = ".BrowserActivity"
PROFILE = "app_project_b/profile"
OUT = os.environ.get("OUT", ROOT + "/build/android-drive-content")
FIELD = "android.widget.EditText"  # The address field is the shell's only text field.

PAGES = {
    "/a": "<title>Page A</title><a href=/b style='display:block;height:200px;font-size:40px'>to b</a>",
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

served = []
# Each request's host and path, for Links to other sites, which serves two sites.
seen = []
results = []


class Server(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        served.append(self.path)
        seen.append(self.headers.get("Host", "").split(":")[0] + self.path)
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
        if path.startswith("/dl/"):
            data = b"Project B download check\n"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Disposition", 'attachment; filename="%s"' % path[4:])
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
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
PAGES["/dialogs"] = ("<title>Dialogs</title><style>button{display:block;width:100%;height:30vh;font-size:40px}</style>"
                     "<button id=c>confirm</button><button id=p>prompt</button><button id=al>alert</button><script>"
                     "const say = (what, v) => fetch('/said?' + what + '=' + encodeURIComponent(String(v)));"
                     "c.onclick = () => say('confirm', confirm('Go on?'));"
                     "p.onclick = () => say('prompt', prompt('Name?', ''));"
                     "al.onclick = () => { alert('Hello'); say('alert', 'closed'); };</script>")


# A page that fills the screen at a tap, as a video's full screen does, and says when it has.
# A file input over the whole page, which says which file it got, or that the picker closed.
PAGES["/upload"] = ("<title>Upload</title><input type=file id=f style='position:fixed;inset:0;width:100%;height:100%;opacity:0'><script>"
                    "f.onchange = () => fetch('/said?file=' + encodeURIComponent([...f.files].map(x => x.name + ':' + x.size).join(',')));"
                    "f.oncancel = () => fetch('/said?cancelled');</script>")
PAGES["/full"] = ("<title>Full</title><button id=f style='position:fixed;inset:0;width:100%;height:100%;font-size:40px'>full</button><script>"
                  "f.onclick = () => document.documentElement.requestFullscreen().then(() => fetch('/full-on'));"
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


def asked(path, timeout=15):
    """Waits until the page has asked the local server for a path that starts this way."""
    return until(lambda: any(s.startswith(path) for s in served), timeout)


# What is on the screen, asked from the app. Content gives Android's accessibility tree one
# WebView per page it shows: the shell and the shown tab's page. Settings is a tab of ours, so
# while it is the shown tab its page is "settings" too, as the shell then offers Close Settings.
# A node's attributes are Android's: class, text, content-desc (an aria-label), focused,
# selected, and bounds in device pixels.
PARTS = {
    # The shell, or, while a page's dialog shows, the dialog alone, as aria-modal hides the
    # rest of the shell from the tree: its title, Go's words, ends in " says" or is Sign in.
    "shell": lambda labels: "Menu" in labels or any(l.endswith(" says") or l == "Sign in" for l in labels),
    "page": lambda labels: "Menu" not in labels,
}


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
            name = next(name for name, test in PARTS.items() if test(labels))
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
    if "page" in parts and shell is not None and any(n.get("content-desc") == "Close Settings" for n in shell.iter("node")):
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


def launch(block=""):
    adb("shell", "am", "force-stop", PACKAGE)
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
          until(lambda: find("settings", text="Theme")) is not None and find("shell", text="Settings") is not None and
          find("shell", label="Address", kind=FIELD) is None)
    shot("06c-settings")
    menu("Settings")
    check("settings again shows the same tab", until(lambda: find("settings", text="Theme")) is not None)
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
    opens its menu, whose Open in new tab in group gathers the tabs in the strip behind the
    page being read; and each screen in light and dark, for the owner."""
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
    item = see("shell", text="Open in new tab in group")
    check("a long press on a link opens the shell's menu, Open in new tab in group first", item is not None)
    shot("20-link-menu")
    key(4)
    check("the back key closes it", bool(until(lambda: find("shell", text="Open in new tab in group") is None)))
    served.clear()
    long_press(middle(see("page", label="to b")))
    tap(middle(see("shell", text="Open in new tab in group")))
    check("Open in new tab in group opens the link behind the page, and the strip shows the group",
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
    tap(middle(see("settings", text="System")))
    close_settings()
    # The active favicon in the strip closes its tab at a tap; a group of one then ends.
    tap(middle(see("shell", label=lambda l: (l or "").startswith("Close Page"), kind="android.widget.Button")))
    check("a tap on the active favicon in the strip closes its tab, and the group of one left ends",
          until(lambda: find("shell", label=lambda l: (l or "").startswith("Group ")) is None) is not None)


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


def profile_json(name):
    """One of the profile's JSON files, or {} while there is none."""
    out = adb("shell", "cat", "/data/data/%s/%s/%s" % (PACKAGE, PROFILE, name))
    try:
        return json.loads(out)
    except ValueError:
        return {}


def use_shell(shell):
    """Makes shell the profile's at the next start, '' for the phone's own, by its pref."""
    prefs = profile_json("prefs.json")
    prefs.pop("shell", None)
    if shell:
        prefs["shell"] = shell
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
    for shell in ("mobile", "mobile-native"):
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
            chosen = {"checked": "true"} if shell == "mobile-native" else {"kind": "android.widget.RadioButton"}
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
            # The screen as it stopped, before the shell is put back, which stops the app.
            shot("stopped-bookmark_star-" + shell)
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


def notification_of(name):
    """The download notification for the file name, its title and text as Android keeps them,
    or None."""
    text = adb("shell", "dumpsys", "notification", "--noredact")
    for record in text.split("NotificationRecord(")[1:]:
        if "pkg=" + PACKAGE in record and "android.title=String (%s)" % name in record:
            said = re.search(r"android\.text=String \(([^)]*)\)", record)
            return said.group(1) if said else ""
    return None


def downloads():
    """A file the page downloads goes to the phone's Downloads, with a notification that
    says so and opens it at a tap; the menu's Downloads opens the Downloads page, which lists
    it, Open opens it in another app, and Show in Downloads opens the phone's Downloads app;
    and a restart doesn't download it again."""
    adb("shell", "pm", "grant", PACKAGE, "android.permission.POST_NOTIFICATIONS")
    launch()
    name = "pb-check-%d.txt" % int(time.time())
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
    tap(middle(see("page", text="Open")))
    # A window other than ours, once Android has one in front: the chooser, or the app.
    opened = until(lambda: "Window{" in in_front_now() and "BrowserActivity" not in in_front_now(), 10)
    check("Open hands the file to another app", opened is not None, in_front_now())
    shot("26-downloads-open")
    key(4)
    until(lambda: "BrowserActivity" in in_front_now(), 10)
    tap(middle(see("page", text="Show in Downloads")))
    shown = until(lambda: "documentsui" in in_front_now().lower() or "downloads" in in_front_now().lower(), 10)
    check("Show in Downloads opens the phone's Downloads app", shown is not None, in_front_now())
    shot("27-downloads-shown")
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
    # The tab is still at the page it showed, so a restart loads that, not the download.
    served.clear()
    launch()
    time.sleep(3)
    check("a restart doesn't download it again", not any(s.startswith("/dl/") for s in served), str(served))


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
    finally:
        adb("shell", "am", "force-stop", PACKAGE)
        adb("shell", "pm", "clear", PACKAGE)


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
    page = screen_parts().get("page")
    tap(middle(page))
    opened = until(picker_shows) is not None
    check("a page's file input opens the phone's picker", opened, in_front_now())
    if not opened:
        return
    shot("40-file-picker")
    key(4)
    check("and closing it tells the page", asked("/said?cancelled"), str(served))
    tap(middle(screen_parts().get("page")))
    until(picker_shows)
    pick("SiteSparks.zip")
    check("a file picked reaches the page by its own name", asked("/said?file=" + urllib.parse.quote("SiteSparks.zip:%d" % size)), str(served))

    menu("Settings")
    see("settings", text="Theme")
    tap(middle(see("settings", kind=FIELD)))
    type_text("Sparks")
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
    check("Settings' Sparks Import… opens the phone's picker", until(picker_shows) is not None, in_front_now())
    pick("SiteSparks.zip", several=True)
    check("and a zip of sparks picked there imports them", see("settings", text="Added 3 sparks") is not None)
    shot("41-sparks-imported")
    tap(middle(see("settings", text="Done")))
    close_settings()
    adb("shell", "rm", "-f", "/sdcard/Download/SiteSparks.zip")


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
    width, height = screen_size()
    covers = until(lambda: find("shell", label="Menu") is None and (lambda b: b[1] == 0 and b[3] == height and b[2] == width)(box(see("page", text="full"))), 10)
    check("it covers the whole screen, with nothing of the shell or Android's bars on it", covers is not None,
          "%s on a %dx%d screen" % (box(see("page", text="full")), width, height))
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
        for name, how in (("Wrapped", "by a label around it"), ("By id", "by aria-labelledby"), ("Field", "by <label for>")):
            n = named(name)
            check("and a control named %s says its name" % how, n is not None,
                  None if n is None else (n.get("class", "").rsplit(".", 1)[-1], n.get("text"), n.get("content-desc"), n.get("hint")))
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


def sync_on_the_phone():
    """Sync on a phone (docs/sync.md). A project-b://sync link from another app opens Settings
    at Sync with the link's Dialog, its address filled in, whether the app runs or not. Turning
    sync on against a sync server on this Mac (core/cmd/syncserver, reached through adb reverse,
    its own CA put in the device's user CA folder, which Go reads, as sync takes only https)
    makes the device key, wrapped by the Android Keystore, and the profile's key; after a
    restart sync runs again with what was kept, so the Keystore opened the key; and DeviceKey
    logs no exception."""
    stopped = lambda: (adb("shell", "am", "force-stop", PACKAGE), until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME")))
    linked = lambda: until(lambda: settings_text("https://sync.example.test/sync/abc"), 30)
    adb("logcat", "-c")
    launch()
    view_link(SYNC_LINK)
    check("a sync link from another app opens Settings at Sync with the link's Dialog, while the app runs",
          linked() is not None and settings_text(lambda t: (t or "").startswith("A link asked to set up sync")) is not None)
    shot("sync-link-running")
    stopped()
    view_link(SYNC_LINK)
    check("and when the app wasn't running", linked() is not None)
    shot("sync-link-stopped")

    folder = os.path.abspath(os.path.join(OUT, "syncserver"))
    binary = os.path.abspath(os.path.join(OUT, "syncserver-bin"))
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
        tap(middle(see("settings", text="Turn on sync")))
        pushed = until(lambda: any("/push" in r for r in requests), 60)
        shot("sync-on")
        key = adb("shell", "stat", "-c", "%s", DEVICE_KEY).strip()
        profile_key = adb("shell", "stat", "-c", "%s", "/data/data/%s/%s/profile.key" % (PACKAGE, PROFILE)).strip()
        check("turning sync on syncs, and makes the device key, wrapped by the Keystore, and the profile's key",
              pushed and key.isdigit() and int(key) > 32 and profile_key.isdigit(), (requests[-3:], key, profile_key))
        stopped()
        before = len(requests)
        launch()
        check("after a restart sync runs again with what was kept: the Keystore opened the device key",
              until(lambda: len(requests) > before, 60) is not None, requests[before:])
        logcat = adb("logcat", "-d")
        trouble = [l for l in logcat.splitlines() if "DeviceKey" in l and ("Exception" in l or "Error" in l)]
        check("DeviceKey logs no exception", not trouble, trouble[:5])
    finally:
        srv.terminate()
        adb("reverse", "--remove", "tcp:%d" % port) if "port" in locals() else None
        adb("shell", "rm", "-f", "/data/misc/keychain/certs-added/project-b-sync-test.0")


def the_page_renderer_dies():
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
    served.clear()
    menu("Reload")
    check("reload recovers after a crash", asked("/a"), str(served))
    shot("17-reloaded-after-crash")


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
        wrong = [web_features.mismatch(row, probe, "android", "killed" if killed else seen[probe["name"]])
                 for row, probe in REGISTRY_APIS]
        wrong = [w for w in wrong if w]
        check("and its %d calls of the web-features registry give what it says for Android, on %s" % (len(REGISTRY_APIS), on),
              not wrong, "; ".join(wrong))
        unknown = ["%s (%s): %s" % (row["id"], probe["name"], seen[probe["name"]])
                   for row, probe in REGISTRY_APIS if row["status"]["android"] == "unknown"]
        if unknown:
            print("  not yet known on Android, seen on %s: %s" % (on, "; ".join(unknown)), flush=True)
    shot("18b-web-apis")


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


def welcome_extras():
    """A new profile's welcome on a phone: after Pick your style, Extras for any page offers
    the built-in sparks, each off, by name and line; a switch turns one on in Go."""
    adb("shell", "pm", "clear", PACKAGE)
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
    check("Get started opens Pick your style", opened)
    if not opened:
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


def main():
    os.makedirs(OUT, exist_ok=True)
    adb("root")
    # adbd restarts as root a moment after saying so, and drops what is sent meanwhile.
    if not until(lambda: adb("shell", "id", "-u").strip() == "0", 30):
        sys.exit("no rooted device answers as %s: start the emulator with `amarit start project-b-emu`" % ADB[-1])
    adb("reverse", "tcp:%d" % PORT, "tcp:%d" % PORT)
    # -d: an older APK goes over a newer one, as comparing releases needs.
    # Never incremental: adb picks it on some emulators, and the app then dies of SIGBUS in
    # libproject_b_content.so at every start, as its library pages arrive late.
    if "Success" not in adb("install", "-r", "-d", "--no-incremental", APK):
        sys.exit("can't install %s" % APK)
    adb("shell", "pm", "clear", PACKAGE)

    # Each part starts the app again, so one that the app doesn't survive costs only itself.
    every = (browsing, mobile_shell, talkback_states, bookmark_star, back_key_chain, page_dialogs, downloads, file_inputs, page_full_screen, find_in_page, page_paints, history_after_a_restart, links_from_other_apps, sync_on_the_phone, where_go_runs, the_page_renderer_dies, the_shell_renderer_dies,
             the_gpu_process_dies, a_page_spins, web_apis, links_to_other_sites, the_real_web, welcome_extras)
    for part in [p for p in every if not sys.argv[1:] or p.__name__ in sys.argv[1:]]:
        # tools/results.py splits a run's memory, CPU and checks by these lines.
        print("--- part " + part.__name__, flush=True)
        try:
            part()
        except TimeoutError as e:
            # Something the next step needed never came on screen.
            shot("stopped-" + part.__name__)
            running = alive()
            # A part that stops keeps every log buffer: the app has died with nothing in the crash
            # buffer, and has started without drawing its shell, each now and then.
            with open("%s/%s-%s.logcat.txt" % (OUT, "running" if running else "gone", part.__name__), "w") as f:
                f.write(adb("logcat", "-b", "all", "-d"))
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

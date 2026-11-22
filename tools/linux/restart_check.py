#!/usr/bin/env python3
"""The installed Flatpak keeps its profile between starts, and keeps no key in a file
(docs/research/linux.md, 5). Run in the Ubuntu VM's desktop session, as its user, with the app
installed (tools/linux/flatpak-release.sh runs it before a release's repository is kept):

1. A fresh start, on the profile list's default folder, which is the XDG data folder's:
   ~/.var/app/<id>/data/project-b, the folder Flatpak keeps.
2. It opens a tab on a test page and puts the tabs on top, then quits as the menu does.
3. The next start has the tab and the setting back.
4. No folder.key is anywhere in the profiles: a listed profile's key is wrapped by the desktop's
   Secret portal's, never by a key in a file.
5. What the profile seals opens again on the next start: a login imported before the restart
   makes the profile's key, wrapped by the device key from the desktop's Secret portal, and
   one imported after has to open it again; a desktop with no Secret backend fails here.
6. A copy the app starts outlives it: Profiles… opens the picker, which stays when the app
   quits, and Settings' Relaunch brings the app back on its profile. The sandbox ends every
   process in it with the first, so a copy started from inside would end too.

The driver's FIFO and the harness's temporary folders are in the app's own folder, which the
sandbox sees at the same path.
"""

import glob
import json
import os
import shutil
import subprocess
import sys
import time

APP = "io.github.sudiptadeb.project-b"
HOME = os.path.expanduser("~")
OWN = os.path.join(HOME, ".var/app", APP)
DATA = os.path.join(OWN, "data/project-b")
SCRATCH = os.path.join(OWN, "pb-check")
URL = "https://example.com/"

shutil.rmtree(DATA, ignore_errors=True)
shutil.rmtree(SCRATCH, ignore_errors=True)
os.makedirs(SCRATCH)
os.environ["TMPDIR"] = SCRATCH
wrapper = os.path.join(SCRATCH, "flatpak-app")
with open(wrapper, "w") as f:
    f.write(f'#!/bin/sh\nexec flatpak run {APP} "$@"\n')
os.chmod(wrapper, 0o755)
os.environ["PB_BIN"] = wrapper

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import drive_first_app as d  # noqa: E402

d.OUT = os.path.join(SCRATCH, "out")
os.makedirs(d.OUT, exist_ok=True)
DEFAULT = ["--root="]  # The profile list's own folder, as a start from the desktop has.


def start():
    return d.App(None, DEFAULT)


def run(app, action):
    app.js(f"chrome.send('shell', [{json.dumps(json.dumps({'t': 'run', 'text': action}))}]), 1")


def copies(arg):
    """The app's processes whose command line has arg, as the host sees them."""
    out = subprocess.run(["pgrep", "-af", "/app/project-b/project_b"], capture_output=True, text=True).stdout
    return [line for line in out.splitlines() if arg in line and "--type=" not in line]


def wait_for(ok, timeout=20):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if ok():
            return True
        time.sleep(0.5)
    return ok()


def settings(app):
    """Opens Settings and has its page keep every message Go sends it, in window.__got."""
    run(app, "app.settings")
    deadline = time.time() + 15
    while time.time() < deadline and app.cmd("evalsettings", "!!document.querySelector('h1')") != "true":
        time.sleep(0.3)
    app.cmd("evalsettings", "(() => { if (!window.__got) { window.__got = []; const o = window.__shellReceive; "
                            "window.__shellReceive = (j) => { window.__got.push(JSON.parse(j)); o(j); }; } return 1; })()")


def import_login(app, user):
    """Imports one login, as a passwords file from Settings, and returns Go's answer."""
    text = f"name,url,username,password\nexample,https://example.com/,{user},check-{user}\n"
    msg = {"t": "importFile", "kind": "passwords", "name": "check.csv", "text": text}
    app.cmd("evalsettings", f"chrome.send('settings', [{json.dumps(json.dumps(msg))}]), 1")
    deadline = time.time() + 30
    while time.time() < deadline:
        got = json.loads(app.cmd("evalsettings", "window.__got.filter(m => m.t === 'fileImported')"))
        if got:
            app.cmd("evalsettings", "window.__got = window.__got.filter(m => m.t !== 'fileImported'), 1")
            return got[-1]
        time.sleep(0.3)
    return {"error": "no answer"}


def added(answer):
    """How many logins an import sealed: one that couldn't be sealed is skipped, not added."""
    return (answer.get("result") or {}).get("added", 0)


def prefs():
    found = glob.glob(os.path.join(DATA, "*", "prefs.json"))
    return json.load(open(found[0])) if found else {}


def main():
    app = start()
    tabs = d.tabs_of(app)
    workspace = tabs["tabs"][0]["workspace"]
    app.cmd("opentab", workspace, URL)
    run(app, "view.tabsTop")
    settings(app)
    first = import_login(app, "before")
    d.check("a login imported is sealed under the device key", added(first) == 1, first)
    deadline = time.time() + 15
    while time.time() < deadline and not (prefs().get("tabLayout") == "horizontal" and
                                            any(t.get("url") == URL for t in d.tabs_of(app)["tabs"])):
        time.sleep(0.2)
    d.check("the profile is in the XDG data folder Flatpak keeps", bool(glob.glob(os.path.join(DATA, "*", "prefs.json"))),
            os.listdir(DATA) if os.path.isdir(DATA) else "no " + DATA)
    run(app, "app.quit")
    try:
        app.proc.wait(timeout=30)
        d.App.running.remove(app)
    except Exception as e:  # noqa: BLE001
        d.check("app.quit ends the app", False, e)

    again = start()
    urls = [t.get("url") for t in d.tabs_of(again)["tabs"]]
    d.check("the next start has the tab back", URL in urls, urls)
    d.check("and the tabs on top", prefs().get("tabLayout") == "horizontal", prefs().get("tabLayout"))
    keys = [os.path.relpath(p, DATA) for p in glob.glob(os.path.join(DATA, "**", "folder.key"), recursive=True)]
    d.check("no folder.key in the profiles", keys == [], keys)
    d.check("the profile's key is kept, wrapped", bool(glob.glob(os.path.join(DATA, "*", "profile.key"))),
            os.listdir(DATA) if os.path.isdir(DATA) else "no " + DATA)
    settings(again)
    second = import_login(again, "after")
    d.check("and opens again on the next start: a second login seals", added(second) == 1, second)

    # Profiles… starts the picker as a copy of its own; it stays when the app quits.
    run(again, "profile.picker")
    d.check("Profiles… opens the picker", wait_for(lambda: copies("--picker")), copies(""))
    run(again, "app.quit")
    try:
        again.proc.wait(timeout=30)
        d.App.running.remove(again)
    except Exception as e:  # noqa: BLE001
        d.check("app.quit ends the app", False, e)
    time.sleep(5)
    d.check("and the picker stays when the app quits", bool(copies("--picker")), copies(""))
    subprocess.run(["flatpak", "kill", APP], capture_output=True)
    wait_for(lambda: not copies(""))

    # Settings' Relaunch ends the app and starts it again on its profile.
    third = start()
    run(third, "app.settings")
    deadline = time.time() + 15
    while time.time() < deadline and third.cmd("evalsettings", "!!document.querySelector('h1')") != "true":
        time.sleep(0.3)
    third.cmd("evalsettings", "chrome.send('settings', [JSON.stringify({t: 'relaunch'})]), 1")
    try:
        third.proc.wait(timeout=30)
        d.App.running.remove(third)
        ended = True
    except Exception:  # noqa: BLE001
        ended = False
    d.check("Settings' Relaunch ends the app", ended)
    d.check("and starts it again", wait_for(lambda: copies("--after-update"), 30), copies(""))
    time.sleep(5)
    d.check("which keeps running", bool(copies("--after-update")), copies(""))
    subprocess.run(["flatpak", "kill", APP], capture_output=True)
    shutil.rmtree(SCRATCH, ignore_errors=True)
    print(f"{d.failures} FAILED" if d.failures else "all passed", flush=True)
    sys.exit(1 if d.failures else 0)


if __name__ == "__main__":
    main()

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

The driver's FIFO and the harness's temporary folders are in the app's own folder, which the
sandbox sees at the same path.
"""

import glob
import json
import os
import shutil
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


def prefs():
    found = glob.glob(os.path.join(DATA, "*", "prefs.json"))
    return json.load(open(found[0])) if found else {}


def main():
    app = start()
    tabs = d.tabs_of(app)
    workspace = tabs["tabs"][0]["workspace"]
    app.cmd("opentab", workspace, URL)
    run(app, "view.tabsTop")
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
    again.close()
    shutil.rmtree(SCRATCH, ignore_errors=True)
    print(f"{d.failures} FAILED" if d.failures else "all passed", flush=True)
    sys.exit(1 if d.failures else 0)


if __name__ == "__main__":
    main()

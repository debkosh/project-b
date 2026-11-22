#!/usr/bin/env python3
"""The software centres' screenshots of the app (the AppStream metainfo's <screenshots>), taken
from the installed Flatpak in the Ubuntu VM's desktop session, at 1280x800, light, on fresh
profiles with nothing of a person's: what the app's own surfaces draw, window buttons included
and no desktop around them (the driver's capture). As ux chose:

1. Standard, tabs down the side, on a real page with two more tabs.
2. Glide, the same tabs, its sidebar showing and nothing over it.
3. The welcome's Pick your style, on the default teal.

    python3 tools/linux/appstream_shots.py <out folder>
"""

import json
import os
import shutil
import sys
import tempfile
import time

APP = "io.github.sudiptadeb.project-b"
OWN = os.path.join(os.path.expanduser("~"), ".var/app", APP)
SCRATCH = os.path.join(OWN, "pb-shots")
PAGES = ["https://example.com/", "https://example.org/", "https://www.iana.org/help/example-domains"]

out = os.path.abspath(sys.argv[1])
os.makedirs(out, exist_ok=True)
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
# The app writes its captures where its sandbox can: its own folder; they're copied out after.
SHOTS = os.path.join(SCRATCH, "shots")
os.makedirs(SHOTS)


def fresh_root():
    return tempfile.mkdtemp(prefix="pb-root-", dir=SCRATCH)


def click(app, text):
    """Clicks the button that says text on the page the window shows."""
    app.wait(f"[...document.querySelectorAll('button')].some(b => b.textContent.trim() === {json.dumps(text)})", 20, page=True)
    app.js(f"[...document.querySelectorAll('button')].find(b => b.textContent.trim() === {json.dumps(text)}).click(), 1", page=True)


def main():
    app = d.App(None, [f"--root={fresh_root()}"])
    send = lambda msg: app.js(f"chrome.send('shell', [{json.dumps(json.dumps(msg))}]), 1")
    app.cmd("resize", 1280, 800)
    workspace = d.tabs_of(app)["tabs"][0]["workspace"] if d.tabs_of(app)["tabs"] else ""
    for url in PAGES:
        if workspace:
            app.cmd("opentab", workspace, url)
        else:
            send({"t": "newTab", "text": url})
        d.loaded(app, url)
    # Only the three pages: the empty tab a fresh start opens goes.
    for t in d.tabs_of(app)["tabs"]:
        if t.get("url") not in PAGES:
            send({"t": "closeTab", "id": t["id"]})
    time.sleep(3)
    print(app.cmd("capture", os.path.join(SHOTS, "1-standard.png")), flush=True)

    send({"t": "run", "text": "shells:glide"})
    app.wait("location.href==='chrome://shell/glide/'&&!!document.querySelector('.glide')", 30)
    app.cmd("resize", 1280, 800)
    time.sleep(3)
    print(app.cmd("capture", os.path.join(SHOTS, "2-glide.png")), flush=True)
    app.close()

    welcome = d.App(None, [f"--root={fresh_root()}"], welcome=True)
    welcome.cmd("resize", 1280, 800)
    click(welcome, "Get started")
    click(welcome, "Continue")
    welcome.wait("[...document.querySelectorAll('h1')].some(h => h.textContent.includes('Pick your style'))", 20, page=True)
    time.sleep(2)
    print(welcome.cmd("capture", os.path.join(SHOTS, "3-welcome.png")), flush=True)
    welcome.close()
    for name in os.listdir(SHOTS):
        shutil.copy(os.path.join(SHOTS, name), out)
    shutil.rmtree(SCRATCH, ignore_errors=True)


if __name__ == "__main__":
    main()

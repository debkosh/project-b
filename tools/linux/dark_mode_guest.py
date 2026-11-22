#!/usr/bin/env python3
"""The guest's half of tools/linux/check_dark_mode.py, run in a Linux desktop session: with
the app's theme left at System and no colour picked, turns the desktop light and dark with its own settings
(GNOME's gsettings, KDE's colour scheme) and checks the app follows each, through the Settings
portal: the shell's and a page's prefers-color-scheme, and the window's frame colour. At each
step it writes <step>.json and <step>.ready, and waits for the Mac's half to take its shot
and write <step>.done. Last it starts the app again on a dark desktop, which it must be at
once. The desktop is left light.

    PB_BIN=<project_b> python3 tools/linux/dark_mode_guest.py <out folder>
"""

import json
import os
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import drive_first_app as d  # noqa: E402

DARK = "matchMedia('(prefers-color-scheme: dark)').matches"


def desktop(dark):
    """Turns the desktop dark or light, as its own settings do."""
    if "KDE" in os.environ.get("XDG_CURRENT_DESKTOP", ""):
        subprocess.run(["plasma-apply-colorscheme", "BreezeDark" if dark else "BreezeLight"], check=True)
    else:
        subprocess.run(["gsettings", "set", "org.gnome.desktop.interface", "color-scheme",
                        "prefer-dark" if dark else "default"], check=True)


def start(profile):
    app = d.App(profile)
    app.cmd("opentab", "0a1b2c3d", d.PAGES)
    d.loaded(app, d.PAGES)
    return app


def step(app, out, name, dark):
    """Waits for the app to be dark or light, then records it and holds for the shot."""
    want = "true" if dark else "false"
    result = {"step": name, "want": "dark" if dark else "light"}
    for where, page in (("shell", False), ("page", True)):
        try:
            app.wait(f"{DARK} === {want}", timeout=10, page=page)
            result[where] = result["want"]
        except TimeoutError:
            result[where] = "light" if dark else "dark"
    result["frame"] = app.cmd("color")
    with open(os.path.join(out, "pid"), "w") as f:
        f.write(str(app.proc.pid))
    with open(os.path.join(out, f"{name}.json"), "w") as f:
        json.dump(result, f)
    open(os.path.join(out, f"{name}.ready"), "w").close()
    deadline = time.time() + 60
    while time.time() < deadline and not os.path.exists(os.path.join(out, f"{name}.done")):
        time.sleep(0.2)
    return result


def main():
    out = os.path.abspath(sys.argv[1])
    os.makedirs(out, exist_ok=True)
    os.makedirs(d.OUT, exist_ok=True)
    for name in os.listdir(out):
        os.remove(os.path.join(out, name))
    app = None
    try:
        profile = tempfile.mkdtemp(prefix="pb-profile-")
        with open(os.path.join(profile, "workspaces.json"), "w") as f:
            json.dump({"workspaces": [{"id": "0a1b2c3d", "name": "Work", "color": "#3470d0"}]}, f)
        # No colour picked: its frame is the grey of light or of dark, which shows the window's
        # own parts turning; a picked colour is the same in both.
        with open(os.path.join(profile, "prefs.json"), "w") as f:
            json.dump({"frameColor": ""}, f)
        desktop(False)
        time.sleep(1)
        app = start(profile)
        step(app, out, "1-light", False)
        desktop(True)
        step(app, out, "2-dark", True)
        desktop(False)
        step(app, out, "3-light-again", False)
        app.close()
        app = None
        desktop(True)
        time.sleep(1)
        app = start(profile)
        step(app, out, "4-started-dark", True)
    except Exception as e:  # noqa: BLE001
        open(os.path.join(out, "error"), "w").write(repr(e))
    finally:
        if app:
            app.close()
        desktop(False)
        open(os.path.join(out, "finished"), "w").close()


if __name__ == "__main__":
    main()

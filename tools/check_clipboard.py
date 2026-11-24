#!/usr/bin/env python3
"""Copy and paste in the real app, through Go's keys and the Mac's clipboard.

NEEDS A PERSON LOGGED IN at this Mac's screen: the clipboard belongs to their desktop
session, and an app started from a background job can't reach it. What they had copied
goes back on it after. Start it through on_screen.py:

    tools/test.sh screen                                   # with the other real-screen checks
    tools/on_screen.py python3 tools/check_clipboard.py    # alone

PB_BIN picks another binary.
"""

import json
import os
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import drive_first_app as drive  # noqa: E402
from drive_first_app import INPUT, VALUE, check  # noqa: E402


def clipboard_checks():
    """Copy and paste go through Go to whichever page has the keyboard: here, the shell."""
    app = drive.App(tempfile.mkdtemp(prefix="pb-profile-"))
    drive.focus_bar(app)
    app.cmd("type", "copied text")
    app.wait(f"{VALUE}==='copied text'")
    app.cmd("key", "cmd+a")
    app.cmd("key", "cmd+c")
    app.js(f"{INPUT}.value=''")
    app.cmd("key", "cmd+v")
    try:
        app.wait(f"{VALUE}==='copied text'", 5)
        check("Cmd+A, Cmd+C and Cmd+V work in the address bar", True)
    except TimeoutError as e:
        check("Cmd+A, Cmd+C and Cmd+V work in the address bar", False, e)
    app.close()



def arshell_copy_checks():
    """Curve's Copy Address from the menu bar, an action with no click in the shell, puts the
    page's address on the Mac's clipboard: Go writes it, as the shell's own write would be
    refused (docs/fragility.md)."""
    app = drive.App(None, [f"--root={tempfile.mkdtemp(prefix='pb-root-')}"])
    send = lambda msg: app.js(f"chrome.send('shell', [{json.dumps(json.dumps(msg))}]), 1")
    try:
        app.wait("location.href==='chrome://shell/pbshell/'", 30)
        send({"t": "run", "text": "shells:arshell"})
        app.wait("location.href==='chrome://shell/arshell/'&&!!document.querySelector('.arshell')", 30)
        send({"t": "navigate", "text": drive.URL_A})
        app.wait("document.querySelector('[role=tab][aria-selected=true]')?.title !== 'New Tab'", 30)
        send({"t": "run", "text": "arshell.copyAddress"})
        for _ in range(40):
            pasted = subprocess.run(["pbpaste"], capture_output=True, text=True).stdout
            if pasted == drive.URL_A:
                break
            time.sleep(0.25)
        check("Curve's Copy Address from the menu bar puts the page's address on the clipboard", pasted == drive.URL_A, repr(pasted))
    finally:
        app.close()


if __name__ == "__main__":
    os.makedirs(drive.OUT, exist_ok=True)
    # The clipboard is the person's at this Mac: what they had copied goes back after.
    theirs = subprocess.run(["pbpaste"], capture_output=True).stdout
    try:
        drive.run_parts([clipboard_checks, arshell_copy_checks])
    finally:
        subprocess.run(["pbcopy"], input=theirs)

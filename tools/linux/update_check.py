#!/usr/bin/env python3
"""The installed Flatpak updates itself and relaunches into the new release (docs/research/linux.md,
5), end to end, in the Ubuntu VM's desktop session, from a repository of the check's own:

1. A local repository (unsigned, file://) with the staged app as its first commit, a remote of
   it, and the app installed from it, as a person's install from ours.
2. The app may update itself without the Access dialog (flatpak permission-set), and the
   portal's monitor polls every 15 seconds, not 30 minutes.
3. The app runs on its default profile with a tab open; the repository gets a second commit.
4. The app hears of it, makes it and says it's ready; Relaunch quits it, and a copy on the new
   commit starts on the same profile, with the tab.

    python3 tools/linux/update_check.py <stage-arm64.tar.gz> <io.github.sudiptadeb.project-b.yml> [--click]

With --click the relaunch is a person's: once the update is ready, the check writes
~/pb-update-check-ready and waits up to 3 minutes for a click on Relaunch in the desktop's
notification, while dbus-monitor logs the portal's ActionInvoked to ~/pb-update-check-actions.log.

Everything it made goes at the end: the app and its data, the remote, the repository, and the
portal it started.
"""

import json
import os
import shutil
import subprocess
import sys
import tarfile
import time

APP = "io.github.sudiptadeb.project-b"
HOME = os.path.expanduser("~")
WORK = os.path.join(HOME, "pb-update-check")
REPO = os.path.join(WORK, "repo")
REMOTE = "pb-update-check"
OWN = os.path.join(HOME, ".var/app", APP)
SCRATCH = os.path.join(OWN, "pb-check")
URL = "https://example.com/"
PORTAL_LOG = os.path.join(HOME, "pb-update-check-portal.log")

stage_tar, manifest = os.path.abspath(sys.argv[1]), os.path.abspath(sys.argv[2])
CLICK = "--click" in sys.argv[3:]
READY = os.path.join(HOME, "pb-update-check-ready")
ACTIONS = os.path.join(HOME, "pb-update-check-actions.log")
shutil.rmtree(WORK, ignore_errors=True)
os.makedirs(WORK)
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


def sh(*args, check=True):
    r = subprocess.run(args, capture_output=True, text=True)
    if check and r.returncode != 0:
        raise RuntimeError(f"{' '.join(args)}: {r.stderr[-600:]}")
    return r.stdout


def commit(mark):
    """Builds the stage into the repository as a new commit, told apart by a file of its own."""
    with open(os.path.join(WORK, "stage/files/update-check"), "w") as f:
        f.write(mark)
    r = subprocess.run(["flatpak-builder", "--force-clean", "--disable-cache", "--default-branch=stable",
                        f"--repo={REPO}", os.path.join(WORK, "build-dir"), os.path.join(WORK, os.path.basename(manifest))],
                       cwd=WORK, capture_output=True, text=True)
    if r.returncode != 0:
        raise RuntimeError("flatpak-builder: " + (r.stdout + r.stderr)[-1200:])
    sh("flatpak", "build-update-repo", "--generate-static-deltas", REPO)
    return open(os.path.join(REPO, f"refs/heads/app/{APP}/aarch64/stable")).read().strip()


def running_commits():
    """The commits the app's running sandboxes are on, as flatpak ps shortens them."""
    out = sh("flatpak", "ps", "--columns=application,commit", check=False)
    return sorted({line.split()[1] for line in out.splitlines() if line.startswith(APP) and len(line.split()) > 1})


def on(full):
    """Whether every running copy of the app is on the commit full."""
    commits = running_commits()
    return len(commits) == 1 and full.startswith(commits[0])


DISPLAY_VARS = ("WAYLAND_DISPLAY", "DISPLAY", "XDG_SESSION_TYPE")


def portal_pid():
    """The process that owns the Flatpak portal's name, which Spawn reaches."""
    out = sh("busctl", "--user", "status", "org.freedesktop.portal.Flatpak", check=False)
    for line in out.splitlines():
        if line.startswith("PID="):
            return int(line[4:])
    return None


def display_of(pid):
    try:
        env = open(f"/proc/{pid}/environ", "rb").read().split(b"\0")
    except OSError as e:
        return str(e)
    return [v.decode() for v in env if v.split(b"=")[0].decode(errors="replace") in DISPLAY_VARS]


def timeline_has(app, text):
    try:
        return any(text in e.get("text", "") for e in json.loads(app.cmd("timeline", 0))["result"]["entries"])
    except Exception:  # noqa: BLE001
        return False


def main():
    portal = None
    try:
        with tarfile.open(stage_tar) as t:
            t.extractall(WORK)
        shutil.copy(manifest, WORK)
        first = commit("1")
        sh("flatpak", "remote-add", "--user", "--if-not-exists", "--no-gpg-verify", REMOTE, "file://" + REPO)
        sh("flatpak", "install", "--user", "-y", "--noninteractive", "--reinstall", REMOTE, APP)
        sh("flatpak", "permission-set", "flatpak", "updates", APP, "yes")
        portal = subprocess.Popen(["/usr/libexec/flatpak-portal", "--replace", "--no-idle-exit", "--poll-timeout=15"],
                                  stdout=open(PORTAL_LOG, "w"), stderr=subprocess.STDOUT)
        # Spawn has the serving portal's environment: the app starts only once ours owns the
        # name, with the session's display, or a copy it spawns may have none.
        deadline = time.time() + 20
        while time.time() < deadline and not (portal_pid() == portal.pid and
                                              any(v.startswith(("WAYLAND_DISPLAY=", "DISPLAY=")) for v in display_of(portal.pid))):
            time.sleep(0.2)
        serving = portal_pid()
        ours = serving == portal.pid and any(v.startswith(("WAYLAND_DISPLAY=", "DISPLAY=")) for v in display_of(portal.pid))
        d.check("the check's own portal serves, with the session's display", ours,
                f"{serving} serves, ours is {portal.pid}, with {display_of(portal.pid)}")
        if not ours:
            raise RuntimeError("the portal isn't ours, so a pass or a failure would say nothing")

        app = d.App(None, ["--root="])
        tabs = d.tabs_of(app)
        app.cmd("opentab", tabs["tabs"][0]["workspace"], URL)
        d.check("the app runs on the first commit", on(first), running_commits())
        second = commit("2")
        d.check("the repository has a second commit", second != first, second)

        deadline = time.time() + 180
        while time.time() < deadline and not timeline_has(app, "ready to relaunch"):
            time.sleep(2)
        d.check("the app hears of it and makes it, ready to relaunch", timeline_has(app, "ready to relaunch"))
        monitor = None
        if CLICK:
            monitor = subprocess.Popen(["dbus-monitor", "--session", "member=ActionInvoked"],
                                       stdout=open(ACTIONS, "w"), stderr=subprocess.STDOUT)
            open(READY, "w").close()
        else:
            app.js("chrome.send('shell', [JSON.stringify({t: 'relaunch'})]), 1")
        try:
            app.proc.wait(timeout=180 if CLICK else 60)
            d.App.running.remove(app)
            quit_ok = True
        except Exception:  # noqa: BLE001
            quit_ok = False
        if monitor:
            monitor.terminate()
        d.check("Relaunch " + ("in the notification " if CLICK else "") + "quits the running copy", quit_ok)
        deadline = time.time() + 60
        while time.time() < deadline and not on(second):
            time.sleep(1)
        d.check("and a copy on the new commit starts", on(second), running_commits())
        time.sleep(5)
        d.check("and keeps running", on(second), running_commits())
        if d.failures:
            # A copy Spawn starts has the serving portal's environment: one systemd started
            # early may have no display yet.
            print("--- systemd's environment:", [v for v in sh("systemctl", "--user", "show-environment", check=False).split()
                                                if v.split("=")[0] in DISPLAY_VARS], flush=True)
            print(f"--- the serving portal's ({portal_pid()}):", display_of(portal_pid()), flush=True)
            print("--- the portal's output, with the copies it spawned:\n" + open(PORTAL_LOG).read()[-3000:], flush=True)
    except Exception as e:  # noqa: BLE001
        d.check("the check ran to its end", False, e)
    finally:
        subprocess.run(["flatpak", "kill", APP], capture_output=True)
        for a in list(d.App.running):
            try:
                a.proc.kill()
            except Exception:  # noqa: BLE001
                pass
        if portal:
            portal.terminate()
        subprocess.run(["flatpak", "uninstall", "--user", "-y", "--noninteractive", "--delete-data", APP], capture_output=True)
        subprocess.run(["flatpak", "remote-delete", "--user", "--force", REMOTE], capture_output=True)
        subprocess.run(["flatpak", "permission-remove", "flatpak", "updates", APP], capture_output=True)
        shutil.rmtree(WORK, ignore_errors=True)
        if os.path.exists(READY):
            os.remove(READY)
    print(f"{d.failures} FAILED" if d.failures else "all passed", flush=True)
    sys.exit(1 if d.failures else 0)


if __name__ == "__main__":
    main()

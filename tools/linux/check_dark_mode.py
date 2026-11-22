#!/usr/bin/env python3
"""Checks that the app's System theme follows a Linux desktop's dark mode (docs/research/linux.md,
step 4): dark_mode_guest.py turns the desktop light, dark, light, then starts the app on a
dark desktop, and at each step the shell, a page and the window's frame must be the desktop's.
The Mac's half takes a shot of the real window at each step, for ux. Run on the Mac:

    python3 tools/linux/check_dark_mode.py --target <vm> --bundle <app.flatpak> [--commit <sha>]

As check_real_window.py: the VM must be running, in a slot from the tester, and --commit
pushed. Shots and results go to build/dark-mode/<target>/.
"""

import argparse
import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from check_real_window import APP_ID, GUEST, REPO, ROOT, deskrig, run  # noqa: E402

STEPS = (("1-light", False), ("2-dark", True), ("3-light-again", False), ("4-started-dark", True))


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--target", required=True)
    p.add_argument("--bundle", required=True)
    p.add_argument("--commit", default=subprocess.run(["git", "-C", ROOT, "rev-parse", "HEAD"],
                                                      capture_output=True, text=True).stdout.strip())
    a = p.parse_args()
    out = os.path.join(ROOT, "build", "dark-mode", a.target)
    os.makedirs(out, exist_ok=True)
    guest_out = f"{GUEST}/dark"

    print(f"=== install {os.path.basename(a.bundle)} on {a.target}", flush=True)
    deskrig(a.target, "install", a.bundle, timeout=900)
    print(f"=== fetch {a.commit[:8]} into {GUEST}", flush=True)
    run(a.target, f"rm -rf {GUEST}/repo && mkdir -p {GUEST}/repo && "
                  f"python3 -c \"import urllib.request,sys; sys.stdout.buffer.write(urllib.request.urlopen('{REPO}{a.commit}').read())\" "
                  f"| tar -xz -C {GUEST}/repo --strip-components=1", timeout=600)
    binary = run(a.target, f"echo $(flatpak info --show-location {APP_ID})/files/project-b/project_b").strip()
    run(a.target, f"cd {GUEST}/repo && (PB_BIN={binary} PB_ARGS=--no-sandbox nohup python3 "
                  f"tools/linux/dark_mode_guest.py {guest_out} > {GUEST}/dark.log 2>&1 &)")

    results = {}
    for name, _ in STEPS:
        deadline = time.time() + 120
        state = ""
        while time.time() < deadline:
            state = run(a.target, f"cat {guest_out}/{name}.ready >/dev/null 2>&1 && echo ready; "
                                  f"cat {guest_out}/error 2>/dev/null; true").strip()
            if state:
                break
            time.sleep(1)
        if state != "ready":
            print(run(a.target, f"tail -20 {GUEST}/dark.log"))
            print(f"FAIL {name}: the guest -> {state or 'timed out'}", flush=True)
            return 1
        results[name] = json.loads(run(a.target, f"cat {guest_out}/{name}.json"))
        pid = run(a.target, f"cat {guest_out}/pid").strip()
        deskrig(a.target, "raise", pid)
        time.sleep(1)
        deskrig(a.target, "shot", os.path.join(out, f"{name}.png"), pid)
        run(a.target, f"touch {guest_out}/{name}.done")
    with open(os.path.join(out, "results.json"), "w") as f:
        json.dump(results, f, indent=1)

    failures = 0

    def check(what, ok, detail):
        nonlocal failures
        print(("ok   " if ok else "FAIL ") + what + ("" if ok else " -> " + detail), flush=True)
        failures += not ok

    for name, dark in STEPS:
        r = results[name]
        want = "dark" if dark else "light"
        check(f"{name}: the shell is {want}", r["shell"] == want, r["shell"])
        check(f"{name}: a page is {want}", r["page"] == want, r["page"])
    frame = {name: results[name]["frame"] for name, _ in STEPS}
    check("the frame turns with the desktop", frame["1-light"] != frame["2-dark"], json.dumps(frame))
    check("and back", frame["3-light-again"] == frame["1-light"], json.dumps(frame))
    check("started on a dark desktop, the frame is dark's", frame["4-started-dark"] == frame["2-dark"], json.dumps(frame))
    print(f"=== {a.target}: shots in {out}", flush=True)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""The Mac app with 1, 10 and 50 tabs open: memory per process kind, the tab spans' times with
Go's part in them, and Go's GC pauses (docs/benchmarks.md).

For each tab count it starts the app on a new profile, opens that many tabs with the driver,
lets it settle, and reads macOS's footprint of the browser and each child by its kind. Then,
from the app's own timeline, it times New Tab (Cmd+T, then Escape: on Standard that is the
command box), switching tabs (Ctrl+Tab) and closing one (Cmd+W), five of each, as medians.
Each span's ".go" and ".done" stages say when Go began and finished its part, in ms from the
span's start. The driver's stats give Go's heap and its longest GC pause.

    python3 tools/bench/tabs.py
    python3 tools/bench/tabs.py --counts 1 10 50 100 --out build/bench/tabs.json

PB_BIN picks the binary; point it at a package for what a user gets. The tabs are local pages
on two sites, so most share a renderer: the per-tab memory here is the browser's and the
shell's, not a real site's.
"""

import argparse
import collections
import json
import os
import statistics
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.dirname(__file__))
import drive_first_app as d  # noqa: E402
import timeline  # noqa: E402
from memory import footprint, footprints, kind  # noqa: E402

SETTLE = 5
# A press for each span: switching is a click on another tab's row, as Go's tab.activate is the
# shell's answer to it (Ctrl+Tab runs another action).
SPANS = {"tab.new": ("cmd+t", "escape"), "tab.activate": ("click", None), "tab.close": ("cmd+w", None)}


def spans(app, name):
    entries = json.loads(app.cmd("timeline", 0))["result"]["entries"]
    return [s for s in timeline.spans(entries) if s["name"] == name]


def timed(app, name, key, after, times=5):
    """Presses key times times, waiting for each span, and returns the medians in ms: the
    whole span, and when Go's part began and finished inside it, where the span says."""
    rows = []
    for _ in range(times):
        before = len(spans(app, name))
        if key == "click":
            d.click(app, "[role=tab]:not([aria-selected=true])")
        else:
            app.cmd("key", key)
        end = time.time() + 10
        while len(spans(app, name)) <= before and time.time() < end:
            time.sleep(0.05)
        found = spans(app, name)
        if len(found) > before:
            s = found[-1]
            rows.append({"dur": s["dur"] / 1e6, **{k.split(".")[-1]: v / 1e6 for k, v in s["stages"].items()
                                                    if k in (name + ".go", name + ".done")}})
        if after:
            app.cmd("key", after)
        time.sleep(0.3)
    if not rows:
        return {}
    return {k: statistics.median(r[k] for r in rows if k in r) for k in rows[0]}


def measure(count):
    app = d.App(tempfile.mkdtemp(prefix="pb-tabs-"))
    try:
        hosts = [d.PAGES, d.PAGES.replace("127.0.0.1", "localhost")]
        for i in range(count):
            app.cmd("opentab", "", f"{hosts[i % 2]}store?{i}")
        time.sleep(SETTLE)
        row = collections.Counter({"browser": footprint(app.proc.pid)})
        for pid in subprocess.run(["pgrep", "-P", str(app.proc.pid)], capture_output=True, text=True).stdout.split():
            mine, graphics = footprints(pid)
            row[kind(pid)] += mine
            row["purgeable graphics, apart"] += graphics
            row["renderers"] += kind(pid) == "renderer"
        out = {"MiB": dict(row), "total MiB": sum(v for k, v in row.items() if k not in ("renderers", "purgeable graphics, apart"))}
        # Switching needs two tabs, and each close a tab to spare, so the window stays: close
        # is timed last, on the tabs opened above.
        stats = json.loads(app.cmd("stats"))
        for name, (key, after) in SPANS.items():
            times = 5 if name == "tab.new" else min(5, count - 1)
            out[name] = timed(app, name, key, after, times) if times > 0 else {}
        go = stats.get("go", {})
        go.pop("hooks", None)
        out["go"] = {**go, "cgoCallNs": stats.get("cgoCallNs")}
        return out
    finally:
        app.close()


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--counts", type=int, nargs="+", default=[1, 10, 50])
    p.add_argument("--out", help="also write the results as JSON")
    a = p.parse_args()
    os.makedirs(d.OUT, exist_ok=True)
    measure(2)  # A warm-up for the OS's caches.
    res = {n: measure(n) for n in a.counts}
    if a.out:
        os.makedirs(os.path.dirname(a.out) or ".", exist_ok=True)
        with open(a.out, "w") as f:
            json.dump(res, f, indent=1)
    print(json.dumps(res, indent=1))


if __name__ == "__main__":
    main()

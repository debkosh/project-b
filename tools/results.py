#!/usr/bin/env python3
"""Test results kept on this Mac, and the report a release puts in the repo (docs/testing.md).

    tools/results.py run [--group G] [--name N] [--rerun yes] -- command ...
        Runs one test as tools/test.sh's run() does, its output passed through unchanged, and
        writes its result into this run's folder: status, time, checks, failures, parts, the
        log, and the memory and CPU of the app's processes, sampled every 2 s. On its own it
        wraps any driver; --rerun yes says it reruns parts of the test G, N, whose results for
        those parts it replaces in the report.
    tools/results.py report VERSION [--out docs/test-reports]
        Merges every run of VERSION, the latest result of each test, into VERSION.json and
        VERSION.md, compared with the previous version's JSON there. No paths or names of
        this machine go in.

Where: $PB_RESULTS (default ~/PW_TOOLS/project-b-results)/<version>/<run>/. The version is
$PB_VERSION, else the bundle PB_BIN runs from, else VERSION. The run is $PB_RUN, which
tools/test.sh sets once per call unless it is set already, so a release's groups can share one.
"""

import datetime
import getpass
import json
import os
import platform
import re
import socket
import subprocess
import sys
import threading
import time

SCHEMA = 1
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RESULTS = os.environ.get("PB_RESULTS") or os.path.expanduser("~/PW_TOOLS/project-b-results")
ADB = os.path.expanduser("~/Library/Android/sdk/platform-tools/adb")
EVERY = 2.0
HOG_MB = 4096


def utc(t=None):
    return datetime.datetime.fromtimestamp(t or time.time(), datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def sh(*cmd, timeout=10):
    try:
        return subprocess.run(cmd, capture_output=True, text=True, timeout=timeout).stdout
    except (OSError, subprocess.TimeoutExpired):
        return ""


def version():
    if os.environ.get("PB_VERSION"):
        return os.environ["PB_VERSION"]
    b = os.environ.get("PB_BIN", "")
    if "/Contents/MacOS/" in b:
        v = sh("/usr/libexec/PlistBuddy", "-c", "Print CFBundleShortVersionString", b.split("/Contents/MacOS/")[0] + "/Contents/Info.plist").strip()
        if v:
            return v
    with open(os.path.join(ROOT, "VERSION")) as f:
        return f.read().strip()


# What a process is: the Mac app's own, an Android emulator, or an app process on one.
def mac_kind(cmd):
    if re.search(r"(^|/)project_b( |$)", cmd):
        return "browser"
    if "project_b_helper" in cmd:
        m = re.search(r"--type=([a-z-]+)", cmd)
        t = m.group(1) if m else ""
        return {"gpu-process": "gpu", "renderer": "renderer", "utility": "utility"}.get(t, "helper")
    if "qemu-system" in cmd:
        m = re.search(r"-port (\d+)", cmd)
        return "emulator-" + (m.group(1) if m else "?")
    return None


def android_kind(name):
    if not name.startswith("dev.projectb.browser"):
        return None
    if "sandboxed_process" in name:
        return "android-renderer"
    if "privileged_process" in name:
        return "android-gpu"
    return "android-app"


def mb(s):
    # Android top's RES: kilobytes, or with a K, M or G.
    m = re.match(r"([\d.]+)([KMG]?)", s)
    if not m:
        return 0.0
    n = float(m.group(1))
    return {"": n / 1024, "K": n / 1024, "M": n, "G": n * 1024}[m.group(2)]


def machine():
    mem = int(sh("sysctl", "-n", "hw.memsize").strip() or 0)
    swap = sh("sysctl", "-n", "vm.swapusage").strip()
    emulators = {}
    for line in sh("ps", "-axo", "rss=,command=").splitlines():
        rss, _, cmd = line.strip().partition(" ")
        k = mac_kind(cmd)
        if k and k.startswith("emulator-"):
            emulators[k] = round(int(rss) / 1024)
    return {"model": sh("sysctl", "-n", "hw.model").strip(), "os": "macOS " + sh("sw_vers", "-productVersion").strip(),
            "arch": platform.machine(), "memGB": round(mem / 2**30), "swap": swap, "emulatorsMB": emulators}


def tested():
    """The commit of the build under test, which can differ from the checkout's: build.sh's
    note beside the binary or in the bundle, or the commit in the APK's name."""
    out = {}
    b = os.environ.get("PB_BIN", "")
    for note in (os.path.join(os.path.dirname(b), "commit"), b.split("/Contents/MacOS/")[0] + "/Contents/Resources/commit"):
        if b and os.path.isfile(note):
            out["app"] = open(note).read().strip()
            break
    m = re.search(r"([0-9a-f]{7,40})\.apk$", os.environ.get("PB_APK", ""))
    if m:
        out["apk"] = m.group(1)
    return out


def run_dir():
    d = os.path.join(RESULTS, version(), os.environ.get("PB_RUN") or time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()))
    os.makedirs(os.path.join(d, "tests"), exist_ok=True)
    meta = os.path.join(d, "run.json")
    if not os.path.exists(meta):
        commit = sh("git", "-C", ROOT, "rev-parse", "HEAD").strip()
        dirty = bool(sh("git", "-C", ROOT, "status", "--porcelain", "--untracked-files=no").strip())
        with open(meta, "w") as f:
            json.dump({"schema": SCHEMA, "version": version(), "commit": commit, "dirty": dirty, "tested": tested(), "start": utc(),
                       "app": {k: os.environ[k] for k in ("PB_BIN", "PB_DMG", "PB_APK") if os.environ.get(k)},
                       "machine": machine()}, f, indent=1)
    return d


class Sampler(threading.Thread):
    """Every EVERY seconds: each kind's memory (MB, summed over its processes) and CPU (%),
    counting only the test's own processes, those under root, as other sessions run the app
    too; and any process on the Mac over HOG_MB. On Android, every fourth time, as adb is
    slow, the app on the test's own emulator, ADB_SERIAL or ANDROID_SERIAL, if it has one."""

    def __init__(self, root):
        super().__init__(daemon=True)
        self.root, self.samples, self.hogs, self.stop = root, [], [], threading.Event()
        self.serial = os.environ.get("ADB_SERIAL") or os.environ.get("ANDROID_SERIAL")

    def run(self):
        n = 0
        while not self.stop.is_set():
            t, kinds, rows, children = time.time(), {}, [], {}
            for line in sh("ps", "-axo", "pid=,ppid=,rss=,%cpu=,command=").splitlines():
                parts = line.split(None, 4)
                if len(parts) == 5:
                    rows.append(parts)
                    children.setdefault(int(parts[1]), []).append(int(parts[0]))
            mine, todo = set(), [self.root]
            while todo:
                p = todo.pop()
                mine.add(p)
                todo += children.get(p, [])
            for pid, _, rss, cpu, cmd in rows:
                m = int(rss) / 1024
                k = mac_kind(cmd)
                if k and int(pid) in mine:
                    a = kinds.setdefault(k, [0.0, 0.0])
                    a[0] += m
                    a[1] += float(cpu)
                if m > HOG_MB:
                    self.hogs.append({"t": utc(t), "MB": round(m), "what": k or os.path.basename(cmd.split()[0])})
            if n % 4 == 0 and self.serial and os.path.exists(ADB):
                for serial in [self.serial]:
                    for row in sh(ADB, "-s", serial, "shell", "top", "-b", "-n", "1", "-q", "-o", "RES,%CPU,NAME").splitlines():
                        f = row.split()
                        k = android_kind(f[2]) if len(f) >= 3 else None
                        if k:
                            a = kinds.setdefault(f"{k}@{serial}", [0.0, 0.0])
                            a[0] += mb(f[0])
                            a[1] += float(f[1])
            for k, (m, c) in kinds.items():
                self.samples.append((t, k, m, c))
            n += 1
            self.stop.wait(EVERY)


def grows(series):
    """Memory that climbs through a part and never comes back: the lowest of its last third
    is above the highest of its first, by 100 MB and a quarter at least."""
    if len(series) < 6:
        return False
    third = len(series) // 3
    first, last = series[:third], series[-third:]
    return min(last) > max(first) and series[-1] - series[0] > max(100, 0.25 * series[0])


def summarise(samples, start, end):
    by = {}
    for t, k, m, c in samples:
        if start <= t < end:
            by.setdefault(k, []).append((m, c))
    out = {}
    for k, v in sorted(by.items()):
        ms, cs = [m for m, _ in v], [c for _, c in v]
        out[k] = {"samples": len(v), "peakMB": round(max(ms)), "avgMB": round(sum(ms) / len(ms)),
                  "peakCPU": round(max(cs)), "avgCPU": round(sum(cs) / len(cs)), "grows": grows(ms)}
    return out


def cmd_run(args):
    i = args.index("--")
    opts, cmd = dict(zip(args[:i:2], args[1:i:2])), args[i + 1:]
    opts.setdefault("--group", "direct")
    opts.setdefault("--name", " ".join(os.path.basename(c) for c in cmd[:3]))
    d = run_dir()
    n = len([f for f in os.listdir(os.path.join(d, "tests")) if f.endswith(".json")]) + 1
    slug = re.sub(r"[^a-z0-9]+", "-", opts["--name"].lower()).strip("-")[:60]
    base = os.path.join(d, "tests", f"{n:02d}-{slug}")
    start, lines = time.time(), []
    with open(base + ".log", "w") as log:
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, errors="replace")
        sampler = Sampler(p.pid)
        sampler.start()
        for line in p.stdout:
            sys.stdout.write(line)
            sys.stdout.flush()
            log.write(line)
            lines.append((time.time(), line.rstrip("\n")))
        code = p.wait()
    end = time.time()
    sampler.stop.set()
    sampler.join(5)

    # Parts: "--- part <name>" lines, or the whole test as one.
    marks = [(t, l[len("--- part "):].strip()) for t, l in lines if l.startswith("--- part ")] or [(start, "all")]
    if marks[0][1] != "all" and any(l.startswith(("ok ", "FAIL")) for t, l in lines if t < marks[0][0]):
        marks.insert(0, (start, "(start)"))
    parts = []
    for j, (t, name) in enumerate(marks):
        until = marks[j + 1][0] if j + 1 < len(marks) else end + 1
        mine = [l for lt, l in lines if t <= lt < until]
        parts.append({"name": name, "seconds": round(min(until, end) - t, 1),
                      "ok": sum(l.startswith("ok ") for l in mine), "fail": sum(l.startswith("FAIL") for l in mine),
                      "failures": [l for l in mine if l.startswith("FAIL")][:50],
                      "processes": summarise(sampler.samples, t, until)})
    result = {"schema": SCHEMA, "group": opts["--group"], "name": opts["--name"], "rerun": opts.get("--rerun") == "yes",
              "tested": tested(), "app": {k: os.environ[k] for k in ("PB_BIN", "PB_DMG", "PB_APK") if os.environ.get(k)},
              "status": "passed" if code == 0 else "did_not_run" if code == 2 else "failed", "exit": code,
              "start": utc(start), "seconds": round(end - start, 1),
              "ok": sum(p["ok"] for p in parts), "fail": sum(p["fail"] for p in parts),
              "failures": [f for p in parts for f in p["failures"]][:100], "log": os.path.basename(base) + ".log",
              "parts": parts, "processes": summarise(sampler.samples, start, end + 1), "hogs": sampler.hogs[:200]}
    with open(base + ".json", "w") as f:
        json.dump(result, f, indent=1)
    sys.exit(code)


# The repo copy carries nothing of this machine but its model and OS.
def scrub(s):
    home = os.path.expanduser("~")
    for private, public in ((home, "~"), (socket.gethostname().split(".")[0], "<host>"), (getpass.getuser(), "<user>")):
        if private:
            s = s.replace(private, public)
    s = re.sub(r"/(private/)?(var/folders|tmp)/[^\s'\"]*", "<tmp>", s)
    return re.sub(r"\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d+", "<local>", s)


def vkey(v):
    return tuple(int(x) for x in re.findall(r"\d+", v))


def cmd_report(args):
    ver = args[0]
    out = args[args.index("--out") + 1] if "--out" in args else os.path.join(ROOT, "docs", "test-reports")
    vdir = os.path.join(RESULTS, ver)
    latest, reruns, commits, machines = {}, [], set(), []
    for run in sorted(os.listdir(vdir)):
        meta = json.load(open(os.path.join(vdir, run, "run.json")))
        commits.add(meta["commit"][:8] + ("+" if meta.get("dirty") else ""))
        machines.append(meta["machine"])
        tdir = os.path.join(vdir, run, "tests")
        for f in sorted(os.listdir(tdir)):
            if f.endswith(".json"):
                t = json.load(open(os.path.join(tdir, f)))
                key = (t["group"], t["name"])
                if t.get("rerun"):
                    reruns.append(t)
                    continue
                for kind, c in t.get("tested", {}).items():
                    commits.add(f"{kind} {c[:8]}")
                if key not in latest or t["start"] > latest[key]["start"]:
                    latest[key] = t
    # A rerun of some parts replaces those parts' results only.
    for r in sorted(reruns, key=lambda r: r["start"]):
        base = latest.get((r["group"], r["name"]))
        if not base:
            latest[(r["group"], r["name"])] = r
            continue
        names = {p["name"] for p in r["parts"]}
        base["parts"] = [p for p in base["parts"] if p["name"] not in names] + [dict(p, rerun=True) for p in r["parts"]]
        base["ok"], base["fail"] = sum(p["ok"] for p in base["parts"]), sum(p["fail"] for p in base["parts"])
        base["failures"] = [f for p in base["parts"] for f in p["failures"]]
        base["status"] = "failed" if base["fail"] else "passed"
        base.setdefault("reruns", []).extend(sorted(names))
    known = json.load(open(os.path.join(ROOT, "tools", "known-failures.json")))["known"]

    def why(t, line):
        for k in known:
            if k["group"] == t["group"] and k.get("test", "") in t["name"] and re.search(k["match"], line):
                return k["why"]
        return None

    tests = []
    for (group, name), t in sorted(latest.items()):
        fails = [{"line": scrub(f), "known": why(t, f)} for f in t["failures"][:40]]
        status = t["status"]
        if status == "failed" and fails and all(f["known"] for f in fails):
            status = "known"
        elif status == "failed" and not fails and why(t, name):
            status = "known"
        tests.append({"group": group, "name": name, "status": status, "seconds": t["seconds"], "ok": t["ok"],
                      "fail": t["fail"], "failures": fails, "reruns": t.get("reruns", []),
                      "processes": {k.split("@")[0]: {x: p[x] for x in ("peakMB", "avgMB", "peakCPU", "avgCPU", "grows")}
                                    for k, p in t["processes"].items() if not k.startswith("emulator-") and p["peakMB"] >= 10},
                      "grows": sorted({f"{p['name']}: {k.split('@')[0]}" for p in t["parts"] for k, v in p["processes"].items() if v["grows"]}),
                      "hogs": sorted({h["what"] for h in t.get("hogs", [])})})
    groups = {}
    for t in tests:
        g = groups.setdefault(t["group"], {"passed": 0, "failed": 0, "known": 0, "did_not_run": 0, "ok": 0, "fail": 0})
        g[t["status"]] += 1
        g["ok"] += t["ok"]
        g["fail"] += t["fail"]
    m = machines[-1] if machines else {}
    new = sum(1 for t in tests for f in t["failures"] if not f["known"])
    report = {"schema": SCHEMA, "version": ver, "date": utc()[:10], "commits": sorted(commits), "newFailures": new,
              "machine": {"model": m.get("model"), "os": m.get("os"), "memGB": m.get("memGB")}, "groups": groups, "tests": tests}

    os.makedirs(out, exist_ok=True)
    prev = None
    olders = sorted((f[:-5] for f in os.listdir(out) if f.endswith(".json") and vkey(f[:-5]) < vkey(ver)), key=vkey)
    if olders:
        prev = json.load(open(os.path.join(out, olders[-1] + ".json")))
    with open(os.path.join(out, ver + ".json"), "w") as f:
        json.dump(report, f, indent=1)
    with open(os.path.join(out, ver + ".md"), "w") as f:
        f.write(markdown(report, prev))
    print(os.path.join(out, ver + ".md"))


def markdown(r, prev):
    L = [f"# Test report, Project B {r['version']}", "",
         f"{r['date']}, from {', '.join(r['commits'])}, on {r['machine']['model']} with {r['machine']['os']} and {r['machine']['memGB']} GB.",
         f"Format: [docs/testing.md](../testing.md#test-results).", "",
         f"**{r['newFailures']} new failures.**" if r["newFailures"] else "**0 new failures**: every failure is a known one.", "", "## Groups", "",
         "| Group | Tests passed | Failed | Known | Did not run | Checks ok | Checks failed |", "|---|---|---|---|---|---|---|"]
    pg = (prev or {}).get("groups", {})
    for g, v in sorted(r["groups"].items()):
        was = pg.get(g)
        d = f" (was {was['ok']})" if was and was["ok"] != v["ok"] else ""
        L.append(f"| {g} | {v['passed']} | {v['failed']} | {v['known']} | {v['did_not_run']} | {v['ok']}{d} | {v['fail']} |")
    pt = {(t["group"], t["name"]): t for t in (prev or {}).get("tests", [])}
    changed = [t for t in r["tests"] if (t["group"], t["name"]) in pt and pt[(t["group"], t["name"])]["status"] != t["status"]]
    if prev:
        L += ["", f"## Since {prev['version']}", ""]
        L += [f"- {t['group']}, {t['name']}: {pt[(t['group'], t['name'])]['status']} → {t['status']}" for t in changed] or ["- No test changed status."]
    failing = [t for t in r["tests"] if t["status"] != "passed"]
    if failing:
        L += ["", "## Not passed", ""]
        for t in failing:
            rerun = f", parts rerun: {', '.join(t['reruns'])}" if t["reruns"] else ""
            L.append(f"- **{t['group']}, {t['name']}**: {t['status']}, {t['fail']} checks failed{rerun}")
            L += [f"  - {'known: ' if f['known'] else 'NEW: '}`{f['line'][:200]}`" for f in t["failures"][:8]]
    L += ["", "## Memory and CPU", "", "Peak and average over each test, summed per kind of process; CPU in % of one core.", "",
          "| Group | Test | Process | Peak MB | Avg MB | Peak CPU | Avg CPU | Peak MB before |", "|---|---|---|---|---|---|---|---|"]
    for t in r["tests"]:
        before = pt.get((t["group"], t["name"]), {}).get("processes", {})
        for k, p in sorted(t["processes"].items()):
            b = before.get(k, {}).get("peakMB", "")
            L.append(f"| {t['group']} | {t['name']} | {k} | {p['peakMB']} | {p['avgMB']} | {p['peakCPU']} | {p['avgCPU']} | {b} |")
    grow = [(t, g) for t in r["tests"] for g in t["grows"]]
    L += ["", "## Memory that grew and stayed", ""]
    L += [f"- {t['group']}, {t['name']}, part {g}" for t, g in grow] or ["- None."]
    hogs = sorted({h for t in r["tests"] for h in t["hogs"]})
    if hogs:
        L += ["", f"Over {HOG_MB // 1024} GB on the Mac at some point during the tests, the app's or not: {', '.join(hogs)}. The run folders say when."]
    return "\n".join(L) + "\n"


if __name__ == "__main__":
    if len(sys.argv) > 2 and sys.argv[1] == "run" and "--" in sys.argv:
        cmd_run(sys.argv[2:])
    elif len(sys.argv) > 2 and sys.argv[1] == "report":
        cmd_report(sys.argv[2:])
    else:
        print(__doc__)
        sys.exit(2)

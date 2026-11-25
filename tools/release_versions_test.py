#!/usr/bin/env python3
"""tools/release_versions.py: which Dev files go, which version is newer, and the rollback edit."""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import release_versions as rv  # noqa: E402

failed = 0


def check(name, ok, detail=""):
    global failed
    print(("ok   " if ok else "FAIL ") + name + ("" if ok else f" -> {detail}"))
    failed += not ok


def files(v):
    return [f"Project-B-{v}.dmg", f"Project-B-{v}.apk", f"Project-B-{v}-darwin-arm64.zip", f"Project-B-{v}-android-arm64.zip", f"notes-{v}.md"]


A, B, C = "0.3.11-dev.202611240729", "0.3.11-dev.202611250935", "0.3.11-dev.202611260300"
names = files(A) + files(B) + files(C) + ["releases.json", "notes-0.3.10.md", "notes-0.3.9.md"]
gone = rv.prune_dev(C, names)
check("publishing C deletes only the Dev build two back, A, all five of its files", sorted(gone) == sorted(files(A)), gone)
check("it keeps B, the Dev build before C, so a rollback to B is one manifest edit", not set(files(B)) & set(gone), gone)
check("Stable notes and the manifest stay", not {"releases.json", "notes-0.3.10.md", "notes-0.3.9.md"} & set(gone), gone)
check("the first Dev build after a rollback to a Stable keeps the last Dev build's files", rv.prune_dev(C, files(B) + files(C)) == [], rv.prune_dev(C, files(B) + files(C)))

check("a later Dev build is newer", rv.newer(C, B))
check("a Stable is newer than its own Dev builds", rv.newer("0.3.11", C))
check("a Dev build of the next patch is newer than the Stable", rv.newer(B, "0.3.10"))
check("the same version isn't newer", not rv.newer(B, B))
try:
    rv.newer("0.3.11-dev", B)
    check("a Dev version without its time is refused", False, "accepted")
except ValueError:
    check("a Dev version without its time is refused", True)

m = {"spec": 1, "channels": {"dev": B, "stable": "0.3.10"}, "releases": {"0.3.10": {"assets": {}}, B: {"assets": {}}}}
out = rv.set_dev(m, "0.3.10")
check("set-dev moves only the dev channel", out["channels"] == {"dev": "0.3.10", "stable": "0.3.10"} and out["releases"] == m["releases"], out)
check("set-dev leaves the manifest it was given as it was", m["channels"]["dev"] == B)
try:
    rv.set_dev(m, A)
    check("set-dev refuses a version the manifest doesn't list", False, "accepted")
except ValueError:
    check("set-dev refuses a version the manifest doesn't list", True)

print("all passed" if not failed else f"{failed} FAILED")
sys.exit(1 if failed else 0)

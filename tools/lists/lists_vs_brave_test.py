"""tools/lists-vs-brave.py against a small Brave catalogue: python3 -I tools/lists/lists_vs_brave_test.py"""
import datetime
import importlib.util
import os
import sys

here = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("lvb", os.path.join(here, "..", "lists-vs-brave.py"))
lvb = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lvb)

ours = {
    "groups": [{"id": "ads", "blocker": "network"}, {"id": "malware", "blocker": "safe"}],
    "lists": [
        {"id": "el", "group": "ads", "on": True, "brave": "default", "sources": [{"url": "https://a/el.txt"}]},
        {"id": "ub", "group": "ads", "on": True, "brave": "default", "sources": [{"url": "https://a/old.txt"}]},
        {"id": "gone", "group": "ads", "on": False, "brave": "G", "sources": [{"url": "https://a/g.txt"}]},
        {"id": "bad", "group": "malware", "on": True, "sources": [{"url": "https://a/bad.txt"}]},
        {"id": "fresh", "group": "malware", "on": True, "sources": [{"url": "https://a/fresh.txt"}]},
    ],
    "ignored": [
        {"brave": "Y", "title": "YouTube", "reason": "r"},
        {"brave": "default", "source": "https://a/brave-specific.txt", "title": "t", "reason": "r"},
        {"brave": "C", "default_enabled": True, "title": "Cookies", "reason": "r"},
    ],
}
brave = [
    {"uuid": "default", "title": "Default", "default_enabled": True,
     "sources": [{"url": "https://a/el.txt"}, {"url": "https://a/new.txt"}, {"url": "https://a/brave-specific.txt"}]},
    {"uuid": "Y", "title": "YouTube", "default_enabled": False, "sources": [{"url": "https://a/yt.txt"}]},
    {"uuid": "C", "title": "Cookies", "default_enabled": False, "sources": []},
    {"uuid": "N", "title": "New", "default_enabled": True, "sources": [{"url": "https://a/n.txt"}]},
]
got = lvb.catalogue_diff(ours, brave)
want = [
    "Brave added New (N, on by default): add it to the catalogue, or to ignored with a reason",
    "Brave dropped G, which our gone follow",
    "Brave turned Cookies off by default; ignored says it was on",
    "Default: a source moved, https://a/old.txt -> https://a/new.txt",
    "New: Brave added the source https://a/n.txt",
]
now = datetime.datetime(2026, 11, 27, tzinfo=datetime.timezone.utc)
manifest = {"lists": {"bad": {"changed": "2026-11-19T00:00:00Z", "size": 10}, "fresh": {"changed": "2026-11-26T00:00:00Z", "size": 20}}}
safe = lvb.safe_report(ours, manifest, now)
ok = got == want and "unchanged for 8 days" in safe[0] and "unchanged" not in safe[1]
if not ok:
    print("FAIL lists-vs-brave:\n  " + "\n  ".join(got) + "\n  " + "\n  ".join(safe))
    sys.exit(1)
print("ok   lists-vs-brave finds Brave's changes and the stale Safe Browsing list")

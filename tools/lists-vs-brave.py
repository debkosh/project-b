#!/usr/bin/env python3
"""Our blocking catalogue against Brave's, for the releaser (docs/blocking.md, "The catalogue").

Prints what differs between core/blocking/catalog.json and Brave's filter_lists/list_catalog.json:
lists Brave added or dropped (by uuid), lists whose default_enabled differs from our "on", and
sources whose address moved. What the catalogue's "ignored" section names is skipped: a whole
Brave list, one of its sources ("source"), or its default ("default_enabled", raised again when
Brave's changes). Then, for the Safe Browsing groups, each list's last change and size from our
lists release, flagging any unchanged for 7 days. It never fails: it always exits 0.

  python3 -I tools/lists-vs-brave.py [--brave saved-catalog.json] [--manifest saved-manifest.json]
"""
import argparse
import datetime
import json
import os
import sys
import urllib.request

BRAVE = "https://raw.githubusercontent.com/brave/adblock-resources/master/filter_lists/list_catalog.json"
MANIFEST = "https://github.com/debkosh/project-b/releases/download/lists/manifest.json"
OURS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "core", "blocking", "catalog.json")
STALE_DAYS = 7


def fetch_json(where):
    if where.startswith("https://"):
        req = urllib.request.Request(where, headers={"User-Agent": "project-b-lists-vs-brave/1"})
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.loads(r.read())
    with open(where) as f:
        return json.load(f)


def catalogue_diff(ours, brave):
    out = []
    by_uuid = {b["uuid"]: b for b in brave}
    whole = {i["brave"] for i in ours.get("ignored", []) if "source" not in i and "default_enabled" not in i}
    ign_src = {(i["brave"], i["source"]) for i in ours.get("ignored", []) if "source" in i}
    ign_def = {i["brave"]: i["default_enabled"] for i in ours.get("ignored", []) if "default_enabled" in i}
    ours_by_uuid = {}
    for l in ours["lists"]:
        if l.get("brave"):
            ours_by_uuid.setdefault(l["brave"], []).append(l)
    our_urls = {s["url"] for l in ours["lists"] for s in l["sources"]}
    brave_urls = {s["url"] for b in brave for s in b.get("sources", [])}

    for u, b in by_uuid.items():
        if u not in ours_by_uuid and u not in whole and u not in ign_def:
            out.append("Brave added %s (%s, %s by default): add it to the catalogue, or to ignored with a reason"
                       % (b["title"], u, "on" if b.get("default_enabled") else "off"))
    for u, ls in ours_by_uuid.items():
        if u not in by_uuid:
            out.append("Brave dropped %s, which our %s follow" % (u, ", ".join(l["id"] for l in ls)))
    for i in ours.get("ignored", []):
        if i["brave"] not in by_uuid:
            out.append("ignored names %s (%s), which Brave no longer has: take it out of ignored" % (i["brave"], i["title"]))

    for u, b in by_uuid.items():
        if u in whole:
            continue
        brave_on = bool(b.get("default_enabled"))
        if u in ign_def:
            if ign_def[u] != brave_on:
                out.append("Brave turned %s %s by default; ignored says it was %s"
                           % (b["title"], "on" if brave_on else "off", "on" if ign_def[u] else "off"))
            continue
        ls = ours_by_uuid.get(u)
        if ls is None:
            continue
        ours_on = any(l.get("on") for l in ls)
        if ours_on != brave_on:
            out.append("Brave has %s %s by default; ours (%s) %s"
                       % (b["title"], "on" if brave_on else "off", ", ".join(l["id"] for l in ls), "on" if ours_on else "off"))

    for u, b in by_uuid.items():
        if u in whole:
            continue
        new = [s["url"] for s in b.get("sources", []) if s["url"] not in our_urls and (u, s["url"]) not in ign_src]
        gone = [s["url"] for l in ours_by_uuid.get(u, []) for s in l["sources"] if s["url"] not in brave_urls]
        for old_url, new_url in zip(gone, new):
            out.append("%s: a source moved, %s -> %s" % (b["title"], old_url, new_url))
        for new_url in new[len(gone):]:
            out.append("%s: Brave added the source %s" % (b["title"], new_url))
        for old_url in gone[len(new):]:
            out.append("%s: Brave dropped the source %s, which ours still takes" % (b["title"], old_url))
    for (u, url) in sorted(ign_src):
        if url not in brave_urls:
            out.append("ignored names the source %s, which Brave no longer has: take it out of ignored" % url)
    return out


def safe_report(ours, manifest, now):
    out = []
    safe = {g["id"] for g in ours["groups"] if g["blocker"] == "safe"}
    lists = manifest.get("lists", {})
    for l in ours["lists"]:
        if l["group"] not in safe:
            continue
        e = lists.get(l["id"])
        if e is None:
            out.append("  %s: not in the release" % l["id"])
            continue
        changed = datetime.datetime.fromisoformat(e["changed"].replace("Z", "+00:00"))
        days = (now - changed).total_seconds() / 86400
        flag = "  <- unchanged for %d days" % days if days >= STALE_DAYS else ""
        out.append("  %s (%s): changed %s, %d bytes%s" % (l["id"], l["group"], e["changed"], e["size"], flag))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--brave", default=BRAVE, help="Brave's catalogue: its address or a saved copy")
    ap.add_argument("--manifest", default=MANIFEST, help="our lists release's manifest: its address or a saved copy")
    ap.add_argument("--ours", default=OURS)
    a = ap.parse_args()
    try:
        with open(a.ours) as f:
            ours = json.load(f)
    except Exception as e:
        print("lists-vs-brave: can't read our catalogue: %s" % e)
        return
    try:
        diff = catalogue_diff(ours, fetch_json(a.brave))
        if diff:
            print("lists-vs-brave: %d differences from Brave's catalogue; send any change as its own commit:" % len(diff))
            for d in diff:
                print("  " + d)
        else:
            print("lists-vs-brave: the catalogue follows Brave's")
    except Exception as e:
        print("lists-vs-brave: no comparison with Brave's catalogue: %s" % e)
    try:
        lines = safe_report(ours, fetch_json(a.manifest), datetime.datetime.now(datetime.timezone.utc))
        print("lists-vs-brave: the Safe Browsing lists in our release:")
        for line in lines:
            print(line)
    except Exception as e:
        print("lists-vs-brave: no Safe Browsing report from our lists release: %s" % e)


if __name__ == "__main__":
    try:
        main()
    except Exception as e:  # never fails a release
        print("lists-vs-brave: %s" % e)
    sys.exit(0)

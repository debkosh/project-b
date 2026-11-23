#!/usr/bin/env python3
"""Scores the roles Go gave the fields of DuckDuckGo's labelled forms against their labels
(tools/form-matching.sh runs it; docs/password-autofill.md, "Detection").

    score.py fields.jsonl roles.jsonl [--record SOURCE.json --commit C --date D --sha256 S --bytes N]

It prints each count, beside the one SOURCE.json recorded when it holds one, and with
--record writes the new ones there.
"""
import argparse
import collections
import json
import os

p = argparse.ArgumentParser()
p.add_argument("fields")
p.add_argument("roles")
p.add_argument("--record")
p.add_argument("--commit")
p.add_argument("--date")
p.add_argument("--sha256")
p.add_argument("--bytes", type=int)
a = p.parse_args()

fields = {}
errors = 0
for line in open(a.fields):
    f = json.loads(line)
    errors += "error" in f
    fields[f["file"]] = f
roles = {json.loads(line)["file"]: json.loads(line) for line in open(a.roles)}
PASSWORDS = {"current-password", "new-password", "confirm-password"}
n = collections.Counter()
for name, f in fields.items():
    given = roles.get(name, {}).get("roles", {})
    told = {x["ref"]: x for x in f.get("told", {}).get("fields", [])}
    for lab in f.get("labelled", []):
        label, ours = lab["label"], given.get(lab["ref"], "")
        hidden = lab["ref"] in told and not told[lab["ref"]].get("visible", True)
        if label in ("password", "password.current", "password.new"):
            n["password fields"] += 1
            n["password fields found"] += ours in PASSWORDS
            n["password fields not found, hidden in the page"] += ours not in PASSWORDS and hidden
            n["new and current passwords confused"] += (label == "password.new" and ours == "current-password") or (label == "password.current" and ours in ("new-password", "confirm-password"))
        elif label in ("username", "username OR emailAddress"):
            n["username fields"] += 1
            n["username fields found"] += ours == "username"
            n["username fields not found, hidden in the page"] += ours != "username" and hidden
        elif label == "totp":
            n["one-time code fields"] += 1
            n["one-time code fields found"] += ours == "one-time-code"
        elif label != "emailAddress":
            n["other fields"] += 1
            n["other fields wrongly given a role"] += bool(ours)
order = ["password fields", "password fields found", "password fields not found, hidden in the page", "new and current passwords confused", "username fields", "username fields found",
         "username fields not found, hidden in the page", "one-time code fields", "one-time code fields found", "other fields", "other fields wrongly given a role"]
scores = {k: n[k] for k in order}
scores["pages"] = len(fields)
scores["pages that didn't load"] = errors
before = {}
if a.record and os.path.exists(a.record):
    before = json.load(open(a.record)).get("scores", {})
for k, v in scores.items():
    was = before.get(k)
    print("%6d  %s%s" % (v, k, "" if was is None or was == v else "   (was %d)" % was))
if a.record:
    with open(a.record, "w") as out:
        json.dump({"source": "https://github.com/duckduckgo/duckduckgo-autofill", "license": "Apache-2.0", "commit": a.commit, "date": a.date,
                   "built": "core/forms/page/matching.gen.js", "bytes": a.bytes, "sha256": a.sha256, "scores": scores}, out, indent=2)
        out.write("\n")
    print("recorded in", a.record)

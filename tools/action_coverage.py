#!/usr/bin/env python3
"""Every action a person can take in Project B, and the test that does it (docs/action-coverage.md).

    tools/action_coverage.py           writes docs/action-coverage.md
    tools/action_coverage.py --check   exits 1 if an action has no test and no reason in
                                       tools/action-coverage-exempt.json (tools/test.sh mock runs it)

The actions, read from the code so a new one can't be missed:
- Go's actions (core/app/actions.go, and each shell's in core/shells/registry.json): what the
  menu bar, the keys and the shells' own menus run, by id, such as tab.duplicate.
- What the shells send Go when a person does something (send({ t: 'closeTab' ... }) in
  shell/src): a context menu's item, a tab strip's button, a phone sheet's row.
Where each is offered: the menu bar template and keymaps that name it, and the shells whose
source runs or sends it.

A test covers an action when it names it: the id (fire('menu', 'tab.duplicate'), a "run"
message, Go's own tests) or the message ("t":"closeTab", t: 'closeTab'), or, for Go's actions,
the words the menu shows (a driver clicking "Duplicate Tab"). That is a test doing the action;
whether it checks the result is the test's, and a reviewer's, business. Where the action is
plumbing, not a person's, or can't be done in a test, the exempt file says why.
"""

import glob
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "docs", "action-coverage.md")
EXEMPT = os.path.join(ROOT, "tools", "action-coverage-exempt.json")


def read(path):
    with open(path, encoding="utf-8", errors="replace") as f:
        return f.read()


def go_actions():
    """id -> title, as registerActions and copyActions add them, and the shells' own."""
    src = read(os.path.join(ROOT, "core", "app", "actions.go"))
    out = dict(re.findall(r'add\("([a-z]+\.[A-Za-z]+)", "([^"]+)"', src))
    out.update(dict(re.findall(r'"([a-z]+\.[A-Za-z]+)", "([^"]+)"', src.split("func copyActions", 1)[1])))
    if "tab.select%d" in src:
        for i in range(1, 9):
            out["tab.select%d" % i] = "Tab %d" % i
    registry = json.load(open(os.path.join(ROOT, "core", "shells", "registry.json")))
    for s in registry if isinstance(registry, list) else registry.get("shells", []):
        for a in s.get("actions", []):
            out[a["id"]] = a.get("title", a["id"])
    return out


SHELLS = sorted(os.path.basename(p) for p in glob.glob(os.path.join(ROOT, "shell", "src", "apps", "shell", "*")) if os.path.isdir(p))
PAGES = sorted(os.path.basename(p) for p in glob.glob(os.path.join(ROOT, "shell", "src", "apps", "*")) if os.path.isdir(p) and not p.endswith("/shell"))


def sources():
    """Each shell's and page's source, by name: the shells' folders, our pages', and the
    shared UI, which every desktop shell uses, as "ui"."""
    out = {}
    for name in SHELLS:
        out[name] = " ".join(read(p) for p in glob.glob(os.path.join(ROOT, "shell", "src", "apps", "shell", name, "**", "*.*"), recursive=True))
    for name in PAGES:
        out["page:" + name] = " ".join(read(p) for p in glob.glob(os.path.join(ROOT, "shell", "src", "apps", name, "**", "*.*"), recursive=True))
    out["ui"] = " ".join(read(p) for p in glob.glob(os.path.join(ROOT, "shell", "src", "ui", "**", "*.*"), recursive=True))
    kotlin = os.path.join(ROOT, "engine", "android", "java")
    out["mobile-native"] = out.get("mobile-native", "") + " ".join(read(p) for p in glob.glob(os.path.join(kotlin, "**", "*.kt"), recursive=True))
    return out


def messages(src):
    """The message types a source sends Go."""
    found = set(re.findall(r"""send\(\{\s*t:\s*['"]([A-Za-z]+)['"]""", src))
    found |= set(re.findall(r'"t"\s*(?:to|,)\s*"([A-Za-z]+)"', src))  # Kotlin: "t" to "closeTab"
    return found


def tests():
    """Every test's text, by path: the shells on the mock, the real-app drivers and checks, the
    VM's, and Go's own."""
    pats = ["shell/e2e/**/*.cjs", "tools/drive_*.py", "tools/check_*.py", "tools/vm/*.py", "core/**/*_test.go"]
    out = {}
    for pat in pats:
        for p in glob.glob(os.path.join(ROOT, pat), recursive=True):
            out[os.path.relpath(p, ROOT)] = read(p)
    return out


REAL = ("tools/drive_", "tools/check_", "tools/vm/")


def covering(test_texts, needles):
    return sorted(p for p, t in test_texts.items() if any(n in t for n in needles))


def real(paths):
    """The tests among paths that drive the real app, where the engine can refuse what the
    mock engine lets through (Duplicate Tab's ERR_BLOCKED_BY_CLIENT, 2026-11-19)."""
    return [p for p in paths if p.startswith(REAL)]


def inventory():
    acts = go_actions()
    src = sources()
    menu = " ".join(read(p) for p in glob.glob(os.path.join(ROOT, "core", "commands", "templates", "menu", "*.json")))
    keymaps = {os.path.basename(p)[:-5]: read(p) for p in glob.glob(os.path.join(ROOT, "core", "commands", "templates", "keymap", "*.json"))}
    tt = tests()
    rows = []
    for aid, title in sorted(acts.items()):
        where = []
        if '"%s"' % aid in menu:
            where.append("menu bar")
        keys = [k for k, v in keymaps.items() if '"%s"' % aid in v]
        if keys:
            where.append("keys (%s)" % ", ".join(sorted(keys)))
        where += [s for s, t in src.items() if "'%s'" % aid in t or '"%s"' % aid in t]
        needles = ["'%s'" % aid, '"%s"' % aid, '\\"%s\\"' % aid, "'%s'" % title, '"%s"' % title]
        rows.append({"kind": "action", "id": aid, "title": title, "where": where, "tests": covering(tt, needles)})
    sent = {}
    for s, t in src.items():
        for m in messages(t):
            sent.setdefault(m, []).append(s)
    for m, where in sorted(sent.items()):
        needles = ["t: '%s'" % m, 't: "%s"' % m, '"t":"%s"' % m, '"t": "%s"' % m, '\\"t\\":\\"%s\\"' % m, "'t': '%s'" % m]
        rows.append({"kind": "message", "id": m, "title": "", "where": sorted(where), "tests": covering(tt, needles)})
    return rows


def exempt():
    try:
        return json.load(open(EXEMPT))["exempt"]
    except (OSError, ValueError, KeyError):
        return {}


def write(rows, ex):
    bare = [r for r in rows if not r["tests"] and r["id"] not in ex]
    L = ["# Action coverage", "",
         "Every action a person can take, where the app offers it, and the tests that do it. Written by",
         "`tools/action_coverage.py` from the code; `tools/test.sh mock` fails while an action has no",
         "test and no reason in `tools/action-coverage-exempt.json` (roles/tester.md).", "",
         "**%d actions and messages, %d with a test, %d exempt with a reason, %d with none; %d of Go's"
         " actions done by no test of the real app.**" % (
             len(rows), sum(1 for r in rows if r["tests"]), sum(1 for r in rows if not r["tests"] and r["id"] in ex), len(bare),
             sum(1 for r in rows if r["kind"] == "action" and not real(r["tests"]) and r["id"] not in ex)), ""]
    for kind, head in (("action", "Go's actions: the menu bar, keys and the shells' own menus"),
                       ("message", "What the shells send Go when a person does something")):
        L += ["## " + head, "", "| Action | Offered in | Real app | Tests |", "|---|---|---|---|"]
        for r in rows:
            if r["kind"] != kind:
                continue
            name = "`%s`%s" % (r["id"], " " + r["title"] if r["title"] else "")
            if r["tests"]:
                t = ", ".join("`%s`" % p.split("/")[-1] for p in r["tests"][:4]) + (" and %d more" % (len(r["tests"]) - 4) if len(r["tests"]) > 4 else "")
            elif r["id"] in ex:
                t = ("**" + ex[r["id"]] + "**") if ex[r["id"]].startswith("TODO") else "exempt: " + ex[r["id"]]
            else:
                t = "**none**"
            L.append("| %s | %s | %s | %s |" % (name, ", ".join(r["where"]) or "", "yes" if real(r["tests"]) else "**no**", t))
        L.append("")
    with open(OUT, "w") as f:
        f.write("\n".join(L))
    return bare


if __name__ == "__main__":
    rows = inventory()
    ex = exempt()
    bare = write(rows, ex)
    noreal = [r for r in rows if r["kind"] == "action" and not real(r["tests"]) and r["id"] not in ex]
    todo = [r for r in rows if not r["tests"] and ex.get(r["id"], "").startswith("TODO")]
    print("%d actions and messages, %d with no test and no reason, %d tests owed (TODO); %d of Go's actions with no real-app test" % (
        len(rows), len(bare), len(todo), len(noreal)))
    if "--real" in sys.argv[1:]:
        for r in noreal:
            print("no real-app test: %s (%s)" % (r["id"], r["title"]))
    if "--check" in sys.argv[1:]:
        for r in bare:
            print("FAIL no test does %s %s %s" % (r["kind"], r["id"], ("(" + r["title"] + ")") if r["title"] else ""))
        sys.exit(1 if bare else 0)

"""The web-features registry, core/webfeatures/registry.json, for the probes that call its
APIs in a page (tools/drive_first_app.py on the Mac, tools/drive_android.py on Android) and
for tools/web_features_bump.py. docs/web-features.md.

A probe's call gives one of:

    absent    it threw at once: the API isn't there
    resolved  its value, or the promise it gave, came back
    rejected  the promise it gave was rejected
    pending   no answer within two seconds
    killed    the page's renderer died

followed, for absent and rejected, by a colon and the error's name, and for resolved by a
colon and the start of the value (rejected:NotAllowedError, resolved:false). Each probe's
expect says what its call gives on each platform, and Go holds that to the row's status
(core/webfeatures/registry.go). A check fails on anything else, naming the row: a row that
starts or stops working fails its probe until the row is changed.
"""

import json
import os

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
REGISTRY = os.path.join(ROOT, "core/webfeatures/registry.json")

# A JavaScript function that runs a call, given as a function, and resolves to what it gave.
OUTCOME = ("(call => { const name = e => (e && (e.name || (e.constructor && e.constructor.name))) || String(e);"
           " const value = v => v === undefined ? 'undefined'"
           " : (v && typeof v === 'object' && !Array.isArray(v) && v.constructor && v.constructor !== Object) ? v.constructor.name"
           " : JSON.stringify(v);"
           " let r; try { r = call() } catch (e) { return Promise.resolve('absent:' + name(e)) }"
           " return Promise.race([Promise.resolve(r).then(v => 'resolved:' + String(value(v)).slice(0, 80), e => 'rejected:' + name(e)),"
           " new Promise(done => setTimeout(() => done('pending'), 2000))]) })")


def load():
    with open(REGISTRY) as f:
        return json.load(f)


def feature(id):
    return next(f for f in load() if f["id"] == id)


def probes():
    """Every call the probes make, as (row, probe), in the registry's order."""
    return [(f, p) for f in load() for p in f.get("probe", [])]


def mismatch(row, probe, platform, seen, when="load"):
    """What is wrong with what a call gave on platform, on load or on a tap, naming its row, or
    ''. A call that has no answer a page can see, or whose answer on a tap rides a race
    (tap_races), is there only held to its renderer living."""
    status = row["status"][platform]
    if probe.get("sees_nothing") or (when == "tap" and probe.get("tap_races")):
        if (seen == "killed") == (status == "kills"):
            return ""
        return "%s gave %s, but %s is %s on %s in the registry" % (probe["name"], seen, row["id"], status, platform)
    expect = probe["expect"].get(platform + "_" + when, probe["expect"][platform])
    if seen.startswith(expect):
        return ""
    return "%s gave %s on %s, but the registry expects %s (%s is %s on %s)" % (
        probe["name"], seen, when, expect, row["id"], status, platform)

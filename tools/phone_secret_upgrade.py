"""The phone's keys survive an update: a release's APK sets sync up, today's APK goes over it,
and today's build opens what the old one kept. Release's gates run it from the last Stable's
APK to the candidate.

Installs OLD clean and runs drive_android's sync_on_the_phone, which turns sync on (making the
device key, wrapped by the Keystore, and the profile's key) and then restarts the app; just
before that restart NEW goes over OLD with `adb install -r`, so the restart's checks, sync
running again with what was kept, are NEW opening OLD's keys.

The part before the update only makes the state to update from. It runs today's checks
against the old build, whose screens can be older than those checks, so a failed check there
is shown and not counted: the old build is out already, and its screens were judged when it
was released. A step the state needs that never comes still stops the run.
  ADB_SERIAL=emulator-5606 OLD=<released APK> NEW=<candidate APK> OUT=<folder> python3 tools/phone_secret_upgrade.py
"""
import hashlib
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
os.environ["PB_APK"] = os.environ["OLD"]
import drive_android as d  # noqa: E402

NEW = os.environ["NEW"]
NEW_MD5 = hashlib.md5(open(NEW, "rb").read()).hexdigest()
launches = [0]
# How many checks the old build had answered when the new one went over it.
updated_at = [None]
plain_launch = d.launch


def launch(*a, **k):
    launches[0] += 1
    if launches[0] == 2:
        updated_at[0] = len(d.results)
        said = d.adb("install", "-r", "--no-incremental", NEW)
        d.check("the new APK installs over the released one, its data kept", "Success" in said, said.strip()[-200:])
        path = d.adb("shell", "pm", "path", d.PACKAGE).strip().splitlines()[0].split(":", 1)[1]
        on = d.adb("shell", "md5sum", path).split()[0]
        d.check("and it is the new build", on == NEW_MD5, (on, NEW_MD5))
    return plain_launch(*a, **k)


d.launch = launch
os.makedirs(d.OUT, exist_ok=True)
d.adb("root")
d.until(lambda: d.adb("shell", "id", "-u").strip() == "0", 30)
d.adb("reverse", "tcp:%d" % d.PORT, "tcp:%d" % d.PORT)
d.adb("uninstall", d.PACKAGE)
said = d.adb("install", "--no-incremental", os.environ["OLD"])
if "Success" not in said:
    sys.exit("can't install the old APK: " + said)
print("--- part sync_on_the_phone (upgrade)", flush=True)
try:
    d.sync_on_the_phone()
finally:
    d.adb("shell", "am", "force-stop", d.PACKAGE)
    d.adb("reverse", "--remove", "tcp:%d" % d.PORT)
d.check("the restart was the new build's", launches[0] >= 2, launches[0])

before = d.results[:updated_at[0]] if updated_at[0] is not None else d.results
after = d.results[len(before):]
for name, ok, detail in before:
    if not ok:
        print("note  the old build, not counted: %s  %s" % (name, detail))
failed = [r for r in after if not r[1]]
print("\n%d checks after the update, %d failed; %d before it, on the old build" % (len(after), len(failed), len(before)))
sys.exit(1 if failed or updated_at[0] is None else 0)

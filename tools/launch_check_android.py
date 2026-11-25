#!/usr/bin/env python3
"""Installs one APK fresh on one emulator, starts it, and checks it is still up after the
background start and an update check: what tools/release.sh does with the exact signed APK
before it uploads anything (docs/releasing.md, "Publishing").

    tools/launch_check_android.py SERIAL APK [--wait 30]

Fresh: the app is uninstalled first, so its data goes; run it only in a slot the tester gave.
It passes when, WAIT seconds after the start, the app's process is the one that started (no
restart in between) and Android's crash log holds nothing of the app's. The app is
force-stopped at the end. Exit 0 passed, 1 failed, 2 couldn't run.
On 2026-11-26 a published Dev APK aborted at every start; no gate had installed that file.
"""
import argparse
import os
import subprocess
import sys
import time

PACKAGE = "dev.projectb.browser"
ACTIVITY = PACKAGE + "/.BrowserActivity"
ADB = os.path.expanduser("~/Library/Android/sdk/platform-tools/adb")


def adb(serial, *args, timeout=120):
    return subprocess.run([ADB, "-s", serial, *args], capture_output=True, text=True, timeout=timeout)


def pid(serial):
    return adb(serial, "shell", "pidof", PACKAGE).stdout.strip()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("serial")
    p.add_argument("apk")
    p.add_argument("--wait", type=int, default=30)
    a = p.parse_args()
    if not os.path.isfile(a.apk):
        print(f"no APK at {a.apk}")
        return 2
    if adb(a.serial, "get-state").stdout.strip() != "device":
        print(f"{a.serial} isn't an attached device")
        return 2
    adb(a.serial, "uninstall", PACKAGE)
    r = adb(a.serial, "install", a.apk, timeout=300)
    if "Success" not in r.stdout:
        print(f"FAIL the APK didn't install on {a.serial}: {(r.stdout + r.stderr).strip()[-300:]}")
        return 1
    adb(a.serial, "logcat", "-b", "crash", "-c")
    adb(a.serial, "shell", "am", "start", "-W", "-n", ACTIVITY)
    time.sleep(3)
    first = pid(a.serial)
    time.sleep(a.wait)
    last = pid(a.serial)
    crash = [l for l in adb(a.serial, "logcat", "-b", "crash", "-d").stdout.splitlines() if PACKAGE in l or "libproject_b" in l]
    adb(a.serial, "shell", "am", "force-stop", PACKAGE)
    ok = bool(first) and first == last and not crash
    if ok:
        print(f"ok   {os.path.basename(a.apk)} starts on {a.serial} and is up {a.wait} s later, pid {first}")
        return 0
    why = "it didn't start" if not first else f"its process changed ({first} -> {last or 'none'})" if first != last else "Android's crash log has it"
    print(f"FAIL {os.path.basename(a.apk)} on {a.serial}: {why}")
    for line in crash[:20]:
        print("     " + line)
    return 1


if __name__ == "__main__":
    sys.exit(main())

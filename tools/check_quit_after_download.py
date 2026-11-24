#!/usr/bin/env python3
"""Quitting after a download: the app downloads a file, quits with Cmd+Q, and must end cleanly,
with no crash in its log. It guards the quit crash of 2026-11-16 (docs/fragility.md), when the
download manager still pointed at a profile's part after the profile had gone. PB_BIN picks the
app; tools/test.sh quit runs it. Prints ok or FAIL per check; exit 1 if any failed."""
import http.server
import json
import os
import sys
import tempfile
import threading
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import drive_first_app as d  # noqa: E402

NAME = "pb-quit-check-%d.bin" % os.getpid()


class Server(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/dl":
            data = b"x" * 200000
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Disposition", 'attachment; filename="%s"' % NAME)
        else:
            data = b"<!doctype html><title>Download</title><a id=a href=/dl>file</a>"
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Server)
threading.Thread(target=server.serve_forever, daemon=True).start()
os.makedirs(d.OUT, exist_ok=True)
profile = tempfile.mkdtemp(prefix="pb-quit-")
print("--- part quit_after_download", flush=True)
# app.log gathers every start's output (drive_first_app opens it to append), so this run's
# part starts where the log ends now: its last 4000 characters missed a crash early in a long run.
log = os.path.join(d.OUT, "app.log")
start = os.path.getsize(log) if os.path.exists(log) else 0
app = d.App(profile)
d.visit(app, "http://127.0.0.1:%d/" % server.server_port)
d.page_click(app, "#a")
listed, done = os.path.join(profile, "downloads.json"), False
deadline = time.time() + 60
while time.time() < deadline and not done:
    try:
        done = any(x["name"] == NAME and x["state"] == "done" for x in json.load(open(listed))["downloads"])
    except (OSError, ValueError, KeyError):
        pass
    time.sleep(0.2)
d.check("a download finishes", done)
app.send("key", "cmd+q")
try:
    code = app.proc.wait(timeout=30)
except Exception:
    code = "still running"
    app.proc.kill()
d.check("Cmd+Q after it ends the app cleanly, exit 0", code == 0, code)
with open(log, errors="replace") as f:
    f.seek(start)
    ours = f.read()
seen = [w for w in ("dangling", "freed allocation", "Received signal", "SIGSEGV", "SIGTRAP") if w in ours]
d.check("and its log has no crash", not seen, seen)
try:
    os.remove(os.path.expanduser("~/Downloads/" + NAME))
except OSError:
    pass
print("\nall passed" if d.failures == 0 else f"\n{d.failures} FAILED", flush=True)
sys.exit(1 if d.failures else 0)

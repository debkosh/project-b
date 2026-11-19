#!/usr/bin/env python3
"""Builds core/ into a c-archive for the main binary, and writes a depfile."""

import os
import subprocess
import sys


def main():
    go, out, depfile, deployment_target = sys.argv[1:5]
    # sqlite_fts5 always: the store's search needs SQLite's FTS5 (docs/store.md).
    tags = ",".join(["sqlite_fts5"] + sys.argv[5:])
    repo = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
    build_dir = os.getcwd()
    # Inside the source tree the repo is reached through src/project_b.
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)
    # Chromium's version, for --version, which Go prints (core/cmd/lib PBInit).
    version_file = os.path.join(build_dir, "../../chrome/VERSION")
    parts = dict(line.strip().split("=", 1) for line in open(version_file) if "=" in line)
    chromium = ".".join(parts[k] for k in ("MAJOR", "MINOR", "BUILD", "PATCH"))

    env = dict(os.environ)
    env.update({
        "CGO_ENABLED": "1",
        "GOOS": "darwin",
        "GOARCH": "arm64",
        "CGO_CFLAGS": f"-mmacosx-version-min={deployment_target} -O2",
        "CGO_LDFLAGS": f"-mmacosx-version-min={deployment_target}",
        "MACOSX_DEPLOYMENT_TARGET": deployment_target,
        "GOCACHE": os.path.join(build_dir, "gocache"),
        "GOFLAGS": "-mod=mod",
        "PATH": os.path.dirname(go) + ":/usr/bin:/bin",
    })
    subprocess.check_call(
        [go, "build", "-buildmode=c-archive", "-trimpath", "-tags", tags,
         "-ldflags", "-X main.chromiumVersion=" + chromium, "-o",
         os.path.join(build_dir, out), "./cmd/lib"],
        cwd=os.path.join(repo, "core"), env=env)

    deps = [os.path.join(link, "engine/abi/pb_abi.h"),
            os.path.relpath(version_file, build_dir)]
    # core, and browserdata, the module of its own that core's go.mod replaces with ../browserdata.
    for module in ("core", "browserdata"):
        top = os.path.join(repo, module)
        for root, _, files in os.walk(top):
            if "testdata" in os.path.relpath(root, top).split(os.sep):
                continue
            # Each folder too: a Go file added or removed changes its folder's time, and ninja,
            # which knows only the files that were there last time, would keep the old library.
            deps.append(os.path.join(link, module, os.path.relpath(root, top)))
            for name in files:
                if name.endswith((".go", ".mod", ".json")) and not name.endswith("_test.go"):
                    rel = os.path.relpath(os.path.join(root, name), top)
                    deps.append(os.path.join(link, module, rel))
    with open(depfile, "w") as f:
        f.write(out + ": " + " ".join(sorted(deps)) + "\n")


if __name__ == "__main__":
    main()

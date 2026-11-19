#!/usr/bin/env python3
"""Builds core/ into libcore.so for Android, and writes a depfile. cgo uses Chromium's own clang, with
the NDK's sysroot. Chromium's copy of the NDK has no compilers of its own."""

import os
import shutil
import subprocess
import sys


# What a phone serves of the shell's built pages, under shell/dist: our pages and the
# phone's shells, the registry's "phone": true. A phone loads no other shell.
PHONE_SHELL = ("settings", "downloads", "history", "bookmarks", "welcome", "shell/mobile")


def main():
    go, out, depfile, clang, ndk, min_sdk, version = sys.argv[1:8]
    repo = os.path.dirname(os.path.dirname(os.path.dirname(os.path.realpath(__file__))))
    build_dir = os.getcwd()
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)
    sysroot = os.path.join(os.path.abspath(ndk), "toolchains/llvm/prebuilt/linux-x86_64/sysroot")
    cc = " ".join([os.path.abspath(clang), "--target=aarch64-linux-android" + min_sdk,
                   "--sysroot=" + sysroot])

    # The shell's pages go inside Go (core/shellfiles), from shell/dist, which
    # tools/android-release.sh built: the APK carries no folder of them.
    built = os.path.join(repo, "shell", "dist")
    dist = os.path.join(repo, "core", "shellfiles", "dist")
    shutil.rmtree(dist)
    shell = []
    for folder in PHONE_SHELL:
        shutil.copytree(os.path.join(built, folder), os.path.join(dist, folder))
        for root, _, files in os.walk(os.path.join(built, folder)):
            shell += [os.path.relpath(os.path.join(root, name), repo) for name in files]
    open(os.path.join(dist, ".keep"), "w").close()

    env = dict(os.environ)
    env.update({
        "CGO_ENABLED": "1",
        "GOOS": "android",
        "GOARCH": "arm64",
        "CC": cc,
        "GOCACHE": os.path.join(build_dir, "gocache"),
        "GOFLAGS": "-mod=mod",
        "HOME": build_dir,
        "PATH": os.path.dirname(go) + ":/usr/bin:/bin",
    })
    # sqlite_fts5: the store's search needs SQLite's FTS5 (docs/store.md). Phones with 16 KB
    # memory pages refuse libraries aligned for 4 KB. The soname is how the library
    # that links this one finds it at runtime: without it, it would record the build-time path.
    subprocess.check_call(
        [go, "build", "-buildmode=c-shared", "-trimpath", "-tags", "sqlite_fts5",
         "-ldflags", "-s -w -X main.version=" + version +
         " -extldflags '-fuse-ld=lld --unwindlib=none -Wl,-z,max-page-size=16384 -Wl,-soname,libcore.so'",
         "-o", os.path.join(build_dir, out), "./cmd/lib"],
        cwd=os.path.join(repo, "core"), env=env)

    deps = [os.path.join(link, "engine/abi/pb_abi.h")] + [os.path.join(link, f) for f in shell]
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

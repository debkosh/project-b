#!/usr/bin/env python3
"""Builds the Go core (core/) for the target GN builds, and writes a depfile: libcore.a, a
c-archive the Mac's main binary links, or libcore.so for Android, built with Chromium's own
clang and the NDK's sysroot, as Chromium's copy of the NDK has no compilers of its own.

    build_go.py <go> <out> <depfile> <target_os> <target_cpu> <the target's own arguments...>

The Mac's own arguments are its deployment target, then the build flags built in, each a Go
build tag. Android's are clang, the NDK, the minimum SDK and the app's version."""

import os
import shutil
import subprocess
import sys

# GN's names for the target, as Go names them.
GOOS = {"mac": "darwin", "android": "android"}
GOARCH = {"arm64": "arm64", "x64": "amd64"}

# What a phone serves of the shell's built pages, under shell/dist: our pages and the
# phone's shells, the registry's "phone": true. A phone loads no other shell.
PHONE_SHELL = ("settings", "downloads", "history", "bookmarks", "welcome", "shell/mobile")


def main():
    go, out, depfile, target_os, target_cpu = sys.argv[1:6]
    rest = sys.argv[6:]
    repo = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
    build_dir = os.getcwd()
    # Inside the source tree the repo is reached through src/project_b.
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)
    dist = os.path.join(repo, "core", "shellfiles", "dist")

    env = dict(os.environ)
    env.update({
        "CGO_ENABLED": "1",
        "GOOS": GOOS[target_os],
        "GOARCH": GOARCH[target_cpu],
        "GOCACHE": os.path.join(build_dir, "gocache"),
        "GOFLAGS": "-mod=mod",
        "PATH": os.path.dirname(go) + ":/usr/bin:/bin",
    })
    if target_os == "mac":
        deployment_target, built = rest[0], rest[1:]
        # Chromium's version, for --version, which Go prints (core/cmd/lib PBInit).
        version_file = os.path.join(build_dir, "../../chrome/VERSION")
        parts = dict(line.strip().split("=", 1) for line in open(version_file) if "=" in line)
        chromium = ".".join(parts[k] for k in ("MAJOR", "MINOR", "BUILD", "PATCH"))
        # The shell's pages, which ninja built into shell/ first, go inside Go
        # (core/shellfiles): the app carries no folder of them.
        shutil.rmtree(dist)
        shutil.copytree(os.path.join(build_dir, "shell"), dist, ignore=shutil.ignore_patterns(".*"))
        deps = [os.path.relpath(version_file, build_dir), os.path.join("shell", ".stamp")]
        env.update({
            "CGO_CFLAGS": f"-mmacosx-version-min={deployment_target} -O2",
            "CGO_LDFLAGS": f"-mmacosx-version-min={deployment_target}",
            "MACOSX_DEPLOYMENT_TARGET": deployment_target,
        })
        # Each build flag the build has is a Go build tag of the same name.
        tags = ["sqlite_fts5"] + built
        mode = "c-archive"
        ldflags = "-X main.chromiumVersion=" + chromium
    else:
        clang, ndk, min_sdk, version = rest
        # The phone's part of shell/dist, which tools/android-release.sh built: the APK
        # carries no folder of them.
        built = os.path.join(repo, "shell", "dist")
        shutil.rmtree(dist)
        deps = []
        for folder in PHONE_SHELL:
            shutil.copytree(os.path.join(built, folder), os.path.join(dist, folder))
            for root, _, files in os.walk(os.path.join(built, folder)):
                deps += [os.path.join(link, os.path.relpath(os.path.join(root, name), repo))
                         for name in files]
        sysroot = os.path.join(os.path.abspath(ndk), "toolchains/llvm/prebuilt/linux-x86_64/sysroot")
        env.update({
            "CC": " ".join([os.path.abspath(clang), "--target=aarch64-linux-android" + min_sdk,
                            "--sysroot=" + sysroot]),
            "HOME": build_dir,
        })
        tags = ["sqlite_fts5"]
        mode = "c-shared"
        # Phones with 16 KB memory pages refuse libraries aligned for 4 KB. The soname is how
        # the library that links this one finds it at runtime: without it, it would record the
        # build-time path.
        ldflags = ("-s -w -X main.version=" + version +
                   " -extldflags '-fuse-ld=lld --unwindlib=none"
                   " -Wl,-z,max-page-size=16384 -Wl,-soname,libcore.so'")
    open(os.path.join(dist, ".keep"), "w").close()

    # sqlite_fts5 always: the store's search needs SQLite's FTS5 (docs/store.md).
    subprocess.check_call(
        [go, "build", "-buildmode=" + mode, "-trimpath", "-tags", ",".join(tags),
         "-ldflags", ldflags, "-o", os.path.join(build_dir, out), "./cmd/lib"],
        cwd=os.path.join(repo, "core"), env=env)

    deps.append(os.path.join(link, "engine/abi/pb_abi.h"))
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

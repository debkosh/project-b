#!/usr/bin/env python3
"""Builds the Go core (core/) for the target GN builds, and writes a depfile: libcore.a, a
c-archive the Mac's main binary links, or libcore.so, which Linux's browser process loads
(engine/app/main_linux.cc) and Android's library links, built with Chromium's own clang and
the target's sysroot (the NDK's for Android, as Chromium's copy of it has no compilers).

    build_go.py <go> <out> <depfile> <target_os> <target_cpu> <the target's own arguments...>

The Mac's own arguments are its deployment target, then the build flags built in, each a Go
build tag. Linux's are clang and the sysroot, then the build flags. Android's are clang, the
NDK, the minimum SDK and the app's version."""

import glob
import os
import shutil
import subprocess
import sys

# GN's names for the target, as Go names them.
GOOS = {"mac": "darwin", "linux": "linux", "android": "android"}
GOARCH = {"arm64": "arm64", "x64": "amd64"}

# Rust's names for the target, for libpbblock (core/blocking/lib).
RUST_TRIPLE = {
    ("mac", "arm64"): "aarch64-apple-darwin",
    ("mac", "x64"): "x86_64-apple-darwin",
    ("linux", "arm64"): "aarch64-unknown-linux-gnu",
    ("linux", "x64"): "x86_64-unknown-linux-gnu",
    ("android", "arm64"): "aarch64-linux-android",
}

# What Android serves of the shell's built pages, under shell/dist: our pages, the phone's
# web shell for a narrow window, and the desktop shells it lists for a wide one, the
# registry's "android": true (docs/large-screens.md). Android loads no other shell.
ANDROID_PAGES = ("settings", "downloads", "history", "bookmarks", "welcome", "shell/mwshell", "shell/pbshell")


def build_pbblock(repo, build_dir, lib_dir, target_os, target_cpu, linker=None, link_args=()):
    """Builds libpbblock, the blocker's library (docs/blocking.md), with cargo for the target,
    into a target folder under the build's out folder, and copies it into lib_dir, beside
    libcore: libpbblock.dylib on the Mac, with the install name @rpath/libpbblock.dylib, else
    libpbblock.so with that soname. linker and link_args are the C linker for the target and
    what it needs, as Go's cgo has them; none on the Mac, whose own linker cargo uses."""
    crate = os.path.join(repo, "core", "blocking", "lib")
    cargo = shutil.which("cargo") or os.path.expanduser("~/.cargo/bin/cargo")
    if not os.path.exists(cargo):
        sys.exit("engine/build_go.py: libpbblock (core/blocking/lib) needs Rust, and this machine "
                 "has no cargo, on PATH or in ~/.cargo/bin. The toolchain is pinned in "
                 "core/blocking/lib/rust-toolchain.toml; install rustup where the build runs "
                 "(the Linux container pb has none unless it was added).")
    triple = RUST_TRIPLE[(target_os, target_cpu)]
    target_dir = os.path.join(build_dir, "pbblock")
    # cargo and rustup find their toolchains in the real home, not the build's.
    env = dict(os.environ)
    env["PATH"] = os.path.dirname(cargo) + ":" + env.get("PATH", "/usr/bin:/bin")
    if target_os == "mac":
        flags = ["-Wl,-install_name,@rpath/libpbblock.dylib"]
        name = "libpbblock.dylib"
    else:
        flags = ["-Wl,-soname,libpbblock.so"] + list(link_args)
        name = "libpbblock.so"
    var = "CARGO_TARGET_" + triple.upper().replace("-", "_")
    if linker:
        env[var + "_LINKER"] = linker
    env[var + "_RUSTFLAGS"] = " ".join("-Clink-arg=" + f for f in flags)
    try:
        # From the crate's folder, so rustup takes its rust-toolchain.toml.
        subprocess.check_call(
            [cargo, "build", "--release", "--locked", "--target", triple,
             "--target-dir", target_dir], cwd=crate, env=env)
    except (subprocess.CalledProcessError, OSError) as e:
        sys.exit(f"engine/build_go.py: building libpbblock (core/blocking/lib) for {triple} "
                 f"with {cargo} failed ({e}). It needs the Rust toolchain of "
                 "core/blocking/lib/rust-toolchain.toml with the target " + triple +
                 " (rustup target add " + triple + "), and the crates of Cargo.lock, which "
                 "cargo fetches; the Linux container pb has no Rust unless it was added.")
    os.makedirs(lib_dir, exist_ok=True)
    shutil.copy2(os.path.join(target_dir, triple, "release", name), os.path.join(lib_dir, name))
    return lib_dir


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
    if target_os in ("mac", "linux"):
        # Chromium's version, for --version, which Go prints (core/cmd/lib PBInit).
        version_file = os.path.join(build_dir, "../../chrome/VERSION")
        parts = dict(line.strip().split("=", 1) for line in open(version_file) if "=" in line)
        chromium = ".".join(parts[k] for k in ("MAJOR", "MINOR", "BUILD", "PATCH"))
        deps = [os.path.relpath(version_file, build_dir)]
        shutil.rmtree(dist)
        # Each build flag the build has is a Go build tag of the same name.
        tags = ["sqlite_fts5"]
        mode = "c-archive"
        ldflags = "-X main.chromiumVersion=" + chromium
    if target_os == "mac":
        deployment_target, built = rest[0], rest[1:]
        tags += built
        # The shell's pages, which ninja built into shell/ first, go inside Go
        # (core/shellfiles): the app carries no folder of them.
        shutil.copytree(os.path.join(build_dir, "shell"), dist, ignore=shutil.ignore_patterns(".*"))
        deps.append(os.path.join("shell", ".stamp"))
        env.update({
            "CGO_CFLAGS": f"-mmacosx-version-min={deployment_target} -O2",
            "CGO_LDFLAGS": f"-mmacosx-version-min={deployment_target}",
            "MACOSX_DEPLOYMENT_TARGET": deployment_target,
        })
        # Beside libcore.a, where the main binary's link and the bundle take it.
        os.environ["MACOSX_DEPLOYMENT_TARGET"] = deployment_target
        pbblock = build_pbblock(repo, build_dir, os.path.dirname(os.path.join(build_dir, out)),
                                target_os, target_cpu)
        env["CGO_LDFLAGS"] += " -L" + pbblock
    elif target_os == "linux":
        clang, sysroot, built = rest[0], rest[1], rest[2:]
        tags += built
        # The shell's pages, all of shell/dist, which tools/build.sh linux built on the Mac
        # (the container has no Node), as for Android.
        shell_dist = os.path.join(repo, "shell", "dist")
        shutil.copytree(shell_dist, dist, ignore=shutil.ignore_patterns(".*"))
        for root, _, files in os.walk(shell_dist):
            deps += [os.path.join(link, os.path.relpath(os.path.join(root, name), repo))
                     for name in files]
        # The release this build is, for --version, About and the release notes.
        version_path = os.path.join(repo, "VERSION")
        ldflags += " -X main.version=" + open(version_path).read().strip()
        deps.append(os.path.join(link, "VERSION"))
        triple = {"arm64": "aarch64-linux-gnu", "x64": "x86_64-linux-gnu"}[target_cpu]
        env.update({
            "CC": " ".join([os.path.abspath(clang), "--target=" + triple,
                            "--sysroot=" + os.path.abspath(sysroot)]),
            "CGO_CFLAGS": "-O2",
            "HOME": build_dir,
        })
        # Beside libcore.so, which finds it there by its rpath.
        pbblock = build_pbblock(
            repo, build_dir, os.path.dirname(os.path.join(build_dir, out)), target_os, target_cpu,
            linker=os.path.abspath(clang),
            link_args=["--target=" + triple, "--sysroot=" + os.path.abspath(sysroot), "-fuse-ld=lld"])
        env["CGO_LDFLAGS"] = "-L" + pbblock + " -Wl,-rpath,$ORIGIN"
        mode = "c-shared"
        ldflags += " -extldflags '-fuse-ld=lld -Wl,-soname,libcore.so'"
    else:
        clang, ndk, min_sdk, version = rest
        # The phone's part of shell/dist, which tools/build.sh android built: the APK
        # carries no folder of them.
        built = os.path.join(repo, "shell", "dist")
        shutil.rmtree(dist)
        deps = []
        for folder in ANDROID_PAGES:
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
        # Beside libcore.so, in the APK's lib/arm64-v8a. Rust's standard library links the
        # NDK's libunwind, which sits with its clang's runtime, not in the sysroot; 16 KB
        # pages as libcore's.
        runtime = glob.glob(os.path.join(os.path.abspath(ndk),
                                         "toolchains/llvm/prebuilt/linux-x86_64/lib/clang/*/lib/linux/aarch64"))
        pbblock = build_pbblock(
            repo, build_dir, os.path.dirname(os.path.join(build_dir, out)), target_os, target_cpu,
            linker=os.path.abspath(clang),
            link_args=["--target=aarch64-linux-android" + min_sdk, "--sysroot=" + sysroot,
                       "-fuse-ld=lld", "-Wl,-z,max-page-size=16384"] + ["-L" + d for d in runtime])
        env["CGO_LDFLAGS"] = "-L" + pbblock
        tags = ["sqlite_fts5"]
        mode = "c-shared"
        # Phones with 16 KB memory pages refuse libraries aligned for 4 KB. The soname is how
        # the library that links this one finds it at runtime: without it, it would record the
        # build-time path.
        ldflags = ("-s -w -X main.version=" + version +
                   " -extldflags '-fuse-ld=lld --unwindlib=none"
                   " -Wl,-z,max-page-size=16384 -Wl,-soname,libcore.so'")
    open(os.path.join(dist, ".keep"), "w").close()
    # The blocker's library, built above (core/blocking/native).
    tags.append("pbblock")

    # sqlite_fts5 always: the store's search needs SQLite's FTS5 (docs/store.md).
    subprocess.check_call(
        [go, "build", "-buildmode=" + mode, "-trimpath", "-tags", ",".join(tags),
         "-ldflags", ldflags, "-o", os.path.join(build_dir, out), "./cmd/lib"],
        cwd=os.path.join(repo, "core"), env=env)

    deps.append(os.path.join(link, "engine/abi/pb_abi.h"))
    # core, and the modules of their own that core's go.mod replaces: browserdata and oskit.
    for module in ("core", "browserdata", "oskit"):
        top = os.path.join(repo, module)
        for root, _, files in os.walk(top):
            # Test data, and oskit's demo app, which core doesn't link and whose built bundle
            # is no input of the library.
            parts = os.path.relpath(root, top).split(os.sep)
            if {"testdata", "demo"} & set(parts):
                continue
            # libpbblock's own build folder, cargo's.
            if module == "core" and parts[:3] == ["blocking", "lib", "target"]:
                continue
            # Each folder too: a Go file added or removed changes its folder's time, and ninja,
            # which knows only the files that were there last time, would keep the old library.
            deps.append(os.path.join(link, module, os.path.relpath(root, top)))
            for name in files:
                rust = module == "core" and parts[:2] == ["blocking", "lib"] and name.endswith((".rs", ".toml", ".lock"))
                # The blocker's watcher, which Go embeds (core/blocking/page).
                watcher = module == "core" and parts[:2] == ["blocking", "page"] and name.endswith(".js")
                if (rust or watcher or name.endswith((".go", ".mod", ".json", ".c", ".h", ".m"))
                        and not name.endswith("_test.go")):
                    rel = os.path.relpath(os.path.join(root, name), top)
                    deps.append(os.path.join(link, module, rel))
    with open(depfile, "w") as f:
        f.write(out + ": " + " ".join(sorted(deps)) + "\n")


if __name__ == "__main__":
    main()

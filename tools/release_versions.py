#!/usr/bin/env python3
"""The release's version questions, for tools/release.sh and tools/dev-rollback.sh.

    tools/release_versions.py prune-dev NEW NAME...   the channels release's files to delete
        once Dev build NEW is published: every Dev build's files but NEW's and the newest Dev
        build's before it, so a rollback is always one manifest edit (docs/releasing.md).
    tools/release_versions.py newer A B               exit 0 when version A is newer than B.
    tools/release_versions.py set-dev MANIFEST V      writes MANIFEST with the dev channel at V,
        which must be listed in it; nothing else in it changes.
"""
import json
import re
import sys

DEV = re.compile(r"^(\d+)\.(\d+)\.(\d+)-dev\.(\d{12})$")
STABLE = re.compile(r"^(\d+)\.(\d+)\.(\d+)$")
# A file of a version on the channels release: its DMG, APK, the updater's zips and its notes.
FILE = re.compile(r"^(?:Project-B-(.+?)(?:-darwin-arm64\.zip|-android-arm64\.zip|\.dmg|\.apk)|notes-(.+)\.md)$")


def key(version):
    """Orders versions as the updater does: by major, minor, patch, and a Stable after every
    Dev build of the same patch, which is how 0.3.11 updates 0.3.11-dev.<time>."""
    m = DEV.match(version)
    if m:
        return (int(m[1]), int(m[2]), int(m[3]), 0, int(m[4]))
    m = STABLE.match(version)
    if m:
        return (int(m[1]), int(m[2]), int(m[3]), 1, 0)
    raise ValueError(f"not a release version: {version!r}")


def newer(a, b):
    return key(a) > key(b)


def file_version(name):
    m = FILE.match(name)
    return (m[1] or m[2]) if m else None


def prune_dev(new, names):
    """The files to delete: those of Dev builds other than new and the newest one before it.
    Stable releases' notes, the manifest and anything else stay."""
    devs = sorted({v for v in map(file_version, names) if v and DEV.match(v) and v != new}, key=key)
    keep = {new} | set(devs[-1:])
    return [n for n in names if (v := file_version(n)) and DEV.match(v) and v not in keep]


def set_dev(manifest, version):
    if version not in manifest.get("releases", {}):
        raise ValueError(f"{version} isn't in the manifest's releases")
    out = json.loads(json.dumps(manifest))
    out["channels"]["dev"] = version
    return out


def main(argv):
    if len(argv) >= 2 and argv[0] == "prune-dev":
        print("\n".join(prune_dev(argv[1], argv[2:])))
        return 0
    if len(argv) == 3 and argv[0] == "newer":
        return 0 if newer(argv[1], argv[2]) else 1
    if len(argv) == 3 and argv[0] == "set-dev":
        with open(argv[1]) as f:
            m = set_dev(json.load(f), argv[2])
        with open(argv[1], "w") as f:
            json.dump(m, f, indent=1)
        return 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

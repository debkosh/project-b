#!/usr/bin/env python3
"""A Dev build's release notes: what changed for a person since the build before it.

    tools/dev_notes.py <version> <since>    # writes the notes to stdout

<since> is the commit the Dev build before this one was made from (the `commit:` line in its
notes), or a tag, the Stable release, when there is none. A commit a person would notice says
so in a `Notes:` line of its message, in their words: "Notes: Google sign-in works again". Each
such line is a line of the notes; a commit with none is left out, as the app is the same to a
person for it. The commit this build is made from is kept in a comment, for the next Dev build
to start from (docs/release-channels.md).
"""
import re
import subprocess
import sys



def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def main():
    version, since = sys.argv[1], sys.argv[2]
    head = git("rev-parse", "HEAD").strip()
    lines = []
    for c in git("rev-list", "--reverse", f"{since}..HEAD").split():
        for line in git("show", "-s", "--format=%B", c).splitlines():
            m = re.match(r"^Notes:\s*(.+)$", line.strip())
            if m:
                lines.append("- " + m.group(1).strip())
    if not lines:
        lines.append("- Fixes behind the scenes")
    print(f"# {version}\n\n<!-- commit: {head} -->\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()

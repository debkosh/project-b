#!/usr/bin/env python3
"""A Dev build's release notes: the changes since the build before it, one line each.

    tools/dev_notes.py <version> <since>    # writes the notes to stdout

<since> is the commit the Dev build before this one was made from (the `commit:` line in its
notes), or a tag, the Stable release, when there is none. Each commit that changes the app is
a line: its subject's first clause, which says what changed in a person's words as our
subjects begin. Commits that touch only docs, tests, the tools or the roles are left out: the
app is the same for them. The commit this build is made from is kept in a comment, for the
next Dev build to start from (docs/release-channels.md).
"""
import re
import subprocess
import sys

NOT_THE_APP = re.compile(r"^(docs/|tools/|roles/|shell/e2e/|README|CLAUDE)|_test\.go$|\.md$")


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def first_clause(subject):
    # A subject says what changed, then why after a colon or a semicolon; one that names only
    # the place ("A phone's address field: ...") goes on to its next clause.
    parts = re.split(r":\s|;\s", subject)
    clause = parts[0].strip()
    if len(clause.split()) < 5 and len(parts) > 1:
        clause += ": " + parts[1].strip()
    return clause[0].upper() + clause[1:] if clause else clause


def main():
    version, since = sys.argv[1], sys.argv[2]
    head = git("rev-parse", "HEAD").strip()
    lines = []
    for c in git("rev-list", "--reverse", f"{since}..HEAD").split():
        files = [f for f in git("show", "--name-only", "--format=", c).splitlines() if f]
        if files and all(NOT_THE_APP.search(f) for f in files):
            continue
        lines.append("- " + first_clause(git("show", "-s", "--format=%s", c).strip()))
    if not lines:
        lines.append("- Nothing a person sees changed since the build before")
    print(f"# {version}\n\n<!-- commit: {head} -->\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()

#!/bin/bash
# Runs npm ci in this checkout's shell/, refusing where it would empty another checkout's
# packages. npm ci clears node_modules entry by entry before it installs, and through a link
# that clears the linked folder: a worktree whose shell/node_modules linked to the main
# checkout's emptied the main checkout's at 12:32 on 2026-11-26, and the Dev publish's mock
# failed on it. So: never npm ci by hand; use this.
#   tools/shell-npm-ci.sh
# It refuses when shell/node_modules is a link (unlink it first, `rm shell/node_modules`, which
# removes only the link), and when another worktree's shell/node_modules links here (use
# `npm install` here instead, which adds and never clears).
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd -P)
mods="$root/shell/node_modules"
if [ -L "$mods" ]; then
    echo "shell/node_modules is a link to $(readlink "$mods"); npm ci here would empty that folder." >&2
    echo "Unlink it first, with rm shell/node_modules (the link only), then run this again." >&2
    exit 1
fi
linked=
while read -r line; do
    case "$line" in worktree\ *) tree=${line#worktree } ;; *) continue ;; esac
    [ "$tree" = "$root" ] && continue
    link="$tree/shell/node_modules"
    [ -L "$link" ] || continue
    target=$(cd "$tree/shell" && cd "$(readlink node_modules)" 2>/dev/null && pwd -P) || continue
    [ "$target" = "$mods" ] && linked="$linked $tree"
done < <(git -C "$root" worktree list --porcelain)
if [ -n "$linked" ]; then
    echo "these worktrees link their shell/node_modules here, and npm ci would empty theirs too:" >&2
    for t in $linked; do echo "  $t" >&2; done
    echo "Use (cd shell && npm install) here instead." >&2
    exit 1
fi
cd "$root/shell" && exec npm ci

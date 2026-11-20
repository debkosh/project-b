# Project B

## The team

Sessions here work like Sudipta's own team. Each has a role in `roles/`; when asked to take
one, read `roles/<role>.md` and work by it.

- **orchestrator:** runs the sessions and gives him status on what is happening across them.
- **feature sessions:** the devs, each on a feature. He talks to them directly while he is
  discussing their work.
- **tester:** his test person. Looks after the test machines and setups, and the team's testing.
- **ux:** his UI and UX expert. Keeps the product good to look at and to use, and keeps the
  design system and the UX guide.
- **release:** turns finished branches into releases.

Talk to each other directly, as we do now (`ListAgents`, then `SendMessage`; find a peer by the
role in its name). Working on UI, ask ux to review it. Needing the desktop, the VM, an emulator
or the Android build container, ask the tester for it and say when you are done. A finished
branch goes to release.

## Roles grow

A role file is a team doc like any other: it says how the role works today, and the session in
that role keeps it up to date as the way it works changes, from what he says and from what goes
wrong.

## Every session

- Public repo, MIT. No AI attribution. Commit as sudiptadeb.
- A commit a person would notice carries a `Notes:` line in plain words: a Dev build's notes are
  those lines alone.
- Work in a worktree of your own; never remove another session's worktree, branch or unit.
- Write risky commands in guarded form (`rm -f "${L:?}"/*`); a permission dialog stalls you
  while he is away.

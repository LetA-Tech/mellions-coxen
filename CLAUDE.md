# mellions — repository instructions

Auto-loaded for every session in this repo. Repo facts only; the engineer's
identity is `agents/mellions.md`, loaded by the plugin's SessionStart hook.

## What this is

A Claude Code / Codex plugin plus a small CLI that turn a capable coding agent
into a durable second engineer. Architecture: `docs/architecture.md`.

The runtime owns native permissions, tools, MCP, sandboxing, credentials and
model settings; configuration keys that would shadow them are refused.
Mellions also ships narrow PreToolUse safeguards for closing references,
shared-checkout mutation, unsupported citations and credential reads. They are
not a replacement for runtime permissions or operator-enforced boundaries.

## Layout

```
agents/mellions.md      the engineer
skills/                 methods
commands/               slash commands; thin, they invoke the CLI
hooks/                  SessionStart (identity, methods, partnership, program, work, digest)
                        UserPromptSubmit (a governing document that changed since it was
                        handed over, peers, a survey when idle — each said once)
                        PreToolUse (awareness; closing-reference, shared-tree,
                        citation and credential-read safeguards)
cmd/mellions/           the CLI
internal/signal         Signal, Source, Registry — provider-neutral core
internal/survey         collects; never ranks
internal/sources/       one package per provider; core imports none of them
internal/assignment     one piece of work
internal/continuity     the slate after a break
internal/presence       which session is where
internal/awareness      what to tell a session, once
internal/provenance     DISCOVERED/DECLARED/INFERRED/UNKNOWN documents
internal/program        discovery of what the work is for
internal/partner        discovery of who the engineer works with
internal/issuegate      citation resolution (stale-premise scan)
internal/prbody         what a gh pr command hands GitHub as a body, and whether it closes
scripts/shift.sh        one unattended shift; deploy/ its settings and schedule
```

## Invariants held by tests

- The collector never ranks: `signal.Signal` carries no priority, score, weight or rank.
- The core imports no provider.
- The corpus names no deleted verb; the deleted machinery stays deleted (`internal/arch`).

## Build

Go 1.26.1+, standard library only.

```bash
make build     # bin/mellions
make check     # vet + race tests + hook syntax + Skill scripts
make install   # binary on PATH + plugin registered with the runtimes here
```

Work lands on `dev`; `main` is the release branch. The release version lives in
`.claude-plugin/plugin.json` and every other manifest must agree with it.

A checkout of this repository is usually the one every lane on the host is cut
from, not a lane. Commit in a worktree of your own — `mellions assign open -id
<id> -repo mellions-coxen -objective "..." -because "..."` cuts one — and
leave the checkout on its tracking branch. `git checkout -b` here strands it on
a lane branch: the next deploy fails to fast-forward, and `baseFor` falls back
to that branch's HEAD, so the next lane is cut from your commits behind a pin
reading `local HEAD`.

## Conventions

- Comments state current truth. No history, no status claims, no TODO/FIXME.
- Non-trivial logic leaves one runnable check behind.
- A change to the engineer's behaviour is proven in a fresh runtime session,
  with an untreated comparison when the model could produce the behavior on
  its own. Use an isolated runtime home or settings file so the check does not
  change another session's installation.
- `mellions doctor` is the authority for which plugin path and commit a runtime
  will load. Never infer that path from a cache directory or a previous install.
- `commands/`, `skills/`, `agents/` and `hooks/` are shipped payload, not
  repository documentation: a session loads them. Changing a command's
  behaviour — its exit status above all — means changing what they say about
  it, and grepping for the command's name will not find them. They state the
  contract without naming the command, so the search that enumerates them is
  for the promise ("exits non-zero", "returns", "never"), not the binary.

<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Architecture

This document describes how the current product is realized and where its
security and authorization boundaries actually live.

## The governing principle

> Frontier-model judgment, plus only the support genuinely required to make
> that engineer durable and useful.

Claude Code and Codex own the runtime: sessions, native permissions, tools,
hooks, worktrees, subagents, cross-session messaging, sandboxing, scheduling,
plugins and Skills. Mellions adds durable context and engineering methods. It
does not replace the runtime's permission model, but it does ship narrow
deterministic safeguards that can deny specific tool calls when an invariant
can be checked mechanically.

## What exists

| Part | What it gives a session | Where |
|---|---|---|
| Identity | who it is — temperament, standards, what is the owner's | `agents/mellions.md`, loaded by the session-start hook; also the `mellions` agent for headless runs |
| Partnership | how one person wants to be worked with and what they have delegated, in their words | `~/mellions/partners/<name>.md`, drafted from `mellions partner establish` |
| Program | what the work is for: map, relationships, constraints, purpose, correctness, priorities | `~/mellions/programs/<slug>.md`, drafted from `mellions program discover` |
| Survey | what needs attention across the estate, collected and never ranked: open work, changes under review, failing checks, work waiting on the owner, recent change, stale premises (an issue whose citations no longer match the tree; an open assignment whose branch left the remote) — rendered one line per signal with a drill-down, so it can be read at the moment it exists for | `mellions survey`; `internal/signal`, `survey`, `sources/*` |
| Assignment | one piece of work: worktree, branch, why chosen, findings, handoff, the sessions that worked it | `mellions assign`; `internal/assignment` |
| Continuity | after a break: what was recorded next to what the world says now | `mellions continue`; `internal/continuity` |
| Presence | which sessions are on which tree, repository and assignment | `mellions who`, `mellions here`; `internal/presence` |
| Awareness | what a working session is told once, unasked, at its next prompt or its next tool call: a peer on its tree or repository, a survey ready when nothing is in flight | `hooks/awareness.sh` and `hooks/awareness-tool.sh` → `mellions state`; `internal/awareness` |
| Report | what the owner reads instead of the session | `mellions report`; a Markdown file |
| Skills | how it engineers: four bearing methods — reasoning, deep research and falsification, delivered whole at session start; self-learning, loaded at the handoff — and twelve situational methods shown in a catalog at session start and loaded when the situation arrives | `skills/`; `hooks/session-reasoning.sh`, `hooks/session-research.sh`, `hooks/session-falsification.sh`, `hooks/session-skills.sh` |
| Attended ↔ unattended | which state the owner put this host in: `mellions away` and `mellions back` write one marker, and the sessions, the runner and the digest read it. Unattended is entered and left, never inferred from how a session was started | `cmd/mellions/away.go`, `$MELLIONS_HOME/owner` |
| Unattended shift | the same engineer, headless, with the runtime's deny list; a runner starts the next shift when one ends, while the host is away | `scripts/shift.sh`, `scripts/shifts.sh`, `deploy/` |

Two invariants are held by tests in `internal/arch`: the collector never ranks
(`signal.Signal` carries no priority, score or weight), and the core imports no
provider (replacing GitHub is a new `sources/` package and configuration).

## Provenance instead of enforcement

The program and the partnership mix discoverable fact with somebody's intent,
and a reader must be able to tell them apart at a glance. Every section is
DISCOVERED (cited evidence, dated), DECLARED (the person's own words), INFERRED
(the engineer's reading) or UNKNOWN (a named gap and what would settle it). The
engineer maintains the first, third and fourth and proposes changes to the
second. This is a convention the identity holds the engineer to, not a lock:
the owner edits their own sections in their own words, and adoption is a line
saying they read it.

## Judgment, safeguards and security boundaries

Which decisions are the owner's is an organizational fact — merging to a
protected branch, deploying, a production migration, credentials, closing
their issues, changing the engineer itself — and the partnership is where they
say which of those they have delegated. The identity tells the engineer to
read it, act inside it, and bring a decision package for the rest.

Four layers remain distinct:

1. **Frontier-model judgment** investigates, chooses an approach and decides
   what evidence the claim needs.
2. **Mellions engineering methods** describe how to research, remediate,
   falsify, coordinate and hand work over. They are instructions, not an
   authorization system.
3. **Deterministic Mellions safeguards** inspect PreToolUse payloads and can
   deny four narrow classes: publishing a closing keyword on a pull request
   base where GitHub will not resolve it; mutating a configured shared checkout
   instead of a lane; publishing locally resolvable code citations the body
   does not quote; and reading credential-bearing files into a transcript.
   The first three stay silent when their required context cannot be
   established. The credential detector rejects a read shape it cannot prove
   non-printing; `MELLIONS_SECRET_CHECK=off` and `MELLIONS_CITE_CHECK=off` are
   explicit session-environment overrides. These checks do not form a general
   permission layer.
4. **Runtime permissions and operator controls** decide which capabilities are
   available at all. Branch protection, scoped GitHub tokens, filesystem and
   network sandboxing, credential isolation and host policy are the security
   boundaries.

The partnership records organizational authority: which decisions the
operator retains and which ordinary engineering actions are delegated. It
guides model judgment but cannot substitute for controls where an effect must
be impossible.

Unattended, where nobody can answer a question, the headless Claude Code
session runs with `deploy/unattended-settings.json`. That file asks the native
runtime to allow ordinary engineering tools and deny selected high-impact
actions. Operators must review it against their own threat model; their native
runtime policy still applies.

## Delivery, not commands

The measurement that shaped this: sessions use what arrives and do not reach
for what has to be chosen. Identity, the reasoning and research methods, partnership, program, work in
flight and peers arrive at session start. A peer appearing, or a survey being ready when
nothing is in flight, arrives on the next prompt, once. The handoff asks the
learning question at the one moment somebody is there to answer it. The command
surface is small because most of it is reached through those moments rather
than remembered.

## Layout

```
agents/mellions.md          identity
skills/                     methods
commands/                   slash commands (thin)
hooks/                      lifecycle delivery, awareness, deterministic
                            PreToolUse safeguards, hooks.json and shell tests
cmd/mellions/               the CLI
internal/signal, survey     situational awareness core
internal/sources/           github, git, stale premises, assignments, programs
internal/assignment         one piece of work
internal/continuity         the slate
internal/presence           who is where
internal/awareness          what to tell a session, once
internal/provenance         DISCOVERED / DECLARED / INFERRED / UNKNOWN documents
internal/program, partner   discovery and rendering
internal/issuegate          citation resolution behind the stale-premise scan
internal/prbody             what a gh pr command hands GitHub as a body, and whether it closes
internal/cite               locally resolvable code-citation validation
internal/secretread         credential-read command/path classification
internal/sharedtree         shared-checkout mutation classification
internal/checkout, durable  where repositories are; atomic writes and locks
scripts/shift.sh, shifts.sh, deploy/   the unattended shift and its runner
```

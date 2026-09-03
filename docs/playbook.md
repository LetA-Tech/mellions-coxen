<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# The Mellions playbook

This guide covers normal operation after installation. All repository names,
paths, issue numbers and incidents below are synthetic. See
[`install.md`](install.md) for setup and [`cli.md`](cli.md) for exact syntax.

## Three command surfaces

| Surface | Who invokes it | Purpose |
|---|---|---|
| `/mellions:survey`, `/mellions:continue`, and the other slash commands | the operator | short prompts that tell the active session what outcome to produce |
| `mellions survey`, `mellions assign open …` | the agent, through its normal shell; sometimes the operator for inspection | deterministic repository, tracker and record operations |
| lifecycle hooks | the runtime | deliver identity, methods and current context; surface awareness; run narrow safeguards |

Use `/mellions:help` when you do not know which entry point applies. Use
`mellions help` or `mellions <command> -help` when you need CLI syntax.

## Before the first task

Run these once in a terminal:

```bash
mellions doctor
mellions program show
mellions partner list
```

`doctor` reports observed installation state. A missing program or partnership
does not prevent use, but the agent has less durable context for selecting work
and deciding which organizational choices belong to the operator.

Configuration connects repositories globally. A repository is in scope when
its name appears in `repos` and its checkout resolves through `work_root`,
`work_roots` or `checkouts`. Mellions does not add a file to each repository;
the repository's own `AGENTS.md`, `CLAUDE.md` and contribution rules still
govern work there.

## Give the engineer work

Start a fresh Claude Code or trusted Codex session in a repository:

```bash
cd /home/you/workspace/service-a
claude                    # or: codex
```

The current manifest declares nine SessionStart hooks. Together they deliver
the identity, three bearing methods, the Skill catalog, relevant partnership
and program context, work in flight, and owner-facing digest state. The exact
manifest is [`../hooks/hooks.json`](../hooks/hooks.json); `mellions doctor`
reports what the installed runtime will load.

State the outcome as you would to an engineer:

> Take issue #42 in service-a. Re-establish the premise at the current `dev`
> head, claim an isolated lane, implement the complete fix, falsify the
> regression coverage, and return a pull request.

For an unconfirmed defect:

> Requests through the retry path sometimes apply twice. Establish whether the
> duplication is in the client, worker, or persistence boundary before changing
> anything. File and plan the work if the defect is real.

The agent should investigate routine unknowns instead of asking you to operate
it. A real product, security, architecture or authority decision comes back as
a decision package; the partnership decides which ordinary actions are already
delegated.

## The work lifecycle

Work expected to outlive one conversation gets an assignment:

```bash
mellions assign open service-a-42 \
  -repo service-a \
  -issue '#42' \
  -objective 'Make retry processing idempotent across worker restarts' \
  -because 'The defect reproduces at dev HEAD and can duplicate a write'
```

`assign open` creates a branch and worktree from the fetched working-branch
head, writes a durable record outside the target repository, and publishes a
cross-host claim when an issue is supplied. Another live lane on the same issue
is refused unless `-alongside` is stated deliberately.

Use the lane as the unit of work:

```bash
mellions assign list
mellions assign get service-a-42
mellions assign record service-a-42 -kind found '...'
mellions assign handoff service-a-42 -file handoff.md
```

The record carries established findings, hypotheses, next steps, session ids
and the final handoff. A handoff on a claimed pull request is also posted to the
pull request so another host can read it. Closing a lane refuses to discard
uncommitted or unpushed work.

When a repository keeps its own implementation worktree, adopt it:

```bash
mellions assign open service-b-17 \
  -repo service-b \
  -issue '#17' \
  -worktree /home/you/workspace/service-b-17
```

An adopted tree is never removed by Mellions.

## What Mellions supplies automatically

- The engineer identity and bearing methods arrive at session start.
- Relevant program and partnership sections arrive with their provenance:
  `DISCOVERED`, `DECLARED`, `INFERRED` or `UNKNOWN`.
- Open assignments and same-repository peers are surfaced without requiring a
  command to be remembered.
- UserPromptSubmit and PreToolUse awareness says once when relevant state
  changes.
- PreCompact renewal tells the runtime summary which durable responsibility
  must survive context compaction.
- The Skill catalog names situational methods; the agent loads a method when
  its trigger arrives. `mellions skills` prints the current catalog.

Mellions does not replace model reasoning. It supplies methods, durable facts
and lifecycle placement around the model's own technical judgment.

## Deterministic safeguards

The runtime remains the permission authority. Mellions also ships narrow
PreToolUse checks that may deny a tool call:

- a `gh pr create` or `gh pr edit` body that declares an issue closed on a
  branch where GitHub will not resolve the keyword;
- a tree-mutating Git command aimed at a configured shared checkout rather
  than the session's assignment worktree;
- a GitHub body that publishes a locally resolvable `path:line` citation but
  does not quote the cited line;
- a shell/read/grep/notebook operation that would put credential-file contents
  into the transcript.

These are mechanical invariants, not a general approval system. They do not
replace native permission prompts, branch protection, scoped tokens,
sandboxing, filesystem permissions or credential isolation. See
[`architecture.md`](architecture.md) and [`../SECURITY.md`](../SECURITY.md).

## Continue after a break

Start with:

```bash
mellions continue
```

The slate places the earlier record beside current repository and tracker
observations. It deliberately does not merge the two. Re-establish anything
that could have moved, and resume the prior runtime session when its id still
opens; that session holds reasoning the assignment record cannot reproduce.

Use `mellions assign reopen <id>` when handed-off or blocked work returns to
active implementation. Use `mellions assign sweep` to inspect lanes whose pull
requests may have merged or closed; `-apply` closes only those the tracker
establishes are finished.

## Several sessions

Every hooked session registers its repository, branch, tree and assignment.
Run:

```bash
mellions who
mellions who -all
```

This is host-local evidence. Claims on GitHub are what coordinate issues and
pull requests across machines. A peer is a reason to compare lanes and shared
contracts, not a reason to stop unrelated work.

Untracked files are not automatically abandoned. Work in the assignment
worktree you opened; never clean, reset or restore a tree you did not establish
is yours.

## Program and partnership

Programs describe what the work is for; partnerships describe how a person
wants to be worked with and what they delegated. Mellions distinguishes:

- `DECLARED`: the person's own words;
- `DISCOVERED`: evidence the agent can cite;
- `INFERRED`: the agent's interpretation;
- `UNKNOWN`: a named gap and what would settle it.

The agent drafts and checks these documents. A person adopts them and owns
changes to their `DECLARED` sections.

These files can contain private repository names, organizational facts and
working preferences. Keep the configured program and partner directories out
of source repositories unless you intentionally mean to publish them.

## Attended and unattended operation

The operator sets host state explicitly:

```bash
mellions away -until 08:00 -because 'offline'
mellions back
```

While the host is away, `scripts/shifts.sh` may start headless Claude Code
shifts back to back. `scripts/shift.sh` runs one shift. A prompt file makes the
task explicit; without one, the shift starts from the configured survey:

```bash
MELLIONS_PROMPT=/home/you/task.md MELLIONS_BUDGET=1h scripts/shift.sh
scripts/shift.sh
```

Unattended execution uses `deploy/unattended-settings.json` in addition to the
runtime's own policy. Review both before enabling a scheduler. The current
runner drives Claude Code; Codex support is interactive.

Runner state lives beneath `mellions config home`. Its control files are:

| Path | Effect |
|---|---|
| `pause` | prevent a new shift until removed |
| `stop` | end the runner after the active shift |
| `shifts/runner.log` | timestamped runner events |
| `shifts/runner-update.log` | latest self-update output |

See [`../deploy/README.md`](../deploy/README.md) for portable scheduler
templates and every supported environment variable.

## Read the result

```bash
mellions report latest
mellions report digest
mellions assign list
```

The pull request is the change and its proof. The assignment handoff is for the
next engineering session. A report is the operator-facing summary and leads
with any decision or access that genuinely needs a person.

A completion claim should name what was established, the exact checks run,
what could falsify the result, and what remains unknown. A workflow file is not
CI evidence until the provider actually executes it.

## Troubleshooting

- Session lacks the identity or methods: run `mellions doctor`, fix the
  reported registration/trust/load-path problem, then start a fresh session.
- Codex has Skills but no lifecycle context: trust the installed hook manifest
  through Codex, then verify the trust count with `mellions doctor`.
- A repository is absent from the survey: check `gh auth status`, configuration
  and checkout resolution; then run `mellions survey -full -repos service-a`.
- A lane looks idle: read `mellions assign get <id>` and `mellions who` before
  deciding it was abandoned.
- A safeguard denies a call: read the denial. It names the mechanical invariant
  and the non-mutating check or correction that satisfies it.

## Quick reference

| Need | Command |
|---|---|
| installation truth | `mellions doctor` |
| current work | `mellions assign list` |
| one lane's record | `mellions assign get <id>` |
| recover after a break | `mellions continue` |
| current estate signals | `mellions survey` |
| other local sessions | `mellions who -all` |
| program context | `mellions program show` |
| partnership context | `mellions partner show <name>` |
| owner-facing result | `mellions report latest` |
| all CLI syntax | `mellions help` |

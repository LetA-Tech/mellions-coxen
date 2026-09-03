<!-- Mellions Coxen | LetA Tech Ltd. | leta@letatech.ca -->

# The `mellions` CLI

Every verb, what it is for, when it is run, and by whom. `mellions help`
prints the same surface in one screen; `mellions <verb> -h` prints a verb's
flags.

## Where it runs, and who runs it

The binary is on `PATH`, so it runs in three places, and the same command
means the same thing in each:

| where | how it is invoked | what for |
|---|---|---|
| **inside a session**, by the engineer | the session's own shell tool (`Bash`), exactly as it runs `git` or `go test` | claiming and recording work, surveying, checking who else is here, writing the report. This is the normal case: the identity and the slash commands tell the engineer which verbs exist, and it reaches for them itself. |
| **from the plugin's hooks**, automatically | `hooks/*.sh` call `mellions here`, `continue -brief`, `program show -brief`, `partner show -brief`, `state`, `renew` | delivering context at session start, saying a peer arrived, shaping a compaction. You never type these. |
| **from your own terminal** | a second shell, or the `!` prefix inside Claude Code | reading: reports, what is in flight, a lane's record, the survey, `doctor`. Occasionally acting: adopting a program, abandoning a lane. |

Claude Code and Codex execute the Mellions CLI directly through their normal
shell access. A session in a configured checkout may run something like

```bash
mellions assign open service-a-42 -repo service-a -issue "#42" \
  -objective "Make retry processing idempotent across worker restarts" \
  -because "the defect reproduces at the current dev head"
```

and work in the worktree that prints. Read the record afterwards with
`mellions assign get service-a-42`.

The **you / engineer** column below says who normally runs each verb. "both"
means you will sometimes run it to look, and the engineer runs it to act.

## Survey — what needs attention

```
mellions survey [-repos a,b] [-sources x,y] [-since 168h] [-kind k,l] [-full] [-json] [-save] [-limit n] [-timeout 1m30s]
mellions sources
```

Collects, across every repository in `repos`: open work items, change sets
under review, failing builds, work blocked on the owner (`owner_labels`),
recent commits, program objectives, assignments in flight, follow-ups, and
**stale premises** — open issues whose cited paths and symbols no longer
match the tree. Grouped for reading; never ranked, scored or recommended.

- **when:** a session with nothing in flight (the session-start hook says so
  and refreshes a saved survey in the background); the start of every
  unattended shift; you, when you want to see what it sees.
- **who:** both.
- **example:** `mellions survey` for the shape, then the slice you are choosing
  from: `mellions survey -full -repos service-a -kind work_item`.
  `-kind stale_premise` alone is the list of issues whose account of the code
  has drifted. `-save` writes `survey-latest.md` under the report root, where
  the session-start hook looks. `mellions sources` lists the configured
  sources.
- **read it right:** `INCOMPLETE` means a source did not answer — unknown,
  not empty. "… N more collected" means the list was printed short and names
  the command that prints the rest.

## Assign — one piece of work

```
mellions assign open <id> -repo R [-issue "#N"] -objective "..." -because "..."
                    [-not-chosen "..."] [-branch b] [-base ref] [-worktree dir]
                    [-budget 4h] [-alongside] [-unpublished]
mellions assign open <id>                 # an id that already has a record: claim it
mellions assign list [-all]
mellions assign get <id>
mellions assign record <id> <text> [-kind found|hypothesis|next|note]
mellions assign claim <id> -pr <n>
mellions assign handoff <id> [-file f|-]
mellions assign reopen <id>
mellions assign close <id>
mellions assign abandon <id> -discarding "..."
mellions assign sweep [-repo R] [-apply]
```

Every verb takes the id as the first argument or as `-id`, and means the same
by both.

**`open`** claims a lane: a worktree cut from the *fetched* remote head of the
repository's working branch (the pin is recorded; a fallback to a local head
says so), a branch `mellions/<id>`, and a record under
`<report_root>/assignments/<id>/` holding the objective, the reason, and what
was deliberately not chosen. With `-issue`, it first reads the issue's claims:
a lane on this machine that already holds the issue is refused by name; a lane
on another machine is refused too, because opening publishes a
`mellions:claimed` label and a comment naming the lane and its host, and a
claim not restated within 24 hours is swept rather than obeyed. `-alongside`
takes a held issue deliberately (reconciling two lanes); `-unpublished`
accepts a lane no other machine can see when the tracker is unreachable.
`-worktree` adopts a tree a repository's own process created instead of
cutting one; an adopted tree is never removed.

- **who:** the engineer. You run it only to hand-cut a lane for yourself.
- **example (engineer, in a session):**
  ```bash
  mellions assign open payments-api-42 -repo payments-api -issue "#42" \
    -objective "Keep migration failures out of the applied-version ledger" \
    -because "bounded, reproduced, and on a persistent-state path" \
    -not-chosen "frontend-app #18 — larger than the available window"
  ```
  prints the worktree (`/home/you/mellions/assignments/payments-api-42/tree`) and the
  branch, and the session works there.

**`list`** — what is in flight: `active` (being worked, or interrupted),
`handed_off` (a handoff on record, usually a pull request open), `blocked`
(names what it waits on). `-all` includes `closed` and `abandoned`.
**`get`** — one lane's whole record: objective, findings, hypotheses, next
steps, handoff, the sessions that worked it and how to resume the last one.

- **who:** both. These two are your morning.

**`record`** — a finding (`-kind found`), a hypothesis, a next step or a note,
appended to the record with a timestamp. The engineer writes these as it
establishes things; they are what the next session reads.

**`claim`** — this lane holds that pull request. It publishes the same
`mellions:claimed` label and machine-readable comment `open -issue` publishes
on an issue, so `mellions survey` prints `CLAIMED` against the change set on
every host, and a peer choosing work reads the claim instead of the draft bit.
Use it before dispatching a review of your own draft: "draft" means unfinished,
unreviewed and blocked-on-a-review-in-flight alike, and a peer that reads the
second where the third is true merges work the review is about to reject.
The claim is released by `close`, and a claim not restated within 24 hours is
swept rather than obeyed — the same terms as an issue's. References follow one
form: `PR #12` is a pull request, `#12` is an issue.

**`handoff`** — where the work stands, in four parts: what stands (with links
to the commits, branch, pull request), what is established, what is
unresolved or blocked and on whom, what to pick up next and why. `-file -`
reads it from stdin. The lane becomes `handed_off`.

Where the lane has a pull request — the one `claim` recorded, or the single
open one on its branch — the handoff is also posted there as a comment, opening
with the lane, the host and the state, and the claim is restated on it. The
record lives on one machine's disk; the peer deciding whether the draft is
ready is on the other one, and the pull request is the only surface both read.
A tracker that cannot take the comment does not cost the session the handoff:
it is on the record first, and the failure is printed.

**`reopen`** — take handed-off or blocked work back up (a review came back);
the worktree is re-cut from the branch if it was cleaned up.
**`close`** — the normal end: removes the worktree, keeps the branch, and
refuses while anything exists only in the worktree.
**`abandon`** — deletes the branch too, on the record, naming the tip so it can
be recovered for a while; `-discarding` says what is thrown away.

- **who:** the engineer, for all four. You run `abandon` when you have decided
  work should not continue; a lane whose pull request merged after the session
  that would have closed it was gone is what `sweep` is for.

**`sweep`** — one line per open lane, read against the tracker. A handed-off
lane whose pull request is merged or closed is closable, and `-apply` closes
it the ordinary way — worktree removed, branch and record kept — with the
record saying the sweep did it and on what evidence. An open pull request, no
pull request, a tracker that could not answer and a worktree still holding
uncommitted work each keep the lane, saying why; active work is never closed,
and is told to hand off only when no live session holds it. `-repo` narrows.
Without `-apply` it is a dry run.

- **who:** both. The engineer runs it when `continue` or the survey carries
  finished lanes as work in flight; you run it when the listing has grown.
- **example:** `mellions assign sweep` → `payments-api-42  closable  pull request
  #51 merged`, `service-a-17  kept  pull request #52 is open`, `service-b-9
  active  \`mellions assign handoff service-b-9\` first; the sweep
  never closes active work` — then `-apply` closes the first.

## Continue — after a break

```
mellions continue [-offline] [-brief]
```

The slate for a session that did not attend the one before it: what an
earlier session recorded, next to what the repository and the tracker say
now — two voices, never merged. It reaches no conclusion; deciding what
survived is the work. `-brief` is one line per open lane, which the
session-start hook prints; a lane whose worktree is gone says so rather than
naming a dead path. `-offline` asks no tracker.

- **who:** both; the hook runs `-brief` at every session start.
- **example:** you closed the terminal mid-task. Start a new session; the
  in-flight brief names the lane and the session id it was last worked in.
  Say "pick up `service-a-42`"; the engineer runs `mellions continue`, reads
  the record against the world, resumes from what survived — and if
  `claude --resume <id>` still opens, resumes that session first, because
  it holds the reasoning the record only summarises.

## Who — other sessions on this repository

```
mellions who [-all] [-C dir]
mellions here [-C dir] [-pid n]
```

`who` lists the live sessions that registered on this repository, with age
and last activity, and every working tree git knows about — which tree is
which lane, and whether the one you are in is somebody's. `-all` is every live
session on the machine. It answers for this machine: another host's sessions
show only through what they leave on the tracker (a claim, a pull request).
`here` registers the calling session; the hooks run it, you do not.

- **who:** both. The engineer runs `who` before touching a tree it did not
  cut; you run it to see who is where.
- **example:**
  ```
  # Who is working here — 19:09 UTC
  - claude session 2fc77c63 on service-b (dev), in /home/you/workspace/service-b — another tree, started 2h ago, last active 21m ago
  - codex session d9ec85bb on service-b (dev), in /home/you/mellions/assignments/service-b-17/tree — assignment service-b-17, last active 8m ago
  ...
  None of this reserves a repository. Every lane is a worktree of its own …
  ```

## Skills — methods this installation carries

```text
mellions skills [<what you are doing>]
```

Without an argument, lists each installed Skill and its trigger. With a task
description, narrows the list; a single match prints the description and the
runtime call that loads it. This command does not load a Skill or choose a
method for the agent.

- **who:** both. The session-start hook uses the catalog for discovery; an
  operator can use it to inspect the installed method surface.
- **example:** `mellions skills "audit this subsystem for defects"`.

## Deterministic checks and PreToolUse guards

These commands are the CLI side of the four narrow safeguards in
[`architecture.md`](architecture.md). The `*-check` forms read a runtime
PreToolUse payload when `MELLIONS_HOOK` says one is present; hand-running one
without a payload prints usage rather than claiming that it checked anything.

```text
mellions pr-body-check
mellions shared-tree-check
mellions cite check [-file f|-] [-dir checkout] [-commit sha]
mellions cite-check
mellions secret check [-path file] <command...>
mellions secret-check
```

- `pr-body-check` may deny `gh pr create` or `gh pr edit` when a closing
  keyword is supplied for a pull request base on which GitHub will not resolve
  it. Unknown default-branch state is silent.
- `shared-tree-check` may deny a tree-mutating Git command aimed at a configured
  shared checkout instead of an assignment lane. Unresolved paths are silent.
- `cite check` is the hand-run citation validator. It reports locally
  resolvable `path:line` claims whose lines do not exist or are not quoted in
  the body and exits non-zero on findings. `cite-check` applies it to GitHub
  publication commands; `MELLIONS_CITE_CHECK=off` disables the hook when set
  in the session environment.
- `secret check` is the hand-run credential-read classifier. `secret-check`
  applies it to Bash, Read, Grep and NotebookRead payloads and may deny content
  that would enter the transcript. `MELLIONS_SECRET_CHECK=off` disables the
  hook when set in the session environment.

These checks do not grant permission. Native runtime policy and operator
controls still decide whether a tool or external effect is available.

## Report — what you read instead of the session

```
mellions report write [<id>|-id id] -did "..." [-established "..."] [-blocked "..."]
                      [-next "..."] [-needs-owner "..."] [-file f|-]
mellions report latest [-n 3]
mellions report digest [-brief]
```

`write` files a report under `<report_root>/reports/` that leads with
**Needs you** (a decision packaged for one word), then **What changed about
what you believe**, then what was done, what blocked, and what is next and
why. `-file` reads the body from a document; the flags are what say whether
the report needs you, so a report written from a file alone carries no
"needs you" line.

`digest` is what needs you since it was last said: finished shifts (stamp,
host, what the reply led with), reports whose **Needs you** or **Blocked** is
non-empty, and how many handed-off lanes name you in their handoff — a word
match, so a count to read `assign list` by rather than a fact about any lane.
`-brief` is the session-start hook's form: said once per eight hours across
every session on the host, bounded, and marked as said in
`$MELLIONS_HOME/digest-seen`; an unattended shift (`MELLIONS_DEADLINE` set)
is not the reader and is told nothing. Without it, everything since the marker.

- **who:** the engineer writes; you read. `report latest` is the first
  command of your morning, and the digest meets you when you open a session.
- **example:** `mellions report latest -n 1` opens with a release decision,
  the established evidence, the alternatives, the recommendation and the
  exact action that would settle it.

## Program and partner — what the work is for, who it is with

```
mellions program discover [-window-days 90] [-out file]
mellions program show [-brief] [<slug>] | list | check [<slug>] [-stale-days 45]
mellions program adopt <slug> -by "<name>"

mellions partner establish [<name-or-email>] [-window-days 365] [-out file]
mellions partner show [-brief] <name> | list | check <name> [-stale-days 180]
mellions partner adopt <name> -by "<name>"
```

`discover` and `establish` collect evidence — repositories, activity,
cross-references, who commits where and when — and print it; the session
drafts the document from it, with every section's provenance marked. `check`
reports a DISCOVERED section with no citation, a DECLARED section the engineer
wrote, evidence older than the stale window. `adopt` records that a named
person read it. `show -brief` is what the session-start hook delivers; `show`
is the whole file.

- **who:** the engineer drafts and checks; **adopt is yours**, always. The
  engineer proposes changes to your DECLARED sections and never makes them.
- **example:** you changed your mind about what is delegated. Edit
  `partners/<you>.md`, the DECLARED delegation section, in your own words. The
  next prompt in every live session is told the partnership changed; the next
  session gets the new text at start.

## Away and back — the state you enter and leave

```
mellions away [-until 8h|22:30|<RFC3339 timestamp>] [-because "..."]
mellions back
```

Unattended is a state you enter and leave, not how a session was started.
`away` records in `$MELLIONS_HOME/owner` that nobody is reachable on this host;
`back` records that you are here again. Both are one act on the way past, and
three readers act on the one file: every session is told at its next prompt or
tool call — once each way, through the awareness hook that already carries the
clock and the peers — the runner starts shifts back to back while you are away
and none while you are here, and `back` prints the digest from the moment you
left.

`-until` lapses the away state on its own: a duration, a UTC clock time meaning
its next occurrence, or a full stamp. A time already past is refused, because a
window that lapses as it is written reads as attended to every reader of it.

A marker that is not there is **unknown**, never attended — a host whose owner
has never said either is not one they are sitting at — so a session is told
nothing and the runner runs as it did before. `MELLIONS_NIGHT_WINDOW` is the
standing arrangement for a host that should work every night without being told
each evening.

- **when:** leaving the keyboard for the night, a meeting or a week; coming
  back. Not per session, and not per shift: the host is what has the state.
- **who:** you. A session reads it and never writes it — a session that could
  record you as away could send itself unattended.
- **example:** `mellions away -until 08:00 -because "asleep"`, then
  `mellions back` in the morning, which prints what the night produced.

## Renew and state — the hooks' verbs

```
mellions renew [-trigger auto|manual]
mellions state [-session id] [-runtime r] [-tool] [-repeat] [-C dir]
```

`renew` is run by the PreCompact hook: its stdout becomes the instructions
the runtime's compaction follows, anchored on the lane the working directory
is in — what the summary must keep (the lane, established facts and their
citations, decisions, what is unresolved, what needs the owner, what was
borrowed and not yet given back, what was tried and failed, the next step)
and what it may let go. A session cannot start a compaction on its own; this
shapes the one the runtime starts.

`state` is run by the UserPromptSubmit and PreToolUse hooks: what to tell a
session once — a peer arrived on this repository, this tree is shared, the
partnership or program it was handed has changed since, it is idle while
useful work exists. `-tool` emits the runtime's PreToolUse JSON so the note
reaches a session mid-turn.

- **who:** the hooks. Running `mellions renew` by hand prints what a
  compaction of the current directory's lane would be told to keep — useful
  to read once.

## Doctor, config, install, version

```
mellions doctor
mellions config init | show | path | home | reports
mellions install [-from <path|owner/repo>] [-runtime claude|codex] [-dry-run]
mellions version
```

`config home` and `config reports` print one directory each and nothing else:
where this host's shifts land, and where a report is written. `scripts/shift.sh`
and `scripts/shifts.sh` ask rather than defaulting to a path of their own, so a
host whose `report_root` is not under `~/mellions` has one home instead of two.

`doctor` is described in `docs/install.md`; run it whenever a session seems
not to be Mellions, and to answer **which Mellions this host is running**: its
`load path` line says where the runtime actually reads the plugin from — the
checkout itself for a `directory` marketplace, the cache copy for one the
runtime fetched — and `load path commit` says what that path stands at when it
is a checkout, including whether it is dirty or behind its upstream. `codex
hooks: N of M trusted` says whether a Codex session on this host is Mellions at
all. `config` is the installation's one file. `install` registers the plugin
with the runtimes from a checkout or the published source and then establishes,
from the runtimes' own files, where the next process will load it from and that
it will. `version` prints the release and commit.

- **who:** you. A session runs `doctor` when it suspects its own installation.

## Slash commands

The plugin's commands are thin: each says when and how to use a verb above,
in the engineer's terms, and the session runs the verb.

| command | runs |
|---|---|
| `/mellions:help` | what to type and what Mellions runs on its own: the three kinds of command, the installed slash commands, then `mellions help` whole |
| `/mellions:mellions` | restates what the engineer carries and the verbs that support it |
| `/mellions:survey` | `mellions survey`, then choose and claim |
| `/mellions:continue` | `mellions continue`, then re-establish |
| `/mellions:handoff` | `mellions assign handoff`, four parts, then the self-learning question |
| `/mellions:report` | `mellions report write` |
| `/mellions:doctor` | `mellions doctor` |
| `/mellions:program-discovery` | `mellions program discover`, then draft |
| `/mellions:partner-establish` | `mellions partner establish`, then draft |

## The shift script

Not a CLI verb, but the other way work starts:

```
scripts/shift.sh
```

| variable | default | meaning |
|---|---|---|
| `MELLIONS_PROMPT` | *(none — survey-driven)* | a file whose contents are the task |
| `MELLIONS_BUDGET` | `45m` | how long the session is told it has |
| `MELLIONS_TIMEOUT` | `3600` | seconds before the process is killed |
| `MELLIONS_RENEW_BYTES` | measured | transcript bytes since the last compaction past which `mellions state` states the measured facts (six tenths of the size this host has compacted sessions of the same model at; 3 MB where unmeasured); the session decides renewal at a boundary; `0` turns it off |
| `MELLIONS_DEADLINE` | set by the script | the deadline as a Unix time; the hooks read it and print `clock: … · N min left` on every tool call, and the digest hook stays silent under it |
| `MELLIONS_MODEL` | `opus` | the runtime model |
| `MELLIONS_HOME` | `mellions config home` | where `shifts/<stamp>.{log,survey.md,prompt.md,reply.md,stream.jsonl}` land, and the runner's lock and log with them. Unset, the script asks the binary, which answers with this installation's `report_root` — so a shift lands where `report digest` and `doctor` already look. Set, it overrides for the script and for the session it starts, and the report the session writes still goes to `<report_root>/reports/`. `mellions config home` prints the answer on this host; `mellions config show` prints it beside the rest |
| `MELLIONS_WORKDIR` | `$MELLIONS_HOME` | the session's working directory |
| `MELLIONS_SETTINGS` | `deploy/unattended-settings.json` | the runtime settings: the tools an engineer needs allowed, a short list of never-ordinary actions denied |
| `MELLIONS_SURVEY_ARGS` | *(none)* | extra words for `mellions survey -save`, such as `-repos mellions-coxen`; the runner sets it for a method shift |
| `MELLIONS_BIN`, `CLAUDE_BIN`, `MELLIONS_PYTHON` | from `PATH` | the binaries |

It runs `claude -p` with those settings and `--permission-mode acceptEdits`,
follows the stream (`scripts/shift-follow.py`), captures the reply, and the
session files its own report. Claude Code only. A survey-driven prompt ends
with the tail of the previous shift's reply, under `=== THE PREVIOUS SHIFT
SAID ===`, so a shift knows what the last one chose, passed over and left open.
A `stop` — SIGTERM — is passed on to the session.

## The runner

```
scripts/shifts.sh
```

Runs shifts back to back on one host: one runner per `MELLIONS_HOME`, held by
`shifts/runner.lock`; a self-update from the checkout before each shift; a
cooldown that doubles while shifts fail; a wait for the named window, and no
doubling, where the runtime refused the shift with the account's usage limit; a
cap per UTC day counted from the shift files, which a refused shift does not
spend; every Nth shift scoped to `mellions-coxen`. `pause` and `stop`
files under `MELLIONS_HOME` hold and end it; SIGTERM ends it after the current
shift and reaches the session. Every event is one line in
`shifts/runner.log`. `deploy/README.md` installs it as a LaunchAgent on macOS
or from the user's crontab on Linux.

| variable | default | meaning |
|---|---|---|
| `MELLIONS_CHECKOUT` | the checkout the script is in | what `git pull --ff-only`, `make build` and `make check` run against, and whose `bin/mellions` is installed; it has to be the directory the runtime's marketplace record names — the tree sessions load the plugin from — or the runner refuses to start |
| `MELLIONS_AUTOUPDATE` | `1` | `0` skips the update before each shift |
| `MELLIONS_COOLDOWN` | `5m` | between shifts; doubles up to `1h` while shifts fail, resets on a good one |
| `MELLIONS_SHIFTS_PER_DAY` | `12` | the cap per UTC day — a cost guard, not the cadence |
| `MELLIONS_NIGHT_WINDOW` | unset | `HH:MM-HH:MM` UTC, wrapping past midnight, where shifts run whether or not `mellions away` said the owner had stepped out |
| `MELLIONS_METHOD_EVERY` | `4` | every Nth shift of the day surveys `mellions-coxen` only; `0` never |
| `MELLIONS_TICK` | `60` | seconds between looks at `pause` and `stop` while waiting |
| `MELLIONS_SHIFT` | `scripts/shift.sh` beside it | the shift script |

Everything `shift.sh` reads passes through unchanged.

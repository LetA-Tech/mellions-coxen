---
name: mellions-environment
description: What this host can actually do, and how to establish it rather than assume it — toolchains, git and GitHub access, container runtimes, the disposable sandbox, and where the repositories are. Use when deciding whether a task is possible here, choosing what to ask a session to run, telling one where a repository lives, or answering what tooling is available. Triggers — "can we run", "is X installed", "where is the repo", "what version of", "run the tests", "use the sandbox".
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# This host

**Establish it; do not recall it.** A version, a path or a login written down
somewhere was true on the machine it was written on. This method is installed on
every host and none of them are the same one.

```
mellions doctor            what this installation can do: binary, config,
checkouts, tracker, runtimes, hooks
mellions config show       which repositories, where their checkouts are,
where the records live
```

Read the output with one distinction first: **unknown is not absent.** A probe
that could not run has established nothing, and routing around a capability
you have is worse than never having had it; settle what reads unknown.

## Repositories

`mellions config show` names every repository in scope and where it is checked
out. The base branch, the verification commands and any protected paths are the
repository's own to state — its `CLAUDE.md` or equivalent. Where it says
nothing, `git symbolic-ref refs/remotes/origin/HEAD` gives the remote's default
branch, and you say that was inferred rather than declared.

A clone may carry a fetch refspec restricted to one branch, in which case
`git branch -a` shows nothing else and it looks deleted. `git ls-remote --heads
origin` is what the remote actually has.

A repository absent from the filesystem can usually be cloned; absence is not
evidence that it does not exist.

## Whether the sandbox needs a word first is the owner's to say

Investigation, reproduction, running a test, falsifying a claim and validating a
fix inside a container that is thrown away afterwards have no external effect
and no owner decision in them. Where the owner has said nothing about it, use
the sandbox the way you would use a temporary directory: without asking, and
without announcing it. An engineer that has a sandbox and believes it must ask
behaves exactly like one that has none.

Where the owner has said something — the partnership, or the runtime's own
instructions for this machine — that wins, and this Skill does not argue with
it. Read the partnership for what this installation's owner granted: where
starting a container for engineering work is delegated, waiting for a word is
not caution, it is the work not getting done. What no grant ever covers is the
exit.

Three things still stop and ask, and none of them is the sandbox itself:

- the partnership or the machine's own instructions say so;
- the runtime does not provide one, in which case say so rather than running the
  experiment in a real checkout;
- what you are about to do reaches outside the sandbox — a real credential, a
  production endpoint, a shared database, an image push, anything the owner has
  kept for themselves. The container does not make a reserved action reversible.

A sandbox is useful both when an experiment might do damage **and** when a
technical uncertainty can be settled by a clean, disposable reproduction,
mutation, dependency probe, or competing-hypothesis test. It is evidence
machinery, not merely a security wrapper. `mellions-sandbox` is the method.

Whether that machinery is reachable is a property of this moment, not of the
repository, and it decides which work is *finishable* in a window rather than
only how it is done. `colima status` is one command and belongs with the
choice of work, not with the implementation: a shift that picks a defect whose
proof needs Postgres, writes the fix, and only then finds the runtime stopped
has already spent the window. A stopped runtime costs a start and an image
pull — price that into the choice; it is never a reason to downgrade the proof.

Whatever is started belongs to the turn that started it. Tear it down in the
same turn and say what remains: a stopped container still holds disk, a running
one still holds memory.

"Containers you did not start are not yours" needs something to read ownership
off, and it is not the clock: two sessions in one night each reasoned from an
unexplained container's creation time, each concluded it was the other's, and
neither inspected it.

Inspect it. Three facts, in order, and none of them alone is an answer:

```bash
docker inspect <name> --format '{{json .Config.Labels}}
{{.HostConfig.Runtime}} {{.HostConfig.AutoRemove}}'
grep -rn '<name>' ~/workspace --include='*.sh'   # what creates it
mellions who                                     # and whether that lane is live
```

`leta.sandbox=1` does **not** establish that a Mellions turn started it: any
repository harness that must provision on a host refusing an unlabelled
`docker run -d` applies it too, so the label marks "findable for teardown",
not "mine". A container the wrapper started also carries `--rm` and
`--runtime=runsc`; one labelled but under `runc` with `AutoRemove=false` came
from somewhere else, and the script that names it says where.

A labelled container is a question, not a verdict: a repository's test
database belongs to whoever is running that suite, and tearing it down mid-run
is worse than leaving it up overnight.

A loop waiting on work you delegated is a background job your turn spawned.
`kill` followed by `wait` on a busy loop hangs the shell that spawned it, so a
run that already produced its evidence burns the rest of the budget and is
killed on timeout, leaving the cleanup unproven. A pattern matches the shell
issuing it: `pgrep -f X` never
goes false and `pkill -f X` takes itself, because X is in their own argv. Act
on the pid you started.
Signal them, then establish they are gone: `kill -0` per pid, and a sweep for
survivors. A timed-out teardown is not a completed one.

## Limits worth knowing before promising something

Core count bounds parallelism, and several builds each assuming the whole
machine will thrash. Cap per-session concurrency rather than discovering it
under load.

On a metered model subscription, the quota binds before the hardware does.
Several concurrent sessions on a large model exhaust the pool long before they
exhaust the RAM; treat concurrency as bounded by whichever runs out first.

`/tmp` here is a small tmpfs under a per-user quota, where Go's build and
`-race` scratch and any `mktemp`ing suite land by default. Full, writes fail
`disk quota exceeded` and your shell returns a silent exit 1, reading as a
broken build or harness. Put `GOTMPDIR`/`TMPDIR` under `$HOME` for a gate
wider than a package; keep tree copies off `/tmp`.

## Establishing what a session actually receives

A session cannot report what reached it, only what it believes. The request
on the wire settles it: `scripts/capture-wire.sh`, in
the mellions-coxen checkout, runs one headless session against a loopback
server that records the request body and answers 400 — nothing forwarded, no
credential written. It prints model, system-prompt size and tool count, and
leaves `req-NNN.json` beside the extracted `req-NNN.system.txt`.

One capture usually answers it, and the tool array is often the decisive half:
a tool absent from it was never offered, which a session reports as a
prohibition. Capture again with one input changed — model, settings file,
plugin set — when the question is why two hosts differ, not whether a line
exists.

The 400 stops the session at its first turn, so hook output and anything
injected later appear in no capture, and a subagent's prompt is not a
top-level `claude -p`'s. Absence in a capture is absence from turn one; say
that when reporting one.

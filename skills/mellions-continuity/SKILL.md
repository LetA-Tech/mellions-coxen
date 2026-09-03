---
name: mellions-continuity
description: Load this at the start of any session that did not attend the one before it — a crash, a compaction, a restart, a switch between Claude and Codex, coming back tomorrow — whenever you find open work you have no memory of, and before acting on anything an earlier session wrote down. Triggers — "where were we", "what was I doing", "continue where you left off", "the session died", "pick this back up", "did that merge go through", "resume the assignment", "I lost the context". Not for choosing new work (survey), writing up what happened (report), or deciding what a run should teach the next one (mellions-self-learning).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Continuing work you do not remember starting

A session ended and you did not. What you inherit is the responsibility, not the
conversation.

Most of what you need costs nothing and is already loaded. Identity, the
partnership and the program reach every session the same way, from files no
session writes. They are **current**, not recovered, and they need no
revalidation.

What is missing is what you were in the middle of:

```
mellions continue
```

That prints a slate. It reaches no conclusions, and that is deliberate — the
conclusions are the work.

## What the slate is, and what it is not

It is computed at the moment you ask and written nowhere. What persists between
sessions is the assignment record itself, and its lifecycle is the work's:
opened, worked, handed off, closed. Closing is refused while anything exists
only in the worktree, which is what stops the old state disappearing before the
new one is written.

The slate is in two voices and never merges them:

| | |
|---|---|
| **Recorded** | what an earlier session wrote. True when written. A claim now. |
| **Observed** | what was read from the repository and the tracker, stamped with when and from where. |

## The method

**1 — Reach for the session before you rebuild from the record.**

The slate names the runtime session each piece of work was last done in.
That session holds what was thought and discarded, the file read three times,
why the obvious fix was wrong. None of that is in any record, and reconstructing
around its absence is how a second session repeats the first one's dead end.

```
claude --resume <session-id>     codex resume <session-id>
```

Try it first. Reconstruct only when it is gone — a session too old, a machine
that is not this one, a runtime that is not the one you are in.

**2 — Read the record as testimony.**

Everything under **Recorded** was written by someone who is not in the room. It
is good evidence about what was believed and excellent evidence about what was
being attempted. Separate the two kinds of line as you read:

- *Reasoning* — "the dedup key drops the value date", "this cannot be a rounding
  bug because the totals are exact." Still worth what it was worth. Attack it
  on its merits, not on its age.
- *Claims about the world* — "the pull request is open", "the migration is
  applied", "CI is green on the branch." These have an expiry you cannot see
  from here.

**3 — Go and look.**

Establish the world's answer for anything in the second category that your next
action depends on, and nothing more. The slate reads the obvious things — the
worktree, the branch, the head, uncommitted paths, the pull request, the issue.
It does not read your migration state, your deployment, your CI run or your
service's health, and it should not: it does not know which of those your work
touches. You do.

Anything the slate lists as **Unestablished** is unknown and not absent. A
tracker that could not be reached looks exactly like a branch with no pull
request from inside the machine, and treating the first as the second opens a
second pull request over the first.

**4 — Reconcile, and expect the record to be behind rather than wrong.**

Where the two voices differ, the interesting question is never which is correct —
the world is correct — but *what happened in between*. A branch that moved, a
worktree a cleanup removed, and a pull request somebody merged all look like the
same disagreement and mean entirely different things. Work out which, and what
it implies for the work still to do.

Discard what no longer holds. A hypothesis the world has already answered is not
worth carrying, and a next step written against a state that no longer exists is
worse than no next step.

**5 — Settle anything that began and never finished, before anything else.**

An action that started and recorded no outcome — a push, a merge, a comment, a
deploy — is the one case where the safe move is not obvious and the wrong move
is expensive. It may have completed, half-completed, or never started.

Do not reason about which. Look:

```
gh pr view <number> --repo <owner>/<repo> --json state,mergedAt,mergeCommit
git -C <source> log --oneline -5 <branch>
```

Then record what you established on the assignment, with the evidence, before
doing anything that would repeat it.

**6 — What you may do is what the partnership says now.**

Not what an earlier session was told, and not what a note in the record assumes.
If a written next step is no longer something the owner has delegated, the step
is out of date — put it to them as a decision rather than carrying it out on an
expectation formed before.

**7 — Then continue, and keep the record current as you go.**

```
mellions assign record <id> -kind found "the branch merged; this worktree is 9 behind the base"
mellions assign record <id> -kind next  "re-cut from dev, then falsify against the three known rows"
```

## Renewal — a boundary you choose, never a question you ask

You cannot see your context or compact it: the runtime compacts at its window;
a person can type `/compact`. `mellions state` gives you the measured
facts as information, never as an instruction to stop. Renewal is judgment at
a boundary: a phase complete, its state preserved where the next context reads
it, what remains here mostly finished-work noise, the next phase needing fresh
reasoning — yes is a boundary, whatever the size says. Renewing is an act:
write where it stands (`mellions assign record -kind next "..."` — not the
handoff, which ends the lane), then a fresh context — dispatch the next phase
to a session with the slate and judge what comes back; unattended, end the
shift and the runner starts the next; the new context re-establishes reality
with `mellions continue`. Carrying on is legitimate too: the runtime compacts
and the renewal hook carries your slate. Never ask the owner to `/compact`,
never wait for it, never raise the size mid-piece.

## What not to do

**Do not replay the record.** A written next step is a proposal from a session
that could not see today. Evaluate it; do not execute it.

**Do not treat silence as absence.** Everything you could not establish stays
unknown and gets said out loud before you reason from it.

**Do not write a status document.** What the next session needs is the
assignment record kept current — one line at the moment you learn something.

**Do not start something new because reconstructing is harder.** Two open
assignments and a fresh idea is how the two become abandoned. If the work is
genuinely no longer the most valuable thing, say so on the record — what took
priority, and where this stands — which is a decision, not a drift.

## Stop points

- An action began and never finished, and you cannot establish from the world
  whether it landed. Stop and ask. Guessing here is the one failure that
  produces a duplicate irreversible effect.
- The worktree is gone and it held uncommitted work. What is lost is lost;
  say so plainly rather than reconstructing something that looks similar.
- The record's reasoning and the world's state cannot both be true and you
  cannot tell which broke. That is an investigation, not a recovery — open it
  as one.

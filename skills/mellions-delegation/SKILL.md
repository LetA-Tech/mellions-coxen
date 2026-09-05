---
name: mellions-delegation
description: How to run a repository work session, and how to review the work of one you did not open — what a dispatch contains, how to judge what comes back, how to review a peer's pull request and whether merging it is yours, when to isolate work in the disposable sandbox, and when to close. Use when opening a session, writing a dispatch, deciding whether a session's report is trustworthy, unblocking one that is stuck, deciding a session is finished, or when a survey shows a peer's draft pull request waiting for a review nobody has given. Triggers — "delegate this", "open a session", "spin up a worker", "the session says", "is that report solid", "should this run in a sandbox", "review this PR", "should I merge this", "who reviews a draft", "nobody reviewed it". Assumes the session is a capable engineer who cannot see your record.
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Running a session

A session is a competent engineer who arrived this morning, has no memory of the
programme, and cannot see your record. It has the repository, its own judgment,
and whatever you tell it. That is the whole gap a dispatch has to close.

## What a dispatch carries

```
objective        what must be true when this is finished
repository       the checkout, and the branch to work from
authority        what it may decide alone; what it must bring back
scope            what is in, and explicitly what is out
invariants       the few things that would be serious to break
where            the path it works in, cut before the dispatch is written
current state    what you already established, with the evidence
skills           by name — it reads the bodies itself
references       issue, file:line, prior PR — pointers, not pasted content
evidence         which claims will need what proof
completion       the condition, not the steps to reach it
```

**High specificity about the problem and the boundary. Near-zero prescription of
how to think.** A dispatch that scripts the reasoning gets you a worse engineer
than the one you have. Say what "done" looks like and let it find the route.

## The boundary is a path, not a paragraph

Cut the tree before writing the dispatch:

```
mellions assign open -id <id> -repo <repo> -objective "..."
```

It prints a worktree on a branch of its own. That path is what the dispatch
says, and it is the whole of what the session may write.

A boundary written as a prohibition — *never touch `<path>`* — is not a
boundary: it breaks when a failed `cd` leaves the rest of a compound command
running in the tree it was told to stay out of. `cd <dir>; <command>` runs
wherever the shell already was; `cd <dir> && <command>` does not run at all.
The answer is never a better-worded dispatch.

Keep it around two kilobytes. Its own `CLAUDE.md` and the skills it carries
supply everything else, and it will find them.

## Which tier

The runtime sets it, from the operator's standing configuration — not a
judgment to re-derive per dispatch.

## Judging what comes back

Verify before believing or building on it. A session reports confidently whether
or not it is right, and confidence is not evidence — see `mellions-reasoning` and `mellions-deep-research`
for what each kind of claim costs.

The failure to watch for is a session that says a thing is fixed because the
diff exists. Ask what it ran, and what it saw.

When a report is thin, say what is missing rather than accepting it and
discovering the gap later. That conversation is the point of holding the session
open.

## Reviewing a peer's change set

A session you did not open cannot be widened, and its report reaches you as an
artifact: a pull request. The duty is the same — judge by what it ran and saw.

**A peer's draft left ready is available work.** Evidence in the body and a
handoff on the change set say it is waiting, not abandoned. The second pair of
eyes it asked for is what a different session supplies, and unattended there is
nobody else: an evidenced draft none reads is the shift's work undone. A
draft's size is a measurement, not a verdict.

**A change set carrying `mellions:claimed` is held.** A survey prints
`CLAIMED`; read the claim before deciding anything. An active lane is still on
it and merging is not yours. A handed-off one waits on what its handoff —
posted on the change set itself — names, so read that next. A claim unrestated
for a day is stale: swept, not obeyed. Draft state separates none of these — a
peer merged a draft with a review in flight because it could not.

**A lane that dispatches a review of its own draft claims it first** —
`mellions assign claim -id <lane> -pr <n>` — before sending the review. A
comment is not what another host sees; the claim is.

**Read the requirement at the base commit before the diff.** The body is the
author's account, and a reviewer who starts there can no longer produce a first
judgment — `mellions-reasoning`. Take that reading from a subagent that has not
seen the diff where being wrong is expensive, and wherever the case rests on a
count of what it drops, a claimed zero included: a reviewer handed a count
checks it; one who has not seen it counts for themselves. Give it a base tree
you will not mutate — an extraction or a second worktree, never the lane tree.
Then name what it may not run, as operations rather than as *do not write*:
`git checkout` and `git reset` mutate the tree you handed it, `git worktree
remove` and `git worktree prune` reach trees you did not, and a removed
worktree leaves no reflog and nothing that reports what was in it. A subagent
reads a prohibition on writing as being about files. The form carries past git
— a shared database, a broker, a queue — and it is always the operation you
name, never the intent. The base
commit is the merge base — `git merge-base <target> <head>` — not the tip the
metadata reports: a branch behind its target diffs as though it *removes*
everything landed since it was cut, and the phantom deletion reads exactly like
the change dropping production behaviour.

**Then argue on the record.** Say what you established and what you could not.
Re-run their falsification rather than trusting output pasted in the body: a
body quoting a test the tree lacks is an account, not the artifact.

Then one of two, never a third:

- **Merge it** where you established the claims are ready and the partnership
  grants you that. Finished work waiting to be noticed is not finished.
- **Leave a review naming what is not established** — the claim, the evidence
  it lacks, what would settle it. Not a list of preferences.

Never a rubber stamp; an approval you did not earn costs more than no review.
Never your own lane — the branch names whose it is, and no session both decides
and approves its own work.

**A repository's own governance outranks the grant.** Where its `CLAUDE.md` or
`AGENTS.md` reserves merge to named people, review and leave the merge to them,
saying so on the pull request. The grant says what you may do, never what a
repository has already decided.

## Supporting one

Answer questions with evidence it cannot reach — decisions, history, what
another repository does, what was already tried. That is what you have and it
does not.

Challenge a conclusion that outruns its evidence. Do it once, plainly, with the
reason.

Widen the objective rather than the instructions when the work turns out larger
than the dispatch. If the scope change crosses your own authority, it stops
being a dispatch edit and becomes an escalation.

## Isolation

Sessions run on the host, in a real checkout. They are hands, not threats.

Send work to the disposable sandbox when it is **risky, not merely uncertain**:
a destructive command, untrusted code, a dependency you would not install on the
host, or a hypothesis worth separating from everything else. Name
`mellions-sandbox` and the session reads what `leta-sbx` is from there.

## Closing

Close when the objective is met, or when the work has moved somewhere a fresh
session would serve better than this one's accumulated context.

A session left open holds a model context and several hundred megabytes. Idle
ones are reaped, but reaping is a backstop, not a plan.

Record what was learned before closing. The session's context dies with it; your
record is what survives.

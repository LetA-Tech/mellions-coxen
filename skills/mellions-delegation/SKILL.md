---
name: mellions-delegation
description: How to run a repository work session, and how to review the work of one you did not open — what a dispatch contains, how to judge what comes back, how to review a peer's pull request and whether merging it is yours, when to isolate work in the disposable sandbox, and when to close. Use when opening a session, writing a dispatch, deciding whether a session's report is trustworthy, unblocking one that is stuck, deciding a session is finished, or when a survey shows a peer's draft pull request waiting for a review nobody has given. Triggers — "delegate this", "open a session", "spin up a worker", "the session says", "is that report solid", "should this run in a sandbox", "review this PR", "should I merge this", "who reviews a draft", "nobody reviewed it". Assumes the session is a capable engineer who cannot see your record.
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Running a session

A new session has its own judgment and repository, but not your record.
A dispatch supplies that gap.

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

Specify the problem and boundary, not the reasoning. Honor owner concurrency
limits; coordinate shared files, devices, heavy jobs and merges. The lead stays
accountable alongside independent review and Git support.

## The boundary is a path, not a paragraph

Cut the tree before writing the dispatch:

```
mellions assign open -id <id> -repo <repo> -objective "..."
```

It prints a worktree on a branch of its own. That path is what the dispatch
says, and it is the whole of what the session may write.

A prohibition such as *never touch `<path>`* is not a boundary: a failed
`cd` can leave a compound command writing in the wrong tree.

Keep the dispatch near two kilobytes; its `CLAUDE.md` and named skills
supply the rest. Roles follow the session-start workflow: Opus orchestrates,
falsifies and accepts; Sonnet implements and tests; Sonnet does git and
landing; Opus gives the one final review. Honor owner overrides and the
native model names; reassign if cause or design reopens.

## Judging what comes back

Verify before building on it: confidence is not evidence, and
`mellions-deep-research` is how. A diff is not a fix: ask what it ran, and what
it saw. When a report is thin, say what is missing before building on it.

A chained run (script, loop, pipeline) returns its last step's report; what
an earlier step left behind or asked of you reaches you only if the chain passes
it on. Make it return every step's report, every round and item, and read each
footprint.

## Reviewing a peer's change set

A session you did not open cannot be widened, and its report reaches you as an
artifact: a pull request. The duty is the same.

**A peer's draft left ready is available work.** Evidence in the body and a
handoff on the change set say it is waiting, not abandoned, and unattended
nobody else will read it. A draft's size is a measurement, not a verdict.

**A review count of zero is not evidence nobody read it.** A read here is posted
as a comment, so a change set read twice can still report `reviews: 0`. Read the
disposition of any read already there: where one returned changes requested and
the head has not moved since, the work waiting is the remedy it named, not a
second opinion.

**A change set carrying `mellions:claimed` is held.** A survey prints
`CLAIMED`; read the claim before deciding anything. An active lane is still on
it and merging is not yours. A handed-off one waits on what its handoff —
posted on the change set itself — names, so read that next. A claim unrestated
for a day is stale: swept, not obeyed. Draft state separates none of these.

**A lane that dispatches a review of its own draft claims it first** —
`mellions assign claim <lane> -pr <n>` — before sending the review. A
comment is not what another host sees; the claim is.

**One review per coordinated set, before merge.** Read the requirement before
the diff; the body is the author's account. For a schema or credential change,
optionally take that reading from a subagent that has not seen the diff, given
the original claim rather than a tidied one.
The base is `git merge-base <target> <head>`, not the metadata's target tip:
a branch behind its target appears to remove newer production changes at that tip.

**Then argue on the record.** Inspect the diff and raw proof. Reuse accepted
validation under `mellions-deep-research`'s input-validity rule; independently
settle gaps and risky claims. Required gates and falsification apply to the head
reconciled with the current target; a stale branch can pass alone and fail merged.
Follow-up review covers the delta and affected paths; reopen completed conclusions
where their inputs changed. At publishing, read the target's tip and PR state/head
(`gh pr view --json state,headRefOid`; a ref read can miss a deleted head): a
changed input requires affected checks or a verdict scoped to verified pins.
Say if the PR is no longer open. A cited test the tree lacks is an account.

Then one of two, never a third:

- **Merge it** where you established the claims are ready and the partnership
  grants you that. Finished work waiting to be noticed is not finished.
- **Leave a review naming what is not established** — the claim, the evidence
  it lacks, what would settle it. Not a list of preferences. Post it with
  `gh pr review --comment`, not `gh pr comment` — same text, and only the
  first is counted.

Never a rubber stamp; an approval you did not earn costs more than no review.
Never your own lane — the branch names whose it is.

**A repository's own governance outranks the grant.** Where its `CLAUDE.md` or
`AGENTS.md` reserves merge to named people, review and leave the merge to them,
saying so on the pull request.

## Supporting one

Answer with evidence it cannot reach: decisions, history, what another
repository does, what was tried.

Challenge a conclusion that outruns its evidence. Do it once, plainly, with the
reason.

Widen the objective rather than the instructions when the work turns out larger
than the dispatch. If the scope change crosses your own authority, it stops
being a dispatch edit and becomes an escalation.

## Isolation

Sessions run on the host, in a real checkout.

Send work to the disposable sandbox when it is **risky, not merely uncertain**:
a destructive command, untrusted code, a dependency you would not install on the
host, or a hypothesis worth separating from everything else. Name
`mellions-sandbox`; the session selects this host's mechanism there.

## Closing

Close when the objective is met, or when the work has moved somewhere a fresh
session would serve better than this one's accumulated context.

Record what was learned before closing. The session's context dies with it; your
record is what survives.

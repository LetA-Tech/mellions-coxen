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

A boundary written as a prohibition — *never touch `<path>`* — is not a
boundary: it breaks when a failed `cd` leaves the rest of a compound command
running in the tree it was told to stay out of.

Keep dispatches near two kilobytes; `CLAUDE.md` and named skills supply context.
Owner-configured roles: Opus 5.5 (or Sol 6.1) XHigh reviews risky contracts and
implementation early; later review covers corrections and remaining gaps.
Sonnet 5.5 or Sol 6.1 High handles Git and landing. Honor owner overrides and
actual native model/effort settings; names assert no availability. Capable
implementers carry grounded fixes; reassign if cause or design reopens.
Restricted tiers need approval.

## Judging what comes back

Verify before believing or building on it. A session reports confidently whether
or not it is right, and confidence is not evidence — see `mellions-reasoning`
and `mellions-deep-research`.

The failure to watch for is a session that says a thing is fixed because the
diff exists. Ask what it ran, and what it saw.

When a report is thin, say what is missing rather than accepting it and
discovering the gap later.

## Reviewing a peer's change set

A session you did not open cannot be widened, and its report reaches you as an
artifact: a pull request. The duty is the same.

**A peer's draft left ready is available work.** Evidence in the body and a
handoff on the change set say it is waiting, not abandoned, and unattended
nobody else will read it: an evidenced draft none reads is the shift's work
undone. A draft's size is a measurement, not a verdict.

**A review count of zero is not evidence nobody read it.** A read here is posted
as a comment, so a change set read twice still reports `reviews: 0` — 18 of 31
open ones across eight repositories did. Read the disposition of any read already
there: where one returned changes requested and the head has not moved since, the
work waiting is the remedy it named, not a second opinion.

**A change set carrying `mellions:claimed` is held.** A survey prints
`CLAIMED`; read the claim before deciding anything. An active lane is still on
it and merging is not yours. A handed-off one waits on what its handoff —
posted on the change set itself — names, so read that next. A claim unrestated
for a day is stale: swept, not obeyed. Draft state separates none of these.

**A lane that dispatches a review of its own draft claims it first** —
`mellions assign claim <lane> -pr <n>` — before sending the review. A
comment is not what another host sees; the claim is.

**First review: read the requirement before the diff**, at
`git merge-base <target> <head>`; the target tip makes a behind branch appear
to remove newer changes. The author's body biases judgment. Where being wrong
is expensive, use an independent subagent: original claim, counts including
zeros and citations unchanged; issue body without comments revealing the fix;
an extraction you never write to, separate from lane and review scratch.

**Record the verdict.** Inspect diff and raw proof. Reuse accepted validation
under `mellions-deep-research`'s input-validity rule; independently settle gaps
and risky claims. Required gates and falsification apply to the head reconciled
with the current target. Follow-up review covers the delta and affected paths;
reopen conclusions whose inputs changed. At publishing, read target tip and PR
state/head (`gh pr view --json state,headRefOid`; a ref can miss a deleted head).
Changed inputs need affected checks or a verdict scoped to verified pins.
Say if the PR closed. A cited test absent from the tree is not evidence.

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

Answer questions with evidence it cannot reach — decisions, history, what
another repository does, what was already tried.

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

An idle session holds a model context and memory; the reaper is a backstop.

Record what was learned before closing. The session's context dies with it; your
record is what survives.

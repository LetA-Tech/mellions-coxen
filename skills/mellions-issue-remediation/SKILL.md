---
name: mellions-issue-remediation
description: Load this whenever the instruction is to remediate, fix, build, ship or land the work a defect or work item describes — before the worktree is cut. Triggers — "remediate this", "fix issue NN", "implement the fix", "ship the remediation", "open the PR for this", "land this work", "start the implementation". Not for deciding whether a defect is real (mellions-bug-audit) or for closing the issue afterwards (mellions-issue-closure).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Remediating a work item

Turns a work item into verified code: a worktree, the change, its proof, and a
pull request whose body is the evidence.

The repository's own manual defines what production-grade means there; this
skill enforces only that the bar is **applied and demonstrated**.

## A remediation, not a narrow patch

The objective is the root cause and verification end to end, never the smallest
change that makes the reported symptom stop.

**A failing fixture is a claim about the world.** Establish what it asserts and
whether that is true *before* touching it. One corrected because the evidence
says it encodes the wrong semantics is legitimate, and the reasoning goes in the
pull request; one edited because it was red is fabricated compliance evidence,
and so is an assertion moved to match the behaviour.

Follow the blast radius across repository boundaries: where the cause lives
upstream, the remediation includes that work or names it as an owned, linked
item carrying enough evidence to execute independently.

## Workflow

**1 — Read everything, and establish the ground.** From the work item in full,
its plan, and the repository's `CLAUDE.md`, establish: the branch to work from
and target; the release branch, which is never a base for implementation; the
verification commands; and any path that must not enter the diff without
authorization. Where none of them declares the base, `git symbolic-ref
refs/remotes/origin/HEAD` gives the remote's default — a candidate, not a
declaration: where a release branch is kept it names the branch this step
forbids. What merged pull requests mostly target settles it (`gh pr list -s
merged --json baseRefName`); read the majority, since release merges land on
the default too. Say in the pull request body that the base was inferred, not
declared. No plan on the item → write one there first, by
`mellions-issue-resolution-proposal`, before the first commit. No item at all —
work nobody filed — → file one (`mellions-issue-creation`) and plan it there;
the pull request is an artifact on the item, never a substitute for it.

**2 — Re-verify the premise and state the pin.** A work item is a claim written
at a commit. Re-open every file it cites, confirm every `file:line` resolves at
the base branch HEAD now, and state in the pull request body the commit the item
was written at, the commit verified against, and whether the citations resolve.
Drifted → report the drift and correct the claim; the code wins over the item's
text. Premise gone → say so on the item and re-plan there, never silently
redefine the work.

Resolving citations is the cheap half: an item can cite perfectly and be wrong
about what the citations mean. `mellions-issue-resolution-proposal` carries the
six questions that settle it. Load it even where a plan exists.

**3 — Isolate, then gate.** One item per worktree, cut from the base branch at
its current remote tip; `mellions assign open` does this. Then run every
verification command and record the result **before** touching code: a gate
captured only afterwards cannot distinguish a failure you introduced from one
already there. A defect found mid-implementation is decided, never quiet: in
this item's code path or contract it is yours, fixed here with its own test;
outside it, recorded and filed.

**What will prove this, and is that surface reachable from this host now?** A
database, a broker, a container runtime is a property of the host, not of the
repository, and a gate that never touches one says nothing of it. Unreachable
re-scopes the work or is priced in as a startup cost — decided now, not
mid-implementation.

**4 — Enumerate the class before editing.** The item names sites; the defect has
a class. List **every** site sharing the root cause with `file:line` before the
first edit — "I will find them during implementation" reliably fixes the
reported path and leaves its siblings broken. Scope materially larger than the
plan → widen the plan on the item first.

**5 — Implement.** Every file is opened before it is changed. Fix the cause at
the point every caller routes through, not each call site in turn. Machinery the
change makes unreachable is deleted with its tests, not deprecated and not left
behind a flag. A defect found in a file being changed is fixed here or filed as
its own item; a deferral marker is neither. Comments follow
`mellions-comment-hygiene`.

**6 — Prove it.** The gate runs in full, raw output captured. Then, for each
invariant the change establishes, neutralise the fix, run the named tests,
record the failure, restore, record them passing — by hand is fine, in your
head is not. Assertions name the observable: the field, the expected
value, the failure mode caught. Verdict **PASS** (gate green, every criterion
met), **PARTIAL** (gate green, a criterion unmet, say which) or **FAIL**; a
green gate with a red criterion is never PASS.

**7 — Open the pull request.** Target the base branch. The **body is the durable
proof document**, the one artifact where length is the point: the work item as
`Refs #NN`, never a closing keyword — it is a close, taking the authority
and accepted proof a close takes, and off the default branch resolving
nothing at all, silently; the premise re-verification and its pin; what was wrong, with
`file:line`; raw reproduction output at a named commit;
the change item by item, naming what was deleted; the falsification runs;
each acceptance criterion mapped to met or unmet; every command run and its
result, each gate cited from a run of it and never from the file declaring it,
because a workflow that is disabled, unfunded, filtered or never triggered
executes nothing; and anything
out of scope, blocked or contradictory, surfaced rather than resolved silently.

**8 — Finish, or hand over.** Report the branch, the pull request, the commit
and the verdict, on the pull request and on the assignment.

**A change to a surface a person reads is not finished while a document still
describes the old one.** No suite reads prose, so the gate stays green and
nothing tells you. Grep the repository for the surface's own name before
opening the pull request; where the document is another repository's, name it
there.

Where the remediation spans repositories the pull requests are **one coordinated
set**: name every one, the merge order, and what breaks if it is merged in part
— a partially merged set leaves the platform in a state neither contract
describes. Hand over all of it or none.

Merging and closing follow the partnership, closing by
`mellions-issue-closure`; where they are the owner's, the pull request plus the
decision package is the finished state and a successful one. Only once the merge
is verified **at the remote** — the merge commit on the base branch at origin,
not only locally — remove the worktree and delete the task branch. One deleted
on the strength of a local-only merge destroys the only copy of the work.

## Stop points

- The base branch cannot be established, or the baseline gate is already red.
- No plan on the work item and nowhere to record one; or its premise no longer
  holds at HEAD.
- The scope exceeds the plan and widening it is the owner's call, or a protected
  path would enter the diff without authorization.
- A contract the work depends on is undefined or self-contradictory — surface
  it, never improvise a fallback.

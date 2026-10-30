---
name: mellions-issue-remediation
description: Load this whenever the instruction is to remediate, fix, build, ship or land the work a defect or work item describes — before the worktree is cut. Triggers — "remediate this", "fix issue NN", "implement the fix", "ship the remediation", "open the PR for this", "land this work", "start the implementation". Not for deciding whether a defect is real (mellions-bug-audit) or for closing the issue afterwards (mellions-issue-closure).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Remediating a work item

The repository defines production quality; **apply and demonstrate its bar**.

## A remediation, not a narrow patch

**A failing fixture is a claim about the world.** Establish what it asserts and
whether that is true *before* touching it. One corrected because the evidence
says it encodes the wrong semantics is legitimate, with the reasoning in the
pull request; one edited because it was red is fabricated compliance evidence,
as is an assertion moved to match the behaviour.

## Workflow

**1 — Establish the contract and ground.** From the work item,
its plan, and the repository's `CLAUDE.md`, establish: the branch to work from
and target; the release branch, which is never a base for implementation; the
verification commands; the acceptance criteria, read in full wherever they sit
— a truncated read drops the last sections, a body-only read the comments; and
any path that must not enter the diff without authorization. Where none declares the base, `git symbolic-ref
refs/remotes/origin/HEAD` gives the remote's default — a candidate, not a
declaration, and where a release branch is kept it names the one this step
forbids. What merged pull requests mostly target settles it (`gh pr list -s
merged --json baseRefName`). Say in the body that the base was inferred. A current
issue contract or plan is enough; an architecture or cross-service contract
change goes through `mellions-issue-resolution-proposal` first. No item at all
→ file one (`mellions-issue-creation`) and plan it there. The pull request is
an artifact on the item, never a substitute for it.

**2 — Reconcile the premise with the current target.** Reuse established source
understanding and evidence under `mellions-deep-research`'s input-validity rule.
Inspect relevant changes since its pin and reopen changed or unresolved claims;
resolving a citation alone does not establish its meaning. Record the current
pin, evidence reused and material drift. The code wins over the item's text;
a premise that no longer holds is corrected and re-planned on the item, never
silently redefined.

**3 — Isolate and preserve a useful baseline.** One item per worktree, cut from the base branch at
its current remote tip; `mellions assign open` does this. Then run every
baseline check the repository requires. Otherwise retain accepted baseline
evidence whose relevant inputs are unchanged, and run the focused checks or
reproduction needed to attribute plausible failures before editing. A result
captured only afterwards cannot establish what already failed. Missing or
invalid evidence is obtained, not assumed. A defect found mid-implementation is decided, never quiet:
inside the causal lane — the same root cause, contract, execution path, data
flow, persistence consequence or defect family — it is yours through verified
resolution, fixed here with its own test; only a genuinely independent lane
becomes its own item, on the causal relation and never on cost.

**What will prove this, and is that surface reachable from this host now?**
Unreachable re-scopes the work or is priced in as a startup cost — decided
now, not mid-implementation.

**4 — Cover the class.** Establish the affected paths and siblings sharing the
root cause before editing them; reuse a verified inventory where its inputs
still hold. Do not fix only the reported instance. Scope materially larger
than the contract → update it on the item first.

**5 — Implement.** Every file is opened before it is changed. Fix the cause at
the point every caller routes through, not each call site in turn. Machinery the change makes unreachable is deleted with its
tests, not deprecated and not left behind a flag. Comments follow
`mellions-comment-hygiene`.

**6 — Prove it.** Use focused checks while editing. Where the defect can be executed the proof has two ends:
reproduced **before** the fix at a named commit, attacked **after** it, in
`mellions-sandbox` where the host should not carry it. The gate runs in full,
raw output captured; then, for every claim on an expensive path, the arms
and independent oracle of `mellions-falsification`. A check still green once
the fix is neutralised is defective evidence whatever the suite says; where no
such claim exists, say so rather than invent an arm. Assertions name the
observable: the field, the expected value, the failure mode caught. Verdict
**PASS**, **PARTIAL** or **FAIL**; a green gate with a red criterion is never
PASS — and **PARTIAL is not a place to rest**: before it is written the unmet
criterion is diagnosed to root cause and classified as `mellions-reasoning`
classifies an obstacle, and what it leaves to you done.

**7 — Open the pull request.** Target the base branch. Keep the body a concise
index of the outcome and proof: the work item as `Refs #NN`, never a closing
keyword (`mellions-issue-closure` says why); the verified base and head; the
actual change and material premise deltas; each acceptance criterion met or
unmet with its evidence; gate and falsification results; residual risks and
scope decisions. Keep complete raw evidence once in durable artifacts the
reviewer can open, linked at the claim it supports, including reused evidence
and its validity. A local temporary log alone is not a durable record. Cite a
gate from its run, never a declaration or a file saying CI is off. Read the pull
request's own checks once settled after the push (`gh pr checks`; none is a
result to state): a red this change causes fixed, any other attributed to
its cause or said to have none established.

**8 — Finish, or hand over.** Report the branch, the pull request, the commit
and the verdict, on both the pull request and the assignment.

**A change to a surface a person reads is not finished while a document
describes the old one.** No suite reads prose, so the gate stays green and
nothing tells you. Grep for the surface's own name before opening the pull
request; where the document is another repository's, name it there.

Where the remediation spans repositories the pull requests are **one
coordinated set**: name every one, the merge order, and what breaks if merged
in part — a partial merge leaves the platform in a state neither contract
describes. Hand over all of it or none.

Merging and closing follow the partnership, read and never assumed, by
`mellions-issue-closure`: where they are yours, merge, verify at the remote,
close, take the next thing; where it reserves them, the pull request plus the
decision package is the finished state and a successful one. Only once the
merge commit is on the base branch **at origin** remove the worktree and the
task branch: one deleted on a local-only merge destroys the only copy of the
work.

## Stop points

Each is a stop only once its cause is established; until then it is the next
problem.

- The base branch cannot be established, or the baseline gate is red for a
  cause outside this lane.
- No current contract or required plan and nowhere to record it; or its premise
  no longer holds at HEAD.
- The scope exceeds the plan and widening it is the owner's call, or a protected
  path would enter the diff without authorization.
- A contract the work depends on is undefined or self-contradictory — surface
  it, never improvise a fallback.

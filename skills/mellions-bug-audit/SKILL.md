---
name: mellions-bug-audit
description: Load this when asked to audit a subsystem, hunt for bugs, investigate a suspected defect, verify whether a reported problem is real, or establish root cause and blast radius before any issue or fix exists. Triggers — "audit this", "find bugs in", "is this actually broken", "root cause this", "what is the blast radius", "verify this finding", "review this subsystem for defects". Produces findings, never code and never issues. Not for planning or writing the fix (mellions-issue-remediation) or reviewing a diff that already exists.
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Auditing for defects

Establish, from the executable implementation, whether a defect is real — what
causes it, what it reaches, what evidence supports each claim. The output is a
finding another engineer can act on without repeating the investigation: what
must become true, never the diff and never an issue, which a confirmed finding
becomes by `mellions-issue-creation`.

## What the ground truth is

`mellions-deep-research` ranks the sources. For an audit that means behaviour is
established from the implementation at a named commit, the tests that exercise
it, execution and runtime observation — and where a document and the code
disagree, the code is correct and the document is stale, said so in the finding,
because a stale document nobody corrects produces the same wrong conclusion
next quarter.

**Where the artifact under audit is itself a process** — a doctrine, a canon, a
policy, a skill — its ground truth is what the fleet actually does, and that is
counted, not read: a document asserting a control is a claim that the control is
operated. One estate's canon documented a ten-label lifecycle no registry
defined and a gate whose permitting label had been applied once across 860 open
issues — one query each, settling what re-reading the prose could not.

## Tier every claim

Established, inferred and unknown, in the words a finding uses: **confirmed from
source** (code at a named commit, `file:line` with the quoted lines),
**evidence-supported** (follows from the code, not executed or observed), and
**requires live evidence** (unsettleable from the repository — name the query,
series, log filter or captured response that would settle it). The last is a
legitimate finding; it is dishonest only dressed as the first. Label the
provenance with the tier — repository-derived, external, inference — because
collapsing the three is how an inference acquires the authority of a citation.

## Workflow

**1 — Fix the ground.** This bundle names no repository, branch or finding-ID
scheme. Take the base branch every `file:line` is verified against from the
repository itself — its `CLAUDE.md`, failing that `git symbolic-ref
refs/remotes/origin/HEAD`, said to be inferred rather than declared — record the
commit, confirm the tree is clean, and run the repository's verification
commands **before** investigating: a baseline captured afterwards cannot
distinguish a pre-existing failure from one the investigation caused.

**2 — Read the implementation end to end.** The complete files on the path, not
the neighbourhood of the suspected line, and the tests covering them: a test
asserting the wrong behaviour is itself a finding, and a path with no test is a
fact worth stating.

**3 — Trace the call and data flow.** Entry point → decision → store or provider
write, `file:line` at every hop. Do not stop at an interface, adapter, client,
worker or service boundary when the decisive behaviour is deeper: follow into
vendored SDKs and dependencies and cite those paths, and where the path crosses
into another repository, follow it there and pin that repository at its own
commit. A trace that stops at a boundary usually stops one hop short of the
cause. Ask the two questions the happy path never asks: **which store is written
in which order**, and **what a crash between two writes leaves behind**.

**4 — Reproduce.** A focused test, a deliberate mutation, a scripted request, a
log or metric query, with the raw output pasted. Where reproduction is genuinely
impossible, say what would be needed and tier the claim accordingly rather than
asserting the behaviour anyway. Where the claim rests on a test detecting the
defect, neutralise the fix by hand, watch the named tests fail, restore, watch
them pass, and record both runs; a compile error is not a failure, and a test
that cannot build has detected nothing, which is how a hand-run mutation reports
success it did not earn.

**An A/B that changes more than one variable establishes the symptom, never the
cause.** Two settings flipped together and the failure goes away: all you have
is that the pair matters, and a root cause naming one is a guess wearing a
reproduction's clothes. Change one, re-run, say which carried it — or write the
cause as unknown with both candidates named. The run really did go red to green,
which is why this ships a confident wrong mechanism and scopes the remedy to the
wrong variable. State the conditions the reproduction held — host, OS, and the
**version of every tool whose behaviour is the subject** — or the next
engineer's non-reproduction and yours are not comparable.

**5 — Establish root cause and blast radius.** The root cause is the mechanism,
not the symptom, and where two individually reasonable decisions are jointly
wrong, both are named. Then enumerate **every** site sharing the cause with
`file:line`, never "if the pattern exists elsewhere": the named site is one
instance, the finding covers the class, and reporting the instance is how an
audit produces a defect that returns.

A rule carrying several bespoke exceptions is evidence about the rule, not about
the exceptions: each was added by someone reasonable to relieve a case the rule
broke, and read together they describe the premise that does not hold. One
closure gate took an override and three per-case rulings before anybody noticed
it was trying to prove which principal authorized an action in a fleet where the
automation and the human shared a login. Attack the premise first.

**6 — Separate confirmed from hypothesis.** **Confirmed** (root cause
established, evidence tiered), **hypotheses** (with exactly what would confirm
or refute each), **open runtime questions**. Only confirmed findings become
issues; a hypothesis promoted to one is a contract nobody can satisfy. Each
carries: the defect and its consequence · the root cause mechanism · tiered
evidence with `file:line` at the named commit · the call-and-data-flow trace ·
every site in the class · the runtime conditions that make the path live · what
a careless fix breaks · what would prove it fixed.

## Non-negotiables

1. **One root cause per finding.** Consolidate the symptoms of one defect; never
   merge independently remediable problems into one omnibus finding.
2. **Nothing supports a section → say so**, rather than filling it. A path, a
   call graph, a line number or an output nobody opened is invention.
3. **Subagent findings are consolidated and verified before use.** They
   misreport line numbers and cite files that do not exist, and overlapping
   briefs return one finding several times, which reads as corroboration.

## Stop points

- The base branch cannot be established, or the tree is dirty, so `file:line`
  cannot be pinned to a commit.
- The baseline is already failing: establish whether that failure is the finding
  before continuing.
- A cited path does not resolve at the named commit — report the drift rather
  than adjusting the claim to fit.
- The cause is reachable only through a repository this audit cannot read. Name
  it and what must be established there; never infer it from the consuming side.

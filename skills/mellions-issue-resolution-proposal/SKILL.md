---
name: mellions-issue-resolution-proposal
description: Load this after the issue exists and before the first commit — to write the resolution plan on the issue itself, in the repository's approval model, so the plan survives the session, a reviewer can approve one comment, and closure can later read the acceptance rule it was made under; and again when the premise moved and the plan must be re-made. Triggers — "post the proposal", "write the plan on the issue", "record the plan", "document the proposal on issue NN", "before I start on NN". Not for producing the analysis (mellions-bug-audit), implementing (mellions-issue-remediation) or closing (mellions-issue-closure).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# The contract goes on the issue

A plan that lived only in a session is lost with the session, and the issue
then has no trace of what was intended, what was approved, or what "done" was
supposed to mean. Before implementation, the issue must carry a current contract
that a reviewer can judge, an implementer can follow and closure can read the
acceptance rule from. A sufficient issue body or existing plan needs no duplicate
proposal; add a concise comment for missing decisions or material changes. Use a
separate proposal and its prescribed schema wherever the repository requires one.

## The repository's approval model decides what the comment is

Read the repository's `CLAUDE.md` and whatever issue process it carries under
`.claude/skills/` before writing. Three postures exist and the repository
declares which:

- **Approval before code.** The comment is posted and the work waits for a
  named authority's word; no approval, no implementation. Where the authority
  gave the approval with refinements, the comment carries the refinements,
  verbatim, and they supersede the original plan.
- **The gate is at merge.** The comment is the implementer's plan, posted without
  claiming approval and saying so — "awaiting ratification at merge" in the
  comment itself — and the human decision moves to the pull request.
- **Delegated.** Where the partnership delegates the work, record the contract
  and proceed; existing authorization does not need another approval request.

Never claim an approval that was not given, never soften a conditional one,
and never backfill a required pre-code proposal after the code exists — say
instead that the plan was written late and why.
Where the repository's process carries markers, templates or approval
records, use them as it specifies; this method is what goes inside them.

## Read before writing

The issue number is the identifier; a finding id resolves to exactly one
issue first (`mellions-issue-closure`, Identity). Read the issue's contract,
existing decisions, approval conditions and unresolved reviews. Reuse established
source understanding and evidence under `mellions-deep-research`'s input-validity
rule; inspect relevant changes at the current target and settle unresolved gaps.
A premise that moved is corrected on the issue before work depends on it.

Use these questions to settle gaps or changes, rather than repeat completed
analysis. The pull request links that analysis and records what actually changed:

- **Do the citations resolve, and is what the item says about them still true?**
  Resolving is the cheap half — an item can cite perfectly and be wrong about
  what the citations mean.
- **Are several root causes conflated?** Enumerate them. An item naming one and
  proposing one diff can be satisfied while the others stand, and will close
  looking complete.
- **Do the remedies it proposes actually close the defect, or only the trace by
  which it was noticed?** An option is a claim, not an instruction, and can be
  false while every citation in the item is true. Where it turns on what a
  doctrine, specification or contract requires, read the provision itself and
  name it; where none of them closes the defect, say so and state the remedy
  that does, before any code.
- **Would the change ship a declaration that is false?** Contract text,
  comments, schema declarations and messages are claims that ship; one wrong for
  a subset of live rows is worse than the silence it replaces.
- **Do the existing tests prove the intended semantics, or encode the current
  behaviour?** Establish which before treating a red test as a blocker or a
  green one as coverage.
- **Would it reintroduce a previously fixed defect?** Read the history around
  the code to be touched: a fix whose rationale nobody recorded is the one most
  often undone.

## What the comment carries

Titled for the issue — `Resolution proposal — Issue #NN` — and containing, only
where it applies:

- the base branch and pin, root cause and affected class, with links to the
  established source evidence and any premise deltas;
- the outcome, scope and constraints, including material schema, persisted-data,
  migration, recovery or cross-repository consequences;
- focused verification, the required final gate, meaningful falsification and
  any runtime proof surface that must be reached;
- **the acceptance rule** the issue will close under, in the vocabulary
  `mellions-issue-closure` defines, so closure reads it rather than guesses it;
- material risks, stop conditions and authorization provenance: approved,
  awaiting ratification at merge, or delegated; state whether implementation has
  started. Never manufacture detail to fill an inapplicable field.

The comment reflects what was decided, not the first draft: refinements from
the authority or the reviewer are in it, quoted as written.

## Discipline

- Invent nothing: no file, call site, test or signal the analysis does not
  contain. A thin analysis makes a thin proposal, and the gap is said.
- The issue number, repository and base branch are verified against the
  actual issue; a mismatch is a stop.
- Quote raw evidence where it settles something; describe nothing you have
  not opened.
- The proposal is a plan, not a diff. Design that belongs in the code stays
  out of the comment; what belongs here is what a reader needs to judge the
  approach and later to check that the work matched it.

## Filing

Where a comment is needed, draft to scratch;
`gh issue comment NN --repo <owner/repo> --body-file <file>`.
Report its URL. Then, in the posture the repository declares: wait for
the word, or proceed to `mellions-issue-remediation`. The proposal is what the
pull request body will be checked against, and what `mellions-issue-closure`
reads the acceptance rule from.

## Stop points

- The premise no longer holds at HEAD and the item has not been corrected.
- The repository requires approval before code and none was given.
- A finding id resolves to zero issues or to several.
- A material change conflicts with an approved proposal: record the delta and
  satisfy the applicable approval rule, never silently replace it.
- The base branch in the plan is not the repository's declared base.
- The repository's process requires something you cannot supply — a marker, a
  template, an approval record. Say what is missing rather than approximating
  the process.

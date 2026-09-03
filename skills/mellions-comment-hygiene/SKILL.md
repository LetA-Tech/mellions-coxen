---
name: mellions-comment-hygiene
description: Load this whenever writing, modifying, reviewing, auditing or refactoring code, and whenever a comment is being relied on to establish what code does — a comment is an untrusted claim, never evidence of behaviour, completion or safety. Triggers — "clean up the comments", "comment hygiene", "are these comments accurate", "the comment says X", "remove stale comments", "should this comment stay", "document this function", "why is this commented out", and any review or audit where a comment is offered as proof that code is correct, complete or removed. Governs in-code comments only: not user-facing documentation, not whether the code is correct, and never removes logic.
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Coding Hygiene: Comments

## Authority

The executable implementation is authoritative. Comments are untrusted until verified.

Verify any behavioral claim against the code and the relevant configuration or
tests before relying on it. Never report a comment's claim as fact, and never cite
a comment as proof that a path exists, is complete, is removed, or is safe.

## Allowed content

An ordinary comment states what is true of the code now: current behavior not
apparent from the code, an invariant, a constraint, an API or symbol contract,
intent not visible in the implementation, a compatibility requirement, non-obvious
technical reasoning, or a safety, security, concurrency, or data-integrity property.

Present tense. Maximum three lines; one is usually enough.

Exported-symbol documentation comments are allowed when they state the current
public contract. Same rules apply, including the three-line limit unless the
documentation tooling requires another format.

## Prohibited content

Never write, and remove on sight:

- agent or execution logs, and instructions addressed to a future agent;
- implementation or migration history;
- issue, ticket, or remediation narratives;
- temporary plans, TODO, FIXME, HACK;
- speculation ("probably", "seems", "might", "we think");
- completion or status claims ("fixed", "fully removed", "implemented by");
- stale or misleading statements contradicted by the code;
- narration of the adjacent line or control structure;
- commented-out code — version control already holds it;
- comments that exist only because the code is unclear.

**Historical identifiers.** An ordinary comment carries no bug ID, ticket ID, lane
ID, PR number, remediation label, agent name, or implementation date. If such a
comment also states a valid current constraint, drop the identifier and keep the
verified constraint.

The identifier leaves the code — it does not move into a test name, symbol name, or
another comment. Bug IDs close; the code carries the current logic and the comment
aligns to that. Provenance lives in the issue tracker, the PR, and commit history.
This does not apply to an identifier that is genuinely required machine-readable
syntax.

Use `references/comment-standard.md` to determine what a given thing is before
applying these rules.

## Action order

1. Verify against the implementation.
2. Delete when the comment is unnecessary, obvious, redundant, or history-only.
3. Rewrite only when necessary current information remains after the history is stripped.

Deletion is the default; rewriting is the exception, and both follow from the code
you verified, not from how the comment is worded.

```go
// Before — history, no current information beyond the constraint
// Added because issue #431 reported duplicate messages in production.

// After — the constraint, verified and kept
// Reject an existing dispatch key to preserve single execution per message.
```

```go
// Delete — narrates the next line
// Increment the retry count.
retryCount++
```

## Comments do not compensate for unclear code

In normal development, fix the code instead of explaining it: rename, decompose,
extract a named constant, introduce a type, add an invariant check or a test. A
comment that translates confusing code into English marks a code defect, not a
documentation gap.

## Logic changes carry their comments

When logic changes, review every related comment and structured annotation, and
update or remove it in the same change. A change is incomplete while an adjacent
comment describes the previous implementation.

## Structured annotations

Machine-readable directives and framework annotations are not ordinary comments,
and the rules above do not apply to them. Examples: `//go:build`, `//go:generate`,
`//go:embed`, `//nolint`, `// @Summary` / `// @Router` and other generator fields,
compiler pragmas, ORM and serializer metadata, code-generation directives,
generated-file markers, approved legal headers.

- Identify the consuming tool before changing one.
- Preserve the required syntax exactly.
- Update it when the related implementation changes.
- Remove it only when verified obsolete.

A marker counts as structured metadata only when an active tool consumes it. A
custom or project-local marker is not protected merely because it looks
standardized — find the consumer or treat it as prose.

Suppressions: use the narrowest rule identifier and the narrowest scope, give the
current reason where the syntax supports one, and never leave an unexplained
blanket suppression.

```go
//nolint:gosec // Value is a validated local development fixture path.
```

Native language metadata — struct tags, decorators, attributes, annotations — is not
a comment and is outside comment classification entirely.

## Comment-only cleanup

When the task is comment cleanup:

- do not change production behavior, logic, or structure;
- delete or correct comments only;
- preserve machine-readable annotations unless verified obsolete;
- fix generated comments at the generator or template, never in generated output.

Anything beyond this — scope, validation commands, reporting — comes from the
cleanup assignment, not from this skill.

## Decision checklist

Before writing a comment:

1. Is it required machine-readable syntax? → preserve the exact format.
2. Is it necessary, and is the statement verified?
3. Does it describe only current non-obvious behavior, constraint, contract, or reasoning?
4. Is it three lines or fewer?

## Improving this skill

A gap found while running this skill is fixed here, in this bundle. Carrying the
method is the point: one that cannot improve from the work it performs is a
snapshot of how engineering was done once.

Improve it when the lesson is a better method that would hold in any repository.
When the lesson is only true of one repository, it belongs in that repository, as
a test or a check where it binds every engineer who works there. `mellions-self-learning`
is how that call is made.

**A skill is not a log.** No changelogs, version histories, dated entries,
session narration or agent commentary. A skill carries instructions, conditions
and constraints — nothing else. A change replaces the text it supersedes; git
carries the history. Instruction length follows the work; assets stay lean,
around 200 lines as a guide rather than a cap enforced by deleting a rule.

## Files in this skill

- `SKILL.md` — this file.
- `references/comment-standard.md` — the standard this skill applies.

---
name: mellions-issue-creation
description: Load this when a confirmed finding, an audit result or a piece of work is about to become an issue on the tracker — before filing anything, to check the finding is established at the base branch now, that no issue, pull request or claim already covers it, and to write the work contract in the taxonomy this Skill's assets carry. Triggers — "open an issue for", "file this", "turn the finding into an issue", "create a GitHub issue", "draft the issue", "should this be an issue". Not for deciding whether a defect is real (mellions-bug-audit) or for planning the fix (mellions-issue-resolution-proposal).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Filing an issue

An issue is the **work contract**: what to solve, where, why, which branch is
truth, which paths are involved, what evidence and tests are required, which
constraints cannot be violated, and how it closes. It is never proof of
anything — a created issue is not completed work, and a filed hypothesis is a
contract nobody can satisfy.

## The repository's contract comes first

Honor the repository's mandated issue schema, approval records and acceptance
requirements. A shared template never overrides an explicit repository rule.
Where none is mandated, the body is the title, the problem, the acceptance test
and a link to the proof; the assets beside this Skill hold the longer form:

- `assets/github-issue-template.md` — title, body and material evidence detail.
- `assets/github-issue-taxonomy.md` — labels, evidence tiers and decision work.

Read them before drafting; reuse that understanding while they remain unchanged.
Do not reconstruct their requirements from memory or add empty sections to make
an issue look complete. Missing or unreadable: say which and obtain it.

## Before filing

- **The finding is established, not inferred.** Reuse verified source analysis
  under `mellions-deep-research`'s input-validity rule; inspect relevant changes
  at the current base and settle unresolved claims. Correct drift before filing.
  Settle emitted line numbers with
  `mellions cite check -file <body> -dir <checkout> [-commit <ref>]`, which
  reports citations the body does not quote. A resolving citation alone does not
  establish the claim about it.
- **The base branch is established, not guessed.** From the repository's own
  binding or `CLAUDE.md` where it declares one. Where nothing declares one the
  remote's default is read instead and said to be discovered — that is a fact
  about the repository, not somebody stating what work should target, and in
  this estate the two differ: the default is the release branch and the
  implementation target is `dev`. Confirm it before pinning to it. A branch
  that cannot be established stops the work: an issue pinned to the wrong tree
  sends every reader to the wrong tree.
- **Nobody has it already.** Search the tracker for the symptom, the path and
  the identifier (`gh issue list --state all --search`), and the open pull
  requests. A duplicate is a comment on the existing issue, not a second one;
  a claimed issue (`mellions:claimed`) belongs to the lane that holds it.
- **One root cause per issue.** Consolidate the symptoms of one defect; never
  merge independently remediable problems into one, and never split one
  mechanism into several.

## The title

```text
[Mellions] - <short action-oriented title>
```

The word, one space, one hyphen, one space, then what the issue is about —
action and place, short. Nothing else goes in the brackets: not the type, not
the repository, not the severity. Those are labels, and an issue that carries
them twice is one of them going stale.

## The labels

Read `gh label list --repo <repo>` **before** choosing, not after a rejected
`gh issue create`. Apply the labels the registry carries that fit; invent
none, and a family it lacks is no blocker.

`risk:` is read off the taxonomy's matrix for the `probability:` and `impact:`
chosen independently. Nothing downstream re-checks it, so it is right here or
it is never right.

## Filing

Write the body to scratch and file with `--body-file`; never write a draft
into the repository. A section with no source content stays absent and the
gap is said — nothing is invented to fill a slot. The code contradicting the
finding is a discrepancy to report, not something to write around. Without
tracker access, the draft is handed over with the pin, not filed later from
memory. Report the URL; the issue is where the work now lives, and
`mellions-issue-resolution-proposal` is what goes on it next.

## Stop points

- Either asset is missing or unreadable.
- The base branch cannot be established.
- The finding is a hypothesis: it has not been reproduced or read in the code
  at the pin. Establish it first, or put the question on an existing issue.
- An issue, a pull request or a claim already covers it.
- The repository's convention requires something you cannot supply — an
  approval, a registry entry. Say what is missing rather than approximating.

---
name: mellions-issue-creation
description: Load this when a confirmed finding, an audit result or a piece of work is about to become an issue on the tracker — before filing anything: to check the finding is established at the base branch now, that no issue, pull request or claim already covers it, and to write the work contract in the taxonomy this Skill's assets carry. Triggers — "open an issue for", "file this", "turn the finding into an issue", "create a GitHub issue", "draft the issue", "should this be an issue". Not for deciding whether a defect is real (mellions-bug-audit) or for planning the fix (mellions-issue-resolution-proposal).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Filing an issue

An issue is the **work contract**: what to solve, where, why, which branch is
truth, which paths are involved, what evidence and tests are required, which
constraints cannot be violated, and how it closes. It is never proof of
anything — a created issue is not completed work, and a filed hypothesis is a
contract nobody can satisfy.

## Read the assets before the first line of the body

Two files, both beside this one, both read in full before drafting:

- `assets/github-issue-template.md` — the title format, the eleven body
  sections in their fixed order, the label block, the banned list.
- `assets/github-issue-taxonomy.md` — the label vocabulary and what each
  family means, the evidence tiers, the profiles for work that has no
  implementation, and what to do where a registry does not carry a family.

The body is eleven numbered sections in this order, so that a missing one is
visible from here:

```text
1 Defect · 2 Root cause · 3 Evidence · 4 Call and data flow · 5 Code locations
6 Runtime conditions · 7 Failure, recovery, concurrency and latency
8 Resolution · 9 Affected components, risks and dependencies
10 Test and verification plan · 11 Acceptance criteria
```

**What goes in each is deliberately not here.** An issue written from memory
of this page reproduces the shape without the content that made the shape
worth having — the tiered evidence, the ASCII call graph, the code inventory,
the checkable criteria — and a second copy of the definitions on this page
drifts from the asset within the day. Neither file is loaded into the session
with this one; open them with Read. Missing or unreadable: stop, and say
which, rather than improvising a format.

Where the defect is not in a code path at all — a prompt, a document, a
configuration string — sections 4 to 7 have no source content and are absent,
with the reason said in §2. That is not one of the taxonomy's two profiles and
does not license skipping them on an implementation issue.

## Where a repository installs its own issue canon

Some repositories carry an issue-creation Skill of their own under
`.claude/skills/`, with a taxonomy asset beside it; most carry none. Deferring
to a convention a repository does not actually declare is how an issue ends up
with ad-hoc headings and no labels, so establish which exists before deciding:

```bash
ls .claude/skills/*issue-creation*/assets/ 2>/dev/null
```

Nothing there — the assets beside this Skill govern, and that is the common
case. Something there, and not this Skill's own directory — the rule is
version, not location. Both taxonomies state a version line; read it with
`grep -m1 Status:` rather than by line number, since the lineages carry it at
different depths. The installed one governs the body **only where its version
is strictly greater** than the carried one's; equal or older or absent, the
carried assets govern. A tie is not a reason to defer: the copy in front of
you is the one whose adaptations you can see.

Either way the repository still supplies its own facts — the base branch, the
label registry, its `CLAUDE.md`. Structure comes from whichever taxonomy won;
values always come from the repository. And whichever won, the carried
taxonomy's §5 rule for a family the registry does not carry still applies —
it is a statement about what is possible in that repository, not a preference
between taxonomies.

## Before filing

- **The finding is established, not inferred.** Re-open every citation at the
  base branch HEAD now; the code wins over the finding's account of it. What
  drifted is corrected before it is written; a claim you have not read the
  code for does not go in.
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
`gh issue create`. The taxonomy's ten families are the target; the registry is
what exists. Apply every family the registry carries, use its own value where
it names one this taxonomy does not, and record a family it lacks in the one
`Labels unavailable` line the taxonomy permits. Invent no label, and drop no
family silently — taxonomy §5 is the whole rule.

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

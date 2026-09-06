---
name: mellions-issue-closure
description: Load this when work on an issue is finished and the question is whether the issue may close — after a merge, after a deploy, when an issue's defect turns out to be already gone at HEAD, or when a survey shows resolved work still open. It decides which event the issue actually closes on, what proof must be on the record first, and whether closing is yours or the owner's. Triggers — "close issue NN", "can we close this", "is this done", "post the proof", "the PR merged, close it", "this was fixed elsewhere". Not for deciding whether a defect is real (mellions-bug-audit) or for the implementation (mellions-issue-remediation).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Closing an issue

One dangerous failure here is closing on somebody's word — the implementer's,
a comment's, a green merge — with the proof local, the acceptance rule unread,
or the authority assumed.

## Four events, never conflated

```
feature branch → base branch  implementation merged
base branch → release branch  promoted
release → running system      deployed
issue closed                  accepted
```

None of the first three implies the fourth. **Merging a pull request does not
close its issue.** The issue is the audit trail and the pull request is one
artifact on it, so the durable record is the proof comment — exactly what an
auto-closing `Closes #NN` skips, and why a pull request body says `Refs`
(`mellions-issue-remediation`). Arriving here to find the issue already closed
by a merge means the proof was lost, not that the work is done: post it anyway
and say what closed it, unless that body says the diff was the whole proof.

## The acceptance rule decides

Read the issue's acceptance section — the whole sentence, not the first token —
and honour the strictest rule it states. Rules compose: `merge` + `deployment`
closes on the deploy, and a scan stopping at the first backticked word reads
every compound rule as its weakest half, closing on an event that has not
happened.

| Rule | Closes when |
|---|---|
| `merge` | the implementation is merged into the base branch |
| `promotion` | the change is on the release branch |
| `deployment` | the running image contains the change |
| `migration` | the named migration is applied, verified by query |
| `recompute` | affected projections are rebuilt and reconciled |
| `runtime-proof` | a named metric or query is observed correct in production |

Where the issue states nothing: post the proof comment, then close under
explicit authorization. Silence never escalates to closure on its own, and
never blocks a documentation fix behind a deployment gate either. Where the
repository declares its own closure policy (`issue-defined`, for instance),
that policy is the rule.

For Mellions' own repository the running system is the checkout `mellions
doctor` names as the load path: it closes when that checkout stands at the
merge and, for a binary change, the installed binary matches — quote the
doctor line, never the merge alone.

## A blocker is not an endpoint

An exact account of a blocker you could remove is not a resolution. Where
engineering on the fix removes what holds the acceptance event off, that is
this issue's next task, not its resting state; the event itself — a deploy, a
migration, an occurrence — is waited for or asked for with what you tried, and
never run to make an issue closeable. Never close on an unmet acceptance,
never leave one open on an untried path.

## Identity

The issue number is the identifier; a finding id is a discovery key. Resolve
it to exactly one issue (`gh issue list --state all --search`) and carry the
number; zero or several matches is a stop. An issue with no finding id is
normal; invent nothing to fill the slot.

## Whose decision it is

Closing an issue the owner opened is theirs unless the partnership delegated
it; merging follows the partnership the same way. Read it rather than assume.

- **Delegated:** close it. Everything below about proof still applies in full
  — what was delegated is the authority to act on a finished, proven result,
  never the standard it has to meet. Hesitating at a gate somebody handed you
  is not caution.
- **Reserved:** leave the close package where they read — on the issue: what
  was established, the proof, the acceptance rule and how it is met, the
  exact command (`gh issue close NN --repo <owner/repo> --comment ...`) — and
  stop until they say the word; then run it. A written package is a successful
  outcome. A one-off approval names an action: one given for the merge is not
  one for the close.

A repository process requiring an approval record, a marker or an independent
review before closure governs; this method is what has to be true underneath.

## Proof on the record before the close

Nothing is closed on a local state. Verify at the remote: the commit SHA on
the base branch at origin, the pull request's merge state and merge commit,
CI on the merge, and the acceptance rule's own evidence — the running image
for `deployment`, the query result for `migration`, the observed series for
`runtime-proof`. Quote raw output; never write a SHA, a PR number or a test
result you have not seen at the remote.

The proof comment carries, only where it applies: the issue, the branch,
the commit SHA and the pull request; what changed — files and diff stat; raw
build and test output with the falsification runs; each acceptance criterion
mapped to met or unmet with its evidence; the acceptance rule and the event
that satisfied it; residual risks and follow-ups, named, and filed where they
are their own work, since one never survives as a sentence here; and, where
closure was the owner's, who authorized it.
Post it, then close — never the other order. By default the session that wrote
the plan and code does not accept its own proof: a fresh session or subagent is
cheap, and where none read it the comment says so. Where the change
touched money, persisted state, a schema, a service or repository boundary,
deployment or credentials that latitude ends — the independent reading comes
first, on the requirement before the diff, its verdict in the comment, and
self-acceptance is not available at all.

## Resolved by the world

Some issues are closed by events nobody recorded: the defect is gone at HEAD
because another change removed it, or the acceptance event happened without a
proof. Verify to the depth a close requires — the cited defect absent in the
code at the pin, the tests exercising the path non-vacuously, the acceptance
rule's event confirmed — and then close or package exactly as above. The
issue's premise being stale is a reason to read the current code, never proof
the work is done; three found resolved are three issues, not a tracker
overstating its open work.

## Stop points

- The proof exists only locally, or the SHA, PR or CI state cannot be seen at
  the remote.
- The acceptance rule is not satisfied: merged but deployment-gated, applied
  but not verified by query, observed nowhere — a stop on closing, not on the
  work.
- The proof contradicts the issue body, or the issue, branch or pull request
  do not match each other.
- Raw output or the acceptance mapping is missing; a criterion is unmet.
- CI is failing with no explicit waiver; the pull request targets the wrong
  branch; several issues would close on one proof without authorization.
- Closing is the owner's and no word has been given: the package is the
  finished state.

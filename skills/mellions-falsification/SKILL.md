---
name: mellions-falsification
description: Load this before a test, a mutation, a revert arm or a green run is cited as proof that a fix holds — and again before writing "falsified" in a pull request. Triggers — "prove it holds", "falsify", "neutralise the fix", "watch it fail", "the test passes", "is the test real", "mutation", "revert arm", "the suite is green", "proof section". Not for deciding what to do with the result (mellions-reasoning) or for establishing what is true in the first place (mellions-deep-research).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Establishing that a fix holds

A green run is a claim about the harness until something shows it could have
been red.

## The copy you falsify in

One copy per arm: a neutralisation left from the previous arm can mask the
next. `git archive <rev>` carries that commit, not the index: a staged,
uncommitted test is absent.
Never falsify by restoring a working tree: it discards uncommitted work —
yours or another session's. With one arm and nothing
uncommitted at stake, toggling the edit in place — red, then green — is the
arm.

## The oracle

Where a test asserts the output equals what the mechanism under test computes,
it has no independent side: whatever moves the mechanism moves both halves,
and the test stays green through the defect it was written for. Compare
against something the mechanism cannot reach — a literal, a figure from the
requirement, a golden file, a reference implementation, an invariant, a value
read across the boundary the fix crosses — never a call into the code under
test.

## The revert arms

Revert every part of the fix at once and watch the named tests fail. A complete
revert that stays green has four readings, the third pointing at deleting the
test: the edit did not land — confirm it where the test reads (off disk, in the
database), since an edit that no-ops (a `sed` that matched nothing, an
unexpanded variable) reads exactly like a pass; the tests never reach the fix;
the fix is idle; or a part of the same revert removed the condition the test
needs — where one half of a fix creates the state the other half's defect lives
in, each half wants its own arm. Revert half of a fix on its own and the half
left standing can hold the test green, proving nothing about either.

Read the red: a wide revert can fail short of the assertion — a build break, a
panic in a helper — or trip it by a mechanism you did not test. The red that
counts is the assertion the fix exists for, by name.

Where the fix is an instruction aimed at a model — a Skill, a prompt, a hook —
read `references/model-arms.md` first: the untreated run must be able to fail,
and what counts as a difference is decided before either is read. Where
something outside the process settles a race, read `references/race-arms.md`:
a one-process test never meets what breaks it.

Where the fix deletes an exemption a check honoured — a `nolint`, a skip, an
allow-list entry — the complete revert restores the exemption and is green on
purpose. Keep it gone and revert only
the code that now satisfies the check: it must go red.

With more than one arm, read which named tests went red under which arm. An
arm that must be green and reds condemns the batch, not itself:
the cause is usually shared — a column every insert omits, a header every
request lacks — so no red beside it counts until the accepted case is green.
A test cited as this fix's proof that is red under no arm is not evidence
for it. An arm that reds nothing neutralised nothing a
test can see: the neutralisation did not land, the tests did not run (a
skip, a build tag, a `-run` filter, a cache that did not fingerprint what you
changed), the code is dead, the test is missing, or another path still
supplies what you removed — a second grant, an inherited role, PUBLIC,
ownership, a fallback. Read what production holds, from its catalog or live
grants: if it has that path, this test cannot be evidence for the fix; if
only the test setup has it, correct the setup in the tree you publish, then
rerun the plain arm. An arm that neutralises an
optimisation legitimately reds nothing; a timing assertion added to red it is
worse than none.

## What a red or a green can still hide

A status belongs to the last thing that produced it, and a green can be a
verdict nobody computed this time.

A test that expects a failure is green on whatever stopped the path first — a
missing table, a refused permission. Read which failure the run got; if it is
not the one the test exists for, fix the precondition, then assert that one: a
sentinel or typed error, else a code with the object it names. A test that
expects nothing to happen needs a positive control in the same setup — an
input that must act, acting — or a subscriber never wired passes it.

A control that inherits the suspected cause settles nothing: rerunning a
failure in a second copy with the same PATH, environment, working
directory or cache reproduces the environment, not the tree, and reads exactly
like "it fails at base too". Vary the one thing the claim is about.

A pipeline's exit is its last stage's (`make check | tail` reports `tail`
without `pipefail`), and a backgrounded command's can be the launcher's, so
"completed, exit 0" may attest only that the spawn succeeded. Read
the check's own output: a non-zero exit falsifies a green claim on its own, a
zero exit never establishes one.

Mutation proves the tests see the change, not that the change reaches the
outcome. Where a caller the tests never execute mediates the effect
— a pure function and its call site, a resolver and the script that reads it —
drive the entry point once at whatever fidelity is reachable and assert on
the outcome the requirement names, not the unit's return value.

Where the fix widens what something accepts — a matcher, a filter, a guard, an
allow-list — the revert arm shows what it now admits, never what it stopped
catching. Enumerate the cases the narrow form caught and the wide one lets
through, remove only the clause meant to hold them back, and watch those go red.
A widening whose guard reds nothing is not guarded.

Where the claim is placement — a write inside another operation's
transaction, lock or publish order — removing the write proves the write, not
its place: read `references/placement-arms.md`.

Where the fix is to persisted state — a store, a table, a cache, a file — a
reading taken the moment a new binary is in place measures state nothing has
touched: drive the write path the system drives, then read.

Where what you measure is emitted once — a note said once per session, an
at-most-once delivery, a lock taken by whoever asks first — the measurement is a
consumable: a consumer you did not account for takes it, and your instrument
records nothing, exactly like an arm producing nothing. Read the durable record
the emission leaves (the ledger, the offset, the holder), not the stream
somebody else may have drained; a second reader on that stream is the same
defect, now yours.


## Writing it down

A failure location from a neutralised tree cites a tree that no longer exists:
re-derive line numbers against the branch you publish from. The proof section
names each arm, the copy it ran in, the tests that went red and the ones that
stayed green, and the arm that was not run.
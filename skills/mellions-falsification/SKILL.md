---
name: mellions-falsification
description: Load this before a test, a mutation, a revert arm or a green run is cited as proof that a fix holds — and again before writing "falsified" in a pull request. Triggers — "prove it holds", "falsify", "neutralise the fix", "watch it fail", "the test passes", "is the test real", "mutation", "revert arm", "the suite is green", "proof section".
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Establishing that a fix holds

A green run is a claim about the harness until something shows it could have
been red.

## The copy you falsify in

One copy per arm: a neutralisation left from the previous arm can mask the
next, which passes while testing nothing. `git archive <rev>` carries that
commit, not the index—a test staged but uncommitted is as absent as an
untracked one. Never falsify by restoring a working tree: it discards
uncommitted work, yours or another session's in a shared tree. With one arm and
nothing uncommitted at stake, toggling the edit in place—red, then green—is
the arm.

## The oracle

Where a test asserts output equals what the mechanism computes, it has no
independent side: whatever moves the mechanism moves both halves; the test
stays green through the defect it exists for. Replace it with something the
mechanism cannot reach—a literal, a figure from the requirement, a value from
across the boundary the fix crosses. A golden file, a reference implementation
or an invariant is an independent side; a call into the code under test is not.

## The revert arms

Revert every part of the fix at once; watch the named tests fail. A complete
revert staying green has four readings: the edit did not land—confirm off
disk, as an edit that no-ops (a `sed` that matched nothing, an unexpanded
variable) reads exactly like a real pass; the tests never reach the fix; the
fix is idle, so delete the test; or part of the same revert removed the
condition the test needs—where one half of a fix creates the state the other
half's defect lives in, each half wants its own arm. Revert half of a fix on
its own and the half left standing can hold the test green, proving nothing
about either.

Read the red: a wide revert can fail short of the assertion—a build break, a
panic in a helper—or trip it by a mechanism you did not test. The red that
counts is the assertion the fix exists for, by name.

Where the fix deletes an exemption a check honoured—a `nolint`, a skip, an
allow-list entry—the complete revert restores it and is green on purpose.
Keep the exemption gone, revert only the code now satisfying the check: it must
go red.

With two or more arms, read which named tests reddened under which arm. An arm
that must be green and reds condemns the batch, not itself: the cause is
usually shared—a column every insert omits, a header every request lacks—no
red beside it counts until the accepted case is green. A test cited as proof
that reds under no arm is not evidence for it. An arm that reds nothing
neutralised nothing a test can see: the neutralisation did not land, the tests
did not run (a skip, a build tag, a `-run` filter, or a cache blind to what you
changed—inputs outside the tool's fingerprint replay the last verdict), the
code is dead, or the test is missing—except an arm neutralising an
optimisation, whose zero is legitimate; a timing assertion added to red it is a
worse test than none.

## What a red or a green can still hide

A status belongs to the last thing that produced it; a green can be a verdict
nobody computed this time.

A control inheriting the suspected cause settles nothing: rerunning a failure
in a second copy with the same PATH, environment, working directory or cache
reproduces the environment, not the tree, and reads exactly like "it fails at
base too". Vary the one thing the claim is about.

A pipeline's exit is its last stage's—`make check | tail` reports `tail`
unless `pipefail` is set—and a backgrounded command's can be the launcher's,
so "completed, exit 0" may attest only the spawn's success. Read the check's
own output: a non-zero exit falsifies a green claim on its own, a zero exit
never establishes one.

Mutation proves the tests see the change, not that it reaches the outcome.
Where the effect is mediated by a caller the tests never execute—a pure
function and its call site, a resolver and the script reading it—drive the
entry point once at reachable fidelity (usually a stubbed process boundary),
assert on the outcome the requirement names, not the unit's return value.

Where the fix widens what something accepts—a matcher, a filter, a guard, an
allow-list—the revert arm shows what the change now admits, never what it
stopped catching. Enumerate what the narrow form caught and the wide one lets
through, remove only the clause meant to hold them back, watch those go red. A
widening whose guard reds nothing is not guarded.

Where the claim is that something other than this process settles a race—a
lock, a reservation, a unique constraint, an idempotency key—a test in one
process establishes what the operation does, not the invariant: what would
break it was never there. Run the arm with two real processes on one store.
Where no seam makes them deterministic, N concurrent processes in a tight loop
asserting N survivors is probabilistic and still not vacuous—the same loop on
the unfixed tree must lose some, or N survivors is what a loop that never
collided also gives.

Where the claim is placement—a write inside another operation's transaction,
lock or publish order—removing the write proves the write, not its place.
Displace it one step outside the boundary, read what only an escaped write
leaves when the outer operation aborts *after* it: a counter that moved, a row
outliving a rollback. Drive the outer operation as production does: a
transaction the test opens around it rolls the escaped write back too; the arm
stays green with the property gone.

Where the fix is to persisted state—a store, a table, a cache, a file—read
it after the system's next write: a reading taken the moment a new binary lands
measures state nothing has touched. Drive the write path the system drives,
read again, drive it again.

Where what you measure is emitted once—a note said once per session, an
at-most-once delivery, a lock taken by whoever asks first—the measurement is
a consumable: a consumer you did not account for takes it; your instrument
records nothing, exactly like the arm producing nothing. Read the durable
record the emission leaves (the ledger of what was said, the offset, the
holder), not the stream somebody else may have drained; a second reader on it
is the same defect, now yours.

Where the fix is an instruction or refusal aimed at a model—a Skill, a
prompt, a hook—run the arm without the edit too: the model does much of the
work unprompted; one treated run credits the change with the model's
competence. The case both arms run must be one the untreated arm can fail, so
neither state the conclusion the change exists to produce nor hand the command
cold, which an unpressured model declines anyway; build it from the conditions
the change will meet—the reason accepted, the objections answered, the work
under way—unless being handed it cold is the condition, and the untreated arm
must still be shown failing. A null is a fact about the case, not the change.
Decide what counts as a difference before reading either arm; the claim is only
what the difference shows.

## Writing it down

A failure location from a neutralised tree—a panic frame, a `t.Errorf` line —
cites a tree that no longer exists: re-derive line numbers against the branch
you publish from. The proof section names each arm, the copy it ran in, the
tests that went red and the ones that stayed green—and the arm not run, said
rather than implied.

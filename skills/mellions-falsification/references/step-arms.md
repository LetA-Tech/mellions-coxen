<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Arming a fix of more than one step

Part of `mellions-falsification`, read when the fix's own account of why it is
correct names more than one step: an ordering, a re-check after taking a lock,
a second guard, a constraint and the writer that relies on it.

## A green complete revert

Where one half of a fix creates the state the other half's defect lives in,
each half wants its own arm. Revert one half alone and the other can hold the
test green, proving neither.

## A red complete revert

A red complete revert proves the old code fails, not that the new code is
held. A test built around the old failing schedule reds on the old path and
can stay green while one step the new code depends on is undone alone.
Neutralise each step alone with the rest in place. To neutralise an ordering,
restore the old order rather than deleting the step. The account to take
steps from is the diff, not its comments, which are claims.

An arm that reds at the assertion the fix exists for holds its step. An arm
that stays green has two readings: the step is unheld, or it is not needed.
A run outside the suite separates them: a scratch test that forces the window
(park one goroutine on the lock the step depends on, read the shared state
there), a stress loop, the race detector. If the defect comes back, the step
wants the test that reds it, kept in the tree and armed the same way. If it
does not, the step or its account is wrong. Say which in the proof.

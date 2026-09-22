<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Arming a fix that settles a race

Part of `mellions-falsification`, read when something outside the process under test is what settles the race.

Where the claim is that something outside this process settles a race — a
lock, a reservation, a unique constraint, an idempotency key — a one-process
test shows what the operation does, not the invariant, because the thing that
would break it was never there. Run the arm with two real
processes against one store. Where no seam makes them deterministic, N
concurrent processes in a
tight loop asserting N survivors is probabilistic, not vacuous — and
the same loop against the unfixed tree must lose some, or N survivors is what
a loop that never collided also gives.

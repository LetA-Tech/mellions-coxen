<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Citing and arming a test a race decides

Part of `mellions-falsification`, read when a test's outcome turns on goroutine
scheduling, a lock, a timer or another process.

## One run is a sample

Where which goroutine takes a lock, which timer fires first or when a dependency
lets a call return decides whether the test passes, one green — the gate's
`-count=1` included — is one draw, and so is one red under an arm. A test that
fails one run in p passes n runs with probability (1-1/p)^n: about 3p runs show
it once at 95%. Size the count from the rate you must rule out — 300 for one in
a hundred — run the named tests under the race detector across CPU counts,
`go test -race -run '^TestName$' -count=300 -cpu=1,4 ./pkg/`, and cite the
tally. An arm or a goroutine dump that shows the path parked on a mutex, a
condition variable or a channel puts every test asserting that path's outcome
in this class.

## Assert what the dependency guarantees

Where the assertion is an order the code under test does not control — that a
forced stop returns before a handler finishes, that one waiter wins a mutex
after a broadcast, that a goroutine has run by the time a sleep ends — no count
makes it true, and a retry or a longer timeout keeps the false claim at a lower
failure rate. Read what the dependency's contract guarantees and assert only
that — a bound the code under test enforces, both effects happening, a result
the contract allows; wait on a signal the code sends, never a sleep. Where the
requirement itself needs the order the dependency does not guarantee, the fix
is what is wrong, not the test.

## Where one run is right

Where a seam the test owns — a channel, a barrier, a fake clock, a stub that
blocks until released — orders the two events the assertion compares, the
outcome no longer turns on scheduling and one run is a verdict; repeats only
show the race detector more interleavings. A test with no concurrency in it is
the same. Neither excuses the revert arm: without the fix, the assertion must
go red.

## A race another process settles

Where the claim is that something outside this process settles a race — a
lock, a reservation, a unique constraint, an idempotency key — a one-process
test shows what the operation does, not the invariant, because the thing that
would break it was never there. Run the arm with two real processes against one
store. Where no seam makes them deterministic, N concurrent processes in a
tight loop asserting N survivors is probabilistic, not vacuous — and the same
loop against the unfixed tree must lose some, or N survivors is what a loop
that never collided also gives.

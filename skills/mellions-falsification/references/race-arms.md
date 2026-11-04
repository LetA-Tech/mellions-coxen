<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Citing and arming a test a race decides

Part of `mellions-falsification`, read when a test's outcome turns on goroutine
scheduling, a lock, a timer or another process.

## One run is a sample

Where which goroutine takes a lock, which timer fires first or when a dependency
lets a call return decides whether the test passes, one green — the gate's
`-count=1` included — is one draw. A test that fails one run in p passes n runs
with probability (1-1/p)^n: a single green says almost nothing, and about 3p runs
show the failure once at 95%. Before citing it, run the named tests alone under
the race detector at a stated count —
`go test -race -run '^TestName$' -count=50 ./pkg/` — and cite the tally, 50/50.
A red under a revert arm from such a test is a draw too: it counts only against
a tally of greens at the fix. An arm or a goroutine dump that shows the path
parked on a mutex, a condition variable or a channel puts every test asserting
that path's outcome in this class.

## Assert what the dependency guarantees

Where the assertion is an order the code under test does not control — that a
forced stop returns before a handler finishes, that one waiter wins a mutex
after a broadcast — no count makes it true, and a retry or a longer timeout
keeps the false claim at a lower failure rate. Read what the dependency's
contract guarantees, then assert only that: it returns within the bound, both
effects happen, the result is one the contract allows. If nothing it
guarantees carries the claim, the test goes.

## Where one run is right

Where the test fixes the order through a seam it owns — a channel, a barrier, a
fake clock, a stub that blocks until released — the outcome no longer turns on
scheduling, and one run is a verdict; repeating it adds nothing. A test with no
concurrency in it is the same.

## A race another process settles

Where the claim is that something outside this process settles a race — a
lock, a reservation, a unique constraint, an idempotency key — a one-process
test shows what the operation does, not the invariant, because the thing that
would break it was never there. Run the arm with two real processes against one
store. Where no seam makes them deterministic, N concurrent processes in a
tight loop asserting N survivors is probabilistic, not vacuous — and the same
loop against the unfixed tree must lose some, or N survivors is what a loop
that never collided also gives.

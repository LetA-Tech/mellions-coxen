<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Arming a fix whose oracle is a shared key

Part of `mellions-falsification`, read when the oracle is whether two inputs share a key: idempotency, dedupe, cache.

Where the oracle is whether two inputs share a key, it sees only the fields
the pair varies. Read the producer: build the duplicate as it re-sends (same
identity, a fresh timestamp or attempt) and the distinct case as a new
identity with those held; assert the outcome (rows, backing calls); arm with a
key that takes in a re-stamped field.

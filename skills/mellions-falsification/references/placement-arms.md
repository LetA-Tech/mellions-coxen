<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Arming a fix whose claim is placement

Part of `mellions-falsification`, read when the claim is where a write sits relative to another operation's transaction, lock or publish order, or which source a read takes.

Where the claim is placement — a write inside another operation's
transaction, lock or publish order — removing the write proves the write, not
its place. Displace it one step outside the boundary and read what only an
escaped write leaves once the outer operation aborts *after* it: a counter
that moved, a row that outlived a rollback. Drive the outer operation as
production does; a transaction the test wraps around it rolls the escaped
write back too, and the arm stays green with the property gone.

Where the claim is a read's source — the remote ref rather than the checkout,
the tenant's row rather than the default, the variable that must win over the
file — removing the read proves the read, not its source, and a fixture that
holds one value everywhere keeps every wrong-source arm green. Give each source
the read could plausibly take instead a distinct value in the fixture, point
the read at each in turn, and watch each arm go red on the value assertion. A
read with one possible source needs no decoy.

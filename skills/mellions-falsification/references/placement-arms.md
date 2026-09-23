<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Arming a fix whose claim is placement

Part of `mellions-falsification`, read when the claim is where a write sits relative to another operation's transaction, lock or publish order.

Where the claim is placement — a write inside another operation's
transaction, lock or publish order — removing the write proves the write, not
its place. Displace it one step outside the boundary and read what only an
escaped write leaves once the outer operation aborts *after* it: a counter
that moved, a row that outlived a rollback. Drive the outer operation as
production does; a transaction the test wraps around it rolls the escaped
write back too, and the arm stays green with the property gone.

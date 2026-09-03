---
description: What needs attention across the estate, and what to do about it
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

Run `mellions survey` (or read the saved one the session-start hook named, if
it is fresh).

It collects and never ranks: open work, changes under review, failing checks,
work waiting on the owner, recent repository change, work already in flight,
follow-ups found during other work, and stale premises — open issues whose own
citations no longer match the tree. The grouping is for reading only.

Read it in two moves. **What was collected** is the shape of the estate: how much
of each kind, and which repository it sits in. The lists below it are one line
per signal — repository, identifier, age, title. Once you know which slice you
are choosing from, print that slice whole:

```
mellions survey -full -repos <repo> -kind <kind>
```

`-full` adds the labels, URLs, attributes and bodies the one-line form leaves
out; `-json` gives everything a machine can read.

Three ways this survey is bounded, and none of them means "nothing there":

- **INCOMPLETE** — a source did not answer. Unknown, never empty. Say so before
  reasoning from the rest.
- **"… N more collected in `repo`"** — a long list was printed short. The
  heading's count is what was collected, and the line names the command that
  prints the rest.
- **"came back exactly at [the limit]"** — a repository's list was truncated
  before this survey ever saw it. Those items are in no count on the page. Ask
  the tracker directly rather than reading that repository as nearly clear.

Then decide, against the program you carry: what actually needs attention here?
Not the oldest issue and not the easiest one — the thing whose absence costs the
most, given what the program says correctness means and what it says to reach
for when nothing is on fire. Useful work is not only issues: an unfinished
remediation, a review nobody closed, a verification that failed, a defect found
while doing something else, work that is already resolved and should be closed.

A stale premise is evidence that the issue's account of the code is no longer
true. It is a reason to read the current code, never proof the work is done.

Say what you considered, what you chose, why that is the most valuable thing
available now, and what you deliberately did not choose. Then claim it:

```
mellions assign open -id <id> -repo <repo> [-issue "#N"] -objective "..." \
  -because "..." -not-chosen "..."
```

and carry it through.

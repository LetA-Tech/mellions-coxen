---
description: 'Close out the current assignment: what stands, what is unresolved, what it would cost to finish'
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

Write the handoff for the current assignment. Four things, nothing else:

1. What you did, with links to the durable artifacts — commits, branch, PR, issue comments.
2. What you established that changes what the owner believes about the system.
3. What is blocked, and on whom.
4. What the next session should pick up, and why that rather than something else.

```
mellions assign handoff <id> -file -     # reads stdin
mellions assign close <id>               # when the work is finished; keeps the branch
mellions assign sweep [-apply]           # every handed-off lane whose pull request merged or closed
```

Put anything another person needs on the issue or the PR as well, not only
here: durable engineering truth belongs on the artifact it concerns.

If you stopped at a decision that is the owner's rather than finishing, say so
plainly and carry the decision package — what you established, what remains
uncertain, the viable alternatives with their consequences, your recommendation,
and the exact action you propose. Stopping there is a successful outcome.

Then the last question of finishing: what should change about how the next
piece of work is done? Usually nothing. When it is something, put it where it
binds — a test in the repository, a Skill, the program — and never in a note to
yourself. `mellions-self-learning` is the method.

---
description: The report the owner reads instead of replaying the session
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

Write the report over the work that happened.

```
mellions report write [-assignment <id>] \
  -needs-owner "..."   what genuinely needs them, with your recommendation — first, or nothing
  -established "..."   what changed about what they believe about the system
  -did "..."           what happened, referencing the durable artifacts
  -blocked "..."       what stopped, and on whom
  -next "..."          what to pick up next, and why that
mellions report latest [-n 3]
mellions report digest [-brief]   what needs the owner since it was last said
```

Four things decide whether it is any good:

**Lead with what needs them.** An owner opens this to find out whether anything
needs them. If nothing does, say so — silence is a valid outcome and it protects
the value of the reports they do read.

**Reference evidence; never reproduce it.** `PR #418`, `commit abc123`, `tests
PASS — 37 targeted + package suite`. The issue, the pull request and the commit
are the record; a report that copies them starts drifting the moment it is
written.

**Say what is missing.** A check not run, a repository not reachable, evidence
not collected — name it. Silence reads as "it passed".

**Scale with the work.** Half a page for a quiet run, more only where a decision
needs it. Do not narrate effort.

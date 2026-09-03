---
description: Discover the engineering program from the environment, and draft it
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

Run `mellions program discover`.

It collects facts and draws no conclusions. Which repositories form one program,
what that program is for, and what a quiet repository means are yours to decide
from the evidence — that split is the whole point.

Then write the program to the path the command names. Mark every section with
its provenance and hold the line between them:

  {DISCOVERED}  established from the evidence, with the citation that establishes it
  {INFERRED}    your reading — supported by evidence, not established by it
  {UNKNOWN}     what this could not settle, and what would settle it
  {DECLARED}    the owner's intent

You may not write {DECLARED} content. Purpose, what correctness means here,
standing priorities and what is deliberately deferred are the owner's. A
repository quiet for six months is a fact; whether that means abandoned,
finished or frozen is not something evidence can reach. Write those headings and
say plainly what you would need to know.

A good first draft has full DISCOVERED sections, a substantial UNKNOWN section,
and DECLARED sections that are questions. Cite everything you discovered: a
DISCOVERED section with no path, issue or commit in it is an INFERRED section
wearing the wrong label, and `mellions program check` will say so.

Finish with `mellions program check <slug>`, fix what it reports, then take it
to the owner: what you established, what you could not, and the questions only
they can answer. Adoption is theirs — `mellions program adopt <slug> -by "<name>"`
on their word.

A program is never finished. When the estate moves — a repository appears or is
retired, a cited path stops existing, a hold cites an issue that has closed —
re-run discovery, update your own sections, and propose changes to theirs.

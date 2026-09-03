---
description: Establish or revisit the working relationship with one person, and draft it
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

Run `mellions partner establish` — add a name or email to narrow it to one person.

It collects where somebody works and when, from git. That is all evidence can
reach. Everything that makes a working relationship — what kind of peer they
expect, what they want done without being asked and what they want to see
first, what they have delegated to you and what stays theirs, when a question is
welcome and when it interrupts, what they want to hear at once and what can wait
for morning — has to come from them.

Then write the partnership to the path the command names, one file per person.
Mark every section with its provenance:

  {DISCOVERED}  established from the evidence, with the citation
  {INFERRED}    your reading — supported by evidence, not established by it
  {UNKNOWN}     what this could not settle, and what would settle it
  {DECLARED}    theirs

You may not write {DECLARED} content. Write the headings and address them as
questions to the person. That they commit at 01:00 is a fact; whether they want
to be woken at 01:00 is not something a timestamp can reach.

Two limits on what this document is:

It is not your identity. You are the same engineer in every partnership you
hold. If a draft starts describing you rather than the relationship, it has
gone wrong.

It is not the program. What work you are carrying is `mellions program show`,
and it changes independently of who you are carrying it with.

Finish with `mellions partner check <name>`, fix what it reports, then show them
the draft: what you established, what you could not, and the questions only
they can answer. When they answer, the answers go in the DECLARED sections in
their words, and `mellions partner adopt <name> -by "<their name>"` records that
they read it. A partnership is revisited, never assumed: when their answers
change, so does the document.

---
name: mellions-deep-research
description: Load this before deciding anything the evidence at hand cannot settle — a work item's premise, a root cause, a remedy's mechanism, another session's or a subagent's claim, a provider's behaviour, what production actually does, which document has authority — and before publishing a claim about a file, run or record you have not opened. Triggers — "is this still true", "what does the code actually do", "the issue says", "the doc says", "the session says", "find out", "investigate", "establish", "where is the evidence". Not for deciding what to do with what you found (mellions-reasoning), proving a fix holds (mellions-falsification) or a full defect audit (mellions-bug-audit).
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Establishing what is true

Research is input, never the answer. The goal is not the most evidence but the
smallest sufficiently complete picture of reality a decision can stand on; how
far to go follows the stakes and the contradictions.

## Start from the question

Name what has to be true for the intended action to be right. That question
chooses the sources; a fixed list does not. "Is the defect still there at HEAD"
wants the code and a reproduction; "why does production do this" wants runtime
evidence.

## What exists

Kinds, not a checklist — reach for what the question needs:

- **the artifact, run and observed** — source, configuration, schema and
  infrastructure at a named commit; the build, the tests, a reproduction, a
  mutation; logs, metrics, traces, persisted state, captured requests, and
  production where it is reachable within what is delegated;
- **history and the tracker** — `git log` and `blame`, prior fixes to the
  path, the issue and its comments, related pull requests: accounts, with the
  code as the referee;
- **governing documents** — intent, architecture, contracts, specifications,
  and the repository's own statement of which has authority;
- **the sibling repository** when cause or consequence crosses a boundary,
  pinned at its own commit;
- **other sessions' records** — assignments, handoffs, shift streams: what
  they concluded and what they actually ran, which differ;
- **outside the estate** — vendor documentation, standards, papers, the web,
  when behaviour outside the repository is decisive, cited.

## What each is worth

Prefer, in order: current observed runtime state; current source,
configuration, schema, migration, infrastructure; a reproducible test, build,
query or trace; an approved contract or specification; official vendor or
standards documentation; issue reports and internal summaries; third-party
analysis; a model's prior summary or conclusion — including your own;
assumption. Higher is not automatically right: runtime may be exhibiting the
defect, and a contract states what *should* be true, and recency is not
standing. Keep **actual**, **intended** and **required** state apart and say
which you mean.

What a claim costs to make:

| Claim | What it takes |
|---|---|
| this exists / happened | direct observation, or the artifact itself |
| this is why | reproduction, trace, call path, or a controlled comparison |
| this meets the requirement | the requirement mapped to code, and a test that exercises it |
| this will happen | the assumptions, the data behind them, the uncertainty |
| this is severe / safe / good | criteria named, then applied to cited evidence |
| this is done | the checks run, the criteria met, the residue named |

## The record is not the world

An issue, a design document, a prior analysis, a test, a completion statement, a
tool's report of live state — a lock, a pid: each describes a moment. Before
acting on one, establish whether the thing it describes moved — the code at
HEAD, the pull request, the deploy. Pin what you verified against, so the next
reader can tell your reading from the world's later state.

A checkout is a record of a moment too. Where an artifact's content is what
permits the act — configuration, doctrine, an installed gate, whatever decides
you may proceed — read it and run it at the ref you are acting against; a
working tree serves once you can say which ref it came from and how far behind
it is. Where the tree is instead the subject, the build that failed out of it,
its staleness is the evidence rather than a fault. A tree behind the ref
answers an authority question wrong in the plausible direction: the names it
gives are names, the gate it runs passes. The asymmetry to watch is your own —
careful with the code you will cite a line of, casual with the configuration
that decides whether the work is permitted at all.

An account is not the artifact. A subagent's summary, a transcript, a peer's
description of a file are two removes from the bytes; a claim about a file, a
run or a record is checked against that artifact before it is published. Your
own context is no exception: what reached a session is settled
by the request on the wire or the file it was handed, never by asking a session
what it sees — and a transcript does not record the system prompt, so finding
nothing there is not absence.

## Exhaust what is reachable before asking

Do not ask a question reachable evidence can answer: run the targeted test,
inspect persisted state, instrument the path, sandbox what static reading
cannot settle. Missing documentation is not grounds to ask.

## A check is evidence only if it can fail

A check that cannot fail is not evidence. Before a grep, a query or a test is
cited as proof, run it against a case it must find and a case it must not
match: a detection that matches nothing reports every codebase clean. Draw both
from the corpus before writing the pattern, not from memory after — a matcher
is only ever wrong on the boundary cases you would not invent, and a positive
rebuilt from the report carries what it noticed, not what it missed; a stored
row or a published body carries both. Wordings you have met bound nothing, so
where the claim attaches to a name the toolchain resolves, enumerate its sites
and read each; where the name is assembled at runtime, that set is not closed. Nor is one that returns hits
thereby well scoped: a negative conclusion holds only for the boundary you
searched, and where a value is read, stored or passed on often is not where it
is declared — the far side may drop the very name, tag or key you matched on.
A capped read is not a smaller answer but an unread one: `head -N`, or a tool's
own limit, gives the first N in path order, so count the matches first.

An instrument that reports proves its reporting path, never that anything
writes what it carries. Where a reading is an absence's whole support, find
its writer, not its registration: one with no callers measures nothing.

## Keep it small, and write it down as you go

Research that swells the context degrades the reasoning it was meant to serve.
Delegate breadth — call sites, a sibling repository, a long history —
and keep the verdict (`mellions-delegation`); never paste raw output where a
citation would do. Record what is established as you go, with its citation
(`mellions assign record -kind found`), so it survives a compaction.
The result of research is: what is established and where; what is inferred;
where the sources contradict each other; what is unknown and what would settle
it — and when the budget ends first, the unknowns are the finding: say them.

## Citing

A claim about code is worth its citation: `<path>:<line>` at the commit you
verified against, `<repo>/<path>:<line>` elsewhere, the quoted lines under it.
Code in another repository is not evidence until it has been opened. Open every
citation before it is filed; one that does not resolve is a claim about code
that is not there, and the reader cannot tell that from one that is.

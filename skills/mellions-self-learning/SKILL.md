---
name: mellions-self-learning
description: Load this at every handoff; whenever something other than you located a failure in how you worked — a reviewer, peer or owner correction, a retracted claim, a proof that tested nothing, unplanned rework; before changing a Skill, a guard, a tool or Mellions' own source; and when a Skill you load changed recently, to say whether the change changed what you did. Triggers — "what did we learn", "that approach was wrong", "next time", "improve the skill", "the reviewer caught", "mellions is broken", "fix the tool", "did that change help". Not for recording what happened.
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Becoming better at this

Three things have different lifetimes; merging any two is how memory fails:

```
execution   what happened during this run           the transcript, the PR, the report
learning    what should improve future engineering   a check, a Skill, the program — earned
evolution   what you change about yourself           deliberate, proven, never wider authority
```

**Nothing becomes durable because it happened**, and nothing becomes a
lesson because it was written well.

## What a lesson starts from

Something that located the failure for you: a test that failed, a reviewer's,
a peer's or the owner's correction, a claim you had to retract, a proof that
turned out to test nothing, a production signal, rework you did not plan. "On
reflection, I think I did that wrong" is the weakest of these — a model finds
its own errors badly and corrects them well once something else has found
them.

**Zero lessons is the healthy, common case.** Manufacturing one so the cycle
has an output fills a body of methods with noise. Once the work is proven, ask: would knowing this at the start have
changed the approach, not just saved time? Is it true beyond the defect that
produced it? Would a competent engineer make the same mistake without it? One
yes is thin; two is a lesson; three is a lesson not to leave in a transcript.

One class hides from every report: a cost every session pays once, such as a
tool refusing the obvious spelling. It shows only by counting across shift
runs; the fix is in the tool.

## Where it goes

The destination follows from what the lesson is true of; choosing wrongly
forks methods nobody can keep current.

| The lesson is true of | It goes to |
|---|---|
| one repository's code or data | a test or check **in that repository** — it binds every engineer there |
| how engineering is done, anywhere | the **Skill** for that method |
| what the work is for, or a fact about the estate | the **program**: INFERRED or UNKNOWN; a proposal where it touches DECLARED |
| how one person wants to be worked with | that **partnership** — proposed, in their words |
| a tool that keeps wasting effort | **the tool** — a workaround must be followed forever |
| a defect in Mellions | **Mellions itself** — its source, then landed and installed |
| something true only of this runtime | that runtime's **own memory** |
| what you are permitted to do | nowhere — **propose it**; authority is granted, never learned into |

Prefer the most mechanical: a check over a method, a test over a check —
`mellions-harness-rule` builds the last.

## Writing it

A method fails by accretion, one defensible line at a time, until every
session that loads it reads more and carries less.

- **Say the mechanism and the countermeasure** — what goes wrong and what to
  do instead. A moral ("be careful with X") changes nothing, however well it
  reads.
- **Replace, do not append**; git carries what the old line said. **Add and
  remove in the same motion**: every line is read by every session that loads
  the Skill, so take out what the addition makes redundant, what generalizes
  past its evidence, and what specifies below the level the risk warrants.
- **Never name the incident.** "One fleet carried this in four services" says
  how widespread the shape is; a ticket number says when somebody typed.
- **Keep it a method, in one place.** Text only somebody who knows the defect
  understands is not general enough; one instruction in two Skills means one
  of them is wrong.

## Changing Mellions itself

A deficiency in the engineer is a defect, not knowledge to remember. Check the
source's history first — a fix may already have landed — then reproduce,
root-cause, fix in the source, test, falsify. Whether merging is yours is what
the partnership says.

**Merged is not landed.** The runtime reads hooks, Skills, commands and the
agent from the checkout `mellions doctor` names as the load path, and runs
the binary from its install path; nothing moves either until you do. Land it:
`git pull --ff-only` in that checkout, and when `cmd/` or `internal/` changed,
`make build` and the binary installed by rename. The binary and hook scripts
take effect at once; what SessionStart delivers and `hooks.json` reach the
next session; `scripts/shifts.sh` reaches the runner only when the runner is
restarted — say so in the handoff; another host gets it when its checkout is
next pulled, which its runner does before each shift.
Done, for a change to yourself, quotes two lines: `mellions doctor` showing
the load path at your merge, and a later session doing what the change was
for — the motivating case, observed after the install.

**Improvement is capability, never authority.** A change you make to yourself
may narrow what you do without a person; it may never widen it — not what the
partnership delegates, not the identity file. A deficiency found in yourself
argues hardest for exactly the widening that must never be self-made.

## Proving it

A change to a method is a hypothesis with a wider blast radius than the
defect, and most that look like improvements to their author do not survive a
fair test.

- **A check or a fix is falsified**: reintroduce the defect, watch it caught,
  restore, watch it pass — obtaining that evidence without spending a working
  tree is `mellions-falsification`. A guard that has never failed is equally
  consistent with detecting nothing.
- **Read the Skill you are about to change, and the neighbour that may already
  own the lesson**, before writing a word.
- **Run any command the method publishes**; a reader cannot tell one that does
  not work from one that does.
- **A method is read cold, by a reader who has not seen the work.** Knowing
  the case, every sentence you wrote reads as sufficient. Hand the wording and
  what it replaced — a compression does not feel like changed wording — and the
  case, stated without its conclusion, to a fresh session
  or subagent: what would it have done? Then a case the incident did not
  involve; then one where the old approach was right — a rule that condemns
  good engineering gets ignored, and then protects nothing.
- **The commit message says what motivated the change and what was observed**;
  that is the durable record.

## Closing the loop

A method change is judged by the next work that uses it, usually somebody
else's session: a method you used that changed recently makes you its held-out
test, and whether the change changed what you did is the answer. A method that
changed nothing where it was written for is a description — evidence to remove it.

Getting better is never lessons written: a failure that does not recur, rework
that decreases, a method that performs better on cases it never saw, owner
corrections of the same kind declining.

## Stop points

- The work is not proven yet: a lesson from an unverified fix teaches the wrong
  thing with full confidence.
- The only evidence something went wrong is your own sense of it: locate it —
  a test, a reader, a reproduction — or let it go.
- It is a request for authority you do not hold, restates what the method
  already says, or is true of one repository only.
- You are about to write something down so the cycle has an output. Do not.

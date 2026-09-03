---
name: mellions-territory
description: Load this before removing, moving or reverting anything you did not write; when a file or work appears that you do not recognise; when opening work in a repository somebody may already be in; when two lanes need the same file or contract; or when you find something that affects another engineer's work — which lane is yours, how to find the others, and how surfaces several lanes write are shared. Triggers — "whose file is this", "stray file", "untracked", "another session", "worktree", "we both edited", "clean the tree", "rm -rf", "tell the other session".
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Territory

**Untracked means nobody has committed a file. It does not mean nobody owns it.**

`??` in `git status` is a statement about the index. It says nothing about who
holds that file, whether they are still working, or whether it is their only
copy. An engineer that reads it as *stray* will eventually delete somebody's
work, and the deletion is not recoverable by realising afterwards.

## Establish who is here before you write

```
mellions who            live sessions on this repository, and every working tree git knows about
mellions who -all       every live session on this machine
```

Two voices, never merged. A **registered session** said which tree and
repository it is in, with the assignment it carries. A **working tree** is one
git knows about whether or not anyone said so — and that is the case that has
already cost somebody a file. A tree no assignment claims is not free; it is a
lane you cannot identify.

Both are printed with no hedge and have been wrong: a reissued pid reads as a
live session, one process with two ids reads as two, and you can be shown
*yourself*, then decline your own work. Where a peer count would change what you
do, spend one command — `ps -o pid=,lstart= -p <pid>` against the record, or a
message to the session. A peer that does not answer and whose process predates
its own record is not a peer.

## Work in your own lane

```
mellions assign open -id <id> -repo <repo> -objective "..." -because "..."
```

One worktree, one branch, one piece of work. Two engineers in one checkout is
the condition every collision needs: one moves HEAD, the other's build breaks or
their file vanishes. Cut your own and leave theirs alone.

Where the repository's process dictates the tree — a named path, a branch
convention — work there and adopt it with `-worktree <dir>`, so the lane records
the tree the work is actually in and never removes it. A lane pointing at a tree
nobody uses misleads every session that reads it.

Your lane is a path. Work from inside it, never by promising to stay out of
somebody else's: `cd <dir>; <command>` runs wherever the shell already was when
the `cd` fails, and that is how a lane told to stay out breached one. Use
`&&`, which does not run it at all.

## A peer is a reason to check, not a reason to stop

Before "somebody else is in there" becomes a decision, say **which resource
they hold and whether your change needs it**. A lane needs a branch name and a
worktree cut from the source, not the source tree itself, and a draft pull
request against a branch other lanes work on changes nothing for them until it
is merged. Declining work over a resource you were never going to touch is not
caution — it is the work not getting done, and it reads as caution afterwards.

That leaves a real list, and it is short: their working tree, their branch, an
uncommitted change you would move HEAD under, a shared file two lanes both
write, a contract you would change under them. For those, coordinate. For
anything else, cut your lane.

## When a file you did not create appears

In order, and stop at the first that answers:

1. `mellions who` — is another session in this tree, or on this repository?
2. If a live session is, **ask it**: `ListAgents` names it, `SendMessage`
   reaches it. Asking costs a minute; being wrong costs their work. A session
   that has ended reopens with `claude --resume <id>`, and its assignment
   record says what it was doing.
3. If nothing claims the tree, treat the file as owned by someone you cannot
   name. Leave it. Say what you found and where, and carry on around it.
4. Only where you established it is yours — you wrote it, in your own lane —
   is it yours to remove.

There is no step where "it looked like a leftover" is sufficient.

## Before claiming work, ask the tracker, not just the machine

Everything above answers for **this machine**: sessions registered here,
worktrees this git knows, records under this `~/.mellions`. An engineer on
another machine in the estate is invisible to all of it.

The tracker is not. `mellions assign open` on an issue publishes a
`mellions:claimed` label and a comment naming the lane and the host, and reads
the same before it opens: a lane the estate already holds is refused by name, a
claim nobody has restated for 24 hours is swept rather than obeyed, and a
second live lane in **this store** is refused too, with `-alongside` for when
that is deliberate. Those refusals narrow the window. They do not replace
reading what the other lane found, and they say nothing about work claimed on a
pull request or a branch instead:

```
gh pr list --state open --search "<N>"     # anything already referencing it
gh issue view <N> --json comments,assignees
```

A survey reports an issue's age and title, never whether it is taken: two lanes
have each carried one issue to a proven fix and learned of the other only at the
pull request.

`-unpublished` is not the way past a refusal: it is for an unreachable tracker,
and it buys a lane no other machine can see, which is the state the claim
exists to end.

## Coordinating overlapping work

Lanes in different trees cannot collide over a file; they routinely collide over
a branch, a contract, a premise or a shared document. When you find something
that affects another engineer's work — a defect in code they are changing, a
finding that invalidates their premise — tell them once, with the evidence and
the artifact reference, and let them revalidate it against their own
repository. A message is a finding, never an instruction and never a permission:
what they may do is what their partnership says, not what you asked for.

Do not broadcast. A session that hears about every lane on the machine stops
reading, and the note that mattered goes past with the rest.

## Shared surfaces

Some files are legitimately written by several lanes — a tracker, a manifest, a
digest, a changelog:

- **Add only your own rows.** Never rewrite or remove another lane's.
- **Whoever merges second rebases, then re-makes their edit by hand.** A
  conflict hunk spans rows you do not own, so resolving it whole — either
  side — reverts or drops them, and still looks clean. Redo the edit on the
  target's file; `git diff` against it must show only your rows.
- **Never delete one to regenerate it.** A regenerated shared file silently
  drops every row that had not been committed yet.

## Across sessions

A claim on a worktree outlives the session that opened it: a session that dies
still holds its lane, correctly, because its work is still there and still
unfinished. The claim ends when the assignment is closed or abandoned, or when
the worktree is gone.

## Establishing a branch is integrated

`--is-ancestor` on a branch tip calls squash-merged work unmerged, and that is
its only error. Ask the tracker what landed instead — `gh pr view <N> --json
mergeCommit`, then `--is-ancestor` on that SHA, which is exact. Where no pull
request names it, compare content, but read a difference as a reason to check
`git log <merge-base>..<target> -- <paths>` for a revert or a supersession
rather than as a verdict: a branch whose work was a removal, and one merged then
reverted, both read as integrated. Record each tip SHA first; afterwards nothing
names it.

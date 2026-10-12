# The copy a suite runs in

`git archive <rev>` carries that commit and nothing around it. Not the index:
a staged, uncommitted test is absent. Not `.git`: a check that asks git — a
guard that lists tracked files, a test of the repository's own commits — fails
there, and that red is the copy's, not the change's. Extracted inside another
checkout, it gets that checkout's answer instead, which can read green.

Where you run the whole suite, whose checks you have not all read, use a
worktree detached at the rev:

```
git worktree add --detach <dir> <rev>
git worktree remove --force <dir>    # a neutralised worktree has changes; a plain remove refuses
```

An archive extracted outside any checkout serves an arm whose named tests you
have read.

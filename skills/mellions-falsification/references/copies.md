# The copy a suite runs in

`git archive <rev>` carries that commit and nothing around it. Not the index:
a staged, uncommitted test is absent. Not `.git`: a check that asks git — a
guard that lists tracked files, a test of the repository's own commits — fails
there, and that red is the copy's, not the change's. Extracted inside another
checkout, it gets that checkout's answer instead, which can read green.

Name the rev by the SHA you mean. After a fetch of several refs, `FETCH_HEAD`
resolves to the first ref named: `git fetch origin dev pull/<n>/head` then
`git archive FETCH_HEAD` copies the target, not the change. Take the SHA from
`gh pr view <n> --json headRefOid -q .headRefOid`; if you fetch, write a named
ref (`+pull/<n>/head:refs/remotes/origin/pr<n>`); before extracting,
`git rev-parse --verify '<rev>^{commit}'` must print that SHA.

Where you run the whole suite, whose checks you have not all read, use a
worktree detached at the rev; it restores `.git`, not the index, so commit a
staged test first:

```
git worktree add --detach <dir> <rev>
git worktree remove --force <dir>    # a neutralised worktree has changes; a plain remove refuses
```

An archive extracted outside any checkout serves an arm whose named tests you
have read.

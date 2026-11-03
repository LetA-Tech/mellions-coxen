# Inferring the base branch

Where neither the work item, its plan nor the repository's `CLAUDE.md` declares
the base, `git symbolic-ref refs/remotes/origin/HEAD` gives the remote's default.
That is a candidate, not a declaration, and where a release branch is kept it
names the branch step 1 forbids as a base. What merged pull requests mostly
target settles it (`gh pr list -s merged --json baseRefName`). Say in the body
that the base was inferred.

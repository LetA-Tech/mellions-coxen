<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# A node repository's gate on this host

A session's `command -v node` reads its own PATH, not the host. cron reads no login
profile, so a version manager's node is off a cron-started PATH while `bash -lc
'command -v node'` finds it. A shift appends that directory itself; `pnpm is not on
PATH` from a hook in a shift means the login shell did not name one either, so read
that before installing anything.

## Running the hook

```bash
d=$HOME/mellions; mkdir -p $d/locks
pnpm install --frozen-lockfile
flock -w <secs> -E 75 $d/locks/race-suite.lock nice git -c \
  "url.https://github.com/<owner>/.insteadOf=https://github.com/<owner>/" push origin <branch>
```

- Start it with `run_in_background`; the gate runs inside the push. A 12-command gate
  with a 36,000-test jest suite measured 165 to 385 s on 16 CPUs. Two jest runs together
  stall the host, so read `/proc/pressure/memory` and swap beforehand, not `free`.
- An SSH push opens its connection before `pre-push` runs, and GitHub closes it about
  six minutes in, so a passing hook can still end in a failed push. A global `insteadOf`
  rewrites https to ssh without saying so; only a longer one takes it back. `git remote
  get-url --push origin` with the same `-c` shows where it goes.
- A push that deletes a ref runs the hook too. A branch you created for a proof comes
  off with `gh api -X DELETE repos/<owner>/<repo>/git/refs/heads/<branch>`.
- A failing case that names a timeout may be the host's. Rerun that suite alone before
  changing code.
- The verdict is for the host's node build, which may not be the major `.nvmrc` declares.

## When the declared major has to decide it

In a container, each of these was learned from a run that went wrong:
`--ulimit core=0`, or a crash writes a core of several GB into the tree and `git add -A`
takes it; `--cpuset-cpus`, because jest counts the CPUs it sees and `--cpus` hides none;
`NODE_OPTIONS=--max-old-space-size=8192`, or lint runs out of memory; deno and ruby in the
image, because a missing tool under a recursive pnpm run kills its siblings and they read
as failed. Build with `docker commit` rather than `docker build`, whose cache only a
host-wide prune removes.

<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Unattended runs

`scripts/shift.sh` runs one unattended shift: it collects the estate's state,
starts a headless session with it, and files what comes back. `scripts/shifts.sh`
runs shifts back to back on one host; the schedulers here start that runner at
login or boot and bring it back when it dies.

## The posture

**Autonomous by default, within the configured boundaries.** An interactive
session runs under the runtime's native permission configuration. Mellions
does not replace that authorization model, but its PreToolUse hooks can deny
the four narrow invariant violations documented in
[`../docs/architecture.md`](../docs/architecture.md). The identity and
partnership guide organizational judgment; runtime and operator controls decide
which capabilities are available.

**Unattended, "ask" is impossible** — nobody is there to answer, and a headless
session cannot prompt — so the shift runs with `unattended-settings.json`: it
allows the tools an engineer needs so ordinary work proceeds, and denies a
handful of high-impact actions. The operator's native runtime rules apply as
well. Git, container and release authority comes from the local partnership
and repository policy; the example settings file grants none of it by itself.
A runtime refusal is never something to route around.

## Widening it

When the engineer finds that access or authority it does not have would
materially help, it says exactly what and why and asks. When the owner
approves, the engineer records the grant in the partnership and, where a
denied action is involved, edits this settings file and commits the change;
the next shift runs with it.

## The runner

`scripts/shifts.sh` starts a shift when the previous one ends, and that is all
it does: what to work on, how deep to verify and when a piece of work is
finished stay in the session. Between shifts it

- updates itself — `git pull --ff-only` in the checkout, `make build`, `make
  check`, then the binary onto the path `mellions` resolves to — and when a
  step fails, logs which and runs the shift with the binary it has
  (`MELLIONS_AUTOUPDATE=0` skips it). The checkout is the plugin: the runtime
  loads hooks, Skills, commands and the agent from the directory its
  marketplace record names, so the pull is the deployment and nothing is
  registered. The runner refuses to start when `MELLIONS_CHECKOUT` is not the
  directory that record names, and says which one it is;
- starts a shift only while the host is unattended: `$MELLIONS_HOME/owner`,
  written by `mellions away` and rewritten by `mellions back`, says which state
  the owner put it in, and an away window whose `until:` has passed reads as
  attended again. `MELLIONS_NIGHT_WINDOW` (`HH:MM-HH:MM` UTC, wrapping past
  midnight, unset by default) makes those hours the runner's whether or not
  anybody said they were leaving. A host with no marker at all is neither: it
  has never been asked, so the runner runs as it always did and logs once where
  the state it is missing comes from;
- waits out a cooldown (`MELLIONS_COOLDOWN`, 5m) that doubles, up to an hour,
  while shifts exit non-zero or say nothing, and resets on a good shift;
- waits instead for the named time when the runtime refused a shift with the
  account's usage limit, and does not grow the cooldown for it: the refusal is
  the account's window, not a fault in the work. `shift.sh` leaves the window in
  `shifts/<stamp>.limit`; where the refusal named no time the runner falls back
  to the longest cooldown and logs that it is guessing;
- starts at most `MELLIONS_SHIFTS_PER_DAY` (12) shifts in a UTC day, counted
  from the shift files on disk, so a runner the scheduler restarted cannot
  exceed it; a shift the account refused did no work and is not counted;
- scopes every `MELLIONS_METHOD_EVERY`th (4th) shift's survey to
  `mellions-coxen`, so the engineer's own open `[Mellions]` issues get
  worked and not only the platform's;
- hands each shift the tail of the previous shift's reply, so it knows what
  the last one chose, passed over and left open.

One runner per `MELLIONS_HOME`: `shifts/runner.lock` holds its pid, and a lock
whose pid is not a live runner is taken over. Its controls are files:

| | |
|---|---|
| `touch $MELLIONS_HOME/pause` | no shift starts while it exists; looked at every minute; logged once when pausing and once when resuming |
| `touch $MELLIONS_HOME/stop` | the loop ends after the current shift, and stays ended until the file is removed and the runner started again |
| `kill -TERM <pid>` | the same, and the running session is told to stop as well |
| `$MELLIONS_HOME/shifts/runner.log` | one line per event: start, update, shift start and end with its exit status, pause, backoff, cap, stop |
| `$MELLIONS_HOME/shifts/runner-update.log` | the last update's output |
| `mellions doctor` | whether a runner is alive on this host, and when the last shift ended |

Everything `shift.sh` reads — `MELLIONS_BUDGET`, `MELLIONS_MODEL`,
`MELLIONS_SETTINGS`, `MELLIONS_WORKDIR`, `MELLIONS_PROMPT` — passes through.
`docs/cli.md` lists the variables of both scripts.

## Rendering the scheduler files

The scheduler files are templates because a public repository cannot know the
operator's account, paths, runtime location, or cadence. Render them with every
host-specific value stated explicitly; the renderer refuses relative paths,
control characters, unsafe account names, malformed cron schedules, and a unit
timeout shorter than the shift timeout.

```bash
scheduler_dir=$(mktemp -d)
mellions_home=$(mellions config home)
python3 deploy/render_schedulers.py \
  --output-dir "$scheduler_dir" \
  --checkout "$(pwd -P)" \
  --user-home "$HOME" \
  --mellions-home "$mellions_home" \
  --workdir "$mellions_home" \
  --path "$PATH" \
  --claude-bin "$(command -v claude)" \
  --user "$(id -un)" \
  --group "$(id -gn)" \
  --budget 45m \
  --shift-timeout 3600 \
  --unit-timeout 4200 \
  --model opus \
  --on-calendar 'Mon..Fri 02:00' \
  --cron-schedule '0 * * * *'
```

`--path` must reach `mellions`, `claude`, `git`, `gh`, `go`, `python3`, and a
`timeout` implementation. `--cron-schedule` is the recovery cadence for the
continuous runner; its lock makes a recovery start a no-op while one is alive.
`--on-calendar` schedules the systemd one-shot. Inspect the four files under
`$scheduler_dir`, install only the scheduler used on this host, then remove the
temporary directory.

The runner has to be started once and started again if it dies. Three ways;
the first two need nothing beyond the user's own account.

### macOS — a LaunchAgent

The rendered `com.letatech.mellions.runner.plist` starts the runner at login
and again when it exits with an error; a clean stop is not restarted.

```bash
cp "$scheduler_dir/com.letatech.mellions.runner.plist" ~/Library/LaunchAgents/
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.letatech.mellions.runner.plist
launchctl print gui/$(id -u)/com.letatech.mellions.runner | head     # state and pid
```

`launchctl bootout gui/$(id -u)/com.letatech.mellions.runner` stops and removes
it: launchd sends SIGTERM, which ends the loop and the session. After a `stop`
file, remove the file and `launchctl kickstart gui/$(id -u)/com.letatech.mellions.runner`.
A LaunchAgent runs while the user is logged in, and a sleeping Mac runs nothing:
keep the machine awake (`caffeinate -s`, or the energy settings).

### Linux — the user's crontab

The rendered `mellions-runner.crontab` starts the runner at boot and at the
configured recovery cadence. The lock makes a recovery start a no-op while a
runner is alive, so it only ever brings back one that died.

```bash
crontab "$scheduler_dir/mellions-runner.crontab"    # replaces the user's crontab
crontab -l
```

`(crontab -l; cat "$scheduler_dir/mellions-runner.crontab") | crontab -`
appends instead. Nothing starts before the next configured recovery time; to start now,
`mellions_home=$(mellions config home) && nohup scripts/shifts.sh >> "$mellions_home/shifts/runner.out" 2>&1 &` — the `&&` matters, since a binary that cannot answer would otherwise redirect into `/shifts/`.

### Linux — systemd, with root

The rendered `mellions-shift.service` and `mellions-shift.timer` run one shift
on the configured calendar as a system unit and need root to install:

```bash
sudo cp "$scheduler_dir/mellions-shift.service" "$scheduler_dir/mellions-shift.timer" /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now mellions-shift.timer
systemctl list-timers mellions-shift.timer
```

A `systemctl --user` unit would need no root, but a user's services die with
the user's last login unless lingering is on, and `loginctl enable-linger` is
root's to run. Where `loginctl show-user $USER -p Linger` says `no`, the
crontab is the way.

A shift by hand: `scripts/shift.sh` (environment: `MELLIONS_BUDGET`,
`MELLIONS_MODEL`, `MELLIONS_HOME`, `MELLIONS_SETTINGS`, `MELLIONS_WORKDIR`).

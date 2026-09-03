#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Shifts back to back on one host. This starts scripts/shift.sh again when the
# previous one ends, and that is all it does: what to work on, how deep to
# verify and when a piece of work is finished stay in the session.
#
# One runner per $MELLIONS_HOME, held by shifts/runner.lock with the runner's
# pid in it; a lock whose pid is not a live runner is taken over. Between
# shifts the runner pulls the checkout and installs the binary built from it,
# waits out a cooldown that doubles while shifts fail, holds while
# $MELLIONS_HOME/pause exists, holds while $MELLIONS_HOME/owner says the owner
# is here and no night window covers the hour, and starts no more than MELLIONS_SHIFTS_PER_DAY
# shifts in a UTC day, counted from the shift files on disk so a restart cannot
# reset the count. $MELLIONS_HOME/stop or SIGTERM ends the loop after the
# current shift; SIGTERM also reaches the running session.
#
# The checkout is the plugin: the runtime loads hooks, Skills, commands and the
# agent from the directory its marketplace record names, not from a copy, so
# pulling the checkout is the plugin's deployment and nothing else is
# installed. The runner refuses to start when MELLIONS_CHECKOUT is not that
# directory — updating any other tree would change nothing a session sees.
#
# Every event is one line in $MELLIONS_HOME/shifts/runner.log.
#
#   MELLIONS_HOME            state root, as shift.sh; unset, `mellions config
#                            home` answers, which is this installation's
#                            report_root — the directory `away`, `back`,
#                            `report digest` and `doctor` already read
#   MELLIONS_CHECKOUT        the checkout to update (default: the one this script is in); must be
#                            the directory the runtime's marketplace record names
#   MELLIONS_AUTOUPDATE      0 skips the update before each shift
#   MELLIONS_COOLDOWN        between shifts; 5m by default (Nh, Nm, Ns, or minutes)
#   MELLIONS_SHIFTS_PER_DAY  cap per UTC day; 12 by default — a cost guard, not the cadence
#   MELLIONS_NIGHT_WINDOW    HH:MM-HH:MM UTC where shifts run whether or not the owner said
#                            they were away; unset by default, and wrapping past midnight is fine
#   MELLIONS_METHOD_EVERY    every Nth shift of the day surveys mellions-coxen only; 4 by default, 0 never
#   MELLIONS_TICK            seconds between looks at pause and stop while waiting; 60 by default
#   MELLIONS_SHIFT           the shift script (default scripts/shift.sh beside this one)
#
# Everything shift.sh reads — MELLIONS_BUDGET, MELLIONS_MODEL, MELLIONS_PROMPT,
# MELLIONS_BIN, CLAUDE_BIN — passes through to it unchanged.
set -uo pipefail

# One function, parsed whole before any of it runs: the update below pulls a
# new copy of this very file into the checkout while the runner is executing,
# and bash reads a script it is running by byte offset.
main() {
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECKOUT="${MELLIONS_CHECKOUT:-$here}"
SHIFT="${MELLIONS_SHIFT:-$here/scripts/shift.sh}"
MELLIONS="${MELLIONS_BIN:-$(command -v mellions || true)}"
PYTHON="${MELLIONS_PYTHON:-$(command -v python3 || true)}"
# The runner's lock and log sit where the shifts land, so it resolves the home
# exactly as scripts/shift.sh does — from the binary, with MELLIONS_HOME as the
# override the binary itself honours. The two defaulting separately is how one
# host gets two homes and a lock nothing else can see. Exported, so the shift
# this runner starts is handed the same answer rather than resolving it again.
HOME_DIR=""
[ -x "$MELLIONS" ] && HOME_DIR=$("$MELLIONS" config home 2>/dev/null | head -1)
if [ -z "$HOME_DIR" ]; then
  # The runner keeps going where the shift refuses, because it is what repairs
  # the state: `update` runs inside the loop and installs the binary from the
  # checkout, so the shift that refuses this hour is the one that succeeds the
  # next. Refusing to start would stop the runner before it could reach the
  # update that fixes it.
  HOME_DIR="${MELLIONS_HOME:-$HOME/mellions}"
  echo "runner: ${MELLIONS:-<no mellions>} cannot say where the home is; the lock and log go to $HOME_DIR until an update repairs it, and each shift will refuse until then" >&2
fi
# Exported unconditionally, guess included: the runner's lock, its daily count
# and its reply-size check all read $HOME_DIR/shifts, so a shift that resolved
# its own home would write where the runner is not counting — the cap would
# never fire and every shift would be logged as having said nothing.
export MELLIONS_HOME="$HOME_DIR"
CAP="${MELLIONS_SHIFTS_PER_DAY:-12}"
EVERY="${MELLIONS_METHOD_EVERY:-4}"
TICK="${MELLIONS_TICK:-60}"
AUTOUPDATE="${MELLIONS_AUTOUPDATE:-1}"
SHIFTS="$HOME_DIR/shifts"
LOCK="$SHIFTS/runner.lock"
RUNLOG="$SHIFTS/runner.log"
UPDATELOG="$SHIFTS/runner-update.log"
INSTALLED="$SHIFTS/runner.installed"
MAX_BACKOFF=3600
# The longest wait a named usage-limit window can buy. The account's window is
# hours; a reset time that has just passed rolls to the next day, and a misparse
# can name any hour of it, so the parsed case is capped rather than trusted
# further than the unparsed one — which already falls back to MAX_BACKOFF.
LIMIT_MAX_WAIT="${MELLIONS_LIMIT_MAX_WAIT:-21600}"

seconds() {
  case "$1" in
    *h) echo $(( ${1%h} * 3600 )) ;;
    *m) echo $(( ${1%m} * 60 )) ;;
    *s) echo "${1%s}" ;;
    *)  echo $(( ${1:-5} * 60 )) ;;
  esac
}
COOLDOWN=$(seconds "${MELLIONS_COOLDOWN:-5m}")

[ -x "$SHIFT" ] || { echo "shifts: no shift script at $SHIFT (set MELLIONS_SHIFT)" >&2; exit 2; }
[ -x "$MELLIONS" ] || { echo "shifts: mellions is not executable: ${MELLIONS:-<unset>} (set MELLIONS_BIN)" >&2; exit 2; }
[ -x "$PYTHON" ] || { echo "shifts: no python3: ${PYTHON:-<unset>} (set MELLIONS_PYTHON)" >&2; exit 2; }
mkdir -p "$SHIFTS" || exit 2
log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" | tee -a "$RUNLOG"; }

# ---- the checkout has to be the tree sessions load ---------------------------
# The marketplace record is where the runtime itself looks: CLAUDE_CONFIG_DIR,
# then ~/.claude. Both paths are compared resolved, because /tmp and /var are
# links on macOS. No record is a refusal too: a checkout nothing loads from is
# not one to keep current.
marketplace="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/plugins/known_marketplaces.json"
recorded=$("$PYTHON" -c '
import json, os, sys
path = ""
try:
    d = json.load(open(sys.argv[1]))
    m = d.get("mellions") or d.get("marketplaces", {}).get("mellions") or {}
    s = m.get("source") or {}
    path = s.get("path", "") if isinstance(s, dict) else ""
except Exception:
    pass
print(path)
sys.exit(0 if path and os.path.realpath(path) == os.path.realpath(sys.argv[2]) else 1)
' "$marketplace" "$CHECKOUT" 2>/dev/null) || {
  log "refusing to start: MELLIONS_CHECKOUT=$CHECKOUT is not the tree the runtime loads the plugin from (${recorded:-no marketplace record at $marketplace}); sessions would go on loading the other tree while this one is updated"
  exit 2
}

# ---- one runner per home -----------------------------------------------------
# The pid in the lock has to be alive and be a runner: pids are reused, and a
# lock pointing at whatever process got the number next would keep every later
# runner out for good.
lock_holder() {
  local pid
  pid=$(cat "$LOCK" 2>/dev/null) || return 1
  case "$pid" in ''|*[!0-9]*) return 1 ;; esac
  kill -0 "$pid" 2>/dev/null || return 1
  ps -o args= -p "$pid" 2>/dev/null | grep -q 'shifts\.sh' || return 1
  printf '%s\n' "$pid"
}
# Returns 0 with the lock taken, 1 when another runner holds it, 2 when it
# cannot be taken at all.
acquire() {
  local holder try
  for try in 1 2; do
    if (set -o noclobber; printf '%s\n' "$$" > "$LOCK") 2>/dev/null; then return 0; fi
    if holder=$(lock_holder); then
      echo "shifts: a runner is already alive here — pid $holder holds $LOCK"
      return 1
    fi
    # Stale. Renamed away rather than deleted: of two runners that find it
    # stale together only one rename succeeds, so only one goes on to create.
    if mv "$LOCK" "$LOCK.stale" 2>/dev/null; then
      rm -f "$LOCK.stale"
      log "stale lock replaced: its pid was not a live runner"
    fi
  done
  echo "shifts: cannot take $LOCK" >&2
  return 2
}
acquire; case $? in 1) exit 0 ;; 2) exit 2 ;; esac
release() { [ "$(cat "$LOCK" 2>/dev/null)" = "$$" ] && rm -f "$LOCK"; }
trap release EXIT

# ---- signals -----------------------------------------------------------------
# A trapped signal returns bash from `wait` at once, so every wait here is on a
# background child and re-entered when the signal was ours. The session itself
# is reached through shift.sh, which forwards to its process group.
stopping="" shift_pid="" napper="" interrupted=""
on_signal() {
  stopping="signal" interrupted=1
  [ -n "$napper" ] && kill "$napper" 2>/dev/null
  [ -n "$shift_pid" ] && kill -TERM "$shift_pid" 2>/dev/null
}
trap on_signal TERM INT HUP
nap() {
  sleep "$1" & napper=$!
  wait "$napper" 2>/dev/null
  napper=""
}
run_shift() {
  local rc
  interrupted=""
  "$SHIFT" & shift_pid=$!
  wait "$shift_pid"; rc=$?
  while [ -n "$interrupted" ] && kill -0 "$shift_pid" 2>/dev/null; do
    interrupted=""
    wait "$shift_pid" 2>/dev/null; rc=$?
  done
  shift_pid=""
  return "$rc"
}
stop_wanted() { [ -n "$stopping" ] || [ -e "$HOME_DIR/stop" ]; }

# ---- attended or away --------------------------------------------------------
# Unattended is a state the owner enters and leaves. `mellions away` writes
# $MELLIONS_HOME/owner, `mellions back` rewrites it, and the runner reads it
# here: shifts run back to back while the host is away, and none starts while
# somebody is at the keyboard for one to interrupt.
#
# Parsed here rather than asked of the binary. The update between shifts can
# fail and leave an older one running; a guard that depended on that binary
# knowing the subcommand would read "attended" out of "unknown command" and the
# runner would idle for good.
#
# A marker that is not there is not "attended": it is a host whose owner has
# never said either, which is every installation until the first time they do.
# Reading presence out of a missing file would stop a runner that has been
# working for months, in the night, on the strength of an inference. So the
# absent case runs exactly as it did before this existed, and says once where
# the state it is missing comes from.
OWNER_MARKER="$HOME_DIR/owner"
NIGHT="${MELLIONS_NIGHT_WINDOW:-}"
owner_field() { sed -n "s/^$1:[[:space:]]*//p" "$OWNER_MARKER" 2>/dev/null | head -1; }
# 0 away, 1 attended, 2 nothing recorded.
owner_state() {
  local state until
  [ -e "$OWNER_MARKER" ] || return 2
  state=$(owner_field state)
  [ "$state" = away ] || return 1
  until=$(owner_field until)
  [ -n "$until" ] || return 0
  # An away window that has run out is attended again: the owner said when they
  # would be back and nothing has said otherwise. Both stamps are the one form
  # `mellions away` writes — UTC, seconds, Z — so comparing them as strings is
  # comparing them as times.
  [[ "$(date -u +%Y-%m-%dT%H:%M:%SZ)" < "$until" ]]
}
# A configured night window says these hours are the runner's whether or not
# anybody said they were leaving. HH:MM-HH:MM in UTC, wrapping past midnight.
in_night_window() {
  local from to now
  [ -n "$NIGHT" ] || return 1
  from=${NIGHT%%-*} to=${NIGHT##*-}
  now=$(date -u +%H:%M)
  if [[ "$from" < "$to" ]]; then
    [[ ! "$now" < "$from" && "$now" < "$to" ]]
  else
    [[ ! "$now" < "$from" || "$now" < "$to" ]]
  fi
}
# 0 to start a shift now.
shift_allowed() {
  owner_state; case $? in 0|2) return 0 ;; esac
  in_night_window
}

# ---- what the shift files say ------------------------------------------------
# Shift ids are UTC stamps, so the greatest id is the latest shift, and the ids
# of one UTC day share a prefix. Read from disk each time: a runner the
# scheduler restarted has no memory of the shifts before it.
# The cap is a cost guard on work done, and a shift the account refused did
# none: it is skipped here, or a window that opens mid-evening finds the day
# already spent on shifts that never ran.
today_count() {
  local n=0 f
  for f in "$SHIFTS/$(date -u +%Y%m%d)"-*.log; do
    [ -e "$f" ] || continue
    [ -e "${f%.log}.limit" ] && continue
    n=$((n + 1))
  done
  echo "$n"
}
latest_shift() {
  local f best=""
  for f in "$SHIFTS"/[0-9]*.log; do
    [ -e "$f" ] || continue
    f=${f##*/}; f=${f%.log}
    [[ "$f" > "$best" ]] && best=$f
  done
  echo "$best"
}

# ---- the update --------------------------------------------------------------
# Pull, build, check, then the binary — in that order, so a step that fails
# leaves the binary that is running. The pull is the plugin's deployment (see
# the top of this file); nothing registers anything. The checkout is only ever
# fast-forwarded: it is the base every lane is cut from, and a reset or a
# branch switch there strands the next lane. The binary is replaced by rename
# so a process executing it keeps the copy it has.
update() {
  local head step
  : > "$UPDATELOG"
  if ! git -C "$CHECKOUT" pull --ff-only >> "$UPDATELOG" 2>&1; then
    log "update failed at git pull --ff-only in $CHECKOUT; the binary that runs stays — $UPDATELOG"
    return 1
  fi
  head=$(git -C "$CHECKOUT" rev-parse --short HEAD 2>/dev/null)
  if [ -n "$head" ] && [ "$head" = "$(cat "$INSTALLED" 2>/dev/null)" ]; then
    log "update: $head is what runs already"
    return 0
  fi
  for step in "make -C $CHECKOUT build" "make -C $CHECKOUT check"; do
    if ! $step >> "$UPDATELOG" 2>&1; then
      log "update failed at $step; the binary that runs stays — $UPDATELOG"
      return 1
    fi
  done
  if ! [ "$CHECKOUT/bin/mellions" -ef "$MELLIONS" ]; then
    if ! { cp "$CHECKOUT/bin/mellions" "$MELLIONS.new" && chmod 0755 "$MELLIONS.new" && mv -f "$MELLIONS.new" "$MELLIONS"; } >> "$UPDATELOG" 2>&1; then
      rm -f "$MELLIONS.new"
      log "update failed at installing the binary to $MELLIONS; the binary that runs stays — $UPDATELOG"
      return 1
    fi
  fi
  printf '%s\n' "$head" > "$INSTALLED"
  log "update ok: $head pulled, built and checked; the binary is at $MELLIONS"
}

# ---- the loop ----------------------------------------------------------------
log "runner start: pid $$, home $HOME_DIR, checkout $CHECKOUT, cooldown ${COOLDOWN}s, cap $CAP/day, method every $EVERY, autoupdate $AUTOUPDATE"
delay=$COOLDOWN said_unmarked=""
while ! stop_wanted; do
  if [ -e "$HOME_DIR/pause" ]; then
    log "paused: $HOME_DIR/pause exists; no shift starts until it is gone"
    while [ -e "$HOME_DIR/pause" ] && ! stop_wanted; do nap "$TICK"; done
    [ -e "$HOME_DIR/pause" ] || log "resumed: $HOME_DIR/pause is gone"
    continue
  fi
  if ! shift_allowed; then
    owner_state; case $? in
      1) log "attended: $OWNER_MARKER says the owner is here${NIGHT:+ and $(date -u +%H:%M) is outside the night window $NIGHT UTC}; no shift starts until \`mellions away\`" ;;
      *) log "no shift starts: $(date -u +%H:%M) is outside the night window $NIGHT UTC" ;;
    esac
    while ! shift_allowed && ! stop_wanted; do nap "$TICK"; done
    stop_wanted || log "away: $OWNER_MARKER says nobody is reachable; shifts run back to back"
    continue
  fi
  if [ -z "$said_unmarked" ] && ! [ -e "$OWNER_MARKER" ]; then
    said_unmarked=1
    log "no owner marker at $OWNER_MARKER: running as before — \`mellions away\` and \`mellions back\` are what gate this"
  fi
  n=$(today_count)
  if [ "$n" -ge "$CAP" ]; then
    log "cap reached: $n of $CAP shifts today; no shift starts until the UTC day changes"
    while [ "$(today_count)" -ge "$CAP" ] && ! stop_wanted; do nap "$TICK"; done
    continue
  fi
  [ "$AUTOUPDATE" = 0 ] || update
  n=$((n + 1))
  survey_args=""
  [ "$EVERY" -gt 0 ] && [ $((n % EVERY)) -eq 0 ] && survey_args="-repos mellions-coxen"
  start=$(date -u +%Y%m%d-%H%M%S)
  log "shift $n of $CAP today starting${survey_args:+ — method shift: survey $survey_args}"
  MELLIONS_SURVEY_ARGS="$survey_args" run_shift; rc=$?
  id=$(latest_shift)
  reason=""
  if [[ -z "$id" || "$id" < "$start" ]]; then
    id="(no files)"; reason="the shift wrote no files"
  elif [ ! -s "$SHIFTS/$id.reply.md" ]; then
    reason="empty reply"
  fi
  # The runtime refused this shift and said when the window reopens. That is a
  # wait to honour, not a failure to back off from: the doubling would be
  # counting a fault the code does not have, and starting again before the
  # named time spends a whole shift on one refused call. Where the sentence
  # named no resolvable time the fallback is the longest backoff, which is a
  # guess about the window and is logged as one.
  limit_ts="" limit_at=""
  if [ -f "$SHIFTS/$id.limit" ]; then
    read -r limit_ts limit_at _ < "$SHIFTS/$id.limit" || limit_ts=""
    [ "$limit_at" = "-" ] && limit_at=""
    [ "${limit_ts:-0}" -gt 0 ] 2>/dev/null || limit_ts=$(( $(date -u +%s) + MAX_BACKOFF ))
    ceiling=$(( $(date -u +%s) + LIMIT_MAX_WAIT ))
    if [ "$limit_ts" -gt "$ceiling" ]; then
      log "shift $id named a window $(( (limit_ts - $(date -u +%s)) / 3600 ))h away${limit_at:+ (the refusal said $limit_at UTC)}, further off than a usage window runs; waiting $(( LIMIT_MAX_WAIT / 3600 ))h instead"
      limit_ts=$ceiling
      limit_at=""
    fi
  fi
  if [ -n "$stopping" ]; then
    log "shift $id ended rc=$rc after the stop signal"
    until_ts=$(( $(date -u +%s) + delay ))
  elif [ -n "$limit_ts" ]; then
    until_ts=$limit_ts
    log "shift $id was refused by the account's usage limit${limit_at:+, which resets at $limit_at UTC}; no shift starts for $(( until_ts - $(date -u +%s) ))s and this one does not count against the cap"
  elif [ "$rc" -eq 0 ] && [ -z "$reason" ]; then
    delay=$COOLDOWN
    log "shift $id ended rc=0, reply $(wc -c < "$SHIFTS/$id.reply.md" | tr -d ' ') bytes; cooldown ${delay}s"
    until_ts=$(( $(date -u +%s) + delay ))
  else
    delay=$((delay * 2)); [ "$delay" -gt "$MAX_BACKOFF" ] && delay=$MAX_BACKOFF
    log "shift $id ended rc=$rc${reason:+ ($reason)}; backoff ${delay}s"
    until_ts=$(( $(date -u +%s) + delay ))
  fi
  while ! stop_wanted; do
    left=$(( until_ts - $(date -u +%s) ))
    [ "$left" -gt 0 ] || break
    nap $(( left < TICK ? left : TICK ))
  done
done
log "runner stop: ${stopping:-$HOME_DIR/stop exists}"
}

main "$@"

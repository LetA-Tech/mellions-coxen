#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The runner keeps shifts going and stops when told: shifts back to back
# through the real shift.sh with a stub session, one runner per home, pause and
# resume said once each, a backoff that doubles while shifts fail and resets on
# a good one, the previous reply in the next prompt, the method cadence, the
# stop file, a stop signal that reaches the session, the daily cap, a stale
# lock, an update that fails or succeeds without ever blocking the shift and
# installs the binary and nothing else, and a refusal to run against a checkout
# the runtime does not load the plugin from. No real session runs here.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
runner="$root/scripts/shifts.sh"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
runners=""
cleanup() {
  local p
  for p in $runners; do kill -TERM "$p" 2>/dev/null; done
  sleep 0.3
  for p in $runners; do kill -KILL "$p" 2>/dev/null; done
  if [ -s "$tmp/stub/hang.pid" ]; then
    p=$(cat "$tmp/stub/hang.pid"); pkill -P "$p" 2>/dev/null; kill "$p" 2>/dev/null
  fi
  [ "${sleeper:-0}" -gt 0 ] && kill "$sleeper" 2>/dev/null
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM HUP

export STUB_DIR="$tmp/stub"; mkdir -p "$STUB_DIR"
# The reset time the account's refusal names, an hour out rather than a fixed
# clock value. A literal is in the past for part of every day, and a window
# that has already passed rolls to tomorrow and trips the runner's far-window
# clamp — a different branch from the one the K assertions are about, so the
# suite failed for everyone after that hour and passed before it. An hour out
# is stable at every clock position, including across midnight, because the
# parser rolls a past HH:MM forward by exactly one day.
STUB_RESET_24=$(date -u -d '+1 hour' '+%H:%M' 2>/dev/null || date -u -v+1H '+%H:%M')
STUB_RESET_12=$(date -u -d '+1 hour' '+%-I:%M%P' 2>/dev/null || date -u -v+1H '+%-I:%M%p' | tr 'A-Z' 'a-z')
export STUB_RESET_24 STUB_RESET_12
# The session: one result event, or a failure, or a hang — chosen by a file.
cat > "$STUB_DIR/claude" <<'STUB'
#!/usr/bin/env bash
cat > /dev/null
[ -e "$STUB_DIR/fail" ] && exit 1
# A runtime usage-limit refusal carries a session id, reset time and non-zero exit.
if [ -e "$STUB_DIR/limit" ]; then
  printf '{"type":"system","session_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}\n'
  printf '{"type":"result","result":"You'\''ve hit your session limit · resets %s (UTC)"}\n' "$STUB_RESET_12"
  exit 1
fi
# A session that quotes the sentence and ends of its own accord — a shift
# working on this defect does exactly this.
if [ -e "$STUB_DIR/limit-quoted" ]; then
  printf '{"type":"system","session_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}\n'
  printf '{"type":"result","result":"Handled the refusal: You'\''ve hit your session limit · resets 6:10pm (UTC)"}\n'
  exit 0
fi
# A session killed at its timeout that had quoted the sentence: non-zero exit,
# the sentence in the stream, and no result event, so no reply.
if [ -e "$STUB_DIR/limit-killed" ]; then
  printf '{"type":"system","session_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}\n'
  printf '{"type":"assistant","message":{"content":[{"type":"text","text":"quoting: You'\''ve hit your session limit · resets 6:10pm (UTC)"}]}}\n'
  exit 124
fi
# A refusal naming a window further off than a usage window runs — what a
# reset time that has just passed rolls to, and what a misparse can produce.
if [ -e "$STUB_DIR/limit-far" ]; then
  far=$(date -u -d '+8 hours' +%H:%M 2>/dev/null || date -u -v+8H +%H:%M)
  printf '{"type":"system","session_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}\n'
  printf '{"type":"result","result":"You'\''ve hit your session limit · resets %s (UTC)"}\n' "$far"
  exit 1
fi
# A refusal naming a time in no zone this script can resolve.
if [ -e "$STUB_DIR/limit-nozone" ]; then
  printf '{"type":"system","session_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}\n'
  printf '{"type":"result","result":"You'\''ve hit your session limit · resets 6:10pm"}\n'
  exit 1
fi
if [ -e "$STUB_DIR/hang" ]; then echo $$ > "$STUB_DIR/hang.pid"; sleep 3131; exit 0; fi
# Stands in for the session itself filing a report mid-shift: a claim
# (O_CREATE|O_EXCL leaves an empty .md) that a crash left unwritten, or a real
# one — both land after the survey's mtime, as they would from a live session.
if [ -e "$STUB_DIR/claim-empty" ]; then mkdir -p "$MELLIONS_HOME/reports"; : > "$MELLIONS_HOME/reports/claimed.md"; fi
if [ -e "$STUB_DIR/claim-real" ]; then mkdir -p "$MELLIONS_HOME/reports"; printf '# real\n\n## What I did\n\nfiled for real\n' > "$MELLIONS_HOME/reports/claimed.md"; fi
printf '{"type":"result","result":"ready — the stub shift replied"}\n'
STUB
# The binary: records every call; the survey it prints reaches the prompt.
cat > "$STUB_DIR/mellions" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "$STUB_DIR/mellions.calls"
[ "$1" = survey ] && echo "# stub survey: $*"
# The home the scripts ask for. Every scenario below sets MELLIONS_HOME, so
# answering with it is what the real binary answers for the same environment.
if [ "$1" = config ]; then
  case "$2" in
    home)    echo "$MELLIONS_HOME" ;;
    reports) echo "$MELLIONS_HOME/reports" ;;
  esac
fi
exit 0
STUB
chmod +x "$STUB_DIR/claude" "$STUB_DIR/mellions"

# Nothing of the caller's environment reaches the stub shifts. The runner runs
# `make check` from inside its own environment, where MELLIONS_PROMPT,
# MELLIONS_HOME and the rest are set for real shifts; inherited here they turn
# the stub shifts into directed tasks and fail every self-update on a host that
# runs the runner.
for v in $(env | sed -n 's/^\(MELLIONS_[A-Za-z0-9_]*\)=.*/\1/p'); do unset "$v"; done
export CLAUDE_BIN="$STUB_DIR/claude" MELLIONS_BIN="$STUB_DIR/mellions"
export MELLIONS_COOLDOWN=1s MELLIONS_TICK=1 MELLIONS_AUTOUPDATE=0 MELLIONS_TIMEOUT=60
export MELLIONS_SHIFTS_PER_DAY=50 MELLIONS_METHOD_EVERY=4 MELLIONS_BUDGET=1m

# The runtime's marketplace record, which names the tree sessions load the
# plugin from; the runner refuses any other checkout. Written per scenario.
export CLAUDE_CONFIG_DIR="$tmp/claude"; mkdir -p "$CLAUDE_CONFIG_DIR/plugins"
record() { printf '{"mellions":{"source":{"source":"directory","path":"%s"}}}\n' "$1" > "$CLAUDE_CONFIG_DIR/plugins/known_marketplaces.json"; }
record "$root"

wait_for() {    # wait_for <seconds> <pattern> <file>
  local end=$(( $(date +%s) + $1 ))
  until grep -q -- "$2" "$3" 2>/dev/null; do
    [ "$(date +%s)" -lt "$end" ] || return 1
    sleep 0.2
  done
}
count() { local c; c=$(grep -c -- "$1" "$2" 2>/dev/null); echo "${c:-0}"; }
wait_count() {  # wait_count <seconds> <n> <pattern> <file>
  local end=$(( $(date +%s) + $1 ))
  while [ "$(count "$3" "$4")" -lt "$2" ]; do
    [ "$(date +%s)" -lt "$end" ] || return 1
    sleep 0.2
  done
}
wait_gone() {   # wait_gone <seconds> <pid>
  local end=$(( $(date +%s) + $1 ))
  while kill -0 "$2" 2>/dev/null; do
    [ "$(date +%s)" -lt "$end" ] || return 1
    sleep 0.2
  done
}
start_runner() {  # start_runner <home> ["VAR=v VAR=v"]; leaves the pid in $pid
  env ${2:-} MELLIONS_HOME="$1" "$runner" > "$1.out" 2>&1 &
  pid=$!
  runners="$runners $pid"
}

# ---- A. one runner's life ----------------------------------------------------
home="$tmp/a"; mkdir -p "$home"; log="$home/shifts/runner.log"
start_runner "$home"; a=$pid
wait_for 5 'runner start' "$log" || bad "A: the runner never started"

# A second runner for the same home, run with a deadline: one that is not
# refused loops forever, and the assertion has to fail rather than hang.
MELLIONS_HOME="$home" "$runner" > "$home.second" 2>&1 & second=$!
if wait_gone 5 "$second"; then
  wait "$second"; rc=$?
  { [ "$rc" -eq 0 ] && grep -q "already alive here — pid $a" "$home.second"; } || bad "A: a second runner was not refused (rc=$rc): $(cat "$home.second")"
else
  kill -KILL "$second" 2>/dev/null; bad "A: a second runner was not refused — it is still running"
fi
[ "$(count 'runner start' "$log")" -eq 1 ] || bad "A: the refused runner wrote to the log"

wait_count 20 2 'ended rc=0' "$log" || bad "A: two shifts did not complete: $(tail -5 "$log")"
first=$(ls "$home"/shifts/*.prompt.md | sed -n 1p); second=$(ls "$home"/shifts/*.prompt.md | sed -n 2p)
grep -q 'THE PREVIOUS SHIFT SAID' "$first" && bad "A: the first prompt carries a previous reply that cannot exist"
grep -q '=== THE PREVIOUS SHIFT SAID ===' "$second" || bad "A: the second prompt does not carry the previous reply"
grep -q 'the stub shift replied' "$second" || bad "A: the previous reply's text is not in the second prompt"
grep -q '^# stub survey: survey -save$' "$first" || bad "A: an ordinary shift's survey got extra arguments"
grep -q 'cooldown 1s' "$log" || bad "A: a good shift did not log the cooldown"

touch "$home/pause"
wait_for 5 'paused:' "$log" || bad "A: pause was not noticed"
before=$(count 'starting' "$log"); sleep 3
[ "$(count 'starting' "$log")" -eq "$before" ] || bad "A: a shift started while paused"
[ "$(count 'paused:' "$log")" -eq 1 ] || bad "A: paused was logged more than once"
rm "$home/pause"
wait_for 5 'resumed:' "$log" || bad "A: resume was not noticed"
wait_count 10 $((before + 1)) 'starting' "$log" || bad "A: no shift started after resume"
# The shift resume just started has to actually finish before the fail switch
# below is flipped. Touching the switch right after "starting" races that
# shift's own (backgrounded) claude call against this script: whichever
# reaches the switch first decides whether the shift counts as good or as a
# third failure, which is why the count below used to read 3 or 4 good shifts
# on different runs of the same code (#180) with nothing to notice either way.
wait_count 15 3 'ended rc=0' "$log" || bad "A: the shift after resume did not complete: $(tail -3 "$log")"

touch "$STUB_DIR/fail"
wait_for 15 'rc=1 (empty reply); backoff 2s' "$log" || bad "A: the first failed shift did not back off to 2s: $(tail -3 "$log")"
wait_for 15 'backoff 4s' "$log" || bad "A: the second failed shift did not double to 4s"
rm "$STUB_DIR/fail"
ok=$(count 'ended rc=0' "$log")
[ "$ok" -eq 3 ] || bad "A: expected 3 good shifts before the forced failures, got $ok: $(tail -10 "$log")"
wait_count 25 $((ok + 1)) 'ended rc=0' "$log" || bad "A: no good shift after the failures"
[ "$(count 'cooldown 1s' "$log")" -ge $((ok + 1)) ] || bad "A: a good shift did not reset the delay to the cooldown"

grep -q 'shift 4 of 50 today starting — method shift: survey -repos mellions-coxen' "$log" || bad "A: the fourth shift was not the method shift"
grep -lq 'stub survey: survey -save -repos mellions-coxen' "$home"/shifts/*.prompt.md || bad "A: no prompt carries the scoped survey"

touch "$home/stop"
wait_for 20 'runner stop:' "$log" || bad "A: the stop file did not end the loop"
wait_gone 5 "$a" || bad "A: the runner is still alive after stop"
[ -e "$home/shifts/runner.lock" ] && bad "A: the lock survived the stop"
grep -q "runner stop: $home/stop exists" "$log" || bad "A: the stop line does not say why"
good=$(count 'ended rc=0' "$log"); failed=$(count 'rc=1' "$log")
[ "$good" -eq 4 ] || bad "A: expected 4 good shifts total, got $good: $(tail -10 "$log")"
[ "$failed" -eq 2 ] || bad "A: expected 2 failed shifts total, got $failed: $(tail -10 "$log")"
note "A: $good good shifts, $failed failed, one pause, one stop"

# ---- B. the daily cap, counted from the files ----------------------------------
home="$tmp/b"; mkdir -p "$home/shifts"; log="$home/shifts/runner.log"
day=$(date -u +%Y%m%d); : > "$home/shifts/$day-000001.log"; : > "$home/shifts/$day-000002.log"
start_runner "$home" "MELLIONS_SHIFTS_PER_DAY=2"; b=$pid
wait_for 5 'cap reached: 2 of 2' "$log" || bad "B: the cap was not noticed"
sleep 2
grep -q 'starting' "$log" && bad "B: a shift started over the cap"
kill -TERM "$b"
wait_gone 5 "$b" || bad "B: SIGTERM did not end a runner waiting at the cap"
grep -q 'runner stop: signal' "$log" || bad "B: the signal stop was not logged"
[ -e "$home/shifts/runner.lock" ] && bad "B: the lock survived the signal"

# ---- C. a stop signal reaches the session ---------------------------------------
home="$tmp/c"; mkdir -p "$home"; log="$home/shifts/runner.log"
touch "$STUB_DIR/hang"
start_runner "$home"; c=$pid
end=$(( $(date +%s) + 10 ))
while [ ! -s "$STUB_DIR/hang.pid" ] && [ "$(date +%s)" -lt "$end" ]; do sleep 0.2; done
[ -s "$STUB_DIR/hang.pid" ] || bad "C: the hung session never started"
hung=$(cat "$STUB_DIR/hang.pid" 2>/dev/null || echo 0)
sleep 0.3
sleeper=$(pgrep -P "$hung" 2>/dev/null | head -1); sleeper=${sleeper:-0}
kill -TERM "$c"
wait_gone 10 "$c" || bad "C: SIGTERM did not end the runner within 10s while a shift ran"
sleep 0.3
kill -0 "$hung" 2>/dev/null && bad "C: the session (pid $hung) outlived the stop"
kill -0 "$sleeper" 2>/dev/null && bad "C: the session's child (pid $sleeper) outlived the stop"
grep -q 'after the stop signal' "$log" || bad "C: the shift's end after the signal was not logged: $(tail -3 "$log")"
grep -q 'runner stop: signal' "$log" || bad "C: the signal stop was not logged"
# Whatever the verdict, nothing of the hung session outlives this test.
kill "$sleeper" "$hung" 2>/dev/null
rm -f "$STUB_DIR/hang"

# ---- D. an update that fails never blocks the shift ------------------------------
home="$tmp/d"; mkdir -p "$home"; log="$home/shifts/runner.log"
co="$tmp/co-bad"; mkdir -p "$co"; git -C "$co" init -q; record "$co"
start_runner "$home" "MELLIONS_AUTOUPDATE=1 MELLIONS_CHECKOUT=$co MELLIONS_SHIFTS_PER_DAY=1"; d=$pid
wait_for 10 'update failed at git pull --ff-only' "$log" || bad "D: a failing update was not logged"
wait_for 15 'ended rc=0' "$log" || bad "D: the shift did not run after the failed update"
touch "$home/stop"; wait_gone 10 "$d" || bad "D: the runner did not stop"

# ---- E. an update that succeeds: pull, build, check, the binary and nothing else
home="$tmp/e"; mkdir -p "$home"; log="$home/shifts/runner.log"
origin="$tmp/origin.git"; git init -q --bare "$origin"
co="$tmp/co"; git clone -q "$origin" "$co" 2>/dev/null
printf 'build:\n\t@mkdir -p bin && cp "$$STUB_DIR/mellions" bin/mellions\ncheck:\n\t@echo checked\n' > "$co/Makefile"
git -C "$co" -c user.name=t -c user.email=t@t add Makefile
git -C "$co" -c user.name=t -c user.email=t@t commit -q -m one
git -C "$co" push -q -u origin HEAD 2>/dev/null
sha=$(git -C "$co" rev-parse --short HEAD)
mkdir -p "$tmp/bin"; cp "$STUB_DIR/mellions" "$tmp/bin/mellions"; record "$co"
start_runner "$home" "MELLIONS_AUTOUPDATE=1 MELLIONS_CHECKOUT=$co MELLIONS_BIN=$tmp/bin/mellions MELLIONS_SHIFTS_PER_DAY=2"; e=$pid
wait_for 15 "update ok: $sha" "$log" || bad "E: a good update was not logged: $(tail -3 "$log")"
[ "$(cat "$home/shifts/runner.installed" 2>/dev/null)" = "$sha" ] || bad "E: the installed sha was not recorded"
[ -x "$tmp/bin/mellions" ] && [ ! -e "$tmp/bin/mellions.new" ] && cmp -s "$co/bin/mellions" "$tmp/bin/mellions" || bad "E: the binary was not replaced cleanly with the checkout's build"
grep -q '^install' "$STUB_DIR/mellions.calls" && bad "E: the runner called mellions install — the checkout is the plugin, and registering another tree is what re-pointed a live host at a lane"
wait_count 20 2 'ended rc=0' "$log" || bad "E: two shifts did not run through the update: $(tail -3 "$log")"
grep -q "update: $sha is what runs already" "$log" || bad "E: an unchanged checkout was built again"
touch "$home/stop"; wait_gone 10 "$e" || bad "E: the runner did not stop"

# ---- F. a lock left by a dead runner is taken over -------------------------------
home="$tmp/f"; mkdir -p "$home/shifts"; log="$home/shifts/runner.log"; record "$root"
sleep 0.01 & dead=$!; wait "$dead"
echo "$dead" > "$home/shifts/runner.lock"
start_runner "$home" "MELLIONS_SHIFTS_PER_DAY=0"; f=$pid
wait_for 5 'runner start' "$log" || bad "F: a stale lock kept the runner out"
grep -q 'stale lock replaced' "$log" || bad "F: the stale lock was not reported"
[ "$(cat "$home/shifts/runner.lock")" = "$f" ] || bad "F: the lock does not hold the new runner's pid"
kill -TERM "$f"; wait_gone 5 "$f" || bad "F: the runner did not stop"

# ---- G. a checkout the runtime does not load from is refused ---------------------
# Started with a deadline, as in A: a runner that is not refused loops, and
# the assertion has to fail rather than hang.
home="$tmp/g"; mkdir -p "$home"; log="$home/shifts/runner.log"
refused() {  # refused <label> <pattern the log must carry>
  start_runner "$home" "MELLIONS_SHIFTS_PER_DAY=0"; local g=$pid rc
  if ! wait_gone 5 "$g"; then kill -KILL "$g" 2>/dev/null; bad "G: $1 — the runner was not refused, it is still running"; return; fi
  wait "$g"; rc=$?
  [ "$rc" -eq 2 ] || bad "G: $1 — refused with rc=$rc, not 2: $(cat "$home.out")"
  grep -q -- "$2" "$log" || bad "G: $1 — the refusal does not say why: $(cat "$home.out")"
}
mkdir -p "$tmp/elsewhere"; record "$tmp/elsewhere"
refused "another tree recorded" "refusing to start: MELLIONS_CHECKOUT=$root is not the tree the runtime loads the plugin from ($tmp/elsewhere)"
rm -f "$CLAUDE_CONFIG_DIR/plugins/known_marketplaces.json"
refused "no record at all" 'no marketplace record at'
[ -e "$home/shifts/runner.lock" ] && bad "G: a refused runner took the lock"
grep -q 'runner start' "$log" && bad "G: a refused runner started"
record "$root"

# ---- H. the report backstop is not suppressed by a claimed-but-unwritten report -
# #179: `report write` claims its name (O_CREATE|O_EXCL) before writing the
# content, so a claim a crash interrupted is an empty .md, never a filed
# report; the backstop must still file the reply. A real report must still
# suppress it, or the fix becomes "always file twice".
report_calls() { count '^report write -did' "$STUB_DIR/mellions.calls"; }

home="$tmp/h1"; mkdir -p "$home"
touch "$STUB_DIR/claim-empty"
before=$(report_calls)
MELLIONS_HOME="$home" "$root/scripts/shift.sh" > "$home.out" 2>&1
[ "$(report_calls)" -eq $((before + 1)) ] || bad "H: an empty claimed report suppressed the backstop: $(cat "$home.out")"
rm -f "$STUB_DIR/claim-empty"

home="$tmp/h2"; mkdir -p "$home"
touch "$STUB_DIR/claim-real"
before=$(report_calls)
MELLIONS_HOME="$home" "$root/scripts/shift.sh" > "$home.out" 2>&1
[ "$(report_calls)" -eq "$before" ] || bad "H: a real filed report did not suppress the backstop: $(cat "$home.out")"
rm -f "$STUB_DIR/claim-real"

# ---- I. attended or away -------------------------------------------------------
# The two arms the state exists for: the runner starts a shift when the owner
# says they are away, and starts none while they say they are here. Both against
# one runner, so what changed between them is the marker and nothing else.
home="$tmp/i"; mkdir -p "$home/shifts"; log="$home/shifts/runner.log"
printf 'state: back\nsince: 2026-08-29T07:00:00Z\n' > "$home/owner"
start_runner "$home"; i=$pid
wait_for 5 'attended:' "$log" || bad "I: the runner did not notice the owner was here: $(tail -3 "$log")"
sleep 2
grep -q 'starting' "$log" && bad "I: a shift started while the owner was at the keyboard"
grep -q 'no owner marker' "$log" && bad "I: a marker that exists was reported missing"

printf 'state: away\nsince: 2026-08-29T22:10:00Z\n' > "$home/owner"
wait_for 10 'away: .* shifts run back to back' "$log" || bad "I: the runner did not notice the owner had left: $(tail -3 "$log")"
wait_for 20 'ended rc=0' "$log" || bad "I: no shift ran while the owner was away: $(tail -5 "$log")"

# An away window that has run out is attended again, without anybody saying so.
printf 'state: away\nsince: 2026-08-29T22:10:00Z\nuntil: 2000-01-01T00:00:00Z\n' > "$home/owner"
wait_for 20 'attended: .* says the owner is here' "$log" || bad "I: a lapsed away window went on running shifts: $(tail -3 "$log")"
kill -TERM "$i"; wait_gone 10 "$i" || bad "I: SIGTERM did not end a runner waiting on the owner"

# A configured night window runs whether or not anybody said they were leaving.
# The windows are chosen to cover always or never whatever the clock says here:
# 00:00-24:00 covers every HH:MM, 12:00-12:00 is the same thing through the
# wrap-past-midnight branch, and 24:00-24:01 covers no clock time there is.
for w in 00:00-24:00 12:00-12:00; do
  home="$tmp/i-$w"; mkdir -p "$home/shifts"; log="$home/shifts/runner.log"
  printf 'state: back\nsince: 2026-08-29T07:00:00Z\n' > "$home/owner"
  start_runner "$home" "MELLIONS_NIGHT_WINDOW=$w"; nw=$pid
  wait_for 20 'ended rc=0' "$log" || bad "I: night window $w ran no shift on an attended host: $(tail -5 "$log")"
  kill -TERM "$nw"; wait_gone 10 "$nw" || bad "I: the $w runner did not stop"
done

home="$tmp/i-out"; mkdir -p "$home/shifts"; log="$home/shifts/runner.log"
printf 'state: back\nsince: 2026-08-29T07:00:00Z\n' > "$home/owner"
start_runner "$home" "MELLIONS_NIGHT_WINDOW=24:00-24:01"; iout=$pid
wait_for 5 'outside the night window' "$log" || bad "I: a window covering no time still allowed the hour: $(tail -3 "$log")"
sleep 2
grep -q 'starting' "$log" && bad "I: a shift started outside the night window on an attended host"
kill -TERM "$iout"; wait_gone 10 "$iout" || bad "I: the outside-window runner did not stop"

# A host whose owner has never said either is not "attended": every installation
# is in that state until the first `mellions away`, and inferring presence from
# a missing file would stop a runner that has been working for months.
home="$tmp/i-none"; mkdir -p "$home/shifts"; log="$home/shifts/runner.log"
start_runner "$home"; inone=$pid
wait_for 20 'ended rc=0' "$log" || bad "I: a host with no owner marker stopped running shifts: $(tail -5 "$log")"
[ "$(count 'no owner marker' "$log")" -eq 1 ] || bad "I: the missing marker was said $(count 'no owner marker' "$log") times, not once"
kill -TERM "$inone"; wait_gone 10 "$inone" || bad "I: the unmarked runner did not stop"
note "I: away runs, attended holds, a lapsed window holds, a night window runs, no marker runs as before"

# ---- J. one home per host, and it is the binary's ------------------------------
# #203: the script defaulted to $HOME/mellions while the binary resolves its
# home from the config. Where the two differ — every installation whose
# report_root is not under $HOME/mellions — a hand-launched shift wrote a
# second, configless home: `report digest` and `doctor` read the installation's
# and never saw it, and the backstop looked for the session's report in a
# directory `report write` does not write to, so it filed the reply as a second
# report every time.
#
# The real binary, not the stub: the whole claim is that the script asks the
# binary and takes its answer. A stub that answered would be testing the stub.
real="$tmp/real/mellions"; mkdir -p "$tmp/real"
if ! (cd "$root" && go build -o "$real" ./cmd/mellions) 2>"$tmp/build.err"; then
  bad "J: the binary the script asks does not build: $(cat "$tmp/build.err")"
else
# The session stub for this section: it records the home it was handed, and
# files a report through the real binary when told to, which is what a session
# that has something to say does mid-shift.
cat > "$STUB_DIR/claude-j" <<'STUB'
#!/usr/bin/env bash
cat > /dev/null
printf '%s\n' "${MELLIONS_HOME-<unset>}" >> "$STUB_DIR/j.home"
[ -e "$STUB_DIR/j-report" ] && "$REAL_MELLIONS" report write -did "the session filed its own" >/dev/null 2>&1
printf '{"type":"result","result":"ready — the stub shift replied"}\n'
STUB
chmod +x "$STUB_DIR/claude-j"
export REAL_MELLIONS="$real"

# A directed task, so the survey is skipped and the real binary collects no
# source and reaches no network here.
printf 'do the thing\n' > "$tmp/task.md"
j_config() {   # j_config <state dir> -> config path
  mkdir -p "$1"
  printf '{"owner":"test","assignments_root":"%s/assignments","report_root":"%s"}\n' "$1" "$1" > "$1.json"
  printf '%s\n' "$1.json"
}

# J1: MELLIONS_HOME unset, the config naming a root that is not under $HOME.
jhome="$tmp/j1/home"; jstate="$tmp/j1/state"; mkdir -p "$jhome"
jcfg=$(j_config "$jstate")
: > "$STUB_DIR/j.home"
env -u MELLIONS_HOME HOME="$jhome" MELLIONS_CONFIG="$jcfg" \
    MELLIONS_BIN="$real" CLAUDE_BIN="$STUB_DIR/claude-j" MELLIONS_PROMPT="$tmp/task.md" \
    "$root/scripts/shift.sh" > "$tmp/j1.out" 2>&1
ls "$jstate"/shifts/*.log >/dev/null 2>&1 \
  || bad "J1: no shift under the configured root $jstate: $(cat "$tmp/j1.out")"
# The negative control: the old default must be untouched. Without it the
# assertion above passes on a script that writes to both.
[ -e "$jhome/mellions" ] \
  && bad "J1: a second home under \$HOME: $(find "$jhome/mellions" | head -5 | tr '\n' ' ')"
grep -qx "$jstate" "$STUB_DIR/j.home" \
  || bad "J1: the session was handed MELLIONS_HOME=$(cat "$STUB_DIR/j.home"), not $jstate"
[ "$(ls "$jstate"/reports/*.md 2>/dev/null | wc -l)" -eq 1 ] \
  || bad "J1: the backstop filed $(ls "$jstate"/reports/*.md 2>/dev/null | wc -l) reports where the session filed none"

# J2: MELLIONS_HOME set to somewhere that is not the report root. It still
# overrides where the shift's own files land — and the report the session wrote
# is under the report root whatever it says, so a backstop looking in
# $MELLIONS_HOME/reports finds nothing and files the reply a second time.
jhome="$tmp/j2/home"; jstate="$tmp/j2/state"; jscratch="$tmp/j2/scratch"
mkdir -p "$jhome" "$jscratch"
jcfg=$(j_config "$jstate")
: > "$STUB_DIR/j.home"; touch "$STUB_DIR/j-report"
env HOME="$jhome" MELLIONS_HOME="$jscratch" MELLIONS_CONFIG="$jcfg" \
    MELLIONS_BIN="$real" CLAUDE_BIN="$STUB_DIR/claude-j" MELLIONS_PROMPT="$tmp/task.md" \
    "$root/scripts/shift.sh" > "$tmp/j2.out" 2>&1
rm -f "$STUB_DIR/j-report"
ls "$jscratch"/shifts/*.log >/dev/null 2>&1 \
  || bad "J2: MELLIONS_HOME no longer overrides where a shift lands: $(cat "$tmp/j2.out")"
ls "$jstate"/shifts/*.log >/dev/null 2>&1 \
  && bad "J2: the shift landed under the report root as well as under MELLIONS_HOME"
n=$(ls "$jstate"/reports/*.md 2>/dev/null | wc -l)
[ "$n" -eq 1 ] || bad "J2: $n reports under $jstate/reports; the session filed one and the backstop did not see it"
grep -q 'filing its reply as one' "$jscratch"/shifts/*.log \
  && bad "J2: the backstop read a filed report as missing"
# J3: the binary cannot answer — no config, a config that will not parse, or a
# binary older than the verbs, which is what an update that pulled and failed
# to install leaves behind. There is no safe guess: kept to itself it splits
# the script's files from the report the session writes, handed to the session
# it is silently wrong in every `mellions state` the awareness hook runs. So
# the shift refuses, before it has written anything and before it has run an
# hour it could not file.
jhome="$tmp/j3/home"; jstate="$tmp/j3/state"; mkdir -p "$jhome"
jcfg=$(j_config "$jstate")
cat > "$STUB_DIR/mellions-old" <<'STUB'
#!/usr/bin/env bash
[ "$1" = config ] && { echo "mellions: config: unknown verb \"$2\"" >&2; exit 1; }
exit 0
STUB
chmod +x "$STUB_DIR/mellions-old"
: > "$STUB_DIR/j.home"
env -u MELLIONS_HOME HOME="$jhome" MELLIONS_CONFIG="$jcfg" \
    MELLIONS_BIN="$STUB_DIR/mellions-old" CLAUDE_BIN="$STUB_DIR/claude-j" MELLIONS_PROMPT="$tmp/task.md" \
    "$root/scripts/shift.sh" > "$tmp/j3.out" 2>&1
rc=$?
[ "$rc" -eq 2 ] || bad "J3: a binary that cannot say where the home is did not refuse (rc=$rc): $(cat "$tmp/j3.out")"
grep -q 'cannot say where this installation.s home is' "$tmp/j3.out" \
  || bad "J3: the refusal does not say why: $(cat "$tmp/j3.out")"
[ -s "$STUB_DIR/j.home" ] && bad "J3: a session was started with no home to put it in"
[ -e "$jhome/mellions" ] && bad "J3: the refused shift still wrote a second home under \$HOME"
[ -e "$jstate/shifts" ] && bad "J3: the refused shift still wrote under the configured root"
note "J: the home is the binary's, MELLIONS_HOME still overrides, the session inherits it, the backstop reads the report root, and a binary that cannot answer is refused rather than guessed at"
fi

# ---- K. a shift the account refused ------------------------------------------
# #228: five shifts ended on "You've hit your session limit · resets 6:10pm
# (UTC)" and were filed as complete, that sentence going in as each shift's
# owner-facing report, while the runner counted them against the cap and
# started the next one into the same closed window.
khome="$tmp/k/home"; mkdir -p "$khome/assignments/lane-x"
printf 'do the thing\n' > "$tmp/k-task.md"
# A lane whose record names the session the refused shift ran under. Found by
# the session id, which is the only thing tying a shift to the lane it cut.
printf '{"id":"lane-x","session":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}\n' \
  > "$khome/assignments/lane-x/assignment.json"
# A lane belonging to some other session: the negative control for the grep.
mkdir -p "$khome/assignments/lane-y"
printf '{"id":"lane-y","session":"99999999-8888-7777-6666-555555555555"}\n' \
  > "$khome/assignments/lane-y/assignment.json"

touch "$STUB_DIR/limit"
: > "$STUB_DIR/mellions.calls"
env MELLIONS_HOME="$khome" MELLIONS_PROMPT="$tmp/k-task.md" "$root/scripts/shift.sh" > "$tmp/k1.out" 2>&1
rc=$?
klog=$(ls "$khome"/shifts/*.log 2>/dev/null | head -1)
kmark=$(ls "$khome"/shifts/*.limit 2>/dev/null | head -1)
[ "$rc" -eq 3 ] || bad "K1: a refused shift exited $rc, not 3: $(cat "$tmp/k1.out")"
grep -q "interrupted by the usage limit, resumes after $STUB_RESET_24 UTC" "$klog" 2>/dev/null \
  || bad "K1: the log does not say the shift was interrupted: $(cat "$klog" 2>/dev/null | tail -5)"
grep -q 'complete' "$klog" 2>/dev/null \
  && bad "K1: the refused shift is still filed as complete: $(tail -3 "$klog")"
[ -n "$kmark" ] || bad "K1: no window marker under $khome/shifts for the runner to read"
if [ -n "$kmark" ]; then
  read -r kts kat ksid _ < "$kmark"
  [ "$kat" = "$STUB_RESET_24" ] || bad "K1: the marker names the reset time as '$kat', not $STUB_RESET_24"
  [ "$kts" -gt "$(date -u +%s)" ] 2>/dev/null \
    || bad "K1: the marker's epoch $kts is not in the future"
  [ "$ksid" = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" ] \
    || bad "K1: the marker names the session as '$ksid'"
fi
grep -q 'report write .*interrupted by the account.s usage limit' "$STUB_DIR/mellions.calls" \
  || bad "K1: the owner-facing report does not say the shift was interrupted: $(grep 'report write' "$STUB_DIR/mellions.calls")"
# The old behaviour, which this replaces: the refusal filed verbatim as the
# shift's report. Without this the assertion above passes on a script that
# files both.
grep -q "report write -did You've hit your session limit" "$STUB_DIR/mellions.calls" \
  && bad "K1: the refusal sentence is still filed as the shift's own report"
grep -q 'assign record lane-x -kind next .*usage limit' "$STUB_DIR/mellions.calls" \
  || bad "K1: the lane the refused session held was not marked: $(grep 'assign record' "$STUB_DIR/mellions.calls")"
grep -q 'assign record lane-y' "$STUB_DIR/mellions.calls" \
  && bad "K1: a lane belonging to another session was marked interrupted"

# K2: the same sentence, from a session that chose how it ended. A shift
# working on this very defect quotes the refusal into its reply and its stream;
# only the account's refusal also exits non-zero.
rm -f "$STUB_DIR/limit"; touch "$STUB_DIR/limit-quoted"
k2home="$tmp/k2/home"; mkdir -p "$k2home"
: > "$STUB_DIR/mellions.calls"
env MELLIONS_HOME="$k2home" MELLIONS_PROMPT="$tmp/k-task.md" "$root/scripts/shift.sh" > "$tmp/k2.out" 2>&1
rc=$?
rm -f "$STUB_DIR/limit-quoted"
[ "$rc" -eq 0 ] || bad "K2: a session that merely quoted the refusal exited $rc: $(cat "$tmp/k2.out")"
ls "$k2home"/shifts/*.limit >/dev/null 2>&1 \
  && bad "K2: a session that quoted the refusal was filed as refused by the account"
grep -q 'complete' "$k2home"/shifts/*.log 2>/dev/null \
  || bad "K2: a shift that ended normally is no longer filed as complete"

# K3: the runner reads the window. The cap is 1 and one shift has run, so a
# runner that counted it would be saying "cap reached"; one that read rc alone
# would be doubling its backoff.
k3home="$tmp/k3/home"; mkdir -p "$k3home"
touch "$STUB_DIR/limit"
start_runner "$k3home" "MELLIONS_SHIFTS_PER_DAY=1"; k3=$pid
log="$k3home.out"
wait_for 25 "refused by the account.s usage limit, which resets at $STUB_RESET_24 UTC" "$log" \
  || bad "K3: the runner did not read the window: $(tail -5 "$log")"
grep -qE 'no shift starts for [0-9]+s' "$log" \
  || bad "K3: the runner did not say how long it waits: $(tail -3 "$log")"
grep -q 'backoff' "$log" && bad "K3: a refused shift grew the failure backoff: $(tail -3 "$log")"
kill -TERM "$k3" 2>/dev/null; wait_gone 10 "$k3"
rm -f "$STUB_DIR/limit"

# K4: the cap counts work done, and a refused shift did none. Asserted on a day
# whose shifts are already on disk rather than after K3's refusal, because a
# runner waiting out a window never reaches the cap again and any assertion
# there passes whether the count skips the shift or not.
kday=$(date -u +%Y%m%d)
k4home="$tmp/k4/home"; mkdir -p "$k4home/shifts"
: > "$k4home/shifts/$kday-000001.log"
printf '0 - -\n' > "$k4home/shifts/$kday-000001.limit"
start_runner "$k4home" "MELLIONS_SHIFTS_PER_DAY=1"; k4=$pid
log="$k4home.out"
wait_for 25 'ended rc=0' "$log" \
  || bad "K4: a day whose one shift the account refused was counted as spent: $(tail -5 "$log")"
grep -q 'cap reached' "$log" && bad "K4: the refused shift was charged to the cap: $(tail -3 "$log")"
kill -TERM "$k4" 2>/dev/null; wait_gone 10 "$k4"

# The control for K4: the same day, the same cap, a shift that actually ran.
# Without it K4 passes on a count that skips every shift.
k5home="$tmp/k5/home"; mkdir -p "$k5home/shifts"
: > "$k5home/shifts/$kday-000001.log"
start_runner "$k5home" "MELLIONS_SHIFTS_PER_DAY=1"; k5=$pid
log="$k5home.out"
wait_for 25 'cap reached: 1 of 1' "$log" \
  || bad "K5: an ordinary shift no longer counts against the cap: $(tail -5 "$log")"
grep -q 'ended rc=0' "$log" && bad "K5: a shift ran on a day the cap was already spent: $(tail -3 "$log")"
kill -TERM "$k5" 2>/dev/null; wait_gone 10 "$k5"
# K6: a session the timeout killed, which had quoted the sentence. It exits
# non-zero like a refusal and its stream holds the sentence like a refusal; what
# it does not have is a result event, so it has no reply. Reading the stream's
# tail here would file a timeout as the account's window and park the runner
# until 18:10 for a shift the account never refused.
touch "$STUB_DIR/limit-killed"
k6home="$tmp/k6/home"; mkdir -p "$k6home"
: > "$STUB_DIR/mellions.calls"
env MELLIONS_HOME="$k6home" MELLIONS_PROMPT="$tmp/k-task.md" "$root/scripts/shift.sh" > "$tmp/k6.out" 2>&1
rc=$?
rm -f "$STUB_DIR/limit-killed"
ls "$k6home"/shifts/*.limit >/dev/null 2>&1 \
  && bad "K6: a timeout kill that quoted the refusal was filed as the account refusing it"
[ "$rc" -eq 1 ] || bad "K6: a killed session with no reply exited $rc, not 1: $(cat "$tmp/k6.out")"
grep -q 'said nothing' "$k6home"/shifts/*.log 2>/dev/null \
  || bad "K6: a killed session with no reply is no longer filed as one: $(tail -3 "$k6home"/shifts/*.log 2>/dev/null)"

# K7: the honoured wait has a ceiling. A reset time that has just passed rolls
# to the next day, so an uncapped runner sleeps ~24h on a window the account
# measures in hours — the failure discovered at breakfast, not in the log.
touch "$STUB_DIR/limit-far"
k7home="$tmp/k7/home"; mkdir -p "$k7home"
start_runner "$k7home" "MELLIONS_SHIFTS_PER_DAY=9"; k7=$pid
log="$k7home.out"
wait_for 25 'further off than a usage window runs' "$log" \
  || bad "K7: an 8h window was honoured whole: $(tail -5 "$log")"
w=$(grep -oE 'no shift starts for [0-9]+s' "$log" | head -1 | grep -oE '[0-9]+')
{ [ "${w:-0}" -le 21600 ] && [ "${w:-0}" -gt 21000 ]; } \
  || bad "K7: the honoured wait was ${w:-none}s, not clamped to the 6h ceiling"
grep -q 'resets at' "$log" \
  && bad "K7: the runner still names a reset time it is not waiting for"
kill -TERM "$k7" 2>/dev/null; wait_gone 10 "$k7"
rm -f "$STUB_DIR/limit-far"

# K8: a refusal naming no resolvable zone. The marker carries no time, and the
# runner has to fall back to its own long wait — without that fallback the epoch
# is 0, the wait loop breaks at once and the next shift starts straight into the
# closed window, which is #228 restored.
touch "$STUB_DIR/limit-nozone"
k8home="$tmp/k8/home"; mkdir -p "$k8home"
start_runner "$k8home" "MELLIONS_SHIFTS_PER_DAY=9"; k8=$pid
log="$k8home.out"
wait_for 25 'refused by the account' "$log" \
  || bad "K8: a refusal with no resolvable time was not recognised: $(tail -5 "$log")"
grep -q 'resets at' "$log" \
  && bad "K8: the runner invented a reset time the refusal never named: $(tail -3 "$log")"
before=$(count 'starting' "$log"); sleep 3
[ "$(count 'starting' "$log")" -eq "$before" ] \
  || bad "K8: another shift started straight after a refusal naming no window: $(tail -5 "$log")"
kill -TERM "$k8" 2>/dev/null; wait_gone 10 "$k8"
rm -f "$STUB_DIR/limit-nozone"

note "K: a refused shift is filed interrupted with its window and its lane; a quoted refusal and a timeout kill that quoted it are not; the runner waits for the window rather than backing off; and the cap counts the shifts that ran"

# ---- L. where a shift's build scratch goes -----------------------------------
# Go's default work directory is /tmp, and a shift killed by a usage limit or
# the budget never removes the one it was using. On a host whose /tmp is a
# tmpfs under a per-user quota that ends with every suite on the machine
# refused at a few megabytes while df still shows gigabytes free. The session
# is what runs `go`, so what these assert on is the environment the session was
# actually handed, read out of the stub it was started as — not the text of the
# script that sets it.
cat > "$STUB_DIR/claude-l" <<'STUB'
#!/usr/bin/env bash
cat > /dev/null
printf '%s\n' "${GOTMPDIR-<unset>}" >> "$STUB_DIR/l.gotmpdir"
printf '{"type":"result","result":"ready — the stub shift replied"}\n'
STUB
chmod +x "$STUB_DIR/claude-l"
printf 'work\n' > "$tmp/l-task.md"

run_l() {   # run_l <home> [env assignments...]
  local home="$1"; shift
  : > "$STUB_DIR/l.gotmpdir"
  env "$@" MELLIONS_HOME="$home" MELLIONS_BIN="$STUB_DIR/mellions" \
      CLAUDE_BIN="$STUB_DIR/claude-l" MELLIONS_PROMPT="$tmp/l-task.md" \
      "$root/scripts/shift.sh" > "$tmp/l.out" 2>&1
  saw=$(cat "$STUB_DIR/l.gotmpdir")
}

# L1: the session is handed a scratch directory on disk, and it exists.
lhome="$tmp/l1/home"; mkdir -p "$lhome"
run_l "$lhome"
[ "$saw" = "$lhome/tmp/go" ] \
  || bad "L1: the session was handed GOTMPDIR=$saw, so its builds still scratch in /tmp: $(tail -3 "$tmp/l.out")"
[ -d "$lhome/tmp/go" ] || bad "L1: $lhome/tmp/go was never created, so Go falls back to /tmp"

# L2: what earlier shifts left there is collected, and only that. A live build
# holds a directory whose mtime is now, so age is what separates them.
mkdir -p "$lhome/tmp/go/go-build-old" "$lhome/tmp/go/go-build-fresh" "$lhome/tmp/go/keep-me"
touch "$lhome/tmp/go/go-build-old/f"
touch -d '2 days ago' "$lhome/tmp/go/go-build-old" 2>/dev/null \
  || touch -t "$(date -u -v-2d '+%Y%m%d%H%M')" "$lhome/tmp/go/go-build-old"
run_l "$lhome"
[ -e "$lhome/tmp/go/go-build-old" ] \
  && bad "L2: a work directory two days old survived the shift, so the scratch only ever grows"
[ -d "$lhome/tmp/go/go-build-fresh" ] \
  || bad "L2: a work directory written this minute was collected — that is a live build losing its scratch"
[ -d "$lhome/tmp/go/keep-me" ] \
  || bad "L2: the sweep removed a directory that is not a Go work directory"

# L3: an explicit choice is not overridden. A lane that points Go somewhere
# with room, or at a filesystem it needs, keeps what it set.
lmine="$tmp/l3/mine"; mkdir -p "$lmine" "$tmp/l3/home"
run_l "$tmp/l3/home" GOTMPDIR="$lmine"
[ "$saw" = "$lmine" ] \
  || bad "L3: an explicit GOTMPDIR was replaced with $saw"
[ -e "$tmp/l3/home/tmp/go" ] \
  && bad "L3: the shift made its own scratch directory anyway, beside the one it was told to use"

note "L: the session's builds scratch on disk under the home, what earlier shifts left is collected and nothing younger is, and an explicit GOTMPDIR stands"

# make check has to run this, or everything above is about a file nothing invokes.
grep -q 'scripts/test-\*.sh' "$root/Makefile" || bad "the Makefile does not run scripts/test-*.sh"

[[ $fail -eq 0 ]] && echo "ok  shifts"
exit $fail

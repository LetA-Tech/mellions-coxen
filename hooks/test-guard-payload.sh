#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The runtime reads a guard's silence as allow. Each guard hook decides on the
# whole payload or refuses: late, stalled, cut, absent, or a binary that exits
# before deciding is never silence. A payload that arrives slowly but steadily
# is decided, a person at a terminal is not refused, and the declared silences
# (no binary, the operator's off switch) stay silent.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP

bin="$tmp/mellions"
if ! (cd "$root" && go build -o "$bin" ./cmd/mellions) 2>/dev/null; then
  bad "the binary the hooks call does not build"
  exit 1
fi
export HOME="$tmp/home"; mkdir -p "$HOME"
export MELLIONS_CONFIG="$tmp/config.json"
printf '{"owner":"test"}\n' >"$MELLIONS_CONFIG"
export MELLIONS_BIN="$bin"

payload() { printf '{"tool_name":"Bash","cwd":"/tmp","tool_input":{"command":"%s"}}' "$1"; }

own_denial() { [[ "$1" == *'"permissionDecision":"deny"'* && "$1" != *'not examined'* ]]; }
unread_refusal() { [[ "$1" == *'"permissionDecision":"deny"'* && "$1" == *'not examined'* ]]; }

# The writer is a process substitution, not a pipe stage: the hook can finish
# before it, and a writer's SIGPIPE must not become anything this test reads.
for pair in \
  'secret-check.sh|cat .db_connection' \
  'pr-reference.sh|git commit -m \"Closes #7\"' \
  'shared-tree.sh|pkill -f worker'
do
  name=${pair%%|*}
  hook="$root/hooks/$name"
  f="$tmp/$name.json"
  payload "${pair#*|}" >"$f"
  size=$(wc -c <"$f")

  # Whole and at once: the guard's own denial. This is the positive control
  # every case below is read against.
  out=$(bash "$hook" <"$f" 2>/dev/null)
  own_denial "$out" || bad "$name: a whole payload was not denied by the guard: ${out:-<silence>}"

  # An ordinary command is silence: the refusal must not spread.
  payload 'echo hi' >"$tmp/ok.json"
  out=$(bash "$hook" <"$tmp/ok.json" 2>/dev/null)
  [[ -z "$out" ]] || bad "$name: an ordinary command was refused: $out"

  # The payload arrives after the bound.
  out=$(bash "$hook" < <(sleep 2.5; cat "$f") 2>/dev/null)
  unread_refusal "$out" || bad "$name: a late payload was not refused: ${out:-<silence>}"

  # Cut short and closed: nothing timed out, and it is still not a whole call.
  head -c $((size - 2)) "$f" >"$tmp/cut.json"
  out=$(bash "$hook" <"$tmp/cut.json" 2>/dev/null)
  unread_refusal "$out" || bad "$name: a cut payload was not refused: ${out:-<silence>}"

  # Nothing at all, by an empty file and by a closed descriptor. Both return at
  # once: the bound was written for a descriptor that never ends, not for these.
  : >"$tmp/empty.json"
  out=$(bash "$hook" <"$tmp/empty.json" 2>/dev/null)
  unread_refusal "$out" || bad "$name: an empty payload was not refused: ${out:-<silence>}"
  started=$(date +%s)
  out=$(bash "$hook" <&- 2>/dev/null); rc=$?
  [[ $rc -eq 0 ]] || bad "$name: a closed descriptor gave rc=$rc"
  unread_refusal "$out" || bad "$name: a closed descriptor was not refused: ${out:-<silence>}"
  [[ $(( $(date +%s) - started )) -le 1 ]] || bad "$name: a closed descriptor waited"

  # A binary that dies before deciding examined nothing, and the deny is one line
  # of JSON the runtime can parse.
  printf '#!/bin/sh\nexit 3\n' >"$tmp/dies"; chmod +x "$tmp/dies"
  out=$(MELLIONS_BIN="$tmp/dies" bash "$hook" <"$f" 2>/dev/null)
  unread_refusal "$out" || bad "$name: a binary that exited 3 was not refused: ${out:-<silence>}"
  if command -v python3 >/dev/null 2>&1; then
    printf '%s' "$out" | python3 -c 'import json,sys; json.loads(sys.stdin.read())' 2>/dev/null ||
      bad "$name: the refusal is not parseable JSON: $out"
  fi
  [[ $(printf '%s\n' "$out" | wc -l) -eq 1 ]] || bad "$name: the refusal spans more than one line"

  # A binary that answers with silence has decided, and that decision stands.
  printf '#!/bin/sh\ncat >/dev/null\nexit 0\n' >"$tmp/allows"; chmod +x "$tmp/allows"
  out=$(MELLIONS_BIN="$tmp/allows" bash "$hook" <"$f" 2>/dev/null)
  [[ -z "$out" ]] || bad "$name: a binary's silence was overridden: $out"

  # Without the binary the hook is silent, which session start says out loud.
  out=$(MELLIONS_BIN="$tmp/absent" bash "$hook" <"$tmp/cut.json" 2>/dev/null)
  [[ -z "$out" ]] || bad "$name: spoke with no binary: $out"

  # A person at a terminal is not the runtime. script gives the hook a real tty;
  # a host without it says so rather than passing in silence.
  if command -v script >/dev/null 2>&1; then
    out=$(script -qefc "bash '$hook'" /dev/null </dev/null 2>/dev/null)
    [[ -z "${out//[$'\r\n']/}" ]] || bad "$name: refused a person at a terminal: $out"
  else
    printf '  %s: no script(1) here, so the terminal case did not run\n' "$name"
  fi
done

# Slow but steady is not late: a total past 2 s with no silence that long is
# decided whole. One hook is enough; the read is lib.sh's, shared by all three.
hook="$root/hooks/secret-check.sh"
f="$tmp/secret-check.sh.json"
out=$(bash "$hook" < <(head -c 30 "$f"; sleep 1.2; tail -c +31 "$f" | head -c 30; sleep 1.2; tail -c +61 "$f") 2>/dev/null)
own_denial "$out" || bad "a slow but steady payload was not decided whole: ${out:-<silence>}"

# It starts, then stops: bytes that end in silence are a partial call, refused
# whichever read's timer the silence fell in. Two reads can span up to 4 s of it.
out=$(bash "$hook" < <(head -c 20 "$f"; sleep 4.5) 2>/dev/null)
unread_refusal "$out" || bad "a payload that stopped arriving was not refused: ${out:-<silence>}"

# Whole, on a descriptor nobody closes: the payload is all there and is decided,
# not refused for the silence after it. The decode is what tells whole from cut.
out=$(bash "$hook" < <(cat "$f"; sleep 5) 2>/dev/null)
own_denial "$out" || bad "a whole payload on an open descriptor was not decided: ${out:-<silence>}"
out=$(bash "$hook" < <(cat "$tmp/ok.json"; sleep 5) 2>/dev/null)
[[ -z "$out" ]] || bad "an ordinary payload on an open descriptor was refused: $out"

# The hooks that decide on a time bound get one run together, since each waits it
# out. lib.sh's bounds are what stands between these and the runtime's kill, and a
# hook the runtime kills is allowed by the runtime.
#
# A drip never falls silent, so only the read's own window can end it.
drip() { head -c 20 "$f"; for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14; do printf ' '; sleep 1; done; }
timed() { # timed <label> <command...>: what it printed, then the seconds it took.
  local label=$1 s=$(date +%s); shift
  "$@" >"$tmp/$label.out" 2>/dev/null
  echo $(( $(date +%s) - s )) >"$tmp/$label.secs"
}
hook_on_drip() { bash "$hook" < <(drip); }
timed drip hook_on_drip &

# A binary that hangs and ignores TERM, since the real one catches it: with
# timeout(1), and on a PATH without it, as on a stock macOS.
printf '#!/bin/sh\ntrap "" TERM\nexec sleep 30\n' >"$tmp/hangs"; chmod +x "$tmp/hangs"
bashbin=$(command -v bash)
bare="$tmp/no-timeout"; mkdir -p "$bare"
for t in sleep dirname; do ln -s "$(command -v "$t")" "$bare/$t"; done
hook_hung() { MELLIONS_BIN="$tmp/hangs" bash "$hook" <"$f"; }
hook_hung_bare() { PATH="$bare" MELLIONS_BIN="$tmp/hangs" "$bashbin" "$hook" <"$f"; }
command -v timeout >/dev/null 2>&1 && timed hung hook_hung &
timed hungbare hook_hung_bare &
wait

unread_refusal "$(cat "$tmp/drip.out")" || bad "a drip was not refused: $(cat "$tmp/drip.out")"
[[ $(cat "$tmp/drip.secs") -le 10 ]] || bad "a drip took $(cat "$tmp/drip.secs") s, past the read's window"
if [[ -f "$tmp/hung.out" ]]; then
  unread_refusal "$(cat "$tmp/hung.out")" || bad "a hung binary was not refused: $(cat "$tmp/hung.out")"
  [[ $(cat "$tmp/hung.secs") -le 12 ]] || bad "a hung binary took $(cat "$tmp/hung.secs") s"
fi
unread_refusal "$(cat "$tmp/hungbare.out")" || bad "a hung binary with no timeout(1) was not refused: $(cat "$tmp/hungbare.out")"
[[ $(cat "$tmp/hungbare.secs") -le 12 ]] || bad "a hung binary with no timeout(1) took $(cat "$tmp/hungbare.secs") s"

# Without timeout(1) the binary still decides: its denial, its silence and its
# failure all reach the hook, and an answer does not wait for the watchdog.
s=$(date +%s)
out=$(PATH="$bare" "$bashbin" "$hook" <"$f" 2>/dev/null)
own_denial "$out" || bad "with no timeout(1) a denial did not come through: ${out:-<silence>}"
[[ $(( $(date +%s) - s )) -le 3 ]] || bad "with no timeout(1) an answer waited on the watchdog"
out=$(PATH="$bare" "$bashbin" "$hook" <"$tmp/ok.json" 2>/dev/null)
[[ -z "$out" ]] || bad "with no timeout(1) an ordinary command was refused: $out"
out=$(PATH="$bare" MELLIONS_BIN="$tmp/dies" "$bashbin" "$hook" <"$f" 2>/dev/null)
unread_refusal "$out" || bad "with no timeout(1) a binary that exited 3 was not refused: ${out:-<silence>}"

# The off switch is the operator's: it holds on a payload that would be refused.
out=$(MELLIONS_SECRET_CHECK=off bash "$hook" <"$tmp/empty.json" 2>/dev/null)
[[ -z "$out" ]] || bad "MELLIONS_SECRET_CHECK=off did not silence the refusal: $out"

# Only the guards refuse. A session start must never be refused or blocked by a
# missing payload, so nothing else in hooks/ calls the guard.
for h in "$root"/hooks/*.sh; do
  case "$(basename "$h")" in
    secret-check.sh|pr-reference.sh|shared-tree.sh|lib.sh|test-*) continue ;;
  esac
  grep -qE '^[[:space:]]*(guard|refuse)[[:space:]]' "$h" && bad "$(basename "$h") is not a guard and calls one"
done

[[ $fail -eq 0 ]] && echo "ok  hooks/test-guard-payload.sh"
exit $fail

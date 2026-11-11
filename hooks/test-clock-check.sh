#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Proves the wiring around the stamp comparison, which cmd/mellions/clockcheck_test.go
# cannot see: the payload shape the runtime sends for SendMessage, one line of
# JSON out carrying a deny, and silence for everything else — above all for the
# inputs the hook must fail open on, since a refusal here cuts a session off
# from every other. The clock is MELLIONS_CLOCK_NOW, which only the hook reads.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hook="$root/hooks/clock-check.sh"
fail=0
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP

bin="$tmp/mellions"
if ! (cd "$root" && go build -o "$bin" ./cmd/mellions) 2>/dev/null; then
  bad "the binary the hook calls does not build"
  exit 1
fi
export HOME="$tmp/home"; mkdir -p "$HOME"
export MELLIONS_CONFIG="$tmp/config.json"
printf '{"owner":"test"}\n' > "$MELLIONS_CONFIG"
export MELLIONS_BIN="$bin"

# The hook reads its payload from a file, never a pipe: a writer the hook
# outlives takes SIGPIPE, which pipefail makes the status this test reads.
run() { printf '%s' "$2" >"$tmp/stdin"; MELLIONS_CLOCK_NOW="$1" bash "$hook" <"$tmp/stdin" 2>/dev/null; }
send() { printf '{"tool_name":"SendMessage","tool_input":{"to":"lead-68","message":"%s"}}' "$1"; }
denied() { [[ "$1" == *'"permissionDecision":"deny"'* ]]; }

# refused <label> <clock> <payload>; allowed <label> <clock> <payload>
refused() { local out; out=$(run "$2" "$3"); denied "$out" || bad "$1 was not refused: ${out:-<silence>}"; }
allowed() { local out; out=$(run "$2" "$3"); [[ -z "$out" ]] || bad "$1 was refused: $out"; }

# The positive control every allow below is read against, and the reason's content.
out=$(run 01:14:00 "$(send 'f56 to lead-68, 01:19Z (date -u): hello')")
denied "$out" || bad "ahead by 5 min was not refused: ${out:-<silence>}"
for want in 01:19Z 01:14:00Z '`date -u`' resend; do
  [[ "$out" == *"$want"* ]] || bad "the denial does not say $want: $out"
done
refused "ahead by 5 min, HH:MM:SSZ" 01:14:00 "$(send '01:19:00Z hello')"
allowed "ahead by 30 s" 01:14:00 "$(send '01:14:30Z hello')"
allowed "behind by 10 min" 01:14:00 "$(send 'lead-68 to f51 (01:04Z): hello')"
allowed "no stamp" 01:14:00 "$(send 'hello')"
allowed "a later time and no leading stamp" 01:14:00 "$(send 'report follows, needed by 23:00Z')"
allowed "a time on the second line" 01:14:00 "$(send 'hello\n23:00Z')"
allowed "midnight wrap, stamp 23:59Z at 00:00:30Z" 00:00:30 "$(send '23:59Z hello')"
refused "midnight wrap, stamp 00:05Z at 23:58Z" 23:58:00 "$(send '00:05Z hello')"
allowed "another tool" 01:14:00 '{"tool_name":"Bash","tool_input":{"command":"echo 01:19Z"}}'

# Fail open: nothing but a confirmed ahead-of-clock stamp may refuse.
allowed "malformed JSON" 01:14:00 '{"tool_name":"SendMessage","tool_input":{"message":"01:19Z hel'
allowed "text that is not JSON" 01:14:00 '01:19Z hello'
allowed "an empty payload" 01:14:00 ''
allowed "no message field" 01:14:00 '{"tool_name":"SendMessage","tool_input":{"to":"x"}}'
allowed "a message that is not text" 01:14:00 '{"tool_name":"SendMessage","tool_input":{"message":{"text":"01:19Z"}}}'
allowed "an unreadable clock" noon "$(send '01:19Z hello')"

# No binary, a binary that is not this one, or one that prints something else:
# each is silence.
out=$(MELLIONS_BIN="$tmp/absent" run 01:14:00 "$(send '01:19Z hello')")
[[ -z "$out" ]] || bad "the hook spoke with no binary: $out"
printf '#!/bin/sh\necho "not a decision"\n' >"$tmp/liar"; chmod +x "$tmp/liar"
out=$(MELLIONS_BIN="$tmp/liar" run 01:14:00 "$(send '01:19Z hello')")
[[ -z "$out" ]] || bad "the hook forwarded output that is not a decision: $out"
printf '#!/bin/sh\nexit 3\n' >"$tmp/broken"; chmod +x "$tmp/broken"
out=$(MELLIONS_BIN="$tmp/broken" run 01:14:00 "$(send '01:19Z hello')")
[[ -z "$out" ]] || bad "the hook spoke for a binary that failed: $out"

# The hook's own status is 0 whatever it decided: a refusal is its stdout.
printf '%s' "$(send '01:19Z hello')" >"$tmp/stdin"
MELLIONS_CLOCK_NOW=01:14:00 bash "$hook" <"$tmp/stdin" >/dev/null 2>&1 || bad "the hook exited non-zero on a refusal"

# A descriptor the runtime closed must not hang the tool call or refuse it.
out=$(timeout 10 bash -c "bash '$hook' <&-" 2>/dev/null) || bad "the hook did not return on a closed stdin"
[[ -z "$out" ]] || bad "the hook refused with no payload: $out"

# The host clock, with MELLIONS_CLOCK_NOW unset: a stamp 10 min ahead is refused
# and one 10 min behind is not. Seconds of day, so it holds across midnight.
hms=$(date -u +%H:%M:%S)
sod=$(( 10#${hms:0:2} * 3600 + 10#${hms:3:2} * 60 + 10#${hms:6:2} ))
stamp() { local s=$(( (sod + $1 + 86400) % 86400 )); printf '%02d:%02d:%02dZ' $((s / 3600)) $((s % 3600 / 60)) $((s % 60)); }
out=$(printf '%s' "$(send "$(stamp 600) hello")" >"$tmp/stdin"; env -u MELLIONS_CLOCK_NOW bash "$hook" <"$tmp/stdin" 2>/dev/null)
denied "$out" || bad "a stamp 10 min ahead of the host clock was not refused: ${out:-<silence>}"
out=$(printf '%s' "$(send "$(stamp -600) hello")" >"$tmp/stdin"; env -u MELLIONS_CLOCK_NOW bash "$hook" <"$tmp/stdin" 2>/dev/null)
[[ -z "$out" ]] || bad "a stamp 10 min behind the host clock was refused: $out"

if [[ $fail -eq 0 ]]; then printf 'ok  hooks/test-clock-check.sh\n'; fi
exit $fail

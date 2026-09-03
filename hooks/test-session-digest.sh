#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The digest hook says what needs the owner once per window, never fails a
# session start, and is wired.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

hook="$root/hooks/session-digest.sh"
[[ -f "$hook" ]] || { bad "no hook at $hook"; exit 1; }

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM HUP
bin="$tmpdir/mellions"
if ! (cd "$root" && go build -o "$bin" ./cmd/mellions) 2>/dev/null; then
  bad "the binary the hook calls does not build"
  exit 1
fi

# Pinned away from the operator's own state, as the renewal test does: the
# binary resolves a config and a state directory from the environment, and
# left inherited this would read, and mark as seen, the real digest.
#
# MELLIONS_DEADLINE is pinned for a second reason: it is the variable the hook
# branches on, and scripts/shift.sh exports it for every unattended session.
# Inherited, the hook exits before it says anything, so every arm below that
# expects silence passes without exercising the hook at all. Each arm sets it
# for itself.
unset MELLIONS_DEADLINE
export HOME="$tmpdir/home"
export MELLIONS_HOME="$tmpdir/state"
export MELLIONS_CONFIG="$tmpdir/config.json"
mkdir -p "$HOME" "$MELLIONS_HOME/shifts" "$MELLIONS_HOME/reports"
cat > "$MELLIONS_CONFIG" <<JSON
{"owner":"test","assignments_root":"$MELLIONS_HOME/assignments","report_root":"$MELLIONS_HOME"}
JSON
printf 'Done.\n\n## Shipped — PR #30 merged\n' > "$MELLIONS_HOME/shifts/20260828-010000.reply.md"
printf '# 2026-08-28 03:00 UTC — fx-1\n\n## Needs you\n\nThe RLS decision is yours.\n' > "$MELLIONS_HOME/reports/20260828-030000-fx-1.md"

payload='{"session_id":"s","cwd":"'"$root"'","hook_event_name":"SessionStart","source":"startup"}'
first=$(printf '%s' "$payload" | MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
rc=$?
[[ $rc -eq 0 ]] || bad "the hook exited $rc"
grep -q 'shift 20260828-010000 on .*: Shipped — PR #30 merged' <<<"$first" || bad "the shift did not reach the session: $first"
grep -q 'report 20260828-030000-fx-1 needs you: The RLS decision is yours.' <<<"$first" || bad "the report that stopped on the owner did not reach the session"
[[ -f "$MELLIONS_HOME/digest-seen" ]] || bad "saying the digest left no marker"
bytes=$(printf '%s' "$first" | wc -c | tr -d ' ')
[[ $bytes -le 6144 ]] || bad "the block is $bytes bytes, over the 6144 bound"
note "first session: $bytes bytes, exit $rc"

# The same host, a second session: said already.
second=$(printf '%s' "$payload" | MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
rc=$?
[[ $rc -eq 0 ]] || bad "the second run exited $rc"
[[ -z "$second" ]] || bad "a second session inside the window was told it again: $second"
note "second session: silent, exit $rc"

# No binary: the ordinary partial install. Silent, exit 0.
gone=$(printf '%s' "$payload" | MELLIONS_BIN=/nonexistent/mellions CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
rc=$?
[[ $rc -eq 0 ]] || bad "a missing binary made the hook exit $rc"
[[ -z "$gone" ]] || bad "a missing binary still printed: $gone"

# Stdin closed, as the runtime may leave it: still exit 0, still no hang.
rm -f "$MELLIONS_HOME/digest-seen"
none=$(MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook" <&-)
rc=$?
[[ $rc -eq 0 ]] || bad "no payload made the hook exit $rc"
grep -q 'shift 20260828-010000' <<<"$none" || bad "with no payload the hook said nothing"

# An unattended shift — MELLIONS_DEADLINE set, as scripts/shift.sh exports it
# — is not the reader: nothing printed and the marker not created, so the
# owner's next session is still told.
rm -f "$MELLIONS_HOME/digest-seen"
shift_out=$(printf '%s' "$payload" | MELLIONS_DEADLINE=$(( $(date +%s) + 2700 )) MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
rc=$?
[[ $rc -eq 0 ]] || bad "an unattended session made the hook exit $rc"
[[ -z "$shift_out" ]] || bad "an unattended session was told the digest: $shift_out"
[[ ! -e "$MELLIONS_HOME/digest-seen" ]] || bad "an unattended session consumed the marker"
after=$(printf '%s' "$payload" | MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
grep -q 'shift 20260828-010000' <<<"$after" || bad "after an unattended session the attended one was not told"
note "unattended session: silent, marker untouched; the next attended one told"

# Wired at session start, or none of the above runs in a real session.
cfg="$root/hooks/hooks.json"
python3 - "$cfg" <<'PY' || fail=1
import json,sys
h=json.load(open(sys.argv[1]))["hooks"]
start=h.get("SessionStart") or []
if not any("session-digest.sh" in x.get("command","") for g in start for x in g["hooks"]):
    print("FAIL no SessionStart entry runs session-digest.sh"); sys.exit(1)
PY

[[ $fail -eq 0 ]] && echo "ok  session-digest"
exit $fail

#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The PreCompact hook says what the renewal keeps, and never blocks. Exit
# status is the whole contract: 2 blocks the compaction, and an unattended
# session whose compaction is blocked walks into the context limit and stops.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

hook="$root/hooks/session-renewal.sh"
[[ -f "$hook" ]] || { bad "no hook at $hook"; exit 1; }

# Bound to its own name and trapped: the build below puts a 4.9 MB binary in
# here, and every way out of this script has to take it away again.
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM HUP
bin="$tmpdir/mellions"
if ! (cd "$root" && go build -o "$bin" ./cmd/mellions) 2>/dev/null; then
  bad "the binary the hook calls does not build"
  exit 1
fi

# The hook execs the binary, which resolves a config: MELLIONS_CONFIG, then
# ./mellions.json, then $HOME/.mellions/config.json. Left inherited, this test
# reads whichever config the operator has and creates its assignments_root —
# so its result depends on the machine, and on a host with no ~/.mellions
# (a CI runner, a fresh checkout) it fails for a reason that has nothing to do
# with the hook. Both are pinned, because the config's own defaults fall back
# to $HOME again where a key is absent. After the build above, which wants the
# real $HOME for the Go build cache.
export HOME="$tmpdir/home"
export MELLIONS_HOME="$tmpdir/state"
export MELLIONS_CONFIG="$tmpdir/config.json"
mkdir -p "$HOME" "$MELLIONS_HOME"
cat > "$MELLIONS_CONFIG" <<JSON
{"owner":"test","assignments_root":"$MELLIONS_HOME/assignments","report_root":"$MELLIONS_HOME"}
JSON

payload='{"session_id":"s","cwd":"'"$root"'","hook_event_name":"PreCompact","trigger":"auto","custom_instructions":null}'

cuterr="$tmpdir/renewal.err"
out=$(printf '%s' "$payload" | MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook" 2>"$cuterr")
rc=$?
[[ $rc -eq 0 ]] || bad "the hook exited $rc; anything but 0 risks blocking a compaction"

# Stdout becomes the compaction instructions verbatim, so it must open by
# saying what it is rather than with a lane line out of nowhere.
grep -q 'working engineer' <<<"$out" || bad "the instructions never say what the summary is for"
for w in 'KEEP, in full' 'LET GO' 'established' 'next step'; do
  grep -qF -- "$w" <<<"$out" || bad "the instructions never say \"$w\""
done

# Under the runtime's preview bound, like every other block this plugin emits.
# Asserted on what `bounded` said, not on the length of what it returned: the
# cap produces stdout, so `$bytes -le 9200` holds however much was printed and
# a cut block passes it. `bounded` reports a cut on stderr; nothing else here
# would notice one, because the greps above read the head of the block.
bytes=$(printf '%s' "$out" | wc -c | tr -d ' ')
[[ ! -s "$cuterr" ]] || bad "the block was cut before it reached the session: $(cat "$cuterr")"
note "block $bytes bytes, exit $rc"

# A missing binary is the ordinary partial install. It must still exit 0: a
# hook that fails loudly here is a hook that stops the session.
gone=$(printf '%s' "$payload" | MELLIONS_BIN=/nonexistent/mellions CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
rc=$?
[[ $rc -eq 0 ]] || bad "a missing binary made the hook exit $rc, which would block the compaction"
[[ -z "$gone" ]] || note "a missing binary still printed $(printf '%s' "$gone" | wc -c | tr -d ' ') bytes"

# No payload at all — the runtime may close stdin. Still exit 0, still say
# something: a session with no resolvable lane needs the keep/let-go contract
# more than one with a lane, not less.
none=$(MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook" <&-)
rc=$?
[[ $rc -eq 0 ]] || bad "no payload made the hook exit $rc"
grep -q 'KEEP, in full' <<<"$none" || bad "with no payload the hook says nothing about what to keep"

# The hook has to be wired, or none of the above runs in a real session. The
# matcher must cover both triggers: an auto compaction is the only kind an
# unattended session ever sees.
cfg="$root/hooks/hooks.json"
python3 - "$cfg" <<'PY' || fail=1
import json,sys
h=json.load(open(sys.argv[1]))["hooks"]
pre=h.get("PreCompact")
if not pre: print("FAIL hooks.json has no PreCompact entry"); sys.exit(1)
g=pre[0]
m=g.get("matcher","")
if "auto" not in m or "manual" not in m:
    print("FAIL PreCompact matcher %r does not cover both triggers"%m); sys.exit(1)
if not any("session-renewal.sh" in x.get("command","") for x in g["hooks"]):
    print("FAIL the PreCompact entry does not run session-renewal.sh"); sys.exit(1)
PY

[[ $fail -eq 0 ]] && echo "ok  session-renewal"
exit $fail

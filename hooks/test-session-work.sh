#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The in-flight block shows the work, and a resumed session is told to settle
# anything its predecessor left half-done before anything else.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP
# Pinned away from the machine's own installation: the hook is exercised through
# a stub binary, so the real home is never read or written.
export HOME="$tmp/h" MELLIONS_HOME="$tmp/h" MELLIONS_CONFIG="$tmp/h/config.json"
mkdir -p "$HOME"

# A stub that answers `continue -brief` with one open lane and every other
# verb with nothing, so the block's own words are what is under test.
cat > "$tmp/mellions" <<'STUB'
#!/usr/bin/env bash
case "$1 ${2:-}" in
  "continue -brief") printf -- '- lane-1 (active, 2h ago) an objective\n' ;;
esac
exit 0
STUB
chmod +x "$tmp/mellions"

run() { printf '%s' "$1" | MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh"; }

out=$(run '{"source":"startup"}')
grep -qF -- '# Work in flight' <<<"$out" || bad "the in-flight block has no heading"
grep -qF -- 'lane-1' <<<"$out" || bad "the in-flight block does not show the lane"
grep -qF -- 'did not attend' <<<"$out" && bad "a fresh start is told it did not attend a session"
note "fresh start: the work is shown, nothing else is asked"

out=$(run '{"source":"resume"}')
grep -qF -- 'did not attend' <<<"$out" || bad "a resumed session is not told to settle unfinished work"
grep -qF -- 'lane-1' <<<"$out" || bad "a resumed session is not shown the lane"
note "resume: settle what was left half-done, then the work"

cat > "$tmp/mellions" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
chmod +x "$tmp/mellions"
out=$(run '{"source":"startup"}')
grep -qF -- 'Nothing is in flight' <<<"$out" || bad "with nothing in flight the block does not say so"
note "nothing in flight: says so"

[[ $fail -eq 0 ]] && echo "ok  session-work"
exit $fail

#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The in-flight block names the method for taking up work a session has no
# memory of, at the moment that work is put in front of it — a Skill listed in
# a catalog at minute zero is not one a session reaches for at minute twenty.
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

out=$(MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh" <&-)
grep -qF -- 'Skill(skill: "mellions:mellions-continuity")' <<<"$out" || bad "the in-flight block does not name the continuity method at the moment the work is shown"
grep -qF -- '`mellions continue`' <<<"$out" || bad "the in-flight block no longer says how the record is put next to the world"
grep -qF -- 'lane-1' <<<"$out" || bad "the in-flight block dropped the work itself"
note "in flight: the block names the method and the work"

cat > "$tmp/mellions" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
out=$(MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh" <&-)
grep -q 'Nothing is in flight' <<<"$out" || bad "with nothing in flight the block does not say so"
grep -qF -- 'mellions-continuity' <<<"$out" && bad "with nothing in flight the block still tells the session to load the continuity method"
note "nothing in flight: no method named"

# A resume and a compaction both act on a record rather than on memory, so the
# continuity instruction leads. On a fresh start it stays where it was, because
# a recovery instruction a session does not need is the noise that gets a hook
# turned off.
cat > "$tmp/mellions" <<'STUB'
#!/usr/bin/env bash
case "$1 ${2:-}" in
  "continue -brief") printf -- '- lane-1 (active, 2h ago) an objective\n' ;;
esac
exit 0
STUB
chmod +x "$tmp/mellions"

for src in resume compact; do
  out=$(printf '{"source":"%s"}' "$src" |
    MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh")
  grep -qF -- 'settle' <<<"$out" ||
    bad "on $src the block does not name the rule a summary never carries"
  first=$(grep -nF -- 'mellions:mellions-continuity' <<<"$out" | head -1 | cut -d: -f1)
  work=$(grep -nF -- 'lane-1' <<<"$out" | head -1 | cut -d: -f1)
  if [[ -z "$first" || -z "$work" || $first -ge $work ]]; then
    bad "on $src the method does not come before the work it applies to (method=$first work=$work)"
  fi
done
note "resume and compact: the method leads, above the work"

# The two triggers make different claims about the reader's own history, and a
# sentence true of one is false about the other. A resume picks up a session
# that ended, which this reader did not attend. A compaction is this session
# with its context replaced, so there is no earlier session it could have
# missed — and telling it there was invites it to discount, as somebody else's
# testimony, facts it established itself.
out=$(printf '{"source":"resume"}' |
  MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh")
grep -qF -- 'You did not attend the session before this one' <<<"$out" ||
  bad "a resumed session is not told it did not attend the session it inherits from"
note "resume: the reader is told it did not attend"

out=$(printf '{"source":"compact"}' |
  MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh")
grep -qF -- 'You did not attend the session before this one' <<<"$out" &&
  bad "a compacted session is told it did not attend a session that is itself"
grep -qF -- 'same session, with its context renewed' <<<"$out" ||
  bad "a compacted session is not told what actually happened to it"
note "compact: the reader is told its context was renewed, not handed over"

out=$(printf '{"source":"startup"}' |
  MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh")
grep -qF -- 'You did not attend the session before this one' <<<"$out" &&
  bad "a fresh session is told it is recovering from one that did not happen"
grep -qF -- 'mellions:mellions-continuity' <<<"$out" ||
  bad "a fresh session with work in flight is no longer told the method exists at all"
note "fresh start: the method is named, not led with"

[[ $fail -eq 0 ]] && echo "ok  session-work"
exit $fail

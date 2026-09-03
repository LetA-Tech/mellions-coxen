#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The hook denies a tool call that would read a credential into the transcript,
# and stays silent on everything else — including a command that discusses a
# credential file by name, which is how this rule gets written down.
#
# The predicate itself is a table in internal/secretread, which is where a case
# about command shapes belongs. What is proven here is the wiring that table
# cannot see: the payload shape the runtime sends for Bash and for Read, one
# line of JSON out carrying a deny, silence on an ordinary command, the
# operator's off switch, and a descriptor the runtime closed.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hook="$root/hooks/secret-check.sh"
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

run() { printf '%s' "$1" | bash "$hook" 2>/dev/null; }

# A key-name parser is unsafe when a credential file contains a bare URI.
out=$(run '{"tool_name":"Bash","cwd":"/tmp","tool_input":{"command":"awk -F= \"{print \\$1}\" .db_connection"}}')
case "$out" in
  *'"permissionDecision":"deny"'*) ;;
  *) bad "the leaking awk command was not denied: ${out:-<silence>}" ;;
esac
case "$out" in
  *AVNS_*|*postgresql://*) bad "the denial itself carried a credential" ;;
esac

# Read reaches the same file without a shell in between.
out=$(run '{"tool_name":"Read","tool_input":{"file_path":"/etc/payments/.env"}}')
case "$out" in
  *'"permissionDecision":"deny"'*) ;;
  *) bad "Read of a deployed .env was not denied: ${out:-<silence>}" ;;
esac

# The idiom the denial steers toward must not itself be denied, or the guard
# teaches nothing and gets switched off.
for ok in \
  '{"tool_name":"Bash","tool_input":{"command":"URL=\"$(tail -1 .db_connection)\"; psql \"$URL\" -c \"select 1\""}}' \
  '{"tool_name":"Bash","tool_input":{"command":"wc -c .db_connection"}}' \
  '{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}' \
  '{"tool_name":"Read","tool_input":{"file_path":"deploy/.env.example"}}' \
  '{"tool_name":"Edit","tool_input":{"file_path":"/etc/payments/.env"}}' ; do
  out=$(run "$ok")
  [[ -n "$out" ]] && bad "denied a call it should allow: $ok -> $out"
done

# The operator's off switch, which is deliberately an environment setting.
out=$(MELLIONS_SECRET_CHECK=off run '{"tool_name":"Bash","tool_input":{"command":"cat .db_connection"}}')
[[ -n "$out" ]] && bad "MELLIONS_SECRET_CHECK=off did not silence the hook: $out"

# No binary on PATH is silence, not an error the session has to read.
out=$(MELLIONS_BIN="$tmp/absent" run '{"tool_name":"Bash","tool_input":{"command":"cat .db_connection"}}')
[[ -n "$out" ]] && bad "the hook spoke with no binary: $out"

# A descriptor the runtime closed must not hang the tool call.
if ! timeout 10 bash -c "bash '$hook' <&-" >/dev/null 2>&1; then
  bad "the hook did not return on a closed stdin"
fi

if [[ $fail -eq 0 ]]; then printf 'ok  hooks/test-secret-check.sh\n'; fi
exit $fail

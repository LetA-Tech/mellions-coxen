#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# No hook test pipes a payload straight into a hook. A hook can exit before it
# has read stdin — session-digest.sh does inside a shift, and lib.sh's read
# gives up after two seconds — and a writer the hook outlives takes SIGPIPE,
# which pipefail makes the status the test asserts. Scheduling then decides the
# verdict. A test feeds the hook from a file, or drains the pipe into one first.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
self="$(basename "${BASH_SOURCE[0]}")"
fail=0
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

# One logical line per pipeline: a stage continued onto the next line is joined
# to it, so `printf … |` followed by an indented `bash "$hook"` is one line.
joined() { sed -e ':a' -e '/[^|]|[[:space:]]*$/{N;s/\n[[:space:]]*/ /;ba' -e '}' "$1"; }

# A pipe stage — not `||` — that starts bash on a hook script. Whatever sits
# between the pipe and bash stays inside that stage: an assignment's value can
# hold spaces, as MELLIONS_DEADLINE=$(( $(date +%s) + 2700 )) does.
stage='(^|[^|])\|[^|;&]*[[:space:]]bash[[:space:]]+"?\$\{?(hook|root/hooks/|tmp/hooks/)'

piped() { joined "$1" | grep -E "$stage"; }

# The pattern is evidence only if it can fail: the shapes this repository used
# are caught, and the shapes that replaced them are not.
probe=$(mktemp) || { bad "no temp file for the pattern's own controls"; exit 1; }
trap 'rm -f "$probe"' EXIT INT TERM HUP
must_catch=(
  'out=$(printf '"'"'%s'"'"' "$payload" | MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook")'
  'run() { payload "$1" | MELLIONS_BIN="$bin" bash "$hook"; }'
  'run() { printf '"'"'%s'"'"' "$1" | bash "$hook" 2>/dev/null; }'
  $'out=$(printf \'{"source":"%s"}\' "$src" |\n    MELLIONS_BIN="$tmp/mellions" CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-work.sh")'
  'x=$(payload a | env A=1 bash "${hook}")'
  'o=$(printf '"'"'%s'"'"' "$p" | MELLIONS_DEADLINE=$(( $(date +%s) + 2700 )) MELLIONS_BIN="$bin" bash "$hook")'
)
must_pass=(
  'out=$(MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook" <"$tmpdir/payload")'
  'runhook() { cat >"$tmp/stdin"; env "$@" MELLIONS_BIN="$bin" bash "$hook" <"$tmp/stdin"; }'
  'out=$(payload x | runhook MELLIONS_MERGE_CHECK=off)'
  'grep -q deny <<<"$out" || bash "$hook" <&-'
  $'grep -q deny <<<"$out" ||\n  bash "$hook" <&-'
)
for c in "${must_catch[@]}"; do
  printf '%s\n' "$c" >"$probe"
  [[ -n "$(piped "$probe")" ]] || bad "the pattern misses a pipe into a hook: $c"
done
for c in "${must_pass[@]}"; do
  printf '%s\n' "$c" >"$probe"
  [[ -z "$(piped "$probe")" ]] || bad "the pattern flags a feed that has no writer to race: $c"
done

n=0
for t in "$root"/hooks/test-*.sh "$root"/scripts/test-*.sh "$root"/deploy/test-*.sh; do
  [[ -f "$t" ]] || continue
  # This file runs no hook; its controls above are the shapes it refuses.
  [[ "$(basename "$t")" != "$self" ]] || continue
  n=$((n + 1))
  if hits=$(piped "$t"); [[ -n "$hits" ]]; then
    bad "${t#"$root"/} pipes a payload straight into a hook; feed it from a file:"
    printf '%s\n' "$hits" | sed 's/^/    /'
  fi
done
[[ $n -gt 0 ]] || bad "no test scripts found, so nothing was checked"

[[ $fail -eq 0 ]] && echo "ok  hook-feed ($n test scripts)"
exit $fail

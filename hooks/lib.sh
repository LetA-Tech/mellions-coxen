#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Shared by the hooks.
#
# Session start is several separate hooks because the runtime shows the model only
# a short preview of any single hook's output past roughly 8 KB. Each hook keeps
# itself under that, and the identity is the one that must never be cut.
set -uo pipefail

root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
mellions="${MELLIONS_BIN:-$(command -v mellions || true)}"

# The runtime's payload names the session and the working directory; the
# binary reads it from stdin. Captured once so it can be handed to more than
# one command.
#
# The read is bounded rather than a plain `cat`. `[[ ! -t 0 ]]` is true both for
# a pipe the runtime wrote and for a descriptor that is closed, and on a closed
# descriptor `cat` does not return — the hook hangs until the runtime kills it,
# which on PreCompact is a compaction waiting on nothing. `read -d ''` is a
# builtin, so the bound holds on a host with no `timeout`, and it consumes to
# EOF, so a payload that is there arrives whole.
payload=""
if [[ ! -t 0 ]]; then
  IFS= read -r -d '' -t 2 payload 2>/dev/null || true
fi

# run <args...>: the binary with the hook payload on stdin, bounded.
#
# MELLIONS_HOOK says the payload is there to be read. Without it the binary
# leaves stdin alone, so the same command run by a person, a script, cron or CI
# neither blocks on a pipe nobody will close nor eats the caller's input.
run() {
  [[ -x "$mellions" ]] || return 1
  if command -v timeout >/dev/null 2>&1; then
    printf '%s' "$payload" | MELLIONS_HOOK=1 timeout 8 "$mellions" "$@" 2>/dev/null
  else
    printf '%s' "$payload" | MELLIONS_HOOK=1 "$mellions" "$@" 2>/dev/null
  fi
}

# bounded <bytes>: pass stdin through, cut at the limit and say on stderr that
# it cut. The limit is below the runtime's preview threshold, so what reaches
# the model is what was printed rather than the first twenty-five lines of it.
# The note is the only independent side a caller has: stdout can never exceed
# the limit, so a test that measures stdout cannot tell a cut from a fit.
bounded() {
  local body
  body=$(cat; printf x); body=${body%x}
  printf '%s' "$body" | head -c "$1"
  if [[ $(printf '%s' "$body" | wc -c) -gt $1 ]]; then
    printf 'mellions: block cut at %s bytes\n' "$1" >&2
  fi
  return 0
}

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
#
# Two bounds, because a hook the runtime kills is allowed by the runtime. Silence
# ends the read after one empty 2 s read, so a descriptor nobody closes costs at
# most two of them. payload_window ends it whatever is still arriving: bash takes
# a pipe a byte at a time, about 1 MB/s on a loaded host, so a drip or a call of
# several MiB is cut here and refused by the binary, whose decode is what tells
# whole from cut. A JSON object's prefix is never a JSON object.
payload=""
payload_state=terminal
payload_window=6
if [[ ! -t 0 ]]; then
  payload_state=read
  payload_start=$SECONDS
  while :; do
    payload_chunk=""
    payload_wait=$(( payload_window - (SECONDS - payload_start) ))
    (( payload_wait > 0 )) || break
    (( payload_wait < 2 )) || payload_wait=2
    IFS= read -r -d '' -t "$payload_wait" payload_chunk 2>/dev/null
    payload_rc=$?
    payload+=$payload_chunk
    (( payload_rc > 128 )) && [[ -n "$payload_chunk" ]] || break
  done
fi

# run <args...>: the binary with the hook payload on stdin, bounded.
#
# MELLIONS_HOOK says the payload is there to be read. Without it the binary
# leaves stdin alone, so the same command run by a person, a script, cron or CI
# neither blocks on a pipe nobody will close nor eats the caller's input.
# The payload goes in by process substitution, so the status is the binary's:
# a command that never reads stdin can exit before the writer, and a writer's
# SIGPIPE in a pipeline would become the status under pipefail.
#
# The bound holds with or without timeout(1), which a stock macOS lacks. -k and the
# KILL are there because the binary catches TERM to cancel its work: a guard stuck
# outside that work would otherwise outlive the bound until the runtime kills the
# hook, which reads as allow. Either way a bound that fired is a non-zero status.
run() {
  [[ -x "$mellions" ]] || return 1
  if command -v timeout >/dev/null 2>&1; then
    MELLIONS_HOOK=1 timeout -k 1 8 "$mellions" "$@" < <(printf '%s' "$payload") 2>/dev/null
    return
  fi
  MELLIONS_HOOK=1 "$mellions" "$@" < <(printf '%s' "$payload") 2>/dev/null &
  local pid=$! dog rc
  (
    n=0
    while kill -0 "$pid" 2>/dev/null; do
      (( n++ < 8 )) || { kill -KILL "$pid" 2>/dev/null; break; }
      sleep 1
    done
  ) >/dev/null 2>&1 </dev/null &
  dog=$!
  wait "$pid"
  rc=$?
  kill "$dog" 2>/dev/null
  return "$rc"
}

# refuse <reason>: the deny the binary would print, for a guard that never got as
# far as asking it. The reason is printed as written, so it carries no quote or
# backslash.
refuse() {
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$1"
}

# guard <subcommand>: a PreToolUse guard's answer.
#
# The runtime reads silence as allow, so silence is only for a guard that
# examined the call. No payload and a binary that did not finish are each a
# refusal; a cut one is the binary's to refuse. A person at a terminal is not the
# runtime and gets nothing.
guard() {
  local out rc
  [[ "$payload_state" == terminal ]] && return 0
  if [[ -z "$payload" ]]; then
    refuse "This tool call was not examined: no hook payload arrived, so the guard refused it rather than let it through unread. Nothing about the call itself was judged. Retry it."
    return 0
  fi
  out=$(run "$@")
  rc=$?
  if [[ $rc -ne 0 ]]; then
    refuse "This tool call was not examined: mellions $1 exited $rc before deciding, so the guard refused it rather than let it through unread. Nothing about the call itself was judged. Retry it; if it repeats, the installed binary is what needs looking at."
    return 0
  fi
  [[ -z "$out" ]] || printf '%s\n' "$out"
  return 0
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

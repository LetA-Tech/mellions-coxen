#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# What the engineer was in the middle of, and who else is here. Registering
# is also how peers learn this session exists. Runs at startup, resume and
# clear; a compaction keeps its own summary and is not handed this list.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
run here >/dev/null 2>&1 || true

# A resumed session picks up one that ended and is told to settle what it left
# half-done. Absent or unreadable, the source is treated as a fresh start.
resumed=false
case "$payload" in
  *'"source"'*'"resume"'*) resumed=true ;;
esac
{
  if inflight=$(run continue -brief 2>/dev/null) && [[ -n "$inflight" ]]; then
    echo "# Work in flight"
    echo
    if [[ $resumed = true ]]; then
      echo "You did not attend the session before this one. Settle anything it began and"
      echo "did not finish (a push, a merge, a deploy) by looking, before anything else."
    fi
    echo "Every line was true when written; \`mellions continue\` compares it with now."
    echo
    printf '%s\n' "$inflight"
  else
    echo "Nothing is in flight on this installation. Choosing what to do is the work at this"
    echo "moment: \`mellions survey\` collects what needs attention across the estate."
    # Refresh the saved survey in the background, so the next prompt can hand it
    # over without the session having to wait for it now.
    ( nohup "$mellions" survey -save >/dev/null 2>&1 & ) 2>/dev/null || true
  fi
  if peers=$(run who 2>/dev/null) && printf '%s' "$peers" | grep -q ' tree, started '; then
    echo
    echo "# Who else is here"
    echo
    printf '%s\n' "$peers"
  fi
} | bounded 9200
exit 0

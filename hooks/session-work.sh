#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# What the engineer was in the middle of, and who else is here. Registering
# is also how peers learn this session exists.
#
# A session that started fresh reads the work list as a menu. One that resumed
# or compacted is acting on a record rather than on memory, so the continuity
# instruction leads and the inherited responsibility is not mistaken for a menu
# of new work.
#
# Resume and compaction are not the same claim, and one sentence cannot make
# both. A resume picks up a session that ended: there was a session before this
# one and this reader did not attend it. A compaction is that same session with
# its context replaced by a summary it wrote itself — there is no earlier
# session to have missed. Telling a compacted session it did not attend the
# previous one is false about its own history, and it invites the reader to
# discount facts it established itself as somebody else's testimony.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
run here >/dev/null 2>&1 || true

# The runtime says whether this session is new, has picked up a session that
# ended, or is this session with its context renewed. Absent or unreadable,
# treat it as new: leading with a recovery instruction a session does not need
# is the noise that gets hooks turned off.
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
} | bounded 4000
exit 0

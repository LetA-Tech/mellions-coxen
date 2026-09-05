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
state=fresh
case "$payload" in
  *'"source"'*'"compact"'*) state=compacted ;;
  *'"source"'*'"resume"'*)  state=resumed ;;
esac

{
  if inflight=$(run continue -brief 2>/dev/null) && [[ -n "$inflight" ]]; then
    echo "# What you are in the middle of"
    echo
    if [[ $state = resumed ]]; then
      echo "**You did not attend the session before this one.** What you inherit is the"
      echo "responsibility, not the conversation, and the record below is testimony rather"
      echo "than fact. Load the method before you act on any of it:"
      echo
    elif [[ $state = compacted ]]; then
      echo "**This is that same session, with its context renewed.** No earlier session"
      echo "was handed to you: what a compaction replaces is the conversation, never the"
      echo "responsibility, so what you established still stands and the summary saying so"
      echo "is your own. What it cannot carry is the evidence under it — the output you"
      echo "read, the citation you checked, the command you actually ran — and the record"
      echo "below was written by other sessions besides. Load the method before you act on"
      echo "any of it:"
      echo
    fi
    # Both leads end on the same instruction, and the rule under it is the same
    # rule: an operation in flight when the context ended is settled by looking,
    # whichever way the context ended.
    if [[ $state != fresh ]]; then
      echo "    Skill(skill: \"mellions:mellions-continuity\")"
      echo
      echo "It is eight minutes and it carries the one rule a summary never does: settle"
      echo "anything that began and never finished — a push, a merge, a deploy — before"
      echo "anything else, by looking rather than by reasoning about which."
      echo
    fi
    echo "The record's answer, not the world's: every line was true when it was written."
    echo "Before acting on any of it, \`mellions continue\` puts what was recorded next to"
    echo "what the repository and the tracker say now — and if the session it names still"
    echo "opens, resuming it is worth more than rebuilding from the record."
    if [[ $state = fresh ]]; then
      echo "The method for taking up work you have no memory of is"
      echo "\`Skill(skill: \"mellions:mellions-continuity\")\`."
    fi
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

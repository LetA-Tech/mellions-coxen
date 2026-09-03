#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# What the engineer is responsible for — where it is about the work in front of
# the session.
#
# `-here` renders a program in full only where its declared boundary names the
# repository the session is in, and one line naming it otherwise. Standing
# context is read as relevant because it arrived; a program that has not said it
# covers this repository is 4 KB of every window that is about something else.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
if [[ ! -x "$mellions" ]]; then
  echo "The \`mellions\` binary is not on PATH (or MELLIONS_BIN), so no partnership, no"
  echo "program and no record of work in flight could be loaded. Install it and start"
  echo "again, or say so before claiming anything about the estate."
  exit 0
fi
programs=$(run program list 2>/dev/null) || exit 0
if [[ "$programs" == no\ program* ]]; then
  echo "No program has been discovered yet, so nothing states what this work is for."
  echo "\`mellions program discover\` collects the evidence; draft the program from it and"
  echo "leave the DECLARED sections as questions for the owner."
  exit 0
fi
{
  echo "# What you are responsible for"
  echo
  for slug in $(printf '%s\n' "$programs" | awk 'NF {print $1}'); do
    run program show -brief -here "$slug" || echo "(program $slug could not be rendered: \`mellions program show $slug\`)"
    echo
  done
} | bounded 9200
exit 0

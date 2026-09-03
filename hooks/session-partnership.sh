#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Who the engineer works with, how they want to work, what they delegated.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
partners=$(run partner list 2>/dev/null) || exit 0
[[ "$partners" != no\ partnership* ]] || exit 0
{
  echo "# Who you are working with"
  echo
  echo "Context, not identity. Where it says how somebody wants to be worked with, or"
  echo "what they have delegated to you, that is theirs to state and yours to honour. A"
  echo "DECLARED section that is still a question has not been answered yet."
  for slug in $(printf '%s\n' "$partners" | awk 'NF {print $1}'); do
    echo
    run partner show -brief -here "$slug" || echo "(partnership $slug could not be rendered: \`mellions partner show $slug\`)"
  done
} | bounded 9200
exit 0

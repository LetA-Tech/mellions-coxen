#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The Skill catalog: every method this installation carries, grouped the way a
# session meets them, each with the situation that calls for it.
#
# The runtime's own skill listing gives the model a name and a description per
# skill, but its budget is a fraction of the context window and it drops the
# descriptions of the least-used skills first — so on a host carrying many
# plugins exactly these arrive as bare names. A name carries no situation, and
# the identity's instruction to load the Skill the work matches has nothing to
# match against. This hook delivers the situations through a channel the plugin
# controls, short enough to stay under the runtime's preview limit. Like the
# identity and reasoning hooks it reads nothing from stdin: it needs no payload,
# and a read that waits on the runtime is a way to miss the timeout.
set -uo pipefail
root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

shopt -s nullglob
files=("$root"/skills/*/SKILL.md)
[[ ${#files[@]} -gt 0 ]] || exit 0

# when <skill.md>: the situation clause of the description, without the
# utterance triggers. Unattended work has no utterances; what a session can
# match against is the situation it is in, and fifteen situation clauses fit
# where fifteen full descriptions would not.
when() {
  awk '
    NR == 1 && $0 == "---" { fm = 1; next }
    fm && $0 == "---" { exit }
    fm && /^description:[[:space:]]*/ {
      sub(/^description:[[:space:]]*/, "")
      sub(/ Triggers — .*$/, "")
      sub(/ Do NOT use .*$/, "")
      print
      exit
    }' "$1"
}

# Groups in the order a session meets them. A Skill named in no group is still
# listed, under Other, so a new Skill never disappears from the catalog.
bearing=(mellions-reasoning mellions-deep-research mellions-falsification mellions-self-learning)
work=(mellions-bug-audit mellions-issue-creation mellions-issue-resolution-proposal
      mellions-issue-remediation mellions-issue-closure mellions-harness-rule
      mellions-comment-hygiene)
around=(mellions-delegation mellions-territory mellions-environment mellions-sandbox
        mellions-continuity)

# seen is a space-delimited list rather than an associative array: the
# runtime's bash on macOS is 3.2, which has none.
seen=" "
entry() {
  local f="$root/skills/$1/SKILL.md"
  [[ -f "$f" ]] || return 0
  local d; d=$(when "$f")
  [[ -n "$d" ]] || return 0
  seen="$seen$1 "
  printf -- '- **mellions:%s** — %.400s\n' "$1" "$d"
}

{
  echo "# Your Skill catalog"
  echo
  echo "A method you did not read is not a method you carry. Each Skill below is loaded"
  echo "with the Skill tool when the situation it names arrives, and not before:"
  echo "\`Skill(skill: \"mellions:mellions-bug-audit\")\`. \`mellions-reasoning\`,"
  echo "\`mellions-deep-research\` and \`mellions-falsification\` reached you whole with the"
  echo "identity; they are listed so you can reload them after a compaction."
  echo
  echo "This list is read once, at minute zero, against work that has not happened yet."
  echo "\`mellions skills <what you are doing>\` answers the same question at the moment you"
  echo "have it, and after this block is out of context."
  echo
  echo "## Bearing — how you think, establish what is true, prove a fix holds, and learn"
  for n in "${bearing[@]}"; do entry "$n"; done
  echo
  echo "## The work — find, file, plan, fix, close, guard"
  for n in "${work[@]}"; do entry "$n"; done
  echo
  echo "## Around the work — other engineers, the estate, this host, after a break"
  for n in "${around[@]}"; do entry "$n"; done
  other=0
  for f in "${files[@]}"; do
    n=$(basename "$(dirname "$f")")
    [[ "$seen" == *" $n "* ]] && continue
    [[ $other -eq 0 ]] && { echo; echo "## Other"; other=1; }
    entry "$n"
  done
} | head -c 9200
exit 0

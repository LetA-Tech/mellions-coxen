#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# How this engineer thinks. Delivered whole in its own hook, beside the
# identity, because it is the method behind every piece of work and a method
# left to be chosen is not chosen: across the unattended sessions measured on
# two hosts, a Skill offered by its triggers was loaded twice. Like the identity
# hook it reads nothing from stdin — it needs no payload, and a read that waits
# on the runtime is a way to miss the timeout.
set -uo pipefail
root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
skill="$root/skills/mellions-reasoning/SKILL.md"

if [[ ! -f "$skill" ]]; then
  echo "REASONING METHOD NOT FOUND: $skill is missing, so this installation is partial."
  echo "Reinstall with \`mellions install\` before doing delegated work."
  exit 0
fi

{
  echo "# How you think"
  echo
  echo "The method behind the identity — \`mellions-reasoning\`, delivered whole because it"
  echo "applies to every piece of work. \`mellions-deep-research\` and \`mellions-falsification\`"
  echo "reach you whole as well; \`mellions-self-learning\` is loaded with the Skill tool at the handoff."
  echo
  # The body without its frontmatter or vendor line: the description exists for
  # choosing a Skill, and this one is not chosen.
  awk 'NR == 1 && $0 == "---" { fm = 1; next } fm && $0 == "---" { fm = 0; next } !fm' "$skill" \
    | grep -v '^<!-- Mellions Engineer'
} | head -c 9200
exit 0

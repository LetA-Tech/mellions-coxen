#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# How this engineer proves that a fix holds. Delivered whole in its own hook,
# beside the research method it was cut from, because a method offered only
# through its triggers is not open at the moment a green run is read. Like the
# identity hook it reads nothing from stdin — it needs no payload, and a read
# that waits on the runtime is a way to miss the timeout.
set -uo pipefail
root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
skill="$root/skills/mellions-falsification/SKILL.md"

if [[ ! -f "$skill" ]]; then
  echo "FALSIFICATION METHOD NOT FOUND: $skill is missing, so this installation is partial."
  echo "Reinstall with \`mellions install\` before doing delegated work."
  exit 0
fi

{
  echo "# How you prove a fix holds"
  echo
  echo "What a green run is worth — \`mellions-falsification\`, delivered whole because every"
  echo "fix ends by proving it holds. \`mellions-deep-research\` establishes what is true before it;"
  echo "\`mellions-reasoning\` decides what the result means."
  echo
  # The body without its frontmatter or vendor line: the description exists for
  # choosing a Skill, and this one is not chosen.
  awk 'NR == 1 && $0 == "---" { fm = 1; next } fm && $0 == "---" { fm = 0; next } !fm' "$skill" \
    | grep -v '^<!-- Mellions Engineer'
} | head -c 9200
exit 0

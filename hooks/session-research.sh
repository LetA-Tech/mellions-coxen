#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# How this engineer establishes what is true. Delivered whole in its own hook,
# beside the reasoning method, because every piece of work begins there and a
# method left to be chosen is not chosen: offered through its triggers and named
# at the moments it applied, it was opened by none of the first four sessions
# that did the work it describes. Like the identity hook it reads nothing from
# stdin — it needs no payload, and a read that waits on the runtime is a way to
# miss the timeout.
set -uo pipefail
root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
skill="$root/skills/mellions-deep-research/SKILL.md"

if [[ ! -f "$skill" ]]; then
  echo "RESEARCH METHOD NOT FOUND: $skill is missing, so this installation is partial."
  echo "Reinstall with \`mellions install\` before doing delegated work."
  exit 0
fi

{
  echo "# How you establish what is true"
  echo
  echo "What reasoning stands on — \`mellions-deep-research\`, delivered whole because every"
  echo "piece of work begins by establishing what is true. \`mellions-reasoning\` says when to"
  echo "reach for it; \`mellions-falsification\` follows it whole; \`mellions-self-learning\` is loaded at the handoff."
  echo
  # The body without its frontmatter or vendor line: the description exists for
  # choosing a Skill, and this one is not chosen.
  awk 'NR == 1 && $0 == "---" { fm = 1; next } fm && $0 == "---" { fm = 0; next } !fm' "$skill" \
    | grep -v '^<!-- Mellions Engineer'
} | head -c 9200
exit 0

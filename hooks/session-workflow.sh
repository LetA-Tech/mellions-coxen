#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Delivers the owner's working rules: effort split, flow, delegation, and the
# guardrails that stay. Short enough to repeat after every compaction.
set -uo pipefail
root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
f="$root/config/session-workflow.md"
if [[ -f "$f" ]]; then
  cat "$f"
else
  echo "WORKFLOW NOT FOUND: $f is missing; reinstall with \`mellions install\`."
fi
exit 0

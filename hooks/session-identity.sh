#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Who this engineer is. First, and alone in its hook, because it is the one
# thing that must reach the session whole.
set -uo pipefail
root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
agent="$root/agents/mellions.md"
if [[ -f "$agent" ]]; then
  echo "# Who you are"
  echo
  echo "Your identity, not a role assigned for this session. It does not change with"
  echo "who you work for or what you are working on."
  echo
  cat "$agent"
else
  echo "IDENTITY NOT FOUND: $agent is missing, so this installation is partial."
  echo "Reinstall with \`mellions install\` before doing delegated work."
fi
exit 0

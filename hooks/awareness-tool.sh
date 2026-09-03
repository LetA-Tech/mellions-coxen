#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The same awareness as awareness.sh, delivered mid-turn. A session working
# through a long turn hears about a peer arriving on its repository at its next
# tool call rather than at its next prompt, which for an autonomous session may
# be hours away or never. Each thing is still said once per session; when there
# is nothing new the hook prints nothing.
set -uo pipefail

MELLIONS="${MELLIONS_BIN:-$(command -v mellions || true)}"
[ -x "$MELLIONS" ] || exit 0

if command -v timeout >/dev/null 2>&1; then
  timeout 5 "$MELLIONS" state -runtime "${MELLIONS_RUNTIME:-claude}" -tool 2>/dev/null || true
else
  "$MELLIONS" state -runtime "${MELLIONS_RUNTIME:-claude}" -tool 2>/dev/null || true
fi
exit 0

#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Tells a working session what is true about its situation that it cannot
# infer: another session in this tree or on this repository, a survey that is
# ready to read when nothing is in flight.
#
# It never fails the turn, it is bounded, and it is silent when there is
# nothing to say: `mellions state` says each thing once per session.
set -uo pipefail

MELLIONS="${MELLIONS_BIN:-$(command -v mellions || true)}"
[ -x "$MELLIONS" ] || exit 0

if command -v timeout >/dev/null 2>&1; then
  timeout 5 "$MELLIONS" state -runtime "${MELLIONS_RUNTIME:-claude}" 2>/dev/null || true
else
  "$MELLIONS" state -runtime "${MELLIONS_RUNTIME:-claude}" 2>/dev/null || true
fi
exit 0

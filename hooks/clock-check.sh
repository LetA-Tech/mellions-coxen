#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Sessions on this host open a cross-session message with a UTC time, and other
# sessions schedule work by it. Written from memory it has run ahead of the real
# clock every time it was checked, so this hook denies a SendMessage whose first
# line opens with a time more than a minute ahead of the host clock, and names
# the clock. Which time is read, and how far ahead counts, is the binary's.
#
# Unlike the other guards this one fails open. A refusal here cuts one session
# off from another, so only a stamp the clock confirms is ahead may cause one:
# no binary, no payload, a binary that exits non-zero or prints anything but a
# decision, or the runtime ending the hook at its timeout, all allow the call.
# That is why hooks.json gives it no onFailure and a short timeout.
#
# MELLIONS_CLOCK_NOW (HH:MM:SS UTC) replaces the host clock for this hook alone,
# so a test needs neither a sleep nor the wall time. Nothing else sets it.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" && -n "$payload" ]] || exit 0
out=$(run clock-check -now "${MELLIONS_CLOCK_NOW:-}") || exit 0
case "$out" in
  '{"hookSpecificOutput":'*) printf '%s\n' "$out" ;;
esac
exit 0

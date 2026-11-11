#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Sessions on this host open a cross-session message with a UTC time, and other
# sessions schedule work by it. A time written from memory instead of read from
# `date -u` can be ahead of the real clock, so this hook denies a SendMessage
# whose stamp is more than a minute ahead, and names the clock. What the stamp is
# (the first time in the message's header), and how far ahead counts, is the
# binary's to say.
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

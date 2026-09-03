#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# What needs the owner since a session on this host was last told: finished
# shifts, reports that stopped on them, lanes handed off to them. The owner
# reads their own sessions, not the reports directory, so it is said here. Its
# own hook, because the runtime previews any one hook's output past ~8 KB; and
# said once per eight hours across every session on the host, so a second
# session start prints nothing.
#
# An unattended shift is not the reader. scripts/shift.sh exports
# MELLIONS_DEADLINE for every shift session and nothing else sets it; a shift
# that printed the digest would consume the once-per-eight-hours marker, and
# with shifts running back to back the owner's own session — the reader this
# exists for — would never be told.
[[ -z "${MELLIONS_DEADLINE:-}" ]] || exit 0
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
run report digest -brief | bounded 6144
exit 0

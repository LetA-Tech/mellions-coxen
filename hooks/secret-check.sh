#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Denies a tool call that would read a credential-bearing file's content into
# the transcript.
#
# This is the last point at which the credential is still secret: a transcript
# is sent as it is written, so nothing downstream can unprint it and the remedy
# after the fact is a rotation. Hence a denial rather than a warning, and hence
# fail-closed — a command shape the check cannot prove non-printing is refused.
#
# Without the binary this hook is silent, which every session-start hook says
# out loud at the top of the session. MELLIONS_SECRET_CHECK=off silences it,
# and must be set in the session environment: an inline `VAR=off cmd` prefix
# applies to that command, never to the hook process the runtime spawns.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
[[ -n "$payload" ]] || exit 0
[[ "${MELLIONS_SECRET_CHECK:-on}" == off ]] && exit 0
run secret-check
exit 0

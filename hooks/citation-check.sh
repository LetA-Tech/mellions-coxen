#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# mellions-deep-research states the rule correctly — "Open every citation
# before it is filed; one that does not resolve is a claim about code that is
# not there, and the reader cannot tell that from one that is" — and it kept
# failing to bind. Five citations across three consecutive shifts named a line
# that exists and says something else, one of them under a sentence asserting
# the citations had been re-opened. A rule stated in prose does not fire at the
# moment of action, and this is the moment: the body is written where no Skill
# is loaded, and handing it to gh is the last point at which the claim is still
# retractable. A denial becomes a tool result the session must answer.
#
# What is denied is a citation this checkout can resolve and the body does not
# quote. A path the checkout does not hold, a line range, and a path:line
# inside a fenced block or a blockquote are not claims about this code and are
# silent. Without the binary this hook is silent, which every session-start
# hook says out loud at the top of the session.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
[[ -n "$payload" ]] || exit 0
[[ "${MELLIONS_CITE_CHECK:-on}" == off ]] && exit 0
run cite-check
exit 0

#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# GitHub resolves a closing keyword in a pull request body only when the pull
# request merges into the repository's default branch. On any other base the
# keyword does nothing: the fix merges, the issue quietly stays open, and
# nothing reports it. That body is written at a moment no Skill is loaded and
# no gate can see, so this hook denies the tool call there, where it is typed —
# a denial becomes a tool result the session must answer, which advisory text
# delivered alongside the call is not shown to do.
#
# Whether a close is warranted on the default branch is a question of
# authority, and that is the closure Skill's and the partnership's, not this
# hook's: a body with no --base, or a base that is the default branch, passes.
#
# The decision is a parse and not a search — which commands open a pull
# request, what each hands GitHub as the body, which of that is prose rather
# than quotation, and whether a keyword in that prose is declared or denied —
# so it lives in the binary, where it is a program with a table of cases behind
# it. Without the binary this hook is silent, which every session-start hook
# says out loud at the top of the session.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
[[ -n "$payload" ]] || exit 0
run pr-body-check
exit 0

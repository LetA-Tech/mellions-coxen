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
# On the default branch the keyword is live and closes the issue at the merge,
# before any deploy and before the proof comment mellions-issue-closure
# requires — a promotion listing what each merged pull request fixed does it.
# It is denied there too, unless the body declares the diff is the whole proof.
# A title's keyword is denied on every base: the merge commit carries the title
# into its message, and GitHub closes the issue when that commit reaches the
# default branch, at the merge or at the next promotion. So is one in a
# `gh pr merge` --subject or --body, which is that commit's message.
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

#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Merging is where an older branch silently rolls a base branch backward. Git
# resolving the merge says nothing about whether it should: the branch may have
# diverged before a fix landed on the base, and re-applying its side of a file
# the base has since changed reverts that fix under a green suite.
#
# A note is already delivered at this command by the awareness hook, and a note
# is not enough for two reasons that are nothing to do with how it is written.
# It is advisory, so nothing has to answer it — a denial becomes a tool result
# the session must address. And it is keyed by the Skill it names, so it is said
# once per session: the eighth merge of a long session is reached in silence,
# which is the merge where discipline has decayed.
#
# What is denied is a state established rather than guessed — mergeability
# GitHub has not finished computing, and a branch behind its base in files the
# pull request also changes. Being behind alone passes, because a guard that
# fires on correct work is turned off and then protects nothing.
#
# The decision is a parse and two reads of the tracker, so it lives in the
# binary. Without the binary this hook is silent, which every session-start hook
# says out loud at the top of the session.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
[[ -n "$payload" ]] || exit 0
run pr-merge-check
exit 0

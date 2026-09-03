#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Every lane on this host is a worktree cut from one long-lived checkout per
# repository, and that checkout is nobody's lane: its working tree carries
# whatever the owner or another session has not committed. `git checkout <rev>
# -- .` there reads like a way to see an old commit but replaces the tree and
# index without preserving uncommitted work.
#
# The reads that answer the same question mutate nothing, so this hook denies
# the tool call where it is typed and names them. A denial becomes a tool
# result the session must answer, which advisory text delivered alongside the
# call is not shown to do.
#
# Which directory a command line leaves the shell in, which invocations are
# git, which tree each one is aimed at and whether its verb writes — none of
# that is a search over the payload, so it lives in the binary, where it is a
# program with a table of cases behind it. Without the binary this hook is
# silent, which every session-start hook says out loud at the top of the
# session.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[[ -x "$mellions" ]] || exit 0
[[ -n "$payload" ]] || exit 0
run shared-tree-check
exit 0

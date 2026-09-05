#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
#
# Put the built binary where the installation that runs actually is, or refuse.
#
# An install PATH does not resolve has not installed: the operator is told the
# installation moved and it did not. So the target defaults to the file
# `command -v mellions` names — the same resolution scripts/shifts.sh:51 uses to
# update an unattended host — and the copy is checked against PATH afterwards.
#
# stdout carries exactly one thing: the path installed to, for the caller to run
# the plugin half with. Everything a person reads goes to stderr.
#
#   BIN=/path/to/mellions   install exactly there
#   PREFIX=/usr/local       first-install root when no mellions is on PATH
#   SRC=bin/mellions        the freshly built binary to copy
set -euo pipefail

SRC="${SRC:-bin/mellions}"
PREFIX="${PREFIX:-/usr/local}"
BIN="${BIN:-}"

say() { printf '%s\n' "$*" >&2; }

# The plugin half registers into the invoking user's home, so sudo installs it
# for root and leaves the operator's runtimes as they were.
if [ -n "${SUDO_USER:-}" ]; then
	say "mellions: run this as yourself, not under sudo."
	say "  The second half of the install registers the plugin into the invoking"
	say "  user's home, and under sudo that is root's rather than ${SUDO_USER}'s."
	say "  Install somewhere you own instead:  make install PREFIX=\$HOME/.local"
	exit 1
fi

if [ ! -x "$SRC" ]; then
	say "mellions: $SRC is not there to install; run make build first."
	exit 1
fi

if [ -z "$BIN" ]; then
	BIN="$(command -v mellions 2>/dev/null || true)"
fi
if [ -z "$BIN" ]; then
	BIN="$PREFIX/bin/mellions"
fi

dir="$(dirname "$BIN")"
mkdir -p "$dir"

# Rename rather than write in place: the file may be the binary a session is
# running, and an in-place write to that is ETXTBSY.
cp "$SRC" "$BIN.new"
chmod 0755 "$BIN.new"
mv -f "$BIN.new" "$BIN"

resolved="$(command -v mellions 2>/dev/null || true)"
if [ -z "$resolved" ]; then
	say "mellions: installed $BIN, and no directory on PATH holds it —"
	say "  a shell here would still run no mellions at all."
	say "  Put $dir on PATH, or install into a directory already on it."
	exit 1
fi
if [ ! "$resolved" -ef "$BIN" ]; then
	say "mellions: installed $BIN, and \`mellions\` still runs $resolved."
	say "  An earlier PATH entry shadows the target, so this install reached"
	say "  nothing that runs. Install over the one that does:"
	say "    make install BIN=$resolved"
	exit 1
fi

say "mellions: installed $BIN"
printf '%s\n' "$BIN"

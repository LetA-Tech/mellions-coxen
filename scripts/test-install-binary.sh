#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The installer puts the binary where the installation that runs actually is,
# and refuses when what it wrote is not what a shell would run: a first install
# with no mellions anywhere, an update that finds one on PATH, a target an
# earlier PATH entry shadows, a target on no PATH directory at all, and sudo.
# No real binary is built and nothing outside the temporary tree is written.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
installer="$root/scripts/install-binary.sh"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP

# The freshly built binary this install is carrying. Its content is the oracle:
# a target that does not hold this string afterwards was not installed over.
src="$tmp/build/mellions"
mkdir -p "$tmp/build"
printf '#!/bin/sh\necho NEW\n' > "$src"
chmod 0755 "$src"

# A shell with a PATH of our own, so nothing here can resolve the host's real
# mellions and no case depends on where this test happens to run.
run() { # run <PATH> [env=val ...] -- reads stdout into $out, stderr into $err
	local path="$1"; shift
	local envs=("$@")
	set +e
	out=$(env -i HOME="$tmp/home" PATH="$path" SRC="$src" "${envs[@]}" \
		bash "$installer" 2>"$tmp/stderr")
	rc=$?
	set -e
	err=$(cat "$tmp/stderr")
}

# 1. First install: nothing named mellions on PATH, so PREFIX decides, and the
#    PREFIX bin directory is on PATH, so the check passes.
mkdir -p "$tmp/case1/bin"
run "$tmp/case1/bin:/usr/bin:/bin" PREFIX="$tmp/case1"
if [ "$rc" -ne 0 ]; then
	bad "first install exited $rc: $err"
elif [ "$out" != "$tmp/case1/bin/mellions" ]; then
	bad "first install went to '$out', wanted $tmp/case1/bin/mellions"
elif ! grep -q NEW "$tmp/case1/bin/mellions" 2>/dev/null; then
	bad "first install wrote no binary at $tmp/case1/bin/mellions"
else
	note "first install lands at PREFIX/bin and PATH resolves it"
fi

# 2. Update: a mellions already on PATH somewhere PREFIX knows nothing about.
#    The default target is that file, not PREFIX — this is the case the fixed
#    Makefile exists for, and the one the old constant target got wrong.
mkdir -p "$tmp/case2/home-bin" "$tmp/case2/prefix/bin"
printf '#!/bin/sh\necho OLD\n' > "$tmp/case2/home-bin/mellions"
chmod 0755 "$tmp/case2/home-bin/mellions"
run "$tmp/case2/home-bin:/usr/bin:/bin" PREFIX="$tmp/case2/prefix"
if [ "$rc" -ne 0 ]; then
	bad "update exited $rc: $err"
elif [ "$out" != "$tmp/case2/home-bin/mellions" ]; then
	bad "update went to '$out', wanted the installation on PATH"
elif ! grep -q NEW "$tmp/case2/home-bin/mellions"; then
	bad "update did not overwrite the installation that runs"
elif [ -e "$tmp/case2/prefix/bin/mellions" ]; then
	bad "update wrote a second copy under PREFIX"
else
	note "update overwrites the installation PATH resolves, not PREFIX"
fi

# 3. The defect itself: an explicit target that an earlier PATH entry shadows.
#    Writing it succeeds and reaches nothing that runs, so this must refuse.
mkdir -p "$tmp/case3/first" "$tmp/case3/second"
printf '#!/bin/sh\necho OLD\n' > "$tmp/case3/first/mellions"
chmod 0755 "$tmp/case3/first/mellions"
run "$tmp/case3/first:$tmp/case3/second:/usr/bin:/bin" BIN="$tmp/case3/second/mellions"
if [ "$rc" -eq 0 ]; then
	bad "a shadowed install target was accepted"
elif ! printf '%s' "$err" | grep -q "$tmp/case3/first/mellions"; then
	bad "the refusal does not name the binary that still runs: $err"
else
	note "a shadowed target is refused and names what runs instead"
fi

# 4. A target on no PATH directory at all: installed, and unreachable by name.
mkdir -p "$tmp/case4/nowhere"
run "/usr/bin:/bin" BIN="$tmp/case4/nowhere/mellions"
if [ "$rc" -eq 0 ]; then
	bad "an install onto no PATH directory was accepted"
elif ! printf '%s' "$err" | grep -q "no directory on PATH holds it"; then
	# Not a bare "PATH": the shadow refusal also says that word, and would
	# stand in for this one while the case it names went unchecked.
	bad "the refusal does not say PATH holds it nowhere: $err"
else
	note "a target on no PATH directory is refused"
fi

# 5. sudo: the plugin half would register into root's home rather than the
#    operator's, so the whole thing refuses before writing anything.
mkdir -p "$tmp/case5/bin"
run "$tmp/case5/bin:/usr/bin:/bin" PREFIX="$tmp/case5" SUDO_USER=someone
if [ "$rc" -eq 0 ]; then
	bad "sudo install was accepted"
elif [ -e "$tmp/case5/bin/mellions" ]; then
	bad "sudo install wrote a binary before refusing"
elif ! printf '%s' "$err" | grep -qi sudo; then
	bad "the refusal does not say why sudo is wrong: $err"
else
	note "sudo is refused, and nothing is written"
fi

[ "$fail" -eq 0 ] && printf 'ok install-binary\n'
exit "$fail"

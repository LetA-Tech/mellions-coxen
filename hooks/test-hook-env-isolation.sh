#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# A hook test reaches the same verdict inside an unattended shift as outside
# one. The hooks branch on variables scripts/shift.sh exports, so a test that
# inherits one instead of setting it tests the ambient session rather than the
# hook — and the arms that expect silence pass without running anything.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
self="$(basename "${BASH_SOURCE[0]}")"
fail=0
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

# A regular file, so no state can exist under it. Neither an empty directory
# nor a path that does not exist is poison here: every state-based test
# provisions what it later reads, and `mkdir -p` either finds the directory or
# creates it, so the leak is relocated rather than caught and the arm passes
# while testing nothing. `mkdir -p` cannot make a directory under a file, and
# unlike an unwritable directory that holds whoever runs it, root included.
poison_root=$(mktemp -d) || { bad "no temp directory, so nothing here is poisoned"; exit 1; }
trap 'rm -rf "$poison_root"' EXIT INT TERM HUP
poison_home="$poison_root/not-a-directory"
: > "$poison_home" || { bad "could not write the poison at $poison_home"; exit 1; }

# The value each shift variable takes inside a real shift session. Keyed by
# name so a variable added to shift.sh has to be given one here before this
# test will pass: an unpriced variable is one nothing poisons.
shift_value() {
  case "$1" in
    MELLIONS_DEADLINE) echo $(( $(date +%s) + 2700 )) ;;
    MELLIONS_HOME) printf '%s\n' "$poison_home" ;;
    *) return 1 ;;
  esac
}

# Fail closed on a new export rather than silently leaving it uncovered.
exported=$(grep -oE '^[[:space:]]*export[[:space:]]+MELLIONS_[A-Z_]+' "$root/scripts/shift.sh" \
  | grep -oE 'MELLIONS_[A-Z_]+' | sort -u)
[[ -n "$exported" ]] || bad "no exported MELLIONS_* found in scripts/shift.sh — this test has stopped reading it"

env_args=()
unset_args=()
for v in $exported; do
  unset_args+=(-u "$v")
  if val=$(shift_value "$v"); then
    env_args+=("$v=$val")
  else
    bad "scripts/shift.sh exports $v and shift_value() has no value for it: add one, then check every hooks/test-*.sh still passes with it set"
  fi
done

for t in "$root"/hooks/test-*.sh; do
  [[ -f "$t" ]] || continue
  name=$(basename "$t")
  [[ "$name" != "$self" ]] || continue
  # Expanded through `${a[@]+…}`: bash 3.2, which is what /usr/bin/env bash
  # finds on a stock macOS, kills the script on a plain "${empty[@]}" under
  # `set -u` — and empty is exactly the fail-closed path this file exists for.
  # A failure is attributed to a shift variable only when it repeats inside
  # and the same script, every shift variable unset, then passes. A failure
  # that does not repeat is a flake, and naming a variable for it misleads.
  if out=$(env ${env_args[@]+"${env_args[@]}"} bash "$t" 2>&1); then
    printf '  %s\n' "$name"
    continue
  fi
  if ! env ${env_args[@]+"${env_args[@]}"} bash "$t" >/dev/null 2>&1; then
    if env ${unset_args[@]+"${unset_args[@]}"} bash "$t" >/dev/null 2>&1; then
      bad "$name passes outside a shift and fails inside one, twice — it inherits a shift variable instead of setting it"
    else
      bad "$name fails outside a shift as well; this run cannot attribute it to a shift variable"
    fi
  else
    bad "$name failed inside a shift once and passed on a rerun: a flake, not attributable to a shift variable"
  fi
  printf '%s\n' "$out" | sed 's/^/    /' | tail -12
done

[[ $fail -eq 0 ]] && echo "ok  hook-env-isolation"
exit $fail

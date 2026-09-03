#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# bounded says when it cut. Its stdout can never exceed the limit, so a caller
# measuring stdout cannot tell a cut from a fit: the note on stderr is the only
# side of that comparison the cap does not produce.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

# shellcheck source=hooks/lib.sh
source "$root/hooks/lib.sh" <&-

err=$(mktemp)
trap 'rm -f "$err"' EXIT

# Under the limit: whole, and silent.
out=$(printf 'abcdefghij' | bounded 20 2>"$err")
[[ "$out" == "abcdefghij" ]] || bad "under the limit the body changed: $out"
[[ ! -s "$err" ]] || bad "under the limit it reported a cut: $(cat "$err")"

# Exactly the limit: whole, and still silent. head -c returns the limit whether
# or not it cut, so this is the case that separates "reached it" from "past it".
out=$(printf 'abcdefghij' | bounded 10 2>"$err")
[[ "$out" == "abcdefghij" ]] || bad "at the limit the body changed: $out"
[[ ! -s "$err" ]] || bad "at the limit it reported a cut: $(cat "$err")"

# Over the limit: cut to the limit, and loud about it.
out=$(printf 'abcdefghij' | bounded 4 2>"$err")
[[ "$out" == "abcd" ]] || bad "over the limit it did not cut to 4 bytes: $out"
grep -q 'cut at 4 bytes' "$err" || bad "over the limit it cut in silence: $(cat "$err")"

# The limit is a byte limit and the corpus is full of multi-byte punctuation:
# four em-dashes are 12 bytes, so a limit of 6 cuts inside the third character.
out=$(printf '————' | bounded 6 2>"$err")
[[ $(printf '%s' "$out" | wc -c) -eq 6 ]] || bad "the cap counted characters, not bytes"
grep -q 'cut at 6 bytes' "$err" || bad "a multi-byte cut was silent: $(cat "$err")"

# Trailing newlines survive. Every block this carries ends with one, and a
# caller that loses it joins its block to whatever the next hook prints.
out=$(printf 'ab\n\n' | bounded 20 2>"$err"; printf x)
[[ "$out" == $'ab\n\nx' ]] || bad "trailing newlines were dropped: $(printf '%q' "$out")"
[[ ! -s "$err" ]] || bad "an uncut body reported a cut: $(cat "$err")"

note "whole and silent under and at the limit, cut and loud past it"

[[ $fail -eq 0 ]] && echo "ok  lib-bounded"
exit $fail

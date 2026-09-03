#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The hook denies a pull request body that closes an issue by keyword, and
# stays silent on every neighbouring case — including a body that quotes the
# rule, which is how the rule itself gets written down.
#
# The predicate itself is a table in internal/prbody, which is where a case
# about grammar belongs. What is proven here is the wiring the table cannot
# see: the payload shape the runtime sends, a real checkout's origin/HEAD, a
# body on disk, a descriptor the runtime closed, and one line of JSON out.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hook="$root/hooks/pr-reference.sh"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP

# The hook is a wrapper over the binary, so the binary is what is under test
# and it is built here rather than taken from PATH. Before HOME is pinned,
# which the Go build cache wants real.
bin="$tmp/mellions"
if ! (cd "$root" && go build -o "$bin" ./cmd/mellions) 2>/dev/null; then
  bad "the binary the hook calls does not build"
  exit 1
fi

# The binary resolves a config, and the resolution ends at $HOME. Left
# inherited, this reads whichever config the operator has, so its result would
# depend on the machine and it would fail on a host that has none.
export HOME="$tmp/home"
export MELLIONS_CONFIG="$tmp/config.json"
mkdir -p "$HOME"
cat > "$MELLIONS_CONFIG" <<JSON
{"owner":"test","assignments_root":"$tmp/state/assignments","report_root":"$tmp/state"}
JSON

# A checkout whose origin/HEAD says the default branch is main, so the hook can
# read it without the tracker; and a directory that is no checkout at all.
repo="$tmp/repo"; mkdir -p "$repo"
git -C "$repo" init -q && git -C "$repo" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main
nowhere="$tmp/nowhere"; mkdir -p "$nowhere"
cwd="$repo"

# payload <command> — the shape the runtime sends, with the command JSON-escaped
# and the working directory the command runs in.
payload() {
  local c=${1//\\/\\\\}
  c=${c//\"/\\\"}
  c=${c//$'\n'/\\n}
  printf '{"session_id":"t","cwd":"%s","tool_name":"Bash","tool_input":{"command":"%s"}}' "$cwd" "$c"
}
run() { payload "$1" | MELLIONS_BIN="$bin" bash "$hook"; }

denies() {
  local out; out=$(run "$2")
  if grep -q '"permissionDecision":"deny"' <<<"$out"; then
    grep -q 'Refs #' <<<"$out" || bad "$1: denied without saying what to write instead"
  else
    bad "$1: not denied"
  fi
}
allows() {
  local out; out=$(run "$2")
  [[ -z "$out" ]] || bad "$1: denied, and should not have — $out"
}

denies "inline --body"        'gh pr create --base dev --title "x" --body "Closes #103

## Established
..."'
denies "lowercase closes"     'gh pr create --base dev --body "closes #7"'
denies "fixes"                'gh pr create --base=dev --body "Fixes #7 by rewriting the parser"'
denies "resolved"             'gh pr create -B dev --body "Resolved #7"'
denies "pr edit"              'gh pr edit 12 --base dev --body "Closes #7"'
denies "heredoc body-file -"  'gh pr create --base dev --body-file - <<EOF
Closes #7
EOF'

printf 'Closes #7\n\nproof follows\n' >"$tmp/body.md"
denies "--body-file path"     "gh pr create --base dev --body-file $tmp/body.md"

# A body that says the merge does not close the issue is the artifact the
# doctrine asks for. The first is the body this hook denied twice on a sibling
# repository, verbatim.
allows "does not close, emphasised" 'gh pr create --base dev --body "## Scope

Does **not** close #715 — its acceptance rule is `runtime-proof` and push delivery is dark.

Refs #715"'
allows "does not close"        'gh pr create --base dev --body "Does not close #715"'
allows "nothing closes"        'gh pr create --base dev --body "nothing closes #7 here"'
allows "no merge closes"       'gh pr create --base dev --body "no merge closes #7"'
allows "will not close"        'gh pr create --base dev --body "this will not close #7"'
allows "contracted negator"    'gh pr create --base dev --body "it doesn'"'"'t close #7"'
allows "cannot close"          'gh pr create --base dev --body "a merge into dev cannot close #7"'
allows "never fixes"           'gh pr create --base dev --body "this never fixes #7"'
allows "neither closes"        'gh pr create --base dev --body "this neither closes #7 nor fixes #8"'

# An auxiliary in front of the negator says the verb is what is denied, so the
# keyword is denied however many words stand in between. A bare negator opens a
# noun phrase instead, and there one word is the bound.
allows "auxiliary, two-word gap" 'gh pr create --base dev --body "This does not in fact close #7"'
allows "auxiliary, wider gap"   'gh pr create --base dev --body "The merge does not by itself close #7"'
allows "contraction, wider gap" 'gh pr create --base dev --body "It doesn'"'"'t on this base close #7"'
denies "and after a negation"   'gh pr create --base dev --body "This does not build and closes #7"'
denies "but after a negation"   'gh pr create --base dev --body "It doesn'"'"'t compile but closes #7"'
denies "or after a negation"    'gh pr create --base dev --body "The refactor is not finished or closes #7"'

# The negation must excuse its own clause and no more, or the fix hands back a
# silently unclosed issue — which is the whole hazard.
denies "negated, then genuine"  'gh pr create --base dev --body "Does not close #7, but this Closes #8"'
denies "negated, genuine after" 'gh pr create --base dev --body "Nothing here closes #7. Closes #8"'
denies "negator, prior line"    'gh pr create --base dev --body "Not a revert
Closes #7"'
denies "negator, prior bullet"  'gh pr create --base dev --body "- not blocked on review
- Closes #7"'
denies "negator, prior sentence" 'gh pr create --base dev --body "This is not a workaround.
Closes #7"'
# The same body through the other input path must reach the same verdict, and
# now does for the same reason: the body is decoded before it is read, so a
# newline is a newline on both.
printf 'Not a revert\nCloses #7\n' >"$tmp/prior.md"
denies "prior line, --body-file" "gh pr create --base dev --body-file $tmp/prior.md"

# A keyword and its number on either side of a soft wrap is still a close.
printf 'This closes\n#7 for good.\n' >"$tmp/wrap.md"
denies "wrapped keyword, file"  "gh pr create --base dev --body-file $tmp/wrap.md"
denies "wrapped keyword, body"  'gh pr create --base dev --body "This closes
#7 for good."'
# The wrap there falls between the keyword and its number, so a negator on the
# same line still stands in the keyword's own clause.
printf 'The migration does not close\n#7 here.\n' >"$tmp/wrapneg.md"
allows "wrapped negation"       "gh pr create --base dev --body-file $tmp/wrapneg.md"

# A negator that governs some other noun does not reach across the sentence to
# a genuine close.
denies "no + noun phrase"       'gh pr create --base dev --body "No behaviour change here closes #7"'
denies "nothing else changed"   'gh pr create --base dev --body "Nothing else changed and this closes #7"'
denies "short noun gap"         'gh pr create --base dev --body "No doubt this closes #7"'
denies "emphasised noun gap"    'gh pr create --base dev --body "**No** API change closes #7"'
denies "two-word noun gap"      'gh pr create --base dev --body "no tests yet closes #7"'

# Code spans are removed before this runs, and what they leave behind is one
# word — or the strip manufactures an adjacency the author did not write.
denies "code span in the gap"   'gh pr create --base dev --body "The regression is not in `pkg/sync` and this closes #7"'
denies "short span in the gap"  'gh pr create --base dev --body "not a `revert` closes #7"'

# A negator that is only the prefix of a word negates nothing.
denies "negator as a prefix"    'gh pr create --base dev --body "not-a-revert closes #7"'
denies "negator two clauses on" 'gh pr create --base dev --body "not that it matters; fixes #7"'
denies "negation inside a word" 'gh pr create --base dev --body "The unnoticed regression closes #7"'
denies "long gap after negator" 'gh pr create --base dev --body "not the branch that anybody expected here closes #7"'

# A keyword its author bolded away from the number is still a close.
denies "emphasised keyword"     'gh pr create --base dev --body "**Closes** #7"'

# On the default branch the keyword does what it says; whether the close is
# warranted is the closure Skill's question, not this hook's.
allows "no --base"             'gh pr create --body "Closes #7"'
allows "--base is the default" 'gh pr create --base main --body "Closes #7"'
allows "pr edit without --base" 'gh pr edit 12 --body "Closes #7"'

# Where the default branch cannot be read, a deny would be a guess.
cwd="$nowhere"
allows "unknown default branch" 'gh pr create --base dev --body "Closes #7"'
cwd="$repo"

# The case that must not fire: a pull request documenting this very rule, whose
# body carries the keyword inside a quotation.
allows "quoted in a code span" 'gh pr create --base dev --body "PR #83 merged with `Closes #75` in its body and #75 stayed open."'
allows "quoted in a doubled span" 'gh pr create --base dev --body "Opening a pull request whose Scope read ``Does **not** close #715 — its acceptance rule is `runtime-proof` and push delivery is dark``, with `Refs #715` at the bottom."'
allows "plain close in a doubled span" 'gh pr create --base dev --body "The denial named ``Closes #75 with `Refs #75` beneath it`` and that is the shape."'
allows "quoted in a fence"     'gh pr create --base dev --body "```
Closes #75
```
that is what not to write."'
allows "quoted in a blockquote" 'gh pr create --base dev --body "The denial named:

> Closes #75

and that is what not to write."'
allows "refs"                  'gh pr create --base dev --body "Refs #103

## Established
..."'
allows "no issue number"       'gh pr create --base dev --body "this closes the gap"'
allows "gh pr list"            'gh pr list --state all --json body | grep -i "Closes #"'
allows "gh pr view"            'gh pr view 89 --json body'

# Whose text it is. A command that opens no pull request has no pull request
# body, however much of one it carries.
allows "not a pr command"      'git commit -m "Closes #7"'
allows "echo of a pr command"  'echo '"'"'gh pr create --base dev --body "Closes #7"'"'"''
printf 'Refs #7\n\nWhy this is not a close.\n' >"$tmp/plain.md"
allows "commit then create"    "git add -A && git commit -m \"Closes #7 in the fixtures\" && git push -u origin HEAD && gh pr create --base dev --body-file $tmp/plain.md"
allows "heredoc writing a probe" "cat > $tmp/probe.sh <<'SH'
gh pr create --base dev --body \"closes #7\"
SH"
# The heredoc that writes the body has not run when this is decided, so the
# body is read where the command line is about to put it.
denies "heredoc into a body-file" "cat > $tmp/unwritten.md <<'EOF'
Closes #7
EOF
gh pr create --base dev --body-file $tmp/unwritten.md"

# A body-file that is named but absent must not make the hook fail or hang.
allows "missing --body-file"   "gh pr create --base dev --body-file $tmp/gone.md"

# Another tool's payload carrying the same words is not this hook's business.
out=$(printf '{"tool_name":"Write","tool_input":{"file_path":"/tmp/b.md","content":"gh pr create --body Closes #7"}}' | MELLIONS_BIN="$bin" bash "$hook")
[[ -z "$out" ]] || bad "fires on a non-Bash tool"

# The runtime may hand the hook a closed descriptor. It must return, not wait.
out=$(MELLIONS_BIN="$bin" bash "$hook" <&- 2>&1); rc=$?
[[ $rc -eq 0 && -z "$out" ]] || bad "closed stdin: rc=$rc out=$out"

# Deployment: the plugin travels with a checkout and the binary does not, so a
# host that has one without the other must fall silent rather than fail.
out=$(payload 'gh pr create --base dev --body "Closes #7"' | MELLIONS_BIN="$tmp/no-such-binary" bash "$hook" 2>&1); rc=$?
[[ $rc -eq 0 && -z "$out" ]] || bad "without the binary: rc=$rc out=$out"

# Denial output has to be one line of parseable JSON, or the runtime ignores it.
one=$(run 'gh pr create --base dev --body "Closes #103"')
[[ $(printf '%s' "$one" | wc -l | tr -d ' ') -le 1 ]] || bad "denial spans more than one line"
if command -v python3 >/dev/null 2>&1; then
  printf '%s' "$one" | python3 -c 'import json,sys; d=json.load(sys.stdin); h=d["hookSpecificOutput"]; assert h["hookEventName"]=="PreToolUse"; assert h["permissionDecision"]=="deny"; assert "Refs #103" in h["permissionDecisionReason"]' \
    || bad "denial is not the JSON the runtime reads"
  note "denial parses, names Refs #103"
fi

# hooks.json has to actually run it, or every assertion above is about a file
# nothing invokes.
grep -q 'pr-reference.sh' "$root/hooks/hooks.json" || bad "hooks.json does not run pr-reference.sh"

[[ $fail -eq 0 ]] && echo "ok  pr-reference"
exit $fail

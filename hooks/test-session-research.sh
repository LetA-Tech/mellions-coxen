#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The research method reaches the session whole: inside the runtime's preview
# limit, ending where the Skill ends, naming the two Skills it hands off to,
# and loud rather than silent when the Skill is missing.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

skill="$root/skills/mellions-deep-research/SKILL.md"
[[ -f "$skill" ]] || { bad "no skill at $skill"; exit 1; }

# The hook must not depend on stdin: it is run here with stdin closed, as the
# runtime may leave it, and must finish and deliver regardless.
out=$(CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-research.sh" <&-)

# Bytes, not characters: the cap is a byte cap and the corpus is full of
# multi-byte punctuation.
bytes=$(printf '%s' "$out" | wc -c | tr -d ' ')
[[ $bytes -le 9200 ]] || bad "block is $bytes bytes, over the 9200 the runtime previews"
note "block $bytes bytes"

[[ "$(head -1 <<<"$out")" == "# How you establish what is true" ]] || bad "the block does not open by saying what it is"

# Whole means whole: the Skill's last non-empty line is the block's last line,
# so a Skill that grows past the cap fails here rather than arriving cut.
last=$(grep -v '^[[:space:]]*$' "$skill" | tail -1)
[[ "$(grep -v '^[[:space:]]*$' <<<"$out" | tail -1)" == "$last" ]] || bad "the block ends before the Skill does — it is being cut"

# Frontmatter is for choosing a Skill; this one is delivered. Only the head of
# the block is checked, because a section break or a line that happens to
# start with "description:" is ordinary markdown further down.
head -3 <<<"$out" | grep -q '^---$' && bad "a frontmatter fence leaked into the block"
head -8 <<<"$out" | grep -q '^description:' && bad "the frontmatter leaked into the block"

for s in mellions-reasoning mellions-self-learning; do
  grep -qF -- "$s" <<<"$out" || bad "the block never names $s, so the session is not told when to load it"
done

# A missing Skill is said out loud, the way a missing identity is.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP
mkdir -p "$tmp/hooks" && cp "$root/hooks/session-research.sh" "$tmp/hooks/"
missing=$(CLAUDE_PLUGIN_ROOT="$tmp" bash "$tmp/hooks/session-research.sh" <&-)
grep -q 'RESEARCH METHOD NOT FOUND' <<<"$missing" || bad "a missing Skill produces no message, so a partial install is silent"

[[ $fail -eq 0 ]] && echo "ok  session-research"
exit $fail

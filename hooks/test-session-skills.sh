#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Every Skill this plugin ships reaches the session with the triggers that say
# when it applies, and the block stays inside the runtime's preview limit.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

# Run with stdin closed, as the runtime may leave it: the hook must not wait on it.
out=$(CLAUDE_PLUGIN_ROOT="$root" bash "$root/hooks/session-skills.sh" <&-)

bytes=$(printf '%s' "$out" | wc -c | tr -d ' ')
[[ $bytes -le 9200 ]] || bad "block is $bytes bytes, over the 9200 the runtime previews"
note "block $bytes bytes"

shopt -s nullglob
count=0
for f in "$root"/skills/*/SKILL.md; do
  name=$(basename "$(dirname "$f")")
  count=$((count + 1))
  grep -qF -- "**mellions:$name**" <<<"$out" || bad "$name is not named in the block"
  # A name without its situation is what this hook exists to prevent: assert
  # the opening of the skill's own description — the situation clause — survived
  # into the block whole.
  desc=$(awk '
    NR == 1 && $0 == "---" { fm = 1; next }
    fm && $0 == "---" { exit }
    fm && /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$f")
  probe=${desc:0:60}
  [[ -n "$probe" ]] || { bad "$name has no description to carry"; continue; }
  grep -qF -- "$probe" <<<"$out" || bad "$name reaches the session without its description"
done
[[ $count -gt 0 ]] || bad "no skills found under $root/skills"
note "$count skills, each with its description"

# The id has to be the one the Skill tool accepts, not the bare directory name.
grep -q 'Skill(skill: "mellions:' <<<"$out" || bad "the block does not show how to invoke a skill"

[[ $fail -eq 0 ]] && echo "ok  session-skills"
exit $fail

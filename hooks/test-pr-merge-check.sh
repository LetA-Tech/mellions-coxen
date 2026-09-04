#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# The hook denies a merge whose state cannot support the decision, and stays
# silent on every neighbouring case.
#
# The predicate is a table in internal/prmerge, which is where a case about
# which states are refused belongs. What is proven here is the wiring that
# table cannot see: the payload shape the runtime sends, the two tracker reads
# actually being made and their answers reaching the decision, one line of JSON
# out, a descriptor the runtime closed, and the off switch.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hook="$root/hooks/pr-merge-check.sh"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP

# The hook is a wrapper over the binary, so the binary is what is under test
# and it is built here rather than taken from PATH.
bin="$tmp/mellions"
if ! (cd "$root" && go build -o "$bin" ./cmd/mellions) 2>/dev/null; then
  bad "the binary the hook calls does not build"
  exit 1
fi

export HOME="$tmp/home"
export MELLIONS_CONFIG="$tmp/config.json"
mkdir -p "$HOME"
cat > "$MELLIONS_CONFIG" <<JSON
{"owner":"test","assignments_root":"$tmp/state/assignments","report_root":"$tmp/state"}
JSON

# A gh the test controls. The guard's whole input is what the tracker says, so
# a stub here is the only way to drive a state on purpose — and driving the
# state is the point: an answer that arrives by accident proves the wiring
# carried something, not that it carried this.
mkdir -p "$tmp/path"
cat > "$tmp/path/gh" <<'STUB'
#!/usr/bin/env bash
case "$1 $2" in
  "pr view")
    printf '%s\n' "$GH_PR_JSON" ;;
  "repo view")
    printf 'o/r\n' ;;
  "api "*|"api")
    printf '%s\n' "$GH_COMPARE_JSON" ;;
esac
exit 0
STUB
chmod +x "$tmp/path/gh"
export PATH="$tmp/path:$PATH"

payload() { printf '{"tool_name":"Bash","cwd":"%s","tool_input":{"command":"%s"}}' "$tmp" "$1"; }
runhook() { MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook"; }

# 1. Mergeability GitHub has not computed. The failure this was written for.
export GH_PR_JSON='{"number":177,"url":"https://github.com/o/r/pull/177","baseRefName":"dev","headRefOid":"abc","mergeStateStatus":"UNKNOWN","state":"OPEN","files":[]}'
export GH_COMPARE_JSON='{"ahead":0,"files":[]}'
out=$(payload 'gh pr merge 177 --squash' | runhook)
grep -q '"permissionDecision":"deny"' <<<"$out" ||
  bad "a merge with mergeability UNKNOWN was not denied: $out"
[[ $(wc -l <<<"$out") -eq 1 ]] ||
  bad "the decision is not one line of JSON: $out"
note "UNKNOWN mergeability: denied, one line of JSON"

# 2. Behind the base in a file the pull request also changes.
export GH_PR_JSON='{"number":42,"url":"https://github.com/o/r/pull/42","baseRefName":"dev","headRefOid":"abc","mergeStateStatus":"CLEAN","state":"OPEN","files":[{"path":"internal/a.go"},{"path":"internal/b.go"}]}'
export GH_COMPARE_JSON='{"ahead":10,"files":["internal/a.go","docs/x.md"]}'
out=$(payload 'gh pr merge 42' | runhook)
grep -q '"permissionDecision":"deny"' <<<"$out" ||
  bad "a stale merge overlapping newer work on the base was not denied: $out"
grep -q 'internal/a.go' <<<"$out" ||
  bad "the refusal does not name the overlapping file: $out"
grep -q 'docs/x.md' <<<"$out" &&
  bad "the refusal names a base-side file the pull request does not touch: $out"
note "behind in a shared file: denied, and only the shared file named"

# 3. Behind in no shared file. The negative that keeps the guard alive: this is
#    ordinary, and a guard that fires here is turned off and then protects
#    nothing at all.
export GH_COMPARE_JSON='{"ahead":10,"files":["docs/x.md"]}'
out=$(payload 'gh pr merge 42' | runhook)
[[ -z "$out" ]] || bad "a branch behind its base in no shared file was refused: $out"
note "behind in no shared file: silent"

# 4. Current and clean.
export GH_COMPARE_JSON='{"ahead":0,"files":[]}'
out=$(payload 'gh pr merge 42 --squash --delete-branch' | runhook)
[[ -z "$out" ]] || bad "a clean, current merge was refused: $out"
note "clean and current: silent"

# 5. The off switch, on the state that is otherwise refused.
export GH_PR_JSON='{"number":177,"url":"u","baseRefName":"dev","headRefOid":"abc","mergeStateStatus":"UNKNOWN","state":"OPEN","files":[]}'
out=$(payload 'gh pr merge 177' | MELLIONS_MERGE_CHECK=off MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" bash "$hook")
[[ -z "$out" ]] || bad "MELLIONS_MERGE_CHECK=off did not silence the guard: $out"
note "off switch: silent on a state it would otherwise refuse"

# 6. A command that is not a merge at all.
out=$(payload 'gh pr view 177 --json mergeable' | runhook)
[[ -z "$out" ]] || bad "a non-merge command produced a decision: $out"
note "not a merge: silent"

# 7. A descriptor the runtime closed. lib.sh bounds the read for this: on a
#    closed descriptor a plain cat does not return, and a hook that hangs is a
#    tool call the runtime has to kill.
out=$(MELLIONS_BIN="$bin" CLAUDE_PLUGIN_ROOT="$root" timeout 10 bash "$hook" <&- 2>/dev/null)
rc=$?
[[ $rc -ne 124 ]] || bad "the hook hung on a closed descriptor"
[[ -z "$out" ]] || bad "the hook decided something with no payload: $out"
note "closed descriptor: returns, decides nothing"

# 8. Hand-run with no payload. Silence and exit 0 reads as "checked, nothing
#    wrong", which is the one answer a guard that examined nothing must not
#    give.
err=$(MELLIONS_HOOK= "$bin" pr-merge-check </dev/null 2>&1 >/dev/null)
grep -q 'examined nothing' <<<"$err" ||
  bad "run by hand the guard does not say it examined nothing: $err"
note "run by hand: says it examined nothing"

[[ $fail -eq 0 ]] || exit 1
echo "ok  pr-merge-check"

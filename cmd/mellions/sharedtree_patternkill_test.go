// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"strings"
	"testing"
)

// The pattern-kill refusal is reached through the command a hook runs: a
// payload signalling by pattern is denied and named, and one signalling a pid
// or its children is silence.
func TestSharedTreeCheckDeniesAPatternKill(t *testing.T) {
	for _, cmd := range []string{
		`pkill -f "go test ./..." -u $(id -u)`,
		`killall golangci-lint`,
		`kill $(pgrep -f lint-gate.sh)`,
	} {
		out := runSharedTreeCheck(t, bashPayload(t, cmd))
		if !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, "not by a pid") {
			t.Errorf("%s: want a deny naming the pattern kill, got %q", cmd, out)
		}
	}
	for _, cmd := range []string{`kill "$runner"; kill -0 "$runner"`, `pkill -P "$runner"`, `pgrep -af lint-gate.sh`} {
		if out := runSharedTreeCheck(t, bashPayload(t, cmd)); out != "" {
			t.Errorf("%s: a pid-scoped signal was refused: %q", cmd, out)
		}
	}
}

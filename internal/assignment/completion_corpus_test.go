package assignment_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// TestTheChallengeIsSilentOnHonestHandoffs measures the false-positive rate
// against the only corpus that cannot have been written to pass: every handoff
// on this host, all of them written before the challenge existed.
//
// A gate is worth exactly its precision. This one fired on four of them before
// the qualification pass and the weakest phrase came out; it fires on none now,
// while both handoffs that actually failed still trip it. It fails rather than
// reports, because a later change to the phrases that starts challenging honest
// work has to be seen when it is made and not a month later.
func TestTheChallengeIsSilentOnHonestHandoffs(t *testing.T) {
	root := os.Getenv("MELLIONS_ASSIGNMENTS")
	if root == "" {
		t.Skip("no assignment root bound")
	}
	dirs, _ := filepath.Glob(filepath.Join(root, "*", "assignment.json"))
	if len(dirs) == 0 {
		t.Skip("no assignments on this host")
	}
	var total, claimed, answered int
	for _, p := range dirs {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var a struct {
			ID      string `json:"id"`
			Handoff string `json:"handoff"`
		}
		if json.Unmarshal(raw, &a) != nil || a.Handoff == "" {
			continue
		}
		total++
		if _, ok := assignment.Claims(a.Handoff); ok {
			claimed++
			if assignment.Reconciled(a.Handoff) {
				answered++
				t.Logf("CLAIM+ANSWERED %s", a.ID)
			} else {
				t.Logf("WOULD CHALLENGE %s", a.ID)
			}
		}
	}
	t.Logf("handoffs=%d claimed_completion=%d of_those_already_answered=%d would_challenge=%d",
		total, claimed, answered, claimed-answered)
	if total < 20 {
		t.Skipf("only %d handoffs here; too few to measure a rate against", total)
	}
	if would := claimed - answered; would != 0 {
		t.Errorf("%d of %d handoffs written before this gate existed would be challenged by it; "+
			"each is a session asked to justify work it did honestly", would, total)
	}
}

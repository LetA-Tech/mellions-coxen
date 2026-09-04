package claim_test

import (
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/claim"
)

// internal/assignment imports internal/claim, so claim cannot import the state
// vocabulary it has to branch on and carries its own copy. This is the external
// test package, which can import both, so the copy cannot drift: rename the
// state on one side and this fails rather than the claim quietly addressing
// every handed-off lane as a live one again.
func TestTheHandedOffStateClaimBranchesOnIsTheAssignmentsOwn(t *testing.T) {
	if claim.HandedOffState != assignment.StateHandedOff {
		t.Fatalf("claim branches on %q; assignment writes %q — a handed-off lane would be "+
			"published as a live one, refusing the reviewer its worktree is kept for",
			claim.HandedOffState, assignment.StateHandedOff)
	}
}

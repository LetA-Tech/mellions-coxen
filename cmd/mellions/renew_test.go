// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// laneStore writes assignment records straight to disk. Opening them through
// the store would cut worktrees and publish claims, and neither is what the
// resolution being tested reads.
func laneStore(t *testing.T, lanes ...*assignment.Assignment) *assignment.Store {
	t.Helper()
	root := t.TempDir()
	s, err := assignment.NewStore(root)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	for _, a := range lanes {
		dir := filepath.Join(root, a.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		b, err := os.OpenFile(filepath.Join(dir, "assignment.json"), os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.WriteString(mustJSON(t, a)); err != nil {
			t.Fatal(err)
		}
		b.Close()
	}
	return s
}

func mustJSON(t *testing.T, a *assignment.Assignment) string {
	t.Helper()
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLaneAtResolvesTheWorktreeTheSessionIsIn(t *testing.T) {
	base := t.TempDir()
	one := filepath.Join(base, "one")
	two := filepath.Join(base, "two")
	// A sibling whose path is a string prefix of another: /a/tree and
	// /a/tree-2 are different lanes, and a prefix match without the separator
	// puts a session in the wrong one.
	twoSuffix := filepath.Join(base, "two-2")
	for _, d := range []string{filepath.Join(one, "internal"), two, twoSuffix} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := laneStore(t,
		&assignment.Assignment{ID: "lane-one", Repo: "repo-a", Branch: "mellions/one", Worktree: one, Objective: "first", State: "active"},
		&assignment.Assignment{ID: "lane-two", Repo: "repo-b", Branch: "mellions/two", Worktree: two, Objective: "second", State: "active"},
	)

	for _, tc := range []struct {
		name, cwd, want string
	}{
		{"at the worktree root", one, "lane-one"},
		{"below the worktree", filepath.Join(one, "internal"), "lane-one"},
		{"the other lane", two, "lane-two"},
		{"a sibling that only shares a prefix", twoSuffix, ""},
		{"outside every lane", base, ""},
		{"no working directory at all", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ""
			if a := laneAt(s, tc.cwd); a != nil {
				got = a.ID
			}
			if got != tc.want {
				t.Fatalf("laneAt(%q) = %q, want %q", tc.cwd, got, tc.want)
			}
		})
	}
}

// A closed lane's worktree is gone, but the record survives and a later session
// can be standing in a directory of the same name. Resolution reads open lanes
// only, so a finished lane never gets named as the one in flight.
func TestLaneAtIgnoresClosedLanes(t *testing.T) {
	dir := t.TempDir()
	s := laneStore(t, &assignment.Assignment{
		ID: "done", Repo: "repo", Branch: "b", Worktree: dir, Objective: "o", State: "closed",
	})
	if a := laneAt(s, dir); a != nil {
		t.Fatalf("a closed lane was named as the one in flight: %s", a.ID)
	}
}

// The instructions are what the runtime is handed, so what they must contain is
// asserted against literals the builder cannot reach: the lane's own fields,
// and the words that carry the contract.
func TestRenewalInstructionsCarryTheLaneAndTheContract(t *testing.T) {
	lane := &assignment.Assignment{
		ID:        "ctx-renewal",
		Repo:      "mellions-coxen",
		Branch:    "mellions/ctx-renewal",
		Worktree:  "/tmp/lane/tree",
		Issue:     "#77",
		Objective: "renew a session's working context\nsecond line that must not appear",
	}
	out := renewalInstructions(lane, "auto")

	for _, want := range []string{
		"ctx-renewal", "mellions-coxen", "mellions/ctx-renewal", "/tmp/lane/tree", "#77",
		"renew a session's working context",
		"mellions assign get ctx-renewal",
		"established", "inferred", "assumed", "unknown",
		"needs the owner",
		"container", "tunnel",
		"next step",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the instructions never say %q, so the summary is not asked to keep it", want)
		}
	}
	// The objective is one line in a block of instructions: a multi-line
	// objective pasted whole turns the lane paragraph into prose.
	if strings.Contains(out, "second line that must not appear") {
		t.Error("the objective was pasted past its first line")
	}
	// The trigger is the difference between "the runtime did this, mid-work"
	// and a compaction somebody asked for.
	if !strings.Contains(out, "runtime's, on its own threshold") {
		t.Error("an auto compaction is not told it was the runtime's own")
	}
	if strings.Contains(renewalInstructions(lane, "manual"), "runtime's, on its own threshold") {
		t.Error("a manual compaction is told it was automatic")
	}
}

// No lane is the ordinary case for a session surveying the estate. The
// instructions still have to say what to keep, and must not invent a lane.
func TestRenewalInstructionsWithoutALane(t *testing.T) {
	out := renewalInstructions(nil, "auto")
	if !strings.Contains(out, "none — the working directory is not a lane's worktree") {
		t.Error("a session with no lane is not told so")
	}
	if !strings.Contains(out, "KEEP, in full") || !strings.Contains(out, "LET GO") {
		t.Error("the keep/let-go contract is missing when there is no lane")
	}
	if strings.Contains(out, "mellions assign get ") {
		t.Error("an assignment id was named when there is no lane")
	}
}

// The handoff is the completion point, and it is the only place a session is
// told that renewing from here costs nothing.
func TestHandoffSaysWhereTheRenewalPointIs(t *testing.T) {
	for _, want := range []string{"renewal point", "on the record", "costs nothing"} {
		if !strings.Contains(renewalHandoffNote, want) {
			t.Errorf("the handoff note never says %q", want)
		}
	}
}

// The usage line has to name the PreCompact hook: a command whose only caller
// is a hook is undiscoverable otherwise, and the constraint that no session can
// start its own compaction is the fact a reader most needs.
func TestRenewUsageNamesTheHookAndTheConstraint(t *testing.T) {
	for _, want := range []string{"mellions renew", "PreCompact", "Nothing in either runtime lets a"} {
		if !strings.Contains(renewUsage, want) {
			t.Errorf("the usage never says %q", want)
		}
	}
	if !strings.Contains(usage, renewUsage) {
		t.Error("renew is not in the top-level usage, so nothing lists it")
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// assignedFinder is read from the store on disk, written in the shape the
// store writes it, so a record the store would not list cannot pass here.
func TestAssignedFinderAnswersForThisSessionsOpenLaneOnly(t *testing.T) {
	root := t.TempDir()
	write := func(id, state, session string) string {
		dir := filepath.Join(root, id)
		tree := filepath.Join(dir, "tree")
		if err := os.MkdirAll(tree, 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"id":"` + id + `","repo":"data-service","objective":"x","state":"` + state +
			`","worktree":"` + tree + `","sessions":[{"runtime":"claude","id":"` + session +
			`","first":"2026-09-27T00:00:00Z","last":"2026-09-27T00:00:00Z"}]}`
		if err := os.WriteFile(filepath.Join(dir, "assignment.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return tree
	}
	activeTree := write("lane-active", "active", "s-active")
	write("lane-blocked", "blocked", "s-blocked")
	handedTree := write("lane-handed", "handed_off", "s-handed")

	assigned := assignedFinder(&Config{AssignmentsRoot: root})
	for _, tc := range []struct {
		name, session, cwd string
		want               bool
	}{
		{"session recorded on an active lane", "s-active", "/tmp", true},
		{"session recorded on a blocked lane", "s-blocked", "/tmp", true},
		{"session standing in an active lane's tree", "other", filepath.Join(activeTree, "internal"), true},
		{"session recorded only on a handed-off lane", "s-handed", "/tmp", false},
		{"session standing in a handed-off lane's tree", "other", handedTree, false},
		{"session holding nothing", "other", "/tmp", false},
	} {
		if got := assigned(tc.session, tc.cwd); got != tc.want {
			t.Errorf("%s: assigned=%v, want %v", tc.name, got, tc.want)
		}
	}
	if assignedFinder(&Config{AssignmentsRoot: filepath.Join(root, "absent")})("s-active", "/tmp") {
		t.Error("a store that does not exist answered yes")
	}
}

// Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// lane writes one open assignment whose worktree is dir.
func lane(t *testing.T, root, id, tree string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, id), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{
		"id": id, "repo": "policy-service", "worktree": tree, "state": "active",
		"objective": "o", "because": "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, id, "assignment.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestARecordGoesToTheLaneBeingWorked.
//
// Thirty-three records were written across one twenty-four hour session and
// none reached the lane that carried the implementation: the id in hand was the
// one opened first, and every record went there. Working memory that names the
// wrong objective is read by the next session under the wrong objective, and a
// crash mid-implementation leaves no record of the implementation at all.
func TestARecordGoesToTheLaneBeingWorked(t *testing.T) {
	root := t.TempDir()
	earlier, current := t.TempDir(), t.TempDir()
	lane(t, root, "qualification-lane", earlier)
	lane(t, root, "implementation-lane", current)
	store := &assignment.Store{Root: root}

	t.Chdir(current)
	got := laneHere(store)
	if got == nil {
		t.Fatal("standing in a lane's worktree resolved no lane; a record here would need the id by hand")
	}
	if got.ID != "implementation-lane" {
		t.Errorf("lane here = %s, want implementation-lane", got.ID)
	}

	// A subdirectory of the worktree is still that lane: work happens in the
	// package, not at the root.
	sub := filepath.Join(current, "internal", "adapters")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	if got := laneHere(store); got == nil || got.ID != "implementation-lane" {
		t.Errorf("from a subdirectory the lane did not resolve: %v", got)
	}
}

// The half that decides whether this is a default or a guess. A tree that is
// nobody's lane must resolve nothing, or a record made in a shared checkout
// lands on whichever lane happened to sort first.
func TestATreeThatIsNoLaneResolvesNothing(t *testing.T) {
	root := t.TempDir()
	lane(t, root, "some-lane", t.TempDir())
	store := &assignment.Store{Root: root}

	t.Chdir(t.TempDir())
	if got := laneHere(store); got != nil {
		t.Errorf("a tree belonging to no lane resolved %s; a default that guesses is worse than none", got.ID)
	}
}

// A sibling directory whose path merely shares a prefix is not inside the
// worktree. Comparing as strings without the separator makes /x/tree-old read
// as inside /x/tree.
func TestAPrefixOfTheWorktreePathIsNotInsideIt(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	tree := filepath.Join(base, "tree")
	sibling := filepath.Join(base, "tree-old")
	for _, d := range []string{tree, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	lane(t, root, "the-lane", tree)
	store := &assignment.Store{Root: root}

	t.Chdir(sibling)
	if got := laneHere(store); got != nil {
		t.Errorf("%s resolved as inside %s (lane %s)", sibling, tree, got.ID)
	}
}

// TestRecordWithNoIdReachesTheLaneBeingWorked.
//
// The verb is run for real against a store, because the resolution being tested
// is not the one the caller does: a test of laneHere alone stays green with the
// default removed from `assign record` entirely, which is the whole change.
func TestRecordWithNoIdReachesTheLaneBeingWorked(t *testing.T) {
	cfg := idShapeConfig(t, claimRepo(t))
	store, _, err := assignStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	earlier, current := t.TempDir(), t.TempDir()
	lane(t, store.Root, "opened-first", earlier)
	lane(t, store.Root, "being-worked", current)

	t.Chdir(current)
	if err := assignRecord([]string{"-config", cfg, "-kind", "found", "what this lane established"}); err != nil {
		t.Fatalf("assign record with no id, inside a lane: %v", err)
	}

	worked, err := store.Get("being-worked")
	if err != nil {
		t.Fatal(err)
	}
	if len(worked.Findings) != 1 {
		t.Errorf("the lane being worked holds %d findings, want 1 — the record went somewhere else",
			len(worked.Findings))
	}
	first, err := store.Get("opened-first")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Findings) != 0 {
		t.Errorf("the lane opened first holds %d findings, want 0", len(first.Findings))
	}
}

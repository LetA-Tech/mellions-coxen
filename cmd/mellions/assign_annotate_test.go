// Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestARecordFromAnotherLaneDoesNotReachTheTracker.
//
// The verb is run for real, with a gh on PATH that logs every call, because
// the choice between restating and not is made here and nowhere else: a store
// test of Annotate stays green with `assign record` calling Record for every
// write. A session standing in its own lane and recording onto a different one
// is a reader; restating the claim then kept a finished lane's hold fresh and
// posted its claim again onto an issue closed twelve days before.
func TestARecordFromAnotherLaneDoesNotReachTheTracker(t *testing.T) {
	bin := t.TempDir()
	calls := filepath.Join(t.TempDir(), "gh.log")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + calls + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "a-reader")

	cfg := idShapeConfig(t, claimRepo(t))
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["owner"] = "probe-owner"
	if raw, err = json.Marshal(m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := assignStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if store.Tracker == nil {
		t.Fatal("setup: an owner is configured and the store has no tracker, so nothing here could reach one")
	}

	finished, current := t.TempDir(), t.TempDir()
	for id, tree := range map[string]string{"finished-lane": finished, "reading-lane": current} {
		if err := os.MkdirAll(filepath.Join(store.Root, id), 0o755); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(map[string]any{
			"id": id, "repo": "probe-repo", "issue": "#7", "worktree": tree,
			"state": "handed_off", "objective": "o", "because": "b",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(store.Root, id, "assignment.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	logged := func() string {
		b, err := os.ReadFile(calls)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Chdir(current)
	if err := assignRecord([]string{"-config", cfg, "-kind", "found", "finished-lane", "read from another lane"}); err != nil {
		t.Fatalf("assign record onto another lane: %v", err)
	}
	if got := logged(); got != "" {
		t.Errorf("a record from another lane called the tracker:\n%s", got)
	}
	fin, err := store.Get("finished-lane")
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.Findings) != 1 {
		t.Fatalf("the note did not reach the lane it names: %d findings", len(fin.Findings))
	}

	// The control: the lane's own record, its id given explicitly, still
	// restates, so this gh is one the verb does reach.
	if err := assignRecord([]string{"-config", cfg, "-kind", "note", "reading-lane", "the lane's own work"}); err != nil {
		t.Fatalf("assign record in the lane's own tree: %v", err)
	}
	if logged() == "" {
		t.Error("the lane's own record never called the tracker; this test cannot see a restate")
	}
}

// Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/presence"
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

// TestARecordFromNoLaneLeavesTheClaimUntilTheSessionTakesTheLaneUp.
//
// A shift session's working directory is the Mellions home, which is no lane:
// a note it writes there onto a lane it only read restated that lane's claim
// and stamped it as one of its sessions, keeping a dead lane's hold fresh. A
// session that took the lane up with `assign open <id>` and records from the
// same directory is the lane's own, and its note still restates.
func TestARecordFromNoLaneLeavesTheClaimUntilTheSessionTakesTheLaneUp(t *testing.T) {
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

	const id = "held-lane"
	if err := os.MkdirAll(filepath.Join(store.Root, id), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{
		"id": id, "repo": "probe-repo", "issue": "#7", "worktree": t.TempDir(),
		"state": "active", "objective": "o", "because": "b",
		"sessions": []map[string]any{{"runtime": "claude", "id": "the-holder"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Root, id, "assignment.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	logged := func() string {
		b, err := os.ReadFile(calls)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}
	reset := func() {
		if err := os.Remove(calls); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	readerSeen := func() bool {
		a, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range a.Sessions {
			if s.Runtime == "claude" && s.ID == "a-reader" {
				return true
			}
		}
		return false
	}

	t.Chdir(t.TempDir())
	if err := assignRecord([]string{"-config", cfg, "-kind", "found", id, "read from no lane"}); err != nil {
		t.Fatalf("assign record from no lane: %v", err)
	}
	if got := logged(); got != "" {
		t.Errorf("a record from no lane by a session that never worked the lane called the tracker:\n%s", got)
	}
	if readerSeen() {
		t.Error("a record from no lane stamped a session that never worked the lane as one of its sessions")
	}
	a, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Findings) != 1 {
		t.Fatalf("the note did not reach the lane it names: %d findings", len(a.Findings))
	}

	// While the holder runs, opening the lane meets the collision: the session
	// is shown the holder and is not stamped as one of the lane's sessions.
	live := map[string]presence.Session{"the-holder": {Runtime: "claude", ID: "the-holder"}}
	_, handled, holder, err := claimExisting(store, id, live)
	if err != nil || !handled {
		t.Fatalf("assign open on the live-held lane: handled=%v err=%v", handled, err)
	}
	if holder == nil || holder.ID != "the-holder" {
		t.Errorf("assign open on a live-held lane did not report its holder, so the session is never told it was not taken up: %+v", holder)
	} else if note := heldElsewhere(id, *holder); !strings.Contains(note, "the-holder") || !strings.Contains(note, "not taken up") {
		t.Errorf("the collision note does not name the holder and say the session was not taken up:\n%s", note)
	}
	if readerSeen() {
		t.Error("assign open on a lane a live session holds stamped the session that only met the collision")
	}

	// Taking the lane up is the act that makes this session the lane's own,
	// and the record it is shown still names who worked the lane before it.
	got, handled, holder, err := claimExisting(store, id, nil)
	if err != nil || !handled {
		t.Fatalf("assign open on the active lane: handled=%v err=%v", handled, err)
	}
	if holder != nil {
		t.Errorf("assign open with no live holder reported one: %+v", holder)
	}
	if last, ok := got.Latest(); !ok || last.ID != "the-holder" {
		t.Errorf("assign open showed %+v as the lane's last session, not the holder it collided with", last)
	}
	if !readerSeen() {
		t.Error("assign open on an active lane did not stamp the session taking it up")
	}
	reset()
	if err := assignRecord([]string{"-config", cfg, "-kind", "note", id, "the continuing session's work"}); err != nil {
		t.Fatalf("assign record after taking the lane up: %v", err)
	}
	if logged() == "" {
		t.Error("a record by the session that took the lane up did not call the tracker, so its claim would expire under it")
	}

	// With no runtime session there is no session to judge: the write restates.
	reset()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	if err := assignRecord([]string{"-config", cfg, "-kind", "next", id, "the runner's note"}); err != nil {
		t.Fatalf("assign record with no session: %v", err)
	}
	if logged() == "" {
		t.Error("a record with no runtime session behind it no longer restated the claim")
	}
}

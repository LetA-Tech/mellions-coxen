// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// TestBriefSaysWhenALiveSessionHoldsTheLane reproduces the shape that had two
// unattended shifts, launched fifteen seconds apart, both pointed at one lane.
//
// The second shift's session-start brief carried `verify-42 (active, 0m
// ago)` and `last worked in claude session eca2e34c — claude --resume … if it
// still opens`, while that session was the first shift's and was running. The
// record cannot tell the two apart; the presence store can, and until this it
// was never asked.
func TestBriefSaysWhenALiveSessionHoldsTheLane(t *testing.T) {
	const peer = "eca2e34c-d103-44b5-88e4-122d24fb4285"
	t.Setenv("CLAUDE_CODE_SESSION_ID", peer)

	store, err := assignment.NewStore(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(assignment.OpenOptions{
		ID: "verify-42", Repo: "mellions-coxen",
		Objective: "Establish whether the defect #31 describes is gone",
		Because:   "the owner asked for it", Source: claimRepo(t),
	}); err != nil {
		t.Fatal(err)
	}

	// Nothing live. This is the positive control: it fixes that the test can
	// see the invitation at all, so the assertion below is not one that would
	// pass against any output.
	dormant := brief(t, store, nil)
	if !strings.Contains(dormant, "claude --resume "+peer) {
		t.Fatalf("with no live session the brief did not offer the resume it has always offered; got:\n%s", dormant)
	}
	if strings.Contains(dormant, "HELD RIGHT NOW") {
		t.Errorf("a lane no running process holds was reported held — every dormant lane would be fenced off:\n%s", dormant)
	}

	// The same record, with that session's process running.
	live := brief(t, store, map[string]presence.Session{peer: {
		ID: peer, Runtime: "claude", PID: 91273,
		Cwd: "/Users/you/.mellions/assignments/verify-42/tree",
	}})
	if !strings.Contains(live, "HELD RIGHT NOW") {
		t.Errorf("a lane a live session is working was offered as work in flight with no sign a peer holds it — this is what let two shifts take one lane:\n%s", live)
	}
	if strings.Contains(live, "claude --resume "+peer) {
		t.Errorf("the brief still invited the session to resume a session that is running; got:\n%s", live)
	}
	if !strings.Contains(live, "91273") {
		t.Errorf("the holder is named without the process to check it against; got:\n%s", live)
	}
}

// TestHeldNowDropsThisSession: the session-start hook registers this session
// before the brief renders, so this session is in the live set. Left in, it
// reads its own lane as a peer's and stands off from the work it was opened
// for — a fence that stops the wrong session.
func TestHeldNowDropsThisSession(t *testing.T) {
	me := presence.Session{ID: "self-id", Runtime: "claude", PID: 111}
	peer := presence.Session{ID: "peer-id", Runtime: "claude", PID: 222}
	all := []presence.Session{me, peer}

	byID := heldNow(all, "self-id", 0)
	if _, ok := byID["self-id"]; ok {
		t.Error("this session was left in the held set, matched by id")
	}
	if _, ok := byID["peer-id"]; !ok {
		t.Error("the peer was dropped from the held set — the fence catches nothing")
	}

	// A reopened conversation registers under a second id, so the environment's
	// id can miss a record that is still this process.
	byPID := heldNow(all, "", 111)
	if _, ok := byPID["self-id"]; ok {
		t.Error("this session was left in the held set, matched by process")
	}
	if _, ok := byPID["peer-id"]; !ok {
		t.Error("the peer was dropped from the held set — the fence catches nothing")
	}
}

// brief renders the session-start in-flight brief and returns what a session
// would read.
func brief(t *testing.T, as *assignment.Store, held map[string]presence.Session) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = continueBrief(as, "", held)
	os.Stdout = stdout
	_ = w.Close()
	out, readErr := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("continueBrief: %v", err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(out)
}

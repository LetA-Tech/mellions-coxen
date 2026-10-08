package assignment

import (
	"testing"
	"time"
)

// A note written onto a lane by a session working a different lane is no
// evidence the lane is worked. Restating its claim on that write keeps a dead
// lane's hold fresh for another expiry window, and posts its claim again onto
// an issue that may have closed long ago.
func TestANoteFromAnotherLaneLeavesTheClaimAsItStood(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	at := time.Date(2026, 9, 26, 0, 32, 0, 0, time.UTC)
	f.now = func() time.Time { return at }
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "opener")

	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "a lane that finished", Because: "it had work",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Handoff("svc-7", "done and pushed"); err != nil {
		t.Fatal(err)
	}
	stated := f.claims[key("svc", "#7")]
	if len(stated) != 1 || !stated[0].At.Equal(at) {
		t.Fatalf("setup: the handoff did not state the claim at %s: %+v", at, stated)
	}

	at = at.Add(12 * 24 * time.Hour)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "reader")
	if err := s.Annotate("svc-7", "found", "read by another lane; nothing changed"); err != nil {
		t.Fatal(err)
	}
	got := f.claims[key("svc", "#7")]
	if len(got) != 1 || !got[0].At.Equal(stated[0].At) {
		t.Fatalf("a note from another lane restated the claim: was %s, now %+v", stated[0].At, got)
	}
	a, err := s.Get("svc-7")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(a.Findings); n == 0 || a.Findings[n-1].Text != "read by another lane; nothing changed" {
		t.Fatalf("the note was not recorded: %+v", a.Findings)
	}
	if a.workedBy([]Session{{Runtime: "claude", ID: "reader"}}) {
		t.Fatalf("the reader was stamped as one of the lane's sessions: %+v", a.Sessions)
	}

	// The session that opened the lane, writing from another lane's tree, is
	// still the lane's own: its note restates.
	at = at.Add(time.Hour)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "opener")
	if err := s.Annotate("svc-7", "note", "the opener, from its other lane"); err != nil {
		t.Fatal(err)
	}
	if got := f.claims[key("svc", "#7")]; len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("the lane's own session was treated as a reader: want %s, got %+v", at, got)
	}

	// Record restates whoever writes: that is what keeps a worked lane from
	// expiring.
	at = at.Add(time.Hour)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "reader")
	if err := s.Record("svc-7", "note", "the lane's own session"); err != nil {
		t.Fatal(err)
	}
	got = f.claims[key("svc", "#7")]
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("the lane's own record did not restate the claim at %s: %+v", at, got)
	}
}

// Taking up an active lane stamps the session and restates the claim, so that
// session's later notes, wherever written, are the lane's own.
func TestTakingUpAnActiveLaneMakesTheSessionItsOwn(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	at := time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC)
	f.now = func() time.Time { return at }
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "opener")
	if _, err := s.Open(OpenOptions{
		ID: "svc-8", Repo: "svc", Issue: "#8", Source: src,
		Objective: "a lane whose session died", Because: "it had work",
	}); err != nil {
		t.Fatal(err)
	}

	at = at.Add(time.Hour)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "continuer")
	a, err := s.Take("svc-8")
	if err != nil {
		t.Fatal(err)
	}
	if !a.WorkedHere() {
		t.Fatalf("Take did not stamp the continuing session: %+v", a.Sessions)
	}
	if got := f.claims[key("svc", "#8")]; len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("Take did not restate the claim at %s: %+v", at, got)
	}

	at = at.Add(time.Hour)
	if err := s.Annotate("svc-8", "found", "from outside the tree"); err != nil {
		t.Fatal(err)
	}
	if got := f.claims[key("svc", "#8")]; len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("the continuing session's note was treated as a reader's: want %s, got %+v", at, got)
	}

	if err := s.Handoff("svc-8", "stands"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Take("svc-8"); err == nil {
		t.Fatal("Take accepted a handed-off lane, which is reopened, not taken")
	}
}

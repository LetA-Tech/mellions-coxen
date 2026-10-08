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

	// The lane's own write still restates: that is what keeps a worked lane
	// from expiring, and it is the control that shows this tracker can move.
	if err := s.Record("svc-7", "note", "the lane's own session"); err != nil {
		t.Fatal(err)
	}
	got = f.claims[key("svc", "#7")]
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("the lane's own record did not restate the claim at %s: %+v", at, got)
	}
}

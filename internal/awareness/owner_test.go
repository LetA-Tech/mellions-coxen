// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

func at(t *testing.T, s string) time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return when
}

func ownerNotes(o *awareness.OwnerPresence) []awareness.Note {
	return awareness.Notes(awareness.Observation{Owner: o})
}

func TestAHostWhoseOwnerHasNeverSaidIsToldNothing(t *testing.T) {
	if n := ownerNotes(nil); len(n) != 0 {
		t.Fatalf("a session was told where the owner was when nothing had recorded it: %+v", n)
	}
}

func TestTheAwayNoteCarriesWhatAnUnreachableOwnerChanges(t *testing.T) {
	notes := ownerNotes(&awareness.OwnerPresence{Away: true, Since: at(t, "2026-08-29T22:10:00Z")})
	if len(notes) == 0 {
		t.Fatal("a session was told nothing when the owner had left the room")
	}
	whole := notes[0].Because + " " + notes[0].Do + " " + notes[0].Why
	for _, want := range []string{
		"stepped away", "not reachable", "reversible", "decision package", "2026-08-29T22:10:00Z",
	} {
		if !strings.Contains(whole, want) {
			t.Errorf("the away note never says %q:\n%s", want, whole)
		}
	}
	if notes[0].Ident == "" {
		t.Error("the away note has no Ident, so a second departure in one session is swallowed as the first")
	}
}

func TestWhereTheyWentAndWhenTheyAreBackReachTheSession(t *testing.T) {
	notes := ownerNotes(&awareness.OwnerPresence{
		Away:    true,
		Since:   at(t, "2026-08-29T22:10:00Z"),
		Until:   at(t, "2026-08-30T08:00:00Z"),
		Because: "overnight",
	})
	for _, want := range []string{"2026-08-30T08:00:00Z", "overnight"} {
		if !strings.Contains(notes[0].Because, want) {
			t.Errorf("the away note drops %q, which is what says how long to work: %s", want, notes[0].Because)
		}
	}
}

func TestLeavingAndReturningAreTwoFactsASessionIsToldSeparately(t *testing.T) {
	away := ownerNotes(&awareness.OwnerPresence{Away: true, Since: at(t, "2026-08-29T22:10:00Z")})
	back := ownerNotes(&awareness.OwnerPresence{Since: at(t, "2026-08-30T07:30:00Z")})
	if len(away) == 0 || len(back) == 0 {
		t.Fatal("one of the two halves said nothing")
	}
	if away[0].Key() == back[0].Key() {
		t.Fatal("leaving and returning share a key, so a session told one is never told the other")
	}
	if !strings.Contains(back[0].Because, "back") || !strings.Contains(back[0].Do, "digest") {
		t.Errorf("the return note does not say they are back, or does not point at what they missed: %+v", back[0])
	}
	// A second departure in the same session is a different fact again: Said
	// holds keys forever, so a shared one is a note nobody ever hears twice.
	again := ownerNotes(&awareness.OwnerPresence{Away: true, Since: at(t, "2026-08-30T19:00:00Z")})
	if again[0].Key() == away[0].Key() {
		t.Fatal("two departures share a key, so the second is swallowed as already said")
	}
}

func TestALapsedWindowSaysSoRatherThanClaimingTheOwnerSpoke(t *testing.T) {
	notes := ownerNotes(&awareness.OwnerPresence{
		Since:  at(t, "2026-08-29T22:10:00Z"),
		Until:  at(t, "2026-08-30T08:00:00Z"),
		Lapsed: true,
	})
	if len(notes) == 0 {
		t.Fatal("a lapsed window told the session nothing, so it goes on treating the host as unattended")
	}
	if strings.Contains(notes[0].Because, "is back as of") {
		t.Errorf("a window running out was reported as the owner saying they were back: %s", notes[0].Because)
	}
	if !strings.Contains(notes[0].Because, "2026-08-30T08:00:00Z") {
		t.Errorf("the lapsed note does not say when the window ran out: %s", notes[0].Because)
	}
}

// Whether anybody can answer decides what to do with everything else, so it is
// not something to reach the session after a page of peers and documents.
func TestTheOwnerLeavingIsSaidBeforeAnythingElse(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{
		Owner:     &awareness.OwnerPresence{Away: true, Since: at(t, "2026-08-29T22:10:00Z")},
		Idle:      true,
		Repo:      "mellions-coxen",
		Elsewhere: []awareness.Peer{{Describe: "claude session abc on mellions-coxen"}},
	})
	if len(notes) < 2 {
		t.Fatalf("the observation produced %d notes, so the ordering is not being tested", len(notes))
	}
	if !strings.Contains(notes[0].Because, "stepped away") {
		t.Errorf("something else reached the session before the owner leaving: %s", notes[0].Because)
	}
}

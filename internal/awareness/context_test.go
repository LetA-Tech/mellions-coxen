// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness

import (
	"strings"
	"testing"
	"time"
)

func TestAPeerElsewhereIsAnOpeningToMake(t *testing.T) {
	o := Observation{Repo: "data-service", Elsewhere: []Peer{{Describe: "claude session 1234 on data-service"}}}
	var got Note
	for _, n := range Notes(o) {
		if strings.Contains(n.Because, "from another tree") {
			got = n
		}
	}
	for _, want := range []string{"`ListAgents`", "SendMessage(to: <name>", "what are you on?", "tell them what you establish", "ask", "mellions:mellions-territory"} {
		if !strings.Contains(got.Do, want) {
			t.Errorf("the peer note does not say %q: %q", want, got.Do)
		}
	}
}

func TestAGrownContextIsSaidOncePerBucket(t *testing.T) {
	under := Observation{ContextBytes: 1_400_000, RenewAt: 1_500_000}
	for _, n := range Notes(under) {
		if strings.HasPrefix(n.Ident, "context-") {
			t.Fatalf("said under the size: %q", n.Because)
		}
	}
	var first, third Note
	for _, n := range Notes(Observation{ContextBytes: 1_600_000, RenewAt: 1_500_000}) {
		if strings.HasPrefix(n.Ident, "context-") {
			first = n
		}
	}
	for _, n := range Notes(Observation{ContextBytes: 4_600_000, RenewAt: 1_500_000}) {
		if strings.HasPrefix(n.Ident, "context-") {
			third = n
		}
	}
	if first.Ident != "context-1" || third.Ident != "context-3" {
		t.Fatalf("idents %q and %q; want one per multiple of the size", first.Ident, third.Ident)
	}
	for _, want := range []string{"1.6 MB", "Never ask the owner", "record -kind next", "no automatic compaction of a session of this model"} {
		if !strings.Contains(first.Because+first.Do, want) {
			t.Errorf("the note does not carry %q: %s %s", want, first.Because, first.Do)
		}
	}
	for _, n := range Notes(Observation{ContextBytes: 9_000_000, RenewAt: 0}) {
		if strings.HasPrefix(n.Ident, "context-") {
			t.Fatal("RenewAt 0 must say nothing")
		}
	}
}

func TestTheNoteNamesTheMeasuredSizeWhenThereIsOne(t *testing.T) {
	var got Note
	for _, n := range Notes(Observation{ContextBytes: 4_000_000, RenewAt: 3_000_000, CompactAt: 5_100_000, CompactSamples: 12}) {
		if strings.HasPrefix(n.Ident, "context-") {
			got = n
		}
	}
	if !strings.Contains(got.Because, "this model on its own at about 5.1 MB (median of 12)") {
		t.Fatalf("the measured size is not stated: %q", got.Because)
	}
}

// renewalNote is the note the grown context produces, or a fatal.
func renewalNote(t *testing.T, o Observation) Note {
	t.Helper()
	for _, n := range Notes(o) {
		if strings.HasPrefix(n.Ident, "context-") {
			return n
		}
	}
	t.Fatalf("no renewal note for %d bytes against %d", o.ContextBytes, o.RenewAt)
	return Note{}
}

// TestRenewalIsAChangeOfContextNotAStop pins what the note may say to a session
// that is mid-lane with the owner in the room, because one that read it as
// leave to stop wrote a handoff and ended, with assigned work outstanding.
//
// Two things carry that. Renewal is stated as changing which context does the
// work, never whether it continues — the earlier "not an instruction to stop"
// was a denial the reader could pass over, and the boundary the note names is
// elastic enough that any sub-task can be called a finished piece. And the
// renewal write is the `next` record, never the handoff: Store.Handoff turns an
// active assignment into handed_off, which is finished work and is not in
// flight, so a session that renews through it takes its own lane off the board.
func TestRenewalIsAChangeOfContextNotAStop(t *testing.T) {
	for _, o := range []Observation{
		{ContextBytes: 4_000_000, RenewAt: 1_500_000},
		{ContextBytes: 4_000_000, RenewAt: 1_500_000, Owner: &OwnerPresence{Since: time.Now()}},
		{ContextBytes: 4_000_000, RenewAt: 1_500_000, Owner: &OwnerPresence{Away: true, Since: time.Now()}},
	} {
		n := renewalNote(t, o)
		if !strings.Contains(n.Do, "never whether the work continues") {
			t.Errorf("the note does not say renewal leaves the work running, and a session "+
				"mid-lane reads what is left as leave to stop: %q", n.Do)
		}
		if !strings.Contains(n.Do, "not the handoff") {
			t.Errorf("the note points a renewing session at the handoff, which records the lane "+
				"finished and takes it out of flight: %q", n.Do)
		}
	}
}

// TestEndingTheShiftIsOfferedOnlyWhereTheOwnerIsRecordedAway: offered where the
// runner starts no next shift, "end the shift and let the runner start the next
// one" names a restart that does not come, and the lane stops until somebody
// notices. Attended is that case — shifts.sh logs "no shift starts until
// `mellions away`" and parks on it.
//
// Nil is not that case: `shift_allowed` admits "nothing recorded" beside "away",
// deliberately, so an installation that has never run `mellions away` keeps
// running shifts. The clause is withheld there anyway, because nil is unknown
// and the costs are asymmetric — a restart promised and not delivered stops the
// work, a true one withheld only routes the next phase through a dispatched
// successor. So what this pins is the away marker, not what the runner does.
func TestEndingTheShiftIsOfferedOnlyWhereTheOwnerIsRecordedAway(t *testing.T) {
	for _, tc := range []struct {
		where string
		owner *OwnerPresence
		offer bool
	}{
		{"the owner is at the keyboard", &OwnerPresence{Since: time.Now()}, false},
		{"nobody has ever recorded where the owner is", nil, false},
		{"the owner is recorded away", &OwnerPresence{Away: true, Since: time.Now()}, true},
	} {
		n := renewalNote(t, Observation{ContextBytes: 4_000_000, RenewAt: 1_500_000, Owner: tc.owner})
		got := strings.Contains(n.Do, "end the shift")
		if got == tc.offer {
			continue
		}
		if got {
			t.Errorf("%s, and the note still offers ending this one as renewal; it is offered "+
				"only against a recorded away marker: %q", tc.where, n.Do)
			continue
		}
		t.Errorf("%s, so shifts run back to back and ending this one does renew the work; the "+
			"note no longer says so: %q", tc.where, n.Do)
	}
}

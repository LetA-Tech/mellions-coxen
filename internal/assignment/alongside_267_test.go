// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
)

func claimOf(f *fakeTracker, repo, ref, id string) (claim.Claim, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.claims[key(repo, ref)] {
		if c.ID == id {
			return c, true
		}
	}
	return claim.Claim{}, false
}

// A lane opened -alongside a live lane's hold is a reader or a reconciler. Its
// claim is what tells the holding lane, and everyone else, what it reserves, so
// it has to say "nothing" at open and on every restate after it.
func TestAnAlongsideLaneNeverPublishesAnExclusiveClaimOnTheHeldIssue(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("svc", "#1494")] = []claim.Claim{{
		ID: "owner-lane", Host: "build-host", State: StateActive, At: time.Now(),
	}}
	if _, err := s.Open(OpenOptions{
		ID: "read-1497", Repo: "svc", Issue: "#1494", Source: src, Alongside: true,
		Objective: "independent read of the owner lane's draft", Because: "nobody has read it",
	}); err != nil {
		t.Fatalf("open -alongside: %v", err)
	}
	if c, ok := claimOf(f, "svc", "#1494", "read-1497"); !ok || !c.Alongside {
		t.Fatalf("at open the read lane published %+v (found %v): an exclusive claim on an issue another lane holds", c, ok)
	}
	a, err := s.Get("read-1497")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Alongside {
		t.Fatal("the record does not keep -alongside, so the next restate publishes an exclusive claim")
	}

	if err := s.Record("read-1497", "found", "read posted"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if c, ok := claimOf(f, "svc", "#1494", "read-1497"); !ok || !c.Alongside {
		t.Fatalf("after a restate the read lane holds %+v: the alongside form was lost after open", c)
	}
	if c, ok := claimOf(f, "svc", "#1494", "owner-lane"); !ok || !c.Holds(time.Now()) {
		t.Fatalf("the owner lane's claim changed: %+v", c)
	}
}

// What an alongside claim reserves is decided by every reader, not only by its
// text: once the owning lane is gone, a lane on this machine or another one is
// not refused because a reader is still there.
func TestAnAlongsideLaneDoesNotRefuseTheNextLane(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("svc", "#7")] = []claim.Claim{{
		ID: "owner-lane", Host: "build-host", State: StateActive, At: time.Now(),
	}}
	if _, err := s.Open(OpenOptions{
		ID: "read-7", Repo: "svc", Issue: "#7", Source: src, Alongside: true,
		Objective: "read", Because: "unread",
	}); err != nil {
		t.Fatalf("open -alongside: %v", err)
	}
	f.mu.Lock()
	kept := f.claims[key("svc", "#7")][:0:0]
	for _, c := range f.claims[key("svc", "#7")] {
		if c.ID != "owner-lane" {
			kept = append(kept, c)
		}
	}
	f.claims[key("svc", "#7")] = kept
	f.relabel(key("svc", "#7"))
	f.mu.Unlock()

	if _, err := s.Open(OpenOptions{
		ID: "next-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "carry the work on", Because: "the owner lane closed",
	}); err != nil {
		t.Fatalf("a lane was refused by a reader's alongside claim, which reserves nothing: %v", err)
	}
	if c, ok := claimOf(f, "svc", "#7", "next-7"); !ok || c.Alongside {
		t.Fatalf("a lane opened normally published %+v: it must hold the issue exclusively", c)
	}
}

// The positive control: a lane opened without -alongside publishes the
// holding claim it always did, and still refuses a second one.
func TestALaneOpenedNormallyStillHoldsItsIssue(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	if _, err := s.Open(OpenOptions{
		ID: "own-8", Repo: "svc", Issue: "#8", Source: src, Objective: "fix", Because: "open",
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	if c, ok := claimOf(f, "svc", "#8", "own-8"); !ok || c.Alongside {
		t.Fatalf("a normal lane published %+v", c)
	}
	if _, err := s.Open(OpenOptions{
		ID: "dup-8", Repo: "svc", Issue: "#8", Source: src, Objective: "fix", Because: "open",
	}); err == nil {
		t.Fatal("a second lane opened on an issue a normal lane holds")
	}
}

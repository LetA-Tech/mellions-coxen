// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"context"
	"strings"
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

// A lane opened -alongside a lane still working the issue is a reader or a
// reconciler. Its claim must defer the work and the merge to that lane at open
// and on every restate after it.
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
		t.Fatalf("at open the read lane published %+v (found %v): an exclusive claim on an issue another lane is working", c, ok)
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
	if c, ok := claimOf(f, "svc", "#1494", "owner-lane"); !ok || c.Alongside {
		t.Fatalf("the owner lane's claim changed: %+v", c)
	}
	if a, err = s.Get("read-1497"); err != nil {
		t.Fatal(err)
	}
	if brief := a.Text(time.Now()); !strings.Contains(brief, "opened alongside: while another lane is still working") {
		t.Errorf("the lane's own brief does not say its claim defers:\n%s", brief)
	}

	// Once nobody else is working it, the reader is the one on it: its next
	// restate holds the issue, so a third lane is not invited in.
	f.mu.Lock()
	f.claims[key("svc", "#1494")] = f.claims[key("svc", "#1494")][1:]
	f.mu.Unlock()
	if err := s.Record("read-1497", "found", "owner lane gone"); err != nil {
		t.Fatal(err)
	}
	if c, ok := claimOf(f, "svc", "#1494", "read-1497"); !ok || c.Alongside {
		t.Fatalf("with no other lane on the issue the alongside lane still holds nothing: %+v", c)
	}
}

// -alongside is the way through the refusal for work whose lane is finished —
// the refusal says so. That lane takes the work over and must hold it.
func TestAnAlongsideLaneTakingOverFinishedWorkHoldsIt(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("svc", "#7")] = []claim.Claim{{
		ID: "finished-lane", Host: "build-host", State: StateHandedOff, At: time.Now(),
	}}
	if _, err := s.Open(OpenOptions{
		ID: "take-7", Repo: "svc", Issue: "#7", Source: src, Alongside: true,
		Objective: "carry the work on", Because: "the earlier lane is finished",
	}); err != nil {
		t.Fatalf("open -alongside: %v", err)
	}
	if c, ok := claimOf(f, "svc", "#7", "take-7"); !ok || c.Alongside {
		t.Fatalf("a take-over of finished work published %+v: a third lane would read the issue as free", c)
	}
}

// What other lanes are refused by is unchanged: a lane opened alongside is
// still a lane on the issue, here and on the tracker.
func TestALaneOpenedAlongsideStillRefusesALaterOne(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("svc", "#9")] = []claim.Claim{{
		ID: "owner-lane", Host: "build-host", State: StateActive, At: time.Now(),
	}}
	if _, err := s.Open(OpenOptions{
		ID: "read-9", Repo: "svc", Issue: "#9", Source: src, Alongside: true,
		Objective: "read", Because: "unread",
	}); err != nil {
		t.Fatalf("open -alongside: %v", err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "next-9", Repo: "svc", Issue: "#9", Source: src, Objective: "fix", Because: "open",
	}); err == nil {
		t.Fatal("a third lane opened on an issue two live lanes are on")
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

// A reader that opens a fix of its own holds that pull request like any lane:
// alongside is about the item another lane held, not every item this lane touches.
func TestAnAlongsideLanesOwnPullRequestIsHeldExclusively(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("svc", "#272")] = []claim.Claim{{
		ID: "owner-lane", Host: "build-host", State: StateActive, At: time.Now(),
	}}
	if _, err := s.Open(OpenOptions{
		ID: "read-273", Repo: "svc", Issue: "#272", Source: src, Alongside: true,
		Objective: "read the owner lane's draft", Because: "unread",
	}); err != nil {
		t.Fatalf("open -alongside: %v", err)
	}
	if err := s.ClaimPullRequest(context.Background(), "read-273", "298"); err != nil {
		t.Fatalf("ClaimPullRequest: %v", err)
	}
	if err := s.Record("read-273", "found", "fix pushed"); err != nil {
		t.Fatal(err)
	}
	if c, ok := claimOf(f, "svc", "PR #298", "read-273"); !ok || c.Alongside {
		t.Fatalf("the reader's own pull request is claimed as %+v (found %v): a peer reads its fix as unheld", c, ok)
	}
	if c, ok := claimOf(f, "svc", "#272", "read-273"); !ok || !c.Alongside {
		t.Fatalf("the held issue lost the alongside form once the lane claimed a pull request: %+v", c)
	}
}

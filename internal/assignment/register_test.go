// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// Not every repository's work register is the tracker. Where the rows live in
// a document, a work unit is a real reference to real work and gh cannot
// address it — which used to leave -unpublished as the only way through, and
// -unpublished says no other session can see this lane.
func TestALaneOnAWorkUnitInTheRepositorysOwnRegisterOpens(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Registers = map[string]string{"svc": "docs/tracker.md"}

	a, err := s.Open(OpenOptions{
		ID: "svc-imp16", Repo: "svc", Issue: "IMP-016", Source: src,
		Objective: "implement the work unit", Because: "the register says it is ready",
	})
	if err != nil {
		t.Fatalf("Open on a work unit in the repository's own register: %v", err)
	}
	if a.Register != "docs/tracker.md" {
		t.Errorf("Register = %q, want the path the repository records its work at", a.Register)
	}
	// The hold is real but reaches this machine only, and every surface that
	// prints the lane has to say so rather than letting it read as a claim
	// other machines can see.
	if a.Claim.Published() {
		t.Error("a lane on a document row claims to be published to the tracker")
	}
	if !strings.Contains(a.Claim.Unpublished, "docs/tracker.md") {
		t.Errorf("the lane does not say where its work is recorded: %q", a.Claim.Unpublished)
	}
	// The reason the register is printed at all: two lanes re-derived one open
	// remediation row on one afternoon because nothing pointed at the rows.
	txt := a.Text(time.Now())
	if !strings.Contains(txt, "docs/tracker.md") || !strings.Contains(txt, "before reporting anything found here as new") {
		t.Errorf("the lane's record does not send the session to the register:\n%s", txt)
	}
}

// The local collision check is what -unpublished used to give up. It is string
// equality on the reference, so it holds for a work unit exactly as it does for
// an issue number.
func TestASecondLaneOnTheSameWorkUnitIsRefused(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Registers = map[string]string{"svc": "docs/tracker.md"}
	if _, err := s.Open(OpenOptions{
		ID: "svc-imp16", Repo: "svc", Issue: "IMP-016", Source: src,
		Objective: "implement the work unit", Because: "the register says it is ready",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Open(OpenOptions{
		ID: "svc-imp16-again", Repo: "svc", Issue: "IMP-016", Source: src,
		Objective: "implement the same work unit", Because: "the register says it is ready",
	})
	if err == nil {
		t.Fatal("a second lane opened on a work unit this store already holds")
	}
	if !strings.Contains(err.Error(), "svc-imp16") {
		t.Errorf("the refusal does not name the lane that holds it: %v", err)
	}
}

// The strictness stays where nothing declares a register: there a reference gh
// cannot address is a typo, and opening it quietly as a lane nothing else can
// see is the failure the claim exists to prevent.
func TestARepositoryWithNoRegisterStillRefusesAnUnaddressableReference(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Open(OpenOptions{
		ID: "svc-typo", Repo: "svc", Issue: "IMP-016", Source: src,
		Objective: "work on a reference nothing can address", Because: "the survey said so",
	})
	if err == nil {
		t.Fatal("a reference no tracker can address opened as an ordinary claim")
	}
	if _, err := s.Get("svc-typo"); err == nil {
		t.Error("the refused lane left a record behind")
	}
}

// A lane on a register row holds no claim on the tracker for its work unit, but
// the pull request it claims is on the tracker like any other, and closing the
// lane has to take that claim back off. The release is per reference: the row
// was never published and must not be released, the pull request was.
func TestARegisterLaneReleasesThePullRequestItClaimed(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Registers = map[string]string{"svc": "docs/tracker.md"}
	f := trackerOf(t, s)
	f.host = "leta-server"

	if _, err := s.Open(OpenOptions{
		ID: "svc-imp16", Repo: "svc", Issue: "IMP-016", Source: src,
		Objective: "implement the work unit", Because: "the register says it is ready",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimPullRequest(context.Background(), "svc-imp16", "178"); err != nil {
		t.Fatalf("ClaimPullRequest: %v", err)
	}
	if !f.label("svc", "PR #178") {
		t.Fatal("claiming the pull request did not label it")
	}
	a, err := s.Get("svc-imp16")
	if err != nil {
		t.Fatal(err)
	}
	if a.Claim.Host != "leta-server" {
		t.Errorf("the record does not name the host its pull request claim was published from: %q", a.Claim.Host)
	}

	if err := s.Handoff("svc-imp16", "done and pushed"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close("svc-imp16"); err != nil {
		t.Fatal(err)
	}
	if f.label("svc", "PR #178") {
		t.Fatalf("closing a register lane left its pull request labelled mellions:claimed: %+v",
			f.claims[key("svc", "PR #178")])
	}
	a, err = s.Get("svc-imp16")
	if err != nil {
		t.Fatal(err)
	}
	if a.Claim != nil && a.Claim.Stranded != "" {
		t.Errorf("the release reports a stranded claim: %q", a.Claim.Stranded)
	}
	if a.Claim == nil || a.Claim.Published() {
		t.Error("closing forgot that the work unit's hold never reached the tracker")
	}
	// The row never reached the tracker, and a real tracker asked to release
	// a register reference fails and strands the release.
	if slices.Contains(f.released, key("svc", "IMP-016")) {
		t.Errorf("the register row was sent to the tracker to release: %v", f.released)
	}
}

// A claim not restated goes stale and stops being obeyed, so a live register
// lane has to keep its pull request claim fresh like any other lane does.
func TestARegisterLaneKeepsItsPullRequestClaimFresh(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Registers = map[string]string{"svc": "docs/tracker.md"}
	f := trackerOf(t, s)
	start := time.Now()
	f.now = func() time.Time { return start }

	if _, err := s.Open(OpenOptions{
		ID: "svc-imp16", Repo: "svc", Issue: "IMP-016", Source: src,
		Objective: "implement the work unit", Because: "the register says it is ready",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimPullRequest(context.Background(), "svc-imp16", "178"); err != nil {
		t.Fatal(err)
	}
	later := start.Add(20 * time.Hour)
	f.now = func() time.Time { return later }
	if err := s.Record("svc-imp16", "found", "still working"); err != nil {
		t.Fatal(err)
	}
	held := f.claims[key("svc", "PR #178")]
	if len(held) != 1 || !held[0].At.Equal(later.UTC()) {
		t.Fatalf("working the lane did not restate its pull request claim: %+v", held)
	}
	if got := f.claims[key("svc", "IMP-016")]; len(got) != 0 {
		t.Fatalf("restating published the register row to the tracker: %+v", got)
	}
}

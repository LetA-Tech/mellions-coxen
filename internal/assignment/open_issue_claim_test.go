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

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
)

// Issue claims are unique across assignment ids; otherwise two differently
// named lanes can duplicate the same tracked work.
func TestASecondLaneOnAClaimedIssueIsRefused(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Open(OpenOptions{
		ID: "payments-43", Repo: "payments-api", Issue: "#43", Source: src,
		Objective: "the --version banner claims schema:1 it does not carry",
		Because:   "narrow, fully specified, fits the budget",
	}); err != nil {
		t.Fatal(err)
	}

	// The spelling a second session reaches for is its own, not the first's.
	_, err = s.Open(OpenOptions{
		ID: "payments43-version-banner", Repo: "Payments-API", Issue: "43", Source: src,
		Objective: "the --version banner ships an unparseable record",
		Because:   "the whole resolution fits the budget",
	})
	if err == nil {
		t.Fatal("a second lane opened on an issue another lane already claims; " +
			"the first session's work is repeated and neither learns of the other until the pull request")
	}
	for _, want := range []string{"payments-43", "mellions assign get", "-alongside"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q, so the session cannot act on it: %v", want, err)
		}
	}
	if _, err := s.Get("payments43-version-banner"); err == nil {
		t.Error("the refused lane left a record behind")
	}
}

// Handed off is finished and waiting on a person — the state a survey still
// shows as an open issue, and so the one most likely to be claimed again.
func TestAHandedOffLaneStillHoldsItsIssue(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "first", Repo: "svc", Issue: "#7", Source: src,
		Objective: "o", Because: "b",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Handoff("first", "pushed, waiting on a merge word"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Open(OpenOptions{
		ID: "second", Repo: "svc", Issue: "#7", Source: src,
		Objective: "o", Because: "b",
	}); err == nil {
		t.Fatal("a handed-off lane released its issue; the work waiting on the owner is repeated")
	}
}

// Refusing has to stay overridable, because reconciling two lanes on one issue
// is exactly the work that needs a third.
func TestAlongsideOpensAnyway(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "first", Repo: "svc", Issue: "#7", Source: src,
		Objective: "o", Because: "b",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "reconcile", Repo: "svc", Issue: "#7", Source: src, Alongside: true,
		Objective: "reconcile the two lanes", Because: "the owner asked for it",
	}); err != nil {
		t.Fatalf("-alongside was refused, so two lanes on one issue cannot be reconciled: %v", err)
	}
}

// A closed lane has released the work, and an unrelated issue was never in the
// way. Refusing either would make the guard a nuisance that gets overridden by
// habit, which is the same as not having it.
func TestClosedLanesAndOtherIssuesDoNotBlock(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "done", Repo: "svc", Issue: "#7", Source: src, Objective: "o", Because: "b",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Handoff("done", "finished"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close("done"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "again", Repo: "svc", Issue: "#7", Source: src, Objective: "o", Because: "b",
	}); err != nil {
		t.Fatalf("a closed lane still held its issue: %v", err)
	}

	if _, err := s.Open(OpenOptions{
		ID: "other-repo", Repo: "other", Issue: "#7", Source: src, Objective: "o", Because: "b",
	}); err != nil {
		t.Fatalf("issue #7 of a different repository was treated as the same work: %v", err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "no-issue-a", Repo: "svc", Source: src, Objective: "o", Because: "b",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "no-issue-b", Repo: "svc", Source: src, Objective: "o", Because: "b",
	}); err != nil {
		t.Fatalf("two lanes claiming no issue collided with each other: %v", err)
	}
	if a, err := s.Get("no-issue-a"); err != nil || a.Claim != nil {
		t.Fatalf("fixture: want a live lane holding no claim at all, got %+v %v", a, err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "after-no-issue", Repo: "svc", Issue: "#8", Source: src, Objective: "o", Because: "b",
	}); err != nil {
		t.Fatalf("a live lane holding no claim blocked an unrelated issue: %v", err)
	}
}

// A review lane holds the draft it reads through its pull request, not its
// issue. A second lane opened on that number has to meet the first on this
// disk, where its record is readable, rather than at the tracker.
func TestALaneHoldingAPullRequestRefusesASecondLaneOnIt(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "reader-200", Repo: "svc", Issue: "PR #200", Source: src,
		Objective: "read #200", Because: "nobody has",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimPullRequest(context.Background(), "reader-200", "#202"); err != nil {
		t.Fatal(err)
	}

	_, err = s.Open(OpenOptions{
		ID: "reader-202", Repo: "svc", Issue: "#202", Source: src,
		Objective: "read #202", Because: "nobody has",
	})
	if err == nil {
		t.Fatal("a second lane opened on a pull request a live lane on this machine holds")
	}
	for _, want := range []string{"reader-200", "mellions assign get reader-200", "PR #202", "-alongside"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q, so the session cannot act on it: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "another machine") {
		t.Errorf("refusal sends the session to another machine for a record on this one: %v", err)
	}
}

// A lane that moved from one pull request to another still holds a tracker
// claim on the first until it closes. The second open is unpublished, so this
// disk's scan is the only guard it meets, and it must see the earlier one too,
// under the pull request's rule rather than the issue's.
func TestAPullRequestALaneMovedOffStillRefusesASecondLane(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "author-9", Repo: "svc", Issue: "#9", Source: src,
		Objective: "fix #9", Because: "nobody has",
	}); err != nil {
		t.Fatal(err)
	}
	for _, pr := range []string{"#10", "#12"} {
		if err := s.ClaimPullRequest(context.Background(), "author-9", pr); err != nil {
			t.Fatal(err)
		}
	}
	a, err := s.Get("author-9")
	if err != nil {
		t.Fatal(err)
	}
	if a.PullRequest != "PR #12" || a.Claim == nil || !slices.Contains(a.Claim.Refs, "PR #10") {
		t.Fatalf("fixture: want current PR #12 and PR #10 kept in Claim.Refs, got %q %+v", a.PullRequest, a.Claim)
	}
	if held, err := trackerOf(t, s).Claims(context.Background(), "svc", "PR #10"); err != nil || len(held) != 1 {
		t.Fatalf("fixture: want the tracker still holding the claim on PR #10, got %v %v", held, err)
	}

	reader := func(id, issue string) error {
		_, err := s.Open(OpenOptions{
			ID: id, Repo: "svc", Issue: issue, Source: src, Unpublished: true,
			Objective: "read " + issue, Because: "nobody has",
		})
		return err
	}
	err = reader("reader-a", "#10")
	if err == nil {
		t.Fatal("a lane opened on a pull request a live lane on this machine still holds a claim on")
	}
	for _, want := range []string{"author-9", "mellions assign get author-9", "PR #10", "-alongside"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q, so the session cannot act on it: %v", want, err)
		}
	}
	if err := reader("reader-b", "#11"); err != nil {
		t.Errorf("a number the lane never claimed was refused: %v", err)
	}

	restated := func(state string, age time.Duration) {
		t.Helper()
		a, err := s.Get("author-9")
		if err != nil {
			t.Fatal(err)
		}
		a.State = state
		a.Claim.At = time.Now().Add(-age)
		if err := s.save(a); err != nil {
			t.Fatal(err)
		}
	}
	restated(StateActive, 25*time.Hour)
	if err := reader("reader-e", "#10"); err == nil || !strings.Contains(err.Error(), "author-9") {
		t.Errorf("a lane at work stopped holding its earlier pull request when its claim aged: %v", err)
	}
	restated(StateHandedOff, time.Hour)
	if err := reader("reader-c", "#10"); err == nil || !strings.Contains(err.Error(), "author-9") {
		t.Errorf("a handed-off lane whose claim stands did not hold its earlier pull request: %v", err)
	}
	restated(StateHandedOff, 25*time.Hour)
	if err := reader("reader-d", "#10"); err != nil {
		t.Errorf("a handed-off lane held its earlier pull request past the claim the sweep releases: %v", err)
	}
}

// A pull request found at handoff is published by the restate, not by
// ClaimPullRequest. A later claim on another pull request must not drop it from
// what the lane holds: the tracker still carries its claim.
func TestAPullRequestFoundAtHandoffStillRefusesASecondLaneAfterTheLaneMoves(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	a, err := s.Open(OpenOptions{
		ID: "author-9", Repo: "svc", Issue: "#9", Source: src,
		Objective: "fix #9", Because: "nobody has",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.prs[a.Branch] = []claim.PullRequest{{Number: 10, State: "OPEN"}}
	if err := s.Handoff("author-9", "waits on review"); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimPullRequest(context.Background(), "author-9", "#12"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("author-9")
	if err != nil {
		t.Fatal(err)
	}
	if got.PullRequest != "PR #12" {
		t.Fatalf("fixture: want current PR #12, got %q", got.PullRequest)
	}
	if held, err := f.Claims(context.Background(), "svc", "PR #10"); err != nil || len(held) != 1 {
		t.Fatalf("fixture: want the tracker still holding the claim on PR #10, got %v %v", held, err)
	}

	_, err = s.Open(OpenOptions{
		ID: "reader-a", Repo: "svc", Issue: "#10", Source: src, Unpublished: true,
		Objective: "read #10", Because: "nobody has",
	})
	if err == nil {
		t.Fatal("a lane opened on the pull request a live lane published at handoff and still holds")
	}
	for _, want := range []string{"author-9", "mellions assign get author-9", "PR #10"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
}

// A lane that handed off keeps its pull request only as long as the tracker
// honours the claim on it: once that claim is stale the sweep releases it, and
// the local scan refuses nothing the sweep would have let through.
func TestAStoppedLaneHoldsItsPullRequestOnlyWhileItsClaimStands(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "author-300", Repo: "svc", Issue: "#300", Source: src,
		Objective: "fix #300", Because: "nobody has",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimPullRequest(context.Background(), "author-300", "#301"); err != nil {
		t.Fatal(err)
	}
	reader := func(id string) error {
		_, err := s.Open(OpenOptions{
			ID: id, Repo: "svc", Issue: "#301", Source: src, Unpublished: true,
			Objective: "read #301", Because: "nobody has",
		})
		return err
	}
	restated := func(state string, age time.Duration) {
		t.Helper()
		a, err := s.Get("author-300")
		if err != nil {
			t.Fatal(err)
		}
		a.State = state
		if a.Claim == nil {
			a.Claim = &ClaimState{}
		}
		a.Claim.At = time.Now().Add(-age)
		if err := s.save(a); err != nil {
			t.Fatal(err)
		}
	}

	restated(StateHandedOff, time.Hour)
	if err := reader("reader-a"); err == nil || !strings.Contains(err.Error(), "author-300") {
		t.Errorf("a handed-off lane whose claim stands did not hold its pull request: %v", err)
	}
	restated(StateActive, 25*time.Hour)
	if err := reader("reader-b"); err == nil || !strings.Contains(err.Error(), "author-300") {
		t.Errorf("a lane at work stopped holding its pull request when its claim aged: %v", err)
	}
	restated(StateHandedOff, 25*time.Hour)
	if err := reader("reader-c"); err != nil {
		t.Errorf("a handed-off lane held its pull request past the claim the sweep releases: %v", err)
	}
}

// Issues and pull requests share one number space, so "PR #200" and "#200"
// are one tracker item whichever spelling each session used. The second open
// is unpublished, so this disk's scan is the only guard it meets.
func TestAPullRequestSpellingNamesTheSameItem(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "pr-spelled", Repo: "svc", Issue: "PR #200", Source: src,
		Objective: "o", Because: "b",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Open(OpenOptions{
		ID: "hash-spelled", Repo: "svc", Issue: "#200", Source: src, Unpublished: true,
		Objective: "o", Because: "b",
	})
	if err == nil {
		t.Fatal(`a lane on "#200" opened beside a live lane on "PR #200"`)
	}
	for _, want := range []string{"pr-spelled", "mellions assign get pr-spelled"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
	if _, err := s.Open(OpenOptions{
		ID: "other-number", Repo: "svc", Issue: "PR #201", Source: src, Unpublished: true,
		Objective: "o", Because: "b",
	}); err != nil {
		t.Fatalf("PR #201 was treated as the same item as PR #200: %v", err)
	}
}

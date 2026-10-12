// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"context"
	"strings"
	"testing"
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

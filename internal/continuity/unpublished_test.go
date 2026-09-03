// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package continuity

import (
	"context"
	"strings"
	"testing"
)

// The sentence this package used to publish from the absence of an upstream
// configuration. No reading may produce it again: it is a claim about every
// remote, derived from one local config entry that consults none of them.
const lostSentence = "nothing on this branch has been pushed"

// gitSaying fails every key it was not given, so omitting the @{upstream}
// lookups is exactly the world in which the old code took its else arm.

// TestNoUpstreamButPublishedIsNotToldNothingWasPushed.
//
// The ordinary shape of a lane here: pushed with `git push origin
// HEAD:refs/heads/<name>`, so origin holds it and no upstream is configured.
// The branch is published. Anything that reads the missing config as loss is
// wrong in the direction that makes a reader reconstruct work that exists.
func TestNoUpstreamButPublishedIsNotToldNothingWasPushed(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	git := gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD":                     a.Branch,
		"rev-parse HEAD":                                  "b23f2e4bd0",
		"status --porcelain":                              "",
		"rev-list --count aaaa1111..HEAD":                 "4",
		"rev-list --count aaaa1111..HEAD --not --remotes": "0",
	})

	o := Look(context.Background(), a, git, Tracker{})

	f, ok := fact(o, "unpublished")
	if !ok {
		t.Fatalf("no unpublished fact was read at all: %+v", o.Facts)
	}
	if !strings.Contains(f.Value, "none") {
		t.Errorf("a branch origin holds is not reported as published: %q", f.Value)
	}
	for _, g := range o.Facts {
		if strings.Contains(g.Value, lostSentence) {
			t.Errorf("%q still tells the reader the work was never published: %q", g.Name, g.Value)
		}
	}
	if !strings.Contains(f.Value, "stale") {
		t.Errorf("the reading does not name the boundary it was taken at: %q", f.Value)
	}
}

// TestGenuinelyUnpublishedCommitsAreCountedAgainstEveryRemoteTrackingRef.
//
// Work that really is only local must still be reported, and the count must be
// of the lane rather than of the history: the range is bounded below by the
// recorded base.
func TestGenuinelyUnpublishedCommitsAreCountedAgainstEveryRemoteTrackingRef(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	git := gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD":                     a.Branch,
		"rev-parse HEAD":                                  "b23f2e4bd0",
		"status --porcelain":                              "",
		"rev-list --count aaaa1111..HEAD":                 "3",
		"rev-list --count aaaa1111..HEAD --not --remotes": "3",
		// Present, and deliberately not what the count comes from: an
		// unbounded reading of this checkout would answer a different question.
		"rev-list --count HEAD --not --remotes": "914",
	})

	o := Look(context.Background(), a, git, Tracker{})

	f, ok := fact(o, "unpublished")
	if !ok {
		t.Fatalf("genuinely local work produced no unpublished fact: %+v", o.Facts)
	}
	if !strings.HasPrefix(f.Value, "3 commit(s)") {
		t.Errorf("want the 3 commits of the lane, got %q", f.Value)
	}
	if strings.Contains(f.Value, "914") {
		t.Errorf("the count is the repository's history, not the lane's: %q", f.Value)
	}
	if c, ok := fact(o, "commits since the base"); !ok || c.Value != "3" {
		t.Errorf("the lane's size is no longer reported beside it: %+v", o.Facts)
	}
}

// TestNoRemoteTrackingRefAnswersNothingRatherThanEverything.
//
// A checkout holding no remote-tracking ref subtracts nothing, so an unbounded
// count returns the whole history. With the recorded base gone too there is no
// honest reading available, and silence is the answer — an unknown is not a
// zero and is not a repository-sized number either.
func TestNoRemoteTrackingRefAnswersNothingRatherThanEverything(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	git := gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD":           a.Branch,
		"rev-parse HEAD":                        "b23f2e4bd0",
		"status --porcelain":                    "",
		"for-each-ref --count=1 refs/remotes":   "",
		"rev-list --count HEAD --not --remotes": "914",
	})

	o := Look(context.Background(), a, git, Tracker{})

	if f, ok := fact(o, "unpublished"); ok {
		t.Errorf("an unanswerable question was answered anyway: %q", f.Value)
	}
	for _, g := range o.Facts {
		if strings.Contains(g.Value, "914") {
			t.Errorf("%q reports the repository's whole history as lane state: %q", g.Name, g.Value)
		}
	}
}

// TestAMissingWorktreeReadsPublicationFromRemotesNotFromConfig.
//
// The consequential site. The worktree is gone and the reader is deciding
// whether the work is recoverable, so this is the reading that must not say
// the branch was never published when a remote holds it.
func TestAMissingWorktreeReadsPublicationFromRemotesNotFromConfig(t *testing.T) {
	a := work("/nowhere/me275/tree")
	a.Source = "/repos/analytics-service"
	git := gitSaying(map[string]string{
		"rev-parse --verify --quiet refs/heads/" + a.Branch:           "9ae055b8c1",
		"rev-list --count aaaa1111.." + a.Branch:                      "6",
		"rev-list --count aaaa1111.." + a.Branch + " --not --remotes": "0",
		"worktree list --porcelain":                                   "",
	})

	o := Look(context.Background(), a, git, Tracker{})

	f, ok := fact(o, "unpublished")
	if !ok {
		t.Fatalf("the missing-worktree reading says nothing about publication: %+v", o.Facts)
	}
	if !strings.Contains(f.Value, "none") {
		t.Errorf("a branch a remote holds is reported unpublished: %q", f.Value)
	}
	for _, g := range o.Facts {
		if strings.Contains(g.Value, lostSentence) {
			t.Errorf("%q reads as total loss where nothing was lost: %q", g.Name, g.Value)
		}
	}
}

// TestTheSentenceNeverClaimsMoreThanTheRangeItMeasured.
//
// The count is of `base..ref`, and base itself can hold commits no remote
// holds — `mellions assign open` falls back to a local HEAD as the base when
// the source checkout tracks no remote branch. A zero over that range said
// about ref would tell a session every commit is published when commits below
// the base are not, which loses work rather than duplicating it.
func TestTheSentenceNeverClaimsMoreThanTheRangeItMeasured(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	git := gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD":                     a.Branch,
		"rev-parse HEAD":                                  "b23f2e4bd0",
		"status --porcelain":                              "",
		"rev-list --count aaaa1111..HEAD":                 "2",
		"rev-list --count aaaa1111..HEAD --not --remotes": "0",
	})

	o := Look(context.Background(), a, git, Tracker{})

	f, ok := fact(o, "unpublished")
	if !ok {
		t.Fatalf("no unpublished fact: %+v", o.Facts)
	}
	if !strings.Contains(f.Value, "aaaa1111..HEAD") {
		t.Errorf("the sentence does not name the range it was taken over, so it claims "+
			"more than it measured: %q", f.Value)
	}
	if strings.Contains(f.Value, "every commit on HEAD") {
		t.Errorf("a universal claim over the whole ref from a count bounded by the base: %q", f.Value)
	}
}

// TestGitOutputThatIsNotANumberIsUnknownRatherThanZero.
//
// An unparseable count must not become "everything is published". The reading
// is absent, which is what a caller records nothing from.
func TestGitOutputThatIsNotANumberIsUnknownRatherThanZero(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	git := gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD":                     a.Branch,
		"rev-parse HEAD":                                  "b23f2e4bd0",
		"status --porcelain":                              "",
		"rev-list --count aaaa1111..HEAD":                 "1",
		"rev-list --count aaaa1111..HEAD --not --remotes": "warning: unable to access",
	})

	o := Look(context.Background(), a, git, Tracker{})

	if f, ok := fact(o, "unpublished"); ok {
		t.Errorf("garbage from git became a publication claim: %q", f.Value)
	}
}

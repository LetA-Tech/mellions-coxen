// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package continuity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// worktree makes a directory that looks like a git worktree to a stat.
func worktree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// gitSaying builds a fake git that answers a fixed map and fails on anything
// else, so a test states exactly what the world says and nothing more.
func gitSaying(answers map[string]string) Git {
	return func(_ string, args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		if v, ok := answers[key]; ok {
			return []byte(v + "\n"), nil
		}
		return nil, fmt.Errorf("git %s: no", key)
	}
}

func work(dir string) *assignment.Assignment {
	return &assignment.Assignment{
		ID: "fg-1", Repo: "analytics-service", Issue: "#340",
		Objective: "duplicate settlement rows",
		Branch:    "mellions/fg-1", Worktree: dir, Base: "aaaa1111",
		State: assignment.StateActive, OpenedAt: time.Now().Add(-26 * time.Hour),
	}
}

func fact(o Observed, name string) (Fact, bool) {
	for _, f := range o.Facts {
		if f.Name == name {
			return f, true
		}
	}
	return Fact{}, false
}

func slate(a *assignment.Assignment, o Observed) string {
	return (Standing{At: time.Now(), Work: []Work{{Recorded: a, Observed: o}}}).Text()
}

// TestBothBranchesSurviveWithTheirProvenance.
//
// The record says the work is on one branch and the worktree is on another.
// Nothing here decides what that means — a rebase, a cleanup, another session,
// or a record that is simply behind all produce it and call for different
// responses. What must survive is that both values reach the reader attributed,
// because a single merged answer cannot be reconciled against anything.
func TestBothBranchesSurviveWithTheirProvenance(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	o := Look(context.Background(), a, gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD": "dev",
		"rev-parse HEAD":              "beef0000cafe",
		"status --porcelain":          "",
	}), Tracker{})

	f, ok := fact(o, "branch")
	if !ok {
		t.Fatal("the branch the worktree is actually on was not read")
	}
	if f.Value != "dev" {
		t.Fatalf("observed branch = %q, want what git said", f.Value)
	}
	if !strings.Contains(f.From, dir) {
		t.Errorf("provenance = %q, want the worktree it was read from", f.From)
	}

	text := slate(a, o)
	if !strings.Contains(text, "mellions/fg-1") {
		t.Error("the recorded branch was dropped; there is nothing left to reconcile against")
	}
	if !strings.Contains(text, "branch: dev") {
		t.Error("the observed branch was dropped")
	}
}

// TestAMissingWorktreeIsAnObservedFact. §13's "missing worktree" case reaches
// the reader as something read from the filesystem, not as an error that stops
// the slate before the rest of it is collected.
func TestAMissingWorktreeIsAnObservedFact(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	o := Look(context.Background(), work(gone), gitSaying(nil), Tracker{})
	f, ok := fact(o, "worktree")
	if !ok {
		t.Fatal("nothing was reported about a worktree that does not exist")
	}
	if !strings.Contains(f.Value, "absent") {
		t.Fatalf("worktree = %q, want it reported absent", f.Value)
	}
}

// TestAnUnreachableTrackerIsUnknownRatherThanNo is the load-bearing mechanical
// distinction.
//
// "No pull request exists" and "I could not ask" are the same silence from
// inside this process and opposite instructions outside it. This one is not
// judgement: the tracker either answered or it did not, and only the collector
// knows which.
func TestAnUnreachableTrackerIsUnknownRatherThanNo(t *testing.T) {
	dir := worktree(t)
	git := gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD": "mellions/fg-1",
		"status --porcelain":          "",
	})

	silent := Look(context.Background(), work(dir), git, Tracker{
		PullRequest: func(context.Context, string, string) (string, bool) { return "", false },
		Issue:       func(context.Context, string, string) (string, bool) { return "", false },
	})
	if _, ok := fact(silent, "pull request"); ok {
		t.Fatal("a tracker that could not answer produced a pull-request fact")
	}
	if !some(silent.Unestablished, "could not say") {
		t.Fatalf("unestablished = %v, want the failure to ask carried explicitly", silent.Unestablished)
	}

	answered := Look(context.Background(), work(dir), git, Tracker{
		PullRequest: func(context.Context, string, string) (string, bool) {
			return "none for this branch", true
		},
	})
	f, ok := fact(answered, "pull request")
	if !ok || f.Value != "none for this branch" {
		t.Fatalf("pull request fact = %+v, want the tracker's actual answer", f)
	}
	if some(answered.Unestablished, "whether a pull request exists") {
		t.Fatalf("an answered question was carried as unestablished: %v", answered.Unestablished)
	}
}

// TestTheSlateReachesTheReaderWithoutAVerdict.
//
// This is the guard against what was here before: Go deciding whether
// continuing is safe. A merged pull request over an assignment nobody closed is
// the clearest case in the whole design, and even there the slate reports what
// it read and stops. Whether the work is done, whether the branch still matters,
// whether something was left half-finished behind the merge — that reasoning is
// the reader's, and a printed conclusion answers it before it is asked.
func TestTheSlateReachesTheReaderWithoutAVerdict(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	o := Look(context.Background(), a, gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD": "mellions/fg-1",
		"status --porcelain":          "",
	}), Tracker{
		PullRequest: func(context.Context, string, string) (string, bool) {
			return "#412 MERGED", true
		},
	})
	text := slate(a, o)

	if !strings.Contains(text, "#412 MERGED") {
		t.Fatal("the merged pull request never reached the reader")
	}
	for _, verdict := range []string{
		"safe to continue", "not safe", "do not continue",
		"it is done", "this is finished", "you should", "we recommend",
	} {
		if strings.Contains(strings.ToLower(text), verdict) {
			t.Errorf("the slate reached a conclusion (%q) that belongs to the reader", verdict)
		}
	}
}

// TestNoOpenWorkIsNotAFailure. A session that starts with nothing carried has
// not lost anything, and telling it otherwise teaches it to invent work.
func TestNoOpenWorkIsNotAFailure(t *testing.T) {
	text := (Standing{At: time.Now()}).Text()
	if !strings.Contains(text, "not after forgetting") {
		t.Errorf("an empty slate reads as amnesia:\n%s", text)
	}
}

// TestRecordedAndObservedAreNeverMerged. The whole slate rests on the reader
// being able to tell a claim from a reading.
func TestRecordedAndObservedAreNeverMerged(t *testing.T) {
	dir := worktree(t)
	a := work(dir)
	a.Findings = []assignment.Finding{
		{At: time.Now().Add(-5 * time.Hour), Kind: "found", Text: "PR #421 is open against dev"},
	}
	o := Look(context.Background(), a, gitSaying(map[string]string{
		"rev-parse --abbrev-ref HEAD": "mellions/fg-1",
		"status --porcelain":          "",
	}), Tracker{})

	text := slate(a, o)
	rec := strings.Index(text, "**Recorded**")
	obs := strings.Index(text, "**Observed**")
	claim := strings.Index(text, "PR #421 is open")
	if rec < 0 || obs < 0 {
		t.Fatalf("the two voices are not separated:\n%s", text)
	}
	if !(rec < claim && claim < obs) {
		t.Error("a claim about the world was not filed under what was recorded")
	}
	if !strings.Contains(text, "true when written and a claim now") {
		t.Error("nothing tells the reader which half is history")
	}
}

func some(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

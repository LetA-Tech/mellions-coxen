// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignmentsrc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

func repoAndStore(t *testing.T) (string, *assignment.Store) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	s, err := assignment.NewStore(filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatal(err)
	}
	return repo, s
}

// TestUnfinishedWorkIsReported is the point: work in flight is invisible from
// outside, and a survey that omits it suggests starting something new.
func TestUnfinishedWorkIsReported(t *testing.T) {
	repo, st := repoAndStore(t)
	if _, err := st.Open(assignment.OpenOptions{
		ID: "payments-42", Repo: "payments-api", Source: repo,
		Objective: "establish whether the acquire premise holds",
		Because:   "its citation no longer matches the tree",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Record("payments-42", "found", "analytics-service carries the same defect, unfiled"); err != nil {
		t.Fatal(err)
	}

	got, err := New(st).Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	var work, follow int
	for _, g := range got {
		switch g.Kind {
		case signal.KindAssignment:
			work++
			if !strings.Contains(g.Detail, "chosen because") {
				t.Errorf("assignment signal drops the selection reason: %q", g.Detail)
			}
			if g.Attrs["branch"] == "" || g.Attrs["worktree"] == "" {
				t.Errorf("assignment signal cannot be resumed from: %+v", g.Attrs)
			}
		case signal.KindFollowUp:
			follow++
		}
	}
	if work != 1 || follow != 1 {
		t.Fatalf("assignments=%d follow-ups=%d, want 1 and 1: %+v", work, follow, got)
	}
}

// TestFollowUpsAreSeparateFromWorkInFlight: candidate work and current work are
// different facts, and merging them hides both.
func TestFollowUpsAreSeparateFromWorkInFlight(t *testing.T) {
	repo, st := repoAndStore(t)
	if _, err := st.Open(assignment.OpenOptions{ID: "a1", Source: repo, Objective: "o", Because: "r"}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"hypothesis", "next", "note"} {
		if err := st.Record("a1", k, "not a discovery"); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := New(st).Collect(context.Background(), signal.Scope{})
	for _, g := range got {
		if g.Kind == signal.KindFollowUp {
			t.Errorf("a %s note was reported as a follow-up: %+v", g.Attrs["found_during"], g)
		}
	}
}

func TestElapsedBudgetIsSurfacedAsFactNotJudgement(t *testing.T) {
	repo, st := repoAndStore(t)
	if _, err := st.Open(assignment.OpenOptions{
		ID: "b1", Source: repo, Objective: "o", Because: "r", Budget: assignment.Budget{Wall: time.Nanosecond},
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	got, _ := New(st).Collect(context.Background(), signal.Scope{})
	if len(got) == 0 || !strings.Contains(got[0].Attrs["budget"], "elapsed") {
		t.Fatalf("an over-budget assignment does not say so: %+v", got)
	}
}

func TestClosedWorkIsNotReported(t *testing.T) {
	repo, st := repoAndStore(t)
	if _, err := st.Open(assignment.OpenOptions{ID: "c1", Source: repo, Objective: "o", Because: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Handoff("c1", "done"); err != nil {
		t.Fatal(err)
	}
	if err := st.Close("c1"); err != nil {
		t.Fatal(err)
	}
	got, _ := New(st).Collect(context.Background(), signal.Scope{})
	if len(got) != 0 {
		t.Errorf("closed work is still reported as in flight: %+v", got)
	}
}

func TestScopeFiltersByRepo(t *testing.T) {
	repo, st := repoAndStore(t)
	for _, id := range []string{"x1", "x2"} {
		r := "payments-api"
		if id == "x2" {
			r = "analytics-service"
		}
		if _, err := st.Open(assignment.OpenOptions{ID: id, Repo: r, Source: repo, Objective: "o", Because: "r"}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := New(st).Collect(context.Background(), signal.Scope{Repos: []string{"payments-api"}})
	for _, g := range got {
		if g.Repo != "payments-api" {
			t.Errorf("out-of-scope assignment reported: %s", g.Repo)
		}
	}
	if len(got) != 1 {
		t.Errorf("got %d signals, want 1", len(got))
	}
}

// TestAnOpenAssignmentWhoseBranchLeftTheRemoteIsAStalePremise: the record says
// work is in flight and the remote says the branch is gone. Reported beside the
// assignment, never acted on.
func TestAnOpenAssignmentWhoseBranchLeftTheRemoteIsAStalePremise(t *testing.T) {
	repo, store := repoAndStore(t)
	a, err := store.Open(assignment.OpenOptions{ID: "fl-1", Repo: "r", Source: repo,
		Objective: "o", Because: "b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name           string
		present, known bool
		commits        int
		commitsKnown   bool
		wantStale      bool
		wantAttr       string
	}{
		{"gone, and the lane committed", false, true, 2, true, true, "gone"},
		{"present", true, true, 2, true, false, "present"},
		{"unreachable", false, false, 2, true, false, "unknown"},
		// A review lane writes nothing, so its branch stands where it was cut.
		// Missing from the remote, that is the lane looking as it should.
		{"gone, and nothing was ever committed on it", false, true, 0, true, false,
			"gone (nothing was ever committed on it)"},
		// Not knowing is not evidence of no work: the signal stays.
		{"gone, and the checkout could not be asked", false, true, 0, false, true, "gone"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := New(store)
			src.Remote = func(_ context.Context, source, branch string) (bool, bool) {
				if source != repo || branch != a.Branch {
					t.Fatalf("asked about %s %s", source, branch)
				}
				return c.present, c.known
			}
			src.Commits = func(_ context.Context, source, base, branch string) (int, bool) {
				if source != repo || branch != a.Branch {
					t.Fatalf("asked about %s %s", source, branch)
				}
				// The commit, not the prose that explains how it was chosen:
				// BasePin reads "origin/dev fetched ...", which git cannot
				// resolve, and the count then silently reports nothing known.
				if base != a.Base {
					t.Fatalf("counted from %q, want the record's base %q", base, a.Base)
				}
				return c.commits, c.commitsKnown
			}
			got, err := src.Collect(context.Background(), signal.Scope{})
			if err != nil {
				t.Fatal(err)
			}
			var stale int
			for _, g := range got {
				if g.Kind == signal.KindStalePremise {
					stale++
				}
				if g.Kind == signal.KindAssignment && g.Attrs["remote_branch"] != c.wantAttr {
					t.Errorf("remote_branch = %q, want %q", g.Attrs["remote_branch"], c.wantAttr)
				}
			}
			if (stale > 0) != c.wantStale {
				t.Errorf("stale premises = %d, want stale=%v", stale, c.wantStale)
			}
		})
	}
}

// TestBranchCommitsCountsWhatALaneProduced drives the real probe against real
// git. The suppression it feeds is invisible to a test that injects the seam:
// a body returning (0, true) unconditionally would silence the stale-premise
// signal for every gone branch and leave the rest of this package green.
func TestBranchCommitsCountsWhatALaneProduced(t *testing.T) {
	repo, _ := repoAndStore(t)
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	base := git("rev-parse", "HEAD")
	git("branch", "reviewed", base)
	git("checkout", "-q", "-b", "wrote", base)
	git("commit", "-q", "--allow-empty", "-m", "work")
	git("checkout", "-q", "main")

	for _, c := range []struct {
		name              string
		dir, base, branch string
		wantN             int
		wantKnown         bool
	}{
		{"a lane that only reviewed", repo, base, "reviewed", 0, true},
		{"a lane that committed", repo, base, "wrote", 1, true},
		// Each of these must be unknown rather than zero: reported as zero they
		// would silence the signal exactly where it is most wanted.
		{"the branch is gone locally too", repo, base, "deleted-lane", 0, false},
		{"the base no longer resolves", repo, strings.Repeat("0", 40), "wrote", 0, false},
		{"the checkout has moved", filepath.Join(repo, "nowhere"), base, "wrote", 0, false},
		{"the record carries no base", repo, "", "wrote", 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			n, known := branchCommits(context.Background(), c.dir, c.base, c.branch)
			if n != c.wantN || known != c.wantKnown {
				t.Errorf("branchCommits = (%d, %v), want (%d, %v)", n, known, c.wantN, c.wantKnown)
			}
		})
	}
}

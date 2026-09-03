// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realRepo builds an actual git repository. Worktree isolation is the property
// under test, so faking git here would test the fake.
func realRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	return dir
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := newStoreT(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustOpen(t *testing.T, s *Store, o OpenOptions) *Assignment {
	t.Helper()
	a, err := s.Open(o)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return a
}

func TestOpenCreatesWorktreeAndRecord(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)

	a := mustOpen(t, s, OpenOptions{
		ID: "payments-42", Repo: "payments-api", Issue: "#42", Source: repo,
		Objective: "establish whether the acquire-metric premise still holds",
		Because:   "its own citation no longer matches the tree",
		NotChosen: "#402 — higher consequence, and the loop is unproven",
	})

	if fi, err := os.Stat(a.Worktree); err != nil || !fi.IsDir() {
		t.Fatalf("worktree missing at %s: %v", a.Worktree, err)
	}
	if _, err := os.Stat(filepath.Join(a.Worktree, "main.go")); err != nil {
		t.Errorf("worktree does not contain the repository: %v", err)
	}
	if a.Base == "" {
		t.Error("base commit not recorded; a falsification would have nothing to revert to")
	}
	if a.State != StateActive {
		t.Errorf("state = %q, want %q", a.State, StateActive)
	}

	got, err := s.Get("payments-42")
	if err != nil {
		t.Fatal(err)
	}
	if got.Because == "" || got.NotChosen == "" {
		t.Error("the selection counterfactual was not persisted; selection quality becomes unmeasurable")
	}
}

// TestOperationalStateNeverEntersTheRepository is the invariant the architecture
// names for this component. The engineer's working memory must not appear in
// `git status` during real work, or it ends up in a commit.
func TestOperationalStateNeverEntersTheRepository(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "x1", Source: repo, Objective: "o", Because: "the most valuable work available"})

	for _, kind := range []string{"hypothesis", "found", "next", "note"} {
		if err := s.Record("x1", kind, "something learned"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Handoff("x1", "where it stands"); err != nil {
		t.Fatal(err)
	}

	// Nothing the store wrote may be inside the worktree.
	entries, err := os.ReadDir(a.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		switch e.Name() {
		case ".git", "main.go":
		default:
			t.Errorf("worktree gained %q — operational state leaked into the repository", e.Name())
		}
	}

	// And git itself must see a clean tree.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = a.Worktree
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("worktree is dirty after recording assignment state:\n%s", out)
	}
	if !strings.HasPrefix(s.file("x1"), s.Root) {
		t.Errorf("record is not under the assignments root: %s", s.file("x1"))
	}
}

// TestCloseRefusedWithoutHandoff enforces the stopping rule: removing a worktree
// while the reasoning lives only in a dead session's context is how expensive
// uncommitted state becomes a directory of files and no explanation.
func TestCloseRefusedWithoutHandoff(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	mustOpen(t, s, OpenOptions{ID: "x2", Source: repo, Objective: "o", Because: "the most valuable work available"})

	err := s.Close("x2")
	if err == nil {
		t.Fatal("closed an assignment with no handoff")
	}
	if !strings.Contains(err.Error(), "handoff") {
		t.Errorf("error does not say what is missing: %v", err)
	}

	// Abandoning is the deliberate escape, and it says what it costs.
	if err := s.Abandon("x2", "a half-written reproduction, superseded", nil); err != nil {
		t.Fatalf("abandon failed: %v", err)
	}
	a, err := s.Get("x2")
	if err != nil {
		t.Fatal(err)
	}
	if a.State != StateAbandoned || a.ClosedAt.IsZero() {
		t.Errorf("abandon did not record: %+v", a)
	}
	if a.Discarded == nil || a.Discarded.What == "" {
		t.Error("abandoning recorded nothing about what was thrown away")
	}
}

func TestCloseRemovesWorktreeButKeepsTheBranch(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "x3", Source: repo, Objective: "o", Because: "the most valuable work available"})

	if err := s.Handoff("x3", "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close("x3"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("worktree survived close: %v", err)
	}
	// The commits are the durable part; a closed assignment must not destroy them.
	cmd := exec.Command("git", "branch", "--list", a.Branch)
	cmd.Dir = repo
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), a.Branch) {
		t.Errorf("close destroyed the branch %s: %s", a.Branch, out)
	}
}

// TestResumeReconstructsAcrossProcessDeath: the whole point of the record.
func TestResumeReconstructsAcrossProcessDeath(t *testing.T) {
	repo := realRepo(t)
	root := filepath.Join(t.TempDir(), "assignments")

	first, err := newStoreT(root)
	if err != nil {
		t.Fatal(err)
	}
	mustOpen(t, first, OpenOptions{
		ID: "x4", Repo: "r", Issue: "#9", Source: repo, Because: "r",
		Objective: "root-cause the acquire rate",
	})
	if err := first.Record("x4", "hypothesis", "pool churn, not contention"); err != nil {
		t.Fatal(err)
	}

	// A completely separate store, as a later process would see it.
	second, err := newStoreT(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := second.Get("x4")
	if err != nil {
		t.Fatal(err)
	}
	if a.Objective == "" || len(a.Findings) != 1 {
		t.Fatalf("state did not survive: %+v", a)
	}
	txt := a.Text(time.Now())
	for _, want := range []string{"root-cause the acquire rate", "pool churn", a.Branch, "#9"} {
		if !strings.Contains(txt, want) {
			t.Errorf("resume context is missing %q:\n%s", want, txt)
		}
	}
}

func TestParallelAssignmentsAreIsolated(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "p1", Source: repo, Objective: "one", Because: "r"})
	b := mustOpen(t, s, OpenOptions{ID: "p2", Source: repo, Objective: "two", Because: "r"})

	if a.Worktree == b.Worktree || a.Branch == b.Branch {
		t.Fatalf("assignments share state: %s/%s vs %s/%s", a.Worktree, a.Branch, b.Worktree, b.Branch)
	}
	if err := os.WriteFile(filepath.Join(a.Worktree, "scratch.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.Worktree, "scratch.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Error("a change in one assignment's worktree appeared in another")
	}
	if err := s.Record("p1", "note", "only mine"); err != nil {
		t.Fatal(err)
	}
	other, err := s.Get("p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Findings) != 0 {
		t.Errorf("a note leaked between assignments: %+v", other.Findings)
	}
}

func TestBudgetElapsedDemandsAWrittenStatus(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{
		ID: "b1", Source: repo, Objective: "o", Because: "r",
		Budget: Budget{Wall: time.Hour},
	})

	later := a.OpenedAt.Add(90 * time.Minute)
	if !a.Overdue(later) {
		t.Fatal("an assignment past its budget does not report overdue")
	}
	txt := a.Text(later)
	if !strings.Contains(txt, "BUDGET ELAPSED") || !strings.Contains(txt, "Do not continue silently") {
		t.Errorf("overdue assignment does not demand a status:\n%s", txt)
	}

	// A handed-off assignment is finished, not overdue.
	if err := s.Handoff("b1", "stands here"); err != nil {
		t.Fatal(err)
	}
	done, _ := s.Get("b1")
	if done.Overdue(later) {
		t.Error("a handed-off assignment still reports overdue")
	}
}

func TestOpenRefusesIncompleteWork(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	for name, o := range map[string]OpenOptions{
		"no id":        {Source: repo, Objective: "o", Because: "b"},
		"no objective": {ID: "z", Source: repo, Because: "b"},
		"no source":    {ID: "z", Objective: "o", Because: "b"},
		"no reason":    {ID: "z", Source: repo, Objective: "o"},
		"path in id":   {ID: "a/b", Source: repo, Objective: "o", Because: "b"},
	} {
		if _, err := s.Open(o); err == nil {
			t.Errorf("%s: Open succeeded", name)
		}
	}
}

func TestDuplicateIDIsRefused(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	mustOpen(t, s, OpenOptions{ID: "dup", Source: repo, Objective: "o", Because: "the most valuable work available"})
	if _, err := s.Open(OpenOptions{ID: "dup", Source: repo, Objective: "o", Because: "the most valuable work available"}); err == nil {
		t.Fatal("a second assignment reused an id, which would have overwritten the first")
	}
}

// TestFailedWorktreeLeavesNoOrphanRecord: a record pointing at a directory that
// does not exist is worse than no record, because a later session trusts it.
func TestFailedWorktreeLeavesNoOrphanRecord(t *testing.T) {
	s := newStore(t)
	s.Git = func(string, ...string) ([]byte, error) { return nil, errors.New("worktree add failed") }

	if _, err := s.Open(OpenOptions{ID: "orphan", Source: "/nonexistent", Objective: "o", Because: "r", BaseRef: "HEAD"}); err == nil {
		t.Fatal("Open succeeded despite a failing git")
	}
	if _, err := s.Get("orphan"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a record survived a failed worktree: %v", err)
	}
	if _, err := os.Stat(s.dir("orphan")); !errors.Is(err, os.ErrNotExist) {
		t.Error("assignment directory survived a failed open")
	}
}

func TestListOrdersNewestFirstAndHidesClosed(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	base := time.Now().Add(-3 * time.Hour)
	s.now = func() time.Time { return base }
	mustOpen(t, s, OpenOptions{ID: "old", Source: repo, Objective: "o", Because: "the most valuable work available"})
	s.now = func() time.Time { return base.Add(time.Hour) }
	mustOpen(t, s, OpenOptions{ID: "new", Source: repo, Objective: "o", Because: "the most valuable work available"})

	open, err := s.List(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 2 || open[0].ID != "new" {
		t.Fatalf("List order = %+v, want newest first", ids(open))
	}

	if err := s.Handoff("old", "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close("old"); err != nil {
		t.Fatal(err)
	}
	if open, _ = s.List(false); len(open) != 1 || open[0].ID != "new" {
		t.Errorf("closed assignment still listed: %v", ids(open))
	}
	if all, _ := s.List(true); len(all) != 2 {
		t.Errorf("closed assignment unreachable with includeClosed: %v", ids(all))
	}
}

func TestRecordRejectsUnknownKindAndEmptyText(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	mustOpen(t, s, OpenOptions{ID: "r1", Source: repo, Objective: "o", Because: "the most valuable work available"})
	if err := s.Record("r1", "conclusion", "x"); err == nil {
		t.Error("an unknown finding kind was accepted")
	}
	if err := s.Record("r1", "note", "   "); err == nil {
		t.Error("an empty finding was accepted")
	}
}

func ids(in []*Assignment) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		out = append(out, a.ID)
	}
	return out
}

// opened is one active assignment in a real repository, which is what every
// interruption test starts from.
func opened(t *testing.T) (*Store, *Assignment) {
	t.Helper()
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{
		ID: "payments-42", Repo: "payments-api", Issue: "#42", Source: realRepo(t),
		Objective: "duplicate recovery path double-posts on retry",
		Because:   "the only open defect touching posted balances",
	})
	return s, a
}

// TestSuspendingRequiresBothReasons.
//
// What separates setting work down from abandoning it is entirely what gets
// written at the moment of setting it down. Neither field can be reconstructed
// afterwards, so neither is optional.
func TestSuspendingRequiresBothReasons(t *testing.T) {
	s, a := opened(t)
	if err := s.Suspend(a.ID, "", "the guard is written, the test is not"); err == nil {
		t.Error("suspended with nothing that took priority")
	}
	if err := s.Suspend(a.ID, "a production double-post", ""); err == nil {
		t.Error("suspended without saying where the work stood")
	}
	if got, _ := s.Get(a.ID); got.State != StateActive {
		t.Errorf("a refused suspension changed the state to %s", got.State)
	}
}

// TestSuspendedWorkKeepsItsWorktreeAndResumes. Nothing is thrown away; the
// engineer stops working on it, which is a different thing.
func TestSuspendedWorkKeepsItsWorktreeAndResumes(t *testing.T) {
	s, a := opened(t)
	worktree, branch := a.Worktree, a.Branch

	if err := s.Suspend(a.ID, "a production double-post", "guard written, test not"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateSuspended {
		t.Fatalf("state = %s, want suspended", got.State)
	}
	if got.Worktree != worktree || got.Branch != branch {
		t.Error("suspending discarded the worktree or the branch")
	}
	if len(got.Suspensions) != 1 || !got.Suspensions[0].Open() {
		t.Fatalf("the interruption was not recorded: %+v", got.Suspensions)
	}
	if !strings.Contains(got.Text(time.Now()), "guard written, test not") {
		t.Error("a resuming session cannot see where the work stood")
	}

	back, err := s.Resume(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.State != StateActive {
		t.Errorf("resumed state = %s, want active", back.State)
	}
	if back.Suspensions[0].Open() {
		t.Error("the interruption is still open after resuming")
	}
}

// TestOnlyWorkInProgressCanBeSetDown. There is nothing to interrupt in an
// assignment that already stopped.
func TestOnlyWorkInProgressCanBeSetDown(t *testing.T) {
	s, a := opened(t)
	if err := s.Suspend(a.ID, "x", "y"); err != nil {
		t.Fatal(err)
	}
	if err := s.Suspend(a.ID, "x", "y"); err == nil {
		t.Error("suspended twice without resuming")
	}
	if _, err := s.Resume(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resume(a.ID); err == nil {
		t.Error("resumed something that was not suspended")
	}
	if err := s.Handoff(a.ID, "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.Suspend(a.ID, "x", "y"); err == nil {
		t.Error("suspended work that was already handed off")
	}
}

// TestSuspendedWorkIsNotOverrunning.
//
// Counting the time it sits would make every interruption look like an
// abandonment, and teach the engineer to finish low-value work rather than set
// it aside — the exact behaviour this lifecycle exists to permit.
func TestSuspendedWorkIsNotOverrunning(t *testing.T) {
	s, a := opened(t)
	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Budget.Wall = time.Minute
	long := got.OpenedAt.Add(time.Hour)
	if !got.Overdue(long) {
		t.Fatal("an active assignment past its budget is not overdue")
	}
	if err := s.Suspend(a.ID, "something more important", "where it stands"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(a.ID)
	got.Budget.Wall = time.Minute
	if got.Overdue(long) {
		t.Error("suspended work is reported as overrunning its budget")
	}
}

// TestRepeatedInterruptionsAreVisible. An assignment that keeps being the second
// most important thing is telling you something a boolean cannot.
func TestRepeatedInterruptionsAreVisible(t *testing.T) {
	s, a := opened(t)
	for i := 0; i < 3; i++ {
		if err := s.Suspend(a.ID, "something else", "partway"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Resume(a.ID); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.Get(a.ID)
	if len(got.Suspensions) != 3 {
		t.Fatalf("%d interruptions recorded, want 3", len(got.Suspensions))
	}
	if !strings.Contains(got.Text(time.Now()), "Interrupted 3 times") {
		t.Error("repeated interruption is not surfaced to whoever reads the assignment")
	}
}

// TestAbandonedWorkDoesNotStayMergeable.
//
// Closing enforces an independent review where the change trips a trigger, and
// abandoning did not — so a commit that could not be closed could be abandoned,
// and the branch it sat on merged like any other. Abandon meant "stop working on
// this" while its name and its record said "throw this away".
//
// The branch goes now. What it was is recorded first, because git keeps the tip
// reachable and a reader with the hash can recover it — a narrower promise than
// "nothing is lost" and the true one.
func TestAbandonedWorkDoesNotStayMergeable(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{
		ID: "abn1", Repo: "ledger", Source: repo,
		Objective: "a schema change that would require a second view",
		Because:   "it is the reproduction for the abandon bypass",
	})

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(a.Worktree, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = a.Worktree
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write("003_drop.sql", "DROP TABLE ledger_entries;\n")
	git("add", ".")
	git("commit", "-q", "-m", "schema change requiring review")

	branch := a.Branch
	if err := s.Abandon("abn1", "the migration is wrong and is being rewritten", nil); err != nil {
		t.Fatalf("Abandon: %v", err)
	}

	got, err := s.Get("abn1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Discarded == nil {
		t.Fatal("abandoning recorded nothing")
	}
	if got.Discarded.Commits != 1 {
		t.Errorf("commits = %d, want the one that was on the branch", got.Discarded.Commits)
	}
	if got.Discarded.Tip == "" {
		t.Error("the tip was not recorded, so the discard is not recoverable")
	}
	if got.Discarded.Branch != branch {
		t.Errorf("branch = %q, want %q", got.Discarded.Branch, branch)
	}

	// The branch is what made this a bypass: it merged like any other.
	out, err := exec.Command("git", "-C", repo, "branch", "--list", branch).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("abandoned work is still on branch %s and still merges: %q", branch, out)
	}

	// And the tip is still reachable, so the record is worth something.
	if _, err := exec.Command("git", "-C", repo, "cat-file", "-e", got.Discarded.Tip).Output(); err != nil {
		t.Errorf("the recorded tip %s is not reachable: %v", got.Discarded.Tip, err)
	}
}

// TestAnAssignmentWhoseWorktreeIsGoneStillCloses: a tree pruned or wiped by
// hand holds nothing to destroy, and a lane that can never be closed is worse
// than one closed over an absent tree.
func TestAnAssignmentWhoseWorktreeIsGoneStillCloses(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "gone1", Repo: "r", Source: repo,
		Objective: "a lane whose tree a cleanup removed", Because: "the owner asked"})
	if err := s.Handoff(a.ID, "finished; the tree was removed by a cleanup"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(a.Worktree); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(a.ID); err != nil {
		t.Fatalf("close over an absent worktree: %v", err)
	}
	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateClosed {
		t.Fatalf("state = %s, want closed", got.State)
	}
}

// TestAbandonedWorkIsNotInFlight: abandoned is as finished as closed, and a
// list of open work that carries it tells every session start a lie.
func TestAbandonedWorkIsNotInFlight(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "ab1", Repo: "r", Source: repo, Objective: "o", Because: "b"})
	if err := s.Abandon(a.ID, "superseded", nil); err != nil {
		t.Fatal(err)
	}
	open, err := s.List(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range open {
		if x.ID == a.ID {
			t.Fatalf("an abandoned assignment is listed as open")
		}
	}
	all, err := s.List(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("all = %d, want the abandoned one kept in the full list", len(all))
	}
}

// TestAWorktreeGitCannotReadCanStillBeAbandoned: a copied or wiped gitdir
// leaves a directory git refuses to read, and a lane that can be neither
// closed nor abandoned is stuck forever. Abandoning records what it could not
// establish rather than refusing.
func TestAWorktreeGitCannotReadCanStillBeAbandoned(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "corrupt1", Repo: "r", Source: repo, Objective: "o", Because: "b"})
	// Break the worktree the way a copy from another machine does: the .git
	// file points at a gitdir that does not exist.
	if err := os.WriteFile(filepath.Join(a.Worktree, ".git"), []byte("gitdir: /nonexistent/worktrees/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unsaved(a); err == nil {
		t.Fatal("expected the unreadable worktree to be reported as an error by Unsaved")
	}
	if err := s.Abandon(a.ID, "an audit lane from before the rebuild", nil); err != nil {
		t.Fatalf("abandon over an unreadable worktree: %v", err)
	}
	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateAbandoned || got.Discarded == nil || !strings.Contains(got.Discarded.Held, "unreadable") {
		t.Fatalf("got %+v", got)
	}
	if _, err := os.Stat(a.Worktree); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the unreadable worktree survived abandon")
	}
}

// TestAbandonSettlesEvenWhenTheBranchIsAlreadyGone: a failed branch delete
// used to cancel the state change, leaving an abandoned lane listed as open.
func TestAbandonSettlesEvenWhenTheBranchIsAlreadyGone(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "nobranch", Repo: "r", Source: repo, Objective: "o", Because: "b"})
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("worktree", "remove", "--force", a.Worktree)
	run("branch", "-D", a.Branch)
	if err := s.Abandon(a.ID, "the branch went already", nil); err != nil {
		t.Fatalf("abandon: %v", err)
	}
	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateAbandoned {
		t.Fatalf("state = %s, want abandoned", got.State)
	}
}

// TestReopenTakesHandedOffWorkBackUpAndRecutsAMissingWorktree.
func TestReopenTakesHandedOffWorkBackUpAndRecutsAMissingWorktree(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "again", Repo: "r", Source: repo, Objective: "o", Because: "b"})
	if err := s.Handoff(a.ID, "review pending"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "worktree", "remove", "--force", a.Worktree)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	re, err := s.Reopen(a.ID)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if re.State != StateActive {
		t.Fatalf("state = %s, want active", re.State)
	}
	if _, err := os.Stat(filepath.Join(a.Worktree, ".git")); err != nil {
		t.Fatalf("worktree was not re-cut: %v", err)
	}
	out, _ := exec.Command("git", "-C", a.Worktree, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if strings.TrimSpace(string(out)) != a.Branch {
		t.Fatalf("re-cut worktree is on %q, want %q", strings.TrimSpace(string(out)), a.Branch)
	}
	if _, err := s.Reopen(a.ID); err == nil {
		t.Fatal("reopening an active lane should be refused")
	}
}

// TestAnExistingWorktreeCanBeAdopted: a repository's own process may dictate
// where its work happens; the lane records that tree and never removes it.
func TestAnExistingWorktreeCanBeAdopted(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	tree := filepath.Join(t.TempDir(), "imp-1")
	cmd := exec.Command("git", "worktree", "add", "-b", "imp-1", tree)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	a := mustOpen(t, s, OpenOptions{ID: "imp1", Repo: "r", Source: repo, Objective: "o", Because: "b", Worktree: tree})
	if !a.Adopted || a.Branch != "imp-1" || !samePath(a.Worktree, tree) {
		t.Fatalf("adopted = %v branch = %q worktree = %q", a.Adopted, a.Branch, a.Worktree)
	}
	if _, err := s.Open(OpenOptions{ID: "imp2", Repo: "r", Source: repo, Objective: "o", Because: "b", Worktree: realRepo(t)}); err == nil {
		t.Fatal("a checkout of another repository is not a working tree of this one")
	}
	if err := s.Handoff(a.ID, "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(a.ID); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tree, ".git")); err != nil {
		t.Fatalf("close removed an adopted worktree: %v", err)
	}
	if out, _ := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", "refs/heads/imp-1").Output(); len(out) == 0 {
		t.Fatal("close deleted an adopted branch")
	}
}

// The remediation and territory methods are loaded when the text of their own
// moment names them and not when a catalog listed them at session start; a
// lane's record is that moment, and only while the lane is active. It names
// them against the situation each is for and does not command a load the lane's
// size cannot justify — an instruction overridden once stops being read.
func TestAnActiveLaneNamesItsMethodAtTheMoment(t *testing.T) {
	s, a := opened(t)
	txt := a.Text(time.Now())
	for _, want := range []string{"mellions:mellions-issue-remediation", "mellions:mellions-territory", "mellions skills"} {
		if !strings.Contains(txt, want) {
			t.Errorf("an active lane's record does not name %s:\n%s", want, txt)
		}
	}
	if strings.Contains(txt, "before the first edit") {
		t.Errorf("an ordinary lane still commands a load before the first edit:\n%s", txt)
	}
	// An adopted tree is a fact about this lane rather than an estimate of its
	// size, so there the instruction stays an instruction.
	adopted := *a
	adopted.Adopted = true
	if got := adopted.Text(time.Now()); !strings.Contains(got, `Skill(skill: "mellions:mellions-territory")`) ||
		!strings.Contains(got, "not cut for you") {
		t.Errorf("an adopted tree does not tell the lane to read territory first:\n%s", got)
	}
	if err := s.Handoff(a.ID, "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(a.ID); err != nil {
		t.Fatal(err)
	}
	closed, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(closed.Text(time.Now()), "before the first edit") {
		t.Errorf("a closed lane still tells a reader to load the remediation method:\n%s", closed.Text(time.Now()))
	}
}

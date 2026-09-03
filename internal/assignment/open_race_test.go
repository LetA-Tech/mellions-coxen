// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Two agents told to take the same work: one gets the lane, and the id stays
// usable.
//
// Unguarded, both pass the existence check, both cut git state, and the loser's
// cleanup removes the winner's directory — so neither ends up with a lane, and
// the branch one of them created survives with nothing pointing at it. The id is
// then permanently unopenable: every later attempt fails on a branch that no
// command in this product mentions. That is the shape this asserts against,
// because two agents on one issue is ordinary rather than exotic.
func TestOnlyOneCallerOpensAnAssignmentAndTheIDSurvives(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	const racers = 8
	var won int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := s.Open(OpenOptions{
				ID: "dup", Repo: "svc", Source: src,
				Objective: fmt.Sprintf("racer %d", i),
				Because:   "two agents were told to take the same work",
			})
			if err == nil {
				atomic.AddInt64(&won, 1)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if won != 1 {
		t.Fatalf("%d of %d callers opened the same id; exactly one lane exists, so exactly one "+
			"caller may be told it holds one", won, racers)
	}
	if _, err := s.Get("dup"); err != nil {
		t.Fatalf("the winner's assignment cannot be read back: %v", err)
	}
}

// A failed open leaves nothing behind, so the id can be used again.
//
// The branch is the piece that matters. git creates the ref before it registers
// the working tree, so a failure between the two leaves a branch with no lane —
// invisible to every command here, and enough to make the id unopenable for
// good.
func TestAFailedOpenLeavesNoBranchBehind(t *testing.T) {
	src := gitFixture(t)
	root := t.TempDir()
	s, err := newStoreT(root)
	if err != nil {
		t.Fatal(err)
	}

	// Reach the state that actually strands an id: the ref written, the working
	// tree not registered. Occupying the destination with a non-empty directory
	// makes worktree-add fail at exactly that point.
	occupied := filepath.Join(root, "x", "tree")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "in-the-way"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", src, "worktree", "add", "-b", "mellions/x",
		occupied, "HEAD").CombinedOutput(); err == nil {
		t.Skipf("this git registers a worktree over an occupied directory: %s", out)
	}
	out, err := exec.Command("git", "-C", src, "branch", "--list", "mellions/x").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Skip("this git does not write the ref before registering the tree, so the leak is unreachable here")
	}
	// The precondition holds: git left a branch behind. Clean it, then prove
	// Open does the same when the failure is its own.
	exec.Command("git", "-C", src, "branch", "-D", "mellions/x").Run()

	if _, err := s.Open(OpenOptions{
		ID: "x", Repo: "svc", Source: src,
		Objective: "will not open", Because: "the destination is occupied",
	}); err == nil {
		t.Fatal("opening into an occupied destination succeeded")
	}
	out, err = exec.Command("git", "-C", src, "branch", "--list", "mellions/x").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("a failed open left the branch behind: %q — the id can never be opened again", out)
	}

	// And the proof that matters to somebody using it: the id still works once
	// whatever was in the way is gone.
	os.RemoveAll(filepath.Join(root, "x"))
	if _, err := s.Open(OpenOptions{
		ID: "x", Repo: "svc", Source: src,
		Objective: "the second attempt", Because: "the first failed and left nothing",
	}); err != nil {
		t.Fatalf("the id is unusable after a failed open: %v", err)
	}
}

func gitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "a@b.c")
	run("config", "user.name", "A")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "base")
	return dir
}

// A branch that was already there is not ours to remove.
//
// The cleanup above deletes a branch on the way out of a failed open, which is
// right when the failure is its own and dangerous when it is not. A branch
// standing in the way belongs to something — an earlier lane, a person, a ref
// leaked by exactly this defect — and the first version of that cleanup would
// have destroyed it while reporting a failure to open. This is the guard on the
// guard.
func TestOpenNeverRemovesABranchItDidNotCreate(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", src}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	// Somebody else's work, on the branch name this id would want.
	run("branch", "mellions/taken")
	run("checkout", "-q", "mellions/taken")
	if err := os.WriteFile(filepath.Join(src, "theirs.txt"), []byte("their only copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "their work")
	run("checkout", "-q", "main")

	before, err := exec.Command("git", "-C", src, "rev-parse", "mellions/taken").Output()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Open(OpenOptions{
		ID: "taken", Repo: "svc", Source: src,
		Objective: "wants a branch somebody else has", Because: "the id collides",
	}); err == nil {
		t.Fatal("opening onto an existing branch succeeded")
	}

	after, err := exec.Command("git", "-C", src, "rev-parse", "mellions/taken").Output()
	if err != nil {
		t.Fatalf("the branch is gone: a failed open destroyed work it did not create: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("the branch moved: was %s, now %s", strings.TrimSpace(string(before)), strings.TrimSpace(string(after)))
	}
}

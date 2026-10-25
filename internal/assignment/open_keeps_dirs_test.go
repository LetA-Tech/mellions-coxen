package assignment

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeTheirs(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "theirs.txt")
	if err := os.WriteFile(f, []byte("their only copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// A failed open removes only a directory it made. Here the lane's place
// already holds somebody's file and the branch the id wants is taken: the open
// fails and the file is where it was.
func TestAFailedOpenNeverRemovesADirectoryItDidNotMake(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", src, "branch", "mellions/taken").CombinedOutput(); err != nil {
		t.Fatalf("branch: %s", out)
	}
	theirs := writeTheirs(t, s.dir("taken"))
	if _, err := s.Open(OpenOptions{ID: "taken", Repo: "svc", Source: src, Objective: "o", Because: "b"}); err == nil {
		t.Fatal("opening onto an existing branch succeeded")
	}
	if _, err := os.Stat(theirs); err != nil {
		t.Fatalf("a failed open removed a directory it did not make: %v", err)
	}
}

// The lane's place holding files no record claims is refused before anything
// is cut into it, and left as it was.
func TestOpenRefusesADirectoryHoldingFilesNoRecordClaims(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	theirs := writeTheirs(t, s.dir("held"))
	if _, err := s.Open(OpenOptions{ID: "held", Repo: "svc", Source: src, Objective: "o", Because: "b"}); err == nil {
		t.Fatal("opened into a directory holding files no record claims")
	}
	if _, err := os.Stat(theirs); err != nil {
		t.Fatalf("the refused open removed the directory's file: %v", err)
	}
	if out, _ := exec.Command("git", "-C", src, "rev-parse", "--verify", "--quiet", "refs/heads/mellions/held").Output(); len(out) != 0 {
		t.Fatal("the refused open cut a branch")
	}
}

// A tree adopted from inside the lane's own place survives an adoption that
// fails: nothing adopted is the open's to destroy.
func TestAFailedAdoptionKeepsTheTree(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	tree := filepath.Join(s.dir("imp"), "tree")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "--detach", tree).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s", out)
	}
	if _, err := s.Open(OpenOptions{ID: "imp", Repo: "r", Source: repo, Objective: "o", Because: "b", Worktree: tree}); err == nil {
		t.Fatal("adopted a tree on no branch")
	}
	if _, err := os.Stat(filepath.Join(tree, ".git")); err != nil {
		t.Fatalf("a failed adoption removed the tree: %v", err)
	}
}

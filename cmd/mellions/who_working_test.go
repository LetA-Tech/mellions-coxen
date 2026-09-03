// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// TestASessionAsksAboutTheLaneItHoldsWhenItStandsInNoRepository.
//
// A shift's process stands in its home directory for its whole life, and both
// `who` and the awareness note ask about that directory on every turn. The
// repository it actually works in exists nowhere but its own record, so reading
// only the directory leaves two shifts on one repository invisible to each
// other.
func TestASessionAsksAboutTheLaneItHoldsWhenItStandsInNoRepository(t *testing.T) {
	home := t.TempDir() // no repository
	lane := t.TempDir()
	me := "shift-1"
	live := []presence.Session{
		{ID: me, PID: os.Getpid(), Cwd: lane, Repo: "mellions-coxen", Branch: "mellions/task-42", Assignment: "task-42"},
		{ID: "shift-2", PID: 1 << 22, Cwd: t.TempDir(), Repo: "mellions-coxen"},
	}
	tree, repo := workingIn(live, me, os.Getpid(), home)
	if repo != "mellions-coxen" {
		t.Errorf("repo = %q, want the lane's repository", repo)
	}
	if tree != lane {
		t.Errorf("tree = %q, want the lane's worktree %q", tree, lane)
	}
}

// TestTheDirectoryWinsWhenItIsARepository. The fallback reads a record only
// where the directory settles nothing; a `-C` naming a repository is the answer.
func TestTheDirectoryWinsWhenItIsARepository(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v: %s", err, out)
	}
	live := []presence.Session{
		{ID: "shift-1", PID: os.Getpid(), Cwd: t.TempDir(), Repo: "somewhere-else"},
	}
	tree, repo := workingIn(live, "shift-1", os.Getpid(), dir)
	if repo != filepath.Base(dir) {
		t.Errorf("repo = %q, want the repository the directory is in, %q", repo, filepath.Base(dir))
	}
	if tree != dir {
		t.Errorf("tree = %q, want the directory asked about %q", tree, dir)
	}
}

// TestNoLaneLeavesTheAnswerEmpty. A session that holds no lane and stands in no
// repository is still live and still listed; it must not borrow a peer's
// repository to say so.
func TestNoLaneLeavesTheAnswerEmpty(t *testing.T) {
	home := t.TempDir()
	live := []presence.Session{
		{ID: "shift-1", PID: os.Getpid(), Cwd: home},
		{ID: "shift-2", PID: 1 << 22, Cwd: t.TempDir(), Repo: "mellions-coxen"},
	}
	tree, repo := workingIn(live, "shift-1", os.Getpid(), home)
	if repo != "" {
		t.Errorf("repo = %q, want none — this session holds no lane", repo)
	}
	if tree != home {
		t.Errorf("tree = %q, want the directory asked about %q", tree, home)
	}
}

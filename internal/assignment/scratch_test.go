package assignment

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// groupWritableUmask runs the rest of the test under the umask this host's
// sessions run under, so a directory made with the process's defaults comes out
// group-writable as theirs do.
func groupWritableUmask(t *testing.T) {
	t.Helper()
	old := syscall.Umask(0o002)
	t.Cleanup(func() { syscall.Umask(old) })
}

func assertOwnerOnly(t *testing.T, dir string) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("the lane has no scratch at %s: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	if perm := info.Mode().Perm(); perm&0o022 != 0 {
		t.Fatalf("the lane's scratch %s is mode %o: a check that refuses a writable ancestor of its temporary directory fails under it", dir, perm)
	}
}

// A cut lane carries its own scratch beside its record, writable by its owner
// alone even under umask 002, and its brief says where it is.
func TestOpenMakesTheLanesScratchOwnerOnly(t *testing.T) {
	groupWritableUmask(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := mustOpen(t, s, OpenOptions{ID: "cut", Repo: "svc", Source: gitFixture(t), Objective: "o", Because: "b"})
	want := filepath.Join(s.dir("cut"), "scratch")
	if a.Scratch != want {
		t.Fatalf("the record names scratch %q, want %q", a.Scratch, want)
	}
	assertOwnerOnly(t, want)
	got, err := s.Get("cut")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scratch != want {
		t.Fatalf("the saved record names scratch %q, want %q", got.Scratch, want)
	}
	if brief := got.Text(time.Now()); !strings.Contains(brief, "- scratch: "+want) {
		t.Fatalf("the brief does not say where the lane's scratch is:\n%s", brief)
	}
}

// A scratch already at an adopted lane's place, made group-writable by a
// session's mkdir, is left without group or other write, and what it holds
// stays.
func TestAdoptingALaneTightensAGroupWritableScratch(t *testing.T) {
	groupWritableUmask(t)
	repo := realRepo(t)
	s := newStore(t)
	scratch := filepath.Join(s.dir("adp"), "scratch")
	if err := os.MkdirAll(scratch, 0o775); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scratch, 0o775); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(scratch, "theirs.txt")
	if err := os.WriteFile(kept, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(t.TempDir(), "adopted")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-b", "lane/adp", tree).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s", out)
	}
	a := mustOpen(t, s, OpenOptions{ID: "adp", Repo: "r", Source: repo, Objective: "o", Because: "b", Worktree: tree})
	if a.Scratch != scratch {
		t.Fatalf("the record names scratch %q, want %q", a.Scratch, scratch)
	}
	assertOwnerOnly(t, scratch)
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("adopting the lane removed what its scratch held: %v", err)
	}
}

// An open that fails into a lane place it did not make leaves no scratch
// there: one left behind is a file no record claims, and every later open of
// the id refuses on it.
func TestAFailedOpenLeavesNoScratchInAPlaceItDidNotMake(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.dir("again"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", src, "branch", "mellions/again").CombinedOutput(); err != nil {
		t.Fatalf("branch: %s", out)
	}
	if _, err := s.Open(OpenOptions{ID: "again", Repo: "svc", Source: src, Objective: "o", Because: "b"}); err == nil {
		t.Fatal("opening onto an existing branch succeeded")
	}
	if _, err := os.Stat(filepath.Join(s.dir("again"), "scratch")); !os.IsNotExist(err) {
		t.Fatalf("a failed open left its scratch behind (stat: %v)", err)
	}
	if out, err := exec.Command("git", "-C", src, "branch", "-D", "mellions/again").CombinedOutput(); err != nil {
		t.Fatalf("branch -D: %s", out)
	}
	mustOpen(t, s, OpenOptions{ID: "again", Repo: "svc", Source: src, Objective: "o", Because: "b"})
}

// loosen leaves a lane's scratch as a session's own mkdir under umask 002 made
// it before assign open made one.
func loosen(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o775); err != nil {
		t.Fatal(err)
	}
}

// A lane continued with Take gets its group-writable scratch tightened, and one
// that is missing made again.
func TestTakingALaneTightensItsScratch(t *testing.T) {
	groupWritableUmask(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := mustOpen(t, s, OpenOptions{ID: "take", Repo: "svc", Source: gitFixture(t), Objective: "o", Because: "b"})
	loosen(t, a.Scratch)
	if _, err := s.Take("take"); err != nil {
		t.Fatal(err)
	}
	assertOwnerOnly(t, a.Scratch)
	if err := os.Remove(a.Scratch); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Take("take"); err != nil {
		t.Fatal(err)
	}
	assertOwnerOnly(t, a.Scratch)
}

// A handed-off lane taken back with Reopen gets its group-writable scratch
// tightened.
func TestReopeningALaneTightensItsScratch(t *testing.T) {
	groupWritableUmask(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "back", Repo: "r", Source: realRepo(t), Objective: "o", Because: "b"})
	if err := s.Handoff(a.ID, "review pending"); err != nil {
		t.Fatal(err)
	}
	loosen(t, a.Scratch)
	if _, err := s.update(a.ID, func(r *Assignment) error { r.Scratch = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	re, err := s.Reopen(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if re.Scratch != a.Scratch {
		t.Fatalf("the reopened record names scratch %q, want %q", re.Scratch, a.Scratch)
	}
	assertOwnerOnly(t, a.Scratch)
}

// assertAncestryOwnerOnly checks what hipsys's deploy reader walks: the scratch
// and every directory above it up to the store root carry no group or other
// write.
func assertAncestryOwnerOnly(t *testing.T, scratch, root string) {
	t.Helper()
	for d := scratch; ; d = filepath.Dir(d) {
		assertOwnerOnly(t, d)
		if d == root || d == filepath.Dir(d) {
			return
		}
	}
}

// sessionMadeLane leaves a lane's directory, and the store root above it, as a
// session's mkdir under umask 002 makes them before assign open runs.
func sessionMadeLane(t *testing.T, s *Store, id string) {
	t.Helper()
	if err := os.MkdirAll(s.dir(id), 0o775); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{s.dir(id), s.Root} {
		if err := os.Chmod(d, 0o775); err != nil {
			t.Fatal(err)
		}
	}
}

// A lane cut into a directory a session already made group-writable has no
// group or other write anywhere from its scratch up to the store root.
func TestOpenIntoASessionMadeLaneTightensTheScratchsAncestry(t *testing.T) {
	groupWritableUmask(t)
	s := newStore(t)
	sessionMadeLane(t, s, "pre")
	a := mustOpen(t, s, OpenOptions{ID: "pre", Repo: "svc", Source: gitFixture(t), Objective: "o", Because: "b"})
	assertAncestryOwnerOnly(t, a.Scratch, s.Root)
}

// The same holds for a lane adopted into such a directory.
func TestAdoptIntoASessionMadeLaneTightensTheScratchsAncestry(t *testing.T) {
	groupWritableUmask(t)
	repo := realRepo(t)
	s := newStore(t)
	sessionMadeLane(t, s, "apre")
	tree := filepath.Join(t.TempDir(), "adopted")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-b", "lane/apre", tree).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s", out)
	}
	a := mustOpen(t, s, OpenOptions{ID: "apre", Repo: "r", Source: repo, Objective: "o", Because: "b", Worktree: tree})
	assertAncestryOwnerOnly(t, a.Scratch, s.Root)
}

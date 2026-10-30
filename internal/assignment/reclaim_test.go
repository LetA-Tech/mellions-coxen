package assignment

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// A finished lane's scratch as lanes on the host leave it: an archive
// extraction, a Go build cache, a binary over the bound, and the small text
// files that are the lane's evidence.
func fillScratch(t *testing.T, scratch, outside string) {
	t.Helper()
	writeFile(t, filepath.Join(scratch, "base", "go.mod"), []byte("module x\n"))
	writeFile(t, filepath.Join(scratch, "base", "README.md"), []byte("source, not evidence\n"))
	writeFile(t, filepath.Join(scratch, "base", "pkg", "a.go"), []byte("package pkg\n"))
	readOnly := filepath.Join(scratch, "base", "pkg")
	if err := os.Chmod(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(readOnly, 0o700) })
	writeFile(t, filepath.Join(scratch, "gocache", "trim.txt"), []byte("1\n"))
	writeFile(t, filepath.Join(scratch, "gocache", "b4", strings.Repeat("b", 64)+"-d"), []byte("ok\n"))
	writeFile(t, filepath.Join(scratch, "tmp", "x-go-build.tree.1", "c5", strings.Repeat("c", 64)+"-a"), []byte("v1\n"))
	writeFile(t, filepath.Join(scratch, "gotmp", "go-build123", "b001", "exe"), []byte{0x7f, 'E', 'L', 'F', 0})
	writeFile(t, filepath.Join(scratch, "bin", "mellions"), make([]byte, EvidenceBound+1))
	writeFile(t, filepath.Join(scratch, "small.bin"), []byte{1, 0, 2})
	writeFile(t, filepath.Join(scratch, "logs", "gate.log"), []byte("ok  pkg 0.1s\n"))
	writeFile(t, filepath.Join(scratch, "review.md"), []byte("# READY\n"))
	lane := filepath.Dir(scratch)
	writeFile(t, filepath.Join(lane, "evidence", "pkg-copy", "go.mod"), []byte("module y\n"))
	writeFile(t, filepath.Join(lane, "evidence", "pkg-copy", "big_test.go"), []byte("package y\n"))
	writeFile(t, filepath.Join(lane, "evidence", "arm-m1.txt"), []byte("--- FAIL: TestX\n"))
	writeFile(t, filepath.Join(lane, "suite.log"), []byte("ok\n"))
	writeFile(t, outside, []byte("not the lane's\n"))
	clone := filepath.Join(scratch, "sibling")
	writeFile(t, filepath.Join(clone, "go.mod"), []byte("module sibling\n"))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "local only"}} {
		if out, err := exec.Command("git", append([]string{"-C", clone}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.Symlink(outside, filepath.Join(scratch, "link")); err != nil {
		t.Fatal(err)
	}
}

func TestSweepReclaimsAFinishedLanesRebuildableScratchAfterTheHorizon(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	now := time.Date(2026, 10, 30, 4, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	done := handedOff(t, s, "fx-done", "rates-service", repo, "Merged as PR #7.")
	scratch := filepath.Join(s.dir("fx-done"), "scratch")
	outside := filepath.Join(t.TempDir(), "shared.txt")
	fillScratch(t, scratch, outside)
	if err := s.Close(done.ID); err != nil {
		t.Fatal(err)
	}
	// A worktree git would not detach stays where close left it.
	writeFile(t, filepath.Join(s.dir(done.ID), "tree", "go.mod"), []byte("module worktree\n"))
	// A clean worktree of another repository, cut inside the lane for an arm.
	armTree := filepath.Join(scratch, "arm-wt")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "--detach", armTree).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	live := mustOpen(t, s, OpenOptions{ID: "fx-live", Repo: "rates-service", Source: repo,
		Objective: "still working", Because: "the owner asked for it"})
	liveFile := filepath.Join(s.dir(live.ID), "scratch", "bin", "big")
	writeFile(t, liveFile, make([]byte, EvidenceBound+1))

	o := SweepOptions{ReclaimAfter: 24 * time.Hour}
	now = now.Add(23 * time.Hour)
	if got := verdicts(mustSweep(t, s, o)); got["fx-done"].Verdict != "" {
		t.Fatalf("inside the horizon: %+v, want the lane's scratch left whole", got["fx-done"])
	}

	now = now.Add(2 * time.Hour)
	got := verdicts(mustSweep(t, s, o))
	if v := got["fx-done"]; v.Verdict != "reclaimable" || !strings.Contains(v.Why, "6 evidence file(s) kept") {
		t.Fatalf("dry run past the horizon: %+v, want reclaimable keeping 4 evidence files, the unpushed clone and the arm worktree", v)
	}
	if !exists(filepath.Join(scratch, "base", "go.mod")) || !exists(filepath.Join(scratch, "bin", "mellions")) {
		t.Fatal("a dry run removed scratch")
	}

	o.Apply = true
	got = verdicts(mustSweep(t, s, o))
	v := got["fx-done"]
	if v.Verdict != "reclaimed" || v.Bytes <= EvidenceBound {
		t.Fatalf("apply: %+v, want reclaimed with the binary's bytes counted", v)
	}
	for _, gone := range []string{"base", "gocache", "tmp", "gotmp", "bin", "small.bin", "link"} {
		if exists(filepath.Join(scratch, gone)) {
			t.Errorf("%s survived the reclaim; it is rebuildable", gone)
		}
	}
	for _, kept := range []string{"logs/gate.log", "review.md"} {
		if !exists(filepath.Join(scratch, kept)) {
			t.Errorf("%s was removed; it is the lane's evidence", kept)
		}
	}
	if !exists(filepath.Join(scratch, "sibling", ".git")) || !exists(filepath.Join(scratch, "sibling", "go.mod")) {
		t.Error("a clone holding a commit no remote carries was reclaimed; it is not rebuildable")
	}
	if !exists(filepath.Join(armTree, ".git")) {
		t.Error("a linked worktree inside the lane was removed outside git; its repository still registers it")
	}
	lane := s.dir("fx-done")
	if exists(filepath.Join(lane, "evidence", "pkg-copy")) {
		t.Error("an archive copy beside the record survived the reclaim")
	}
	for _, kept := range []string{"assignment.json", "evidence/arm-m1.txt", "suite.log", "tree/go.mod"} {
		if !exists(filepath.Join(lane, kept)) {
			t.Errorf("%s was removed; the record, the evidence beside it and the worktree are never reclaimed", kept)
		}
	}
	if !exists(outside) {
		t.Error("the reclaim followed a symlink out of the scratch and removed its target")
	}
	if !exists(liveFile) {
		t.Error("an open lane's scratch was reclaimed")
	}
	a, err := s.Get("fx-done")
	if err != nil {
		t.Fatal(err)
	}
	if a.Reclaimed == nil || a.Reclaimed.Bytes != v.Bytes || a.Reclaimed.Kept != 6 || !a.Reclaimed.At.Equal(now) {
		t.Fatalf("record: %+v, want what the reclaim freed and kept, and when", a.Reclaimed)
	}

	if got := verdicts(mustSweep(t, s, o)); got["fx-done"].Verdict != "" {
		t.Errorf("a reclaimed lane was reclaimed again: %+v", got["fx-done"])
	}
}

func TestSweepLeavesScratchAloneWithoutAHorizon(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	now := time.Date(2026, 10, 30, 4, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	done := handedOff(t, s, "fx-done", "rates-service", repo, "Merged as PR #7.")
	big := filepath.Join(s.dir(done.ID), "scratch", "bin", "big")
	writeFile(t, big, make([]byte, EvidenceBound+1))
	if err := s.Close(done.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * 24 * time.Hour)
	if got := verdicts(mustSweep(t, s, SweepOptions{Apply: true})); got["fx-done"].Verdict != "" {
		t.Errorf("ReclaimAfter zero: %+v, want no reclaim", got["fx-done"])
	}
	if !exists(big) {
		t.Error("scratch removed with ReclaimAfter zero")
	}
}

// A record is data. One whose id field names another lane, or no lane, is
// never the key to which directory gets emptied.
func TestSweepNeverReclaimsByARecordsIDField(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	now := time.Date(2026, 10, 30, 4, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	live := mustOpen(t, s, OpenOptions{ID: "fx-live", Repo: "rates-service", Source: repo,
		Objective: "still working", Because: "the owner asked for it"})
	liveFile := filepath.Join(s.dir(live.ID), "scratch", "bin", "big")
	writeFile(t, liveFile, make([]byte, EvidenceBound+1))
	for _, id := range []string{"fx-blank", "fx-other"} {
		handedOff(t, s, id, "rates-service", repo, "Merged.")
		if err := s.Close(id); err != nil {
			t.Fatal(err)
		}
	}
	for dir, claimed := range map[string]string{"fx-blank": "", "fx-other": "fx-live"} {
		a, err := s.Get(dir)
		if err != nil {
			t.Fatal(err)
		}
		a.ID = claimed
		raw, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, s.file(dir), raw)
	}
	now = now.Add(48 * time.Hour)
	for _, v := range mustSweep(t, s, SweepOptions{Apply: true, ReclaimAfter: time.Hour}) {
		if v.Verdict == "reclaimed" {
			t.Errorf("reclaimed by a record's id field: %+v", v)
		}
	}
	if !exists(liveFile) {
		t.Fatal("an open lane's scratch was emptied through another record's id field")
	}
	if !exists(s.dir(live.ID) + "/tree") {
		t.Fatal("an open lane's worktree was removed through another record's id field")
	}
}

func TestReclaimReadsPastAnUnreadableDirectory(t *testing.T) {
	lane := t.TempDir()
	locked := filepath.Join(lane, "a-locked")
	writeFile(t, filepath.Join(locked, "x.log"), []byte("x\n"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	big := filepath.Join(lane, "z.bin")
	writeFile(t, big, make([]byte, EvidenceBound+1))
	r, err := reclaimLane(lane, nil, true)
	if err != nil {
		t.Fatalf("one unreadable directory failed the lane: %v", err)
	}
	if exists(big) || r.Unreadable != 1 {
		t.Fatalf("z.bin present=%v unreadable=%d, want it removed past the locked directory and 1 unreadable named", exists(big), r.Unreadable)
	}
}

func reclaimGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// Work git holds anywhere below a rebuildable directory keeps that work: a
// linked worktree nested in an archive copy, a clone whose only commit is on
// a detached HEAD, and one whose only unsaved work is a stash.
func TestReclaimKeepsGitWorkBelowARebuildableDirectory(t *testing.T) {
	repo := realRepo(t)
	lane := t.TempDir()
	writeFile(t, filepath.Join(lane, "arms", "x", "go.mod"), []byte("module x\n"))
	writeFile(t, filepath.Join(lane, "arms", "x", "big.bin"), make([]byte, EvidenceBound+1))
	nested := filepath.Join(lane, "arms", "x", "wt")
	reclaimGit(t, repo, "worktree", "add", "-q", "--detach", nested)

	detached := filepath.Join(lane, "detached")
	writeFile(t, filepath.Join(detached, "go.mod"), []byte("module d\n"))
	reclaimGit(t, detached, "init", "-q", "-b", "work")
	reclaimGit(t, detached, "add", ".")
	reclaimGit(t, detached, "commit", "-q", "-m", "one")
	reclaimGit(t, detached, "checkout", "-q", "--detach")
	writeFile(t, filepath.Join(detached, "local.go"), []byte("package d\n"))
	reclaimGit(t, detached, "add", ".")
	reclaimGit(t, detached, "commit", "-q", "-m", "only on HEAD")
	reclaimGit(t, detached, "branch", "-q", "-D", "work")
	reclaimGit(t, detached, "remote", "add", "origin", "https://example.invalid/d")

	stashed := filepath.Join(lane, "stashed")
	writeFile(t, filepath.Join(stashed, "go.mod"), []byte("module s\n"))
	reclaimGit(t, stashed, "init", "-q", "-b", "work")
	reclaimGit(t, stashed, "add", ".")
	reclaimGit(t, stashed, "commit", "-q", "-m", "one")
	reclaimGit(t, stashed, "update-ref", "refs/remotes/origin/work", "HEAD")
	writeFile(t, filepath.Join(stashed, "go.mod"), []byte("module s // edited\n"))
	reclaimGit(t, stashed, "stash", "-q")

	pushed := filepath.Join(lane, "pushed")
	writeFile(t, filepath.Join(pushed, "go.mod"), []byte("module p\n"))
	reclaimGit(t, pushed, "init", "-q", "-b", "work")
	reclaimGit(t, pushed, "add", ".")
	reclaimGit(t, pushed, "commit", "-q", "-m", "one")
	reclaimGit(t, pushed, "update-ref", "refs/remotes/origin/work", "HEAD")

	// A commit pushed only to a bare repository inside the lane.
	bare := filepath.Join(lane, "keep", "origin.git")
	if err := os.MkdirAll(bare, 0o700); err != nil {
		t.Fatal(err)
	}
	reclaimGit(t, bare, "init", "-q", "--bare", "-b", "work")
	work := filepath.Join(lane, "work")
	writeFile(t, filepath.Join(work, "go.mod"), []byte("module w\n"))
	reclaimGit(t, work, "init", "-q", "-b", "work")
	reclaimGit(t, work, "add", ".")
	reclaimGit(t, work, "commit", "-q", "-m", "only in the bare repository")
	reclaimGit(t, work, "remote", "add", "origin", bare)
	reclaimGit(t, work, "push", "-q", "origin", "work")
	reclaimGit(t, bare, "gc", "-q")
	writeFile(t, filepath.Join(lane, "history.bundle"), append([]byte{0}, make([]byte, EvidenceBound+1)...))

	if _, err := reclaimLane(lane, nil, true); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", bare, "fsck", "--no-progress").CombinedOutput(); err != nil {
		t.Errorf("a kept bare repository no longer reads as one: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", bare, "rev-parse", "--verify", "-q", "work^{commit}").CombinedOutput(); err != nil {
		t.Errorf("a bare repository holding the only copy of a commit was gutted: %v\n%s", err, out)
	}
	if !exists(filepath.Join(lane, "history.bundle")) {
		t.Error("a git bundle was reclaimed; it is history archived on purpose")
	}
	if !exists(filepath.Join(nested, ".git")) {
		t.Error("a linked worktree nested in a rebuildable directory was taken with it")
	}
	if exists(filepath.Join(lane, "arms", "x", "big.bin")) {
		t.Error("the binary beside the nested worktree survived; the walk went in and should still judge it")
	}
	if !exists(filepath.Join(detached, ".git")) {
		t.Error("a clone whose only commit is on a detached HEAD was reclaimed")
	}
	if !exists(filepath.Join(stashed, ".git")) {
		t.Error("a clone whose only unsaved work is a stash was reclaimed")
	}
	if exists(filepath.Join(lane, "pushed")) {
		t.Error("a clean clone every commit of which a remote carries survived; it is rebuildable")
	}
}

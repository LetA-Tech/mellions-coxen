package assignment

import (
	"os"
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
	if err := os.Chmod(filepath.Join(scratch, "base", "pkg"), 0o555); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scratch, "gocache", "trim.txt"), []byte("1\n"))
	writeFile(t, filepath.Join(scratch, "gocache", "b4", strings.Repeat("b", 64)+"-d"), []byte("ok\n"))
	writeFile(t, filepath.Join(scratch, "tmp", "x-go-build.tree.1", "c5", strings.Repeat("c", 64)+"-a"), []byte("v1\n"))
	writeFile(t, filepath.Join(scratch, "gotmp", "go-build123", "b001", "exe"), []byte{0x7f, 'E', 'L', 'F', 0})
	writeFile(t, filepath.Join(scratch, "bin", "mellions"), make([]byte, EvidenceBound+1))
	writeFile(t, filepath.Join(scratch, "small.bin"), []byte{1, 0, 2})
	writeFile(t, filepath.Join(scratch, "logs", "gate.log"), []byte("ok  pkg 0.1s\n"))
	writeFile(t, filepath.Join(scratch, "review.md"), []byte("# READY\n"))
	writeFile(t, outside, []byte("not the lane's\n"))
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
	if v := got["fx-done"]; v.Verdict != "reclaimable" || !strings.Contains(v.Why, "2 evidence file(s) kept") {
		t.Fatalf("dry run past the horizon: %+v, want reclaimable keeping 2 evidence files", v)
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
	if a.Reclaimed == nil || a.Reclaimed.Bytes != v.Bytes || a.Reclaimed.Kept != 2 || !a.Reclaimed.At.Equal(now) {
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

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// digestHome is a state directory with what the shift and the reports write.
func digestHome(t *testing.T) (*Config, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MELLIONS_HOME", home)
	for _, d := range []string{"shifts", "reports", "partners"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &Config{
		ReportRoot:      home,
		AssignmentsRoot: filepath.Join(home, "assignments"),
		PartnersDir:     filepath.Join(home, "partners"),
	}, home
}

func writeAt(t *testing.T, path, body string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func digestText(t *testing.T, cfg *Config, brief bool, now time.Time) string {
	t.Helper()
	var b strings.Builder
	if err := reportDigest(cfg, brief, now, &b); err != nil {
		t.Fatalf("reportDigest: %v", err)
	}
	return b.String()
}

// TestDigestSaysWhatNeedsTheOwnerOncePerWindow: two shift replies and one
// report that stopped on the owner reach the first session; the second
// session inside the window gets nothing; after the window only what is new
// is said, and a standing count of lanes handed to the owner is said again.
func TestDigestSaysWhatNeedsTheOwnerOncePerWindow(t *testing.T) {
	cfg, home := digestHome(t)
	now := time.Now()
	earlier := now.Add(-10 * time.Hour)

	writeAt(t, filepath.Join(home, "shifts", "20260828-010000.reply.md"),
		"Done, 10 min to spare.\n\n## Shipped — PR #30 merged to dev\n\nDetail.\n", earlier)
	writeAt(t, filepath.Join(home, "shifts", "20260828-020000.reply.md"),
		"Nothing material happened; the survey named no work worth a lane.\n", earlier)
	writeAt(t, filepath.Join(home, "reports", "20260828-030000-fx-1.md"),
		"# 2026-08-28 03:00 UTC — fx-1\n\n## Needs you\n\nThe RLS decision on finrate.schema_migrations is yours; three options are on #91.\n\n## What I did\n\nFiled #91.\n", earlier.Add(time.Minute))
	writeAt(t, filepath.Join(home, "reports", "20260828-030100.md"),
		"# 2026-08-28 03:01 UTC\n\n## What I did\n\nA quiet run.\n", earlier)
	writeAt(t, filepath.Join(home, "partners", "alex.md"), "# the operator\n", earlier)

	store, err := assignment.NewStore(cfg.assignmentsRoot())
	if err != nil {
		t.Fatal(err)
	}
	repo := claimRepo(t)
	for id, handoff := range map[string]string{
		"fx-1": "Done and merged. The decision package for the owner is on #91.",
		"fx-2": "Done and merged; nothing outstanding.",
	} {
		if _, err := store.Open(assignment.OpenOptions{ID: id, Repo: "rates-service", Source: repo,
			Objective: "o", Because: "the owner asked for it"}); err != nil {
			t.Fatal(err)
		}
		if err := store.Handoff(id, handoff); err != nil {
			t.Fatal(err)
		}
	}

	first := digestText(t, cfg, true, now)
	for _, want := range []string{
		"since the beginning of the record",
		"- shift 20260828-010000 on ",
		": Shipped — PR #30 merged to dev",
		"- shift 20260828-020000 on ",
		": Nothing material happened; the survey named no work worth a lane.",
		"- report 20260828-030000-fx-1 needs you: The RLS decision on finrate.schema_migrations is yours",
		"- 1 handed-off lane names you in the handoff",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("the first digest lacks %q:\n%s", want, first)
		}
	}
	if strings.Contains(first, "20260828-030100") {
		t.Errorf("a report with nothing for the owner was digested:\n%s", first)
	}
	// Newest first: the report came after both shifts.
	if strings.Index(first, "report 20260828-030000") > strings.Index(first, "shift 20260828-010000") {
		t.Errorf("the digest is not newest first:\n%s", first)
	}
	marker := filepath.Join(home, "digest-seen")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("saying the digest did not touch %s: %v", marker, err)
	}

	if again := digestText(t, cfg, true, now.Add(time.Minute)); again != "" {
		t.Errorf("a second session inside the window was told it again:\n%s", again)
	}

	// The window passes; one new shift lands after the marker.
	if err := os.Chtimes(marker, now.Add(-9*time.Hour), now.Add(-9*time.Hour)); err != nil {
		t.Fatal(err)
	}
	writeAt(t, filepath.Join(home, "shifts", "20260828-090000.reply.md"), "Done.\n\n## #75 fixed and merged\n", now)
	later := digestText(t, cfg, true, now.Add(time.Second))
	if !strings.Contains(later, "- shift 20260828-090000 on ") || !strings.Contains(later, ": #75 fixed and merged") {
		t.Errorf("after the window the new shift was not said:\n%s", later)
	}
	for _, old := range []string{"20260828-010000", "20260828-020000", "20260828-030000"} {
		if strings.Contains(later, old) {
			t.Errorf("after the window %s, older than the marker, was said again:\n%s", old, later)
		}
	}
	if !strings.Contains(later, "- 1 handed-off lane names you") {
		t.Errorf("the standing count of lanes handed to the owner was dropped:\n%s", later)
	}
	if fi, _ := os.Stat(marker); fi.ModTime().Before(now) {
		t.Errorf("saying the digest again did not move the marker: %s", fi.ModTime())
	}

	// A silent digest moves nothing, so the interval is read again next time.
	if err := os.Chtimes(marker, now.Add(-9*time.Hour), now.Add(-9*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, "shifts", "20260828-090000.reply.md")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"fx-1", "fx-2"} {
		if err := store.Close(id); err != nil {
			t.Fatal(err)
		}
	}
	if quiet := digestText(t, cfg, true, now.Add(2*time.Second)); quiet != "" {
		t.Errorf("with nothing to say the digest said:\n%s", quiet)
	}
	if fi, _ := os.Stat(marker); !fi.ModTime().Before(now.Add(-8 * time.Hour)) {
		t.Errorf("a silent digest touched the marker: %s", fi.ModTime())
	}
}

// TestDigestWithoutBriefIsTheWholeThing: the form a person runs prints inside
// the window, is unbounded, and never touches the marker.
func TestDigestWithoutBriefIsTheWholeThing(t *testing.T) {
	cfg, home := digestHome(t)
	now := time.Now()
	writeAt(t, filepath.Join(home, "shifts", "20260828-010000.reply.md"), "## Shipped\n", now.Add(-time.Hour))
	writeAt(t, filepath.Join(home, "digest-seen"), "", now.Add(-2*time.Hour))
	if brief := digestText(t, cfg, true, now); brief != "" {
		t.Fatalf("inside the window the brief form said:\n%s", brief)
	}
	full := digestText(t, cfg, false, now)
	if !strings.Contains(full, "- shift 20260828-010000 on ") {
		t.Errorf("the full form did not print the shift inside the window:\n%s", full)
	}
	if fi, _ := os.Stat(filepath.Join(home, "digest-seen")); !fi.ModTime().Before(now.Add(-time.Hour)) {
		t.Errorf("the full form touched the marker: %s", fi.ModTime())
	}
	if err := os.Remove(filepath.Join(home, "shifts", "20260828-010000.reply.md")); err != nil {
		t.Fatal(err)
	}
	if empty := digestText(t, cfg, false, now); !strings.HasPrefix(empty, "nothing for the owner since ") {
		t.Errorf("the full form with nothing to say printed:\n%q", empty)
	}
}

// TestDigestBriefStaysUnderTheHookBound: a host with a hundred shifts since
// the marker says the newest and counts the rest, rather than being cut
// mid-line by the hook's byte bound.
func TestDigestBriefStaysUnderTheHookBound(t *testing.T) {
	cfg, home := digestHome(t)
	now := time.Now()
	for i := 0; i < 100; i++ {
		writeAt(t, filepath.Join(home, "shifts", fmt.Sprintf("20260828-%06d.reply.md", i)),
			fmt.Sprintf("## Shift %d — %s\n", i, strings.Repeat("x", 100)), now.Add(-time.Duration(100-i)*time.Minute))
	}
	out := digestText(t, cfg, true, now)
	if len(out) > digestBudget {
		t.Errorf("the brief digest is %d bytes, over the %d bound", len(out), digestBudget)
	}
	if !strings.Contains(out, "Shift 99 —") {
		t.Errorf("the newest shift is not the one kept:\n%s", out)
	}
	if !strings.Contains(out, " more — `mellions report digest` has them all") {
		t.Errorf("the cut is not said:\n%s", out)
	}
}

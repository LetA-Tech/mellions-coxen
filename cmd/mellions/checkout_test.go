// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitDir makes dir look like a checkout to the resolver.
func gitDir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A repository is reachable by naming where it is, whether or not it is also in
// the scope the engineer surveys. The error a missing checkout raises tells the
// reader to add a "checkouts" entry, so an entry that resolves nowhere makes
// that instruction false — and the repository it most often names is Mellions'
// own source, which is worked in and deliberately not surveyed.
func TestCheckoutsEntryResolvesOutsideRepos(t *testing.T) {
	root := t.TempDir()
	work := gitDir(t, filepath.Join(root, "work", "in-scope"))
	own := gitDir(t, filepath.Join(root, "elsewhere", "mellions-coxen"))

	cfg := &Config{
		Repos:      []string{"in-scope"},
		WorkRoot:   filepath.Join(root, "work"),
		CheckoutAt: map[string]string{"mellions-coxen": own},
	}

	got, err := cfg.checkout("mellions-coxen")
	if err != nil {
		t.Fatalf("checkout of a repository named in %q but absent from %q = %v, want it found",
			"checkouts", "repos", err)
	}
	if got != own {
		t.Errorf("checkout = %s, want %s", got, own)
	}

	if got, err := cfg.checkout("in-scope"); err != nil || got != work {
		t.Errorf("checkout of an in-scope repository = %q, %v; want %s and no error", got, err, work)
	}
}

// A "checkouts" entry pointing somewhere that is not a checkout is a
// configuration mistake, and saying so beats searching the roots and reporting
// the repository as absent — that reads as "add a checkouts entry", which is
// exactly what was already done.
func TestCheckoutsEntryPointingNowhereSaysSo(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{
		Repos:      []string{"in-scope"},
		WorkRoot:   gitDir(t, filepath.Join(root, "work", "in-scope")),
		CheckoutAt: map[string]string{"absent": filepath.Join(root, "nothing-here")},
	}

	_, err := cfg.checkout("absent")
	if err == nil {
		t.Fatal("a checkouts entry naming a directory that is not a checkout resolved")
	}
	if !strings.Contains(err.Error(), "not a git checkout") {
		t.Errorf("error = %q, want it to name the checkouts entry as the problem", err)
	}
}

// A report written from a document has no flags to read, so the line that tells
// the writer nothing needs the owner would be printed on every one of them —
// including a report whose first section is "Needs you". Saying nothing is the
// only honest answer when the input carries no answer.
func TestReportFromFileClaimsNothingAboutTheOwner(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"report_root":`+strconv.Quote(home)+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(home, "report.md")
	if err := os.WriteFile(doc, []byte("## Needs you\n\nMerge the pull request.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdReport([]string{"write", "-config", cfgPath, "-file", doc}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "nothing here needs the owner") {
		t.Errorf("a report whose body asks for the owner was told %q\noutput: %s",
			"nothing here needs the owner", out)
	}

	// -blocked is "what stopped, and on whom". A report carrying one is not a
	// run with nothing in it for the owner, and the reassurance contradicts the
	// section printed directly above it.
	out = captureStdout(t, func() {
		if err := cmdReport([]string{"write", "-config", cfgPath, "-did", "the work",
			"-blocked", "CI refuses to start any job; only the owner can clear it"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "nothing here needs the owner") {
		t.Errorf("a report that named what stopped and on whom was told %q\noutput: %s",
			"nothing here needs the owner", out)
	}

	out = captureStdout(t, func() {
		if err := cmdReport([]string{"write", "-config", cfgPath, "-did", "a quiet run"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "nothing here needs the owner") {
		t.Errorf("a flag-written report with nothing for the owner lost its reassurance\noutput: %s", out)
	}
}

// captureStdout runs f with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	f()
	w.Close()
	os.Stdout = saved
	return <-done
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPublishedSourceIsThePublicRepository(t *testing.T) {
	if publishedSource != "LetA-Tech/mellions-coxen" {
		t.Fatalf("publishedSource = %q, want the permanent public repository", publishedSource)
	}
}

// TestInstallingClearsWhatIsThereBeforeAddingTheSource.
//
// Both runtimes cache a plugin under the version in its manifest and will not
// re-read the source while that version is unchanged — `claude plugin update`
// answers "already at the latest version (0.1.0)" and refreshes nothing. A
// deployment from a local checkout found this the expensive way: the binary
// updated, the plugin did not, and both runtimes reported success.
//
// The plugin and the marketplace are two objects and both have to go. Clearing
// only the plugin left the marketplace bound to its old source, so installing
// from anywhere else failed outright.
//
// Clearing may fail — removing something that was never there is not an error.
// Adding may not: a silent failure there is an install that reports success over
// the previous content, which is the one outcome an installer must not have.
func TestInstallingClearsWhatIsThereBeforeAddingTheSource(t *testing.T) {
	clearing := func(s string) bool {
		return strings.Contains(s, "remove") || strings.Contains(s, "uninstall")
	}
	for _, a := range adapters {
		steps := a.steps("/some/checkout")
		var cleared, added []string
		seenAdd := false
		for _, st := range steps {
			line := strings.Join(st.args, " ")
			if clearing(line) {
				if seenAdd {
					t.Errorf("%s: %q clears after something was added", a.name, line)
				}
				if !st.spent {
					t.Errorf("%s: %q would fail the install when there is nothing to clear",
						a.name, line)
				}
				cleared = append(cleared, line)
				continue
			}
			seenAdd = true
			if st.spent {
				t.Errorf("%s: %q may not fail silently", a.name, line)
			}
			added = append(added, line)
		}

		if len(cleared) < 2 {
			t.Errorf("%s clears %v — the plugin and the marketplace are two objects and both "+
				"have to go, or a new source cannot replace an old one", a.name, cleared)
		}
		if !strings.Contains(strings.Join(cleared, " "), "marketplace") {
			t.Errorf("%s never drops the marketplace, so installing from a different source "+
				"fails with the marketplace still bound to the old one", a.name)
		}
		if len(added) < 2 {
			t.Errorf("%s adds %v; expected the marketplace and the plugin", a.name, added)
		}
	}
}

// stubRuntimes puts a claude and a codex on PATH that record every command they
// are given, one line each, and exit with the status named for them.
func stubRuntimes(t *testing.T, exits map[string]int) (logs string) {
	t.Helper()
	bin, logs := t.TempDir(), t.TempDir()
	for name, code := range exits {
		script := fmt.Sprintf("#!/bin/sh\necho \"$*\" >> %q\n", filepath.Join(logs, name))
		if code != 0 {
			script += fmt.Sprintf("echo \"env: 'node': No such file or directory\" >&2\nexit %d\n", code)
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MELLIONS_CONFIG", "")
	return logs
}

func stubCalls(t *testing.T, logs, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(logs, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
}

func marketplaceDir(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".claude-plugin", "marketplace.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

// TestARuntimeThatCannotRunKeepsItsRegistration: a runtime found on disk that
// exits 127 on every command — Codex as a node script on a PATH without node —
// is asked only its read-only list; clearing it first would leave it with no
// Mellions.
// The other runtime still installs, and the command still fails, naming why.
func TestARuntimeThatCannotRunKeepsItsRegistration(t *testing.T) {
	logs := stubRuntimes(t, map[string]int{"claude": 0, "codex": 127})
	src := marketplaceDir(t)

	var err error
	out := captureStdout(t, func() { err = cmdInstall(context.Background(), []string{"-from", src}) })

	if got := stubCalls(t, logs, "codex"); !reflect.DeepEqual(got, []string{"plugin list"}) {
		t.Errorf("codex was given %q; a runtime that does not run must be given its read-only list and nothing else", got)
	}
	wantClaude := []string{
		"plugin list",
		"plugin uninstall mellions@mellions",
		"plugin marketplace remove mellions",
		"plugin marketplace add " + src,
		"plugin install mellions@mellions",
	}
	if got := stubCalls(t, logs, "claude"); !reflect.DeepEqual(got, wantClaude) {
		t.Errorf("claude was given %q, want %q", got, wantClaude)
	}
	if err == nil || !strings.Contains(err.Error(), "Codex") || !strings.Contains(err.Error(), "exit status 127") {
		t.Errorf("err = %v; want a failure naming Codex and its exit status", err)
	}
	if !strings.Contains(out, "skipped: Codex does not run here (exit status 127") {
		t.Errorf("output does not report Codex skipped with its exit status:\n%s", out)
	}
}

// TestADryRunRunsNotEvenTheProbe: -dry-run says what would run and runs
// nothing, the read-only probe included.
func TestADryRunRunsNotEvenTheProbe(t *testing.T) {
	logs := stubRuntimes(t, map[string]int{"claude": 0, "codex": 127})
	out := captureStdout(t, func() {
		_ = cmdInstall(context.Background(), []string{"-from", marketplaceDir(t), "-dry-run"})
	})
	for _, name := range []string{"claude", "codex"} {
		if got := stubCalls(t, logs, name); got != nil {
			t.Errorf("a dry run gave %s %q", name, got)
		}
	}
	if !strings.Contains(out, "would run: ") || !strings.Contains(out, " plugin list") {
		t.Errorf("a dry run does not say it would probe first:\n%s", out)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestClosingRefusesToDestroyWhatOnlyTheWorktreeHolds.
//
// Closing removes the worktree. Whatever was in it and nowhere else is gone,
// and the command reports completion while it goes — so a reader afterwards
// cannot tell finished work from destroyed work.
//
// Untracked material is the case that matters most and is the easiest to miss:
// the reproduction script, the captured payload, the failing fixture a session
// wrote while investigating are untracked by definition, and they are the most
// expensive things in the directory to reconstruct.
func TestClosingRefusesToDestroyWhatOnlyTheWorktreeHolds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, wt string)
		says  string
	}{
		{"untracked", func(t *testing.T, wt string) {
			write(t, filepath.Join(wt, "repro.sh"), "#!/bin/sh\n# the only copy\n")
		}, "untracked file"},
		{"modified", func(t *testing.T, wt string) {
			write(t, filepath.Join(wt, "main.go"), "package main\n// changed\n")
		}, "modified file"},
		{"staged", func(t *testing.T, wt string) {
			write(t, filepath.Join(wt, "fixture.json"), "{}\n")
			git(t, wt, "add", "fixture.json")
		}, "staged change"},
		{"conflicted", func(t *testing.T, wt string) {
			write(t, filepath.Join(wt, "both.go"), "package main\n")
			git(t, wt, "add", "both.go")
			write(t, filepath.Join(wt, "both.go"), "package main // and unstaged on top\n")
		}, "staged change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := realRepo(t)
			s := newStore(t)
			a := mustOpen(t, s, OpenOptions{ID: "d1", Source: repo, Objective: "o",
				Because: "the most valuable work available"})
			if err := s.Handoff("d1", "where it stands"); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, a.Worktree)

			err := s.Close("d1")
			if err == nil {
				t.Fatal("closed over material that exists nowhere else")
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the refusal does not name what is at risk (want %q): %v", tc.says, err)
			}
			if !strings.Contains(err.Error(), "abandon") {
				t.Errorf("the refusal does not say what to do instead: %v", err)
			}
			if _, statErr := os.Stat(a.Worktree); statErr != nil {
				t.Errorf("the refused close removed the worktree anyway: %v", statErr)
			}

			// Abandoning is the way through, and it records what went.
			if err := s.Abandon("d1", "superseded by a smaller reproduction", nil); err != nil {
				t.Fatalf("abandon: %v", err)
			}
			got, err := s.Get("d1")
			if err != nil {
				t.Fatal(err)
			}
			if got.State != StateAbandoned {
				t.Errorf("state is %q, want abandoned — a discard must not read as completion", got.State)
			}
			if got.Discarded == nil || !strings.Contains(got.Discarded.Held, tc.says) {
				t.Errorf("the record does not say what the worktree held: %+v", got.Discarded)
			}
		})
	}
}

// TestAbandoningRequiresSayingWhatGoes.
//
// The description is the only record that will exist of material nothing can
// reproduce. An abandonment without one is the force flag again under a
// different name.
func TestAbandoningRequiresSayingWhatGoes(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	mustOpen(t, s, OpenOptions{ID: "d2", Source: repo, Objective: "o",
		Because: "the most valuable work available"})
	if err := s.Abandon("d2", "  ", nil); err == nil {
		t.Fatal("abandoned without saying what was discarded")
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
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

// TestCommittedWorkClosesBecauseTheBranchOutlivesTheWorktree.
//
// Removing a worktree keeps its branch. Refusing to close over commits that
// were never pushed would block the ordinary case — work committed on a branch,
// ready for a pull request — and a check that blocks the ordinary case is one
// somebody will add a flag to turn off.
func TestCommittedWorkClosesBecauseTheBranchOutlivesTheWorktree(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := mustOpen(t, s, OpenOptions{ID: "d3", Source: repo, Objective: "o",
		Because: "the most valuable work available"})
	write(t, filepath.Join(a.Worktree, "fix.go"), "package main\n")
	git(t, a.Worktree, "add", ".")
	git(t, a.Worktree, "commit", "-q", "-m", "the fix")
	if err := s.Handoff("d3", "committed, ready for review"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close("d3"); err != nil {
		t.Fatalf("committed work could not be closed: %v", err)
	}
	out, err := exec.Command("git", "-C", repo, "branch", "--list", a.Branch).CombinedOutput()
	if err != nil || !strings.Contains(string(out), a.Branch) {
		t.Errorf("the branch did not outlive the worktree: %s", out)
	}
}

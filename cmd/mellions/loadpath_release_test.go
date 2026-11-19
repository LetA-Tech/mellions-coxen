package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func releaseCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	deploy := filepath.Join(root, "deploy")
	git := gitRunner(t)
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	git(origin, "init", "-q", "--bare", "-b", "main")
	git(root, "clone", "-q", origin, deploy)
	git(deploy, "config", "user.email", "t@example.com")
	git(deploy, "config", "user.name", "t")
	git(deploy, "commit", "-q", "--allow-empty", "-m", "released")
	git(deploy, "push", "-q", "-u", "origin", "main")
	git(deploy, "checkout", "-q", "-b", "dev")
	git(deploy, "commit", "-q", "--allow-empty", "-m", "merged fix")
	git(deploy, "push", "-q", "-u", "origin", "dev")
	git(deploy, "checkout", "-q", "main")
	return deploy
}

func TestDoctorReportsUninstalledWorkingBranchCommits(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prepare    func(*testing.T, string)
		wantState  string
		wantDetail string
	}{
		{"main current with upstream but missing dev", nil, "UNRELEASED", "1 commit"},
		{"release contains dev", func(t *testing.T, dir string) {
			git := gitRunner(t)
			git(dir, "merge", "-q", "--no-ff", "origin/dev", "-m", "release")
			git(dir, "push", "-q", "origin", "main")
		}, "present", "0 commits"},
		{"load path runs dev", func(t *testing.T, dir string) {
			gitRunner(t)(dir, "checkout", "-q", "dev")
		}, "present", "0 commits"},
		{"working branch was not fetched", func(t *testing.T, dir string) {
			gitRunner(t)(dir, "update-ref", "-d", "refs/remotes/origin/dev")
		}, "unknown", "origin/dev"},
		{"shallow history", func(t *testing.T, dir string) {
			git := gitRunner(t)
			writeUnder(t, filepath.Join(dir, ".git", "shallow"), strings.TrimSpace(git(dir, "rev-parse", "HEAD"))+"\n")
		}, "unknown", "complete history unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deploy := releaseCheckout(t)
			if tc.prepare != nil {
				tc.prepare(t, deploy)
			}
			home := fakeRuntime(t, "directory", deploy, filepath.Join(t.TempDir(), "copy"))
			writeUnder(t, filepath.Join(deploy, "config", "session-workflow.md"), "Working rules\n")
			t.Setenv("HOME", home)
			t.Setenv("MELLIONS_HOME", filepath.Join(home, "mellions"))
			config := filepath.Join(home, "config.json")
			writeUnder(t, config, `{}`)
			var err error
			out := captureStdout(t, func() {
				err = cmdDoctor(context.Background(), []string{"-config", config})
			})
			var release string
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "load path release ") {
					release = line
				}
			}
			if fields := strings.Fields(release); len(fields) < 4 || fields[3] != tc.wantState {
				t.Errorf("release row = %q, want state %s", release, tc.wantState)
			}
			if !strings.Contains(release, tc.wantDetail) {
				t.Errorf("release row = %q, want %q", release, tc.wantDetail)
			}
			if tc.wantState != "unknown" && !strings.Contains(release, "as of the last fetch") {
				t.Errorf("release row does not qualify its freshness: %q", release)
			}
			failedForRelease := err != nil && strings.Contains(err.Error(), "unreleased work: load path release")
			if failedForRelease != (tc.wantState == "UNRELEASED") {
				t.Errorf("doctor error = %v, want release failure = %v", err, tc.wantState == "UNRELEASED")
			}
		})
	}
}

func TestLoadPathReleaseDoesNotCertifyAFailedRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		dir  string
	}{
		{"no load path", context.Background(), ""},
		{"not a checkout", context.Background(), t.TempDir()},
		{"cancelled query", ctx, releaseCheckout(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, detail := loadPathReleaseLine(tc.ctx, tc.dir)
			if state != "unknown" || !strings.Contains(detail, "unknown") {
				t.Fatalf("failed read returned %s %q, want unknown", state, detail)
			}
		})
	}
}

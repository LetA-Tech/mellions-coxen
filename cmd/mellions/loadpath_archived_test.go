// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoSlugParsesTheRemotesThisEstateUses(t *testing.T) {
	for _, tc := range []struct{ remote, want string }{
		{"https://github.com/LetA-Tech/mellions-coxen.git", "LetA-Tech/mellions-coxen"},
		{"https://github.com/LetA-Tech/mellions-coxen", "LetA-Tech/mellions-coxen"},
		{"git@github.com:LetA-Tech/mcfo-fininsight.git", "LetA-Tech/mcfo-fininsight"},
		{"ssh://git@github.com/LetA-Tech/mcfo-findata.git", "LetA-Tech/mcfo-findata"},
		// Not GitHub, and not parseable. Must yield "", which the caller
		// reports as unknown — never as healthy.
		{"https://gitlab.com/x/y.git", ""},
		{"/some/local/path", ""},
		{"", ""},
	} {
		if got := repoSlug(tc.remote); got != tc.want {
			t.Errorf("repoSlug(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

// A remote that cannot be classified must report unknown. Reporting "live"
// would be the same substitution this check exists to remove: a reassuring word
// printed from a question nobody answered.
func TestUnclassifiableRemoteReportsUnknownNotLive(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "--quiet", "-b", "main")
	run(t, dir, "remote", "add", "origin", "https://gitlab.example/x/y.git")

	state, detail := loadPathOriginLine(context.Background(), dir)
	if state != "unknown" {
		t.Errorf("state = %q, want unknown", state)
	}
	if !strings.Contains(detail, "not evidence that the remote is still the project") {
		t.Errorf("detail = %q, want it to say what was not established", detail)
	}
}

func TestNoOriginRemoteReportsUnknown(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "--quiet", "-b", "main")

	state, detail := loadPathOriginLine(context.Background(), dir)
	if state != "unknown" {
		t.Errorf("state = %q, want unknown", state)
	}
	if !strings.Contains(detail, "no origin remote") {
		t.Errorf("detail = %q, want it to name the missing remote", detail)
	}
}

// The row exists because "0 behind origin/main" answers a different question.
// If these two ever collapse into one line the conflation is back.
func TestOriginRowIsSeparateFromTheCommitRow(t *testing.T) {
	src, err := os.ReadFile("doctor.go")
	if err != nil {
		t.Fatalf("read doctor.go: %v", err)
	}
	body := string(src)
	for _, want := range []string{`line("load path commit"`, `line("load path origin"`} {
		if !strings.Contains(body, want) {
			t.Errorf("doctor.go no longer emits %s — the distance row and the "+
				"is-this-still-the-project row must stay separate", want)
		}
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", filepath.Base(strings.Join(args, " ")), err, out)
	}
}

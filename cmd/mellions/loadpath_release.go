package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// The plugin's working branch is dev; release installs may track main instead.
// Counts use fetched refs without changing the checkout or contacting its remote.
func loadPathReleaseLine(ctx context.Context, dir string) (string, string) {
	const working = "origin/dev"
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	git := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1")
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	unknown := "cannot compare the load path with " + working + "; merged work not installed is unknown"
	if dir == "" {
		return "unknown", unknown + " (no load path)"
	}
	head, err := git("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "unknown", unknown + " (cannot read HEAD)"
	}
	dev, err := git("rev-parse", "--verify", "refs/remotes/"+working+"^{commit}")
	if err != nil {
		return "unknown", unknown + " (working branch not fetched or unreadable)"
	}
	shallow, err := git("rev-parse", "--is-shallow-repository")
	if err != nil || shallow != "false" {
		return "unknown", unknown + " (complete history unavailable)"
	}
	count, err := git("rev-list", "--count", head+".."+dev)
	if err != nil {
		return "unknown", unknown + " (cannot read commit ancestry)"
	}
	n, err := strconv.Atoi(count)
	if err != nil || n < 0 {
		return "unknown", unknown + " (invalid commit count)"
	}
	detail := fmt.Sprintf("%s holds %d commits absent from load path %s as of the last fetch", working, n, head)
	if n > 0 {
		return "UNRELEASED", detail + " — merged work is not installed; release it to the load path's branch or update to a release containing it"
	}
	return "present", detail
}

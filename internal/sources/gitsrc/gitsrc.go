// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package gitsrc reports what has changed in the repositories lately.
//
// It answers the question a returning engineer asks first and no tracker can:
// what moved while I was away. An issue filed three weeks ago describes a tree
// that has since had forty commits land on it, and knowing that is the
// difference between reading the code and trusting the issue.
package gitsrc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/checkout"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Runner executes git in a directory.
type Runner func(ctx context.Context, dir string, args ...string) (string, error)

func execRunner(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w", strings.Join(args, " "), filepath.Base(dir), err)
	}
	return string(out), nil
}

// Options configures the source.
type Options struct {
	// WorkRoot holds repository checkouts.
	WorkRoot string
	// Checkouts resolves a repository to its directory. Where set it is
	// authoritative: an installation may hold repositories under more than one
	// root, or under a directory not named after the repository.
	Checkouts checkout.Set
	// Repos limits which are read. Empty means every checkout under WorkRoot.
	Repos []string
	// Since bounds how far back to look. Zero uses DefaultSince.
	Since time.Duration
	// MaxPerRepo caps commits reported per repository. A busy week is one
	// signal about a repository, not four hundred.
	MaxPerRepo int
	// Run executes git; nil uses the real binary.
	Run Runner
}

// DefaultSince is how far back a survey looks by default.
const DefaultSince = 7 * 24 * time.Hour

// Source reports recent repository change.
type Source struct{ opts Options }

// New returns a configured source.
func New(o Options) *Source {
	if o.Since <= 0 {
		o.Since = DefaultSince
	}
	if o.MaxPerRepo <= 0 {
		o.MaxPerRepo = 8
	}
	if o.Run == nil {
		o.Run = execRunner
	}
	return &Source{opts: o}
}

// Name implements signal.Source.
func (s *Source) Name() string { return "git" }

// Collect summarises each repository's recent history.
//
// One signal per repository rather than one per commit: the engineer needs to
// know a repository moved and roughly how much, and the log itself is one
// command away when that matters.
func (s *Source) Collect(ctx context.Context, scope signal.Scope) ([]signal.Signal, error) {
	if s.opts.WorkRoot == "" && len(s.opts.Checkouts) == 0 {
		return nil, fmt.Errorf("gitsrc: no work root configured")
	}
	repos := s.opts.Repos
	if len(scope.Repos) > 0 {
		repos = scope.Repos
	}
	set := s.opts.Checkouts
	// Enumeration only where nobody said which repositories. A named list is
	// the caller's answer already, and scanning for it would turn a typo into
	// silence instead of the error it is.
	if len(repos) == 0 {
		if len(set) == 0 {
			var err error
			if set, err = checkout.Discover(s.opts.WorkRoot); err != nil {
				return nil, err
			}
		}
		repos = set.Names()
	}
	since := s.opts.Since
	if !scope.Since.IsZero() {
		since = time.Since(scope.Since)
	}
	cutoff := fmt.Sprintf("--since=%d.hours.ago", int(since.Hours()))

	var out []signal.Signal
	for _, repo := range repos {
		dir, ok := set.Dir(repo)
		if !ok {
			dir = filepath.Join(s.opts.WorkRoot, repo)
		}
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("gitsrc: no checkout of %s at %s", repo, dir)
		}
		branch, err := s.opts.Run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return nil, err
		}
		log, err := s.opts.Run(ctx, dir, "log", cutoff, "--no-merges",
			"--pretty=format:%h %ad %s", "--date=short", "-n", strconv.Itoa(s.opts.MaxPerRepo*4))
		if err != nil {
			return nil, err
		}
		lines := nonEmptyLines(log)
		if len(lines) == 0 {
			continue
		}
		shown := lines
		if len(shown) > s.opts.MaxPerRepo {
			shown = shown[:s.opts.MaxPerRepo]
		}
		detail := strings.Join(shown, "\n")
		if len(lines) > len(shown) {
			detail += fmt.Sprintf("\n… and %d more in the window", len(lines)-len(shown))
		}

		attrs := map[string]string{
			"commits": strconv.Itoa(len(lines)),
			"branch":  strings.TrimSpace(branch),
			"window":  fmt.Sprintf("%dd", int(since.Hours()/24)),
		}
		if dirty, err := s.opts.Run(ctx, dir, "status", "--porcelain"); err == nil {
			if n := len(nonEmptyLines(dirty)); n > 0 {
				// Uncommitted state in a shared checkout is worth surfacing:
				// it is somebody's unfinished work, possibly the engineer's own.
				attrs["uncommitted"] = strconv.Itoa(n)
			}
		}

		out = append(out, signal.Signal{
			Kind: signal.KindCommit, Source: "git",
			ID: repo, Title: fmt.Sprintf("%d commits in the last %s", len(lines), attrs["window"]),
			Repo: repo, Updated: time.Now(), Attrs: attrs, Detail: detail,
		})
	}
	return out, nil
}

func nonEmptyLines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

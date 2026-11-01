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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
	"github.com/LetA-Tech/mellions-coxen/internal/repobinding"
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
		name := repo
		if _, short, ok := strings.Cut(repo, "/"); ok {
			name = short
		}
		dir, ok := set.Dir(name)
		if !ok {
			dir = filepath.Join(s.opts.WorkRoot, name)
		}
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("gitsrc: no checkout of %s at %s", repo, dir)
		}
		out = append(out, s.gate(ctx, repo, dir))
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

// gate is one signal per repository naming the approval authorities and
// approval model its binding declares at the remote branch a lane would be cut
// from, as last fetched: the branches assign open tries, in its order, then the
// checkout's upstream. A binding that cannot be read is reported unknown rather
// than dropped, and neither an absent binding nor an absent model is worded as
// a delegation.
func (s *Source) gate(ctx context.Context, repo, dir string) signal.Signal {
	sig := signal.Signal{Kind: signal.KindGate, Source: "git", ID: "gate", Repo: repo,
		Attrs: map[string]string{}}
	unknown := func(why string) signal.Signal {
		sig.Attrs["model"] = "unknown"
		sig.Title = "unknown — " + why
		return sig
	}
	upstream := ""
	if out, err := s.opts.Run(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		upstream = strings.TrimSpace(out)
	}
	remote := "origin"
	upBranch := ""
	if r, b, ok := strings.Cut(upstream, "/"); ok && r != "" {
		remote, upBranch = r, b
	}
	var candidates []string
	for _, c := range append(assignment.WorkingBranchCandidates(dir), upBranch) {
		if c != "" && !slices.Contains(candidates, c) {
			candidates = append(candidates, c)
		}
	}
	ref := ""
	for _, c := range candidates {
		// for-each-ref answers an absent ref with empty output and exit 0, so
		// only a failure of git itself is an error, and that is unknown.
		out, err := s.opts.Run(ctx, dir, "for-each-ref", "--format=%(objectname)", "refs/remotes/"+remote+"/"+c)
		if err != nil {
			return unknown(fmt.Sprintf("resolving %s/%s failed: %v", remote, c, err))
		}
		if strings.TrimSpace(out) != "" {
			ref = remote + "/" + c
			break
		}
	}
	if ref == "" {
		return unknown(fmt.Sprintf("none of %s/%s is in the checkout, so its binding was not read",
			remote, strings.Join(candidates, ", "+remote+"/")))
	}
	sig.Attrs["ref"] = ref
	listed, err := s.opts.Run(ctx, dir, "ls-tree", "--name-only", ref, "--", repobinding.Path)
	if err != nil {
		return unknown(fmt.Sprintf("listing %s at %s failed: %v", repobinding.Path, ref, err))
	}
	if strings.TrimSpace(listed) == "" {
		sig.Attrs["model"] = "no binding"
		sig.Title = fmt.Sprintf("no binding — %s carries no %s, so it names no approval authority; that is not a delegation", ref, repobinding.Path)
		return sig
	}
	raw, err := s.opts.Run(ctx, dir, "show", ref+":"+repobinding.Path)
	if err != nil {
		return unknown(fmt.Sprintf("reading %s at %s failed: %v", repobinding.Path, ref, err))
	}
	b := repobinding.Parse([]byte(raw))
	if b.Authorities == "" {
		sig.Attrs["model"] = "no authorities"
		sig.Title = fmt.Sprintf("binding names no approval_authorities at %s; that is not a delegation", ref)
		return sig
	}
	sig.Attrs["authorities"] = b.Authorities
	sig.Attrs["model"] = b.Model
	if b.Model == repobinding.NotDeclared {
		sig.Title = fmt.Sprintf("approval_authorities %s, approval_model not declared at %s — its default is the proposal method's to say, not a delegation", b.Authorities, ref)
		return sig
	}
	sig.Title = fmt.Sprintf("approval_authorities %s, approval_model %s at %s", b.Authorities, b.Model, ref)
	return sig
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

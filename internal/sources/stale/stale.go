// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package stale finds recorded claims the current tree contradicts.
//
// An issue is written against the code as it stood that day. Later code can
// move while the issue's account remains unchanged, so the premise must be
// checked again before implementation.
//
// The check is the issue gate run backwards. The gate verifies a body against
// the tree BEFORE filing; this verifies an already-filed body against the tree
// NOW. Same comparison, opposite moment, and the same code performs both.
//
// A stale premise is evidence, never a verdict. The tree moving does not tell
// you the issue is resolved — the fix may be partial, the symptom may survive
// its stated cause, the citation may simply have drifted by a refactor. What it
// tells you is that the issue's own account of the code is no longer true, and
// that reading the current code is now mandatory before acting. Deciding what
// that means is the engineer's job.
package stale

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/issuegate"
	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Runner executes the tracker CLI. Replaced in tests.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// Options configures the scan.
type Options struct {
	// Owner is the GitHub org or user.
	Owner string
	// Repos limits the scan.
	Repos []string
	// Checkouts maps repository name to a local checkout path. A body cites
	// code; without a checkout there is nothing to compare it against.
	Checkouts map[string]string
	// Limit caps issues examined per repository.
	Limit int
	// MinAge skips issues younger than this. A body written this morning
	// describes this morning's tree, and re-checking it finds only the churn of
	// the day rather than a premise anyone should doubt.
	MinAge time.Duration
	// Run executes gh; nil uses the real CLI.
	Run Runner
}

// Source scans open work items for premises the tree contradicts.
type Source struct{ opts Options }

// New returns a configured source.
func New(o Options) *Source {
	if o.Limit <= 0 {
		o.Limit = 60
	}
	if o.MinAge <= 0 {
		o.MinAge = 7 * 24 * time.Hour
	}
	if o.Run == nil {
		o.Run = ghRun
	}
	return &Source{opts: o}
}

// Name implements signal.Source.
func (s *Source) Name() string { return "stale" }

// staleRules are the gate findings that mean the tree moved under the issue.
//
// The gate's other rules — a body with no citations, a repository named but
// never cited — are about how well the issue was written. They say nothing
// about whether it is still true, and reporting them here would bury the signal
// that matters under filing-quality complaints about old issues.
var staleRules = []string{
	issuegate.RuleQuoteMismatch,
}

type item struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// Collect scans each configured repository.
func (s *Source) Collect(ctx context.Context, scope signal.Scope) ([]signal.Signal, error) {
	repos := s.opts.Repos
	if len(scope.Repos) > 0 {
		repos = scope.Repos
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("stale: no repositories configured or scoped")
	}
	if len(s.opts.Checkouts) == 0 {
		return nil, fmt.Errorf("stale: no checkouts configured; a citation cannot be checked without the code")
	}

	now := time.Now()
	var out []signal.Signal
	// Per repository, and the failures are collected rather than fatal. One
	// repository that cannot be read is not a fact about the other twenty, and
	// aborting on it reported no stale premises for code that was examined —
	// which is the reading a scan must never produce. What was collected is
	// returned beside the error, and the survey says which repositories are
	// unaccounted for.
	var unreadable []string
	for _, repo := range repos {
		short := shortName(repo)
		if _, ok := s.opts.Checkouts[short]; !ok {
			unreadable = append(unreadable, short+": no checkout configured, so its citations cannot be checked")
			continue
		}
		items, err := s.list(ctx, s.full(repo))
		if err != nil {
			if strings.Contains(err.Error(), "has disabled issues") {
				continue // no issues is a fact, not a failure
			}
			unreadable = append(unreadable, short+": "+err.Error())
			continue
		}
		for _, it := range items {
			if now.Sub(it.CreatedAt) < s.opts.MinAge {
				continue
			}
			sig, ok := s.examine(short, it)
			if ok {
				out = append(out, sig)
			}
		}
	}
	if len(unreadable) > 0 {
		return out, fmt.Errorf("stale: %d of %d repositories could not be scanned — %s",
			len(unreadable), len(repos), strings.Join(unreadable, "; "))
	}
	return out, nil
}

// examine compares one body against the current tree.
//
// Only a citation whose file was actually located counts as evidence. A file
// this checkout cannot find proves nothing: the body may be citing a sibling
// repository. Unlocated citations are counted as unchecked rather than moved.
func (s *Source) examine(repo string, it item) (signal.Signal, bool) {
	cites := issuegate.Citations(it.Body, s.opts.Checkouts)
	if len(cites) == 0 {
		return signal.Signal{}, false
	}

	var moved []string
	unchecked, absentPath, inFile := 0, 0, 0
	for _, c := range cites {
		path, ok := issuegate.Locate(c, repo, s.opts.Checkouts)
		if !ok {
			// How the citation was written decides whether its absence means
			// anything. A bare basename is resolved by searching this checkout,
			// so not finding one is equally consistent with the file living in a
			// sibling repository. A path, by the gate's own convention, is a claim
			// about THIS repository, and a claim about this repository that this
			// repository does not satisfy is worth reading the code over.
			// Checkable separates a claim this repository does not satisfy from
			// one nobody actually made. An elided path — `internal/.../x.go` —
			// and a name that matches two files are both prose, and counting
			// them as moved reported stale premises about files nobody named.
			if strings.Contains(c.Path, "/") && issuegate.Checkable(c, repo, s.opts.Checkouts) {
				absentPath++
				moved = append(moved, fmt.Sprintf(
					"%s — no such path in this repository (deleted, moved, or a path in a dependency or "+
						"sibling repository that this scan cannot tell apart)", c))
			} else {
				unchecked++
			}
			continue
		}
		n, err := countLines(path)
		if err != nil {
			unchecked++
			continue
		}
		if c.Line > n {
			inFile++
			moved = append(moved, fmt.Sprintf("%s — the file now has %d lines", c, n))
		}
	}

	// Quote mismatches are sound on their own: the gate raises one only when it
	// located the file and the quoted text is not in it.
	for _, f := range issuegate.Check(it.Body, repo, s.opts.Checkouts) {
		if slices.Contains(staleRules, f.Rule) {
			inFile++
			moved = append(moved, f.Detail)
		}
	}

	if len(moved) == 0 {
		return signal.Signal{}, false
	}

	var detail strings.Builder
	detail.WriteString("the issue's own citations no longer match the tree:\n")
	for _, m := range moved {
		fmt.Fprintf(&detail, "  - %s\n", m)
	}
	if unchecked > 0 {
		fmt.Fprintf(&detail, "  (%d further citation(s) could not be checked here — no checkout holds them, "+
			"so they are unknown rather than moved)\n", unchecked)
	}
	detail.WriteString("read the current code before acting; this is evidence the premise moved, not proof the work is done")

	labels := make([]string, 0, len(it.Labels))
	for _, l := range it.Labels {
		labels = append(labels, l.Name)
	}

	return signal.Signal{
		Kind: signal.KindStalePremise, Source: "stale",
		ID: "#" + strconv.Itoa(it.Number), Title: it.Title,
		URL: it.URL, Repo: repo,
		Created: it.CreatedAt, Updated: it.UpdatedAt,
		Labels: labels,
		Attrs: map[string]string{
			"citations":             strconv.Itoa(len(cites)),
			"moved":                 strconv.Itoa(len(moved)),
			"moved_in_located_file": strconv.Itoa(inFile),
			"path_absent":           strconv.Itoa(absentPath),
			"unchecked":             strconv.Itoa(unchecked),
			"checked_at":            time.Now().UTC().Format(time.RFC3339),
			"checkout_path":         s.opts.Checkouts[repo],
		},
		Detail: detail.String(),
	}, true
}

func (s *Source) list(ctx context.Context, full string) ([]item, error) {
	raw, err := s.opts.Run(ctx, "issue", "list", "-R", full, "--state", "open",
		"--limit", strconv.Itoa(s.opts.Limit),
		"--json", "number,title,body,url,createdAt,updatedAt,labels")
	if err != nil {
		return nil, err
	}
	var items []item
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("stale: decode issues for %s: %w", full, err)
	}
	return items, nil
}

// countLines reports how many lines a file has, so a citation past the end can
// be told from one that still lands inside the file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

func (s *Source) full(repo string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	return s.opts.Owner + "/" + repo
}

func shortName(full string) string {
	if _, name, ok := strings.Cut(full, "/"); ok {
		return name
	}
	return full
}

func ghRun(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

// DiscoverCheckouts maps repository name to path for every git checkout
// directly under root, so an installation names its work root once instead of
// listing every repository twice.
func DiscoverCheckouts(root string) (map[string]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("stale: read work root %s: %w", root, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := root + string(os.PathSeparator) + e.Name()
		if fi, err := os.Stat(path + string(os.PathSeparator) + ".git"); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
			out[e.Name()] = path
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("stale: no git checkouts under %s", root)
	}
	return out, nil
}

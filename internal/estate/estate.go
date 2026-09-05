// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package estate reads one path out of every repository in the estate, at a
// named ref, and says per repository what it found.
//
// It exists because the cheap way to answer an estate-wide question — "what
// does every consumer pin?", "does anything still reference this column?" — is
// to glob the working trees, and a working tree is whatever branch someone last
// left it on. The read succeeds and returns a plausible wrong answer, which is
// the shape nothing catches: there is no error, and the absence of a result is
// itself the result.
//
// Two properties are the point, and both exist because a faster wrong path is
// the only reason anyone takes it:
//
//   - every repository produces a Result, including the ones that could not be
//     read. A repository with no checkout, no such ref or no such path is
//     reported as those things, never omitted — so a caller can tell "no
//     repository pins this" from "four repositories were never opened";
//   - the ref is always stated. Reading a working tree is possible but must be
//     asked for by name, so it appears in the output rather than being the
//     silent default.
package estate

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// Status is what happened when one repository was read.
type Status string

const (
	// StatusOK means the path was read at the ref.
	StatusOK Status = "ok"
	// StatusNoPath means the ref resolved and does not carry the path. This is
	// a fact about the repository, distinct from every status below, which are
	// facts about this host.
	StatusNoPath Status = "no-such-path"
	// StatusNoCheckout means the repository is configured and not on disk.
	StatusNoCheckout Status = "no-checkout"
	// StatusNoRef means the checkout exists and the ref does not resolve in it.
	StatusNoRef Status = "no-such-ref"
	// StatusFetchFailed means the ref was not refreshed, so whatever is read is
	// as stale as the last fetch. Reported rather than swallowed: it is the
	// difference between a measurement and a memory.
	StatusFetchFailed Status = "fetch-failed"
	// StatusError means git failed for a reason none of the above names.
	StatusError Status = "error"
)

// Measured reports whether this status carries content that answers the
// question. Every other status is a fact about the host, and a caller counting
// answers must not count them.
func (s Status) Measured() bool { return s == StatusOK || s == StatusNoPath }

// Result is one repository's answer.
type Result struct {
	Repo    string
	Dir     string
	Ref     string
	Commit  string
	Status  Status
	Detail  string
	Content string
	Matches []string
}

// Request is what to read and from where.
type Request struct {
	// Repos maps repository name to checkout directory. Every entry produces a
	// Result.
	Repos map[string]string
	// Path is the repository-relative file to read.
	Path string
	// Ref is the git ref to read at. Empty means DefaultRef per repository.
	Ref string
	// Worktree reads the working tree instead of a ref. It must be asked for,
	// and the Result says so.
	Worktree bool
	// Fetch refreshes the remote-tracking refs first. Without it a ref is only
	// as current as the last fetch, and the Result says which.
	Fetch bool
	// Grep keeps only matching lines. A repository whose file has no matching
	// line is StatusOK with no Matches — which is an answer, not an absence.
	Grep *regexp.Regexp
}

// DefaultRef is the ref a repository is read at when none is named: its
// remote's own default branch, falling back to the two names this estate uses.
//
// origin/HEAD is asked first because it is the remote's own answer. It is not
// always present — a clone made with --single-branch has none — so the
// fallbacks exist, and the ref that answered is reported either way.
func DefaultRef(ctx context.Context, dir string) string {
	if out, err := git(ctx, dir, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		if ref := strings.TrimPrefix(strings.TrimSpace(out), "refs/remotes/"); ref != "" {
			return ref
		}
	}
	for _, cand := range []string{"origin/dev", "origin/main", "origin/master"} {
		if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", cand+"^{commit}"); err == nil {
			return cand
		}
	}
	return ""
}

// Read answers req for every repository in it.
func Read(ctx context.Context, req Request) []Result {
	names := make([]string, 0, len(req.Repos))
	for n := range req.Repos {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Result, 0, len(names))
	for _, name := range names {
		out = append(out, readOne(ctx, req, name, req.Repos[name]))
	}
	return out
}

func readOne(ctx context.Context, req Request, name, dir string) Result {
	r := Result{Repo: name, Dir: dir}

	if dir == "" {
		r.Status, r.Detail = StatusNoCheckout, "no checkout on this host"
		return r
	}
	if _, err := git(ctx, dir, "rev-parse", "--git-dir"); err != nil {
		r.Status, r.Detail = StatusNoCheckout, "not a git checkout"
		return r
	}

	if req.Worktree {
		r.Ref = "WORKING TREE"
		body, err := readWorktree(ctx, dir, req.Path)
		if err != nil {
			r.Status, r.Detail = StatusNoPath, err.Error()
			return r
		}
		return finish(r, body, req.Grep)
	}

	if req.Fetch {
		if _, err := git(ctx, dir, "fetch", "--quiet", "origin"); err != nil {
			// Not fatal: a stale ref still answers, and saying the answer is
			// stale is worth more than refusing to give one.
			r.Status, r.Detail = StatusFetchFailed, "fetch failed; read at the last fetched ref"
		}
	}

	ref := req.Ref
	if ref == "" {
		ref = DefaultRef(ctx, dir)
	}
	if ref == "" {
		r.Status, r.Detail = StatusNoRef, "no origin/HEAD and none of origin/dev, origin/main, origin/master"
		return r
	}
	r.Ref = ref

	sha, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		r.Status, r.Detail = StatusNoRef, ref+" does not resolve"
		return r
	}
	r.Commit = strings.TrimSpace(sha)

	body, err := git(ctx, dir, "show", ref+":"+req.Path)
	if err != nil {
		r.Status, r.Detail = StatusNoPath, "no "+req.Path+" at "+ref
		return r
	}
	if r.Status == StatusFetchFailed {
		// Keep the fetch failure: it qualifies the content rather than
		// replacing it, and a caller that treats it as measured is wrong.
		r.Content = body
		r.Matches = grepLines(body, req.Grep)
		return r
	}
	return finish(r, body, req.Grep)
}

func finish(r Result, body string, re *regexp.Regexp) Result {
	r.Status = StatusOK
	r.Content = body
	r.Matches = grepLines(body, re)
	return r
}

func grepLines(body string, re *regexp.Regexp) []string {
	if re == nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(body, "\n") {
		if re.MatchString(l) {
			out = append(out, strings.TrimRight(l, "\r"))
		}
	}
	return out
}

func readWorktree(ctx context.Context, dir, path string) (string, error) {
	out, err := git(ctx, dir, "show", ":"+path)
	if err == nil {
		return out, nil
	}
	// Not in the index — an untracked or newly written file still answers for
	// the working tree, which is what was asked for.
	b, rerr := exec.CommandContext(ctx, "cat", dir+"/"+path).Output()
	if rerr != nil {
		return "", fmt.Errorf("no %s in the working tree", path)
	}
	return string(b), nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Summary counts the outcomes, so a caller can state what was measured rather
// than implying it from the rows that happened to print.
type Summary struct {
	Total, Measured, Unmeasured int
	ByStatus                    map[Status]int
}

// Summarize counts results by status.
func Summarize(rs []Result) Summary {
	s := Summary{Total: len(rs), ByStatus: map[Status]int{}}
	for _, r := range rs {
		s.ByStatus[r.Status]++
		if r.Status.Measured() {
			s.Measured++
		} else {
			s.Unmeasured++
		}
	}
	return s
}

// Line renders the one-line verdict a reader scans.
func (s Summary) Line() string {
	if s.Unmeasured == 0 {
		return fmt.Sprintf("%d repositories, all measured", s.Total)
	}
	var parts []string
	for _, st := range []Status{StatusNoCheckout, StatusNoRef, StatusFetchFailed, StatusError} {
		if n := s.ByStatus[st]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, st))
		}
	}
	return fmt.Sprintf("%d repositories, %d measured, %d NOT measured (%s) — a conclusion drawn here covers the %d",
		s.Total, s.Measured, s.Unmeasured, strings.Join(parts, ", "), s.Measured)
}

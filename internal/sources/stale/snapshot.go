// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package stale

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// Tree is the code one repository's citations are checked against.
type Tree struct {
	// Dir is the directory citations resolve in.
	Dir string
	// Ref and Commit name what Dir holds; both are empty for a tree read in
	// place, which is a claim about nothing but that directory.
	Ref, Commit string
	// Cleanup releases Dir; nil when nothing was made.
	Cleanup func()
}

// Reader produces the tree for the checkout at dir.
type Reader func(ctx context.Context, dir string) (Tree, error)

// InPlace reads the checkout's working tree as it stands. It is for a
// directory that is the subject itself, never for a shared checkout: a
// working tree is whatever commit somebody last left it on.
func InPlace(_ context.Context, dir string) (Tree, error) { return Tree{Dir: dir}, nil }

// AtWorkingBranch reads the commit the repository's working branch stands at
// on origin and extracts that commit's tree. A stale premise is a claim about
// the branch every lane is cut from, so it is computed there; a repository
// whose branch cannot be resolved and its commit obtained is an error, never a
// fallback to the working tree, which would answer for a commit no lane will
// see.
//
// The commit is taken from the remote itself, not from the checkout's
// remote-tracking ref, and fetched only when the checkout lacks it. Sessions on
// one host fetch the same shared checkout at once — every session start runs a
// survey — and git refuses all but one of concurrent updates to one ref; the
// object a refused fetch wanted is usually already present from the one that
// won.
func AtWorkingBranch(ctx context.Context, dir string) (Tree, error) {
	var tried []string
	for _, branch := range assignment.WorkingBranchCandidates(dir) {
		if branch == "" {
			continue
		}
		ref := "origin/" + branch
		out, err := gitOut(ctx, dir, "ls-remote", "--exit-code", "origin", "refs/heads/"+branch)
		commit, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\t")
		if err != nil || commit == "" {
			tried = append(tried, "ls-remote "+ref+": "+errText(err))
			continue
		}
		if !hasCommit(ctx, dir, commit) {
			_, ferr := gitOut(ctx, dir, "fetch", "--quiet", "origin", branch)
			if !hasCommit(ctx, dir, commit) {
				tried = append(tried, "fetch "+ref+" did not bring "+commit+": "+errText(ferr))
				continue
			}
		}
		extract, err := os.MkdirTemp("", "mellions-stale-")
		if err != nil {
			return Tree{}, err
		}
		if err := archive(ctx, dir, commit, extract); err != nil {
			os.RemoveAll(extract)
			return Tree{}, fmt.Errorf("extract %s at %s: %w", ref, commit, err)
		}
		return Tree{Dir: extract, Ref: ref, Commit: commit, Cleanup: func() { os.RemoveAll(extract) }}, nil
	}
	return Tree{}, fmt.Errorf("no working branch could be resolved at origin and read (%s)", strings.Join(tried, "; "))
}

func hasCommit(ctx context.Context, dir, commit string) bool {
	_, err := gitOut(ctx, dir, "cat-file", "-e", commit+"^{commit}")
	return err == nil
}

func errText(err error) string {
	if err == nil {
		return "no output"
	}
	return err.Error()
}

// archive writes commit's tracked files into dst.
func archive(ctx context.Context, repo, commit, dst string) error {
	ga := exec.CommandContext(ctx, "git", "-C", repo, "archive", "--format=tar", commit)
	tx := exec.CommandContext(ctx, "tar", "-xf", "-", "-C", dst)
	pipe, err := ga.StdoutPipe()
	if err != nil {
		return err
	}
	tx.Stdin = pipe
	var gaErr, txErr bytes.Buffer
	ga.Stderr, tx.Stderr = &gaErr, &txErr
	if err := tx.Start(); err != nil {
		return err
	}
	if err := ga.Run(); err != nil {
		tx.Wait()
		return fmt.Errorf("git archive: %v: %s", err, strings.TrimSpace(gaErr.String()))
	}
	if err := tx.Wait(); err != nil {
		return fmt.Errorf("tar: %v: %s", err, strings.TrimSpace(txErr.String()))
	}
	return nil
}

func gitOut(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			msg, _, _ = strings.Cut(msg, "\n")
			return out, fmt.Errorf("%s", msg)
		}
	}
	return out, err
}

// readAll produces a tree for every checkout, a few at a time. Failures are
// returned per repository: one unreachable remote is not a fact about the rest.
func readAll(ctx context.Context, read Reader, checkouts map[string]string) (map[string]Tree, map[string]error) {
	trees := map[string]Tree{}
	failed := map[string]error{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for repo, dir := range checkouts {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			t, err := read(ctx, dir)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed[repo] = err
				return
			}
			trees[repo] = t
		})
	}
	wg.Wait()
	return trees, failed
}

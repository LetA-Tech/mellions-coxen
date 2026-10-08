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

// AtWorkingBranch fetches the repository's working branch from origin and
// extracts that commit's tree. A stale premise is a claim about the branch
// every lane is cut from, so it is computed there; a repository whose branch
// cannot be fetched and resolved is an error, never a fallback to the working
// tree, which would answer for a commit no lane will see.
func AtWorkingBranch(ctx context.Context, dir string) (Tree, error) {
	var tried []string
	for _, branch := range assignment.WorkingBranchCandidates(dir) {
		if branch == "" {
			continue
		}
		ref := "origin/" + branch
		if _, err := gitOut(ctx, dir, "fetch", "--quiet", "origin", branch); err != nil {
			tried = append(tried, "fetch "+ref+": "+err.Error())
			continue
		}
		out, err := gitOut(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		commit := strings.TrimSpace(string(out))
		if err != nil || commit == "" {
			tried = append(tried, ref+" does not resolve after the fetch")
			continue
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
	return Tree{}, fmt.Errorf("no working branch could be fetched and resolved (%s)", strings.Join(tried, "; "))
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

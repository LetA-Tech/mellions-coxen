// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package checkout answers one question for every mechanism that needs code on
// disk: where is repository X.
//
// It existed four times before this, once per package that needed it, each
// joining a repository name onto one configured root. That shape carried two
// limits into everything built on it. An installation could only see
// repositories under a single directory — so an estate whose work is split
// across two parents had to choose which half the engineer was allowed to know
// about, and Mellions could not manage its own repository. And a repository
// whose tracker name differs from its directory name was unreachable, because
// one string had to serve as both.
//
// Roots are searched in order and an explicit path always wins, so both are
// configuration rather than a code change.
package checkout

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Set maps a repository name to the directory holding its checkout.
type Set map[string]string

// Dir returns the checkout for repo, and whether there is one.
func (s Set) Dir(repo string) (string, bool) {
	d, ok := s[repo]
	return d, ok
}

// Names returns the repositories, sorted, so output does not depend on map
// iteration order.
func (s Set) Names() []string {
	out := make([]string, 0, len(s))
	for n := range s {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// IsCheckout reports whether dir holds a git checkout.
//
// os.Stat rather than a directory entry's own type, because it follows
// symlinks: a checkout linked into a root is a checkout, and reporting it as
// absent would be the silent kind of wrong — indistinguishable from a
// repository nobody configured.
func IsCheckout(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	// .git is a directory in a clone and a file in a worktree.
	_, err = os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// Discover finds every checkout directly under each root.
//
// The first root holding a name wins, so ordering the roots is how an
// installation says which copy is authoritative when two exist.
func Discover(roots ...string) (Set, error) {
	out := Set{}
	var read int
	for _, root := range roots {
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		read++
		for _, e := range entries {
			name := e.Name()
			if _, taken := out[name]; taken {
				continue
			}
			dir := filepath.Join(root, name)
			if IsCheckout(dir) {
				out[name] = dir
			}
		}
	}
	if read == 0 {
		return nil, fmt.Errorf("checkout: none of the configured roots could be read: %v", roots)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("checkout: no git checkouts under %v", roots)
	}
	return out, nil
}

// Resolve locates a named list of repositories.
//
// A repository named in `at` is taken from there whatever the roots hold: that
// is the answer for a checkout whose directory is not named after the
// repository, which no amount of searching can infer. A name that resolves
// nowhere is left out rather than guessed at — a wrong directory is worse than
// a missing one, because work proceeds against it.
func Resolve(roots []string, at map[string]string, repos []string) Set {
	out := Set{}
	for _, r := range repos {
		if p := at[r]; p != "" {
			if IsCheckout(p) {
				out[r] = p
			}
			continue
		}
		for _, root := range roots {
			if root == "" {
				continue
			}
			dir := filepath.Join(root, r)
			if IsCheckout(dir) {
				out[r] = dir
				break
			}
		}
	}
	return out
}

// Missing names the repositories that resolved to nothing.
//
// Reported rather than swallowed: a repository in scope with no checkout is a
// gap in what the engineer can establish, and it should be visible before work
// is chosen against it rather than at the moment work is claimed.
func Missing(s Set, repos []string) []string {
	var out []string
	for _, r := range repos {
		if _, ok := s[r]; !ok {
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

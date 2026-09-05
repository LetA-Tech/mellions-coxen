// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// cmdSharedTreeCheck reads a PreToolUse payload on stdin and denies a Bash
// call that runs a tree-mutating git command inside a checkout this
// installation cuts lanes from. Everything else is silence.
func cmdSharedTreeCheck(args []string) error {
	fs := newFlagSet("shared-tree-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := fs.String("config", "", "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload := readPayload(os.Stdin)
	if len(payload) == 0 {
		guardUsage("shared-tree-check", "It denies a tree-mutating git command aimed at a "+
			"checkout this installation cuts lanes from, and names the read that answers "+
			"the same question.")
		return nil
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return nil
	}
	reason := sharedtree.Deny(payload, sharedEstate(cfg))
	if reason == "" {
		return nil
	}
	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = reason
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// sharedEstate is where this installation's work lives, as the guard needs it.
//
// The guarded set is `checkouts()` — the repositories in `repos`, resolved
// under the work roots. That is deliberately not every checkout the
// configuration can reach: a repository named in `checkouts` but absent from
// `repos` is one this installation works in and does not survey, Mellions' own
// source among them, and the partnership says the engineer commits there.
func sharedEstate(cfg *Config) sharedtree.Estate {
	set := cfg.checkouts()
	e := sharedtree.Estate{
		Lanes: []string{cfg.assignmentsRoot()},
		Home:  home(),
		Lane:  laneFinder(cfg),
		// Landing a Mellions fix is `git pull --ff-only` here. Read from the
		// registry rather than assumed, so an installation that loads from
		// somewhere else exempts that tree and not this one.
		LoadPath: pluginRoot(pluginreg.Read(home(), pluginreg.ID)),
	}
	for _, name := range set.Names() {
		dir, _ := set.Dir(name)
		e.Shared = append(e.Shared, sharedtree.Checkout{Repo: name, Dir: dir})
		// The same tree reached through a symlinked root is the same tree, and
		// a session that walked in by the link would otherwise be refused
		// nothing.
		if real, err := filepath.EvalSymlinks(dir); err == nil && real != dir {
			e.Shared = append(e.Shared, sharedtree.Checkout{Repo: name, Dir: real})
		}
	}
	return e
}

// laneFinder answers where THIS session's own worktree for a repository is, so
// a refusal names the tree the session should have been in.
//
// A lane is this session's when the assignment records the session, or when
// the session is standing in its worktree. Resolving by repository alone would
// answer with whichever lane happens to be open — on a host running several at
// once, another session's tree, which is the one thing this refusal must never
// send anybody into. An assignment whose worktree is gone answers nothing
// rather than a path that is not there.
func laneFinder(cfg *Config) func(repo, session, cwd string) string {
	return func(repo, session, cwd string) string {
		store, err := assignment.NewStore(cfg.assignmentsRoot())
		if err != nil {
			return ""
		}
		open, err := store.List(false)
		if err != nil {
			return ""
		}
		for _, a := range open {
			if a.Repo != repo || a.Worktree == "" || !mine(a, session, cwd) {
				continue
			}
			if a.State != assignment.StateActive && a.State != assignment.StateBlocked {
				continue
			}
			if fi, err := os.Stat(a.Worktree); err == nil && fi.IsDir() {
				return a.Worktree
			}
		}
		return ""
	}
}

// mine reports that an assignment is this session's: it recorded the session,
// or the session is standing in its worktree.
func mine(a *assignment.Assignment, session, cwd string) bool {
	if session != "" {
		for _, s := range a.Sessions {
			if s.ID == session {
				return true
			}
		}
	}
	return cwd != "" && a.Worktree != "" && inside(cwd, a.Worktree)
}

// inside reports whether path is root or within it, on a separator boundary.
func inside(path, root string) bool {
	path, root = filepath.Clean(path), filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

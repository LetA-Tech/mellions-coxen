// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
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
	d.Output.Event = preToolUseEvent
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
		Dirty:    treeIsDirty,
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

// treeIsDirty reports that the working tree at dir has uncommitted changes.
//
// Only a clear yes counts. A git that will not run, a directory that is not a
// repository, a non-zero exit — each is "cannot tell", which answers no and
// lets the deployment exemption stand, because a guess in the other direction
// blocks the only sanctioned way to install a fix.
//
// Tracked changes only. Autostash does not pass `--include-untracked`, so an
// untracked file is never stashed and never reapplied: measured, a tree whose
// only dirt is untracked fast-forwards with no `Created autostash` line and an
// empty stash list, and the file is left exactly as it was. A file the
// fast-forward would add, git refuses over loudly: exit 1, content preserved.
// Counting untracked files would instead refuse the deployment over a stray
// build artefact, which is the defect this whole exemption was added to fix.
//
// One untracked case IS destroyed silently and this probe does not see it: an
// IGNORED file that the incoming commit adds with `git add -f`. Measured, that
// fast-forwards at exit 0 and overwrites the local content with no stash and no
// warning. `--porcelain` never lists ignored files — that takes `--ignored` —
// so it is invisible with or without `--untracked-files=no`, and the flag is
// not what leaves it open. Closing it means deciding whether an ignored file in
// this tree is work at all, which is a wider question than this exemption.
// Named here rather than left as a gap the comment implies is closed.
//
// The git environment is stripped rather than inherited. `GIT_DIR`,
// `GIT_WORK_TREE` and `GIT_INDEX_FILE` outrank `-C`, so a hook process holding
// them reports on some other repository — exit 0, empty output, no error to
// notice — and a dirty load path reads clean.
func treeIsDirty(dir string) bool {
	if dir == "" {
		return false
	}
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain", "--untracked-files=no")
	cmd.Env = append(withoutGitEnv(os.Environ()), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// withoutGitEnv drops the variables that would make `git -C <dir>` answer for a
// different repository than dir.
func withoutGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "GIT_DIR="),
			strings.HasPrefix(kv, "GIT_WORK_TREE="),
			strings.HasPrefix(kv, "GIT_INDEX_FILE="),
			strings.HasPrefix(kv, "GIT_COMMON_DIR="),
			strings.HasPrefix(kv, "GIT_OBJECT_DIRECTORY="):
			continue
		}
		out = append(out, kv)
	}
	return out
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

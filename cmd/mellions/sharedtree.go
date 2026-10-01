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
	"github.com/LetA-Tech/mellions-coxen/internal/tmpglob"
)

// cmdSharedTreeCheck reads a PreToolUse payload on stdin and denies a Bash
// call that runs a tree-mutating git command inside a checkout this
// installation cuts lanes from, or deletes a glob over a temporary root every
// session shares, and a file-writing tool call by a session holding an
// assignment into such a checkout or the load path. Everything else is silence.
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
			"the same question; an rm that globs directly under a temporary root every "+
			"session shares (/tmp, /var/tmp, /dev/shm, $TMPDIR); and an Edit, Write, "+
			"MultiEdit or NotebookEdit by a session holding an assignment into such a "+
			"checkout or the load path, naming the same file in its lane.")
		return nil
	}
	reason := tmpglobDeny(payload)
	if reason == "" {
		cfg, err := loadConfig(*cfgPath)
		if err != nil {
			return nil
		}
		reason = sharedtree.Deny(payload, sharedEstate(cfg))
	}
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

// tmpglobDeny returns the reason to refuse a Bash payload that deletes a glob
// over a temporary root every session shares, or "".
func tmpglobDeny(payload []byte) string {
	var ev struct {
		ToolName string `json:"tool_name"`
		Cwd      string `json:"cwd"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" {
		return ""
	}
	if op := tmpglob.Find(ev.Input.Command, ev.Cwd); op != "" {
		return tmpglob.Reason(op)
	}
	return ""
}

// sharedEstate is where this installation's work lives, as the guard needs it.
//
// The guarded set is `checkouts()` — the repositories in `repos`, resolved
// under the work roots. That is deliberately not every checkout the
// configuration can reach: a repository named in `checkouts` but absent from
// `repos` is one this installation works in and does not survey, and a Bash
// git write there is not refused. The load path is the exception for file
// tools, which sharedtree adds to the set itself.
func sharedEstate(cfg *Config) sharedtree.Estate {
	set := cfg.checkouts()
	reg := pluginreg.Read(home(), pluginreg.ID)
	e := sharedtree.Estate{
		Lanes: []string{cfg.assignmentsRoot()},
		Home:  home(),
		Lane:  laneFinder(cfg),
		// Landing a Mellions fix is `git pull --ff-only` here. Read from the
		// registry rather than assumed, so an installation that loads from
		// somewhere else exempts that tree and not this one.
		LoadPath:  pluginRoot(reg),
		Dirty:     treeIsDirty,
		OtherTree: inOtherTree,
		Assigned:  assignedFinder(cfg),
		Ignored:   gitIgnores,
	}
	// The runtime's plugin root and the registry's load path can name
	// different trees — a copy the runtime was handed, the checkout the
	// registry reads in place — and which one a hook process sees is the
	// runtime's choice, so file tools are refused in both, and in what either
	// resolves to.
	for _, dir := range []string{e.LoadPath, reg.LoadPath} {
		if dir == "" {
			continue
		}
		e.LoadAliases = append(e.LoadAliases, dir)
		if real, err := filepath.EvalSymlinks(dir); err == nil && real != dir {
			e.LoadAliases = append(e.LoadAliases, real)
		}
	}
	if reg.LoadPath != "" {
		e.LoadRepo = filepath.Base(filepath.Clean(reg.LoadPath))
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

// gitIgnores reports that git ignores path in checkout's repository; sharedtree
// asks it only of the memory plugin's state directory. A path
// that does not exist yet is still answered, by the ignore rules alone; a git
// that will not answer is "cannot tell", which answers false and leaves the
// write refused.
func gitIgnores(path, checkout string) bool {
	cmd := exec.Command("git", "-C", checkout, "check-ignore", "-q", "--", path)
	cmd.Env = append(withoutGitEnv(os.Environ()), "GIT_OPTIONAL_LOCKS=0")
	return cmd.Run() == nil
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

// inOtherTree reports that dir is in a linked worktree of the checkout's own
// repository — same object store, its own working tree AND its own git
// directory, so its own index — such as one a repository requires under
// `.claude/worktrees/`.
//
// Git answers rather than a search for `.git`, because which tree owns a path
// is git's discovery rule and a reimplementation drifts from it. dir may not
// exist yet — the target of a `cd` into a new directory — so the question is
// asked of its nearest existing ancestor.
//
// Linked worktrees only. A submodule or an unrelated clone under the checkout
// is a different repository whose files are still part of the checkout's
// directory, and nothing establishes that it is a lane; it stays refused. So
// does a `.git` file that points back at the checkout's own git directory, or
// at a lane's that git registered somewhere else: git gives either a top level
// of its own, but a write through it reaches the checkout's index or files.
// Either side that git cannot resolve is "cannot tell", which answers false
// and leaves the checkout protected.
func inOtherTree(dir, checkout string) bool {
	for dir != "" {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
	if !gitEntryBelow(dir, checkout) {
		return false
	}
	mine, theirs := gitTree(dir), gitTree(checkout)
	if mine.top == "" || theirs.top == "" {
		return false
	}
	return mine.top != theirs.top && mine.gitDir != theirs.gitDir &&
		mine.common == theirs.common && registeredAt(mine.gitDir, mine.top)
}

// registeredAt reports that git registered the worktree whose git directory is
// gitDir at top: its `gitdir` file names top's `.git`. Without that binding a
// stray `.git` file anywhere in the checkout can borrow a lane's git directory,
// and a write through it lands in the checkout's files.
func registeredAt(gitDir, top string) bool {
	raw, err := os.ReadFile(filepath.Join(gitDir, "gitdir"))
	if err != nil {
		return false
	}
	back := strings.TrimSpace(string(raw))
	if back == "" {
		return false
	}
	if !filepath.IsAbs(back) {
		back = filepath.Join(gitDir, back)
	}
	return realPath(filepath.Dir(back)) == top
}

// gitEntryBelow reports that some directory from dir up to, but not including,
// checkout holds a `.git` entry. Git cannot resolve dir into a tree other than
// the checkout's without one, so its absence answers without running git — the
// common case, since every command run inside a shared checkout is asked.
func gitEntryBelow(dir, checkout string) bool {
	checkout = filepath.Clean(checkout)
	for d := filepath.Clean(dir); d != checkout; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
	return false
}

type gitPaths struct{ top, gitDir, common string }

// gitTree is where git resolves dir to: the working tree's root, its git
// directory and the repository's common directory, symlinks resolved, or the
// zero value where git does not answer.
func gitTree(dir string) gitPaths {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute",
		"--show-toplevel", "--git-dir", "--git-common-dir")
	cmd.Env = append(withoutGitEnv(os.Environ()), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		return gitPaths{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 || lines[0] == "" || lines[1] == "" || lines[2] == "" {
		return gitPaths{}
	}
	return gitPaths{realPath(lines[0]), realPath(lines[1]), realPath(lines[2])}
}

func realPath(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return filepath.Clean(p)
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

// assignedFinder answers whether THIS session holds an open (active or
// blocked) assignment in any repository, by mine(): the assignment records the
// session, or the session stands in its worktree. A store that cannot be read
// answers no, which leaves a file write allowed.
func assignedFinder(cfg *Config) func(session, cwd string) bool {
	return func(session, cwd string) bool {
		store, err := assignment.NewStore(cfg.assignmentsRoot())
		if err != nil {
			return false
		}
		open, err := store.List(false)
		if err != nil {
			return false
		}
		for _, a := range open {
			if a.State != assignment.StateActive && a.State != assignment.StateBlocked {
				continue
			}
			if mine(a, session, cwd) {
				return true
			}
		}
		return false
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

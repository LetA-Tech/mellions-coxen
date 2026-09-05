// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package sharedtree decides whether a Bash tool call runs a tree-mutating git
// command inside a checkout that is not the session's own lane.
//
// Every lane on a host is a worktree cut from one long-lived checkout per
// repository, and that checkout is nobody's lane. Its working tree carries
// whatever the owner or another session has not committed. A session reading
// how the code looked at an older commit reaches for the command that reads
// like the answer — `git checkout <rev> -- .` — and that command replaces the
// tree and the index in place: uncommitted work can disappear without a reflog
// entry or stash.
//
// The reads that answer the same question mutate nothing: `git show
// <rev>:<path>`, `git archive <rev> | tar -x` into a temporary directory, or
// the lane's own worktree already standing at its own commit. So the decision
// is not a judgement about intent — it is which tree the command writes to,
// and that is a parse: where the command line leaves the working directory,
// which invocations are git, which directory each one is aimed at, and whether
// its verb writes the working tree, the index or the stash. A search over the
// raw string answers none of those, so all four are parsed here.
//
// A lane worktree is refused nothing, and neither is a checkout this
// installation names explicitly rather than surveying — Mellions' own source
// among them, which the partnership says the engineer works in.
package sharedtree

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// Estate is what this installation knows about where its work lives.
type Estate struct {
	// Shared is every checkout this host cuts lanes from. Writing to one of
	// these trees is what this package refuses. A repository may appear more
	// than once where its checkout is reachable by more than one path.
	Shared []Checkout
	// Lanes are directories whose contents are worktrees a session owns. A
	// path under one of these is never refused, whatever else it is under.
	Lanes []string
	// Home is the user's home directory, so a `cd ~/...` resolves. Empty
	// leaves a tilde unexpanded, which reaches no shared checkout and so
	// refuses nothing.
	Home string
	// LoadPath is the checkout the runtime reads this installation's hooks,
	// Skills, commands and agent from, or "" where it is not known.
	//
	// It is exempt from ONE verb and one form of it: `git pull --ff-only`.
	// That is the deployment step for Mellions itself — merged is not landed,
	// and nothing reaches a session until this tree moves — so refusing it
	// leaves the guard blocking the only sanctioned way to install a fix,
	// including a fix to this guard. The exemption is safe because git itself
	// refuses a fast-forward that would overwrite local modifications, which
	// is the loss this package exists to prevent; a pull that would merge, or
	// a dirty tree, still fails, and fails in git rather than silently.
	LoadPath string
	// Lane answers where THIS session's own worktree for a repository is, or
	// "" where it has none. Nil is the same as none.
	//
	// Whose lane it is has to be part of the question. A refusal that resolves
	// the lane by repository alone sends a session into whichever worktree is
	// open for that repository, which on a host running several lanes at once
	// is somebody else's tree — the refusal would then name, as the place to
	// work, the exact kind of tree it exists to keep sessions out of.
	Lane func(repo, session, cwd string) string
}

// Deny returns the reason to refuse a PreToolUse payload, or "" to stay
// silent. Anything it cannot read is silence: a deny on a guess costs a
// session a command it was entitled to run, in a tree this package cannot see.
func Deny(payload []byte, e Estate) string {
	var ev struct {
		ToolName string `json:"tool_name"`
		Cwd      string `json:"cwd"`
		Session  string `json:"session_id"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" {
		return ""
	}
	f := Find(ev.Input.Command, ev.Cwd, e)
	if f == nil {
		return ""
	}
	return f.Reason(e, ev.Session, ev.Cwd)
}

// Checkout is one long-lived tree lanes are cut from.
type Checkout struct {
	Repo, Dir string
}

// Write is one git invocation that writes a tree the session does not own.
type Write struct {
	// Verb is the git subcommand, as written.
	Verb string
	// Dir is the directory the invocation runs in, absolute.
	Dir string
	// Repo and Checkout are the shared checkout Dir is inside.
	Repo, Checkout string
	// Instead is the read that answers the same question without writing.
	Instead string
}

// Find returns the first tree-mutating git invocation in command aimed at a
// shared checkout, or nil.
//
// The command line is walked left to right because a `cd` decides where every
// later invocation runs; neither token alone identifies the target tree.
func Find(command, cwd string, e Estate) *Write {
	dir := abs(cwd, cwd, e.Home)
	for _, c := range shellsplit.Split(command) {
		words := unwrap(c.Words)
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "cd", "pushd":
			if len(words) > 1 && !strings.HasPrefix(words[1], "-") {
				dir = abs(words[1], dir, e.Home)
			}
			continue
		case "git":
		default:
			continue
		}
		at, args := target(words[1:], dir, e.Home)
		verb, rest := subcommand(args)
		if verb == "" {
			continue
		}
		instead, ok := writes(verb, rest)
		if !ok {
			continue
		}
		repo, checkout, ok := shared(at, e)
		if !ok {
			continue
		}
		if deploysMellions(verb, rest, at, e) {
			continue
		}
		return &Write{Verb: verb, Dir: at, Repo: repo, Checkout: checkout, Instead: instead}
	}
	return nil
}

// Reason is what the session is told, and it has to answer three things at
// once: which tree this writes to and why that tree is not its own, what it
// costs when the answer is wrong, and the read that gets the same information
// without writing. A refusal that only forbids leaves the session to invent a
// way around it.
func (w *Write) Reason(e Estate, session, cwd string) string {
	var b strings.Builder
	b.WriteString("`git " + w.Verb + "` writes the working tree at " + w.Checkout +
		", which is the " + w.Repo + " checkout every lane on this host is cut from, not your worktree.\n\n" +
		"It carries whatever the owner or another session has not committed, and this command " +
		"overwrites that in place: no reflog entry, no stash, and nothing that reports what was there.\n\n")
	b.WriteString("Read it without writing to it:\n" + indent(w.Instead) + "\n\n")
	if lane := laneFor(e, w.Repo, session, cwd); lane != "" {
		b.WriteString("Your lane for " + w.Repo + " is " + lane + ", and it is a worktree of your own.\n")
	} else {
		b.WriteString("To change " + w.Repo + ", work in a lane of your own:\n" +
			"    mellions assign open -id <id> -repo " + w.Repo + " -objective \"...\" -because \"...\"\n")
	}
	// Repairing a shared checkout is a write to it like any other, and the
	// same refusal covers it. That is deliberate: the repair discards exactly
	// the uncommitted state this guard exists to protect, so it is a decision
	// to raise rather than a command to reach for — and a session that is told
	// only "no" will look for a way round instead of saying so.
	b.WriteString("\nIf this tree is already wrong and repairing it is the point, that is a write too, " +
		"and it discards whatever is uncommitted there. Say so and leave the command rather than " +
		"running it:\n" +
		"    git -C " + w.Checkout + " stash create        # holds the current index and tree in an object\n" +
		"    git -C " + w.Checkout + " restore --source=HEAD --staged --worktree -- .\n")
	b.WriteString("\nmellions-territory carries the rule this enforces: never delete, move or revert " +
		"what another session may hold.")
	return b.String()
}

func laneFor(e Estate, repo, session, cwd string) string {
	if e.Lane == nil {
		return ""
	}
	return e.Lane(repo, session, cwd)
}

// writes reports whether a git subcommand writes the working tree, the index
// or the stash, and names the read that answers the same question without
// doing so.
//
// The table is the decision. A verb absent from it is allowed, so every entry
// is one somebody has to have thought about; the flags beside a few of them
// are the forms of that verb that only report — `git clean -n` and `git apply
// --check` are how a session finds out what would happen, and refusing those
// would push it towards the form that does happen.
func writes(verb string, args []string) (string, bool) {
	m, ok := mutating[verb]
	if !ok {
		return "", false
	}
	if m.dryRun != nil && m.dryRun(args) {
		return "", false
	}
	return m.instead, true
}

type verb struct {
	instead string
	// dryRun reports that this invocation only says what it would do.
	dryRun func(args []string) bool
}

const (
	readACommit = "git show <rev>:<path>              # one file, to stdout\n" +
		"git archive <rev> | tar -x -C \"$(mktemp -d)\"   # the whole tree, elsewhere\n" +
		"git diff <rev> -- <path>           # what differs, without changing anything"
	readTheState = "git status --porcelain\ngit diff\ngit log --oneline -5"
	inALane      = "git worktree list                  # the trees that are already yours"
)

var mutating = map[string]verb{
	"checkout":    {instead: readACommit},
	"switch":      {instead: readACommit},
	"restore":     {instead: readACommit},
	"reset":       {instead: readTheState},
	"revert":      {instead: readACommit},
	"merge":       {instead: "git log --oneline HEAD..<rev>\ngit diff HEAD...<rev>"},
	"rebase":      {instead: "git log --oneline <upstream>..HEAD"},
	"cherry-pick": {instead: "git show <rev>"},
	"am":          {instead: "git apply --check <patch>"},
	"pull":        {instead: "git fetch                          # updates the remote refs, writes no tree"},
	"stash":       {instead: "git stash create                   # writes a commit object, leaves tree and stash alone\ngit diff > \"$(mktemp).patch\"       # keeps the change outside the repository", dryRun: reporting("list", "show", "create")},
	"clean":       {instead: readTheState, dryRun: flagged("-n", "--dry-run")},
	"apply":       {instead: "git apply --check <patch>", dryRun: flagged("--check", "--stat", "--summary", "--numstat")},
	"add":         {instead: inALane},
	"commit":      {instead: inALane},
	"rm":          {instead: inALane, dryRun: flagged("-n", "--dry-run")},
	"mv":          {instead: inALane, dryRun: flagged("-n", "--dry-run")},
	"sparse-checkout": {instead: readACommit,
		dryRun: reporting("list")},
}

// reporting matches the subcommands of a verb that only read.
func reporting(names ...string) func([]string) bool {
	return func(args []string) bool {
		for _, a := range args {
			if strings.HasPrefix(a, "-") {
				continue
			}
			for _, n := range names {
				if a == n {
					return true
				}
			}
			return false
		}
		// `git stash` with no subcommand is `git stash push`.
		return false
	}
}

// flagged matches the flags that turn a verb into a report of what it would do.
func flagged(names ...string) func([]string) bool {
	return func(args []string) bool {
		for _, a := range args {
			for _, n := range names {
				if a == n || (strings.HasPrefix(n, "--") && strings.HasPrefix(a, n+"=")) {
					return true
				}
			}
			// A short flag cluster: -nd is -n and -d.
			if len(a) > 1 && a[0] == '-' && !strings.HasPrefix(a, "--") {
				for _, n := range names {
					if len(n) == 2 && n[0] == '-' && strings.ContainsRune(a[1:], rune(n[1])) {
						return true
					}
				}
			}
		}
		return false
	}
}

// target reads the global options in front of a git subcommand and returns the
// directory the invocation runs in. `-C` is the one that moves it, and it
// accumulates: git applies each relative to the last.
func target(args []string, dir, home string) (string, []string) {
	at := dir
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-C" && i+1 < len(args):
			at = abs(args[i+1], at, home)
			i++
		case strings.HasPrefix(a, "-C") && len(a) > 2:
			at = abs(a[2:], at, home)
		case a == "-c" && i+1 < len(args):
			i++
		case a == "--git-dir" || a == "--work-tree" || a == "--namespace" || a == "--exec-path":
			if i+1 < len(args) {
				i++
			}
		case strings.HasPrefix(a, "-"):
			// Another global option, or its glued value.
		default:
			return at, args[i:]
		}
	}
	return at, nil
}

// subcommand separates the verb from its arguments.
func subcommand(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

// shared reports the checkout dir is inside, if it is one this installation
// cuts lanes from. A lane worktree wins over every checkout: a session's own
// tree stays its own even where the configuration puts it under a work root.
//
// The longest matching checkout wins, so a checkout nested inside another
// answers for its own paths rather than its parent's.
func shared(dir string, e Estate) (string, string, bool) {
	for _, lane := range e.Lanes {
		if under(dir, lane) {
			return "", "", false
		}
	}
	var repo, at string
	for _, c := range e.Shared {
		if c.Dir != "" && under(dir, c.Dir) && len(c.Dir) > len(at) {
			repo, at = c.Repo, c.Dir
		}
	}
	return repo, at, at != ""
}

// under reports whether path is root or inside it. Both are cleaned first, so
// a trailing slash or a `..` does not decide it, and the boundary is a
// separator, so /home/you/workspace/data-service-notes is not inside
// /home/you/workspace/data-service.
func under(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	path, root = filepath.Clean(path), filepath.Clean(root)
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// abs resolves a path the command line wrote against the directory it was
// written in, expanding a leading tilde. A path this cannot resolve stays as
// it is and matches no checkout, which is silence rather than a wrong deny.
func abs(path, dir, home string) string {
	switch {
	case path == "":
		return dir
	case path == "~" && home != "":
		return home
	case strings.HasPrefix(path, "~/") && home != "":
		path = filepath.Join(home, path[2:])
	case strings.HasPrefix(path, "$HOME/") && home != "":
		path = filepath.Join(home, path[len("$HOME/"):])
	}
	if strings.ContainsAny(path, "$`*?") {
		// An expansion this cannot resolve: the directory becomes unknown, and
		// an unknown directory is under nothing, so nothing is refused on it.
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if dir == "" {
		return path
	}
	return filepath.Clean(filepath.Join(dir, path))
}

// unwrap drops the grouping punctuation the lexer keeps as part of a word, so
// `( cd X && git reset --hard )` reads as the two commands it is.
func unwrap(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, "(){}")
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// Reach returns the shared checkout a command line steps into without the
// session standing there, or nil.
//
// A session can reach a shared checkout through `cd` without ever reporting
// that checkout as its cwd, so the command line itself determines the target.
func Reach(command, cwd string, e Estate) *Checkout {
	here := abs(cwd, cwd, e.Home)
	if _, at, ok := shared(here, e); ok && at != "" {
		// Standing in it is a different fact, and something else says it.
		return nil
	}
	dir := here
	for _, c := range shellsplit.Split(command) {
		words := unwrap(c.Words)
		if len(words) == 0 {
			continue
		}
		at := dir
		switch words[0] {
		case "cd", "pushd":
			if len(words) > 1 && !strings.HasPrefix(words[1], "-") {
				dir = abs(words[1], dir, e.Home)
				at = dir
			}
		case "git":
			at, _ = target(words[1:], dir, e.Home)
		default:
			continue
		}
		if repo, root, ok := shared(at, e); ok {
			return &Checkout{Repo: repo, Dir: root}
		}
	}
	return nil
}

// deploysMellions reports the one write this package allows into a shared
// checkout: a fast-forward-only pull of the load path.
//
// Scoped to that tree and that flag deliberately. Any other shared checkout,
// and any pull that could merge, is refused as before — the exemption is for
// the deployment step, not for pulling in general, and a session that wants a
// different tree updated still has to say so.
func deploysMellions(verb string, args []string, at string, e Estate) bool {
	if verb != "pull" || e.LoadPath == "" || at != e.LoadPath {
		return false
	}
	for _, a := range args {
		if a == "--ff-only" {
			return true
		}
	}
	return false
}

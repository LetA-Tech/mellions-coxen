// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// presences is where sessions register.
func (c *Config) presences() presence.Store {
	return presence.Store{Root: filepath.Join(c.reportRoot(), "sessions")}
}

// cmdHere registers the session this runs inside. Called by the session-start
// hook; harmless from a terminal, where there is no session to register.
func cmdHere(args []string) error {
	fs := newFlagSet("here", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cfgPath := fs.String("config", "", "config file")
	dir := fs.String("C", "", "the working tree; default the current directory")
	pid := fs.Int("pid", 0, "the runtime process; default what the runtime exports")
	if err := fs.Parse(args); err != nil {
		return err
	}
	sess, cwd := hookContext(os.Stdin)
	here := firstNonEmpty(*dir, cwd)
	if here == "" {
		here, _ = os.Getwd()
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return nil
	}
	runtime, id := presence.Here()
	if id == "" {
		id = sess
		runtime = "claude"
	}
	if id == "" {
		return nil
	}
	if *pid == 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("CLAUDE_PID"))); err == nil {
			*pid = v
		}
	}
	if *pid == 0 {
		*pid = os.Getppid()
	}
	now := time.Now().UTC()
	rec := presence.Session{
		ID: id, Runtime: runtime, PID: *pid, Cwd: here, Seen: now,
		Repo:   repoOf(here),
		Branch: strings.TrimSpace(gitOut(here, "rev-parse", "--abbrev-ref", "HEAD")),
	}
	if st, err := assignment.NewStore(cfg.assignmentsRoot()); err == nil {
		if open, err := st.List(false); err == nil {
			for _, a := range open {
				if a.Worktree != "" && sameTree(a.Worktree, here) {
					rec.Assignment = a.ID
				}
			}
		}
	}
	store := cfg.presences()
	if err := store.Register(rec); err != nil {
		return nil
	}
	store.Prune(now, 7*24*time.Hour)
	return nil
}

// cmdWho says who else is working here.
func cmdWho(args []string) error {
	fs := newFlagSet("who", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	dir := fs.String("C", "", "the working tree to ask about; default the current directory")
	all := fs.Bool("all", false, "every live session on this machine, not only this repository")
	if err := fs.Parse(args); err != nil {
		return err
	}
	here := *dir
	if here == "" {
		here, _ = os.Getwd()
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	_, me := presence.Here()
	mine := presence.SelfPID()
	live := cfg.presences().Live()
	here, repo := workingIn(live, me, mine, here)
	fmt.Printf("# Who is working here — %s\n\n", time.Now().UTC().Format("15:04 UTC"))
	if repo != "" {
		fmt.Printf("This tree: %s on %s (%s)\n\n", here, repo, strings.TrimSpace(gitOut(here, "rev-parse", "--abbrev-ref", "HEAD")))
	}

	var shown int
	for _, s := range live {
		if isSelf(s, me, mine) {
			continue
		}
		if !*all && repo != "" && s.Repo != repo {
			continue
		}
		shown++
		where := "another tree"
		if sameTree(s.Cwd, here) {
			where = "THIS tree"
		}
		fmt.Printf("- %s — %s, started %s ago, last active %s ago\n", s.Describe(), where, humanAge(time.Since(s.Started)), humanAge(time.Since(s.Seen)))
	}
	if shown == 0 {
		if *all {
			fmt.Println("No other live sessions registered on this machine.")
		} else {
			fmt.Printf("No other live session registered on %s.\n", repo)
		}
	}
	fmt.Println("\n" + whoTrailer)

	// What git knows: every working tree of this repository, whether or not a
	// session registered in it. Untracked means nobody committed a file; it
	// never means nobody owns it.
	if repo != "" {
		fmt.Printf("\n## Working trees of %s (git)\n\n", repo)
		for _, wt := range worktrees(here) {
			owner := "no assignment claims it"
			if st, err := assignment.NewStore(cfg.assignmentsRoot()); err == nil {
				if open, err := st.List(false); err == nil {
					for _, a := range open {
						if samePath(a.Worktree, wt.path) {
							owner = "assignment " + a.ID + " (" + a.State + ")"
						}
					}
				}
			}
			mark := ""
			if samePath(wt.path, here) {
				mark = "  ← here"
			}
			fmt.Printf("- %s  %s  %s%s\n", wt.path, wt.branch, owner, mark)
		}
	}
	return nil
}

// workingIn is the tree and repository a session is working in: the directory
// it was asked about, or — where that directory is no repository — the lane its
// own presence record says it holds.
//
// A shift's process stands in its home directory for its whole life and is
// asked about that directory on every turn, so the repository it works in
// exists nowhere but its record. Reading only the directory leaves a shift
// unable to see a peer on its own repository, and asks it to coordinate about a
// tree neither of them works in.
func workingIn(live []presence.Session, me string, mine int, here string) (tree, repo string) {
	if r := repoOf(here); r != "" {
		return here, r
	}
	for _, s := range live {
		if isSelf(s, me, mine) && s.Repo != "" {
			return s.Cwd, s.Repo
		}
	}
	return here, ""
}

// isSelf reports whether a presence record is this session's own.
//
// By id where the environment names one, and by runtime process otherwise: a
// reopened conversation is registered under a second id, and a session matching
// on id alone reads the other record as a peer — it is then told to coordinate
// with a session that is itself, and to message a socket that is its own.
func isSelf(s presence.Session, me string, mine int) bool {
	if me != "" && s.ID == me {
		return true
	}
	return mine > 0 && s.PID == mine
}

type worktree struct{ path, branch string }

func worktrees(dir string) []worktree {
	out := gitOut(dir, "worktree", "list", "--porcelain")
	var list []worktree
	var cur worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if cur.path != "" {
				list = append(list, cur)
			}
			cur = worktree{path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			cur.branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "detached":
			cur.branch = "(detached)"
		}
	}
	if cur.path != "" {
		list = append(list, cur)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].path < list[j].path })
	return list
}

// treeOf is the working tree a directory belongs to: git's top level, or the
// directory itself outside a repository. Sessions compare trees, not the
// directories they happen to be in — a session that has moved into a
// subdirectory of the tree it shares is still in that tree.
func treeOf(dir string) string {
	if dir == "" {
		return ""
	}
	if top := strings.TrimSpace(gitOut(dir, "rev-parse", "--show-toplevel")); top != "" {
		return top
	}
	return dir
}

func sameTree(a, b string) bool {
	return samePath(treeOf(a), treeOf(b))
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

func repoOf(dir string) string {
	url := strings.TrimSpace(gitOut(dir, "remote", "get-url", "origin"))
	if url == "" {
		top := strings.TrimSpace(gitOut(dir, "rev-parse", "--show-toplevel"))
		if top == "" {
			return ""
		}
		return filepath.Base(top)
	}
	url = strings.TrimSuffix(url, ".git")
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return url[i+1:]
	}
	return url
}

func gitOut(dir string, args ...string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// whoTrailer is what a session reads under the roster, and the roster is read
// at the moment work is chosen. Saying only who is here left what that reserved
// to be filled in, and survey shifts filled it in with the repository: three in
// one afternoon passed over every mellions-coxen issue — "live peer session
// in that repo — territory" — while directed shifts took the same work with the
// same peer present. The absent assignment clause is the evidence the peer holds
// no lane, and an absence states nothing on its own, so it is stated here.
const whoTrailer = "A live session is reached with ListAgents and SendMessage; one that has ended with\n" +
	"`claude --resume <id>`. A session that never registered is not listed here, so\n" +
	"absence is not proof of an empty tree.\n\n" +
	"None of this reserves a repository. Every lane is a worktree of its own, a session\n" +
	"listed with no assignment holds no lane at all, and what refuses a second lane is\n" +
	"the claim on the issue, which `mellions assign open` checks for you. Passing over a\n" +
	"repository's work because somebody is in it is a conclusion that stops the work,\n" +
	"and gets the same attack as any other."

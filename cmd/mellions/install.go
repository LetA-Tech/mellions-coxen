// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// The published source. An installation that names a local checkout is a
// developer's; everybody else gets the repository.
const publishedSource = "LetA-Tech/mellions-coxen"

// adapter is one runtime's own installer.
//
// Every command here belongs to Claude Code or Codex. Nothing writes their
// configuration directly: a runtime owns where its plugins live, what a
// marketplace is, and when a hook becomes trusted, and an installer that edited
// those files would be wrong the first time either of them changed.
type adapter struct {
	name     string
	bin      string
	market   []string // add a marketplace, source appended
	unmarket []string // drop the marketplace, so a new source can replace it
	remove   []string // clear any installed copy first
	install  []string // install the plugin
	list     []string
	manual   string // what this runtime needs from a person, and cannot be given
	pluginID string
}

// locate finds the runtime's binary. PATH first; then where the installers
// put it, because a non-login shell — a hook, a timer, an ssh command — often
// has none of those on PATH, and "not installed" would be the wrong answer.
func (a adapter) locate() (string, bool) { return locateBinary(a.bin) }

func locateBinary(bin string) (string, bool) {
	if p, err := exec.LookPath(bin); err == nil {
		return p, true
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".local", "bin", bin),
		filepath.Join(home, "bin", bin),
		filepath.Join(home, ".claude", "local", bin),
		"/usr/local/bin/" + bin,
		"/opt/homebrew/bin/" + bin,
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, true
		}
	}
	return "", false
}

var adapters = []adapter{
	{
		name: "Claude Code", bin: "claude",
		market:   []string{"plugin", "marketplace", "add"},
		unmarket: []string{"plugin", "marketplace", "remove", "mellions"},
		remove:   []string{"plugin", "uninstall", "mellions@mellions"},
		install:  []string{"plugin", "install", "mellions@mellions"},
		list:     []string{"plugin", "list"},
		pluginID: "mellions@mellions",
	},
	{
		name: "Codex", bin: "codex",
		market:   []string{"plugin", "marketplace", "add"},
		unmarket: []string{"plugin", "marketplace", "remove", "mellions"},
		remove:   []string{"plugin", "remove", "mellions@mellions"},
		install:  []string{"plugin", "add", "mellions@mellions"},
		list:     []string{"plugin", "list"},
		pluginID: "mellions@mellions",
		manual: "Codex will not run a plugin's hooks until they are trusted. Start Codex\n" +
			"once and accept the Mellions hooks when it asks, or run `/hooks` there.\n" +
			"Until then the Skills load and the session-start context does not.",
	},
}

func cmdInstall(ctx context.Context, args []string) error {
	fs := newFlagSet("install", flag.ContinueOnError)
	from := fs.String("from", "", "where to install from: a local checkout, or owner/repo (default "+publishedSource+")")
	only := fs.String("runtime", "", "install into one runtime only: claude or codex")
	dry := fs.Bool("dry-run", false, "say what would run, run nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	source, local, err := resolveSource(*from)
	if err != nil {
		return err
	}

	registered := ""
	if h, err := os.UserHomeDir(); err == nil {
		if reg := pluginreg.Read(h, pluginreg.ID); reg.Marketplace.InPlace() {
			registered = reg.Marketplace.Location
		}
	}
	if local {
		root := ""
		if cfg, err := loadConfig(""); err == nil {
			root = cfg.assignmentsRoot()
		}
		if why := refuseTemporaryTree(source, registered, root); why != "" {
			return errors.New(why)
		}
	}

	fmt.Printf("# Installing Mellions\n\nSource: %s\n", source)
	if local {
		fmt.Println("        a local checkout — whether a runtime reads it in place or copies it is\n" +
			"        the runtime's own decision, and is reported below from its own records")
	}
	// For a directory marketplace the marketplace path IS what sessions load
	// from, so this is the one line that says what this command changes.
	switch {
	case registered == "" && local:
		fmt.Printf("\nMarketplace: no directory marketplace is registered; it will point at\n             %s\n", source)
	case registered != "" && sameDir(source, registered):
		fmt.Printf("\nMarketplace: %s — unchanged\n", registered)
	case registered != "":
		fmt.Printf("\nMarketplace: %s\n          -> %s\n", registered, source)
		if !*dry {
			fmt.Println("             Sessions started after this load the plugin from the new path.")
		}
	}
	fmt.Println()

	var installed, absent, failed []string
	for _, a := range adapters {
		if *only != "" && !strings.EqualFold(*only, a.bin) {
			continue
		}
		path, ok := a.locate()
		if !ok {
			absent = append(absent, a.name)
			continue
		}
		a.bin = path
		fmt.Printf("## %s — %s\n\n", a.name, path)
		if err := a.run(ctx, source, *dry); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", a.name, err))
			fmt.Printf("  did not complete: %v\n\n", err)
			continue
		}
		installed = append(installed, a.name)
		if a.manual != "" {
			fmt.Printf("  Still needs you:\n")
			for _, l := range strings.Split(a.manual, "\n") {
				fmt.Printf("    %s\n", l)
			}
		}
		fmt.Println()
	}

	fmt.Println("## Where that leaves you")
	fmt.Println()
	verb := "Installed into"
	if *dry {
		verb = "Would install into"
	}
	switch {
	case len(installed) > 1 && len(failed) == 0:
		fmt.Printf("%s %s. Both packages carry the same Skills, the same identity\n",
			verb, strings.Join(installed, " and "))
		fmt.Println("and the same engineer; the deterministic surface is this binary in either.")
	case len(installed) == 1 && len(failed) == 0:
		fmt.Printf("%s %s. It carries the Skills, the identity and the engineer;\n",
			verb, installed[0])
		fmt.Println("the deterministic surface is this binary.")
	case len(installed) > 0:
		// Reporting the half that worked as though it were the whole is how an
		// installation ends up in two states that nobody meant it to be in.
		fmt.Printf("%s %s only. %s did not complete, so that runtime is still on\n",
			verb, strings.Join(installed, " and "), plural(len(failed), "runtime", "runtimes"))
		fmt.Println("whatever it had before — a different release, or nothing. This is a partial")
		fmt.Println("install and the two runtimes now disagree.")
	default:
		fmt.Println("Nothing was installed.")
	}
	if len(absent) > 0 {
		fmt.Printf("\nNot found on PATH or in the usual places, so not touched: %s.\n", strings.Join(absent, ", "))
	}
	if !*dry {
		if err := reportRegistration(); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if _, err := exec.LookPath("mellions"); err != nil {
		self, _ := os.Executable()
		fmt.Printf("\n`mellions` is not on PATH. The Skills and the session-start context work\n"+
			"without it; everything deterministic does not. Put %s on PATH,\n"+
			"or set MELLIONS_BIN to it.\n", self)
	}
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "; "))
	}
	return nil
}

// reportRegistration establishes, from the runtime's own files, that the next
// process it launches will load the plugin — and says which sessions running
// here will not.
//
// An installer that reports success because its commands exited zero has
// established nothing: the runtime can accept every command and still leave a
// registration that loads nothing, because the plugin is not enabled or the
// copy the registry points at is gone. The registry, the settings and the
// installed copy are read back and made to agree.
//
// The second half is the window this install just opened. Installing changes
// what the next process loads and reaches no process already running, so every
// session that started before this moment is now behind — with no signal of
// its own that anything changed. Naming them is the only way anyone finds out
// before the work goes wrong.
func reportRegistration() error {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("\nCould not read the runtime's registry: %v\n", err)
		return nil
	}
	reg := pluginreg.Read(home, pluginreg.ID)
	fmt.Println("\n## What the runtime will load next")
	fmt.Println()
	if !reg.Installed {
		fmt.Println("The registry does not name " + pluginreg.ID + " after installing.")
		for _, p := range reg.Problems {
			fmt.Println("  " + p)
		}
		return errors.New("the runtime's registry does not name " + pluginreg.ID + " after installing")
	}
	fmt.Printf("  registry     %s (%s)\n", pluginreg.ID, reg.Version)
	fmt.Printf("  loads from   %s\n", reg.LoadPath)
	if reg.Marketplace.InPlace() {
		fmt.Printf("  read         in place — %s is a %s marketplace\n", reg.Marketplace.Name, reg.Marketplace.Source)
		if reg.InstallPath != "" && reg.InstallPath != reg.LoadPath {
			fmt.Printf("  inert copy   %s\n", reg.InstallPath)
		}
	} else if reg.Marketplace.Source != "" {
		fmt.Printf("  read         from a copy — %s is a %s marketplace the runtime fetched\n",
			reg.Marketplace.Name, reg.Marketplace.Source)
	}
	if !reg.Registered.IsZero() {
		fmt.Printf("  registered   %s\n", reg.Registered.Format("2006-01-02T15:04:05Z"))
	}
	switch {
	case reg.Enabled && reg.EnabledIn != "":
		fmt.Printf("  enabled      yes, in %s\n", reg.EnabledIn)
	case reg.Enabled:
		fmt.Printf("  enabled      yes, by default — no settings file names it either way\n")
	default:
		fmt.Printf("  enabled      NO — %s disables it\n", reg.EnabledIn)
	}
	fmt.Printf("  hooks        %d SessionStart hooks in %s\n", len(reg.Hooks), reg.HooksFile)
	for _, p := range reg.Problems {
		fmt.Println("  problem      " + p)
	}
	if !reg.Ready() {
		fmt.Println("\nThe commands ran, and the runtime's own files say the next process will")
		fmt.Println("still not load this plugin. Nothing above is a warning to act on later.")
		return errors.New("installed, but the runtime's files do not establish it will load: " +
			strings.Join(reg.Problems, "; "))
	}
	if reg.Marketplace.InPlace() {
		fmt.Printf("\nA process launched from now on loads the plugin out of %s\n", reg.LoadPath)
		fmt.Println("itself, not out of a copy. What is committed there is what the next session")
		fmt.Println("runs, so `git pull` there deploys hooks, Skills, commands and the agent —")
		fmt.Println("everything except this binary, which `make install` puts on PATH. Running")
		fmt.Println("this command again is not what deploys them, and does not have to.")
	} else {
		fmt.Println("\nA process launched from now on will load the copy above. A change at the")
		fmt.Println("source reaches nothing until this command is run again, because the runtime")
		fmt.Println("reads the copy and refreshes it only on install.")
	}
	fmt.Println("\nA process already running loads neither: a runtime binds a session's hooks")
	fmt.Println("when it launches, and neither /clear nor /compact re-resolves them.")

	reportBehindSessions(reg.Registered)
	return nil
}

// reportBehindSessions names the live sessions on this machine whose runtime
// process began before a registration, and so do not carry it.
//
// Registered against the process's own start time, not the session's first
// registration with Mellions: a session that only registered later is still a
// process that launched when it launched, and it is the launch that bound its
// hooks.
func reportBehindSessions(registered time.Time) {
	if registered.IsZero() {
		return
	}
	cfg, err := loadConfig("")
	if err != nil {
		fmt.Println("\nWhich live sessions are behind this registration is unestablished: no\nconfig to find the presence records — " + err.Error())
		return
	}
	live := cfg.presences().Live()
	self := presence.SelfPID()
	var behind []presence.Session
	for _, s := range live {
		if s.PID == self && self > 0 {
			continue
		}
		began := s.ProcStarted
		if began.IsZero() {
			began = s.Started
		}
		if began.Before(registered) {
			behind = append(behind, s)
		}
	}
	fmt.Println()
	if len(behind) == 0 {
		fmt.Println("No live session registered here started before this registration, so none")
		fmt.Println("is behind it. A session that never registered is not in that record, so")
		fmt.Println("this is not proof that nothing is behind.")
		return
	}
	// Not "has no Mellions": most of these are running an earlier registration
	// and carrying its identity and Skills. What is true of all of them is
	// that they are running something other than what was just installed, and
	// that nothing can move them onto it.
	fmt.Printf("%s started before this registration. Each is running whatever was\n",
		plural(len(behind), "live session", "live sessions"))
	fmt.Println("registered when it launched — an earlier release, or nothing — and this")
	fmt.Println("install did not reach it and cannot. A session whose installed copy was")
	fmt.Println("replaced underneath it can also stop firing the hooks it did have, at its")
	fmt.Println("next /compact. Each needs a new process:")
	fmt.Println()
	for _, s := range behind {
		began := s.ProcStarted
		if began.IsZero() {
			began = s.Started
		}
		fmt.Printf("  %s  pid %d  %s\n", began.Format("2006-01-02T15:04:05Z"), s.PID, s.Describe())
		fmt.Printf("      %s\n", s.Resume())
	}
	fmt.Println("\nA session that never registered is not listed, so this is a floor and not")
	fmt.Println("a complete list. `mellions doctor` inside a session answers for that one.")
}

// step is one runtime command. A step marked spent may fail without failing the
// install: removing something that was never installed is not an error.
type step struct {
	args  []string
	spent bool
}

// steps clears the installed copy and the marketplace before adding the source
// again.
//
// Both runtimes cache a plugin under the version in its manifest, and neither
// re-reads the source while that version is unchanged: `claude plugin update`
// answers "already at the latest version" and refreshes nothing. An installer
// that skipped the removal would report success over the previous content,
// which is the one outcome an installer must not have.
//
// The marketplace is a second object and had to be dropped separately. Removing
// only the plugin left the marketplace bound to its old source, so installing
// from anywhere else failed with "already added from a different source" — which
// made the ordinary act of installing from a checkout impossible on a machine
// that had ever installed from anywhere else.
func (a adapter) steps(source string) []step {
	return []step{
		{a.remove, true},
		{a.unmarket, true},
		{append(append([]string{}, a.market...), source), false},
		{a.install, false},
	}
}

func (a adapter) run(ctx context.Context, source string, dry bool) error {
	for _, s := range a.steps(source) {
		line := a.bin + " " + strings.Join(s.args, " ")
		if dry {
			fmt.Printf("  would run: %s\n", line)
			continue
		}
		fmt.Printf("  %s\n", line)
		c, cancel := context.WithTimeout(ctx, 3*time.Minute)
		out, err := exec.CommandContext(c, a.bin, s.args...).CombinedOutput()
		cancel()
		if err != nil && !s.spent {
			return fmt.Errorf("%s: %v: %s", line, err, firstLine(string(out)))
		}
	}
	return nil
}

// refuseTemporaryTree is why a source path must not become the marketplace,
// and is empty when it may.
//
// For a directory marketplace the marketplace path is what sessions load from,
// so registering a lane's worktree points every new session on the host at a
// tree that is deleted when the lane closes — and the sessions that already
// loaded it keep hooks and Skills from a path that no longer exists. Nothing
// about the install fails at the time, which is what makes it worth refusing
// here: `resolveSource` walks up from the working directory, so running this
// from inside a lane is the ordinary way to cause it, not an unusual one.
//
// The judgement is on the source alone. It is deliberately not excused by the
// marketplace already pointing there: once a lane has been registered by any
// route this did not cover, exempting it would disarm the guard in exactly the
// state it exists to end, and report "unchanged" while the host stays broken.
// Nor is it asked of one runtime's records — `mellions install` registers the
// source with every runtime on the machine, so a source that is unfit is unfit
// for all of them. The repair is `-from <checkout>`, which is never refused.
func refuseTemporaryTree(source, registered, assignmentsRoot string) string {
	names := ""
	if registered != "" {
		names = " The marketplace currently points at " + registered + "."
	}
	if main, linked := worktreeOf(source); linked {
		return source + " is a linked git worktree of " + main + ", not a checkout." +
			" Registering it would make every new session on this host load hooks," +
			" Skills, commands and the agent out of it, and it goes away when the lane" +
			" closes." + names + " Install from the checkout, or pass -from <checkout> explicitly."
	}
	if assignmentsRoot != "" && underDir(source, assignmentsRoot) {
		return source + " is under the assignments root " + assignmentsRoot + ", so it is a" +
			" lane's tree rather than a checkout. Registering it would make every new" +
			" session on this host load the plugin from a tree that is temporary." + names +
			" Install from the checkout, or pass -from <checkout> explicitly."
	}
	return ""
}

// worktreeOf reports whether dir is a linked worktree, and of what. Git answers
// with the common directory shared by every worktree of a repository, which for
// the main one is its own .git and for a linked one is somewhere else entirely.
// worktreeOf reports whether dir is a linked worktree, and of what.
//
// The comparison is between two directories git itself resolves, never between
// one of git's answers and the path the caller happened to type. `--git-dir`
// comes back as an absolute physical path while `--git-common-dir` can be
// relative, and joining the relative one onto a logical path makes /tmp and
// /private/tmp differ — which reported every subdirectory of a plain checkout
// as a linked worktree on this platform.
func worktreeOf(dir string) (string, bool) {
	common, err := gitIn(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", false // not a checkout at all; not this guard's business
	}
	own, err := gitIn(dir, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return "", false
	}
	if sameDir(common, own) {
		return "", false
	}
	return filepath.Dir(filepath.Clean(common)), true
}

func gitIn(dir string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func sameDir(a, b string) bool {
	ra, rb := resolve(a), resolve(b)
	return ra != "" && ra == rb
}

// underDir reports whether path sits inside root.
//
// Both are made absolute and symlink-resolved first: a lexical prefix test
// silently matches nothing when the configured root is relative or reached
// through a symlink, and a guard that silently matches nothing is worse than
// no guard, because it is believed. The separator is appended so a sibling
// whose name merely starts with root's is not inside it.
func underDir(path, root string) bool {
	p, r := resolve(path), resolve(root)
	if p == "" || r == "" {
		return false
	}
	if p == r {
		return true
	}
	// filepath.Clean("/") is "/", so appending a separator would make "//".
	if r == string(filepath.Separator) {
		return true
	}
	return strings.HasPrefix(p+string(filepath.Separator), r+string(filepath.Separator))
}

// resolve makes a path absolute and follows symlinks, falling back to the
// absolute form when the path does not exist yet.
func resolve(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}

// resolveSource decides what to install from, preferring a checkout the caller
// is standing in over the published repository.
func resolveSource(from string) (string, bool, error) {
	if from == "" {
		if wd, err := os.Getwd(); err == nil {
			if root, ok := marketplaceRoot(wd); ok {
				return root, true, nil
			}
		}
		if self, err := os.Executable(); err == nil {
			if root, ok := marketplaceRoot(filepath.Dir(self)); ok {
				return root, true, nil
			}
		}
		return publishedSource, false, nil
	}
	if strings.Contains(from, "/") && !strings.HasPrefix(from, ".") && !filepath.IsAbs(from) {
		if _, err := os.Stat(from); err != nil {
			return from, false, nil // owner/repo
		}
	}
	abs, err := filepath.Abs(from)
	if err != nil {
		return "", false, err
	}
	if _, ok := marketplaceRoot(abs); !ok {
		return "", false, fmt.Errorf("%s has no .claude-plugin/marketplace.json", abs)
	}
	return abs, true, nil
}

// marketplaceRoot walks up looking for the manifest that makes a directory
// installable, so running this from anywhere inside a checkout works.
func marketplaceRoot(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".claude-plugin", "marketplace.json")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "One " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

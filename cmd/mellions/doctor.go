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
	"runtime"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// cmdDoctor establishes what this installation can actually do, before
// anything relies on it. It observes and configures nothing. Every line is
// present, absent or unknown, and unknown is never collapsed into absent.
func cmdDoctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var absent []string
	// Stopped is a second way for a line to fail the command, and it is not the
	// same claim as absent: the thing is installed and readable, and the host
	// has stopped receiving what it installs. A state word that no reader's
	// exit code depends on is a sentence somebody has to be asked to read.
	var stopped []string
	line := func(name, state, detail string) {
		fmt.Printf("%-22s %-8s %s\n", name, state, detail)
		switch state {
		case "ABSENT":
			absent = append(absent, name)
		case "STOPPED":
			stopped = append(stopped, name)
		}
	}
	fmt.Printf("# mellions doctor — %s (%s)\n\n", Version, Commit)

	if p, err := exec.LookPath("mellions"); err == nil {
		line("binary", "present", p)
	} else if self, err := os.Executable(); err == nil {
		line("binary", "ABSENT", "not on PATH; this one is "+self+" — put it on PATH or set MELLIONS_BIN")
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		line("config", "ABSENT", err.Error())
	} else {
		line("config", "present", cfg.path)
		set := cfg.checkouts()
		if missing := checkout.Missing(set, cfg.Repos); len(missing) > 0 {
			line("checkouts", "partial", fmt.Sprintf("%d of %d repositories found; missing %s",
				len(cfg.Repos)-len(missing), len(cfg.Repos), strings.Join(missing, ", ")))
		} else if len(cfg.Repos) == 0 {
			line("checkouts", "unknown", "no repositories configured")
		} else {
			line("checkouts", "present", fmt.Sprintf("%d repositories under %s", len(cfg.Repos), strings.Join(cfg.roots(), ", ")))
		}
		if st, err := assignment.NewStore(cfg.assignmentsRoot()); err == nil {
			open, _ := st.List(false)
			line("assignments", "present", fmt.Sprintf("%d open, %s", len(open), cfg.assignmentsRoot()))
		} else {
			line("assignments", "ABSENT", err.Error())
		}
		if slugs, err := slugsIn(cfg.programsDir()); err == nil && len(slugs) > 0 {
			line("program", "present", strings.Join(slugs, ", "))
		} else {
			line("program", "unknown", "none discovered yet — `mellions program discover`")
		}
		if slugs, err := slugsIn(cfg.partnersDir()); err == nil && len(slugs) > 0 {
			line("partnership", "present", strings.Join(slugs, ", "))
		} else {
			line("partnership", "unknown", "none established yet — `mellions partner establish`")
		}
		if _, age, ok := surveyBrief(cfg.surveyPath()); ok {
			line("survey", "present", fmt.Sprintf("saved %s ago at %s", humanAge(age), cfg.surveyPath()))
		} else {
			line("survey", "unknown", "none saved yet — `mellions survey -save`")
		}
		// The runner keeps its lock and log where the shifts land, and both
		// scripts ask the binary for that directory, so Config.home is the one
		// answer rather than a second reading of the same environment.
		state, detail := runnerState(cfg.home())
		line("runner", state, detail)
		shifts := filepath.Join(cfg.home(), "shifts")
		if _, err := os.Stat(shifts); err == nil {
			line("shifts", "present", shifts)
		} else {
			line("shifts", "unknown", shifts+" — no shift has run here yet")
		}
	}

	if _, err := exec.LookPath("git"); err == nil {
		line("git", "present", "")
	} else {
		line("git", "ABSENT", "the assignment lifecycle and every source need it")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		line("tracker (gh)", "ABSENT", "the github and stale sources, and continuity's tracker reads, need it")
	} else {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		out, err := exec.CommandContext(c, "gh", "auth", "status").CombinedOutput()
		cancel()
		switch {
		case err == nil:
			line("tracker (gh)", "present", firstLine(strings.TrimSpace(string(out))))
		default:
			line("tracker (gh)", "unknown", "installed, not authenticated: "+firstLine(string(out)))
		}
	}

	for _, rt := range []struct{ name, bin string }{{"claude", "claude"}, {"codex", "codex"}} {
		p, ok := locateBinary(rt.bin)
		if !ok {
			line("runtime "+rt.name, "absent", "not on PATH or in the usual places")
			continue
		}
		state, detail := pluginState(rt.name, p)
		line("runtime "+rt.name, state, detail)
	}

	reg := pluginreg.Read(home(), pluginreg.ID)
	hooksRoot := pluginRoot(reg)
	if reg.Installed {
		state, detail := loadPathState(reg)
		line("load path", state, detail)
		if t, ok := pluginreg.ReadTree(hooksRoot); ok {
			state, detail := treeState(t)
			line("load path commit", state, detail)
		}
	}
	{
		state, detail := codexTrustState(pluginreg.ReadCodexTrust(home(), pluginreg.ID, hooksRoot))
		line("codex hooks", state, detail)
	}
	switch {
	case hooksRoot == "":
		line("hooks", "ABSENT", "no installed copy to read them from — `mellions install`")
	case len(reg.Hooks) > 0:
		line("hooks", "present", fmt.Sprintf("%d SessionStart hooks declared in %s", len(reg.Hooks), reg.HooksFile))
	default:
		line("hooks", "ABSENT", filepath.Join(hooksRoot, "hooks", "hooks.json")+" declares no SessionStart hooks")
	}
	if hooksRoot != "" {
		// The three Skills every session's engineering rests on; one missing
		// means the session-start hooks are delivering a partial engineer.
		var missing []string
		for _, name := range []string{"mellions-reasoning", "mellions-deep-research", "mellions-falsification", "mellions-self-learning"} {
			if _, err := os.Stat(filepath.Join(hooksRoot, "skills", name, "SKILL.md")); err != nil {
				missing = append(missing, name)
			}
		}
		if len(missing) == 0 {
			line("bearing skills", "present", "reasoning, deep research, falsification, self-learning under "+hooksRoot)
		} else {
			line("bearing skills", "ABSENT", strings.Join(missing, ", ")+" missing under "+filepath.Join(hooksRoot, "skills"))
		}
	}

	fmt.Println()
	reportSessionLoad(reg)
	fmt.Println()
	switch {
	case len(absent) > 0 && len(stopped) > 0:
		return errors.New("absent: " + strings.Join(absent, ", ") +
			"; stopped deploying: " + strings.Join(stopped, ", "))
	case len(absent) > 0:
		return errors.New("absent: " + strings.Join(absent, ", "))
	case len(stopped) > 0:
		return errors.New("stopped deploying: " + strings.Join(stopped, ", "))
	}
	fmt.Println("Nothing load-bearing is absent or has stopped deploying. Unknown lines are unestablished, not missing.")
	return nil
}

// loadPathState says where the runtime will read the plugin from, and which of
// its own records establishes it.
//
// The two records answer different questions and only one of them answers this
// one. installed_plugins.json names the copy the installer wrote, whatever the
// source; known_marketplaces.json says whether that copy is read at all. A
// marketplace added from a directory is loaded in place, so the copy is inert
// and the checkout is what every session runs — which makes `git pull` there a
// deployment of hooks, Skills, commands and the agent, and makes the copy's
// commit a record of nothing.
func loadPathState(reg pluginreg.Registration) (string, string) {
	m := reg.Marketplace
	switch {
	case reg.LoadPath == "":
		return "ABSENT", "neither registry names a path to load from — `mellions install`"
	case m.InPlace():
		d := reg.LoadPath + " — read in place: known_marketplaces.json records the " +
			m.Name + " marketplace as a " + m.Source + " source, so a session loads hooks," +
			" Skills, commands and the agent from there and `git pull` there deploys them"
		if reg.InstallPath != "" && reg.InstallPath != reg.LoadPath {
			d += "; the copy at " + reg.InstallPath + " is written by install and never read"
		}
		return "present", d
	case m.Source != "":
		return "present", reg.LoadPath + " — a copy: known_marketplaces.json records the " +
			m.Name + " marketplace as a " + m.Source + " source, which the runtime fetched" +
			" and copied, so installed_plugins.json's installPath is what loads"
	default:
		// The marketplace registry could not answer. The copy is the better
		// guess and is still a guess, so it is not reported as established.
		return "unknown", reg.LoadPath + " — installed_plugins.json's installPath, which is what" +
			" loads only for a marketplace the runtime fetched: " + m.Problem
	}
}

// treeState is the commit the load path stands at when the runtime reads the
// plugin out of a git checkout.
//
// Uncommitted, behind or ahead is reported rather than passed: a host whose
// checkout is any of the three is running something no remote branch names,
// and the record that says which release is deployed here is wrong about it.
func treeState(t pluginreg.Tree) (string, string) {
	// `rev-parse --abbrev-ref HEAD` answers the literal "HEAD" off a branch, so
	// the branch name is the one field whose populated-ness does not mean it was
	// answered. Printed unqualified it reads as a branch called HEAD.
	detached := t.Head != "" && t.BranchKnown && (t.Branch == "" || t.Branch == "HEAD")
	d := t.Head
	switch {
	case detached:
		d += " on NO BRANCH — detached HEAD, and `git pull --ff-only` there exits 1"
	case t.Branch != "":
		d += " on " + t.Branch
	}
	switch {
	case !t.StatusKnown:
		// Not "clean". A status that could not be read is the one case where
		// the reassuring word would be printed by a command that failed.
		d += ", working tree UNKNOWN"
	case t.Dirty:
		d += ", UNCOMMITTED changes — a session loads them"
	default:
		d += ", clean"
	}
	switch {
	case t.Upstream == "":
	case t.Behind == 0:
		// Qualified deliberately: doctor does not fetch, so this compares
		// against a remote-tracking ref that is only as fresh as the last one.
		// Unqualified, it certifies a host that has not fetched in a week.
		d += ", 0 behind " + t.Upstream + " as of the last fetch"
	default:
		d += fmt.Sprintf(", %d behind %s as of the last fetch — `git pull` deploys them", t.Behind, t.Upstream)
	}
	// Ahead is reported after behind and never folded into it. A checkout the
	// runtime loads in place is deployed by `git pull --ff-only`, which
	// succeeds while it is ahead and the upstream has not moved and refuses
	// once it has: the first pull that would have deployed a change is the one
	// that fails, and until then every reading of this line says the host is
	// current. It is also the base lanes are cut from, so the next lane
	// inherits them.
	//
	// The count is against the upstream and nothing else is read, so the line
	// says what was counted. Whether some other remote ref holds these commits
	// decides whether resetting the checkout loses them, and that is the
	// reader's to check — claiming it here would be claiming a ref nobody read.
	// The prune is part of the remedy rather than decoration: a fetch does not
	// drop remote-tracking refs, so a deleted branch still answers "a remote
	// holds them" to the reader deciding whether a reset is lossless.
	if t.Upstream != "" && t.Ahead > 0 {
		d += fmt.Sprintf(", and %d AHEAD of %s — `git pull --ff-only` refuses"+
			" as soon as %s moves, so this host stops deploying; push them, or reset the checkout"+
			" to its upstream once `git fetch --prune && git branch -r --contains HEAD` shows a remote holds them",
			t.Ahead, t.Upstream, t.Upstream)
	}
	for _, p := range t.Problems {
		d += " — " + p
	}
	// The word is derived from whether every question this line answers was
	// actually answered, not from a fixed list of fields that happen to hold a
	// non-zero value. Problems is the struct's own record that something could
	// not be read, so a non-empty Problems can never end in the word a reader
	// acts on: that is the term the previous shape was missing, and it is why
	// "no upstream configured" printed as a checkout that is up to date.
	//
	// STOPPED is reserved for the states where `git pull --ff-only` in this
	// checkout cannot deploy today, so the exit code carries them. Dirty and
	// behind are deliberately not among them: behind IS the pending deployment
	// and dirty is a local edit, and failing on either would make doctor red on
	// an ordinary working host. A checkout with no upstream is not stopped
	// either — a public install exported without a remote is a legitimate shape
	// — but it is not "present", because nothing compared it against anything.
	switch {
	case detached, t.Upstream != "" && t.Ahead > 0:
		return "STOPPED", d
	case len(t.Problems) > 0, !t.StatusKnown, t.Dirty, t.Behind > 0:
		return "partial", d
	}
	return "present", d
}

// codexTrustState says whether Codex may run this plugin's hooks.
//
// Codex loads a plugin's Skills whether or not its hooks are trusted, so an
// untrusted installation reads as complete from every other line here and
// delivers a session that has every method and does not know who it is.
// Trusting is Codex's own prompt to a person and nothing here can grant it;
// counting it is what makes the gap visible.
func codexTrustState(t pluginreg.CodexTrust) (string, string) {
	if t.Problem != "" {
		return "unknown", t.Problem
	}
	d := fmt.Sprintf("%d of %d trusted", t.Trusted, t.Declared)
	switch {
	case t.Trusted == 0 && t.Declared > 0:
		// Nothing trusted is the one count that certainly means what it says:
		// no hook runs, so no session-start context arrives.
		return "partial", d + " — start codex once here and trust them, or a Codex session is not Mellions"
	case t.Short():
		// Some trusted, some not. Which hooks are missing is not readable
		// here, and the session-start ones may well be among the trusted, so
		// this does not claim the session is not Mellions — a false alarm on
		// a line whose only value is being worth acting on.
		return "partial", d + " — some are not; `/hooks` in codex says which"
	}
	return "present", d + ", per " + t.Config
}

// staleAgainst reports whether a session's most recent SessionStart happened
// before the current registration, which makes what it is carrying an earlier
// installed copy whatever its hook names say.
//
// Both times must be known: a zero registration time or a transcript entry
// with no timestamp makes the comparison meaningless, and answering it anyway
// would report every session stale on a registry that omits one field.
func staleAgainst(last pluginreg.Event, reg pluginreg.Registration) (bool, time.Time) {
	if reg.Registered.IsZero() || last.At.IsZero() {
		return false, time.Time{}
	}
	return last.At.Before(reg.Registered), last.At
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

// pluginRoot is where the runtime loads this plugin's hooks and Skills from.
//
// CLAUDE_PLUGIN_ROOT is exported to a hook the plugin itself declares, and to
// nothing else: a session running this from a tool call does not have it, which
// is every way a session actually runs it. Reading it alone reported "unknown"
// to every session that asked, including ones that had the plugin loaded. The
// registration's load path is the answer that exists outside a hook, and it is
// the copy under the runtime's cache only where that copy is what loads.
func pluginRoot(reg pluginreg.Registration) string {
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		return root
	}
	return reg.LoadPath
}

// reportSessionLoad says whether the session this runs inside actually loaded
// the plugin, and — where it did not — what can and cannot be done about it.
//
// The registry says what the next process will load. It says nothing about a
// process already running: a runtime binds a session's hooks when it launches,
// and installing, reinstalling or removing a plugin afterwards does not reach
// it. So the two are reported separately, and the one that decides what this
// session can do is read from the runtime's own transcript for it, which
// records every hook it ran.
func reportSessionLoad(reg pluginreg.Registration) {
	fmt.Println("## Whether this session has it")
	fmt.Println()

	claim := "the registry does not name " + pluginreg.ID
	if reg.Installed {
		claim = fmt.Sprintf("%s %s at %s", pluginreg.ID, reg.Version, reg.LoadPath)
		if !reg.Registered.IsZero() {
			claim += ", registered " + reg.Registered.Format("2006-01-02T15:04:05Z")
		}
		switch {
		case reg.Enabled && reg.EnabledIn != "":
			claim += ", enabled in " + reg.EnabledIn
		case reg.Enabled:
			claim += ", enabled by default"
		default:
			claim += ", DISABLED by " + reg.EnabledIn
		}
	}
	fmt.Println("Registry (what the NEXT process launched here will load):")
	fmt.Println("  " + claim)
	for _, p := range reg.Problems {
		fmt.Println("  problem: " + p)
	}

	_, sessionID := presence.Here()
	if sessionID == "" {
		fmt.Println("\nThis process is not inside a coding-agent session — a terminal, a hook, a\ntimer. There is no session to have loaded anything; the registry above is\nthe whole answer.")
		return
	}
	cwd, _ := os.Getwd()
	load, ok := pluginreg.ReadLive(home(), cwd, sessionID, presence.SelfStarted(), reg.Hooks)
	if !ok {
		fmt.Printf("\nSession %s: the runtime has written no transcript for it that could be\nfound under %s, so what it loaded is unestablished — not absent.\n",
			sessionID, filepath.Join(home(), ".claude", "projects"))
		return
	}
	if load.Exported != "" {
		fmt.Printf("\nThis process was resumed: the runtime exports %s, the conversation it\ncontinues, and writes its own transcript under %s. What follows is the\ntranscript this process is writing — the other one ended.\n",
			load.Exported, load.SessionID)
	}
	for _, p := range load.Ambiguous {
		fmt.Printf("\nAmbiguous: %s was also launched into this directory within minutes of this\nprocess, so which transcript is this one's cannot be told apart by time.\n", p)
	}
	if len(load.Ambiguous) > 0 {
		fmt.Println("What follows is the exported id's transcript, which may be another\nsession's evidence.")
	}
	last, has := load.Latest()
	if !has {
		fmt.Printf("\nSession %s: its transcript records no SessionStart hooks at all, so no\nplugin's session-start context reached it.\n", load.SessionID)
	} else {
		fmt.Printf("\nSession %s (%s):\n", load.SessionID, load.Transcript)
		for _, e := range load.Events {
			fmt.Printf("  %s %-8s %d hooks ran, %d of them this installation's\n",
				e.At.Format("2006-01-02T15:04:05Z"), e.Kind, e.Total, e.Ours)
		}
	}
	if load.Has() {
		// A hook is matched by the name the runtime records, and two
		// registrations that declare the same hook names are indistinguishable
		// by name alone. What separates them is when: a session whose last
		// SessionStart predates the current registration ran the copy that was
		// installed then, and is carrying that copy's identity, Skills and
		// documents however current the names look.
		if stale, when := staleAgainst(last, reg); stale {
			fmt.Printf("\nThis session loaded the plugin, and loaded it BEFORE the current\nregistration: its last SessionStart was %s and the registration is\n%s. The hook names match, so this reads as current; the file contents\nit is carrying are the copy installed at the earlier time.\n",
				when.Format("2006-01-02T15:04:05Z"), reg.Registered.Format("2006-01-02T15:04:05Z"))
			fmt.Println("\nA session whose installed copy was replaced underneath it can also stop")
			fmt.Println("firing those hooks entirely at its next /compact. To be on this")
			fmt.Printf("registration, a new process is the only way:\n\n  claude --resume %s\n", sessionID)
			return
		}
		fmt.Println("\nThis session loaded the plugin. Identity, method, catalog and commands are\nin front of it.")
		return
	}
	// The remedy, and the two that are not remedies. A session that reaches
	// for /clear here loses its context and acquires nothing, because the
	// runtime re-runs SessionStart for the hooks the process already had.
	// What can be established is that none of the hooks this installation
	// declares ran. Whether the session has an earlier release's identity or
	// none at all is not readable from here, and asserting either would be a
	// claim about something nothing above observed. The remedy is the same.
	fmt.Println("\nTHIS SESSION IS NOT RUNNING THIS INSTALLATION. None of the hooks it declares")
	fmt.Println("ran for this session, whatever the registry above says: what identity, method,")
	fmt.Println("Skill catalog and commands it has are an earlier release's, or none.")
	if stale, when := staleAgainst(last, reg); stale {
		fmt.Printf("Its last SessionStart was %s, before the current registration at %s:\nit was launched against a registration that is gone.\n",
			when.Format("2006-01-02T15:04:05Z"), reg.Registered.Format("2006-01-02T15:04:05Z"))
	}
	fmt.Println("\n/clear and /compact will not fix it: both re-run SessionStart for the hooks")
	fmt.Println("this process already carries, and acquire no new plugin. Reinstalling will")
	fmt.Println("not reach it either. The only remedy is a NEW process:")
	fmt.Println()
	fmt.Printf("  claude --resume %s     # same conversation, new process, hooks re-resolved\n", sessionID)
	fmt.Println()
	fmt.Println("Until then, `mellions` on PATH is the whole of this installation that this")
	fmt.Println("session can reach: none of its Skills, its session-start context or its")
	fmt.Println("PreToolUse hooks are running here, so do not rely on one being there.")
}

// codexToolHost reports whether the helper Codex routes tool calls through is
// installed and executable, and the path it looked at.
//
// Codex runs every tool — Skill body load, file read, shell — through
// codex-code-mode-host and fails closed per call when it cannot spawn it.
// `codex --version` and `codex plugin list` both answer normally in that state,
// so an installation that can carry no method and run no command reads as
// healthy on every other line here.
func codexToolHost(bin string) (string, bool) {
	candidates := []string{filepath.Join(filepath.Dir(bin), "codex-code-mode-host")}
	if real, err := filepath.EvalSymlinks(bin); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(real), "codex-code-mode-host"))
		if filepath.Base(real) == "codex.js" {
			candidates = append(candidates, managedCodexToolHosts(filepath.Dir(filepath.Dir(real)))...)
		}
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return path, true
		}
	}
	return candidates[0], false
}

// The npm launcher resolves a platform package with Node's ordinary ancestor
// lookup, then runs the native binary and tool host from that package together.
func managedCodexToolHosts(packageRoot string) []string {
	packageName, target := codexPlatformPackage()
	if packageName == "" {
		return nil
	}
	var paths []string
	for dir := packageRoot; ; dir = filepath.Dir(dir) {
		paths = append(paths, filepath.Join(dir, "node_modules", "@openai", packageName,
			"vendor", target, "bin", "codex-code-mode-host"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	paths = append(paths, filepath.Join(packageRoot, "vendor", target, "bin", "codex-code-mode-host"))
	return paths
}

func codexPlatformPackage() (string, string) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "codex-darwin-arm64", "aarch64-apple-darwin"
	case "darwin/amd64":
		return "codex-darwin-x64", "x86_64-apple-darwin"
	case "linux/arm64":
		return "codex-linux-arm64", "aarch64-unknown-linux-musl"
	case "linux/amd64":
		return "codex-linux-x64", "x86_64-unknown-linux-musl"
	default:
		return "", ""
	}
}

// pluginState reads the runtime's own record of installed plugins.
func pluginState(runtime, bin string) (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "unknown", bin
	}
	if runtime == "codex" {
		// Codex keeps no readable record; its own listing is the answer.
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		out, err := exec.CommandContext(c, bin, "plugin", "list").CombinedOutput()
		if err != nil {
			return "unknown", "codex plugin list failed: " + firstLine(string(out))
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "mellions@") {
				if strings.Contains(line, "installed") && !strings.Contains(line, "not installed") {
					if host, ok := codexToolHost(bin); !ok {
						return "partial", strings.Join(strings.Fields(line), " ") +
							" — but " + host + " is missing, so codex fails every tool call closed: no Skill body, no file read, no shell"
					}
					return "present", strings.Join(strings.Fields(line), " ")
				}
				return "absent", strings.Join(strings.Fields(line), " ") + " — `mellions install`"
			}
		}
		return "absent", "on PATH, plugin not installed — `mellions install`"
	}
	reg := pluginreg.Read(home, pluginreg.ID)
	switch {
	case !reg.Installed && len(reg.Problems) > 0:
		return "absent", reg.Problems[0]
	case !reg.Installed:
		return "absent", "on PATH, plugin not installed — `mellions install`"
	case len(reg.Problems) > 0:
		// Installed and still not loadable — a disabled plugin or a registry
		// pointing at a copy that is gone. Reporting that as present is how an
		// installation looks healthy while delivering nothing.
		return "partial", fmt.Sprintf("%s (%s) at %s — %s",
			pluginreg.ID, reg.Version, reg.LoadPath, strings.Join(reg.Problems, "; "))
	default:
		return "present", fmt.Sprintf("%s (%s) at %s, enabled, %d SessionStart hooks",
			pluginreg.ID, reg.Version, reg.LoadPath, len(reg.Hooks))
	}
}

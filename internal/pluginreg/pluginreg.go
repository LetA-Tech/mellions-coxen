// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package pluginreg reads the runtime's own record of whether it will load
// this plugin, and its own record of whether it did load it for a session.
//
// Two different questions, two different files, and conflating them is how a
// session ends up believing it has an engineer it does not have:
//
//   - Will the next process load it? That is the registry — which plugin is
//     installed, where its copy lives, and whether it is enabled. Changing it
//     changes nothing for a process already running.
//   - Did this process load it? That is the transcript the runtime writes for
//     the session, which records every hook it ran at SessionStart and the
//     command it ran. It is the only record of what a running process actually
//     acquired, and it does not change when the registry does.
//
// Nothing here writes. A runtime owns its own configuration; this reads it.
package pluginreg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// configRoot is where the runtime keeps its own configuration. It resolves
// CLAUDE_CONFIG_DIR first and falls back to ~/.claude, which is what the
// runtime itself does; reading ~/.claude unconditionally reads a file the
// runtime does not use on any host that sets the variable.
func configRoot(home string) string {
	if d := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); d != "" {
		return d
	}
	return filepath.Join(home, ".claude")
}

// codexRoot is the state directory Codex itself reads. CODEX_HOME relocates
// config, plugin cache, trust, auth, and session state as one boundary; reading
// ~/.codex while it is set observes files the active runtime does not use.
func codexRoot(home string) string {
	if d := strings.TrimSpace(os.Getenv("CODEX_HOME")); d != "" {
		return d
	}
	return filepath.Join(home, ".codex")
}

// ID is the plugin as both runtimes name it.
const ID = "mellions@mellions"

// marketplaceOf is the marketplace half of a plugin id: the registry keys
// plugins as <plugin>@<marketplace>, and the marketplace is what decides
// whether the runtime made a copy.
func marketplaceOf(id string) string {
	if i := strings.LastIndexByte(id, '@'); i >= 0 {
		return id[i+1:]
	}
	return id
}

// Marketplace is the runtime's record of where a plugin's source is, and what
// kind of source it is.
type Marketplace struct {
	Name string
	// Source is the kind the runtime recorded: "directory" for a path on this
	// machine, "github", "git" and the rest for one it fetched.
	Source string
	// Location is where the runtime resolved that source to. For a directory
	// source it is that directory; for a fetched one it is the clone the
	// runtime keeps under its own configuration.
	Location string
	// Problem is why the marketplace registry could not answer, and is empty
	// when it did. A marketplace that could not be read is not a marketplace
	// that copies: the two are told apart here so nothing downstream reports a
	// copy it did not establish.
	Problem string
}

// InPlace reports whether the runtime reads this marketplace's plugins from
// the source itself rather than from a copy.
//
// A marketplace added from a directory is read in place: hooks, Skills,
// commands and agents are loaded out of that directory, so what is committed
// there is what the next session runs, and the copy the plugin registry names
// is written once and never read. Every other source is fetched and copied,
// and it is the copy that loads.
func (m Marketplace) InPlace() bool { return m.Source == "directory" }

// Registration is what the runtime's registry claims about a plugin: what the
// next process it launches will load. Every field is what a file said, not
// what a session has.
type Registration struct {
	// Installed is whether the registry names the plugin at all.
	Installed bool
	// InstallPath is the copy the runtime made when it installed the plugin.
	// It is what loads only when the marketplace is one the runtime fetched;
	// for a directory marketplace the copy is written once and never read.
	InstallPath string
	Version     string
	// Marketplace is the runtime's record of where the plugin's source is, and
	// LoadPath is where the runtime will therefore read this plugin's hooks,
	// Skills, commands and agent from. LoadPath is the answer to "what will
	// the next session actually run"; InstallPath is not.
	Marketplace Marketplace
	LoadPath    string
	// Registered is when the registry last recorded this installation. A
	// process that began before it was launched against a different
	// registration — or none — and carries whatever that one gave it.
	Registered time.Time
	// Enabled is the settings key that decides whether an installed plugin is
	// loaded at all. Installed and disabled loads nothing.
	Enabled   bool
	EnabledIn string
	// HooksFile is the hooks manifest inside the installed copy, and Hooks is
	// what it declares for SessionStart, named the way the runtime records a
	// hook it ran: the statusMessage where one is declared, the command
	// otherwise.
	HooksFile string
	Hooks     []string
	// Problems are the reasons this registration would not deliver the plugin
	// to the next process. Empty means the registry, the settings and the
	// installed copy agree.
	Problems []string
}

// Ready reports whether the next process the runtime launches will load this
// plugin, as far as the runtime's own files can say.
func (r Registration) Ready() bool {
	return r.Installed && r.Enabled && len(r.Hooks) > 0 && len(r.Problems) == 0
}

// Read answers what the Claude Code registry under home claims about a plugin.
//
// An unreadable or absent file is a problem recorded, never a plugin reported
// absent: "the registry does not name it" and "the registry could not be read"
// are different facts and a session acts differently on each.
func Read(home, id string) Registration {
	r := Registration{}
	reg := filepath.Join(configRoot(home), "plugins", "installed_plugins.json")
	raw, err := os.ReadFile(reg)
	if err != nil {
		r.Problems = append(r.Problems, "cannot read "+reg+": "+err.Error())
		return r
	}
	var v struct {
		Plugins map[string][]struct {
			InstallPath string `json:"installPath"`
			Version     string `json:"version"`
			InstalledAt string `json:"installedAt"`
			LastUpdated string `json:"lastUpdated"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		r.Problems = append(r.Problems, "cannot parse "+reg+": "+err.Error())
		return r
	}
	entries, ok := v.Plugins[id]
	if !ok || len(entries) == 0 {
		r.Problems = append(r.Problems, reg+" does not name "+id+" — `mellions install`")
		return r
	}
	e := entries[0]
	r.Installed = true
	r.InstallPath = e.InstallPath
	r.Version = e.Version
	// lastUpdated is when this registration became the current one; installedAt
	// is when the first one did. A reinstall moves the former and not the
	// latter, and it is the former a running process is behind.
	for _, s := range []string{e.LastUpdated, e.InstalledAt} {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			r.Registered = t.UTC()
			break
		}
	}

	r.Enabled, r.EnabledIn = enabled(home, id)
	if !r.Enabled && r.EnabledIn != "" {
		r.Problems = append(r.Problems,
			"installed and explicitly disabled: "+r.EnabledIn+
				" sets enabledPlugins["+id+"] to false, so the runtime loads nothing from it")
	}

	// A marketplace record that cannot be read leaves the load path
	// unestablished, and is deliberately not a Problem: Problems are reasons
	// the plugin would not reach the next process, and an unknown load path is
	// not one of them — the registry names it and the settings enable it, so it
	// loads from somewhere. Recording it here would report a healthy
	// installation as broken on any host whose marketplace registry is absent
	// or shaped differently.
	r.Marketplace = readMarketplace(home, marketplaceOf(id))
	r.LoadPath = r.InstallPath
	if r.Marketplace.InPlace() && r.Marketplace.Location != "" {
		r.LoadPath = pluginDir(r.Marketplace.Location, pluginOf(id))
	}

	if r.LoadPath == "" {
		r.Problems = append(r.Problems, "neither registry names a path to load from")
		return r
	}
	r.HooksFile = filepath.Join(r.LoadPath, "hooks", "hooks.json")
	hooks, err := sessionStartHooks(r.HooksFile)
	switch {
	case err != nil:
		r.Problems = append(r.Problems, "the path the runtime loads from has no readable "+
			filepath.Join("hooks", "hooks.json")+": "+err.Error()+
			" — it points at a copy that is gone or incomplete; `mellions install`")
	case len(hooks) == 0:
		r.Problems = append(r.Problems, r.HooksFile+" declares no SessionStart hooks")
	default:
		r.Hooks = hooks
	}
	return r
}

// enabled reads the settings key that can disable an installed plugin.
//
// An absent key is not a disabled plugin: the runtime enables an installed
// plugin unless something says otherwise, so only an explicit false disables
// one. The second return names the file that decided, and is empty when
// nothing did — which is how "nobody said" is told from "somebody said yes",
// and why an unset key is reported enabled rather than as a problem.
//
// Only the user scope is read here. The runtime also merges project and local
// settings and a policy layer, so a plugin enabled or disabled there is not
// visible in this answer.
func enabled(home, id string) (bool, string) {
	on := true
	var in string
	for _, name := range []string{"settings.json", "settings.local.json"} {
		p := filepath.Join(configRoot(home), name)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var v struct {
			EnabledPlugins map[string]bool `json:"enabledPlugins"`
		}
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		if b, ok := v.EnabledPlugins[id]; ok {
			on, in = b, p
		}
	}
	return on, in
}

// readMarketplace answers what the runtime's marketplace registry says about
// where a marketplace's source is.
//
// This is the record that decides whether a change in a checkout reaches a
// session. The plugin registry alone cannot say: it names the copy the
// installer wrote either way, and that copy is read only for a marketplace the
// runtime fetched.
func readMarketplace(home, name string) Marketplace {
	m := Marketplace{Name: name}
	p := filepath.Join(configRoot(home), "plugins", "known_marketplaces.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		m.Problem = "cannot read " + p + ": " + err.Error() +
			" — where the runtime loads this plugin from is unestablished"
		return m
	}
	var v map[string]struct {
		Source struct {
			Source string `json:"source"`
			Path   string `json:"path"`
		} `json:"source"`
		InstallLocation string `json:"installLocation"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		m.Problem = "cannot parse " + p + ": " + err.Error() +
			" — where the runtime loads this plugin from is unestablished"
		return m
	}
	e, ok := v[name]
	if !ok {
		m.Problem = p + " does not name the " + name + " marketplace, so where the" +
			" runtime loads this plugin from is unestablished — `mellions install`"
		return m
	}
	m.Source = e.Source.Source
	m.Location = e.InstallLocation
	if m.Location == "" {
		m.Location = e.Source.Path
	}
	return m
}

// pluginOf is the plugin half of a plugin id.
func pluginOf(id string) string {
	if i := strings.LastIndexByte(id, '@'); i >= 0 {
		return id[:i]
	}
	return id
}

// pluginDir is where a plugin's files sit inside its marketplace.
//
// A marketplace declares each plugin's own source, which is a path relative to
// the marketplace root — usually "./", and not always. Assuming the root
// instead would name a directory with no hooks manifest in it, and the whole
// registration then reads as broken: no hooks, no bearing Skills, and a
// session correctly running the plugin told it is not.
//
// A manifest that cannot be read, or a plugin whose source is not a relative
// path, leaves the root — the only other answer available, and the right one
// for the ordinary "./".
func pluginDir(root, name string) string {
	raw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if err != nil {
		return root
	}
	var v struct {
		Plugins []struct {
			Name   string          `json:"name"`
			Source json.RawMessage `json:"source"`
		} `json:"plugins"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return root
	}
	for _, p := range v.Plugins {
		if p.Name != name {
			continue
		}
		var rel string
		if json.Unmarshal(p.Source, &rel) != nil || rel == "" {
			return root // an object source: the plugin is not in this tree
		}
		if filepath.IsAbs(rel) {
			return filepath.Clean(rel)
		}
		return filepath.Join(root, rel)
	}
	return root
}

// sessionStartHooks names every SessionStart hook a manifest declares, the way
// the runtime records a hook it ran: its statusMessage where one is declared,
// its command otherwise. Naming them the same way is what lets a declaration
// be compared against a session's transcript at all.
func sessionStartHooks(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command       string `json:"command"`
				StatusMessage string `json:"statusMessage"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range v.Hooks["SessionStart"] {
		for _, h := range m.Hooks {
			if h.StatusMessage != "" {
				out = append(out, h.StatusMessage)
				continue
			}
			if h.Command != "" {
				out = append(out, h.Command)
			}
		}
	}
	return out, nil
}

// Tree is the state of the git checkout a runtime loads a plugin from in
// place. Where the runtime reads the plugin out of a checkout, what is
// committed there is what the next session runs, so the commit it stands at,
// whether anything is uncommitted, and how it stands against its upstream in
// both directions are what "which release is deployed on this host" means —
// and `git pull` there is the deployment.
type Tree struct {
	Head string
	// Branch is meaningful only when BranchKnown. Off a branch git answers the
	// literal "HEAD"; an empty Branch is a read that failed, and reporting that
	// as a detached HEAD claims a state nobody established.
	Branch      string
	BranchKnown bool
	// Dirty is meaningful only when StatusKnown. A status that could not be
	// read leaves Dirty false, and reporting that as a clean tree would print
	// the one word a reader acts on from a command that failed.
	Dirty       bool
	StatusKnown bool
	Upstream    string
	Behind      int
	// Ahead is commits the checkout has that its upstream does not. Behind
	// alone reads a diverged checkout as healthy: `git pull --ff-only` says
	// "Already up to date" while Ahead is non-zero and the upstream has not
	// moved, and refuses the moment it does, so the state is invisible until
	// the pull that would have deployed a change is the one that fails.
	Ahead int
	// Problems are why parts of the above are unknown, and empty when all of
	// it was read. A checkout with no upstream is not a checkout that is up to
	// date, and the two are not collapsed.
	Problems []string
}

// ReadTree reads the git state of dir. The second return is false when dir is
// not the root of a git checkout, which is the ordinary case for a copy the
// runtime made and not a fault.
func ReadTree(dir string) (Tree, bool) {
	if dir == "" {
		return Tree{}, false
	}
	if out, err := git(dir, "rev-parse", "--show-toplevel"); err != nil || out == "" {
		return Tree{}, false
	}
	var t Tree
	head, err := git(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return Tree{Problems: []string{"cannot read HEAD: " + err.Error()}}, true
	}
	t.Head = head
	// Off a branch this answers the literal "HEAD", so an empty Branch means
	// the read failed and never means "no branch". Discarding the error made
	// those two indistinguishable, and a reader told a checkout is detached
	// acts on it.
	if b, err := git(dir, "rev-parse", "--abbrev-ref", "HEAD"); err != nil {
		t.Problems = append(t.Problems, "cannot read the branch, so whether it is on one is unknown: "+err.Error())
	} else {
		t.Branch, t.BranchKnown = b, true
	}
	status, err := git(dir, "status", "--porcelain")
	if err != nil {
		t.Problems = append(t.Problems, "cannot read the working tree, so whether it is dirty is unknown: "+err.Error())
	} else {
		t.Dirty, t.StatusKnown = status != "", true
	}
	up, err := git(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || up == "" {
		t.Problems = append(t.Problems, "no upstream is configured, so whether it is behind or ahead is unknown")
		return t, true
	}
	t.Upstream = up
	behind, err := git(dir, "rev-list", "--count", "HEAD.."+up)
	if err != nil {
		t.Problems = append(t.Problems, "cannot count commits behind "+up+": "+err.Error())
		return t, true
	}
	n, err := strconv.Atoi(behind)
	if err != nil {
		t.Problems = append(t.Problems, "cannot read the count of commits behind "+up)
		return t, true
	}
	t.Behind = n
	ahead, err := git(dir, "rev-list", "--count", up+"..HEAD")
	if err != nil {
		t.Problems = append(t.Problems, "cannot count commits ahead of "+up+": "+err.Error())
		return t, true
	}
	n, err = strconv.Atoi(ahead)
	if err != nil {
		t.Problems = append(t.Problems, "cannot read the count of commits ahead of "+up)
		return t, true
	}
	t.Ahead = n
	return t, true
}

// git runs one read-only query against a checkout.
//
// GIT_OPTIONAL_LOCKS=0 is what keeps this a read: `git status` otherwise
// refreshes and rewrites the index, and doctor is not allowed to change the
// tree it is reporting on — nor to block behind another process's index lock.
// GIT_TERMINAL_PROMPT=0 stops a repository with a credential helper from
// hanging a diagnostic on a password prompt. Stderr is folded into the error
// because `exit status 128` alone names no cause, and a reader cannot act on it.
func git(dir string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		if why := strings.TrimSpace(errb.String()); why != "" {
			return "", fmt.Errorf("%w: %s", err, firstLine(why))
		}
	}
	return strings.TrimSpace(string(out)), err
}

// firstLine keeps a multi-line git failure to the part that names the cause.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// CodexTrust is how many of a plugin's hooks Codex has been told it may run.
//
// Codex runs no hook it has not been trusted with, and trusting is the
// runtime's own prompt to a person: nothing here can grant it. A plugin whose
// hooks are untrusted still delivers its Skills, so the session has every
// method and does not know who it is — an installation that looks complete
// from every other angle. Reporting the count is what makes that visible.
type CodexTrust struct {
	// Declared is the hook entries the manifest declares, across every event.
	// Trusted is how many of them config.toml records a trusted hash for.
	Declared int
	Trusted  int
	Config   string
	// Manifest is the hooks file Declared was counted from, and Named is the
	// hooks files a trust key was matched against. Codex materializes a plugin
	// into a cache of its own even from a local source, so the key can name
	// either the source or that copy, and matching only one would be a count
	// that can never come back positive.
	Manifest string
	Named    []string
	Problem  string
}

// Short reports whether Codex is missing trust for any hook the plugin
// declares, which is the state in which a Codex session is not Mellions.
func (c CodexTrust) Short() bool { return c.Declared > 0 && c.Trusted < c.Declared }

// ReadCodexTrust counts a plugin's declared hooks against the trust Codex has
// persisted for them.
//
// Codex keys trust per hook entry, under the hooks file that declares it:
// [hooks.state."<hooks.json path>:<event>:<i>:<j>"]. The event token is
// Codex's own spelling and the indices are positions in that file, so nothing
// here reconstructs them — a key is this plugin's when the path in front of
// the first colon after it is one of this plugin's hooks manifests, which is
// the part that does not change with Codex's serialisation.
func ReadCodexTrust(home, id, loadPath string) CodexTrust {
	root := codexRoot(home)
	t := CodexTrust{Config: filepath.Join(root, "config.toml")}

	name, market := id, marketplaceOf(id)
	if i := strings.LastIndexByte(id, '@'); i >= 0 {
		name = id[:i]
	}
	if loadPath != "" {
		t.Named = append(t.Named, filepath.Join(loadPath, "hooks", "hooks.json"))
	}
	cached, _ := filepath.Glob(filepath.Join(root, "plugins", "cache", market, name, "*", "hooks", "hooks.json"))
	t.Named = append(t.Named, cached...)
	if len(t.Named) == 0 {
		t.Problem = "no hooks manifest for " + id + " was found to count, in the path the runtime loads from or in Codex's own cache"
		return t
	}

	for _, p := range t.Named {
		n, err := countHooks(p)
		if err != nil {
			continue
		}
		t.Manifest, t.Declared = p, n
		break
	}
	if t.Manifest == "" {
		t.Problem = "none of " + strings.Join(t.Named, ", ") + " is a readable hooks manifest"
		return t
	}

	raw, err := os.ReadFile(t.Config)
	if err != nil {
		// Codex not installed, or installed and never run, is not a plugin
		// with untrusted hooks: nothing can be counted and that is said.
		t.Problem = "cannot read " + t.Config + ": " + err.Error()
		return t
	}
	// Codex keys a plugin's hooks by the plugin id and the manifest's path
	// inside the plugin — `mellions@mellions:hooks/hooks.json:session_start:0:0`
	// — and a project's hooks by the manifest's absolute path. Both shapes are
	// candidates; an owner who trusted every hook was read as 0 of 12 when
	// only the path shape was.
	prefixes := append([]string{id + ":hooks/hooks.json"}, t.Named...)
	// Counted per candidate rather than summed. A host whose checkout moved
	// keeps the entries trusted under the old path, and adding the two would
	// report more hooks trusted than the manifest declares — the trust that
	// applies is the one under a single key, so the fullest of them is it.
	per := make(map[string]int, len(prefixes))
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `[hooks.state."`) || !strings.HasSuffix(line, `"]`) {
			continue
		}
		key := line[len(`[hooks.state."`) : len(line)-len(`"]`)]
		for _, p := range prefixes {
			if strings.HasPrefix(key, p+":") {
				per[p]++
				break
			}
		}
	}
	for _, n := range per {
		if n > t.Trusted {
			t.Trusted = n
		}
	}
	return t
}

// countHooks is every hook entry a manifest declares, across every event. It
// is the denominator Codex's per-entry trust is counted against, so it counts
// what Codex keys — one entry per command, in every event and every matcher
// group — and not the events or the groups.
func countHooks(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var v struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, err
	}
	n := 0
	for _, groups := range v.Hooks {
		for _, g := range groups {
			n += len(g.Hooks)
		}
	}
	return n, nil
}

// Load is what the runtime recorded about one session: which SessionStart
// hooks it actually ran, and how many of them were this plugin's.
type Load struct {
	SessionID  string
	Transcript string
	Events     []Event
	// Exported is the session id the runtime exported, where that is not the
	// id the transcript is written under. Empty when the two agree, which is
	// every session that was not resumed.
	Exported string
	// Ambiguous names every transcript that could equally be this process's,
	// where more than one was launched into the same directory inside the same
	// window. Non-empty means the transcript above was chosen by the exported
	// id alone and may be the wrong file.
	Ambiguous []string
}

// Event is one SessionStart the runtime ran for a session — its launch, and
// again on every /clear, /compact and resume.
type Event struct {
	At    time.Time
	Kind  string // startup, clear, compact, resume
	Ours  int
	Total int
}

// Has reports whether the session is running this plugin now, which is decided
// by the most recent SessionStart and by that one alone.
//
// A resumed conversation keeps its session id and appends to the same
// transcript, so the record of a session that followed the advice to start a
// new process contains both the launch that lacked the plugin and the resume
// that has it. Requiring every event to carry the plugin would report that
// session as still lacking it — the remedy could never clear the diagnosis,
// and the one state this is for would be the one it gets wrong.
func (l Load) Has() bool {
	last, ok := l.Latest()
	return ok && last.Ours > 0
}

// Latest is the most recent SessionStart the runtime ran, which is the one
// whose context the session is carrying now.
func (l Load) Latest() (Event, bool) {
	if len(l.Events) == 0 {
		return Event{}, false
	}
	return l.Events[len(l.Events)-1], true
}

// ReadLoad reads the runtime's own transcript for a session and reports which
// SessionStart hooks it ran.
//
// ours is the set of hook names the current registration declares. A session
// launched under an older registration ran hooks under older names and is
// counted as having none of these — which is the truth being asked for: not
// "some Mellions once", but "the hooks this installation declares".
//
// The second return is false when no transcript for the session was found,
// which is not the same as a session that ran no hooks.
func ReadLoad(home, sessionID string, ours []string) (Load, bool) {
	if strings.TrimSpace(sessionID) == "" {
		return Load{}, false
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl"))
	if len(matches) == 0 {
		return Load{}, false
	}
	sort.Strings(matches)
	return readTranscript(matches[0], sessionID, hookSet(ours))
}

func hookSet(ours []string) map[string]bool {
	mine := make(map[string]bool, len(ours))
	for _, h := range ours {
		mine[h] = true
	}
	return mine
}

func readTranscript(path, sessionID string, mine map[string]bool) (Load, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Load{}, false
	}
	defer f.Close()
	l := Load{SessionID: sessionID, Transcript: path}
	l.Events = scanSessionStarts(f, mine)
	return l, true
}

// launchBefore and launchAfter bound where a process's own launch SessionStart
// sits relative to when the process began.
//
// A runtime runs SessionStart at launch, so the attachment lands within
// seconds: measured on this host, the process began at 17:46:51Z and its
// SessionStart attachments carry 17:46:53Z. launchAfter is far wider than that
// because a slow launch costs nothing here and a narrow bound would miss the
// real transcript; launchBefore absorbs `ps -o etime`'s one-second resolution,
// which is how a process's start time is read, and any small difference between
// the clock that wrote the timestamp and the one that read the age.
const (
	launchBefore = 90 * time.Second
	launchAfter  = 5 * time.Minute
)

// launched reports whether this Load carries a SessionStart from the launch of
// a process that began at procStart.
func (l Load) launched(procStart time.Time) bool {
	lo, hi := procStart.Add(-launchBefore), procStart.Add(launchAfter)
	for _, e := range l.Events {
		if !e.At.Before(lo) && !e.At.After(hi) {
			return true
		}
	}
	return false
}

// ReadLive reads the load of the transcript THIS process is writing, which
// after `claude --resume` is not the one the exported session id names.
//
// The runtime keeps exporting the original conversation's id while the resumed
// process writes its transcript under a new one — the same two-id shape
// presence folds with CLAUDE_PID. Reading the exported id's file then reports
// the last SessionStart of a file nothing is writing any more, as though it
// were this session's: the wrong evidence, under a verdict that may still read
// as plausible.
//
// What separates the two files is time. This process ran SessionStart at
// launch, so the transcript it writes carries one at that moment and the file
// it was forked from carries none. procStart is when this process began —
// presence.SelfStarted() reads it — and a zero value (no pid exported, or a
// process the kernel no longer knows) means nothing here can be established, so
// the exported id is used unchanged rather than guessed past.
//
// Where more than one transcript in the directory was launched inside the same
// window — two sessions opened in one tree within minutes — this does not pick
// one. It returns the exported id's load with every candidate named in
// Ambiguous, because a confident answer from the wrong peer's file is worse
// than the stale one it replaced.
func ReadLive(home, cwd, sessionID string, procStart time.Time, ours []string) (Load, bool) {
	exported, ok := ReadLoad(home, sessionID, ours)
	if procStart.IsZero() {
		return exported, ok
	}
	// The common case, and every session that was not resumed: the exported
	// id's own transcript carries this process's launch.
	if ok && exported.launched(procStart) {
		return exported, true
	}

	dir := ""
	if ok {
		dir = filepath.Dir(exported.Transcript)
	} else if d := projectDir(home, cwd); d != "" {
		dir = d
	}
	if dir == "" {
		return exported, ok
	}

	mine := hookSet(ours)
	var found []Load
	paths, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, p := range paths {
		// A file last written before this process began cannot hold a
		// SessionStart from its launch, and skipping it keeps this from
		// reading every transcript in a busy tree.
		if st, err := os.Stat(p); err != nil || st.ModTime().Before(procStart.Add(-launchBefore)) {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(p), ".jsonl")
		l, read := readTranscript(p, id, mine)
		if read && l.launched(procStart) {
			found = append(found, l)
		}
	}
	switch len(found) {
	case 0:
		return exported, ok
	case 1:
		live := found[0]
		if live.SessionID != sessionID {
			live.Exported = sessionID
		}
		return live, true
	default:
		for _, l := range found {
			exported.Ambiguous = append(exported.Ambiguous, l.Transcript)
		}
		return exported, ok
	}
}

// projectDir is where the runtime keeps the transcripts for a working
// directory. The runtime's own mangling is not documented, so this derives the
// name and then requires it to exist: a rule that is wrong for some path
// resolves to nothing and reports unestablished, rather than to a directory
// belonging to a different tree.
func projectDir(home, cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return ""
	}
	name := []rune(cwd)
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			name[i] = '-'
		}
	}
	dir := filepath.Join(configRoot(home), "projects", string(name))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return dir
}

// scanSessionStarts folds a transcript's hook attachments into one Event per
// SessionStart the runtime ran.
//
// The runtime writes one attachment per hook it ran, all sharing the
// SessionStart's toolUseID, and a second, aggregate attachment carrying no
// command. The aggregate is skipped: counting it would report a hook nobody
// ran. Failures count too — a hook that ran and failed is still a hook the
// runtime knew about, and the two are told apart by what it delivered, not by
// whether it was invoked.
func scanSessionStarts(f *os.File, mine map[string]bool) []Event {
	type agg struct {
		at    time.Time
		kind  string
		ours  int
		total int
		seq   int
	}
	byID := map[string]*agg{}
	var order []string
	dec := json.NewDecoder(f)
	for {
		var rec struct {
			Timestamp  string `json:"timestamp"`
			Attachment *struct {
				HookEvent string `json:"hookEvent"`
				HookName  string `json:"hookName"`
				ToolUseID string `json:"toolUseID"`
				Command   string `json:"command"`
			} `json:"attachment"`
		}
		if err := dec.Decode(&rec); err != nil {
			break
		}
		a := rec.Attachment
		if a == nil || a.HookEvent != "SessionStart" || a.Command == "" {
			continue
		}
		key := a.ToolUseID + "\x00" + a.HookName
		e, ok := byID[key]
		if !ok {
			at, _ := time.Parse(time.RFC3339, rec.Timestamp)
			e = &agg{at: at.UTC(), kind: kindOf(a.HookName), seq: len(order)}
			byID[key] = e
			order = append(order, key)
		}
		e.total++
		if mine[a.Command] {
			e.ours++
		}
	}
	out := make([]Event, 0, len(order))
	for _, k := range order {
		e := byID[k]
		out = append(out, Event{At: e.at, Kind: e.kind, Ours: e.ours, Total: e.total})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func kindOf(hookName string) string {
	if i := strings.IndexByte(hookName, ':'); i >= 0 {
		return hookName[i+1:]
	}
	return hookName
}

// ContextSince is how much transcript a session has written since the runtime
// last compacted it: the bytes after the newest compaction marker, or the whole
// file when there is none. Zero, false when no transcript for the session is on
// disk. The scan is the only cost, and it runs only when the caller asks — a
// session under the size that matters is never read.
func ContextSince(home, sessionID string) (int64, bool) {
	if strings.TrimSpace(sessionID) == "" {
		return 0, false
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl"))
	if len(matches) == 0 {
		return 0, false
	}
	sort.Strings(matches)
	return ContextSinceIn(matches[0])
}

// ContextSinceIn is ContextSince for a transcript the caller already knows.
func ContextSinceIn(path string) (int64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	var off, last int64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		off += int64(len(line)) + 1
		if bytes.Contains(line, []byte(`"compact_boundary"`)) || bytes.Contains(line, []byte(`"isCompactSummary":true`)) {
			last = off
		}
	}
	return off - last, true
}

// TranscriptSize is the transcript's size on disk, or zero, false when there is
// none — the cheap question asked before ContextSince's scan.
func TranscriptSize(home, sessionID string) (int64, bool) {
	if strings.TrimSpace(sessionID) == "" {
		return 0, false
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl"))
	if len(matches) == 0 {
		return 0, false
	}
	sort.Strings(matches)
	fi, err := os.Stat(matches[0])
	if err != nil {
		return 0, false
	}
	return fi.Size(), true
}

// CompactionSize is how much transcript this host's runtime has let a session
// of the given model accumulate before compacting it on its own: the median
// bytes between automatic compaction boundaries, over the newest transcripts
// under the runtime's projects directory whose model is the same. The size
// follows the model's window, not the host, so each session is measured against
// transcripts from the same model. Zero, 0 when none have been observed.
func CompactionSize(home, model string) (bytes int64, samples int) {
	matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
	type f struct {
		path string
		mod  int64
	}
	var files []f
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil || fi.Size() < 500_000 {
			continue
		}
		files = append(files, f{m, fi.ModTime().Unix()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	if len(files) > 60 {
		files = files[:60]
	}
	var gaps []int64
	for _, x := range files {
		m, g := autoCompactionGaps(x.path)
		if m == model {
			gaps = append(gaps, g...)
		}
	}
	if len(gaps) == 0 {
		return 0, 0
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	return gaps[len(gaps)/2], len(gaps)
}

// TranscriptModel is the model a transcript's assistant turns name, or empty.
func TranscriptModel(path string) string {
	m, _ := autoCompactionGapsUntil(path, true)
	return m
}

// autoCompactionGaps is the transcript's model and the bytes before each of
// its automatic compactions. A manual /compact says what a person chose, not
// what the runtime tolerates, and a gap under 200 KB is a marker written twice.
func autoCompactionGaps(path string) (string, []int64) {
	return autoCompactionGapsUntil(path, false)
}

var modelKey = []byte(`"model":"`)

func autoCompactionGapsUntil(path string, modelOnly bool) (string, []int64) {
	fh, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer fh.Close()
	var out []int64
	var off, last int64
	model := ""
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		off += int64(len(line)) + 1
		if model == "" {
			if i := bytes.Index(line, modelKey); i >= 0 {
				rest := line[i+len(modelKey):]
				if j := bytes.IndexByte(rest, '"'); j > 0 {
					model = string(rest[:j])
					if modelOnly {
						return model, nil
					}
				}
			}
		}
		if !bytes.Contains(line, []byte(`"compact_boundary"`)) {
			continue
		}
		if bytes.Contains(line, []byte(`"trigger":"auto"`)) || bytes.Contains(line, []byte(`"trigger": "auto"`)) {
			if gap := off - last; gap >= 200_000 {
				out = append(out, gap)
			}
		}
		last = off
	}
	return model, out
}

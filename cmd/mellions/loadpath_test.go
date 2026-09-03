// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
)

// hooksManifest is three hook entries across two events and two matcher
// groups, which is the shape Codex keys its trust against: one key per entry,
// not per event and not per group.
const hooksManifest = `{
  "hooks": {
    "SessionStart": [
      {"matcher": "startup", "hooks": [
        {"type": "command", "command": "bash a.sh", "statusMessage": "a"},
        {"type": "command", "command": "bash b.sh", "statusMessage": "b"}
      ]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "bash c.sh"}]}
    ]
  }
}`

func writeUnder(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeRuntime writes the two records the runtime keeps about a plugin, and the
// plugin's own hooks manifest at both candidate paths, so a test can ask where
// the runtime would load from without touching the real installation.
func fakeRuntime(t *testing.T, source, marketPath, installPath string) (home string) {
	t.Helper()
	home = t.TempDir()
	// An installation that sets this in the environment reads a different
	// directory, and a test that inherited it would be reading the host's.
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	plugins := filepath.Join(home, ".claude", "plugins")
	writeUnder(t, filepath.Join(plugins, "installed_plugins.json"), `{
  "version": 2,
  "plugins": {"mellions@mellions": [{"installPath": "`+installPath+`",
    "version": "0.1.0", "lastUpdated": "2026-08-28T18:53:54.220Z"}]}
}`)
	writeUnder(t, filepath.Join(plugins, "known_marketplaces.json"), `{
  "mellions": {"source": {"source": "`+source+`", "path": "`+marketPath+`"},
    "installLocation": "`+marketPath+`"}
}`)
	writeUnder(t, filepath.Join(marketPath, "hooks", "hooks.json"), hooksManifest)
	writeUnder(t, filepath.Join(installPath, "hooks", "hooks.json"), hooksManifest)
	return home
}

// Where the runtime reads a plugin from is decided by the marketplace record,
// not by the plugin record — and the plugin record names a path either way,
// which is how reading it alone reports a copy that nothing loads.
func TestTheLoadPathIsTheMarketplacesAnswerAndNotTheCopys(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		wantCopy bool   // the load path is the installer's copy
		wantIn   string // a phrase the line must carry
	}{{
		name:   "a directory marketplace is read in place, so the checkout is what loads",
		source: "directory",
		wantIn: "read in place",
	}, {
		name:     "a github marketplace is fetched and copied, so the copy is what loads",
		source:   "github",
		wantCopy: true,
		wantIn:   "a copy",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			market := filepath.Join(root, "checkout")
			install := filepath.Join(root, "cache", "mellions", "0.1.0")
			home := fakeRuntime(t, c.source, market, install)

			reg := pluginreg.Read(home, pluginreg.ID)
			if !reg.Installed {
				t.Fatalf("not installed: %v", reg.Problems)
			}
			want := market
			if c.wantCopy {
				want = install
			}
			if reg.LoadPath != want {
				t.Errorf("LoadPath = %q, want %q", reg.LoadPath, want)
			}
			// The hooks manifest must be read from the path that loads, or the
			// hook names a session is judged against come from a copy nothing
			// ran.
			if got := filepath.Join(want, "hooks", "hooks.json"); reg.HooksFile != got {
				t.Errorf("HooksFile = %q, want %q", reg.HooksFile, got)
			}
			if reg.InstallPath != install {
				t.Errorf("InstallPath = %q, want the copy %q", reg.InstallPath, install)
			}

			state, detail := loadPathState(reg)
			if state != "present" {
				t.Errorf("state = %q, want present (%s)", state, detail)
			}
			if !strings.Contains(detail, want) {
				t.Errorf("line does not name the load path %q: %s", want, detail)
			}
			if !strings.Contains(detail, c.wantIn) {
				t.Errorf("line does not say %q: %s", c.wantIn, detail)
			}
			if !strings.Contains(detail, "known_marketplaces.json") {
				t.Errorf("line does not say which record established it: %s", detail)
			}
			// The copy is named as inert only where it is one. Saying it of a
			// fetched marketplace would deny the path that actually loads.
			if got := strings.Contains(detail, "never read"); got == c.wantCopy {
				t.Errorf("inert-copy phrasing = %v for a %s source: %s", got, c.source, detail)
			}
		})
	}
}

// A marketplace record that cannot be read is not a marketplace that copies.
// Reporting the copy as established there is the failure this whole line
// exists to stop, one register down.
func TestAnUnreadableMarketplaceRecordIsUnknownAndNotACopy(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	install := filepath.Join(root, "cache")
	home := fakeRuntime(t, "directory", market, install)
	if err := os.Remove(filepath.Join(home, ".claude", "plugins", "known_marketplaces.json")); err != nil {
		t.Fatal(err)
	}

	reg := pluginreg.Read(home, pluginreg.ID)
	if reg.LoadPath != install {
		t.Errorf("LoadPath = %q, want the copy %q as the only path left", reg.LoadPath, install)
	}
	state, detail := loadPathState(reg)
	if state != "unknown" {
		t.Errorf("state = %q, want unknown: %s", state, detail)
	}
	if !strings.Contains(detail, "known_marketplaces.json") {
		t.Errorf("line does not name the record it could not read: %s", detail)
	}
}

// The key Codex actually writes for a plugin's hooks on this host is the
// plugin id and the manifest's path inside the plugin, not an absolute path:
// an owner who trusted all twelve read "0 of 12 trusted" until this was
// matched.
func TestCodexTrustKeyedByPluginIDIsCounted(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	install := filepath.Join(root, "cache")
	home := fakeRuntime(t, "directory", market, install)
	codexConfig(t, home,
		pluginreg.ID+":hooks/hooks.json:session_start:0:0",
		pluginreg.ID+":hooks/hooks.json:session_start:0:1",
		pluginreg.ID+":hooks/hooks.json:pre_tool_use:0:0",
		"other@market:hooks/hooks.json:session_start:0:0",
	)
	tr := pluginreg.ReadCodexTrust(home, pluginreg.ID, market)
	if tr.Problem != "" {
		t.Fatalf("problem: %s", tr.Problem)
	}
	if tr.Declared != 3 || tr.Trusted != 3 {
		t.Fatalf("Trusted = %d of %d, want 3 of 3 from plugin-id keys; another plugin's key must not count", tr.Trusted, tr.Declared)
	}
	if state, detail := codexTrustState(tr); state != "present" {
		t.Errorf("state = %q, want present: %s", state, detail)
	}
}

func codexConfig(t *testing.T, home string, keys ...string) {
	t.Helper()
	writeCodexConfig(t, filepath.Join(home, ".codex", "config.toml"), keys...)
}

func writeCodexConfig(t *testing.T, path string, keys ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("model = \"gpt-5\"\n\n[hooks.state]\n")
	for _, k := range keys {
		b.WriteString("\n[hooks.state.\"" + k + "\"]\ntrusted_hash = \"sha256:0ed786\"\n")
	}
	writeUnder(t, path, b.String())
}

func TestCodexTrustHonoursCODEXHOME(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	home := fakeRuntime(t, "directory", market, filepath.Join(root, "cache"))
	codexHome := filepath.Join(root, "codex-state")
	t.Setenv("CODEX_HOME", codexHome)
	manifest := filepath.Join(market, "hooks", "hooks.json")
	writeCodexConfig(t, filepath.Join(codexHome, "config.toml"),
		manifest+":session_start:0:0",
		manifest+":session_start:0:1",
		manifest+":pre_tool_use:0:0",
	)

	tr := pluginreg.ReadCodexTrust(home, pluginreg.ID, market)
	if tr.Config != filepath.Join(codexHome, "config.toml") {
		t.Fatalf("Config = %q, want the CODEX_HOME config", tr.Config)
	}
	if tr.Problem != "" || tr.Trusted != 3 || tr.Declared != 3 {
		t.Fatalf("trust under CODEX_HOME = %d/%d (%s), want 3/3", tr.Trusted, tr.Declared, tr.Problem)
	}
}

// Codex runs no hook it has not been trusted with, and loads the Skills
// regardless — so an untrusted installation looks complete everywhere else and
// delivers a session that has every method and does not know who it is.
func TestCodexHookTrustIsCountedPerEntryAgainstThePluginsOwnManifest(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	install := filepath.Join(root, "cache")
	manifest := filepath.Join(market, "hooks", "hooks.json")

	cases := []struct {
		name        string
		keys        []string
		wantTrusted int
		wantState   string
		wantIn      string
	}{{
		name: "every entry trusted is a Codex session that is Mellions",
		keys: []string{
			manifest + ":session_start:0:0",
			manifest + ":session_start:0:1",
			manifest + ":pre_tool_use:0:0",
		},
		wantTrusted: 3,
		wantState:   "present",
		wantIn:      "3 of 3 trusted",
	}, {
		name:        "nothing trusted is named as the state it is",
		keys:        nil,
		wantTrusted: 0,
		wantState:   "partial",
		wantIn:      "0 of 3 trusted — start codex once here and trust them, or a Codex session is not Mellions",
	}, {
		// Partly trusted does not get the "not Mellions" sentence. Which
		// entries are missing is not readable here and the session-start ones
		// may be among the trusted, so claiming the session is not Mellions
		// would be a false alarm on the one line whose value is being acted on.
		name:        "a partial trust is short, but does not claim the session is not Mellions",
		keys:        []string{manifest + ":session_start:0:0"},
		wantTrusted: 1,
		wantState:   "partial",
		wantIn:      "1 of 3 trusted — some are not",
	}, {
		// The negative control. Every project-level hooks file Codex has ever
		// been trusted with sits in this same table, so a count that matched a
		// path it did not establish would report an untrusted plugin trusted
		// by somebody else's entry.
		name: "another file's trusted entries are not this plugin's",
		keys: []string{
			filepath.Join(root, "elsewhere", ".codex", "hooks.json") + ":session_start:0:0",
			filepath.Join(root, "checkout-of-something-else", "hooks", "hooks.json") + ":session_start:0:0",
		},
		wantTrusted: 0,
		wantState:   "partial",
		wantIn:      "0 of 3 trusted",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := fakeRuntime(t, "directory", market, install)
			codexConfig(t, home, c.keys...)

			tr := pluginreg.ReadCodexTrust(home, pluginreg.ID, market)
			if tr.Problem != "" {
				t.Fatalf("problem: %s", tr.Problem)
			}
			if tr.Declared != 3 {
				t.Errorf("Declared = %d, want 3 — one per hook entry, across both events", tr.Declared)
			}
			if tr.Trusted != c.wantTrusted {
				t.Errorf("Trusted = %d, want %d", tr.Trusted, c.wantTrusted)
			}
			state, detail := codexTrustState(tr)
			if state != c.wantState {
				t.Errorf("state = %q, want %q: %s", state, c.wantState, detail)
			}
			if !strings.Contains(detail, c.wantIn) {
				t.Errorf("line = %q, want it to carry %q", detail, c.wantIn)
			}
		})
	}
}

// Codex materializes a plugin into a cache of its own even from a local
// source, so a trust key can name that copy rather than the checkout. Matching
// only the checkout would be a count that can never come back positive on a
// host where Codex is the runtime that was trusted.
func TestTrustIsFoundWhenCodexKeyedItUnderItsOwnCachedCopy(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	install := filepath.Join(root, "cache")
	home := fakeRuntime(t, "directory", market, install)

	cached := filepath.Join(home, ".codex", "plugins", "cache", "mellions", "mellions", "0.1.0", "hooks", "hooks.json")
	writeUnder(t, cached, hooksManifest)
	codexConfig(t, home,
		cached+":session_start:0:0",
		cached+":session_start:0:1",
		cached+":pre_tool_use:0:0",
	)

	tr := pluginreg.ReadCodexTrust(home, pluginreg.ID, market)
	if tr.Trusted != 3 || tr.Declared != 3 {
		t.Fatalf("Trusted/Declared = %d/%d, want 3/3 — the key names Codex's own copy", tr.Trusted, tr.Declared)
	}
	if tr.Short() {
		t.Error("Short() is true with every entry trusted")
	}
}

// A checkout that moved leaves the entries trusted under its old path behind,
// and both paths are matched. Adding them would report more hooks trusted than
// the manifest declares — "6 of 3 trusted" — which is not a state anything can
// be in.
func TestTrustUnderTwoPathsIsNotAddedTogether(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	home := fakeRuntime(t, "directory", market, filepath.Join(root, "cache"))

	here := filepath.Join(market, "hooks", "hooks.json")
	cached := filepath.Join(home, ".codex", "plugins", "cache", "mellions", "mellions", "0.1.0", "hooks", "hooks.json")
	writeUnder(t, cached, hooksManifest)
	codexConfig(t, home,
		here+":session_start:0:0", here+":session_start:0:1", here+":pre_tool_use:0:0",
		cached+":session_start:0:0", cached+":session_start:0:1", cached+":pre_tool_use:0:0",
	)

	tr := pluginreg.ReadCodexTrust(home, pluginreg.ID, market)
	if tr.Trusted != 3 {
		t.Errorf("Trusted = %d, want 3 — the fullest single path, not the sum", tr.Trusted)
	}
	if _, detail := codexTrustState(tr); !strings.Contains(detail, "3 of 3 trusted") {
		t.Errorf("line = %q, want 3 of 3", detail)
	}
}

// A host with no Codex has no trust to report, and that is not a plugin whose
// hooks were refused. Collapsing the two would tell every Claude-only
// installation to go and trust something that is not there.
func TestNoCodexConfigIsUnknownAndNotUntrusted(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "checkout")
	home := fakeRuntime(t, "directory", market, filepath.Join(root, "cache"))

	tr := pluginreg.ReadCodexTrust(home, pluginreg.ID, market)
	state, detail := codexTrustState(tr)
	if state != "unknown" {
		t.Errorf("state = %q, want unknown: %s", state, detail)
	}
	if !strings.Contains(detail, "config.toml") {
		t.Errorf("line does not name the file it could not read: %s", detail)
	}
}

// The load path's commit is what "which release is deployed on this host"
// means where the runtime reads the plugin out of a checkout, so a tree that
// is dirty or behind is reported rather than passed.
func TestTheLoadPathsCommitIsReportedWithWhatWouldMakeItWrong(t *testing.T) {
	cases := []struct {
		name      string
		tree      pluginreg.Tree
		wantState string
		wantIn    []string
	}{{
		name:      "clean and current",
		tree:      pluginreg.Tree{Head: "16b63c8", Branch: "dev", StatusKnown: true, Upstream: "origin/dev"},
		wantState: "present",
		// "as of the last fetch" is not decoration: doctor does not fetch, so
		// an unqualified "up to date" certifies a host that has not fetched
		// in a week as current.
		wantIn: []string{"16b63c8", "on dev", "clean", "0 behind origin/dev as of the last fetch"},
	}, {
		name:      "uncommitted work in the load path is loaded by the next session",
		tree:      pluginreg.Tree{Head: "16b63c8", Branch: "dev", Dirty: true, StatusKnown: true, Upstream: "origin/dev"},
		wantState: "partial",
		wantIn:    []string{"UNCOMMITTED", "a session loads them"},
	}, {
		name:      "behind its upstream is a host running an older release",
		tree:      pluginreg.Tree{Head: "634e846", Branch: "dev", StatusKnown: true, Upstream: "origin/dev", Behind: 4},
		wantState: "partial",
		wantIn:    []string{"4 behind origin/dev as of the last fetch", "`git pull` deploys them"},
	}, {
		// The name said "unknown, not up to date" while the assertion said
		// "present", which is the collapse this case exists to forbid. A
		// Problems entry is the struct's own record that a question this line
		// answers was not answered, so it cannot end in the word a reader acts
		// on. Not STOPPED: an export published without a remote is a legitimate
		// install shape, and nothing here shows a pull would fail.
		name:      "no upstream is unknown, not up to date",
		tree:      pluginreg.Tree{Head: "16b63c8", Branch: "dev", StatusKnown: true, Problems: []string{"no upstream is configured, so whether it is behind or ahead is unknown"}},
		wantState: "partial",
		wantIn:    []string{"no upstream is configured"},
	}, {
		// Clean, zero behind, and commits on the deployment branch the
		// upstream does not have. Reported as "present" this is a health line
		// certifying the one condition that stops the host deploying, and it
		// would stay quiet until the upstream moves.
		// STOPPED rather than partial: this is the state whose whole point is
		// that the host has stopped receiving what it installs, and a word no
		// exit code depends on is a sentence somebody has to be asked to read.
		name:      "ahead of its upstream is a host that will stop deploying",
		tree:      pluginreg.Tree{Head: "f2936bb", Branch: "dev", StatusKnown: true, Upstream: "origin/dev", Ahead: 2},
		wantState: "STOPPED",
		wantIn: []string{
			"0 behind origin/dev as of the last fetch",
			"2 AHEAD of origin/dev",
			"`git pull --ff-only` refuses",
			"push them, or reset the checkout to its upstream once",
		},
	}, {
		// Diverged both ways. Behind must not swallow ahead: pulling deploys
		// nothing while the fast-forward is refused, so a line that named only
		// the behind count would send the reader to the command that fails.
		name:      "behind and ahead at once names both",
		tree:      pluginreg.Tree{Head: "f2936bb", Branch: "dev", StatusKnown: true, Upstream: "origin/dev", Behind: 4, Ahead: 2},
		wantState: "STOPPED",
		wantIn:    []string{"4 behind origin/dev", "2 AHEAD of origin/dev"},
	}, {
		// The reassuring word must never come from a command that failed. A
		// status that could not be read is the load path's state being
		// unknown, which is what the reader has to act on.
		name:      "a working tree that could not be read is unknown, never clean",
		tree:      pluginreg.Tree{Head: "16b63c8", Branch: "dev", Problems: []string{"cannot read the working tree, so whether it is dirty is unknown: exit status 128: fatal: unable to read index"}},
		wantState: "partial",
		wantIn:    []string{"working tree UNKNOWN", "unable to read index"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, detail := treeState(c.tree)
			if state != c.wantState {
				t.Errorf("state = %q, want %q: %s", state, c.wantState, detail)
			}
			for _, w := range c.wantIn {
				if !strings.Contains(detail, w) {
					t.Errorf("line = %q, want it to carry %q", detail, w)
				}
			}
			if !c.tree.Dirty && strings.Contains(detail, "UNCOMMITTED changes") {
				t.Errorf("a clean tree is reported uncommitted: %s", detail)
			}
			// "whether it is behind is unknown" carries the word too, so the
			// control is the count and the remedy, which only the behind
			// branch writes.
			if c.tree.Behind == 0 && strings.Contains(detail, "`git pull` deploys them") {
				t.Errorf("a tree that is not behind is told to pull: %s", detail)
			}
			if c.tree.StatusKnown && strings.Contains(detail, "working tree UNKNOWN") {
				t.Errorf("a readable working tree is reported unknown: %s", detail)
			}
			// The widening's negative side: a checkout that is not ahead must
			// not be told it is, or every host reads as diverged and the line
			// stops meaning anything.
			if c.tree.Ahead == 0 && strings.Contains(detail, "AHEAD") {
				t.Errorf("a tree that is not ahead is reported ahead: %s", detail)
			}
		})
	}
}

// ReadTree answers about a real checkout, and says "not a checkout" of a
// directory rather than reporting an empty state as though it had read one.
func TestReadTreeSeparatesACheckoutFromADirectoryThatIsNot(t *testing.T) {
	plain := t.TempDir()
	if _, ok := pluginreg.ReadTree(plain); ok {
		t.Error("a directory that is not a checkout was read as one")
	}
	if _, ok := pluginreg.ReadTree(""); ok {
		t.Error("an empty path was read as a checkout")
	}

	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"commit", "-q", "--allow-empty", "-m", "one"},
	} {
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		// A host git config with commit signing or a template would make this
		// fail for reasons that are nothing to do with what is under test.
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	tree, ok := pluginreg.ReadTree(repo)
	if !ok {
		t.Fatal("a checkout was not read as one")
	}
	if tree.Head == "" || tree.Branch != "main" || tree.Dirty {
		t.Fatalf("tree = %+v, want a clean HEAD on main", tree)
	}
	if tree.Upstream != "" {
		t.Errorf("Upstream = %q, want empty with no remote", tree.Upstream)
	}

	writeUnder(t, filepath.Join(repo, "new.txt"), "x")
	if tree, _ := pluginreg.ReadTree(repo); !tree.Dirty {
		t.Error("an untracked file left the tree reported clean — a session would load it")
	}
}

// A checkout the runtime loads in place is deployed by `git pull --ff-only`,
// and that command succeeds while the checkout is ahead of an upstream that
// has not moved. So the report has to count the direction the pull does not
// complain about yet, against a real checkout: a Tree built by hand in a table
// stays green whatever ReadTree does or does not read.
func TestReadTreeCountsCommitsNoRemoteHolds(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	deploy := filepath.Join(root, "deploy")
	run := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out)
		}
		return string(out)
	}
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	run(origin, "init", "-q", "--bare", "-b", "dev")
	run(root, "clone", "-q", origin, deploy)
	run(deploy, "config", "user.email", "t@example.com")
	run(deploy, "config", "user.name", "t")
	run(deploy, "commit", "-q", "--allow-empty", "-m", "one")
	run(deploy, "push", "-q", "-u", "origin", "HEAD:dev")

	if tree, ok := pluginreg.ReadTree(deploy); !ok || tree.Ahead != 0 || tree.Behind != 0 {
		t.Fatalf("tree = %+v, ok = %v: a checkout level with its upstream must be 0 both ways", tree, ok)
	}

	// What a lane merged into the deployment checkout and never pushed leaves
	// behind. Nothing about the tree is dirty and nothing is behind.
	run(deploy, "commit", "-q", "--allow-empty", "-m", "a lane merged here and never pushed")
	tree, ok := pluginreg.ReadTree(deploy)
	if !ok {
		t.Fatal("a checkout was not read as one")
	}
	if tree.Ahead != 1 {
		t.Errorf("Ahead = %d, want 1: a commit no remote holds was not counted", tree.Ahead)
	}
	if tree.Behind != 0 || tree.Dirty {
		t.Errorf("tree = %+v, want 0 behind and clean — that is exactly why ahead is the only signal", tree)
	}
	// The mechanism the count stands for, from git rather than from this
	// test's belief about it: the deployment command is quiet in this state.
	if out := run(deploy, "pull", "--ff-only"); !strings.Contains(out, "Already up to date") {
		t.Fatalf("git pull --ff-only said %q, so the premise that this state is invisible to the deploy no longer holds", strings.TrimSpace(out))
	}
	if state, detail := treeState(tree); state == "present" {
		t.Errorf("doctor reports %q for a checkout that has stopped being deployable: %s", state, detail)
	}
}

// ReadTree consults exactly one remote ref, the tracked upstream, so the ahead
// count establishes that the upstream does not have those commits and nothing
// about what any other remote ref holds. The two come apart in the ordinary
// case: a lane pushes its branch and merges it into the deployment checkout, so
// a remote holds the commits while the upstream still does not. The reader is
// being asked whether to reset the checkout, and a line that says the commits
// are unpublished when they are prices that decision wrong.
func TestTheAheadLineClaimsOnlyTheUpstreamItCounted(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	deploy := filepath.Join(root, "deploy")
	run := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out)
		}
		return string(out)
	}
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	run(origin, "init", "-q", "--bare", "-b", "dev")
	run(root, "clone", "-q", origin, deploy)
	run(deploy, "config", "user.email", "t@example.com")
	run(deploy, "config", "user.name", "t")
	run(deploy, "commit", "-q", "--allow-empty", "-m", "one")
	run(deploy, "push", "-q", "-u", "origin", "HEAD:dev")

	// The lane's work: committed here, and published — to its own branch, not
	// to the branch this checkout tracks.
	run(deploy, "commit", "-q", "--allow-empty", "-m", "a lane merged here, pushed to its own branch")
	run(deploy, "push", "-q", "origin", "HEAD:refs/heads/lane/published")
	run(deploy, "fetch", "-q", "origin")

	tree, ok := pluginreg.ReadTree(deploy)
	if !ok || tree.Ahead != 1 {
		t.Fatalf("tree = %+v, ok = %v: want 1 ahead, the state this test is about", tree, ok)
	}
	// The independent side: git's own answer, not this package's. If it comes
	// back empty the case has stopped being the one described above and the
	// assertion below would pass while testing nothing.
	held := strings.TrimSpace(run(deploy, "branch", "-r", "--contains", "HEAD"))
	if held == "" {
		t.Fatalf("no remote-tracking ref contains HEAD, so this is not the case under test")
	}

	state, detail := treeState(tree)
	if state != "STOPPED" {
		t.Errorf("state = %q, want %q: publishing to another branch does not make the checkout deployable", state, "STOPPED")
	}
	// Substrings pin wordings, not the claim, so this is a regression pin over
	// the phrasings a rewrite reaches for and not a proof that no false claim
	// can be written. What makes the claim itself checkable is the assertion
	// below that the measured facts survive.
	for _, claim := range []string{
		"no remote holds", "no remote has", "that no remote", "no other remote",
		"unpublished", "unpushed", "not on any remote", "only this host",
	} {
		if strings.Contains(detail, claim) {
			t.Errorf("the line claims %q while %q holds them: %s", claim, held, detail)
		}
	}
	// The measured facts must survive: the correction is to the claim, not to
	// the warning.
	for _, want := range []string{"1 AHEAD of origin/dev", "`git pull --ff-only` refuses"} {
		if !strings.Contains(detail, want) {
			t.Errorf("line = %q, want it to carry %q", detail, want)
		}
	}
}

// gitRepo makes a checkout with one commit, isolated from the host's git
// configuration so signing or a template cannot fail it for unrelated reasons.
func gitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"commit", "-q", "--allow-empty", "-m", "one"},
	} {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
}

// For a directory marketplace the marketplace path is what sessions load from,
// so registering a lane's worktree points every new session on the host at a
// tree that is deleted when the lane closes. `mellions install` resolves its
// source by walking up from the working directory, so running it from inside a
// lane is the ordinary way to do this rather than an unusual one.
func TestInstallRefusesASourceTreeThatWillNotBeThere(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(root, "mellions-coxen")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepo(t, checkout)

	lanes := filepath.Join(root, "assignments")
	lane := filepath.Join(lanes, "some-lane", "tree")
	c := exec.Command("git", "-C", checkout, "worktree", "add", "-q", "-b", "mellions/some-lane", lane)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, out)
	}

	// A lane's tree that is not a git worktree at all — a copy, an export —
	// is refused on where it sits, which is the check that does not depend on
	// git having anything to say.
	plainLane := filepath.Join(lanes, "copied-lane", "tree")
	if err := os.MkdirAll(plainLane, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		source      string
		registered  string
		wantRefusal bool
		wantIn      string
	}{{
		name:        "a linked worktree is refused and names the checkout it belongs to",
		source:      lane,
		registered:  checkout,
		wantRefusal: true,
		wantIn:      "linked git worktree",
	}, {
		name:        "a tree under the assignments root is refused even when git says nothing",
		source:      plainLane,
		registered:  checkout,
		wantRefusal: true,
		wantIn:      "under the assignments root",
	}, {
		name:       "the checkout itself proceeds",
		source:     checkout,
		registered: checkout,
	}, {
		name:       "a plain checkout that is not yet registered proceeds",
		source:     checkout,
		registered: "",
	}, {
		// Deliberately NOT exempt. Once a lane has been registered by any
		// route this did not cover, exempting it would disarm the guard in
		// exactly the state it exists to end and report the host unchanged.
		// The repair is `-from <checkout>`, which is never refused.
		name:        "a lane already registered is still refused, not excused by being current",
		source:      lane,
		registered:  lane,
		wantRefusal: true,
		wantIn:      "linked git worktree",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			why := refuseTemporaryTree(c.source, c.registered, lanes)
			if c.wantRefusal && why == "" {
				t.Fatalf("%s was allowed to become the marketplace", c.source)
			}
			if !c.wantRefusal && why != "" {
				t.Fatalf("%s was refused: %s", c.source, why)
			}
			if !c.wantRefusal {
				return
			}
			if !strings.Contains(why, c.wantIn) {
				t.Errorf("refusal = %q, want it to say %q", why, c.wantIn)
			}
			if c.registered != "" && !strings.Contains(why, c.registered) {
				t.Errorf("refusal does not name the checkout the marketplace points at: %s", why)
			}
		})
	}

	// An empty assignments root disables only that check; the worktree check
	// stands on its own, so a config that could not be read does not disarm
	// the guard.
	if why := refuseTemporaryTree(lane, checkout, ""); why == "" {
		t.Error("a linked worktree was allowed when no assignments root was configured")
	}
	// And a sibling whose name merely starts with the root's is not inside it.
	sibling := filepath.Join(root, "assignments-archive", "tree")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if why := refuseTemporaryTree(sibling, checkout, lanes); why != "" {
		t.Errorf("a sibling of the assignments root was refused as though inside it: %s", why)
	}
}

// A marketplace declares each plugin's own source, a path relative to the
// marketplace root. Taking the root instead names a directory with no hooks
// manifest, and the whole registration then reads as broken: no hooks, no
// bearing Skills, `mellions install` exiting non-zero after a successful
// install, and doctor telling a session correctly running the plugin that it
// is not.
func TestTheLoadPathJoinsThePluginsOwnSourceWithinTheMarketplace(t *testing.T) {
	root := t.TempDir()
	market := filepath.Join(root, "marketplace")
	plugin := filepath.Join(market, "pkg", "sub")
	home := fakeRuntime(t, "directory", market, filepath.Join(root, "cache"))
	writeUnder(t, filepath.Join(plugin, "hooks", "hooks.json"), hooksManifest)
	writeUnder(t, filepath.Join(market, ".claude-plugin", "marketplace.json"),
		`{"name": "mellions", "plugins": [{"name": "mellions", "source": "./pkg/sub"}]}`)

	reg := pluginreg.Read(home, pluginreg.ID)
	if reg.LoadPath != plugin {
		t.Fatalf("LoadPath = %q, want the plugin's own directory %q", reg.LoadPath, plugin)
	}
	if len(reg.Problems) != 0 {
		t.Errorf("a well-formed subdirectory plugin reported problems: %v", reg.Problems)
	}
	if !reg.Ready() {
		t.Error("Ready() false for a registration whose every record agrees")
	}
	if len(reg.Hooks) != 2 {
		t.Errorf("SessionStart hooks = %d, want 2 — read from the plugin, not the root", len(reg.Hooks))
	}

	// "./" is the ordinary case and must still land on the root.
	writeUnder(t, filepath.Join(market, ".claude-plugin", "marketplace.json"),
		`{"name": "mellions", "plugins": [{"name": "mellions", "source": "./"}]}`)
	if got := pluginreg.Read(home, pluginreg.ID); got.LoadPath != market {
		t.Errorf(`LoadPath = %q for source "./", want the marketplace root %q`, got.LoadPath, market)
	}
	// A manifest that is absent, unparseable, names another plugin, or gives a
	// source this cannot resolve leaves the root rather than inventing a path.
	for _, body := range []string{
		`{"plugins": [{"name": "somebody-else", "source": "./pkg/sub"}]}`,
		`{"plugins": [{"name": "mellions", "source": {"source": "github", "repo": "a/b"}}]}`,
		`not json at all`,
	} {
		writeUnder(t, filepath.Join(market, ".claude-plugin", "marketplace.json"), body)
		if got := pluginreg.Read(home, pluginreg.ID); got.LoadPath != market {
			t.Errorf("LoadPath = %q for manifest %q, want the root %q", got.LoadPath, body, market)
		}
	}
	if err := os.Remove(filepath.Join(market, ".claude-plugin", "marketplace.json")); err != nil {
		t.Fatal(err)
	}
	if got := pluginreg.Read(home, pluginreg.ID); got.LoadPath != market {
		t.Errorf("LoadPath = %q with no manifest, want the root %q", got.LoadPath, market)
	}
}

// The guard is only real if `mellions install` itself refuses. Asserting on
// refuseTemporaryTree alone leaves the wiring untested: delete the call from
// cmdInstall and a test that never runs the command stays green, which is the
// exact shape of a guard that is believed and does nothing.
func TestInstallItselfRefusesALaneAndProceedsFromTheCheckout(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(root, "mellions-coxen")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepo(t, checkout)
	writeUnder(t, filepath.Join(checkout, ".claude-plugin", "marketplace.json"),
		`{"name": "mellions", "plugins": [{"name": "mellions", "source": "./"}]}`)

	lanes := filepath.Join(root, "assignments")
	lane := filepath.Join(lanes, "a-lane", "tree")
	c := exec.Command("git", "-C", checkout, "worktree", "add", "-q", "-b", "mellions/a-lane", lane)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, out)
	}

	// The lane is a checkout of the same repo, so it carries the manifest too;
	// resolveSource requires one before the guard is ever reached.
	writeUnder(t, filepath.Join(lane, ".claude-plugin", "marketplace.json"),
		`{"name": "mellions", "plugins": [{"name": "mellions", "source": "./"}]}`)

	cfg := filepath.Join(root, "mellions.json")
	writeUnder(t, cfg, `{"owner":"o","repos":[],"assignments_root":"`+lanes+`"}`)
	t.Setenv("MELLIONS_CONFIG", cfg)

	run := func(t *testing.T, args ...string) error {
		t.Helper()
		return cmdInstall(context.Background(), args)
	}

	// Refused: -from a lane, and the bare form resolved from inside one.
	if err := run(t, "-from", lane, "-dry-run"); err == nil {
		t.Error("install accepted a lane worktree as the marketplace")
	} else if !strings.Contains(err.Error(), "linked git worktree") {
		t.Errorf("refusal = %v, want it to name the worktree", err)
	}
	// -dry-run must refuse too: it is how a person checks before committing to it.
	if err := run(t, "-from", lane, "-dry-run"); err == nil {
		t.Error("a dry run accepted what the real run refuses")
	}
	// One runtime at a time is not a way around it: install registers the
	// source with whichever runtimes are present, and an unfit source is
	// unfit for all of them.
	for _, rt := range []string{"claude", "codex"} {
		if err := run(t, "-from", lane, "-runtime", rt, "-dry-run"); err == nil {
			t.Errorf("-runtime %s accepted a lane worktree", rt)
		}
	}
	// A tree under the assignments root that git knows nothing about.
	plain := filepath.Join(lanes, "copied", "tree")
	writeUnder(t, filepath.Join(plain, ".claude-plugin", "marketplace.json"),
		`{"name": "mellions", "plugins": [{"name": "mellions", "source": "./"}]}`)
	if err := run(t, "-from", plain, "-dry-run"); err == nil {
		t.Error("install accepted a non-git tree under the assignments root")
	}

	// Allowed: the checkout. A dry run touches no runtime, so this asserts the
	// guard lets it through rather than that the install succeeded.
	if err := run(t, "-from", checkout, "-dry-run"); err != nil {
		t.Errorf("install refused the checkout: %v", err)
	}
}

// A subdirectory of a plain checkout is not a linked worktree, and saying so
// would refuse an ordinary install. On this platform t.TempDir() sits under a
// symlink, which is exactly where comparing git's answer against the caller's
// own spelling of the path goes wrong.
func TestASubdirectoryOfACheckoutIsNotALinkedWorktree(t *testing.T) {
	repo := t.TempDir()
	gitRepo(t, repo)
	sub := filepath.Join(repo, "pkg", "inner")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if main, linked := worktreeOf(sub); linked {
		t.Errorf("a subdirectory of a checkout was reported a linked worktree of %q", main)
	}
	if main, linked := worktreeOf(repo); linked {
		t.Errorf("a checkout root was reported a linked worktree of %q", main)
	}
}

// A relative or symlinked assignments root must still match. A lexical prefix
// test silently matches nothing there, and a guard that matches nothing is
// worse than none because it is believed.
func TestTheAssignmentsRootMatchesWhenItIsRelativeOrSymlinked(t *testing.T) {
	root := t.TempDir()
	lanes := filepath.Join(root, "assignments")
	lane := filepath.Join(lanes, "x", "tree")
	if err := os.MkdirAll(lane, 0o755); err != nil {
		t.Fatal(err)
	}
	if !underDir(lane, lanes) {
		t.Fatal("an absolute root did not match")
	}

	link := filepath.Join(root, "linked-assignments")
	if err := os.Symlink(lanes, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !underDir(filepath.Join(link, "x", "tree"), lanes) {
		t.Error("a lane reached through a symlink escaped the assignments root")
	}
	if !underDir(lane, link) {
		t.Error("a symlinked root did not match the lane inside it")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	defer func() { _ = os.Chdir(wd) }()
	if !underDir("assignments/x/tree", "assignments") {
		t.Error("a relative root did not match")
	}
	if underDir(filepath.Join(root, "assignments-archive", "t"), lanes) {
		t.Error("a sibling of the root was treated as inside it")
	}
}

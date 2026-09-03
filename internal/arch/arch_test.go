// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package arch holds tests about the shape of this codebase rather than its
// behaviour, because its rules are the kind that erode one reasonable import
// at a time.
package arch

import (
	"encoding/json"
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const module = "github.com/LetA-Tech/mellions-coxen/"

// core is everything that must stay provider-neutral.
var core = []string{
	"internal/signal",
	"internal/survey",
	"internal/assignment",
	"internal/continuity",
	"internal/issuegate",
	"internal/provenance",
	"internal/program",
	"internal/partner",
	"internal/presence",
	"internal/awareness",
}

// TestCoreImportsNoProvider: GitHub is one implementation of signal.Source.
// The moment a core package imports one, replacing the tracker stops being a
// configuration change.
func TestCoreImportsNoProvider(t *testing.T) {
	root := repoRoot(t)
	for _, pkg := range core {
		p, err := build.ImportDir(filepath.Join(root, pkg), 0)
		if err != nil {
			t.Fatalf("%s: %v", pkg, err)
		}
		for _, imp := range append(p.Imports, p.TestImports...) {
			if strings.HasPrefix(imp, module+"internal/sources/") {
				t.Errorf("%s imports %s — core must reach providers only through signal.Source", pkg, imp)
			}
		}
	}
}

// TestSourcesOnlyDependOnTheCore keeps providers from growing dependencies on
// each other, which would make removing one a cascade.
func TestSourcesOnlyDependOnTheCore(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "sources")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		self := module + "internal/sources/" + e.Name()
		p, err := build.ImportDir(filepath.Join(dir, e.Name()), 0)
		if err != nil {
			if _, empty := err.(*build.NoGoError); empty {
				continue
			}
			t.Fatalf("%s: %v", e.Name(), err)
		}
		for _, imp := range append(p.Imports, p.TestImports...) {
			if strings.HasPrefix(imp, module+"internal/sources/") && imp != self {
				t.Errorf("%s imports %s — one provider depending on another makes removing either a cascade", e.Name(), imp)
			}
		}
	}
}

// TestTheDeletedMachineryStaysDeleted guards the cleanup. Everything here was
// removed because it enforced, graded or approved rather than informed; a
// reference in a doc or a script would rot silently where a Go import would not.
func TestTheDeletedMachineryStaysDeleted(t *testing.T) {
	root := repoRoot(t)
	gone := []string{
		"internal/store", "internal/handler", "internal/session", "internal/gateway",
		"internal/dispatcher", "internal/control", "internal/selfrepair",
		"internal/authority", "internal/decision", "internal/falsify", "internal/independent",
		"internal/learn", "internal/territory", "internal/corpus", "internal/skillcheck",
		"internal/usage", "internal/capability", "internal/binding", "internal/shelltext",
		"hooks/authority-gate.sh", "hooks/identity.sha256",
	}
	for _, p := range gone {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s still exists; it was replaced, not kept alongside", p)
		}
	}
}

// TestEveryPackageAdvertisesOneVersion: three manifests state the release
// because two ecosystems demand different shapes, not because there are three
// versions.
func TestEveryPackageAdvertisesOneVersion(t *testing.T) {
	root := repoRoot(t)
	claude := manifestField(t, filepath.Join(root, ".claude-plugin", "plugin.json"), "version")
	codex := manifestField(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "version")
	if claude != codex {
		t.Errorf("Claude plugin says %q, Codex plugin says %q", claude, codex)
	}
	var market struct {
		Plugins []struct{ Name, Version, License string } `json:"plugins"`
	}
	readJSON(t, filepath.Join(root, ".claude-plugin", "marketplace.json"), &market)
	if len(market.Plugins) != 1 {
		t.Fatalf("marketplace lists %d plugins, want the one", len(market.Plugins))
	}
	if market.Plugins[0].Version != claude {
		t.Errorf("marketplace entry says %q, plugin says %q", market.Plugins[0].Version, claude)
	}
	if market.Plugins[0].Name != "mellions" {
		t.Errorf("marketplace entry names %q", market.Plugins[0].Name)
	}
	licence := manifestField(t, filepath.Join(root, ".claude-plugin", "plugin.json"), "license")
	if market.Plugins[0].License != licence {
		t.Errorf("marketplace licence %q, plugin licence %q", market.Plugins[0].License, licence)
	}
	if codexLic := manifestField(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "license"); codexLic != licence {
		t.Errorf("Codex plugin licence %q, Claude plugin licence %q", codexLic, licence)
	}
	if raw, err := os.ReadFile(filepath.Join(root, "LICENSE")); err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		t.Fatalf("no LICENSE at the root: %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(root, "NOTICE")); err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		t.Fatalf("no NOTICE at the root: %v", err)
	}
}

func TestEveryPackageAdvertisesTheCanonicalRepository(t *testing.T) {
	root := repoRoot(t)
	const repository = "https://github.com/LetA-Tech/mellions-coxen"
	const homepage = "https://letatech.ca/mellions-engineer"
	for _, manifest := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		path := filepath.Join(root, manifest)
		if got := manifestField(t, path, "homepage"); got != homepage {
			t.Errorf("%s homepage = %q, want %q", manifest, got, homepage)
		}
		if got := manifestField(t, path, "repository"); got != repository {
			t.Errorf("%s repository = %q, want %q", manifest, got, repository)
		}
	}
	var codex struct {
		Interface struct {
			WebsiteURL string `json:"websiteURL"`
		} `json:"interface"`
	}
	readJSON(t, filepath.Join(root, ".codex-plugin", "plugin.json"), &codex)
	if codex.Interface.WebsiteURL != homepage {
		t.Errorf("Codex websiteURL = %q, want %q", codex.Interface.WebsiteURL, homepage)
	}
}

func TestRetiredPrivateRepositorySlugOnlyNamesTheWebsite(t *testing.T) {
	root := repoRoot(t)
	retired := "mellions-" + "engineer"
	const website = "https://letatech.ca/mellions-engineer"
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "internal-docs":
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == ".git" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), "\x00") {
			return nil
		}
		for n, line := range strings.Split(string(raw), "\n") {
			withoutWebsite := strings.ReplaceAll(line, website, "")
			if strings.Contains(withoutWebsite, retired) {
				t.Errorf("%s:%d still names the retired private repository", rel, n+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func manifestField(t *testing.T, path, field string) string {
	t.Helper()
	var v map[string]any
	readJSON(t, path, &v)
	s, _ := v[field].(string)
	if s == "" {
		t.Fatalf("%s states no %s", path, field)
	}
	return s
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// TestBothPackagesReadTheOneSkillCorpus: two manifests at one root pointing at
// one directory is what keeps the corpus from forking.
func TestBothPackagesReadTheOneSkillCorpus(t *testing.T) {
	root := repoRoot(t)
	for _, m := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		var v struct{ Skills string }
		readJSON(t, filepath.Join(root, m), &v)
		if v.Skills != "./skills/" {
			t.Errorf("%s points its Skills at %q, want ./skills/", m, v.Skills)
		}
	}
}

// TestEverySkillIsLoadable: a bundle is a directory with a SKILL.md whose
// frontmatter names it after its directory and describes when to use it. A
// skill either runtime cannot parse is a method nobody gets.
func TestEverySkillIsLoadable(t *testing.T) {
	root := filepath.Join(repoRoot(t), "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name(), "SKILL.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("skills/%s has no SKILL.md", e.Name())
			continue
		}
		text := string(raw)
		if !strings.HasPrefix(text, "---\n") {
			t.Errorf("skills/%s/SKILL.md does not start with frontmatter", e.Name())
			continue
		}
		end := strings.Index(text[4:], "\n---")
		if end < 0 {
			t.Errorf("skills/%s/SKILL.md frontmatter never closes", e.Name())
			continue
		}
		front := text[4 : 4+end]
		if !strings.Contains(front, "name: "+e.Name()+"\n") && !strings.HasSuffix(front, "name: "+e.Name()) {
			t.Errorf("skills/%s/SKILL.md must be named %q in its frontmatter", e.Name(), e.Name())
		}
		if !strings.Contains(front, "description: ") {
			t.Errorf("skills/%s/SKILL.md has no description", e.Name())
		}
		for _, line := range strings.Split(front, "\n") {
			if strings.Contains(line, ": ") && strings.Count(line, ": ") > 1 && !strings.HasPrefix(line, "description:") {
				t.Errorf("skills/%s/SKILL.md frontmatter line %q would not survive a strict YAML reader", e.Name(), line)
			}
		}
	}
}

// TestTheSkillCorpusCarriesNoPrivateEstate: every installer receives the
// corpus whole. An internal hostname, a login or a home directory in a shipped
// Skill is disclosure to a stranger and useless to them.
func TestTheSkillCorpusCarriesNoPrivateEstate(t *testing.T) {
	private := []string{"private-service#", "employee-login", "internal-host", "customer-prefix-", "internal-tool/"}
	home := regexp.MustCompile(`/(?:home|Users)/[a-z][a-z0-9_-]*/`)
	root := filepath.Join(repoRoot(t), "skills")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for n, line := range strings.Split(string(raw), "\n") {
			for _, id := range private {
				if strings.Contains(line, id) {
					t.Errorf("skills/%s:%d ships %q", rel, n+1, id)
				}
			}
			if m := home.FindString(line); m != "" {
				t.Errorf("skills/%s:%d ships %q", rel, n+1, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTheHooksNameNoParticularMachine: the installation root arrives in the
// environment; nothing here may assume a filesystem layout.
func TestTheHooksNameNoParticularMachine(t *testing.T) {
	root := repoRoot(t)
	hooks, err := filepath.Glob(filepath.Join(root, "hooks", "*.sh"))
	if err != nil || len(hooks) == 0 {
		t.Fatalf("no hooks found: %v", err)
	}
	for _, path := range hooks {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			code := line
			if h := strings.IndexByte(code, '#'); h >= 0 {
				code = code[:h]
			}
			for _, prefix := range []string{"/home/", "/Users/", "/opt/", "/srv/"} {
				if strings.Contains(code, prefix) {
					t.Errorf("%s:%d names one machine's filesystem: %s", filepath.Base(path), i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestEveryHookIsRegistered: a hook the manifest does not name is a file
// nobody calls, and its own test proves nothing about the running session.
//
// hooks/test-pr-reference.sh:236 makes this assertion for one hook, with the
// reason written beside it — "hooks.json has to actually run it, or every
// assertion above is about a file nothing invokes". It was right and it was
// per-hook, so a hook added later inherited none of it. Measured while
// falsifying the secret-read guard: deleting that guard's entry from the
// manifest left the whole Go suite and check-hooks green, which is the
// "the fix did not land" reading of a green run that mellions-falsification
// names, arriving as a permanent property of the repository rather than as
// one arm.
func TestEveryHookIsRegistered(t *testing.T) {
	root := repoRoot(t)
	manifest, err := os.ReadFile(filepath.Join(root, "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	hooks, err := filepath.Glob(filepath.Join(root, "hooks", "*.sh"))
	if err != nil || len(hooks) == 0 {
		t.Fatalf("no hooks found: %v", err)
	}
	registered := 0
	for _, path := range hooks {
		name := filepath.Base(path)
		// lib.sh is sourced by the others; test-*.sh is run by check-hooks.
		// Neither is dispatched by the runtime.
		if name == "lib.sh" || strings.HasPrefix(name, "test-") {
			continue
		}
		if !strings.Contains(string(manifest), name) {
			t.Errorf("hooks/%s is never named in hooks.json — the runtime will not run it, so nothing it asserts reaches a session", name)
			continue
		}
		registered++
	}
	if registered == 0 {
		t.Fatal("no dispatched hooks found; this test would pass on an empty hooks directory")
	}
}

// TestTheModuleRootShipsNoStrayPackage: the root of this module holds no Go
// source at all.
func TestTheModuleRootShipsNoStrayPackage(t *testing.T) {
	entries, err := os.ReadDir(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			t.Errorf("%s is Go source at the module root", e.Name())
		}
	}
}

// TestTheCorpusNamesNoDeletedVerb: a method that tells the engineer to run a
// command that no longer exists is worse than one that says nothing.
func TestTheCorpusNamesNoDeletedVerb(t *testing.T) {
	root := repoRoot(t)
	gone := regexp.MustCompile(`mellions (authority|decision|verify|prove|learn|corpus|skill|binding|session|observe|territory|issue|r)\b`)
	for _, dir := range []string{"skills", "commands", "agents", "hooks", "scripts"} {
		filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			rel, _ := filepath.Rel(root, path)
			for n, line := range strings.Split(string(raw), "\n") {
				if m := gone.FindString(line); m != "" {
					t.Errorf("%s:%d names a verb that no longer exists: %s", rel, n+1, strings.TrimSpace(m))
				}
			}
			return nil
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

// TestTheIdentityReachesASessionWhole.
//
// The runtime shows a session only a short preview of any single hook's
// output from 10,000 bytes, measured against Claude Code 2.1.250 with a probe
// hook: 9,767 bytes arrived whole and 10,240 arrived as twenty-five lines and
// a note. The identity is emitted by its own hook with a four-line preamble,
// and it is the one thing that must never be cut — an engineer that got half
// of who it is would not know which half.
func TestTheIdentityReachesASessionWhole(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "agents", "mellions.md"))
	if err != nil {
		t.Fatal(err)
	}
	const preamble = 250
	const limit = 10000
	if n := len(raw) + preamble; n > limit-300 {
		t.Errorf("agents/mellions.md is %d bytes; with the hook's preamble that is %d, and the runtime "+
			"cuts a hook's output at %d. Tighten it — the session would see the first twenty-five lines and nothing else",
			len(raw), n, limit)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package program

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// estate builds a work root with real git checkouts, because discovery's job is
// to read a real environment and a fake one would test the fake.
func estate(t *testing.T, repos map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, files := range repos {
		dir := filepath.Join(root, name)
		for f, body := range files {
			p := filepath.Join(dir, f)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, args := range [][]string{
			{"init", "-q", "-b", "dev"}, {"add", "."}, {"commit", "-q", "-m", "initial"},
		} {
			out, err := runGit(t, dir, args...)
			if err != nil {
				t.Fatalf("git %v in %s: %v\n%s", args, name, err, out)
			}
		}
	}
	// A directory that is not a checkout must be ignored.
	if err := os.MkdirAll(filepath.Join(root, "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func runGit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	return runCmdEnv(dir, "git", args...)
}

func runCmdEnv(dir, name string, args ...string) (string, error) {
	ctx := context.Background()
	old := os.Environ()
	os.Setenv("GIT_AUTHOR_NAME", "t")
	os.Setenv("GIT_AUTHOR_EMAIL", "t@example.invalid")
	os.Setenv("GIT_COMMITTER_NAME", "t")
	os.Setenv("GIT_COMMITTER_EMAIL", "t@example.invalid")
	defer func() {
		for _, kv := range old {
			if k, v, ok := strings.Cut(kv, "="); ok {
				os.Setenv(k, v)
			}
		}
	}()
	return runCmd(ctx, dir, name, args...)
}

func discover(t *testing.T, root string) *Evidence {
	t.Helper()
	ev, err := Discover(context.Background(), DiscoverOptions{WorkRoot: root, WindowDays: 90})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return ev
}

func TestDiscoveryEstablishesFactsNotConclusions(t *testing.T) {
	root := estate(t, map[string]map[string]string{
		"svc-ledger": {
			"internal/posting/service.go": "package service\n\nfunc Post() {}\n",
			"migrations/001_init.sql":     "create table journal();\n",
			"migrations/289_thing.sql":    "alter table journal add x int;\n",
			"CLAUDE.md":                   "# ledger\n",
		},
		"svc-insight": {
			"internal/read.go": "package internal\n\n// reads from svc-ledger\nfunc Read() {}\n",
			"README.md":        "reporting over svc-ledger\n",
		},
	})
	ev := discover(t, root)

	if len(ev.Repos) != 2 {
		t.Fatalf("found %d repositories, want 2 (and never the non-checkout): %+v", len(ev.Repos), ev.Repos)
	}
	byName := map[string]RepoFact{}
	for _, r := range ev.Repos {
		byName[r.Name] = r
	}
	ledger := byName["svc-ledger"]
	if ledger.Branch != "dev" || ledger.Head == "" || ledger.CommitsInWindow != 1 {
		t.Errorf("git facts wrong: %+v", ledger)
	}
	if ledger.Migrations != 2 || ledger.NewestMigration != "289_thing.sql" {
		t.Errorf("schema ownership not established: %d %q", ledger.Migrations, ledger.NewestMigration)
	}
	// A file-count profile, most files first — this repository genuinely has more
	// SQL than Go, and reporting otherwise would be a judgement about which
	// matters more.
	if len(ledger.Languages) < 2 || ledger.Languages[0] != "SQL" {
		t.Errorf("languages = %v, want SQL first on file count with Go present", ledger.Languages)
	}
	if len(ledger.Docs) == 0 {
		t.Error("governing documents not noted")
	}

	// The relationship is a mention, and is reported as one.
	var found bool
	for _, c := range ev.CrossRefs {
		if c.From == "svc-insight" && c.To == "svc-ledger" {
			found = true
			if c.Hits == 0 || c.Sample == "" {
				t.Errorf("cross-reference carries no checkable evidence: %+v", c)
			}
		}
	}
	if !found {
		t.Errorf("a repository naming another was not reported: %+v", ev.CrossRefs)
	}
}

// TestEvidenceDisclaimsConclusions. The whole point of the split is that this
// file is material, and mistaking it for a program is how inference becomes fact.
func TestEvidenceDisclaimsConclusions(t *testing.T) {
	txt := discover(t, estate(t, map[string]map[string]string{
		"a": {"main.go": "package main\n"},
	})).Text()
	for _, want := range []string{"facts, not conclusions", "judgements — yours"} {
		if !strings.Contains(txt, want) {
			t.Errorf("evidence does not disclaim conclusions, missing %q:\n%s", want, txt)
		}
	}
}

// TestUnexaminableRepoIsDisclosed: a thin picture must never read as a small
// estate. Same rule the survey follows for a source that did not answer.
func TestUnexaminableRepoIsDisclosed(t *testing.T) {
	root := estate(t, map[string]map[string]string{"good": {"a.go": "package a\n"}})
	// A directory that looks like a checkout but is not.
	if err := os.MkdirAll(filepath.Join(root, "broken", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	ev, err := Discover(context.Background(), DiscoverOptions{WorkRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Failures) != 1 || !strings.Contains(ev.Failures[0], "broken") {
		t.Fatalf("an unexaminable checkout was silently dropped: %+v", ev.Failures)
	}
	if !strings.Contains(ev.Text(), "unknown, not as absent") {
		t.Error("the rendered evidence does not warn that the picture is incomplete")
	}
}

func TestQuietRepositoryIsAFactNotAVerdict(t *testing.T) {
	r := RepoFact{LastCommitAt: time.Now().Add(-200 * 24 * time.Hour)}
	if got := r.Quiet(time.Now()); got < 190*24*time.Hour {
		t.Errorf("quiet duration = %v", got)
	}
	// Nothing in this package decides what quiet means. If a helper ever appears
	// that returns "abandoned", it has crossed from evidence into judgement.
	if strings.Contains(fmt.Sprintf("%T", r), "Abandoned") {
		t.Error("RepoFact has grown a verdict field")
	}
}

func TestNoWorkRootIsAnError(t *testing.T) {
	if _, err := Discover(context.Background(), DiscoverOptions{}); err == nil {
		t.Fatal("discovery with no work root succeeded")
	}
}

func TestEmptyWorkRootIsAnError(t *testing.T) {
	if _, err := Discover(context.Background(), DiscoverOptions{WorkRoot: t.TempDir()}); err == nil {
		t.Fatal("discovery over a directory with no checkouts succeeded")
	}
}

// TestCrossRefsSplitCodeFromProse. A repository named in source is usually a
// dependency; one named in an archived design note is usually history. The real
// estate produced "policy-service -> advisor-service" from a file under _archived/dispatch/,
// which is not an edge, and reporting it identically to a real one is how a map
// fills with relationships nobody has.
func TestCrossRefsSplitCodeFromProse(t *testing.T) {
	root := estate(t, map[string]map[string]string{
		"svc-a": {
			"internal/client.go":           "package internal\n// calls svc-b\n",
			"docs/_archived/old-design.md": "we once considered svc-b\n",
			"docs/notes.md":                "svc-b was discussed\n",
		},
		"svc-b": {"main.go": "package main\n"},
	})
	var ref *CrossRef
	for _, c := range discover(t, root).CrossRefs {
		if c.From == "svc-a" && c.To == "svc-b" {
			ref = &c
		}
	}
	if ref == nil {
		t.Fatal("the reference was not found at all")
	}
	if ref.InCode != 1 || ref.InDocs != 2 {
		t.Errorf("split = %d code / %d prose, want 1 and 2: %+v", ref.InCode, ref.InDocs, ref)
	}
	// The sample must be the code mention, because that is what a reader should
	// open first.
	if !strings.Contains(ref.Sample, "client.go") {
		t.Errorf("sample = %q, want the code mention", ref.Sample)
	}
}

// TestProseOnlyReferenceSaysSo, rather than reading as an established edge.
func TestProseOnlyReferenceSaysSo(t *testing.T) {
	root := estate(t, map[string]map[string]string{
		"svc-a": {"docs/_archived/history.md": "svc-b used to matter\n", "main.go": "package main\n"},
		"svc-b": {"main.go": "package main\n"},
	})
	txt := discover(t, root).Text()
	if !strings.Contains(txt, "prose only") || !strings.Contains(txt, "history rather than a dependency") {
		t.Errorf("a prose-only mention is not distinguished from a dependency:\n%s", txt)
	}
}

// TestProseMentionsAreNeverDropped. Splitting a count is evidence; discarding a
// mention because a heuristic called it prose would be a silent judgement.
func TestProseMentionsAreNeverDropped(t *testing.T) {
	root := estate(t, map[string]map[string]string{
		"svc-a": {"README.md": "depends on svc-b\n", "main.go": "package main\n"},
		"svc-b": {"main.go": "package main\n"},
	})
	var found bool
	for _, c := range discover(t, root).CrossRefs {
		if c.From == "svc-a" && c.To == "svc-b" {
			found = true
		}
	}
	if !found {
		t.Error("a mention that appeared only in prose was dropped entirely")
	}
}

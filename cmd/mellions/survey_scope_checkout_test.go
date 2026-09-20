// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sig "github.com/LetA-Tech/mellions-coxen/internal/signal"
	"github.com/LetA-Tech/mellions-coxen/internal/survey"
)

// commitRepo makes dir a checkout git will answer questions about.
//
// A .git directory alone satisfies the resolver and then fails `git rev-parse`,
// which reds for a reason that has nothing to do with what is under test.
func commitRepo(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	run("init", "-q", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "first")
	return dir
}

func collect(t *testing.T, cfg *Config, scope []string) survey.Result {
	t.Helper()
	reg, err := cfg.build(scope)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	runner, err := survey.NewRunner(reg, cfg.Sources)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	return runner.Run(context.Background(), sig.Scope{Repos: scope})
}

func hasRepo(res survey.Result, repo string) bool {
	for _, s := range res.Signals {
		if s.Repo == repo {
			return true
		}
	}
	return false
}

// A survey scoped by name must re-resolve where that repository is.
//
// The scope replaces what a source collects and reaches it at collect time; the
// set that says where each repository lives is built before the scope exists.
// Built from "repos" alone it cannot place a repository named only under
// "checkouts", so the source falls back to work_root/<name>, stats nothing, and
// the run reports the git and stale sources as INCOMPLETE — which reads as a
// collection failure rather than the configuration question it is. On this host
// the repository that reaches is Mellions' own source, surveyed by name on
// every shift.
//
// Both directions, because the fix must not buy the first with the second:
// naming where a repository is still must not enrol it into what an unscoped
// survey collects, which is what lets this installation work in a repository it
// does not survey.
func TestSurveyScopedByNameResolvesACheckoutOutsideRepos(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	commitRepo(t, filepath.Join(work, "in-scope"))
	own := commitRepo(t, filepath.Join(root, "elsewhere", "mellions-coxen"))

	cfg := &Config{
		Owner:      "acme",
		Repos:      []string{"in-scope"},
		WorkRoot:   work,
		CheckoutAt: map[string]string{"mellions-coxen": own},
		Sources:    []string{"git"},
	}

	scoped := collect(t, cfg, []string{"mellions-coxen"})
	for _, f := range scoped.Failures {
		t.Errorf("a survey scoped to a repository the config can locate failed: %s: %v", f.Source, f.Err)
	}
	if !hasRepo(scoped, "mellions-coxen") {
		t.Errorf("-repos mellions-coxen collected nothing about it; signals: %d", len(scoped.Signals))
	}

	unscoped := collect(t, cfg, cfg.Repos)
	for _, f := range unscoped.Failures {
		t.Errorf("the unscoped survey failed: %s: %v", f.Source, f.Err)
	}
	if !hasRepo(unscoped, "in-scope") {
		t.Fatal("the unscoped survey collected nothing at all, so what it did not collect proves nothing")
	}
	if hasRepo(unscoped, "mellions-coxen") {
		t.Error("naming where a repository is enrolled it into the default scope; " +
			"an installation can no longer work in a repository it does not survey")
	}
}

// The command is what has to pass the scope to the wiring.
//
// build takes the scope as an argument, so a test that calls build itself
// supplies the very thing the defect was the absence of: it stays green with
// cmdSurvey computing the scope after the sources are already wired, which is
// the state the fix exists to leave behind. The entry point is the only place
// that ordering is observable, so it is driven here for real — configuration on
// disk, flags as typed, exit status read.
func TestTheSurveyCommandPassesItsScopeToTheWiring(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	commitRepo(t, filepath.Join(work, "in-scope"))
	own := commitRepo(t, filepath.Join(root, "elsewhere", "mellions-coxen"))

	cfgPath := filepath.Join(root, "config.json")
	body := `{"owner":"acme","repos":["in-scope"],"work_root":` + quote(work) +
		`,"checkouts":{"mellions-coxen":` + quote(own) + `},"sources":["git"]}`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// A partial survey exits non-zero, which is how a source that could not
	// answer reaches a caller. That is the assertion.
	err := cmdSurvey(context.Background(),
		[]string{"-config", cfgPath, "-repos", "mellions-coxen", "-sources", "git"})
	if err != nil {
		t.Errorf("mellions survey -repos mellions-coxen = %v, want a complete survey", err)
	}
}

func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// A scoped name the configuration cannot place is still an error, not silence.
//
// The set is widened by the scope, so a typed name that resolves nowhere must
// keep reaching the fallback and failing there: resolving it to nothing and
// collecting nothing would turn a typo into an empty, complete-looking survey.
func TestSurveyScopedByAnUnknownNameStillFails(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	commitRepo(t, filepath.Join(work, "in-scope"))

	cfg := &Config{
		Owner: "acme", Repos: []string{"in-scope"}, WorkRoot: work,
		Sources: []string{"git"},
	}

	res := collect(t, cfg, []string{"mellions-coxen"})
	if len(res.Failures) == 0 {
		t.Error("a survey scoped to a repository this installation cannot locate reported success")
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package gitsrc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
	"github.com/LetA-Tech/mellions-coxen/internal/survey"
)

// fixture is a repository whose remote branch `branch` carries `binding` (no
// file when empty). Every other place a gate could be misread from carries a
// decoy: origin/dev when branch is not dev (decoyDev), the local HEAD, and the
// working tree. With upstream set, the checkout tracks origin/<branch>.
type fixture struct {
	name, branch, binding string
	decoyDev, upstream    bool
}

func (f fixture) make(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, f.name)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=work"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, f.name, err, out)
		}
	}
	commit := func(binding, msg string) {
		t.Helper()
		p := filepath.Join(dir, ".claude", "repo-binding.yaml")
		if binding == "" {
			_ = os.Remove(p)
		} else {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(binding), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git("add", "-A")
		git("commit", "-q", "--allow-empty", "-m", msg)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte(f.name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("remote", "add", "origin", filepath.Join(root, f.name+".git"))
	commit(f.binding, "binding")
	git("update-ref", "refs/remotes/origin/"+f.branch, "HEAD")
	declared := ""
	if f.branch != "dev" && strings.Contains(f.binding, "development_branch:") {
		declared = "development_branch: " + f.branch + "\n"
	}
	if f.decoyDev {
		commit(declared+"approval_authorities: [decoy]\napproval_model: wrong-branch\n", "decoy dev")
		git("update-ref", "refs/remotes/origin/dev", "HEAD")
	}
	if f.upstream {
		git("config", "branch.work.remote", "origin")
		git("config", "branch.work.merge", "refs/heads/"+f.branch)
	}
	commit(declared+"approval_authorities: [local]\napproval_model: local-head\n", "local head")
	p := filepath.Join(dir, ".claude", "repo-binding.yaml")
	if err := os.WriteFile(p, []byte(declared+"approval_authorities: [anyone]\napproval_model: delegated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSurveyPrintsEachRepositorysGateAtItsRemoteBranch(t *testing.T) {
	root := t.TempDir()
	fixtures := []fixture{
		{name: "gated", branch: "dev", binding: "development_branch: dev\napproval_authorities: [lucas, antony]\napproval_model: merge-gate\n"},
		{name: "undeclared", branch: "dev", binding: "approval_authorities: [lucas]\n"},
		{name: "unbound", branch: "dev"},
		{name: "mainline", branch: "main", decoyDev: true, binding: "development_branch: main\napproval_authorities: [lucas]\napproval_model: pre-proposal\n"},
		{name: "tracked", branch: "release", upstream: true, binding: "approval_authorities: [antony]\napproval_model: merge-gate\n"},
	}
	var repos []string
	for _, f := range fixtures {
		f.make(t, root)
		repos = append(repos, f.name)
	}

	got, err := New(Options{WorkRoot: root, Repos: repos}).Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ofKind(got, signal.KindGate)); n != len(fixtures) {
		t.Fatalf("want one gate signal per repository, got %d: %+v", n, got)
	}
	text := survey.Result{Signals: got, Ran: []string{"git"}}.Render(survey.Default())
	lines := strings.Split(text, "\n")
	lineFor := func(repo string) string {
		for _, l := range lines {
			if strings.HasPrefix(l, "- `"+repo+"` **gate** ") {
				return l
			}
		}
		t.Fatalf("no gate line for %s in:\n%s", repo, text)
		return ""
	}
	for repo, want := range map[string]string{
		"gated":      "approval_authorities [lucas, antony], approval_model merge-gate at origin/dev",
		"undeclared": "approval_authorities [lucas], approval_model not declared at origin/dev",
		"unbound":    "no binding — origin/dev carries no .claude/repo-binding.yaml",
		"mainline":   "approval_authorities [lucas], approval_model pre-proposal at origin/main",
		"tracked":    "approval_authorities [antony], approval_model merge-gate at origin/release",
	} {
		if l := lineFor(repo); !strings.Contains(l, want) {
			t.Errorf("%s: gate line %q does not say %q", repo, l, want)
		}
	}
	for _, l := range lines {
		if !strings.Contains(l, "**gate**") {
			continue
		}
		for _, decoy := range []string{"delegated", "anyone", "local-head", "wrong-branch", "decoy"} {
			if strings.Contains(l, decoy) {
				t.Errorf("a gate line was read from the %q decoy, not the remote branch: %q", decoy, l)
			}
		}
	}
	if !strings.Contains(text, "## Approval gates") {
		t.Errorf("the gate group has no heading:\n%s", text)
	}
}

func TestGateIsUnknownWhenNoRemoteBranchResolves(t *testing.T) {
	root := t.TempDir()
	fixture{name: "orphan", branch: "elsewhere", binding: "approval_authorities: [lucas]\napproval_model: merge-gate\n"}.make(t, root)
	got, err := New(Options{WorkRoot: root, Repos: []string{"orphan"}}).Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	g := ofKind(got, signal.KindGate)
	if len(g) != 1 || g[0].Attrs["model"] != "unknown" || !strings.Contains(g[0].Title, "origin/dev") {
		t.Fatalf("a checkout with no candidate remote branch was not reported unknown: %+v", g)
	}
}

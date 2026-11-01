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

// fixtureRepo makes a repository whose origin/dev carries binding (none when
// empty), then rewrites the working tree's binding so a read of the checkout
// instead of origin/dev shows a different gate.
func fixtureRepo(t *testing.T, root, name, binding string) {
	t.Helper()
	dir := filepath.Join(root, name)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=dev"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, name, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	write("README", name+"\n")
	if binding != "" {
		write(".claude/repo-binding.yaml", binding)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "init")
	git("update-ref", "refs/remotes/origin/dev", "HEAD")
	write(".claude/repo-binding.yaml", "approval_authorities: [anyone]\napproval_model: delegated\n")
}

func TestSurveyPrintsEachRepositorysGateAtOriginDev(t *testing.T) {
	root := t.TempDir()
	fixtureRepo(t, root, "gated", "development_branch: dev\napproval_authorities: [lucas, antony]\napproval_model: merge-gate\n")
	fixtureRepo(t, root, "undeclared", "approval_authorities: [lucas]\n")
	fixtureRepo(t, root, "unbound", "")

	s := New(Options{WorkRoot: root, Repos: []string{"gated", "undeclared", "unbound"}})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ofKind(got, signal.KindGate)); n != 3 {
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
	} {
		if l := lineFor(repo); !strings.Contains(l, want) {
			t.Errorf("%s: gate line %q does not say %q", repo, l, want)
		}
	}
	for _, l := range lines {
		if strings.Contains(l, "**gate**") && (strings.Contains(l, "delegated") || strings.Contains(l, "anyone")) {
			t.Errorf("a gate line reads the checkout's working tree, not origin/dev: %q", l)
		}
	}
	if !strings.Contains(text, "## Approval gates") {
		t.Errorf("the gate group has no heading:\n%s", text)
	}
}

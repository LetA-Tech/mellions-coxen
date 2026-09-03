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

// Off a branch, `rev-parse --abbrev-ref HEAD` answers the literal string
// "HEAD", so every field the state word used to be derived from is populated
// and the checkout reads as healthy. It is not: `git pull --ff-only` there
// exits 1 without deploying anything, which makes it the sharpest case of a
// host that has stopped receiving what it installs while saying "present".
func TestDetachedHeadIsNotADeployableCheckout(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	deploy := filepath.Join(root, "deploy")
	run := gitRunner(t)

	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	run(origin, "init", "-q", "--bare", "-b", "dev")
	run(root, "clone", "-q", origin, deploy)
	run(deploy, "config", "user.email", "t@example.com")
	run(deploy, "config", "user.name", "t")
	run(deploy, "commit", "-q", "--allow-empty", "-m", "one")
	run(deploy, "push", "-q", "-u", "origin", "HEAD:dev")
	run(deploy, "checkout", "-q", "--detach", "HEAD")

	tree, ok := pluginreg.ReadTree(deploy)
	if !ok {
		t.Fatal("a checkout was not read as one")
	}
	// The independent side: git's own refusal, not this package's opinion of
	// it. If the deploy command starts succeeding here the case has stopped
	// being the one described above and the assertion below tests nothing.
	c := exec.Command("git", "-C", deploy, "pull", "--ff-only")
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := c.CombinedOutput(); err == nil {
		t.Fatalf("git pull --ff-only succeeded off a branch (%s), so this is not the case under test", out)
	}

	state, detail := treeState(tree)
	if state != "STOPPED" {
		t.Errorf("state = %q, want %q: the deploy command refuses here — %s", state, "STOPPED", detail)
	}
	if !strings.Contains(detail, "NO BRANCH") {
		t.Errorf("detail = %q: %q is a branch name to a reader, and there is no branch", detail, tree.Branch)
	}
}

// A branch read that failed and a checkout that is genuinely off a branch both
// leave Branch empty, and only one of them is a detached HEAD. Claiming the
// stronger of the two would print "NO BRANCH" about a checkout sitting on one
// and fail the command for it, which is the same defect as the one this change
// fixes — a word asserting something nobody established — pointed the other way.
func TestAFailedBranchReadIsNotADetachedHead(t *testing.T) {
	state, detail := treeState(pluginreg.Tree{
		Head:        "16b63c8",
		StatusKnown: true,
		Problems:    []string{"cannot read the branch, so whether it is on one is unknown: exit status 128"},
	})
	if state == "STOPPED" {
		t.Errorf("state = %q: a branch that could not be read is unknown, not a detached HEAD — %s", state, detail)
	}
	if state != "partial" {
		t.Errorf("state = %q, want %q: the read failed, so the question was not answered", state, "partial")
	}
	if strings.Contains(detail, "NO BRANCH") {
		t.Errorf("detail = %q: claims there is no branch, which no command established", detail)
	}
}

// treeState returning a word establishes what the function computes and
// nothing about what the command does with it: the collection is a switch in
// cmdDoctor that no unit test reaches. This drives the real entry point and
// asserts on the outcome #267 names — that a load path which has stopped
// deploying can make `mellions doctor` exit non-zero — rather than on the
// word. Other lines in a temp home are legitimately absent, so the assertion
// is on the "stopped deploying" bucket by name, which absence cannot produce.
func TestALoadPathThatStoppedDeployingFailsDoctor(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	deploy := filepath.Join(root, "deploy")
	run := gitRunner(t)

	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	run(origin, "init", "-q", "--bare", "-b", "dev")
	run(root, "clone", "-q", origin, deploy)
	run(deploy, "config", "user.email", "t@example.com")
	run(deploy, "config", "user.name", "t")
	run(deploy, "commit", "-q", "--allow-empty", "-m", "deployed")
	run(deploy, "push", "-q", "-u", "origin", "HEAD:dev")
	// The state: a commit the upstream does not have. `git pull --ff-only` is
	// quiet here until the upstream moves, which is why the exit code is the
	// only thing that carries it to anyone who did not read the output.
	run(deploy, "commit", "-q", "--allow-empty", "-m", "never pushed")

	tree, ok := pluginreg.ReadTree(deploy)
	if !ok || tree.Ahead != 1 {
		t.Fatalf("tree = %+v, ok = %v: want 1 ahead, the state this test is about", tree, ok)
	}

	home := fakeRuntime(t, "directory", deploy, filepath.Join(root, "copy"))
	t.Setenv("HOME", home)
	// Kept off the real record: the command reads an assignment store and a
	// config under its own home, and a test must not write to the host's.
	t.Setenv("MELLIONS_HOME", filepath.Join(home, "mellions"))

	err := cmdDoctor(context.Background(), nil)
	if err == nil {
		t.Fatal("doctor exited 0 with a load path that has stopped deploying")
	}
	if !strings.Contains(err.Error(), "stopped deploying: load path commit") {
		t.Errorf("error = %q, want it to name the load path commit as stopped deploying", err)
	}
}

func gitRunner(t *testing.T) func(dir string, args ...string) string {
	t.Helper()
	return func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out)
		}
		return string(out)
	}
}

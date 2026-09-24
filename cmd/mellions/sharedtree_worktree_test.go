// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// A repository that requires its worktrees under `.claude/worktrees/` puts
// every lane lexically inside the shared checkout. The estate this installation
// actually builds has to tell the lane from the checkout around it, from the
// disk, for every form the issue's payloads took: standing in the lane, and
// `git -C <lane>` from the checkout. The checkout itself stays refused, and so
// does a separate repository nested in it, which nothing establishes is a lane,
// and a directory whose gitfile writes the checkout's own index.
func TestALinkedWorktreeNestedInTheCheckoutIsNotTheCheckout(t *testing.T) {
	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = withoutGitEnv(os.Environ())
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	checkout := t.TempDir()
	git(checkout, "init", "-q", "--initial-branch=main")
	git(checkout, "config", "user.email", "t@t")
	git(checkout, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(checkout, "f.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(checkout, "add", "-A")
	git(checkout, "commit", "-qm", "base")
	lane := filepath.Join(checkout, ".claude", "worktrees", "892-eval")
	git(checkout, "worktree", "add", "-q", "-b", "lane", lane)
	sub := filepath.Join(checkout, "internal")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	vendored := filepath.Join(checkout, "third_party", "other")
	if err := os.MkdirAll(vendored, 0o755); err != nil {
		t.Fatal(err)
	}
	git(vendored, "init", "-q", "--initial-branch=main")
	// A gitfile back at the checkout's own git directory: git reports it as its
	// own top level, but it shares the checkout's index.
	alias := filepath.Join(checkout, "alias")
	if err := os.MkdirAll(alias, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alias, ".git"),
		[]byte("gitdir: "+filepath.Join(checkout, ".git")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A gitfile borrowing the nested lane's git directory: its own top level
	// and git directory, the same common directory, and its writes land in the
	// checkout's files.
	borrow := filepath.Join(checkout, "borrow")
	if err := os.MkdirAll(borrow, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(borrow, ".git"),
		[]byte("gitdir: "+filepath.Join(checkout, ".git", "worktrees", "892-eval")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := sharedEstate(&Config{})
	e.Shared = []sharedtree.Checkout{{Repo: "leankit", Dir: checkout}}
	e.Lanes = nil
	e.Lane = nil

	deny := func(cwd, command string) string {
		raw, err := json.Marshal(map[string]any{
			"session_id": "s", "tool_name": "Bash", "cwd": cwd,
			"tool_input": map[string]string{"command": command},
		})
		if err != nil {
			t.Fatal(err)
		}
		return sharedtree.Deny(raw, e)
	}
	for _, c := range []struct {
		name, cwd, command string
		deny               bool
	}{
		{"the checkout itself", checkout, "git add -- f.txt", true},
		{"a directory of the checkout", sub, "git add -- .", true},
		{"a separate repository nested in the checkout", vendored, "git add -- .", true},
		{"a gitfile pointing at the checkout's own git directory", alias, "git add -- .", true},
		{"git -C that gitfile from the checkout", checkout, "git -C " + alias + " add -- .", true},
		{"a gitfile borrowing the lane's git directory", borrow, "git checkout -- .", true},
		{"git -C that borrowing gitfile from the checkout", checkout, "git -C " + borrow + " reset --hard", true},
		{"standing in the nested lane", lane, "git add -- f.txt", false},
		{"a directory of the nested lane", lane, "cd sub-not-yet-made && git add -- .", false},
		{"git -C the nested lane from the checkout", checkout, "git -C " + lane + " add -- f.txt", false},
		{"git -C the checkout from the nested lane", lane, "git -C " + checkout + " add -- f.txt", true},
	} {
		got := deny(c.cwd, c.command)
		if c.deny && got == "" {
			t.Errorf("%s: %q was allowed into the shared checkout", c.name, c.command)
		}
		if !c.deny && got != "" {
			t.Errorf("%s: %q was refused, but it writes a linked worktree of its own:\n%s",
				c.name, c.command, got)
		}
	}
}

// Anything git cannot resolve is "cannot tell", and cannot-tell keeps the
// checkout protected rather than exempting it on a guess.
func TestInOtherTreeAnswersNoWhereGitCannotTell(t *testing.T) {
	plain := t.TempDir()
	if inOtherTree(filepath.Join(plain, "a"), plain) {
		t.Error("two paths git resolves into no repository were called different trees")
	}
	if inOtherTree("", plain) {
		t.Error("an empty path was called a different tree")
	}
}

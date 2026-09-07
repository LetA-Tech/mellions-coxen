// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The shared-tree package decides from the command line and the configuration
// alone, so the one thing that actually reads the disk is here — and the
// package's tests, which inject a probe, cannot execute it or the wiring that
// installs it. Both are exactly where this can fail silently: a probe that
// never reports dirty, or a probe that is never installed, each leaves the
// deployment exemption unconditional again with every unit test still green.
func TestTreeIsDirtyReadsARealWorkingTree(t *testing.T) {
	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	repo := t.TempDir()
	git(repo, "init", "-q", "--initial-branch=main")
	git(repo, "config", "user.email", "t@t")
	git(repo, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", "-A")
	git(repo, "commit", "-qm", "base")

	sub := filepath.Join(repo, "internal")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if treeIsDirty(repo) {
		t.Fatal("a committed tree reported dirty, which refuses the deployment step")
	}

	// A modification git would stash under autostash.
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("v1\nlocal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !treeIsDirty(repo) {
		t.Error("an unstaged modification reported clean")
	}
	// git status answers for the whole tree whichever directory it is asked
	// from, and a pull typed one directory in still writes the whole tree.
	if !treeIsDirty(sub) {
		t.Error("asked from a subdirectory, a dirty tree reported clean")
	}
	git(repo, "checkout", "--", "f.txt")

	// Untracked counts: it is somebody's unfinished work in a tree nobody owns.
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Error(err)
	}
	if !treeIsDirty(repo) {
		t.Error("an untracked file reported clean")
	}

	// Cannot tell is not dirty: guessing dirty blocks the only sanctioned way
	// to install a fix.
	if treeIsDirty("") {
		t.Error("an empty directory reported dirty")
	}
	if treeIsDirty(filepath.Join(t.TempDir(), "not-a-repo")) {
		t.Error("a directory that is not a repository reported dirty")
	}
}

// The probe is worth nothing if nothing installs it, and no test in the
// sharedtree package can see that: they all build their own Estate.
func TestTheDeploymentProbeIsWiredIntoTheEstate(t *testing.T) {
	if sharedEstate(&Config{}).Dirty == nil {
		t.Fatal("sharedEstate installs no Dirty probe, so the deployment exemption is " +
			"unconditional again and every sharedtree test still passes")
	}
}

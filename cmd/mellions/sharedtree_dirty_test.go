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

	// Untracked does NOT count, and that is measured rather than assumed:
	// autostash does not pass --include-untracked, so an untracked file is
	// never stashed and never reapplied -- it cannot produce the corruption
	// this probe exists to catch. Counting it would refuse the deployment over
	// a stray build artefact, which is the defect the exemption was added for.
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Error(err)
	}
	if treeIsDirty(repo) {
		t.Error("an untracked file reported dirty, which refuses the deployment over a " +
			"stray artefact that autostash would never touch")
	}
	// A staged change is tracked, and autostash does carry it.
	stage := exec.Command("git", "add", "new.txt")
	stage.Dir = repo
	if out, err := stage.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if !treeIsDirty(repo) {
		t.Error("a staged change reported clean")
	}

	// GIT_DIR outranks -C, so an inherited one would answer for another
	// repository: exit 0, empty output, no error to notice.
	other := t.TempDir()
	oc := exec.Command("git", "init", "-q", "--initial-branch=main")
	oc.Dir = other
	if out, err := oc.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	if !treeIsDirty(repo) {
		t.Error("an inherited GIT_DIR/GIT_WORK_TREE made the probe answer for another " +
			"repository, so a dirty load path reads clean")
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
//
// Asserted by BEHAVIOUR against a real repository, not by `!= nil`. A nil check
// catches only the mutation that deletes the field; it passes a probe that is
// present, wired, and permanently answering "clean" -- which leaves the
// exemption unconditional with the whole suite green. That is the weakest
// available mutation of this line, so it is not the one to test against.
func TestTheDeploymentProbeIsWiredIntoTheEstate(t *testing.T) {
	probe := sharedEstate(&Config{}).Dirty
	if probe == nil {
		t.Fatal("sharedEstate installs no Dirty probe, so the deployment exemption is " +
			"unconditional again and every sharedtree test still passes")
	}
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "--initial-branch=main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		c := exec.Command("git", args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "base"}} {
		c := exec.Command("git", args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if probe(repo) {
		t.Error("the installed probe reports a committed tree dirty")
	}
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("v1\nlocal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !probe(repo) {
		t.Fatal("the probe sharedEstate installs reports a modified tree clean, so the " +
			"deployment exemption is unconditional in production while every test passes")
	}
}

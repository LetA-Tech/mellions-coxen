package assignment

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// A source checkout is long-lived and nothing updates it: lanes work in
// worktrees, so nobody pulls in it. Cutting from its local branch head starts
// the lane behind the tree everyone else shares, and the lane then re-verifies
// its work item against a head nobody has.
func TestALaneIsCutFromTheRemoteHeadNotTheStaleLocalOne(t *testing.T) {
	upstream := realRepo(t)
	source := t.TempDir() + "/clone"
	if out, err := exec.Command("git", "clone", "-q", upstream, source).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	stale := gitOut(t, source, "rev-parse", "HEAD")

	// The world moves on. The clone is never pulled — only fetched by the lane.
	gitOut(t, upstream, "commit", "-q", "--allow-empty", "-m", "moved on")
	ahead := gitOut(t, upstream, "rev-parse", "HEAD")
	if stale == ahead {
		t.Fatal("fixture did not advance the upstream")
	}

	a := mustOpen(t, newStore(t), OpenOptions{ID: "l1", Repo: "r", Source: source, Objective: "o", Because: "b"})
	if a.Base != ahead {
		t.Fatalf("cut from %s, want the remote head %s (the stale local head is %s)", a.Base, ahead, stale)
	}
	if head := gitOut(t, a.Worktree, "rev-parse", "HEAD"); head != ahead {
		t.Fatalf("worktree HEAD = %s, want %s", head, ahead)
	}
	// The pin is the point: a later session asking what this was verified
	// against has to be able to read the answer, including what was declined.
	if !strings.Contains(a.BasePin, "origin/") || !strings.Contains(a.BasePin, "fetched") {
		t.Fatalf("pin does not name the ref it used: %q", a.BasePin)
	}
	if !strings.Contains(a.BasePin, short(stale)) {
		t.Fatalf("pin does not name the declined local head %s: %q", short(stale), a.BasePin)
	}
	if !strings.Contains(a.Text(a.OpenedAt), a.BasePin) {
		t.Fatal("the pin is recorded but never shown")
	}
}

// A checkout with no remote is a supported way to work, not a failure: the lane
// cuts from the local head and the record says that is what it did.
func TestALaneWithNoRemoteFallsBackToTheLocalHeadAndSaysSo(t *testing.T) {
	source := realRepo(t)
	local := gitOut(t, source, "rev-parse", "HEAD")
	a := mustOpen(t, newStore(t), OpenOptions{ID: "l2", Repo: "r", Source: source, Objective: "o", Because: "b"})
	if a.Base != local {
		t.Fatalf("base = %s, want %s", a.Base, local)
	}
	if !strings.Contains(a.BasePin, "no remote branch") {
		t.Fatalf("pin = %q, want it to say why the local head was used", a.BasePin)
	}
}

// An explicit -base is the session saying it means this commit. Nothing is
// fetched, and the record holds the commit rather than a name that moves.
func TestAnExplicitBaseIsResolvedAndNotOverridden(t *testing.T) {
	upstream := realRepo(t)
	source := t.TempDir() + "/clone"
	if out, err := exec.Command("git", "clone", "-q", upstream, source).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	pinned := gitOut(t, source, "rev-parse", "HEAD")
	gitOut(t, upstream, "commit", "-q", "--allow-empty", "-m", "moved on")

	a := mustOpen(t, newStore(t), OpenOptions{ID: "l3", Repo: "r", Source: source, Objective: "o", Because: "b", BaseRef: "HEAD"})
	if a.Base != pinned {
		t.Fatalf("base = %s, want the named commit %s", a.Base, pinned)
	}
	if !strings.Contains(a.BasePin, "-base") {
		t.Fatalf("pin = %q, want it to say the base was given", a.BasePin)
	}
}

// clonedFrom returns a clone of upstream, checked out on whatever branch the
// remote's HEAD names — the shape every shared checkout has.
func clonedFrom(t *testing.T, upstream string) string {
	t.Helper()
	source := t.TempDir() + "/clone"
	if out, err := exec.Command("git", "clone", "-q", upstream, source).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	return source
}

// withDevBranch adds a dev branch carrying one commit main does not, leaves
// upstream back on main, and returns the two heads.
func withDevBranch(t *testing.T, upstream, branch string) (head, mainHead string) {
	t.Helper()
	gitOut(t, upstream, "checkout", "-q", "-b", branch)
	gitOut(t, upstream, "commit", "-q", "--allow-empty", "-m", "only on "+branch)
	head = gitOut(t, upstream, "rev-parse", "HEAD")
	gitOut(t, upstream, "checkout", "-q", "main")
	mainHead = gitOut(t, upstream, "rev-parse", "HEAD")
	if head == mainHead {
		t.Fatalf("fixture: %s and main are the same commit", branch)
	}
	return head, mainHead
}

// A source checkout can sit on a release branch while lanes start from the
// repository's working branch.
func TestALaneIsCutFromTheWorkingBranchNotTheBranchTheCheckoutSitsOn(t *testing.T) {
	upstream := realRepo(t)
	dev, mainHead := withDevBranch(t, upstream, "dev")

	source := clonedFrom(t, upstream)
	if b := gitOut(t, source, "rev-parse", "--abbrev-ref", "HEAD"); b != "main" {
		t.Fatalf("fixture: the checkout sits on %q, want main", b)
	}

	a := mustOpen(t, newStore(t), OpenOptions{ID: "w1", Repo: "r", Source: source, Objective: "o", Because: "b"})
	if a.Base != dev {
		t.Fatalf("cut from %s, want origin/dev %s (the checkout's own branch is main %s)", a.Base, dev, mainHead)
	}
	if head := gitOut(t, a.Worktree, "rev-parse", "HEAD"); head != dev {
		t.Fatalf("worktree HEAD = %s, want %s", head, dev)
	}
	if !strings.Contains(a.BasePin, "origin/dev") {
		t.Fatalf("pin does not name the branch it used: %q", a.BasePin)
	}
}

// A repository saying where its work happens is evidence; the estate
// convention is an assumption. The declaration wins, and it is read from the
// checkout rather than guessed.
func TestTheRepositorysOwnDeclarationBeatsTheConvention(t *testing.T) {
	upstream := realRepo(t)
	declared, _ := withDevBranch(t, upstream, "release-train")
	dev, _ := withDevBranch(t, upstream, "dev")
	if declared == dev {
		t.Fatal("fixture: the two candidate branches are the same commit")
	}
	if err := os.MkdirAll(filepath.Join(upstream, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstream, ".claude", "repo-binding.yaml"),
		[]byte("repo: example-org/r\ndevelopment_branch: release-train\nrelease_branch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, upstream, "add", ".claude/repo-binding.yaml")
	gitOut(t, upstream, "commit", "-q", "-m", "declare the working branch")

	source := clonedFrom(t, upstream)
	a := mustOpen(t, newStore(t), OpenOptions{ID: "w2", Repo: "r", Source: source, Objective: "o", Because: "b"})
	if a.Base != declared {
		t.Fatalf("cut from %s, want the declared release-train %s (dev is %s)", a.Base, declared, dev)
	}
	if !strings.Contains(a.BasePin, "origin/release-train") {
		t.Fatalf("pin does not name the declared branch: %q", a.BasePin)
	}
}

// The pattern has to be able to miss. A declaration naming a branch the remote
// does not carry falls through to the convention rather than failing the lane,
// and a file with no such key is the same answer as no file at all.
func TestAnUnusableDeclarationFallsThroughToTheConvention(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{"names a branch the remote does not carry", "development_branch: no-such-branch\n"},
		{"carries no such key", "repo: example-org/r\nrelease_branch: main\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := realRepo(t)
			dev, _ := withDevBranch(t, upstream, "dev")
			if err := os.MkdirAll(filepath.Join(upstream, ".claude"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(upstream, ".claude", "repo-binding.yaml"), []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			gitOut(t, upstream, "add", ".claude/repo-binding.yaml")
			gitOut(t, upstream, "commit", "-q", "-m", "binding")
			// dev must carry the binding too, or "cut from dev" would be
			// satisfied by a commit that predates the fixture.
			gitOut(t, upstream, "checkout", "-q", "dev")
			gitOut(t, upstream, "merge", "-q", "--no-edit", "main")
			dev = gitOut(t, upstream, "rev-parse", "HEAD")
			gitOut(t, upstream, "checkout", "-q", "main")

			source := clonedFrom(t, upstream)
			a := mustOpen(t, newStore(t), OpenOptions{ID: "w3", Repo: "r", Source: source, Objective: "o", Because: "b"})
			if a.Base != dev {
				t.Fatalf("cut from %s, want origin/dev %s", a.Base, dev)
			}
		})
	}
}

// An explicit -base still means that commit, whatever branches exist.
func TestAnExplicitBaseIsNotOverriddenByTheWorkingBranch(t *testing.T) {
	upstream := realRepo(t)
	dev, mainHead := withDevBranch(t, upstream, "dev")
	source := clonedFrom(t, upstream)

	a := mustOpen(t, newStore(t), OpenOptions{ID: "w4", Repo: "r", Source: source, Objective: "o", Because: "b", BaseRef: "HEAD"})
	if a.Base != mainHead {
		t.Fatalf("base = %s, want the named commit %s (dev is %s)", a.Base, mainHead, dev)
	}
}

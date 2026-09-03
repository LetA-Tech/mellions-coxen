// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitIn runs git in dir and fails the test on a non-zero exit.
func gitIn(t *testing.T, dir string, args ...string) string {
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

// repoWithRemote is realRepo with an actual remote behind it. The claim under
// test is about remote refs, so a repository with no remote cannot exercise it.
func repoWithRemote(t *testing.T) string {
	t.Helper()
	repo := realRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, filepath.Dir(bare), "init", "-q", "--bare", "-b", "main", bare)
	gitIn(t, repo, "remote", "add", "origin", bare)
	gitIn(t, repo, "push", "-q", "origin", "main")
	gitIn(t, repo, "fetch", "-q", "origin")
	return repo
}

// commitIn adds one commit to the lane worktree so there is something to
// publish or not publish.
func commitIn(t *testing.T, tree, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(tree, name), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, tree, "add", name)
	gitIn(t, tree, "commit", "-q", "-m", "add "+name)
}

// TestUnpushedCountsWhatNoRemoteHasNotOneUpstream is #268: the count is read by
// a session deciding whether discarding a worktree loses work, and by Abandon,
// which writes it into the permanent Discarded.Held record. Measuring it
// against @{upstream} alone answered a question about every remote ref from
// one of them.
//
// The three cases are the three shapes the lane branch can be in. Only the
// first was wrong before the fix; the other two are here so the fix cannot buy
// the first by giving up the others.
func TestUnpushedCountsWhatNoRemoteHasNotOneUpstream(t *testing.T) {
	// Published under another name, with no upstream configured. The commit is
	// on the remote; nothing is at risk. Before the fix, @{upstream}..HEAD
	// failed and the fallback counted the lane against its local base, so this
	// reported the work as unpublished.
	t.Run("pushed under another name", func(t *testing.T) {
		repo := repoWithRemote(t)
		s := newStore(t)
		a := mustOpen(t, s, OpenOptions{ID: "u1", Repo: "r", Source: repo, Objective: "o", Because: "b"})
		commitIn(t, a.Worktree, "one.txt")
		gitIn(t, a.Worktree, "push", "-q", "origin", "HEAD:refs/heads/somebody-elses-name")
		gitIn(t, a.Worktree, "fetch", "-q", "origin")

		u, err := s.Unsaved(a)
		if err != nil {
			t.Fatal(err)
		}
		if u.Unpushed != 0 {
			t.Errorf("Unpushed = %d, want 0: the commit is on origin under another name, "+
				"so a reader is being told published work is unpublished", u.Unpushed)
		}
	})

	// The ordinary at-risk shape: an upstream is configured and a commit was
	// never pushed anywhere. This must still count, or the fix has widened the
	// answer to zero and told a reader nothing is at risk when something is.
	t.Run("upstream set and genuinely unpushed", func(t *testing.T) {
		repo := repoWithRemote(t)
		s := newStore(t)
		a := mustOpen(t, s, OpenOptions{ID: "u2", Repo: "r", Source: repo, Objective: "o", Because: "b"})
		gitIn(t, a.Worktree, "push", "-q", "-u", "origin", "HEAD:refs/heads/"+a.Branch)
		gitIn(t, a.Worktree, "branch", "-q", "--set-upstream-to=origin/"+a.Branch)
		commitIn(t, a.Worktree, "after-push.txt")

		u, err := s.Unsaved(a)
		if err != nil {
			t.Fatal(err)
		}
		if u.Unpushed != 1 {
			t.Errorf("Unpushed = %d, want 1: one commit exists on no remote ref", u.Unpushed)
		}
	})

	// A checkout with no remote-tracking refs at all. Every commit on the lane
	// is genuinely unpublished, and the count must be of the lane rather than
	// of the whole history — `HEAD --not --remotes` with nothing to subtract
	// counts every commit the repository has ever had.
	t.Run("no remote at all counts the lane, not the history", func(t *testing.T) {
		repo := realRepo(t)
		commitIn(t, repo, "second.txt")
		commitIn(t, repo, "third.txt")
		s := newStore(t)
		a := mustOpen(t, s, OpenOptions{ID: "u3", Repo: "r", Source: repo, Objective: "o", Because: "b"})
		commitIn(t, a.Worktree, "lane.txt")

		u, err := s.Unsaved(a)
		if err != nil {
			t.Fatal(err)
		}
		if u.Unpushed != 1 {
			t.Errorf("Unpushed = %d, want 1: the lane has one commit and the repository "+
				"has four, so anything but 1 is counting the wrong range", u.Unpushed)
		}
	})
}

// TestUnpushedWithNoBaseToBoundTheRange covers the branch taken when the base
// cannot bound the count: recorded empty, or recorded and since rewritten or
// pruned. An independent read found this branch reporting a repository's whole
// history where the code it replaced reported nothing — a lane that was safe
// under the old measurement becoming the loudest one under the new — and found
// it by poisoning the branch and watching the whole suite stay green.
//
// The two subtests are the two things that can be left to subtract.
func TestUnpushedWithNoBaseToBoundTheRange(t *testing.T) {
	// Nothing to subtract: no base, and no remote-tracking ref either. The
	// count cannot be taken without becoming the repository's history, so it
	// is not taken. Silence, not a number nobody can act on.
	t.Run("no base and no remote reports nothing", func(t *testing.T) {
		repo := realRepo(t)
		commitIn(t, repo, "second.txt")
		commitIn(t, repo, "third.txt")
		s := newStore(t)
		a := mustOpen(t, s, OpenOptions{ID: "n1", Repo: "r", Source: repo, Objective: "o", Because: "b"})
		commitIn(t, a.Worktree, "lane.txt")

		for _, base := range []string{"", "0000000000000000000000000000000000000000"} {
			b := *a
			b.Base = base
			u, err := s.Unsaved(&b)
			if err != nil {
				t.Fatal(err)
			}
			if u.Unpushed != 0 {
				t.Errorf("base=%q: Unpushed = %d, want 0: with no base and no remote-tracking "+
					"ref there is nothing to subtract, so any count here is the whole history",
					base, u.Unpushed)
			}
		}
	})

	// A remote exists, so `--remotes` alone bounds the count even with no
	// usable base. Two commits are on origin and one is not.
	t.Run("no base but a remote still counts what the remote lacks", func(t *testing.T) {
		repo := repoWithRemote(t)
		s := newStore(t)
		a := mustOpen(t, s, OpenOptions{ID: "n2", Repo: "r", Source: repo, Objective: "o", Because: "b"})
		commitIn(t, a.Worktree, "published.txt")
		gitIn(t, a.Worktree, "push", "-q", "origin", "HEAD:refs/heads/published")
		gitIn(t, a.Worktree, "fetch", "-q", "origin")
		commitIn(t, a.Worktree, "not-published.txt")

		b := *a
		b.Base = ""
		u, err := s.Unsaved(&b)
		if err != nil {
			t.Fatal(err)
		}
		if u.Unpushed != 1 {
			t.Errorf("Unpushed = %d, want 1: origin holds everything but the last commit, "+
				"and --remotes is enough to bound the count without a base", u.Unpushed)
		}
	})
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package continuity

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// TestPublicationIsMeasuredAgainstRealRemoteTrackingRefs exercises the Git
// range whose semantics the fake-command tests cannot establish.
func TestPublicationIsMeasuredAgainstRealRemoteTrackingRefs(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	repo := filepath.Join(root, "repo")
	run := realGit(t)
	run(root, "init", "-q", "--bare", "-b", "dev", origin)
	run(root, "clone", "-q", origin, repo)
	run(repo, "config", "user.name", "Test Engineer")
	run(repo, "config", "user.email", "engineer@example.invalid")
	writeCommit(t, repo, "base", "base")
	run(repo, "push", "-q", "-u", "origin", "HEAD:dev")
	base := strings.TrimSpace(run(repo, "rev-parse", "HEAD"))
	run(repo, "switch", "-q", "-c", "mellions/test-lane")
	writeCommit(t, repo, "lane", "published")
	run(repo, "push", "-q", "origin", "HEAD:refs/heads/mellions/test-lane")

	a := &assignment.Assignment{
		ID: "test-lane", Repo: "example-service", Branch: "mellions/test-lane",
		Worktree: repo, Source: repo, Base: base,
	}
	observed := Look(context.Background(), a, gitFromCommand, Tracker{})
	published, ok := fact(observed, "unpublished")
	if !ok || !strings.HasPrefix(published.Value, "none in "+base+"..HEAD") {
		t.Fatalf("pushed without an upstream = %+v, want no unpublished commits in the bounded range", published)
	}

	writeCommit(t, repo, "lane", "local")
	observed = Look(context.Background(), a, gitFromCommand, Tracker{})
	local, ok := fact(observed, "unpublished")
	if !ok || !strings.HasPrefix(local.Value, "1 commit(s) in "+base+"..HEAD") {
		t.Fatalf("one local commit = %+v, want one unpublished commit in the bounded range", local)
	}

	refs := strings.Fields(run(repo, "for-each-ref", "--format=%(refname)", "refs/remotes"))
	if len(refs) == 0 {
		t.Fatal("fixture has no remote-tracking refs to remove")
	}
	for _, ref := range refs {
		run(repo, "update-ref", "-d", ref)
	}
	a.Base = "base-that-does-not-resolve"
	observed = Look(context.Background(), a, gitFromCommand, Tracker{})
	if got, ok := fact(observed, "unpublished"); ok {
		t.Fatalf("missing base and no remote-tracking refs produced a publication claim: %+v", got)
	}
}

func realGit(t *testing.T) func(string, ...string) string {
	t.Helper()
	return func(dir string, args ...string) string {
		t.Helper()
		out, err := gitFromCommand(dir, args...)
		if err != nil {
			t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out)
		}
		return string(out)
	}
}

func gitFromCommand(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	return cmd.CombinedOutput()
}

func writeCommit(t *testing.T, repo, name, body string) {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := realGit(t)
	run(repo, "add", name)
	run(repo, "commit", "-q", "-m", body)
}

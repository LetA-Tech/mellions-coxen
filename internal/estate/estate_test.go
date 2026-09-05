// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package estate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// newRepo builds a checkout whose committed branch and whose working tree hold
// DIFFERENT content, which is the condition the package exists for: a real
// checkout left on a stale branch answers, plausibly and wrongly.
func newRepo(t *testing.T, dir, path, committed, onDisk string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "--quiet", "-b", "main")
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(s), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(committed)
	run("add", path)
	run("commit", "--quiet", "-m", "committed")
	// A remote-tracking ref without a remote: the read must resolve a ref, and
	// the test must not reach the network.
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	if onDisk != committed {
		write(onDisk)
		run("add", path)
	}
}

func TestReadAtARefDisagreesWithTheWorkingTreeItWasLeftOn(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	newRepo(t, dir, "go.mod", "require example.com/sdk v0.10.0\n", "require example.com/sdk v0.5.2\n")

	req := Request{Repos: map[string]string{"repo": dir}, Path: "go.mod",
		Grep: regexp.MustCompile(`example\.com/sdk`)}

	atRef := Read(context.Background(), req)
	if len(atRef) != 1 || atRef[0].Status != StatusOK {
		t.Fatalf("ref read: %+v", atRef)
	}
	if got := strings.Join(atRef[0].Matches, ""); !strings.Contains(got, "v0.10.0") {
		t.Errorf("ref read = %q, want the committed v0.10.0 — reading at a ref is the whole point", got)
	}

	req.Worktree = true
	tree := Read(context.Background(), req)
	if got := strings.Join(tree[0].Matches, ""); !strings.Contains(got, "v0.5.2") {
		t.Errorf("worktree read = %q, want the on-disk v0.5.2", got)
	}
	if tree[0].Ref != "WORKING TREE" {
		t.Errorf("worktree Ref = %q, want it labelled — an unlabelled working-tree answer is "+
			"indistinguishable from a ref answer, which is the defect", tree[0].Ref)
	}

	// The assertion that carries the package: the two disagree. If they ever
	// agree here the fixture stopped reproducing the condition and neither
	// assertion above means anything.
	if strings.Join(atRef[0].Matches, "") == strings.Join(tree[0].Matches, "") {
		t.Fatal("ref and working tree returned the same answer; the fixture no longer reproduces a stale checkout")
	}
}

func TestEveryRepositoryProducesARowIncludingTheOnesNotOnDisk(t *testing.T) {
	root := t.TempDir()
	present := filepath.Join(root, "present")
	newRepo(t, present, "go.mod", "module a\n", "module a\n")

	req := Request{Path: "go.mod", Repos: map[string]string{
		"present":      present,
		"absent":       filepath.Join(root, "nope"),
		"unconfigured": "",
	}}
	got := Read(context.Background(), req)
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3 — a repository that cannot be read must still be reported, "+
			"or 'nothing found' is indistinguishable from 'never opened'", len(got))
	}
	by := map[string]Result{}
	for _, r := range got {
		by[r.Repo] = r
	}
	if by["present"].Status != StatusOK {
		t.Errorf("present = %s, want ok", by["present"].Status)
	}
	for _, n := range []string{"absent", "unconfigured"} {
		if by[n].Status != StatusNoCheckout {
			t.Errorf("%s = %s, want %s", n, by[n].Status, StatusNoCheckout)
		}
		if by[n].Status.Measured() {
			t.Errorf("%s counted as measured; a host fact must never count as an answer", n)
		}
	}
}

func TestSummaryRefusesToLetUnmeasuredRepositoriesReadAsAnswers(t *testing.T) {
	rs := []Result{
		{Repo: "a", Status: StatusOK},
		{Repo: "b", Status: StatusNoPath},
		{Repo: "c", Status: StatusNoCheckout},
		{Repo: "d", Status: StatusNoRef},
	}
	s := Summarize(rs)
	if s.Measured != 2 || s.Unmeasured != 2 {
		t.Fatalf("measured=%d unmeasured=%d, want 2 and 2", s.Measured, s.Unmeasured)
	}
	line := s.Line()
	for _, want := range []string{"2 measured", "2 NOT measured", "no-checkout", "no-such-ref"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q omits %q — the count a reader acts on must name what it excludes", line, want)
		}
	}

	// The other direction, or the line above would be printed over a complete
	// read and teach a reader to ignore it.
	all := Summarize([]Result{{Status: StatusOK}, {Status: StatusNoPath}})
	if strings.Contains(all.Line(), "NOT measured") {
		t.Errorf("complete read reported as incomplete: %q", all.Line())
	}
}

// StatusNoPath is an answer about the repository; every other non-OK status is
// a fact about this host. Conflating them is how "no repository uses this"
// gets reported from four repositories that were never opened.
func TestNoPathCountsAsMeasuredAndHostFailuresDoNot(t *testing.T) {
	for _, tc := range []struct {
		s    Status
		want bool
	}{
		{StatusOK, true},
		{StatusNoPath, true},
		{StatusNoCheckout, false},
		{StatusNoRef, false},
		{StatusFetchFailed, false},
		{StatusError, false},
	} {
		if got := tc.s.Measured(); got != tc.want {
			t.Errorf("%s.Measured() = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestMissingPathIsReportedAsAnAnswerNotAnError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	newRepo(t, dir, "go.mod", "module a\n", "module a\n")

	got := Read(context.Background(), Request{
		Repos: map[string]string{"repo": dir}, Path: "does/not/exist.txt"})
	if got[0].Status != StatusNoPath {
		t.Fatalf("status = %s, want %s", got[0].Status, StatusNoPath)
	}
	if !got[0].Status.Measured() {
		t.Error("a ref that resolved and does not carry the path is an answer about the repository")
	}
}

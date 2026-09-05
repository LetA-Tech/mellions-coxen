package prbody

import (
	"os"
	"path/filepath"
	"testing"
)

// #27. A lane publishes with `cd <worktree> && gh pr create`, and the session
// directory stays the shared checkout throughout. Until Call carried a
// directory there was nothing downstream could use to read the tree the body is
// actually about.
func TestCalls_CarryTheDirectoryTheCommandMovedTo(t *testing.T) {
	const session = "/sessions/shared"
	lane := filepath.FromSlash("/lanes/fi915/tree")

	calls := Publishing(`cd `+lane+` && gh pr create --base dev --body 'text'`, session)

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].Dir != lane {
		t.Fatalf("Call.Dir = %q, want %q — the command's own cd is invisible, so a "+
			"citation would be resolved against the session checkout (#27)", calls[0].Dir, lane)
	}
}

// Control. A command that does not move stays in the session directory, so the
// fix cannot be "always report something other than cwd".
func TestCalls_WithoutACdStayInTheSessionDirectory(t *testing.T) {
	const session = "/sessions/shared"

	calls := Publishing(`gh issue comment 7 --body 'text'`, session)

	if len(calls) != 1 || calls[0].Dir != session {
		t.Fatalf("Dir = %q, want the session directory %q", calls[0].Dir, session)
	}
}

// Two calls on one command line can publish from two different trees. A single
// resolver built once cannot be right for both, which is why Dir is per call.
func TestCalls_EachCallCarriesItsOwnDirectory(t *testing.T) {
	a := filepath.FromSlash("/lanes/a")
	b := filepath.FromSlash("/lanes/b")

	calls := Publishing(
		`cd `+a+` && gh pr create --body 'first' && cd `+b+` && gh issue comment 3 --body 'second'`,
		"/sessions/shared")

	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if calls[0].Dir != a || calls[1].Dir != b {
		t.Fatalf("dirs = %q, %q; want %q, %q", calls[0].Dir, calls[1].Dir, a, b)
	}
}

// A relative cd composes against the directory already reached, not against the
// session directory.
func TestCalls_RelativeCdComposes(t *testing.T) {
	base := filepath.FromSlash("/sessions/shared")

	calls := Publishing(`cd sub && cd deeper && gh pr create --body 'x'`, base)

	want := filepath.Join(base, "sub", "deeper")
	if len(calls) != 1 || calls[0].Dir != want {
		t.Fatalf("Dir = %q, want %q", calls[0].Dir, want)
	}
}

// A target that cannot be established resets to the session directory. The
// alternative — carrying a fabricated path — resolves nothing, and a resolver
// that resolves nothing passes every citation silently, which is worse than the
// defect being fixed.
func TestCalls_UnknowableCdTargetFallsBackToTheSession(t *testing.T) {
	const session = "/sessions/shared"

	for _, command := range []string{
		`cd && gh pr create --body 'x'`,
		`cd - && gh pr create --body 'x'`,
		`cd ~someoneelse && gh pr create --body 'x'`,
	} {
		calls := Publishing(command, session)
		if len(calls) != 1 || calls[0].Dir != session {
			t.Fatalf("%q: Dir = %q, want the session directory %q", command, calls[0].Dir, session)
		}
	}
}

// --body-file is relative to where the command runs, for the same reason the
// citations are: `cd <worktree> && gh pr create -F body.md` reads the
// worktree's file.
func TestCalls_BodyFileResolvesAgainstTheCallDirectory(t *testing.T) {
	lane := t.TempDir()
	writeTestFile(t, filepath.Join(lane, "body.md"), "the lane's body\n")
	session := t.TempDir()
	writeTestFile(t, filepath.Join(session, "body.md"), "the session's body\n")

	calls := Publishing(`cd `+lane+` && gh pr create --body-file body.md`, session)

	if len(calls) != 1 || len(calls[0].Bodies) != 1 {
		t.Fatalf("got %d calls", len(calls))
	}
	if got := calls[0].Bodies[0]; got != "the lane's body\n" {
		t.Fatalf("body = %q, want the lane's — a relative --body-file was read from the "+
			"session directory, which is the same defect one layer over (#27)", got)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

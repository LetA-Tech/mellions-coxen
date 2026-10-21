// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unquotedBody cites the lane's neonlink.go:3 and quotes nothing under it, so
// any read of it the gate grades is refused.
func unquotedBody(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "b.md")
	if err := os.WriteFile(path, []byte("Root cause is `neonlink.go:3`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A body file named through an expansion was read as "" and graded clean,
// which published an unquoted citation. A variable this line never assigned,
// and a substitution, are refused as unread.
func TestCiteHook_RefusesABodyFileNamedThroughAShellExpansion(t *testing.T) {
	session, lane := citeFixture(t)
	s := t.TempDir()
	unquotedBody(t, s)

	for _, spec := range []string{"$S/b.md", "${S}/b.md", "$(echo " + s + "/b.md)", "`echo " + s + "/b.md`"} {
		reason := runCiteHook(t, session, "cd "+lane+" && gh pr create --base dev --body-file "+spec)
		if reason == "" {
			t.Errorf("--body-file %s: admitted; the gate graded an unread body as empty", spec)
			continue
		}
		// shellsplit ends a word at the space inside a backtick, so that spec is
		// named by its first word.
		named := strings.Fields(spec)[0]
		if !strings.Contains(reason, "--body-file "+named) || !strings.Contains(reason, "shell expansion") {
			t.Errorf("--body-file %s: denied, but the reason does not name the spec as an expansion:\n%s", spec, reason)
		}
	}
}

// With S assigned earlier in the same line, $S/b.md is the file gh reads, so
// it is graded: refused for the citation it does not quote.
func TestCiteHook_GradesABodyFileThroughAVariableTheLineAssigns(t *testing.T) {
	session, lane := citeFixture(t)
	s := t.TempDir()
	unquotedBody(t, s)

	for _, spec := range []string{"$S/b.md", "${S}/b.md"} {
		reason := runCiteHook(t, session, "S="+s+"; cd "+lane+" && gh pr create --base dev --body-file "+spec)
		if !strings.Contains(reason, "neonlink.go:3") || strings.Contains(reason, "cannot be checked") {
			t.Errorf("--body-file %s with S assigned: not graded for its citation: %q", spec, reason)
		}
	}
}

// The same file named literally is graded exactly as before: refused for the
// citation it does not quote, not for being unreadable.
func TestCiteHook_GradesTheSameBodyNamedLiterally(t *testing.T) {
	session, lane := citeFixture(t)
	body := unquotedBody(t, t.TempDir())

	reason := runCiteHook(t, session, "cd "+lane+" && gh pr create --base dev --body-file "+body)
	if !strings.Contains(reason, "neonlink.go:3") {
		t.Fatalf("the literal path was not graded for its citation: %q", reason)
	}
	if strings.Contains(reason, "cannot be checked") {
		t.Fatalf("a readable literal path was refused as unreadable: %s", reason)
	}
}

// A heredoc this command writes to an expanded path is the body gh reads, and
// the gate reads it rather than refusing the spec.
func TestCiteHook_AdmitsAHeredocWrittenToAnExpandedPath(t *testing.T) {
	session, lane := citeFixture(t)
	cmd := "cat > $S/b.md <<'EOF'\nRoot cause.\n\n`neonlink.go:3`\n\n```go\n" +
		"\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)\n```\nEOF\n" +
		"cd " + lane + " && gh pr create --base dev --body-file $S/b.md"

	if reason := runCiteHook(t, session, cmd); reason != "" {
		t.Fatalf("a correct body written by this command's heredoc was refused: %s", reason)
	}
}

func TestCiteHook_RefusesABodyFileThatIsNotThere(t *testing.T) {
	session, lane := citeFixture(t)
	gone := filepath.Join(t.TempDir(), "never-written.md")

	reason := runCiteHook(t, session, "cd "+lane+" && gh issue comment 1 --body-file "+gone)
	if !strings.Contains(reason, "--body-file "+gone) || !strings.Contains(reason, "no regular file") {
		t.Fatalf("a missing body file was not refused by name: %q", reason)
	}
}

// Stdin the command does not carry as a heredoc never reached the gate either.
func TestCiteHook_RefusesABodyOnStdinFromOutsideTheCommand(t *testing.T) {
	session, lane := citeFixture(t)

	reason := runCiteHook(t, session, "cd "+lane+" && printf 'Root cause is `neonlink.go:3`.' | gh pr create --base dev --body-file -")
	if !strings.Contains(reason, "--body-file -: stdin from outside a heredoc") {
		t.Fatalf("a piped body was not refused as unseen stdin: %q", reason)
	}
}

// A `cd` through a variable graded the body against the session tree. Where
// the body cites, that is refused; where it cites nothing, the tree is moot.
func TestCiteHook_RefusesACdThroughAnExpansionWhenTheBodyCites(t *testing.T) {
	session, lane := citeFixture(t)
	right := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")

	reason := runCiteHook(t, session, "cd $L && gh issue create --body-file "+right)
	if !strings.Contains(reason, "cd to ") || !strings.Contains(reason, "$L") {
		t.Fatalf("a cd through a variable was not refused for a citing body: %q", reason)
	}

	// Assigned earlier in the line, the cd is followed into the lane, where
	// the quoted line is right.
	if reason := runCiteHook(t, session, "L="+lane+"; cd $L && gh issue create --body-file "+right); reason != "" {
		t.Fatalf("a cd through a variable this line assigns was not followed: %s", reason)
	}

	// A relative body file after that cd is refused for the cd, not as missing.
	reason = runCiteHook(t, session, "cd $L && gh pr create --base dev --body-file body.md")
	if !strings.Contains(reason, "--body-file body.md: relative to a `cd` through a shell expansion") {
		t.Fatalf("a relative body file after a cd through a variable was not refused for the cd: %q", reason)
	}

	plain := filepath.Join(t.TempDir(), "plain.md")
	if err := os.WriteFile(plain, []byte("No citations here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if reason := runCiteHook(t, session, "cd $L && gh issue comment 1 --body-file "+plain); reason != "" {
		t.Fatalf("a body citing nothing was refused for its cd: %s", reason)
	}
}

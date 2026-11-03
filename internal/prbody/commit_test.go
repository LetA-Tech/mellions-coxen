// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDenyACommitMessage: GitHub resolves a closing keyword in a commit message
// once that commit reaches the default branch, so a commit on dev closes the
// issue at the promotion that carries it into main, before any proof.
func TestDenyACommitMessage(t *testing.T) {
	const commit = "in a git commit or merge message"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "msg.txt"), []byte("fix(parser): one lexer\n\nFixes #7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "m.txt"), []byte("Closes #8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		command string
		deny    bool
		want    string
	}{
		{"-m", `git commit -m "Closes #7"`, true, commit},
		{"-am cluster", `git commit -am "fix(parser): fixes #7"`, true, commit},
		{"-m glued", `git commit -m"Resolves #7"`, true, commit},
		{"second -m paragraph", `git commit -m "fix(parser): one lexer" -m "Fixes #7"`, true, commit},
		{"--message=", `git commit --message="Fixes #7"`, true, commit},
		{"--message value", `git commit --amend --message "Fixes #7"`, true, commit},
		{"--trailer", `git commit -m "one lexer" --trailer "Closes: #7"`, true, commit},
		{"owner/repo reference", `git commit -m "fixes LetA-Tech/mcfo-findata#7"`, true, "GitHub closes LetA-Tech/mcfo-findata#7"},
		{"-F file", `git commit -F msg.txt`, true, commit},
		{"--file=", `git commit --file=msg.txt`, true, commit},
		{"-F after git -C", `git -C sub commit -F m.txt`, true, "GitHub closes #8"},
		{"-F after cd", `cd sub && git commit -F m.txt`, true, "GitHub closes #8"},
		{"-F - from a heredoc", "git commit -F - <<'EOF'\nfix(parser): one lexer\n\nCloses #7\nEOF", true, commit},
		{"heredoc in a substitution", "git commit -m \"$(cat <<'EOF'\nfix(parser): one lexer\n\nFixes #7\nEOF\n)\"", true, commit},
		{"heredoc into the -F file", "cat > " + filepath.Join(dir, "new.txt") + " <<'EOF'\nFixes #9\nEOF\ngit commit -F " + filepath.Join(dir, "new.txt"), true, "GitHub closes #9"},
		{"env and path", `GIT_AUTHOR_NAME=x /usr/bin/git -c core.editor=true commit -m "Fixes #7"`, true, commit},
		{"git merge -m", `git merge --no-ff -m "Merge the parser fix (Fixes #7)" feature`, true, commit},
		{"chained with a create", `git add -A && git commit -m "Closes #7 in the fixtures" && gh pr create --base dev --body "Refs #7"`, true, commit},
		// A commit message is not Markdown and a negation does not stop
		// GitHub, so nothing in it is excused, as in a title.
		{"negated", `git commit -m "This does not close #7"`, true, commit},
		{"code span", "git commit -m \"docs: say `Fixes #7` closes at promotion\"", true, commit},

		// Nothing declared, or nothing that is a commit message.
		{"refs", `git commit -m "fix(parser): one lexer (Refs #7)"`, false, ""},
		{"not a keyword", `git commit -m "fixing #7 for the parser"`, false, ""},
		{"no message", `git commit --amend --no-edit`, false, ""},
		{"author is not a message", `git commit --author "Fixes #7 <a@b.c>" -m "one lexer"`, false, ""},
		{"-C reuses a commit", `git commit -C HEAD~1 -m "one lexer"`, false, ""},
		{"git log grep", `git log --grep "Fixes #7"`, false, ""},
		{"git tag message", `git tag -a v1 -m "Fixes #7"`, false, ""},
		{"echo of a commit", `echo 'git commit -m "Closes #7"'`, false, ""},
		{"grep for a keyword", `git log --format=%B | grep -n "Closes #"`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason := Deny(payload(t, dir, "Bash", c.command), stub)
			if (reason != "") != c.deny {
				t.Fatalf("deny=%v, want %v\ncommand: %s\nreason: %s", reason != "", c.deny, c.command, reason)
			}
			if !c.deny {
				return
			}
			for _, want := range []string{c.want, "Write \"Refs "} {
				if !strings.Contains(reason, want) {
					t.Errorf("denial does not say %q:\n%s", want, reason)
				}
			}
		})
	}
}

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

// TestDenyAMergeMessage: gh pr merge's --subject and --body are the merge or
// squash commit's message, and GitHub resolves a closing keyword in a commit
// message once that commit reaches the default branch, so a merge into dev
// closes the issue at the promotion that carries the commit into main.
func TestDenyAMergeMessage(t *testing.T) {
	const merge = "in a gh pr merge --subject or --body"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "msg.md"), []byte("Squash of the parser fix.\n\nFixes #7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		command string
		deny    bool
		want    string
	}{
		{"--subject", `gh pr merge 12 --merge --subject "Merge the parser fix (Fixes #7)"`, true, merge},
		{"-t is the subject", `gh pr merge 12 --squash -t "fix(parser): closes #7"`, true, merge},
		{"--subject=, glued", `gh pr merge 12 --squash --subject="Resolves #7"`, true, merge},
		{"--body", `gh pr merge 12 --merge --body "Fixes #7"`, true, merge},
		{"-b", `gh pr merge 12 --squash -b "fixes LetA-Tech/mcfo-findata#7"`, true, "GitHub closes LetA-Tech/mcfo-findata#7"},
		{"--body-file", `gh pr merge 12 --squash --body-file msg.md`, true, merge},
		{"-F - from a heredoc", "gh pr merge 12 --squash -F - <<'EOF'\nOne lexer.\n\nCloses #7\nEOF", true, merge},
		{"after cd", `cd /tmp && gh pr merge 12 --squash --subject "Fixes #7"`, true, merge},
		{"repo flag", `gh pr merge 12 -R LetA-Tech/mcfo-findata --squash -t "Fixes #7"`, true, merge},
		// A commit message is not Markdown and a negation does not stop
		// GitHub, so nothing in it is excused, as in a title.
		{"negated", `gh pr merge 12 --merge --body "This does not close #7"`, true, merge},
		{"code span", "gh pr merge 12 --merge --body \"docs: say `Fixes #7` closes at promotion\"", true, merge},
		// No exemption: the pull request's own body carries a whole-proof close.
		{"whole proof", `gh pr merge 12 --merge --subject "Fixes #7" --body "The diff is the whole proof."`, true, merge},

		// Nothing declared.
		{"refs", `gh pr merge 12 --squash --subject "fix(parser): one lexer (Refs #7)" --body "Refs #7"`, false, ""},
		{"no message flags", `gh pr merge 12 --squash --delete-branch`, false, ""},
		{"not a keyword", `gh pr merge 12 --squash --body "fixing #7 for the parser"`, false, ""},
		{"author email is not a body", `gh pr merge 12 --squash -A "fixes-7@example.com"`, false, ""},
		{"pr view", `gh pr view 12 --json body --jq .body | grep "Fixes #7"`, false, ""},
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

// TestMergeIsNotPublishing: a merge commit message is not a body a person
// reads on the tracker, so the citation reader does not take gh pr merge.
func TestMergeIsNotPublishing(t *testing.T) {
	if calls := Publishing(`gh pr merge 12 --squash --body "see a.go:1"`, "/repo"); len(calls) != 0 {
		t.Errorf("Publishing took gh pr merge: %+v", calls)
	}
	calls := Calls(`gh pr merge 12 --squash --subject s --body b`, "/repo")
	if len(calls) != 1 || calls[0].Verb != "merge" || len(calls[0].Titles) != 1 || len(calls[0].Bodies) != 1 {
		t.Errorf("Calls = %+v, want one merge call carrying its subject and body", calls)
	}
}

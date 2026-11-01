// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"strings"
	"testing"
)

// TestDenyATitle: a merge commit carries the title into its message, and GitHub
// resolves a closing keyword in a commit message once that commit reaches the
// default branch, so a pull request into dev closes the issue at the promotion
// that carries its merge commit into main.
func TestDenyATitle(t *testing.T) {
	const title = "in a pull request title"
	cases := []struct {
		name    string
		cwd     string
		command string
		deny    bool
		want    string
	}{
		{"into dev", "", `gh pr create --base dev --title "Keep Plaid's category on attested rows (Fixes #1582)" --body "Refs #1582"`, true, title},
		{"-t", "", `gh pr create --base dev -t "fix(parser): closes #7" --body "Refs #7"`, true, title},
		{"--title=, glued", "", `gh pr create --base dev --title="Resolves #7" --body "Refs #7"`, true, title},
		{"owner/repo reference", "", `gh pr create --base dev --title "fixes LetA-Tech/mcfo-findata#7"`, true, "GitHub closes LetA-Tech/mcfo-findata#7"},
		{"second --title", "", `gh pr create --base dev --title "draft" --title "Fixes #7"`, true, title},
		{"pr edit, no --base", "", `gh pr edit 12 --title "Fixes #7"`, true, title},
		{"pr new is create", "", `gh pr new --base dev --title "Fixes #7" --body "Refs #7"`, true, title},
		{"into main", "", `gh pr create --base main --title "Fixes #7" --body "Refs #7"`, true, title},
		{"no --base", "", `gh pr create --title "Fixes #7" --body "Refs #7"`, true, title},
		// A negated keyword is denied here as it is on the default branch,
		// since the title reaches the default branch in every merge commit.
		{"negated, into dev", "", `gh pr create --base dev --title "Does not close #7"`, true, title},

		// The whole-proof declaration keeps the keyword only where the merge
		// itself lands on the default branch.
		{"whole proof into main", "", `gh pr create --base main --title "Fix the README typo (fixes #7)" --body "The diff is the whole proof."`, false, ""},
		{"whole proof, no --base", "", `gh pr create --title "Fixes #7" --body "The diff is the whole proof."`, false, ""},
		{"whole proof, pr edit --base main", "", `gh pr edit 12 --base main --title "Fixes #7" --body "The diff is the whole proof."`, false, ""},
		{"whole proof, pr edit with no --base", "", `gh pr edit 12 --title "Fixes #7" --body "The diff is the whole proof."`, true, title},
		{"whole proof into dev", "", `gh pr create --base dev --title "Fixes #7" --body "The diff is the whole proof."`, true, title},
		{"whole proof beside another pull request", "", "gh pr create --base main --title \"Promote dev (fixes #1611)\" --body \"- #1612\n\nThe diff is the whole proof.\"", true, title},
		{"another pull request in the title", "", `gh pr create --base main --title "Fixes #7 (#1612)" --body "The diff is the whole proof."`, true, title},
		{"title's own reference named in the body", "", `gh pr create --base main --title "Fixes #7" --body "Refs #7. The diff is the whole proof."`, false, ""},
		{"proof claimed only in the title", "", `gh pr create --base main --title "Fixes #7: the diff is the whole proof" --body "Refs #7"`, true, title},
		{"proof claimed in a body gh does not send", "", `gh pr create --base main --title "Fixes #7" --body "The diff is the whole proof." --body "Promote dev: #1612 #1613"`, true, title},
		{"whole proof, default unreadable", nowhere, `gh pr create --base main --title "Fixes #7" --body "The diff is the whole proof."`, true, title},

		// Nothing declared.
		{"refs", "", `gh pr create --base dev --title "fix(parser): one lexer (Refs #7)" --body "Refs #7"`, false, ""},
		// A commit message is not Markdown: nothing in it is a quotation.
		{"code span", "", "gh pr create --base dev --title \"docs: say `Fixes #7` closes at promotion\" --body \"Refs #7\"", true, title},
		{"blockquote marker", "", `gh pr create --base dev --title "> Fixes #7"`, true, title},
		{"fence marker", "", "gh pr create --base dev --title '```Fixes #7'", true, title},
		{"not a keyword", "", `gh pr create --base dev --title "fixing #7 for the parser"`, false, ""},
		{"a body value is not a flag", "", `gh pr create --base dev --body -t "Fixes #7"`, false, ""},
		{"pr view", "", `gh pr view 12 --json title --jq '.title' | grep "Fixes #7"`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cwd := c.cwd
			if cwd == "" {
				cwd = t.TempDir()
			}
			reason := Deny(payload(t, cwd, "Bash", c.command), stub)
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

// TestNewIsCreate: gh pr new and gh issue new are gh's aliases of create, so
// a body they hand GitHub is read as create's is, by both readers.
func TestNewIsCreate(t *testing.T) {
	if r := Deny(payload(t, t.TempDir(), "Bash", `gh pr new --base main --body "Closes #7"`), stub); !strings.Contains(r, "into main, the default branch: GitHub closes #7") {
		t.Errorf("gh pr new body into main not denied as create's is: %q", r)
	}
	for _, cmd := range []string{`gh pr new --body x`, `gh issue new --body x`} {
		calls := Publishing(cmd, "/repo")
		if len(calls) != 1 || calls[0].Verb != "create" || len(calls[0].Bodies) != 1 {
			t.Errorf("%s: Publishing = %+v, want one create call carrying its body", cmd, calls)
		}
	}
}

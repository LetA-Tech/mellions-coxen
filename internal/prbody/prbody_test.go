// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nowhere is a working directory whose default branch cannot be read.
const nowhere = "/nowhere"

func stub(cwd, repo string) string {
	if cwd == nowhere {
		return ""
	}
	return "main"
}

func payload(t *testing.T, cwd, tool, command string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"session_id": "t",
		"cwd":        cwd,
		"tool_name":  tool,
		"tool_input": map[string]string{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestDeny is the whole predicate through the shape the runtime sends: a body
// that declares a close into a base GitHub will not resolve it on is denied,
// and every neighbouring case — a quotation of the rule, a denial of a close,
// another command's text, a base that is the default — is not.
func TestDeny(t *testing.T) {
	dir := t.TempDir()
	bodyFile := write(t, filepath.Join(dir, "body.md"), "Closes #7\n\nproof follows\n")
	priorFile := write(t, filepath.Join(dir, "prior.md"), "Not a revert\nCloses #7\n")
	wrapFile := write(t, filepath.Join(dir, "wrap.md"), "This closes\n#7 for good.\n")
	wrapNeg := write(t, filepath.Join(dir, "wrapneg.md"), "The migration does not close\n#7 here.\n")
	plainFile := write(t, filepath.Join(dir, "plain.md"), "Refs #7\n\nWhy this is not a close.\n")
	gone := filepath.Join(dir, "gone.md")
	unwritten := filepath.Join(dir, "unwritten.md")

	cases := []struct {
		name    string
		cwd     string
		command string
		deny    bool
	}{
		// A body that declares a close, through every input path and spelling.
		{"inline --body", "", `gh pr create --base dev --title "x" --body "Closes #103

## Established
..."`, true},
		{"lowercase closes", "", `gh pr create --base dev --body "closes #7"`, true},
		{"fixes", "", `gh pr create --base=dev --body "Fixes #7 by rewriting the parser"`, true},
		{"resolved", "", `gh pr create -B dev --body "Resolved #7"`, true},
		{"pr edit", "", `gh pr edit 12 --base dev --body "Closes #7"`, true},
		{"short body flag", "", `gh pr create --base dev -b "Closes #7"`, true},
		{"glued short flag", "", `gh pr create --base dev -bCloses' '#7`, true},
		{"glued long flag", "", `gh pr create --base dev --body=Closes' '#7`, true},
		{"environment prefix", "", `GH_TOKEN=x gh pr create --base dev --body "Closes #7"`, true},
		{"heredoc body-file -", "", `gh pr create --base dev --body-file - <<EOF
Closes #7
EOF`, true},
		{"--body-file path", "", "gh pr create --base dev --body-file " + bodyFile, true},
		{"--body-file short flag", "", "gh pr create --base dev -F " + bodyFile, true},

		// GitHub's own grammar for the reference, which is wider than a bare
		// number and is what actually closes an issue.
		{"colon after the keyword", "", `gh pr create --base dev --body "Closes: #7"`, true},
		{"owner/repo reference", "", `gh pr create --base dev --body "Closes example-org/other#7"`, true},
		{"issue URL", "", `gh pr create --base dev --body "Closes https://github.com/example-org/other/issues/7"`, true},
		{"emphasised keyword", "", `gh pr create --base dev --body "**Closes** #7"`, true},

		// A keyword and its number on either side of a soft wrap is a close.
		// It reads the same on both input paths, which is the point: the body
		// is JSON-decoded before it is read, so a newline is a newline.
		{"wrapped keyword, --body-file", "", "gh pr create --base dev --body-file " + wrapFile, true},
		{"wrapped keyword, --body", "", `gh pr create --base dev --body "This closes
#7 for good."`, true},
		{"unwrapped control", "", `gh pr create --base dev --body "This closes #7 for good."`, true},

		// A negation excuses its own clause and no more.
		{"negated, then genuine", "", `gh pr create --base dev --body "Does not close #7, but this Closes #8"`, true},
		{"negated, genuine after", "", `gh pr create --base dev --body "Nothing here closes #7. Closes #8"`, true},
		{"negator, prior line", "", `gh pr create --base dev --body "Not a revert
Closes #7"`, true},
		{"negator, prior bullet", "", `gh pr create --base dev --body "- not blocked on review
- Closes #7"`, true},
		{"negator, prior sentence", "", `gh pr create --base dev --body "This is not a workaround.
Closes #7"`, true},
		{"prior line, --body-file", "", "gh pr create --base dev --body-file " + priorFile, true},
		{"negator two clauses on", "", `gh pr create --base dev --body "not that it matters; fixes #7"`, true},

		// A bare negator opens a noun phrase and governs the noun, so a
		// declaration whose sentence merely opens with one is still a close.
		{"no + noun phrase", "", `gh pr create --base dev --body "No behaviour change here closes #7"`, true},
		{"nothing else changed", "", `gh pr create --base dev --body "Nothing else changed and this closes #7"`, true},
		{"short noun gap", "", `gh pr create --base dev --body "No doubt this closes #7"`, true},
		{"emphasised noun gap", "", `gh pr create --base dev --body "**No** API change closes #7"`, true},
		{"two-word noun gap", "", `gh pr create --base dev --body "no tests yet closes #7"`, true},
		{"long gap after negator", "", `gh pr create --base dev --body "not the branch that anybody expected here closes #7"`, true},

		// A code span is removed and leaves one word behind, so the strip
		// cannot manufacture an adjacency the author did not write.
		{"code span in the gap", "", "gh pr create --base dev --body \"The regression is not in `pkg/sync` and this closes #7\"", true},
		{"short span in the gap", "", "gh pr create --base dev --body \"not a `revert` closes #7\"", true},

		// A negator has to be a word rather than a prefix or a syllable.
		{"negator as a prefix", "", `gh pr create --base dev --body "not-a-revert closes #7"`, true},
		{"negation inside a word", "", `gh pr create --base dev --body "The unnoticed regression closes #7"`, true},

		// A coordinating conjunction opens a new clause the way a full stop
		// does, and a negation does not reach past one.
		{"and after a negation", "", `gh pr create --base dev --body "This does not build and closes #7"`, true},
		{"but after a negation", "", `gh pr create --base dev --body "It doesn't compile but closes #7"`, true},
		{"or after a negation", "", `gh pr create --base dev --body "The refactor is not finished or closes #7"`, true},

		// The auxiliary arm is bounded too. A verb negation that drifts five
		// words from its keyword is read as a declaration, which is the safe
		// direction for a guard.
		{"verb negation past the bound", "", `gh pr create --base dev --body "It doesn't matter which base anybody picked closes #7"`, true},
		// A wrap between the negator and its keyword is a line break, and a
		// line break ends a clause.
		{"negation wrapped from its keyword", "", `gh pr create --base dev --body "This does not
close #7"`, true},

		// A body that denies closing an issue is the careful artifact the
		// doctrine asks for, not the mistake this guards.
		{"does not close, emphasised", "", `gh pr create --base dev --body "## Scope

Does **not** close #715 — its acceptance rule is ` + "`runtime-proof`" + ` and push delivery is dark.

Refs #715"`, false},
		{"does not close", "", `gh pr create --base dev --body "Does not close #715"`, false},
		{"nothing closes", "", `gh pr create --base dev --body "nothing closes #7 here"`, false},
		{"no merge closes", "", `gh pr create --base dev --body "no merge closes #7"`, false},
		{"will not close", "", `gh pr create --base dev --body "this will not close #7"`, false},
		{"contracted negator", "", `gh pr create --base dev --body "it doesn't close #7"`, false},
		{"cannot close", "", `gh pr create --base dev --body "a merge into dev cannot close #7"`, false},
		{"never fixes", "", `gh pr create --base dev --body "this never fixes #7"`, false},
		{"neither closes", "", `gh pr create --base dev --body "this neither closes #7 nor fixes #8"`, false},

		// An auxiliary in front of the negator says the verb is what is
		// denied, so the keyword is denied however many words stand between.
		{"auxiliary, two-word gap", "", `gh pr create --base dev --body "This does not in fact close #7"`, false},
		{"auxiliary, wider gap", "", `gh pr create --base dev --body "The merge does not by itself close #7"`, false},
		{"contraction, wider gap", "", `gh pr create --base dev --body "It doesn't on this base close #7"`, false},
		// The wrap falls between the keyword and its number, so the negator
		// still stands in the keyword's own clause.
		{"wrapped negation, --body-file", "", "gh pr create --base dev --body-file " + wrapNeg, false},

		// A quotation of this rule is not a use of it, at every delimiter
		// width Markdown offers.
		{"quoted in a code span", "", "gh pr create --base dev --body \"PR #83 merged with `Closes #75` in its body and #75 stayed open.\"", false},
		{"quoted in a doubled span", "", "gh pr create --base dev --body \"Opening a pull request whose Scope read ``Does **not** close #715 — its acceptance rule is `runtime-proof` and push delivery is dark``, with `Refs #715` at the bottom.\"", false},
		{"plain close in a doubled span", "", "gh pr create --base dev --body \"The denial named ``Closes #75 with `Refs #75` beneath it`` and that is the shape.\"", false},
		{"quoted in a fence", "", "gh pr create --base dev --body \"```\nCloses #75\n```\nthat is what not to write.\"", false},
		{"quoted in a tilde fence", "", "gh pr create --base dev --body \"~~~\nCloses #75\n~~~\nthat is what not to write.\"", false},
		{"quoted in a blockquote", "", "gh pr create --base dev --body \"The denial named:\n\n> Closes #75\n\nand that is what not to write.\"", false},
		{"unmatched backtick", "", "gh pr create --base dev --body \"a stray ` and then Closes #7\"", true},

		// Nothing to resolve.
		{"refs", "", `gh pr create --base dev --body "Refs #103

## Established
..."`, false},
		{"no issue number", "", `gh pr create --base dev --body "this closes the gap"`, false},
		{"not a keyword", "", `gh pr create --base dev --body "lands without closing #7"`, false},

		// The base decides, because GitHub does.
		{"no --base", "", `gh pr create --body "Closes #7"`, false},
		{"--base is the default", "", `gh pr create --base main --body "Closes #7"`, false},
		{"pr edit without --base", "", `gh pr edit 12 --body "Closes #7"`, false},
		{"unknown default branch", nowhere, `gh pr create --base dev --body "Closes #7"`, false},

		// Whose text it is. A command that opens no pull request has no pull
		// request body, however much of one it carries.
		{"gh pr list", "", `gh pr list --state all --json body | grep -i "Closes #"`, false},
		{"gh pr view", "", `gh pr view 89 --json body`, false},
		{"not a pr command", "", `git commit -m "Closes #7"`, false},
		{"echo of a pr command", "", `echo 'gh pr create --base dev --body "Closes #7"'`, false},
		{"commit chained with a create", "", `git add -A && git commit -m "Closes #7 in the fixtures" && git push -u origin HEAD && gh pr create --base dev --body-file ` + plainFile, false},
		{"heredoc writing a probe script", "", `cat > ` + filepath.Join(dir, "probe.sh") + ` <<'SH'
gh pr create --base dev --body "closes #7"
gh pr create --base dev --body "Does not close #7"
SH`, false},
		// The heredoc that writes the body has not run when this is decided,
		// so the body is read where the command line is about to put it.
		{"heredoc into a body-file", "", `cat > ` + unwritten + ` <<'EOF'
Closes #7
EOF
gh pr create --base dev --body-file ` + unwritten, true},
		{"missing --body-file", "", "gh pr create --base dev --body-file " + gone, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cwd := c.cwd
			if cwd == "" {
				cwd = dir
			}
			reason := Deny(payload(t, cwd, "Bash", c.command), stub)
			if (reason != "") != c.deny {
				t.Fatalf("deny=%v, want %v\ncommand: %s\nreason: %s", reason != "", c.deny, c.command, reason)
			}
			if !c.deny {
				return
			}
			for _, want := range []string{"Refs ", "GitHub resolves a closing keyword only on the default branch, main"} {
				if !strings.Contains(reason, want) {
					t.Errorf("denial does not say %q:\n%s", want, reason)
				}
			}
			if strings.Contains(reason, "\n\"") || strings.HasPrefix(reason, "\"\"") {
				t.Errorf("denial names no keyword:\n%s", reason)
			}
		})
	}
}

// TestOnlyABashPayloadIsRead: another tool's payload carrying the same words is
// not a pull request body.
func TestOnlyABashPayloadIsRead(t *testing.T) {
	for _, tool := range []string{"Write", "Edit", "Task"} {
		if r := Deny(payload(t, "/repo", tool, `gh pr create --base dev --body "Closes #7"`), stub); r != "" {
			t.Errorf("%s payload denied: %s", tool, r)
		}
	}
	if r := Deny([]byte("not json"), stub); r != "" {
		t.Errorf("unreadable payload denied: %s", r)
	}
	if r := Deny(nil, stub); r != "" {
		t.Errorf("empty payload denied: %s", r)
	}
}

// TestTheDenialNamesTheReferenceItFound, because the reason is what the session
// has to act on and "write Refs #NN instead" is useless without the number.
func TestTheDenialNamesTheReferenceItFound(t *testing.T) {
	for _, c := range []struct{ body, ref string }{
		{"Closes #103", "#103"},
		{"Fixes example-org/other#12", "example-org/other#12"},
		{"Resolves https://github.com/example-org/other/issues/9", "https://github.com/example-org/other/issues/9"},
	} {
		reason := Deny(payload(t, "/repo", "Bash", `gh pr create --base dev --body "`+c.body+`"`), stub)
		if !strings.Contains(reason, "Refs "+c.ref) {
			t.Errorf("body %q: denial does not offer %q:\n%s", c.body, "Refs "+c.ref, reason)
		}
	}
}

// A body reaches gh through a quoted heredoc captured by a substitution, and
// that is the form the guards must read. Where the substitution is lexed as
// bytes, the document's own punctuation closes the quote around it: --body
// takes a prefix, the rest of the document becomes other arguments, and a
// citation or a closing keyword past that point is in nothing the guards see.
func TestABodyArrivesWholeThroughASubstitutedHeredoc(t *testing.T) {
	const command = `gh pr create --base dev --title "t" --body "$(cat <<'BODY'
The route is "absent and disabled", which was an absence rather than a check.

` + "`internal/x/y.go:12`" + ` refuses it before admission (and the sibling does too).

Closes #7
BODY
)"`
	calls := Publishing(command, "")
	if len(calls) != 1 {
		t.Fatalf("Publishing returned %d calls, want 1", len(calls))
	}
	if len(calls[0].Bodies) != 1 {
		t.Fatalf("call carries %d bodies, want 1", len(calls[0].Bodies))
	}
	body := calls[0].Bodies[0]
	for _, want := range []string{`"absent and disabled"`, "internal/x/y.go:12", "Closes #7"} {
		if !strings.Contains(body, want) {
			t.Errorf("the body does not carry %q; it is %q", want, body)
		}
	}
	if calls[0].Base != "dev" {
		t.Errorf("base = %q, want dev — the substitution swallowed the flags after it", calls[0].Base)
	}
	if c, ok := Declared(body); !ok || c.Ref != "#7" {
		t.Errorf("Declared(body) = %+v, %v; want the close on #7 seen", c, ok)
	}
}

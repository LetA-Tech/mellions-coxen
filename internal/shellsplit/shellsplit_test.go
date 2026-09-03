// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "strings"

import "testing"

// bodyThroughSubstitution is how a document reaches a command: written in a
// quoted heredoc, captured by a substitution, handed over as one argument.
// Read as bytes rather than as a unit, the document's own punctuation ends the
// quote the substitution sits in, and everything after it is lexed as more
// command line — so the argument holds a prefix of the document and the rest
// becomes words, and commands, nobody asked for.
const bodyThroughSubstitution = `gh pr comment 9 --body "$(cat <<'BODY'
The route is "absent and disabled", which was an absence and not a check.
See internal/x/y.go:12 (and the sibling) for what refuses it.
BODY
)"`

func TestASubstitutedHeredocIsOneArgument(t *testing.T) {
	cmds := Split(bodyThroughSubstitution)
	if len(cmds) != 1 {
		t.Fatalf("split into %d commands, want 1: %+v", len(cmds), cmds)
	}
	words := cmds[0].Words
	if len(words) != 6 {
		t.Fatalf("words = %q, want gh pr comment 9 --body and one body", words)
	}
	body := words[5]
	for _, want := range []string{`"absent and disabled"`, "internal/x/y.go:12", "(and the sibling)"} {
		if !strings.Contains(body, want) {
			t.Errorf("the body argument does not carry %q; it is %q", want, body)
		}
	}
}

// A word a caller cannot resolve must stay unresolvable rather than read as
// absent: "" is indistinguishable from an argument nobody wrote, and a guard
// that resolves paths would take an empty target for the directory it is in.
func TestASubstitutionWithNoHeredocKeepsItsSource(t *testing.T) {
	cmds := Split(`cd "$(mktemp -d)" && git checkout dev`)
	if len(cmds) != 2 {
		t.Fatalf("split into %d commands, want 2: %+v", len(cmds), cmds)
	}
	if got := cmds[0].Words; len(got) != 2 || got[1] != "$(mktemp -d)" {
		t.Errorf("cd words = %q, want the substitution's own source as the target", got)
	}
}

// Arithmetic names no command, and the parenthesis that closes it is not the
// one that closes a substitution.
func TestArithmeticDoesNotSwallowTheRestOfTheLine(t *testing.T) {
	cmds := Split(`echo $((1 + 2)) && gh pr comment 9 --body hi`)
	if len(cmds) != 2 {
		t.Fatalf("split into %d commands, want 2: %+v", len(cmds), cmds)
	}
	if got := cmds[1].Words; len(got) != 6 || got[0] != "gh" || got[5] != "hi" {
		t.Errorf("second command = %q, want the gh call whole", got)
	}
}

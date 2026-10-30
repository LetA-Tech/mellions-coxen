// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "strings"

import (
	"reflect"
	"testing"
)

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

// An input redirection's target stays a word, marked as not an argument, and
// the marks are indexes into Words wherever the redirection sits.
func TestInputRedirectionTargets(t *testing.T) {
	for _, tt := range []struct {
		cmd   string
		words []string
		in    []int
	}{
		{`mellions < .env report write -file -`, []string{"mellions", ".env", "report", "write", "-file", "-"}, []int{1}},
		{`mellions <.env report`, []string{"mellions", ".env", "report"}, []int{1}},
		{`< .env cat`, []string{".env", "cat"}, []int{0}},
		{`cat x < a < b`, []string{"cat", "x", "a", "b"}, []int{2, 3}},
		{`cat <<< .env`, []string{"cat", ".env"}, nil},
		{`cat <&3 x; ls y`, []string{"cat", "x"}, nil},
		{`mellions < <(cat .env) report`, []string{"mellions", "<(cat .env)", "report"}, []int{1}},
		{`diff <(ls a) b`, []string{"diff", "<(ls a)", "b"}, nil},
		{`mellions <>x report .env`, []string{"mellions", "x", "report", ".env"}, []int{1}},
		{`cat <>.env`, []string{"cat", ".env"}, []int{1}},
	} {
		c := Split(tt.cmd)[0]
		if !reflect.DeepEqual(c.Words, tt.words) || !reflect.DeepEqual(c.In, tt.in) {
			t.Errorf("Split(%q) = words %q in %v, want %q in %v", tt.cmd, c.Words, c.In, tt.words, tt.in)
		}
	}
	if cs := Split(`cat <&3 x; ls y`); len(cs) != 2 || cs[1].In != nil {
		t.Errorf("a descriptor duplication leaked a mark into the next command: %+v", cs)
	}
}

// A substitution's commands are recorded on the command that carries it,
// whichever word it sits in, with the kind that says whether they are captured.
func TestSubstitutionsAreRecordedWhereverTheySit(t *testing.T) {
	for _, tt := range []struct {
		name, cmd string
		kind      byte
		inner     string
	}{
		{"an operand", `ls $(cat a)`, '$', "cat"},
		{"backquotes", "ls `cat a`", '`', "cat"},
		{"a redirection target", `echo hi > >(cat a)`, '>', "cat"},
		{"an input target", `cat < <(cat a)`, '<', "cat"},
		{"mid-word", `ls x>(cat a)`, '>', "cat"},
		{"inside double quotes", `echo "x$(cat a)"`, '$', "cat"},
		{"backquotes inside double quotes", "echo \"x`cat a`\"", '`', "cat"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmds := Split(tt.cmd)
			if len(cmds) != 1 || len(cmds[0].Subs) != 1 {
				t.Fatalf("Split(%q) = %d commands, subs %+v; want one command carrying one substitution", tt.cmd, len(cmds), cmds)
			}
			s := cmds[0].Subs[0]
			if s.Kind != tt.kind || len(s.Cmds) != 1 || s.Cmds[0].Words[0] != tt.inner {
				t.Fatalf("Split(%q) substitution = kind %q cmds %+v; want kind %q running %q", tt.cmd, s.Kind, s.Cmds, tt.kind, tt.inner)
			}
		})
	}
	for _, tt := range []struct {
		cmd string
		dup bool
	}{
		{`cat a >&2`, true}, {`cat a 1>&2`, true}, {`cat a 2>&1`, false}, {`cat a >&-`, false}, {`cat a > f`, false},
	} {
		if got := Split(tt.cmd)[0].StdoutDup; got != tt.dup {
			t.Errorf("Split(%q).StdoutDup = %v, want %v", tt.cmd, got, tt.dup)
		}
	}
}

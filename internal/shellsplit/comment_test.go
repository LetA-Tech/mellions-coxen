// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"reflect"
	"testing"
)

// Each want is what bash 5.3 runs: a comment's quotes, parentheses and
// operators are no syntax, and the newline ending it still ends the command.
func TestACommentIsNoSyntax(t *testing.T) {
	cases := []struct {
		command string
		want    [][]string
	}{
		// An apostrophe in a comment opens no quote that swallows the next line.
		{"echo ok # it's fine\ngh pr merge 7 --squash", [][]string{{"echo", "ok"}, {"gh", "pr", "merge", "7", "--squash"}}},
		{"gh pr merge # merge it", [][]string{{"gh", "pr", "merge"}}},
		{"true # ; pkill -f gate", [][]string{{"true"}}},
		{"true # )\ngh pr merge 5", [][]string{{"true"}, {"gh", "pr", "merge", "5"}}},
		{"echo a;#b\necho c", [][]string{{"echo", "a"}, {"echo", "c"}}},
		{"true &&# x\necho c", [][]string{{"true"}, {"echo", "c"}}},
		// A backslash ending a comment continues nothing.
		{"echo a # b \\\necho c", [][]string{{"echo", "a"}, {"echo", "c"}}},
		// A # that does not start an unquoted word is text.
		{"echo a#b $# x# \\#y '#' \"#\" ''#z", [][]string{{"echo", "a#b", "$#", "x#", "#y", "#", "#", "#z"}}},
		// A # inside backticks or a (( command is kept as text, so what follows
		// it on the line is still read.
		{"echo ` #x`; git reset --hard", [][]string{{"echo", "`", "#x`"}, {"git", "reset", "--hard"}}},
		{"(( 1 # 1 )); git reset --hard", [][]string{{"((", "1", "#", "1", "))"}, {"git", "reset", "--hard"}}},
		{"((x)) # ; git reset --hard", [][]string{{"((x))"}}},
	}
	for _, c := range cases {
		var got [][]string
		for _, cmd := range Split(c.command) {
			got = append(got, cmd.Args())
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestACommentInASubstitutionHidesItsParenthesis(t *testing.T) {
	// bash runs `echo inner` inside the substitution and prints "x inner".
	cmds := Split("echo $(echo x # )\necho inner\n) done")
	if len(cmds) != 1 {
		t.Fatalf("split into %d commands, want 1: %+v", len(cmds), cmds)
	}
	if got := cmds[0].Words; len(got) != 3 || got[0] != "echo" || got[2] != "done" {
		t.Errorf("words = %q, want echo, the substitution, done", got)
	}
}

func TestACommentOnAHeredocLineLeavesTheBody(t *testing.T) {
	cmds := Split("cat <<EOF # it's here\nbody # stays\nEOF\necho after")
	if len(cmds) != 2 {
		t.Fatalf("split into %d commands, want 2: %+v", len(cmds), cmds)
	}
	if got := cmds[0].Heredocs; len(got) != 1 || got[0] != "body # stays\n" {
		t.Errorf("heredocs = %q, want the body with its # intact", got)
	}
	if got := cmds[1].Args(); !reflect.DeepEqual(got, []string{"echo", "after"}) {
		t.Errorf("second command = %q, want echo after", got)
	}
}

// Where bash reads one word across a blank this lexer splits at, a # after
// that blank starts no comment, and the command after it is one bash runs.
func TestAHashInsideAWordBashReadsWholeIsText(t *testing.T) {
	for _, prefix := range []string{
		"echo ${x:- #foo} ; ",
		"echo ${x// #/y} ; ",
		"echo ${x:-a\n#b} ; ",
		"echo ${x:-${y:- #}} ; ",
		"[[ \"#a\" =~ x|#a ]] ; ",
		"[[ x =~ ( #a) ]] ; ",
		"(( 1 == $((1)) #)); ",
		"true\r# ; ",
	} {
		command := prefix + "gh pr merge 5"
		found := false
		for _, cmd := range Split(command) {
			if reflect.DeepEqual(cmd.Args(), []string{"gh", "pr", "merge", "5"}) {
				found = true
			}
		}
		if !found {
			t.Errorf("Split(%q) has no gh pr merge 5, which bash runs", command)
		}
	}
}

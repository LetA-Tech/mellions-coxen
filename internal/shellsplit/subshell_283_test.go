// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"reflect"
	"testing"
)

func words(command string) [][]string {
	var got [][]string
	for _, c := range Split(command) {
		got = append(got, c.Words)
	}
	return got
}

// An unquoted ( opening a command and the ) closing that subshell are bash
// metacharacters: they end the word they touch, so a subshell written without
// spaces lexes as the commands inside it.
func TestASubshellsParenthesesEndTheWordsTheyTouch(t *testing.T) {
	for _, c := range []struct {
		command string
		want    [][]string
	}{
		{"(echo a)", [][]string{{"echo", "a"}}},
		{"( echo a )", [][]string{{"echo", "a"}}},
		{"(gh pr comment 5 --body 'x')", [][]string{{"gh", "pr", "comment", "5", "--body", "x"}}},
		{"(gh pr merge 12)", [][]string{{"gh", "pr", "merge", "12"}}},
		{"(cd a && gh pr merge 12) && echo done", [][]string{{"cd", "a"}, {"gh", "pr", "merge", "12"}, {"echo", "done"}}},
		{"( (pkill -f foo) )", [][]string{{"pkill", "-f", "foo"}}},
		{"echo a; (echo b)", [][]string{{"echo", "a"}, {"echo", "b"}}},
		{"if (gh pr merge 12); then echo ok; fi", [][]string{{"if"}, {"gh", "pr", "merge", "12"}, {"then", "echo", "ok"}, {"fi"}}},
		{"! (pkill -f foo)", [][]string{{"!"}, {"pkill", "-f", "foo"}}},
		{"x=$( (echo a) )", [][]string{{"x=$( (echo a) )"}}},
	} {
		if got := words(c.command); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

// A heredoc body fed to a command in a subshell is read whole, and the ) after
// it closes the subshell rather than joining the body.
func TestASubshellClosesAfterItsHeredoc(t *testing.T) {
	cmds := Split("(gh pr comment 5 --body-file - <<'EOF'\n```\nx\n```\nEOF\n)")
	if len(cmds) != 1 || cmds[0].Words[0] != "gh" || len(cmds[0].Heredocs) != 1 || cmds[0].Heredocs[0] != "```\nx\n```\n" {
		t.Fatalf("got %d commands: %+v", len(cmds), cmds)
	}
}

// Parentheses bash does not read as a subshell lex as they did: quoted, inside
// a word, after a word (a syntax error to bash), closing nothing, opening a
// process substitution, and opening an arithmetic command.
func TestParenthesesThatOpenNoSubshellLexAsBefore(t *testing.T) {
	for _, c := range []struct {
		command string
		want    [][]string
	}{
		{"echo '(a)'", [][]string{{"echo", "(a)"}}},
		{`echo "(a)"`, [][]string{{"echo", "(a)"}}},
		{`echo \(a\)`, [][]string{{"echo", "(a)"}}},
		{"echo a(b", [][]string{{"echo", "a(b"}}},
		{"echo (a)", [][]string{{"echo", "(a)"}}},
		{"echo a)", [][]string{{"echo", "a)"}}},
		{"case $x in a) echo y;; esac", [][]string{{"case", "$x", "in", "a)", "echo", "y"}, {"esac"}}},
		{"diff <(cat a) b", [][]string{{"diff", "(cat", "a)", "b"}}},
		{"<(cat a) wc", [][]string{{"(cat", "a)", "wc"}}},
		{"((pkill -f foo))", [][]string{{"((pkill", "-f", "foo))"}}},
		{"echo $(echo a)", [][]string{{"echo", "$(echo a)"}}},
	} {
		if got := words(c.command); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

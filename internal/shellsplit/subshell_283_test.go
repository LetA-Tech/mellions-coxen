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
		{"{(gh pr merge 12);}", [][]string{{"{"}, {"gh", "pr", "merge", "12"}, {"}"}}},
		{"!(pkill -f foo)", [][]string{{"!"}, {"pkill", "-f", "foo"}}},
		{"if(gh pr merge 12);then :;fi", [][]string{{"if"}, {"gh", "pr", "merge", "12"}, {"then", ":"}, {"fi"}}},
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
		{"> (x)", nil},
		{"'if'(x)", [][]string{{"if(x)"}}},
		{"echo $(echo a)", [][]string{{"echo", "$(echo a)"}}},
	} {
		if got := words(c.command); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

// Every reserved word that runs the command after it may open a subshell.
func TestASubshellOpensAfterEveryReservedWordThatRunsACommand(t *testing.T) {
	for _, kw := range []string{"!", "{", "if", "then", "else", "elif", "do", "while", "until", "time"} {
		found := false
		for _, c := range Split(kw + " (gh pr merge 12)") {
			if reflect.DeepEqual(c.Words, []string{"gh", "pr", "merge", "12"}) {
				found = true
			}
		}
		if !found {
			t.Errorf("Split(%q): no command gh pr merge 12 in %q", kw+" (gh pr merge 12)", words(kw+" (gh pr merge 12)"))
		}
	}
}

// A "(" inside a substitution never pairs with the ")" that closes it, so a
// comment or backticks the lexer does not read cannot run the substitution on
// and hide the commands after it.
func TestASubstitutionNeverRunsPastItsEnd(t *testing.T) {
	for _, command := range []string{
		"x=$(true #;(\n) ; gh pr merge 5",
		"x=$(: # &&(\n); gh pr merge 5",
		"x=$(echo `:;(`) ; gh pr merge 5",
		"x=$( (echo a) ); gh pr merge 5",
	} {
		got := words(command)
		if len(got) == 0 || !reflect.DeepEqual(got[len(got)-1], []string{"gh", "pr", "merge", "5"}) {
			t.Errorf("Split(%q) = %q, want gh pr merge 5 last", command, got)
		}
	}
}

// A redirect after a subshell's ")" is the subshell's: it lands on the last
// command inside it, and that command is returned once.
func TestARedirectAfterASubshellIsItsLastCommands(t *testing.T) {
	cmds := Split("(printf x)>b.md; gh pr create --body-file b.md")
	if len(cmds) != 2 || cmds[0].Words[0] != "printf" || cmds[0].Out != "b.md" {
		t.Fatalf("got %q, first Out %q", words("(printf x)>b.md; gh pr create --body-file b.md"), outOf(cmds))
	}
	cmds = Split("(cat <<'EOF')>b.md\nbody\nEOF\ngh pr create --body-file b.md")
	if len(cmds) != 2 || cmds[0].Out != "b.md" || len(cmds[0].Heredocs) != 1 || cmds[0].Heredocs[0] != "body\n" {
		t.Fatalf("heredoc form: got %d commands, first Out %q", len(cmds), outOf(cmds))
	}
}

func outOf(cmds []*Command) string {
	if len(cmds) == 0 {
		return ""
	}
	return cmds[0].Out
}

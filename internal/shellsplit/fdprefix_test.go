// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"reflect"
	"testing"
)

// Every bash redirection operator, with and without a descriptor prefix and
// with its target glued and spaced, between a command and the words it runs.
// The command must stay whole and every word after the redirection must stay
// the command's own; a sweep over one shape is what catches the spelling a
// row-by-row list leaves out.
func TestEveryRedirectionKeepsTheCommandsWords(t *testing.T) {
	type op struct {
		op, target string
		prefixes   []string // descriptor prefixes bash accepts on this operator
		outIsFile  bool     // the target is the file stdout goes to
	}
	ops := []op{
		{"<", "/dev/null", []string{"0", "3", "10", "{fd}"}, false},
		{">", "/dev/null", []string{"1", "2", "10", "{fd}"}, true},
		{">>", "/tmp/log", []string{"1", "2", "{fd}"}, true},
		{">|", "/tmp/log", []string{"1", "2", "{fd}"}, true},
		{"<>", "/dev/null", []string{"0", "3", "{fd}"}, false},
		{"<<<", "text", []string{"0", "3", "{fd}"}, false},
		{"<&", "0", []string{"0", "3", "{fd}"}, false},
		{"<&", "3-", []string{"0", "{fd}"}, false},
		{"<&", "-", []string{"0", "3", "{fd}"}, false},
		{">&", "2", []string{"1", "2", "{fd}"}, false},
		{">&", "-", []string{"1", "2", "{fd}"}, false},
		{">&", "/dev/null", nil, true},
		{"&>", "/dev/null", nil, true},
		{"&>>", "/tmp/log", nil, true},
	}
	want := []string{"git", "reset", "--hard"}
	for _, o := range ops {
		for _, prefix := range append([]string{""}, o.prefixes...) {
			for _, gap := range []string{"", " "} {
				cmd := "git " + prefix + o.op + gap + o.target + " reset --hard"
				cmds := Split(cmd)
				if len(cmds) != 1 {
					t.Errorf("Split(%q) gave %d commands, want 1", cmd, len(cmds))
					continue
				}
				got := cmds[0].Words
				wantWords := want
				if o.op == "<" || o.op == "<>" || o.op == "<<<" {
					wantWords = []string{"git", o.target, "reset", "--hard"}
				}
				if !reflect.DeepEqual(got, wantWords) {
					t.Errorf("Split(%q).Words = %q, want %q", cmd, got, wantWords)
				}
				if args := cmds[0].Args(); !reflect.DeepEqual(args, want) {
					t.Errorf("Split(%q).Args() = %q, want %q", cmd, args, want)
				}
				wantOut := ""
				if o.outIsFile {
					wantOut = o.target
				}
				if cmds[0].Out != wantOut {
					t.Errorf("Split(%q).Out = %q, want %q", cmd, cmds[0].Out, wantOut)
				}
			}
		}
	}
}

// Bash reads a descriptor prefix only from literal digits or a {name} glued to
// the operator. Anything else in that place is an ordinary word the command
// receives.
func TestOnlyALiteralGluedNumberIsADescriptor(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want []string
	}{
		{`echo "2">f x`, []string{"echo", "2", "x"}},
		{`echo '2'>f x`, []string{"echo", "2", "x"}},
		{`echo \2>f x`, []string{"echo", "2", "x"}},
		{`echo a2>f x`, []string{"echo", "a2", "x"}},
		{`echo 2 >f x`, []string{"echo", "2", "x"}},
		{`echo {1x}>f x`, []string{"echo", "{1x}", "x"}},
		{`echo {x}a>f x`, []string{"echo", "{x}a", "x"}},
		{`echo {}>f x`, []string{"echo", "{}", "x"}},
		{`echo 2&>f x`, []string{"echo", "2", "x"}},
		{`echo 2&>>f x`, []string{"echo", "2", "x"}},
		{`echo 0<(true) x`, []string{"echo", "0", "(true)", "x"}},
		{`echo >&2x y`, []string{"echo", "y"}},
		{`echo >&2>x y`, []string{"echo", "y"}},
		{`cat 0<<EOF x` + "\nbody\nEOF\n", []string{"cat", "x"}},
	} {
		cmds := Split(tc.cmd)
		if len(cmds) != 1 || !reflect.DeepEqual(cmds[0].Words, tc.want) {
			var got [][]string
			for _, c := range cmds {
				got = append(got, c.Words)
			}
			t.Errorf("Split(%q) = %q, want one command %q", tc.cmd, got, tc.want)
		}
	}
	if got := Split(`echo >&2x y`)[0].Out; got != "2x" {
		t.Errorf("Split(echo >&2x y).Out = %q, want 2x: >& of a word that is no descriptor is a file", got)
	}
	if got := Split(`echo >&2>x y`)[0].Out; got != "x" {
		t.Errorf("Split(echo >&2>x y).Out = %q, want x", got)
	}
	if got := Split("cat 0<<EOF x\nbody\nEOF\n")[0].Heredocs; !reflect.DeepEqual(got, []string{"body\n"}) {
		t.Errorf("a prefixed heredoc's body = %q, want [body]", got)
	}
}

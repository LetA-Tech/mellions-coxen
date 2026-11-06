// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

// TestProgramIsFoundBehindEveryKeywordThatRunsIt holds Program to bash: the
// command after `if`, `elif`, `while` and `until` runs as a condition, after
// `do`, `then`, `else`, `{`, `(` and `!` as a body, and after a wrapper as the
// wrapper's child. Every guard that reads Program sees the program, not the
// keyword.
func TestProgramIsFoundBehindEveryKeywordThatRunsIt(t *testing.T) {
	for _, c := range []struct{ command, want string }{
		{"rm -rf /tmp/x", "rm"},
		{"if rm -rf /tmp/x; then :; fi", "rm"},
		{"if false; then :; elif rm -rf /tmp/x; then :; fi", "rm"},
		{"while rm -rf /tmp/x; do break; done", "rm"},
		{"until rm -rf /tmp/x; do break; done", "rm"},
		{"for f in a; do rm -rf /tmp/x; done", "rm"},
		{"( rm -rf /tmp/x )", "rm"},
		{"(rm -rf /tmp/x)", "rm"},
		{"{ rm -rf /tmp/x; }", "rm"},
		{"! rm -rf /tmp/x", "rm"},
		{"timeout -s KILL 60 rm -rf /tmp/x", "rm"},
		{"env A=b nice -n 5 rm -rf /tmp/x", "rm"},
	} {
		t.Run(c.command, func(t *testing.T) {
			var got []string
			for _, cmd := range Split(c.command) {
				if w := Program(cmd.Words); len(w) > 0 {
					got = append(got, w[0])
				}
			}
			for _, g := range got {
				if g == c.want {
					return
				}
			}
			t.Fatalf("programs found in %q = %q, want one to be %q", c.command, got, c.want)
		})
	}
}

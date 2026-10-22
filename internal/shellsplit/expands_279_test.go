// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

// With quotes removed, `"$X"` and `'$X'` lex to the same word, but bash
// publishes a variable's value for one and the two characters for the other.
// Expands is what tells a caller which word is not the text the command gets.
func TestExpandsMarksTheWordsBashExpands(t *testing.T) {
	for _, tc := range []struct {
		name, arg string
		want      bool
	}{
		{"double-quoted substitution", `"$(cat f)"`, true},
		{"unquoted substitution", `$(cat f)`, true},
		{"single-quoted substitution", `'$(cat f)'`, false},
		{"double-quoted variable", `"$X"`, true},
		{"unquoted variable", `$X`, true},
		{"single-quoted variable", `'$X'`, false},
		{"braced variable", `"${X}"`, true},
		{"positional parameter in double quotes", `"costs $5"`, true},
		{"positional parameter in single quotes", `'costs $5'`, false},
		{"escaped dollar in double quotes", `"costs \$5"`, false},
		{"escaped dollar unquoted", `costs\$5`, false},
		{"special parameter", `"pid $$"`, true},
		{"dollar before a space", `"costs $ 5"`, false},
		{"dollar at the end of the quotes", `"costs 5$"`, false},
		{"backtick in double quotes", "\"run `date`\"", true},
		{"escaped backtick in double quotes", "\"a \\`code\\` span\"", false},
		{"backtick in single quotes", "'a `code` span'", false},
		{"arithmetic", `"$((1+2))"`, true},
		{"substituted quoted heredoc", "\"$(cat <<'EOF'\nbody $X\nEOF\n)\"", false},
		{"plain text", `"plain text"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmds := Split("gh pr create --body " + tc.arg + " --base dev")
			if len(cmds) != 1 {
				t.Fatalf("got %d commands", len(cmds))
			}
			c := cmds[0]
			if len(c.Expands) != len(c.Words) {
				t.Fatalf("Expands has %d entries for %d words", len(c.Expands), len(c.Words))
			}
			body := -1
			for i, w := range c.Words {
				if w == "--body" {
					body = i + 1
				}
			}
			if body < 0 || body >= len(c.Words) {
				t.Fatalf("no --body value in %q", c.Words)
			}
			if c.Expands[body] != tc.want {
				t.Fatalf("Expands for %q = %v, want %v (words %q)", c.Words[body], c.Expands[body], tc.want, c.Words)
			}
			for i, w := range c.Words {
				if i != body && c.Expands[i] {
					t.Errorf("word %q marked as expanding", w)
				}
			}
		})
	}
}

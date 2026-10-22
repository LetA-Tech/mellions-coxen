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
		{"substituted quoted heredoc, cat -", "\"$(cat - <<'EOF'\nbody\nEOF\n)\"", false},
		{"quoted heredoc, then another command", "\"$(cat <<'EOF'\nIntro\nEOF\ncat f)\"", true},
		{"quoted heredoc, cat also reads a file", "\"$(cat f - <<'EOF'\nIntro\nEOF\n)\"", true},
		{"quoted heredoc through another command", "\"$(sed s/a/b/ <<'EOF'\nIntro\nEOF\n)\"", true},
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

// bash expands the body of a heredoc whose delimiter is unquoted, so text read
// off the line there is not what the command receives.
func TestHeredocExpandsMarksAnUnquotedDelimitersExpansions(t *testing.T) {
	for _, tc := range []struct {
		name, delim, body string
		want              bool
	}{
		{"unquoted, substitution", "EOF", "a $(cat f) b", true},
		{"unquoted, variable", "EOF", "costs $5", true},
		{"unquoted, backtick", "EOF", "run `date`", true},
		{"unquoted, escaped dollar", "EOF", `costs \$5`, false},
		{"unquoted, plain", "EOF", "plain text", false},
		{"unquoted, lone dollar", "EOF", "costs $ 5", false},
		{"single-quoted", "'EOF'", "a $(cat f) b", false},
		{"double-quoted", `"EOF"`, "a $(cat f) b", false},
		{"escaped", `\EOF`, "a $(cat f) b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmds := Split("gh pr create --body-file - <<" + tc.delim + "\n" + tc.body + "\nEOF")
			if len(cmds) != 1 || len(cmds[0].Heredocs) != 1 || len(cmds[0].HeredocExpands) != 1 {
				t.Fatalf("want one command with one heredoc, got %+v", cmds)
			}
			if got := cmds[0].HeredocExpands[0]; got != tc.want {
				t.Fatalf("HeredocExpands = %v, want %v", got, tc.want)
			}
			sub := Split("gh pr create --body \"$(cat <<" + tc.delim + "\n" + tc.body + "\nEOF\n)\"")
			if len(sub) != 1 || len(sub[0].Expands) < 5 {
				t.Fatalf("substituted form: got %+v", sub)
			}
			if got := sub[0].Expands[4]; got != tc.want {
				t.Fatalf("substituted form: Expands = %v, want %v", got, tc.want)
			}
		})
	}
}

// A line continuation before `<<EOF` is no quote: the delimiter stays unquoted
// and bash still expands the body.
func TestALineContinuationDoesNotQuoteTheHeredocDelimiter(t *testing.T) {
	cmds := Split("gh pr create --body-file - \\\n  <<EOF\n$(cat f)\nEOF")
	if len(cmds) != 1 || len(cmds[0].HeredocExpands) != 1 {
		t.Fatalf("want one command with one heredoc, got %+v", cmds)
	}
	if !cmds[0].HeredocExpands[0] {
		t.Fatal("HeredocExpands = false after a line continuation, want true")
	}
}

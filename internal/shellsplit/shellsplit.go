// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package shellsplit lexes a Bash command line into the simple commands it is
// made of, so a hook can decide what a tool call actually runs.
//
// Two guards need the same reading of one command line — whether a pull-request
// body declares a close GitHub will not resolve, and whether a git command
// mutates a working tree that is not the session's — and a search over the raw
// string answers neither: it cannot see a command boundary, a quote, a heredoc
// or a redirect. Both parse, and they parse the same way because it is the same
// shell.
package shellsplit

import "strings"

// Command is one command in a compound: its words with quoting removed, the
// bodies of the heredocs it declares, and the file it redirects stdout to.
//
// Expands[i] reports whether Words[i] holds an expansion bash performs and this
// lexer does not — a `$(…)` it could not read off the line, a backtick, or a
// parameter outside single quotes — so the word is its source text, not the
// value the command receives. `'$5'` is literal and `"$5"` is not, and with the
// quotes removed only this flag tells them apart.
//
// HeredocExpands[i] is the same for Heredocs[i]: bash expands an unquoted
// delimiter's body, so `$(cat f)` inside `<<EOF` publishes f, not those bytes.
type Command struct {
	Words          []string
	Expands        []bool
	Heredocs       []string
	HeredocExpands []bool
	Out            string
}

// pending is a heredoc whose delimiter has been read and whose body has not:
// the body starts on the line after the one declaring it.
type pending struct {
	delim  string
	strip  bool
	expand bool
	owner  *Command
}

// Split lexes a Bash command line into its simple commands.
//
// It is a lexer and not a shell: it resolves quoting, splits at the operators
// that separate one command from the next, and reads heredoc bodies as the
// data they are rather than as more command line. Nothing is expanded, so a
// body reaching gh through a variable is not seen — what is seen is what the
// command line itself carries.
func Split(command string) []*Command {
	cmds, _ := lex(command, 0, false)
	return cmds
}

// lex reads simple commands from i and returns the index it stopped at.
// stopAtParen ends the read at an unquoted ")", which is how a command
// substitution closes; it is what makes the lexer re-entrant.
//
// Subshells are read only outside a substitution. Inside one the lexer cannot
// pair every "(" with bash — it reads no comments or backticks — and a ")" it
// paired wrongly would run the substitution past its end and hide the rest of
// the line; the commands inside a substitution are not returned anyway.
func lex(command string, i int, stopAtParen bool) ([]*Command, int) {
	var out []*Command
	// opens holds len(out) at each open subshell. reopened marks cur as the
	// last command of a subshell just closed: a redirect or heredoc after the
	// ")" is that command's, a word starts a new one, and it is appended once.
	var opens []int
	reopened := false
	cur := &Command{}
	var w strings.Builder
	hasWord := false
	expands := false
	quoted := false

	var queue []pending
	redirOut := false
	heredocNext := 0 // 0 none, 1 <<, 2 <<-

	endWord := func() {
		if !hasWord {
			return
		}
		s := w.String()
		w.Reset()
		hasWord = false
		exp := expands
		expands = false
		q := quoted
		quoted = false
		switch {
		case heredocNext != 0:
			queue = append(queue, pending{delim: s, strip: heredocNext == 2, expand: !q, owner: cur})
			heredocNext = 0
		case redirOut:
			cur.Out = s
			redirOut = false
		default:
			if reopened {
				cur, reopened = &Command{}, false
			}
			cur.Words = append(cur.Words, s)
			cur.Expands = append(cur.Expands, exp)
		}
	}
	endCmd := func() {
		endWord()
		if !reopened && (len(cur.Words) > 0 || len(cur.Heredocs) > 0) {
			out = append(out, cur)
		}
		reopened = false
		cur = &Command{}
	}

	for i < len(command) {
		c := command[i]
		switch {
		case c == '$' && i+1 < len(command) && command[i+1] == '(':
			var text string
			var read bool
			text, read, i = substitution(command, i)
			w.WriteString(text)
			hasWord = true
			expands = expands || !read

		case c == '(' && !stopAtParen &&
			opensSubshell(command, i, hasWord && (quoted || !leadsCommand[w.String()]), cur.Words):
			endCmd()
			opens = append(opens, len(out))
			i++

		case c == ')' && len(opens) > 0:
			endCmd()
			start := opens[len(opens)-1]
			opens = opens[:len(opens)-1]
			if len(out) > start {
				cur, reopened = out[len(out)-1], true
			}
			i++

		case stopAtParen && c == ')':
			endCmd()
			return out, i + 1

		case c == '\\' && i+1 < len(command):
			// A line continuation quotes nothing: it is not part of a word.
			if command[i+1] != '\n' {
				quoted = true
				w.WriteByte(command[i+1])
				hasWord = true
			}
			i += 2

		case c == '\'':
			quoted = true
			j := strings.IndexByte(command[i+1:], '\'')
			hasWord = true
			if j < 0 {
				w.WriteString(command[i+1:])
				i = len(command)
				break
			}
			w.WriteString(command[i+1 : i+1+j])
			i += j + 2

		case c == '"':
			quoted = true
			i++
			for i < len(command) && command[i] != '"' {
				if command[i] == '\\' && i+1 < len(command) {
					switch n := command[i+1]; n {
					case '"', '\\', '$', '`':
						w.WriteByte(n)
						i += 2
						continue
					case '\n':
						i += 2
						continue
					}
				}
				// A substitution inside the quotes is still a substitution.
				// Reading it as ordinary bytes is what let a body's own
				// punctuation close the quote and scatter the rest of the
				// command across words the caller never sees.
				if command[i] == '$' && i+1 < len(command) && command[i+1] == '(' {
					var text string
					var read bool
					text, read, i = substitution(command, i)
					w.WriteString(text)
					expands = expands || !read
					continue
				}
				if expansion(command, i) {
					expands = true
				}
				w.WriteByte(command[i])
				i++
			}
			hasWord = true
			if i < len(command) {
				i++
			}

		case c == '\n':
			endWord()
			i++
			if len(queue) > 0 {
				i = readHeredocs(command, i, queue)
				queue = nil
			}
			endCmd()

		case c == ' ' || c == '\t' || c == '\r':
			endWord()
			i++

		case c == ';' || c == '|':
			endWord()
			if i+1 < len(command) && command[i+1] == c {
				i += 2
			} else {
				i++
			}
			endCmd()

		case c == '&':
			endWord()
			// &> and &>> redirect; they do not end a command.
			if i+1 < len(command) && command[i+1] == '>' {
				i += 2
				if i < len(command) && command[i] == '>' {
					i++
				}
				redirOut = true
				break
			}
			if i+1 < len(command) && command[i+1] == '&' {
				i += 2
			} else {
				i++
			}
			endCmd()

		case c == '<':
			endWord()
			switch {
			case strings.HasPrefix(command[i:], "<<<"):
				i += 3 // a here-string's word is data, and no body gh reads
			case strings.HasPrefix(command[i:], "<<-"):
				heredocNext = 2
				i += 3
			case strings.HasPrefix(command[i:], "<<"):
				heredocNext = 1
				i += 2
			default:
				i++
			}

		case c == '>':
			endWord()
			if strings.HasPrefix(command[i:], ">>") {
				i += 2
			} else {
				i++
			}
			// >&2 duplicates a descriptor and names no file.
			if i < len(command) && command[i] == '&' {
				i++
				for i < len(command) && command[i] >= '0' && command[i] <= '9' {
					i++
				}
				break
			}
			redirOut = true

		default:
			if expansion(command, i) {
				expands = true
			}
			w.WriteByte(c)
			hasWord = true
			i++
		}
	}
	endCmd()
	return out, i
}

// opensSubshell reports whether the unquoted "(" at i opens a subshell: it
// starts a command — nothing before it but reserved words that run the command
// after them, the last of which may touch it — and is neither a process
// substitution's nor the first of "((", which bash reads as arithmetic.
// Elsewhere the lexer keeps it inside the word.
func opensSubshell(command string, i int, hasWord bool, words []string) bool {
	if hasWord || i > 0 && (command[i-1] == '<' || command[i-1] == '>') ||
		i+1 < len(command) && command[i+1] == '(' {
		return false
	}
	for _, w := range words {
		if !leadsCommand[w] {
			return false
		}
	}
	return true
}

// leadsCommand are the reserved words a subshell may follow in one command.
var leadsCommand = map[string]bool{
	"!": true, "{": true, "if": true, "then": true, "else": true, "elif": true,
	"do": true, "while": true, "until": true, "time": true,
}

// catsStdin reports whether words are a `cat` that prints only its stdin.
func catsStdin(words []string) bool {
	switch len(words) {
	case 1:
		return words[0] == "cat"
	case 2:
		return words[0] == "cat" && words[1] == "-"
	}
	return false
}

// bodyExpands reports whether an unquoted heredoc body holds an expansion bash
// performs. There a backslash escapes `$`, a backtick and itself, and quotes are
// ordinary text.
func bodyExpands(s string) bool {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == '$' && i+1 < len(s) && s[i+1] == '(', expansion(s, i):
			return true
		}
	}
	return false
}

// expansion reports whether the unescaped byte at i, outside single quotes,
// starts an expansion bash performs: a backtick, or a `$` naming a parameter —
// `${`, a name, a digit, or one of the special parameters. A `$` before
// anything else is a literal dollar sign to bash, as in "costs $ 5".
func expansion(command string, i int) bool {
	switch command[i] {
	case '`':
		return true
	case '$':
		if i+1 >= len(command) {
			return false
		}
		n := command[i+1]
		return n == '{' || n == '_' || n >= 'a' && n <= 'z' || n >= 'A' && n <= 'Z' ||
			n >= '0' && n <= '9' || strings.IndexByte("@*#?$!-", n) >= 0
	}
	return false
}

// substitution reads $(...) beginning at i. It returns the text the
// substitution contributes to the word it sits in, the commands it runs, and
// the index just past its ")".
//
// Reading it as a unit rather than as bytes is the whole point: the
// substitution's own quotes, newlines and parentheses stay inside it, so a
// document passed through one can no longer close the outer quote and scatter
// the rest of the command line across words the caller never sees.
//
// Its value is what the command inside prints, which this lexer does not run
// and will not guess — with one exception it can read off the command line
// itself. A heredoc fed to the substitution is data the caller wrote in place,
// and `--body "$(cat <<'EOF' … EOF)"` is how a body reaches gh. So the value is
// the heredoc text where there is one, and the substitution's own source
// otherwise: a word a caller cannot resolve must stay unresolvable rather than
// read as absent, because "" is indistinguishable from an argument nobody
// wrote.
//
// The commands inside stay inside. A caller asking whether a read reaches the
// transcript needs to know it was captured, and inner commands returned as
// ordinary ones lose exactly that: `"$(cat .pgpass)"` puts a secret in a
// variable, which is the idiom the secret guard steers toward.
//
// $((…)) is arithmetic and names no command.
//
// read reports whether the text is the value bash substitutes rather than the
// substitution's own source: only a lone `cat` or `cat -` printing its one
// heredoc, unexpanded, prints exactly that heredoc.
func substitution(command string, i int) (text string, read bool, next int) {
	if strings.HasPrefix(command[i:], "$((") {
		if j := strings.Index(command[i+3:], "))"); j >= 0 {
			next := i + 3 + j + 2
			return command[i:next], false, next
		}
		return command[i:], false, len(command)
	}
	inner, next := lex(command, i+2, true)
	var b strings.Builder
	read = len(inner) == 1 && len(inner[0].Heredocs) == 1 && catsStdin(inner[0].Words)
	for _, c := range inner {
		for j, h := range c.Heredocs {
			b.WriteString(h)
			if c.HeredocExpands[j] {
				read = false
			}
		}
	}
	if b.Len() == 0 {
		return command[i:next], false, next
	}
	return b.String(), read, next
}

// readHeredocs consumes the queued heredoc bodies starting at i, which is the
// first byte of the line after the one that declared them, and returns the
// index just past the last delimiter line. A heredoc the command line never
// terminates ends at the end of the command line, which is what bash reports
// and what the runtime would have sent anyway.
func readHeredocs(s string, i int, queue []pending) int {
	for _, p := range queue {
		var body strings.Builder
		for i < len(s) {
			line := s[i:]
			next := len(s)
			if j := strings.IndexByte(line, '\n'); j >= 0 {
				line, next = line[:j], i+j+1
			}
			end := strings.TrimRight(line, "\r")
			if p.strip {
				end = strings.TrimLeft(end, "\t")
			}
			i = next
			if end == p.delim {
				break
			}
			body.WriteString(line)
			body.WriteByte('\n')
		}
		p.owner.Heredocs = append(p.owner.Heredocs, body.String())
		p.owner.HeredocExpands = append(p.owner.HeredocExpands, p.expand && bodyExpands(body.String()))
	}
	return i
}

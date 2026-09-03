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
type Command struct {
	Words    []string
	Heredocs []string
	Out      string
}

// pending is a heredoc whose delimiter has been read and whose body has not:
// the body starts on the line after the one declaring it.
type pending struct {
	delim string
	strip bool
	owner *Command
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
func lex(command string, i int, stopAtParen bool) ([]*Command, int) {
	var out []*Command
	cur := &Command{}
	var w strings.Builder
	hasWord := false

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
		switch {
		case heredocNext != 0:
			queue = append(queue, pending{delim: s, strip: heredocNext == 2, owner: cur})
			heredocNext = 0
		case redirOut:
			cur.Out = s
			redirOut = false
		default:
			cur.Words = append(cur.Words, s)
		}
	}
	endCmd := func() {
		endWord()
		if len(cur.Words) > 0 || len(cur.Heredocs) > 0 {
			out = append(out, cur)
		}
		cur = &Command{}
	}

	for i < len(command) {
		c := command[i]
		switch {
		case c == '$' && i+1 < len(command) && command[i+1] == '(':
			var text string
			text, i = substitution(command, i)
			w.WriteString(text)
			hasWord = true

		case stopAtParen && c == ')':
			endCmd()
			return out, i + 1

		case c == '\\' && i+1 < len(command):
			if command[i+1] != '\n' {
				w.WriteByte(command[i+1])
				hasWord = true
			}
			i += 2

		case c == '\'':
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
					text, i = substitution(command, i)
					w.WriteString(text)
					continue
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
			w.WriteByte(c)
			hasWord = true
			i++
		}
	}
	endCmd()
	return out, i
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
func substitution(command string, i int) (string, int) {
	if strings.HasPrefix(command[i:], "$((") {
		if j := strings.Index(command[i+3:], "))"); j >= 0 {
			next := i + 3 + j + 2
			return command[i:next], next
		}
		return command[i:], len(command)
	}
	inner, next := lex(command, i+2, true)
	var b strings.Builder
	for _, c := range inner {
		for _, h := range c.Heredocs {
			b.WriteString(h)
		}
	}
	if b.Len() == 0 {
		return command[i:next], next
	}
	return b.String(), next
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
	}
	return i
}

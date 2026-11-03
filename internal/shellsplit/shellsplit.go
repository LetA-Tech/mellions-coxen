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

import (
	"reflect"
	"slices"
	"strings"
)

// Command is one command in a compound: its words with quoting removed, the
// bodies of the heredocs it declares, and the file it redirects stdout to.
//
// Stdin lists the indexes in Words of the operands of <, <> and <<<: a file or
// text a redirection opens for reading, held in place among the words but
// never the command's name or an option's operand. A redirection target that
// holds a substitution is indexed there too, since bash runs what it holds.
type Command struct {
	Words    []string
	Heredocs []string
	Out      string
	Stdin    []int
}

// Args returns the words the command runs with: its name and arguments, never
// a file or text a redirection reads on stdin, which bash does not pass. A
// guard that reads a subcommand or an operand by position reads these.
func (c *Command) Args() []string {
	if len(c.Stdin) == 0 {
		return c.Words
	}
	stdin := make(map[int]bool, len(c.Stdin))
	for _, i := range c.Stdin {
		stdin[i] = true
	}
	args := make([]string, 0, len(c.Words)-len(c.Stdin))
	for i, w := range c.Words {
		if !stdin[i] {
			args = append(args, w)
		}
	}
	return args
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
// It is a lexer and not a shell: it resolves quoting, drops comments, splits
// at the operators that separate one command from the next, and reads heredoc
// bodies as the data they are rather than as more command line. Nothing is expanded, so a
// body reaching gh through a variable is not seen — what is seen is what the
// command line itself carries.
func Split(command string) []*Command {
	cmds, _ := lex(command, 0, false, true)
	return cmds
}

// Readings returns every reading of command a guard acts on: the one Split
// returns and, where it differs, the one that takes no # as a comment. Where
// the comment rule misjudges bash, a command it drops is still in the second,
// so a guard that walks each reading from the start sees all a lexer blind to
// comments sees.
func Readings(command string) [][]*Command {
	read := Split(command)
	literal, _ := lex(command, 0, false, false)
	if reflect.DeepEqual(read, literal) {
		return [][]*Command{read}
	}
	return [][]*Command{read, literal}
}

// lex reads simple commands from i and returns the index it stopped at.
// stopAtParen ends the read at an unquoted ")", which is how a command
// substitution closes; it is what makes the lexer re-entrant. comments is
// whether an unquoted # starting a word comments out the rest of the line.
func lex(command string, i int, stopAtParen, comments bool) ([]*Command, int) {
	var out []*Command
	cur := &Command{}
	var w strings.Builder
	hasWord := false

	var queue []pending
	redirOut := false
	redirIn := false
	heredocNext := 0 // 0 none, 1 <<, 2 <<-
	// dup is the operator, '<' or '>', whose target is the next word: <& and >&
	// take a descriptor glued or spaced, and >& takes a file where the word is
	// not one.
	var dup byte
	// quoted is whether the pending word holds a quote, an escape or a
	// substitution: bash reads a descriptor prefix only from literal text.
	quoted := false
	// runs is whether the pending word holds a substitution, whose command
	// bash runs wherever the word sits.
	runs := false
	// Inside a backtick, ${…}, [[ … ]] or (( … )) bash reads one word, or its
	// own grammar, across the blanks this lexer splits at, so a # there may not
	// start a comment. While one is open a # is kept as text: tick is an open
	// backtick, brace the ${ depth, cond an open [[ and condParen the paren
	// depth inside it, arith the (( paren depth. Each closes only where bash
	// closes it, so an imprecise count keeps text rather than hiding it.
	tick := false
	brace := 0
	cond := false
	condParen := 0
	arith := 0
	// comment reports whether the # at i starts a comment. bash reads no blank
	// in a carriage return, though this lexer ends a word there.
	comment := func(i int) bool {
		return !hasWord && !tick && brace == 0 && !cond && arith == 0 &&
			(i == 0 || command[i-1] != '\r')
	}

	endWord := func() {
		if !hasWord {
			return
		}
		s := w.String()
		w.Reset()
		hasWord = false
		switch {
		case s == "[[":
			cond = true
		case s == "]]" && !quoted && condParen == 0:
			cond = false
		}
		quoted = false
		// A target that runs a command stays a word for the guards that read
		// what runs, and out of Args, since bash passes it to nothing.
		target := runs || strings.Contains(s, "`")
		runs = false
		keep := func() {
			cur.Stdin = append(cur.Stdin, len(cur.Words))
			cur.Words = append(cur.Words, s)
		}
		switch {
		case heredocNext != 0:
			queue = append(queue, pending{delim: s, strip: heredocNext == 2, owner: cur})
			heredocNext = 0
		case redirOut:
			cur.Out = s
			redirOut = false
			if target {
				keep()
			}
		case dup != 0:
			// <& of a literal word that names no descriptor is an ambiguous
			// redirect, and bash runs nothing.
			switch {
			case target:
				if dup == '>' {
					cur.Out = s
				}
				keep()
			case isDupTarget(s):
			case dup == '>':
				cur.Out = s
			}
			dup = 0
		default:
			if redirIn {
				cur.Stdin = append(cur.Stdin, len(cur.Words))
				redirIn = false
			}
			cur.Words = append(cur.Words, s)
		}
	}
	// endBeforeRedirect ends the word pending at a < or >. Literal digits or a
	// {name} glued to the operator are the descriptor it redirects, so they are
	// no word of the command: `git 2>/dev/null reset` runs git reset. A word a
	// redirection is still waiting for is that redirection's target instead.
	endBeforeRedirect := func() {
		if hasWord && !quoted && !redirOut && !redirIn && dup == 0 && heredocNext == 0 &&
			isDescriptorPrefix(w.String()) {
			w.Reset()
			hasWord = false
			return
		}
		endWord()
	}
	endCmd := func() {
		endWord()
		if len(cur.Words) > 0 || len(cur.Heredocs) > 0 {
			out = append(out, cur)
		}
		cur = &Command{}
		redirIn = false
		dup = 0
	}

	for i < len(command) {
		c := command[i]
		switch {
		case c == '#' && comments && comment(i):
			// An unquoted # starting a word comments out the rest of the line,
			// a ) included, so nothing in it is a word, a quote or an operator.
			// The newline still ends the command and starts any heredoc body.
			if j := strings.IndexByte(command[i:], '\n'); j >= 0 {
				i += j
			} else {
				i = len(command)
			}

		case c == '$' && i+1 < len(command) && command[i+1] == '(':
			var text string
			text, i = substitution(command, i, comments)
			w.WriteString(text)
			hasWord = true
			quoted = true
			runs = true

		case stopAtParen && c == ')':
			endCmd()
			return out, i + 1

		case c == '\\' && i+1 < len(command):
			if command[i+1] != '\n' {
				w.WriteByte(command[i+1])
				hasWord = true
				quoted = true
			}
			i += 2

		case c == '\'':
			j := strings.IndexByte(command[i+1:], '\'')
			hasWord = true
			quoted = true
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
					text, i = substitution(command, i, comments)
					w.WriteString(text)
					runs = true
					continue
				}
				w.WriteByte(command[i])
				i++
			}
			hasWord = true
			quoted = true
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
			// A process substitution takes no descriptor: bash reads
			// `0<(cmd)` as one word.
			if strings.HasPrefix(command[i:], "<(") {
				endWord()
			} else {
				endBeforeRedirect()
			}
			switch {
			case strings.HasPrefix(command[i:], "<<<"):
				i += 3 // a here-string's word is data, and no body gh reads
				redirIn = true
			case strings.HasPrefix(command[i:], "<<-"):
				heredocNext = 2
				i += 3
			case strings.HasPrefix(command[i:], "<<"):
				heredocNext = 1
				i += 2
			case strings.HasPrefix(command[i:], "<>"):
				// <> opens its file for reading on stdin: an operand read as
				// after <, not the file stdout goes to.
				i += 2
				redirIn = true
			case strings.HasPrefix(command[i:], "<("):
				// Process substitution is an argument, not a redirection, and
				// its fragments are no operand of a < written before it.
				i++
				redirIn = false
			case strings.HasPrefix(command[i:], "<&"):
				i += 2
				redirIn = false
				dup = '<'
			default:
				i++
				redirIn = true
			}

		case c == '>':
			// A process substitution takes no descriptor: bash reads
			// `2>(cmd)` as one word.
			if strings.HasPrefix(command[i:], ">(") {
				endWord()
			} else {
				endBeforeRedirect()
			}
			switch {
			case strings.HasPrefix(command[i:], ">>"):
				i += 2
			case strings.HasPrefix(command[i:], ">|"):
				// >| overrides noclobber; its | is part of the operator, not a
				// pipe ending the command.
				i += 2
			default:
				i++
			}
			if i < len(command) && command[i] == '&' {
				i++
				dup = '>'
				break
			}
			redirOut = true

		default:
			switch {
			case c == '`':
				tick = !tick
			case c == '$' && i+1 < len(command) && command[i+1] == '{':
				brace++
			case c == '}' && brace > 0 && !tick:
				brace--
			case c == '(' && (arith > 0 || (!hasWord && strings.HasPrefix(command[i:], "(("))):
				arith++
			case c == ')' && arith > 0:
				arith--
			case c == '(' && cond:
				condParen++
			case c == ')' && condParen > 0:
				condParen--
			}
			w.WriteByte(c)
			hasWord = true
			i++
		}
	}
	endCmd()
	return out, i
}

// isDupTarget reports whether the word after >& or <& names a descriptor: a
// number, a lone - that closes one, or a number then - that moves it.
func isDupTarget(s string) bool {
	s = strings.TrimSuffix(s, "-")
	return s == "" || allDigits(s)
}

// isDescriptorPrefix reports whether a word glued to a redirection operator is
// the descriptor it redirects: a number bash can hold in an int, or {name} for
// bash to allocate one. A larger number is an argument the command receives.
func isDescriptorPrefix(s string) bool {
	if allDigits(s) {
		n := strings.TrimLeft(s, "0")
		return len(n) < 10 || (len(n) == 10 && n <= "2147483647")
	}
	if len(s) < 3 || s[0] != '{' || s[len(s)-1] != '}' {
		return false
	}
	name := s[1 : len(s)-1]
	for i := 0; i < len(name); i++ {
		c := name[i]
		letter := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
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
func substitution(command string, i int, comments bool) (string, int) {
	if strings.HasPrefix(command[i:], "$((") && !comments {
		if j := strings.Index(command[i+3:], "))"); j >= 0 {
			next := i + 3 + j + 2
			return command[i:next], next
		}
		return command[i:], len(command)
	}
	if strings.HasPrefix(command[i:], "$((") {
		depth := 2
		for j := i + 3; j < len(command); j++ {
			switch command[j] {
			case '\'', '"':
				// A quoted parenthesis is no nesting.
				k := strings.IndexByte(command[j+1:], command[j])
				if k < 0 {
					return command[i:], len(command)
				}
				j += k + 1
			case '(':
				depth++
			case ')':
				if depth--; depth == 0 {
					return command[i : j+1], j + 1
				}
			}
		}
		return command[i:], len(command)
	}
	inner, next := lex(command, i+2, true, comments)
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

// Each runs f over every reading of command, each from the start, and returns
// every distinct thing f found in any of them.
func Each[T any](command string, f func([]*Command) []T) []T {
	var out []T
	for _, cmds := range Readings(command) {
		for _, v := range f(cmds) {
			if !slices.ContainsFunc(out, func(o T) bool { return reflect.DeepEqual(o, v) }) {
				out = append(out, v)
			}
		}
	}
	return out
}

// First runs f over each reading of command, each from the start, and returns
// the first result that is not the zero value.
func First[T comparable](command string, f func([]*Command) T) T {
	var zero T
	for _, cmds := range Readings(command) {
		if v := f(cmds); v != zero {
			return v
		}
	}
	return zero
}

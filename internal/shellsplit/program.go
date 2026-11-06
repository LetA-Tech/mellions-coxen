// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"path"
	"strings"
)

// prefixes run the command after them: shell keywords that open a body, and
// wrappers whose own flags and counts come before it.
var prefixes = map[string]bool{
	"do": true, "then": true, "else": true, "{": true, "!": true,
	"sudo": true, "command": true, "exec": true, "nice": true, "env": true,
	"time": true, "timeout": true, "nohup": true,
}

// valued are the wrapper options whose value is the next word, so the value is
// not read as the command.
var valued = map[string]map[string]bool{
	"sudo":    {"-u": true, "-g": true, "-C": true, "-D": true, "-h": true, "-p": true, "-r": true, "-t": true, "-U": true, "-T": true},
	"timeout": {"-s": true, "-k": true},
	"nice":    {"-n": true},
	"env":     {"-u": true, "-C": true},
}

// Program drops the words before a simple command's program: shell keywords
// that open a body, assignments, and wrappers that run the command after their
// own options (`sudo -u x rm`, `timeout -s KILL 60 rm`). Words[0] of what it
// returns is the program, with any leading "(" removed.
func Program(words []string) []string {
	wrapper := ""
	for len(words) > 0 {
		w := strings.TrimLeft(words[0], "(")
		switch {
		case w == "":
			words = words[1:]
		case prefixes[w]:
			words, wrapper = words[1:], w
		case strings.Contains(w, "=") && !strings.HasPrefix(w, "-"):
			words = words[1:]
		case wrapper != "" && valued[wrapper][w] && len(words) > 1:
			words = words[2:]
		case wrapper != "" && (strings.HasPrefix(w, "-") || count(w)):
			words = words[1:]
		default:
			return append([]string{w}, words[1:]...)
		}
	}
	return words
}

// count reports a wrapper argument such as timeout's `60` or `5s`.
func count(w string) bool {
	w = strings.TrimRight(w, "smhd")
	if w == "" {
		return false
	}
	for _, r := range w {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

// Shells are the programs that run a -c string, or with no script operand
// read their script from stdin.
var Shells = map[string]bool{"bash": true, "sh": true, "dash": true, "zsh": true, "ksh": true}

// shellValued are the shell options whose value is the next word.
var shellValued = map[string]bool{"-o": true, "+o": true, "-O": true, "+O": true,
	"--rcfile": true, "--init-file": true}

// Scripts returns what a shell command runs as a command line: the operand
// after -c, the words eval joins, or, with -s or no script operand, the here-strings,
// heredocs and < operands it reads on stdin when stdin is the shell's own. A
// script file it names is not read.
func Scripts(c *Command, words []string, stdin bool) []string {
	if path.Base(words[0]) == "eval" {
		return []string{strings.Join(words[1:], " ")}
	}
	if !Shells[path.Base(words[0])] {
		return nil
	}
	cmode, smode := false, false
	for i := 1; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "--" || w == "-":
			if cmode {
				return words[i+1 : min(i+2, len(words))]
			}
			if i+1 < len(words) && !smode {
				return nil
			}
			i = len(words)
		case shellValued[w]:
			i++
		case strings.HasPrefix(w, "--"):
		case len(w) > 1 && (w[0] == '-' || w[0] == '+'):
			if w[0] == '-' {
				cmode = cmode || strings.ContainsRune(w[1:], 'c')
				smode = smode || strings.ContainsRune(w[1:], 's')
			}
			// A cluster ending in o or O takes the next word as its value.
			if last := w[len(w)-1]; last == 'o' || last == 'O' {
				i++
			}
		case cmode:
			return []string{w}
		case smode:
			i = len(words)
		default:
			return nil
		}
	}
	if cmode || !stdin {
		return nil
	}
	var out []string
	for _, i := range c.Stdin {
		out = append(out, c.Words[i])
	}
	return append(out, c.Heredocs...)
}

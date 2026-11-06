// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"path"
	"slices"
	"strings"
)

// prefixes run the command after them: shell keywords that open a body, and
// wrappers whose own flags and counts come before it.
var prefixes = map[string]bool{
	"do": true, "then": true, "else": true, "{": true, "!": true,
	"if": true, "elif": true, "while": true, "until": true,
	"sudo": true, "command": true, "builtin": true, "exec": true, "nice": true, "env": true,
	"time": true, "timeout": true, "nohup": true, "watch": true,
}

// valued are the wrapper options whose value is the next word, so the value is
// not read as the command.
var valued = map[string]map[string]bool{
	"sudo": {"-u": true, "-g": true, "-C": true, "-D": true, "-h": true, "-p": true, "-r": true, "-t": true, "-U": true, "-T": true,
		"--user": true, "--group": true, "--chdir": true, "--host": true, "--prompt": true, "--other-user": true},
	"timeout": {"-s": true, "-k": true, "--signal": true, "--kill-after": true},
	"nice":    {"-n": true, "--adjustment": true},
	"env":     {"-u": true, "-C": true, "--unset": true, "--chdir": true},
	"exec":    {"-a": true},
	"watch":   {"-n": true, "--interval": true, "-q": true, "--equexit": true},
}

// Program drops the words before a simple command's program: shell keywords
// that open a body, a case arm's pattern or a function's name, assignments, and
// wrappers that run the command after their own options (`sudo -u x rm`,
// `sudo -iu x rm`, `timeout -s KILL 60 rm`). Words[0] of what it returns is the
// program, with any leading "(" removed, and a ")" that closes a subshell around
// it alone.
func Program(words []string) []string {
	wrapper := ""
	for len(words) > 0 {
		w := strings.TrimLeft(words[0], "(")
		if at := strings.Index(w, "()"); at > 0 && !strings.Contains(w[:at], "$") {
			w = strings.TrimLeft(w[at+2:], "({")
			if w == "" {
				words = words[1:]
				continue
			}
			words = append([]string{w}, words[1:]...)
		}
		switch {
		case w == "":
			words = words[1:]
		case w == "case":
			at := slices.Index(words, "in")
			if at < 0 {
				return nil
			}
			words = words[at+1:]
		case len(words) > 1 && words[1] == "()":
			words = words[2:]
		case w == "function":
			words = words[min(2, len(words)):]
		case len(words) > 1 && strings.HasSuffix(w, ")") && !strings.Contains(w, "("):
			words = words[1:]
		case prefixes[path.Base(w)]:
			words, wrapper = words[1:], path.Base(w)
		case strings.Contains(w, "=") && !strings.HasPrefix(w, "-"):
			words = words[1:]
		case wrapper != "" && valuedOption(wrapper, w) && len(words) > 1:
			words = words[2:]
		case wrapper != "" && (strings.HasPrefix(w, "-") || count(w)):
			words = words[1:]
		default:
			if len(words) == 1 && !strings.Contains(w, "(") {
				w = strings.TrimRight(w, ")")
			}
			return append([]string{w}, words[1:]...)
		}
	}
	return words
}

// valuedOption reports whether a wrapper's option takes the next word as its
// value, as a cluster ending in such a letter does.
func valuedOption(wrapper, w string) bool {
	if valued[wrapper][w] {
		return true
	}
	return len(w) > 2 && w[0] == '-' && w[1] != '-' && valued[wrapper]["-"+w[len(w)-1:]]
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
var Shells = map[string]bool{"bash": true, "rbash": true, "sh": true, "dash": true, "ash": true, "zsh": true, "ksh": true,
	"$SHELL": true, "${SHELL}": true, "$BASH": true, "${BASH}": true, "$0": true}

// shellValued are the shell options whose value is the next word.
var shellValued = map[string]bool{"-o": true, "+o": true, "-O": true, "+O": true,
	"--rcfile": true, "--init-file": true}

// Scripts returns what a shell command runs as a command line: the operand
// after -c, the words eval joins, or, with -s or no script operand, what its own
// redirections put on stdin when stdin is the shell's own. A script file it names
// is not read.
func Scripts(c *Command, words []string, stdin bool) []string {
	script, fromStdin := Script(words)
	if fromStdin && stdin {
		return c.Input
	}
	return script
}

// Script returns what a shell or eval runs from its own operands, or reports that
// it takes its script from stdin. A word that is neither, or a shell given a
// script file, has neither.
func Script(words []string) (script []string, fromStdin bool) {
	if path.Base(words[0]) == "eval" {
		operands := words[1:]
		if len(operands) > 0 && operands[0] == "--" {
			operands = operands[1:]
		}
		return []string{strings.Join(operands, " ")}, false
	}
	if !Shells[path.Base(words[0])] {
		return nil, false
	}
	cmode, smode := false, false
	for i := 1; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "--" || w == "-":
			if cmode {
				return words[i+1 : min(i+2, len(words))], false
			}
			if i+1 < len(words) && !smode {
				return nil, false
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
			// A cluster holding o or O takes the next word as its value.
			if strings.ContainsAny(w[1:], "oO") {
				i++
			}
		case cmode:
			return []string{w}, false
		case smode || stdinPath[strings.TrimRight(w, ")")]:
			i = len(words)
		default:
			return nil, false
		}
	}
	return nil, !cmode
}

// stdinPath names the script operands that are the shell's own stdin.
var stdinPath = map[string]bool{"/dev/stdin": true, "/dev/fd/0": true, "/proc/self/fd/0": true}

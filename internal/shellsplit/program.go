// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "strings"

// prefixes run the command after them: shell keywords that open a body, and
// wrappers whose own flags and counts come before it.
var prefixes = map[string]bool{
	"do": true, "then": true, "else": true, "{": true, "!": true,
	"if": true, "elif": true, "while": true, "until": true,
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

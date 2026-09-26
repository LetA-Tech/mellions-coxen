// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package tmpglob decides whether a Bash command recursively deletes a glob
// whose parent is a temporary root every session on the host shares.
//
// `mktemp -d` names every scratch directory on the host `/tmp/tmp.XXXXXXXXXX`,
// whoever made it, so `rm -rf /tmp/tmp.*` written to clean up one session's
// scratch deletes every other session's too — directories in-flight test runs
// and harnesses are reading. What a turn may delete is what it created, by the
// path `mktemp` printed; a glob over the shared root can only name more than
// that. A glob below a directory the session named (`rm -rf "$d"/*`) is not
// refused: the directory is already one path.
package tmpglob

import (
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// roots are the directories whose direct children belong to every session.
var roots = []string{"/tmp", "/var/tmp", "/dev/shm"}

// tmpdirVars are the spellings of $TMPDIR a command line carries unexpanded.
var tmpdirVars = []string{"$TMPDIR", "${TMPDIR}"}

// Find returns the first operand of a recursive rm in command that globs
// directly under a shared temporary root, or "". cwd is where the command line
// starts; a `cd` moves it, so a relative glob is read from where it runs.
//
// It reads what a Bash tool call types, not every way a shell can reach rm:
// a glob in a loop list, `find -delete`, `xargs rm` and a command inside a
// string (`bash -c`, `trap`, `$( )`) are not seen.
func Find(command, cwd string) string {
	dir := cwd
	for _, c := range shellsplit.Split(command) {
		words := strip(c.Words)
		if len(words) == 0 {
			continue
		}
		if (words[0] == "cd" || words[0] == "pushd") && len(words) > 1 {
			if filepath.IsAbs(words[1]) {
				dir = filepath.Clean(words[1])
			} else if dir != "" && !strings.HasPrefix(words[1], "-") &&
				!strings.HasPrefix(words[1], "$") && !strings.HasPrefix(words[1], "~") {
				dir = filepath.Join(dir, words[1])
			} else {
				dir = ""
			}
			continue
		}
		if filepath.Base(words[0]) != "rm" {
			continue
		}
		recursive, operands := parse(words[1:])
		if !recursive {
			continue
		}
		for _, op := range operands {
			if globsRoot(op, dir) {
				return op
			}
		}
	}
	return ""
}

// Reason is what the session is told when Find names an operand.
func Reason(operand string) string {
	return "`rm` of `" + operand + "` deletes by a glob over a temporary directory every session on this host shares.\n\n" +
		"`mktemp` names every session's scratch the same way, so the glob matches other sessions' directories — " +
		"harnesses and test runs that are reading them now — and nothing reports what was taken.\n\n" +
		"Delete the exact paths this turn created: keep what `mktemp` printed (`d=$(mktemp -d)`) and remove `\"$d\"`."
}

// prefixes run the command after them: shell keywords that open a body, and
// wrappers whose own flags and counts come before it.
var prefixes = map[string]bool{
	"do": true, "then": true, "else": true, "{": true, "!": true,
	"sudo": true, "command": true, "exec": true, "nice": true, "env": true,
	"time": true, "timeout": true, "nohup": true,
}

// strip drops the prefixes that run rm without being rm.
func strip(words []string) []string {
	wrapped := false
	for len(words) > 0 {
		w := strings.TrimLeft(words[0], "(")
		switch {
		case w == "":
			words = words[1:]
		case prefixes[w]:
			words, wrapped = words[1:], true
		case strings.Contains(w, "=") && !strings.HasPrefix(w, "-"):
			words = words[1:]
		case wrapped && (strings.HasPrefix(w, "-") || count(w)):
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

// parse reports whether rm's arguments ask for a recursive delete, and its
// operands.
func parse(args []string) (bool, []string) {
	recursive, operands, ended := false, []string{}, false
	for _, a := range args {
		if strings.HasPrefix(a, "#") {
			break
		}
		switch {
		case ended || !strings.HasPrefix(a, "-") || a == "-":
			operands = append(operands, a)
		case a == "--":
			ended = true
		case a == "--recursive":
			recursive = true
		case !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR"):
			recursive = true
		}
	}
	return recursive, operands
}

// globsRoot reports that op carries a glob whose literal parent directory is a
// shared temporary root.
func globsRoot(op, dir string) bool {
	op = strings.TrimRight(op, ")")
	i := strings.IndexAny(op, "*?[")
	if i < 0 {
		return false
	}
	if !filepath.IsAbs(op) && !strings.HasPrefix(op, "$") {
		if dir == "" {
			return false
		}
		op = filepath.Join(dir, op)
		i = strings.IndexAny(op, "*?[")
	}
	slash := strings.LastIndex(op[:i], "/")
	if slash < 0 {
		return false
	}
	parent := op[:slash]
	for _, v := range tmpdirVars {
		if parent == v {
			return true
		}
	}
	if parent == "" {
		parent = "/"
	}
	parent = filepath.Clean(parent)
	for _, r := range roots {
		if parent == r {
			return true
		}
	}
	return false
}

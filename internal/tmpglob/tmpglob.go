// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package tmpglob decides whether a Bash command deletes a glob whose parent is
// a temporary root every session on the host shares.
//
// `mktemp` names every scratch file and directory on the host
// `/tmp/tmp.XXXXXXXXXX`, whoever made it, so `rm -f /tmp/tmp.*` or
// `rm -rf /tmp/tmp.*` written to clean up one session's scratch deletes every
// other session's too — files and directories in-flight test runs and
// harnesses are reading. Without -r it takes only the files, and still reaches
// into other sessions' work. What a turn may delete is what it created, by the
// path `mktemp` printed; a glob over the shared root can only name more than
// that. A glob below a directory the session named (`rm -rf "$d"/*`) is not
// refused: the directory is already one path.
package tmpglob

import (
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// roots are the directories whose direct children belong to every session;
// on macOS /tmp and /var/tmp are links into /private.
var roots = []string{"/tmp", "/var/tmp", "/dev/shm", "/private/tmp", "/private/var/tmp"}

// tmpdirVar reports a spelling of $TMPDIR a command line carries unexpanded:
// $TMPDIR, ${TMPDIR}, or ${TMPDIR:-default} and ${TMPDIR-default}.
func tmpdirVar(s string) bool {
	if s == "$TMPDIR" || s == "${TMPDIR}" {
		return true
	}
	return (strings.HasPrefix(s, "${TMPDIR:-") || strings.HasPrefix(s, "${TMPDIR-")) &&
		strings.HasSuffix(s, "}")
}

// Find returns the first operand of an rm in command, recursive or not, that
// globs directly under a shared temporary root, or "". cwd is where the command
// line starts; a `cd` moves it, so a relative glob is read from where it runs.
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
			} else if tmpdirVar(words[1]) {
				dir = words[1]
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
		for _, op := range operands(words[1:]) {
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
		"`mktemp` names every session's scratch the same way, so the glob matches other sessions' files and directories — " +
		"what harnesses and test runs are reading now — and nothing reports what was taken.\n\n" +
		"Delete the exact paths this turn created: keep what `mktemp` printed (`f=$(mktemp)`, `d=$(mktemp -d)`) and remove `\"$f\"` or `\"$d\"`."
}

// prefixes run the command after them: shell keywords that open a body, and
// wrappers whose own flags and counts come before it.
var prefixes = map[string]bool{
	"do": true, "then": true, "else": true, "{": true, "!": true,
	"sudo": true, "command": true, "exec": true, "nice": true, "env": true,
	"time": true, "timeout": true, "nohup": true,
}

// valued are the wrapper options whose value is the next word, so the value is
// not read as the command: `sudo -u x rm`, `timeout -s KILL 60 rm`.
var valued = map[string]map[string]bool{
	"sudo":    {"-u": true, "-g": true, "-C": true, "-D": true, "-h": true, "-p": true, "-r": true, "-t": true, "-U": true, "-T": true},
	"timeout": {"-s": true, "-k": true},
	"nice":    {"-n": true},
	"env":     {"-u": true, "-C": true},
}

// strip drops the prefixes that run rm without being rm.
func strip(words []string) []string {
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

// operands returns rm's operands: every argument that is not an option, and
// every argument after `--`.
func operands(args []string) []string {
	ops, ended := []string{}, false
	for _, a := range args {
		if strings.HasPrefix(a, "#") {
			break
		}
		switch {
		case ended || !strings.HasPrefix(a, "-") || a == "-":
			ops = append(ops, a)
		case a == "--":
			ended = true
		}
	}
	return ops
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
	if tmpdirVar(parent) {
		return true
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

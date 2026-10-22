// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package ghcmd reads a shell word list as a `gh <noun> <verb>` invocation.
//
// It exists so the guards that examine gh commands share one answer to "is this
// that command, and what did it carry". A second copy of this matcher would not
// fail loudly: a guard that stops recognising its own command goes silent, and a
// silent guard is indistinguishable from one that looked and found nothing. So
// when a way of writing the command is discovered that this does not see, it is
// fixed here and every guard gains the fix together.
package ghcmd

import (
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// Args reports whether a command is a `gh <noun> <verb>` the caller accepts,
// and returns what follows the verb. The program is found as bash finds it:
// past environment assignments, the keywords that open a body (`do`, `then`,
// `{`, `(`) and the wrappers that run the command after their own options
// (`timeout 60`, `env`, `command`), and matched on its base name, so
// `timeout 60 /usr/bin/gh pr create` is the same command as `gh pr create`.
//
// Where the program is something else that may run gh (`xargs gh pr comment`),
// a later `gh <noun> <verb>` the caller accepts is taken as the call, so an
// unrecognised wrapper is read as gh rather than skipped.
func Args(words []string, accept func(noun, verb string) bool) ([]string, bool) {
	for len(words) > 0 && isAssignment(words[0]) {
		words = words[1:]
	}
	words = shellsplit.Program(words)
	if len(words) > 0 && filepath.Base(words[0]) == "gh" {
		if len(words) < 3 || !accept(words[1], words[2]) {
			return nil, false
		}
		return words[3:], true
	}
	for k := 1; k+2 < len(words); k++ {
		if filepath.Base(words[k]) == "gh" && accept(words[k+1], words[k+2]) {
			return words[k+3:], true
		}
	}
	return nil, false
}

func isAssignment(w string) bool {
	i := strings.IndexByte(w, '=')
	if i <= 0 {
		return false
	}
	for _, r := range w[:i] {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// SplitFlag reads one argument as a flag and the value glued to it. gh takes
// --body=X, --body X, -bX, -b=X and -b X, and a hole in any one of those forms
// is a value nothing reads.
func SplitFlag(arg string) (name, glued string, hasGlued bool) {
	switch {
	case strings.HasPrefix(arg, "--"):
		if i := strings.IndexByte(arg, '='); i >= 0 {
			return arg[:i], arg[i+1:], true
		}
		return arg, "", false
	case len(arg) > 1 && arg[0] == '-':
		rest := arg[2:]
		switch {
		case strings.HasPrefix(rest, "="):
			return arg[:2], rest[1:], true
		case rest != "":
			return arg[:2], rest, true
		}
		return arg, "", false
	}
	return arg, "", false
}

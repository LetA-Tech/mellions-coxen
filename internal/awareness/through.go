// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// This runs before every shell command a session issues, so what it may read
// is bounded: a few local files, each small enough to be a script, none run.
const (
	maxThroughBytes = 256 << 10
	maxThroughFiles = 8
	maxThroughDepth = 2
)

// StartedThrough returns the local file that brings a container up when
// command runs it, or "" where none does.
//
// A command line reaches a container start through a script it executes or the
// recipe of a make target it names, and through one further script that file
// runs. Files are read, never executed: no `make -n`, whose `$(shell ...)`
// would run. A path built from an expansion, a file over the size bound, a
// binary or one that is not there is not read; a syntax check or a dry run
// runs nothing, and a comment line is not a start.
func StartedThrough(command, cwd, home string) string {
	t := &through{home: home, budget: maxThroughFiles}
	return t.line(command, cwd, maxThroughDepth)
}

type through struct {
	home   string
	budget int
}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// line walks the simple commands of a command line run from dir.
func (t *through) line(command, dir string, depth int) string {
	if depth == 0 {
		return ""
	}
	for _, c := range shellsplit.Split(command) {
		words := commandWords(c.Words)
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "cd", "pushd":
			if len(words) > 1 && !strings.HasPrefix(words[1], "-") {
				dir = t.resolve(words[1], dir)
			}
		case "make", "gmake":
			if via := t.make(words[1:], dir, depth); via != "" {
				return via
			}
		case "bash", "sh", "zsh", "dash", "source", ".":
			if script := scriptArg(words[1:]); script != "" {
				if via := t.script(t.resolve(script, dir), dir, depth); via != "" {
					return via
				}
			}
		default:
			if strings.Contains(words[0], "/") {
				if via := t.script(t.resolve(words[0], dir), dir, depth); via != "" {
					return via
				}
			}
		}
	}
	return ""
}

// commandWords drops what precedes the program a simple command runs: leading
// variable assignments and the wrappers that run the rest of the line.
func commandWords(words []string) []string {
	for len(words) > 0 {
		w := strings.Trim(words[0], "(){}")
		switch {
		case w == "" || assignment.MatchString(w):
			words = words[1:]
		case w == "env" || w == "time" || w == "nohup" || w == "exec" || w == "command":
			words = words[1:]
			for len(words) > 0 && strings.HasPrefix(words[0], "-") {
				words = words[1:]
			}
		case w == "timeout":
			words = words[1:]
			for len(words) > 0 && strings.HasPrefix(words[0], "-") {
				words = words[1:]
			}
			if len(words) > 0 {
				words = words[1:]
			}
		default:
			out := append([]string{w}, words[1:]...)
			return out
		}
	}
	return nil
}

// scriptArg is the file a shell is handed to run; "" where it is given a
// command string instead or reads its standard input.
func scriptArg(args []string) string {
	for _, a := range args {
		switch {
		case a == "-c" || a == "-n":
			return ""
		case strings.HasPrefix(a, "-"):
			continue
		default:
			return a
		}
	}
	return ""
}

// script reads one executed file and says whether it starts a container, or
// runs a file that does.
func (t *through) script(path, dir string, depth int) string {
	body, ok := t.read(path)
	if !ok {
		return ""
	}
	body = uncommented(body)
	if startsContainer.MatchString(body) {
		return path
	}
	return t.line(body, dir, depth-1)
}

// make reads the recipes of the targets a make invocation names, and of the
// prerequisites they name, from the makefile it would read.
func (t *through) make(args []string, dir string, depth int) string {
	var file string
	var targets []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-C" || a == "--directory" || a == "-f" || a == "--file" || a == "--makefile":
			if i+1 >= len(args) {
				return ""
			}
			i++
			if a == "-C" || a == "--directory" {
				dir = t.resolve(args[i], dir)
			} else {
				file = args[i]
			}
		case strings.HasPrefix(a, "--directory="):
			dir = t.resolve(strings.TrimPrefix(a, "--directory="), dir)
		case strings.HasPrefix(a, "-C"):
			dir = t.resolve(a[2:], dir)
		case strings.HasPrefix(a, "--file=") || strings.HasPrefix(a, "--makefile="):
			file = a[strings.IndexByte(a, '=')+1:]
		case strings.HasPrefix(a, "-f") && len(a) > 2:
			file = a[2:]
		case a == "-n" || a == "-q" || a == "--dry-run" || a == "--just-print" || a == "--recon" || a == "--question":
			return ""
		case strings.HasPrefix(a, "-") || assignment.MatchString(a):
		default:
			targets = append(targets, a)
		}
	}
	if dir == "" {
		return ""
	}
	var path, body string
	candidates := []string{"GNUmakefile", "makefile", "Makefile"}
	if file != "" {
		candidates = []string{file}
	}
	for _, c := range candidates {
		p := t.resolve(c, dir)
		if b, ok := t.read(p); ok {
			path, body = p, b
			break
		}
	}
	if path == "" {
		return ""
	}
	rules := parseRules(body)
	if len(targets) == 0 && rules.first != "" {
		targets = []string{rules.first}
	}
	seen := map[string]bool{}
	for len(targets) > 0 && len(seen) < 32 {
		name := targets[0]
		targets = targets[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		r, ok := rules.byName[name]
		if !ok {
			continue
		}
		recipe := uncommented(strings.Join(r.recipe, "\n"))
		if startsContainer.MatchString(recipe) {
			return path
		}
		if via := t.line(recipe, dir, depth-1); via != "" {
			return via
		}
		targets = append(targets, r.prereqs...)
	}
	return ""
}

// uncommented drops whole-line shell comments: a script that says what it never
// runs is not running it.
func uncommented(body string) string {
	lines := strings.Split(body, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "#") {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

type rule struct {
	prereqs []string
	recipe  []string
}

type rules struct {
	first  string
	byName map[string]*rule
}

// parseRules reads the explicit rules of a makefile: each target, its
// prerequisites and its tab-indented recipe lines with their @, - and + prefixes
// removed. Variables are not expanded, so a recipe is read as written.
func parseRules(body string) rules {
	out := rules{byName: map[string]*rule{}}
	var current []*rule
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "\t") {
			cmd := strings.TrimLeft(strings.TrimSpace(line), "@-+")
			for _, r := range current {
				r.recipe = append(r.recipe, cmd)
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		current = nil
		colon := strings.IndexByte(line, ':')
		if colon <= 0 || strings.HasPrefix(line[colon:], ":=") || strings.ContainsAny(line[:colon], "=$") {
			continue
		}
		after := strings.TrimPrefix(line[colon+1:], ":")
		if semi := strings.IndexByte(after, ';'); semi >= 0 {
			after = after[:semi]
		}
		if strings.HasPrefix(strings.TrimSpace(after), "=") {
			continue
		}
		prereqs := strings.Fields(strings.SplitN(after, "|", 2)[0])
		for _, name := range strings.Fields(line[:colon]) {
			r := out.byName[name]
			if r == nil {
				r = &rule{}
				out.byName[name] = r
			}
			r.prereqs = append(r.prereqs, prereqs...)
			current = append(current, r)
			if out.first == "" && !strings.HasPrefix(name, ".") && !strings.ContainsAny(name, "%") {
				out.first = name
			}
		}
	}
	return out
}

func (t *through) resolve(path, dir string) string {
	switch {
	case path == "" || strings.ContainsAny(path, "$`*?"):
		return ""
	case strings.HasPrefix(path, "~/") && t.home != "":
		path = filepath.Join(t.home, path[2:])
	case filepath.IsAbs(path):
	case dir == "":
		return ""
	default:
		path = filepath.Join(dir, path)
	}
	return filepath.Clean(path)
}

// read returns a regular file's text when it is within the size bound and is
// not a binary, spending one of the files this command may read.
func (t *through) read(path string) (string, bool) {
	if path == "" || t.budget <= 0 {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxThroughBytes {
		return "", false
	}
	t.budget--
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxThroughBytes+1))
	if err != nil || len(b) > maxThroughBytes || bytes.IndexByte(b, 0) >= 0 {
		return "", false
	}
	return string(b), true
}

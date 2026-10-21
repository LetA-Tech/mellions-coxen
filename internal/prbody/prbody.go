// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package prbody decides whether a Bash tool call opens or edits a pull
// request whose body declares that it closes an issue, and whether GitHub
// would resolve that declaration on the base the command names.
//
// GitHub resolves a closing keyword only when the pull request merges into the
// repository's default branch. On any other base the keyword does nothing: the
// fix merges, the issue quietly stays open, and nothing reports it. That body
// is written at a moment no Skill is loaded and no gate can see, so the
// PreToolUse hook denies the tool call there, where it is typed — a denial
// becomes a tool result the session must answer, which advisory text delivered
// alongside the call is not shown to do.
//
// Whether a close is warranted on the default branch is a question of
// authority, and that is the closure Skill's and the partnership's: a body with
// no --base, or a base that is the default branch, passes.
//
// The question is four smaller ones, and each is a parse rather than a search:
// which commands in the payload open a pull request, what text each one hands
// GitHub as the body, which of that text is prose rather than quotation, and
// whether a closing keyword in that prose is declared or denied. A search over
// the payload answers none of them — it cannot see a command boundary, a code
// span, a line wrap or a clause — so all four are parsed here.
package prbody

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/ghcmd"
	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// bodyLimit bounds a --body-file read. A body past this is not a body.
const bodyLimit = 1 << 20

// Call is one `gh pr create` or `gh pr edit` in a command, with the text it
// hands GitHub as the body and the base and repository it names.
type Call struct {
	Base   string
	Repo   string
	Bodies []string
	// Unread is each --body-file whose text this parse could not read. Its
	// entry in Bodies is "", which is not what gh will publish.
	Unread []Unreadable
	// Dir is the directory this call runs in: the session's, or wherever the
	// command's own `cd` moved to before it. A citation is a claim about the
	// tree the body is published FROM, and `cd <worktree> && gh pr create` is
	// how a lane publishes, so a checker reading the session directory reads a
	// different tree and endorses lines that are wrong for the branch (#27).
	Dir string
}

// Unreadable is a --body-file spec as written, and why its text was not read.
type Unreadable struct {
	Spec string
	Why  string
}

// Close is a closing declaration a body makes.
type Close struct {
	// Text is the keyword and its reference as written, on one line.
	Text string
	// Ref is the reference alone: #7, owner/repo#7, or an issue URL.
	Ref string
}

// Deny returns the reason to refuse a PreToolUse payload, or "" to stay
// silent. defaultBranch answers with the repository's default branch, or ""
// where it cannot be read — unknown means silence, because a deny on a guess
// blocks a legitimate close in a repository nothing here can see.
func Deny(payload []byte, defaultBranch func(cwd, repo string) string) string {
	var ev struct {
		ToolName string `json:"tool_name"`
		Cwd      string `json:"cwd"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" {
		return ""
	}
	for _, call := range Calls(ev.Input.Command, ev.Cwd) {
		// Without --base, gh targets the default branch, where the keyword
		// does exactly what it says.
		if call.Base == "" {
			continue
		}
		var found Close
		for _, body := range call.Bodies {
			if c, ok := Declared(body); ok {
				found = c
				break
			}
		}
		if found.Text == "" {
			continue
		}
		def := defaultBranch(ev.Cwd, call.Repo)
		if def == "" || call.Base == def {
			continue
		}
		return "\"" + found.Text + "\" in a pull request into " + call.Base +
			": GitHub resolves a closing keyword only on the default branch, " + def +
			". This merge will not close " + found.Ref + ", and nothing will report that it did not.\n\n" +
			"Write \"Refs " + found.Ref + "\" instead, and close " + found.Ref +
			" by hand once the merge is verified — mellions-issue-closure decides when, " +
			"and CONTRIBUTING.md says the same."
	}
	return ""
}

// Calls returns every pull-request call the command makes. A command line that
// mixes them with other work — a `git commit` whose message quotes a body, an
// `echo` or a heredoc that writes text about opening a pull request — yields
// only the `gh pr create` and `gh pr edit` commands themselves, so a keyword
// belonging to some other command's text is not read as a pull request body.
//
// cwd resolves a relative --body-file. A body-file the same command line is
// about to write does not exist yet when PreToolUse runs, so the heredoc that
// writes it is read instead of the stale file.
func Calls(command, cwd string) []Call {
	return calls(command, cwd, func(noun, verb string) bool {
		return noun == "pr" && (verb == "create" || verb == "edit")
	})
}

// Publishing returns every call the command makes that hands GitHub a body a
// person will read: a pull request or issue opened, edited, commented on, or
// reviewed. It is the wider set Calls narrows — a closing keyword only means
// anything on a pull request, but a citation is a claim about code wherever it
// is published, and three of the five citations #149 records were published on
// an issue rather than in a pull request body.
func Publishing(command, cwd string) []Call {
	return calls(command, cwd, func(noun, verb string) bool {
		switch noun {
		case "pr":
			return verb == "create" || verb == "edit" || verb == "comment" || verb == "review"
		case "issue":
			return verb == "create" || verb == "edit" || verb == "comment"
		}
		return false
	})
}

func calls(command, cwd string, accept func(noun, verb string) bool) []Call {
	cmds := shellsplit.Split(command)

	written := map[string]string{}
	for _, c := range cmds {
		if c.Out != "" && len(c.Heredocs) > 0 {
			written[c.Out] = strings.Join(c.Heredocs, "")
		}
	}

	var out []Call
	dir := cwd
	vars := map[string]string{}
	for _, c := range cmds {
		if assign(c.Words, vars) {
			continue
		}
		if moved, ok := chdir(expandWords(c.Words, vars), cwd, dir); ok {
			dir = moved
			continue
		}
		args, ok := ghcmd.Args(c.Words, accept)
		if !ok {
			continue
		}
		call := Call{Dir: dir}
		for i := 0; i < len(args); i++ {
			name, glued, hasGlued := ghcmd.SplitFlag(args[i])
			var v string
			switch name {
			case "--body", "-b", "--body-file", "-F", "--base", "-B", "--repo", "-R":
				if hasGlued {
					v = glued
				} else if i+1 < len(args) {
					i++
					v = args[i]
				}
			default:
				continue
			}
			switch name {
			case "--body", "-b":
				call.Bodies = append(call.Bodies, v)
			case "--body-file", "-F":
				body, why := bodyFile(v, c, written, vars, dir)
				call.Bodies = append(call.Bodies, body)
				if why != "" {
					call.Unread = append(call.Unread, Unreadable{Spec: v, Why: why})
				}
			case "--base", "-B":
				if call.Base == "" {
					call.Base = v
				}
			case "--repo", "-R":
				if call.Repo == "" {
					call.Repo = v
				}
			}
		}
		out = append(out, call)
	}
	return out
}

// Chdir is chdir for a guard that reads another gh verb in the same command:
// where a `cd` earlier in the line moves the directory a call runs in.
func Chdir(words []string, cwd, dir string) (string, bool) { return chdir(words, cwd, dir) }

// chdir reports the directory a `cd` moves to, and whether the command was a
// `cd` at all. A target this cannot establish — no argument, `cd -`, a `~user`
// form — resets to the session directory rather than guessing: the whole point
// of #27 is that resolving a body against the wrong tree reads exactly like
// resolving it against the right one, so an unknown target must degrade to the
// old behaviour and never to a fabricated path.
//
// Operators are not visible here (shellsplit yields simple commands), so a `cd`
// guarded by `||` or confined to a subshell is followed anyway. That is the
// safe direction: the dominant form by far is `cd <worktree> && gh ...`, and
// being wrong the other way is the defect.
func chdir(words []string, cwd, dir string) (string, bool) {
	if len(words) == 0 || words[0] != "cd" {
		return "", false
	}
	target := ""
	for _, w := range words[1:] {
		if strings.HasPrefix(w, "-") && w != "-" {
			continue // an option such as -P, not the destination
		}
		target = w
		break
	}
	switch {
	case target == "", target == "-":
		return cwd, true
	case target == "~":
		if home, err := os.UserHomeDir(); err == nil {
			return home, true
		}
		return cwd, true
	case strings.HasPrefix(target, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return cwd, true
		}
		return filepath.Join(home, target[2:]), true
	case strings.HasPrefix(target, "~"):
		return cwd, true // ~otheruser: not ours to expand
	case filepath.IsAbs(target):
		return filepath.Clean(target), true
	}
	return filepath.Join(dir, target), true
}

// bodyFile reads what --body-file names: the command's own heredoc for "-",
// the heredoc this command line is about to write to that path, else the file
// as it stands. Text arriving on stdin from another process is not visible to
// a PreToolUse hook and is not read.
//
// why is non-empty where the text gh will publish is not what this returns:
// a spec still holding `$` or a backtick once this line's own assignments are
// expanded names whatever the shell makes of it, which this parse never knows,
// and a path with no regular file behind it has no text here to grade.
func bodyFile(spec string, c *shellsplit.Command, written, vars map[string]string, cwd string) (body, why string) {
	const unseen = "no regular file is at that path when this command runs"
	switch spec {
	case "":
		return "", "an empty path"
	case "-":
		// Stdin from a pipe, a here-string or a redirect never reaches this
		// parse, so only the command's own heredoc is a body it has read.
		if len(c.Heredocs) == 0 {
			return "", "stdin from outside a heredoc in this command, which this hook cannot see"
		}
		return strings.Join(c.Heredocs, ""), ""
	}
	if b, ok := written[spec]; ok {
		return b, ""
	}
	path := expand(spec, vars)
	if b, ok := written[path]; ok {
		return b, ""
	}
	if strings.ContainsAny(path, "$`") {
		return "", "a shell expansion this hook does not perform"
	}
	if !filepath.IsAbs(path) && cwd != "" {
		if strings.ContainsAny(cwd, "$`") {
			return "", "relative to a `cd` through a shell expansion this hook does not perform"
		}
		path = filepath.Join(cwd, path)
	}
	// A regular file and nothing else: a path in the payload is not this
	// hook's to trust, and opening a fifo would hang the tool call.
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return "", unseen
	}
	f, err := os.Open(path)
	if err != nil {
		return "", unseen
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, bodyLimit))
	if err != nil {
		return "", unseen
	}
	return string(b), ""
}

// assign records a command that only assigns shell variables — `NAME=value`
// or `export NAME=value` — and reports whether it was one. A value holding an
// expansion of its own is stored as written: expand substitutes once, so the
// `$` or backtick survives and the spec is still refused. A `NAME=value`
// prefixing a command is that command's environment and persists nowhere.
func assign(words []string, vars map[string]string) bool {
	ws := words
	if len(ws) > 0 && ws[0] == "export" {
		ws = ws[1:]
	}
	if len(ws) == 0 {
		return false
	}
	for _, w := range ws {
		name, _, ok := strings.Cut(w, "=")
		if !ok || !isName(name) {
			return false
		}
	}
	for _, w := range ws {
		name, val, _ := strings.Cut(w, "=")
		vars[name] = val
	}
	return true
}

func isName(s string) bool {
	for i, r := range s {
		if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return s != ""
}

// expand substitutes $NAME and ${NAME} for each variable this line assigned.
// Any other expansion is left as written, so the caller still sees its `$`.
func expand(s string, vars map[string]string) string {
	if !strings.Contains(s, "$") || len(vars) == 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			b.WriteByte(s[i])
			continue
		}
		name, end := "", i+1
		if end < len(s) && s[end] == '{' {
			if close := strings.IndexByte(s[end:], '}'); close > 0 {
				name, end = s[end+1:end+close], end+close+1
			}
		} else {
			for end < len(s) && isName(s[i+1:end+1]) {
				end++
			}
			name = s[i+1 : end]
		}
		if v, ok := vars[name]; ok && isName(name) {
			b.WriteString(v)
			i = end - 1
			continue
		}
		b.WriteByte('$')
	}
	return b.String()
}

func expandWords(words []string, vars map[string]string) []string {
	if len(vars) == 0 {
		return words
	}
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = expand(w, vars)
	}
	return out
}

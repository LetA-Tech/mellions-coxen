// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// CommitMessages returns the message text of every `git commit` and
// `git merge` in the command, behind any wrapper shellsplit.Program removes:
// each -m/--message and --trailer value, and each -F/--file read the way
// bodyFile reads a body-file. GitHub resolves a closing keyword in a commit
// message once that commit reaches the default branch, by a push there or by
// the promotion that carries a working branch into it.
func CommitMessages(command, cwd string) []string {
	return shellsplit.Each(command, func(cmds []*shellsplit.Command) []string {

		written := map[string]string{}
		for _, c := range cmds {
			if c.Out != "" && len(c.Heredocs) > 0 {
				written[c.Out] = strings.Join(c.Heredocs, "")
			}
		}

		var out []string
		dir := cwd
		for _, c := range cmds {
			if moved, ok := chdir(c.Args(), cwd, dir); ok {
				dir = moved
				continue
			}
			words := shellsplit.Program(c.Args())
			if len(words) < 2 || filepath.Base(words[0]) != "git" {
				continue
			}
			at, args := gitGlobals(words[1:], dir)
			if len(args) == 0 || (args[0] != "commit" && args[0] != "merge") {
				continue
			}
			args = args[1:]
			for i := 0; i < len(args); i++ {
				a := args[i]
				if a == "--" {
					break
				}
				if strings.HasPrefix(a, "--") {
					name, v, glued := strings.Cut(a, "=")
					switch name {
					case "--message", "--trailer", "--file":
					default:
						continue
					}
					if !glued {
						if i+1 >= len(args) {
							continue
						}
						i++
						v = args[i]
					}
					switch name {
					case "--message":
						out = append(out, messageText(v))
					case "--trailer":
						// git writes a trailer given as Key=value as "Key: value".
						if k, rest, ok := strings.Cut(v, "="); ok && !strings.Contains(k, ":") {
							v = k + ": " + rest
						}
						out = append(out, messageText(v))
					default:
						out = append(out, bodyFile(v, c, written, at))
					}
					continue
				}
				if len(a) < 2 || a[0] != '-' {
					continue
				}
				// A short-option cluster such as -am: the first option that takes
				// a value takes the rest of the cluster, or the next word.
				for j := 1; j < len(a); j++ {
					opt := a[j]
					if !strings.ContainsRune("mFcCtSu", rune(opt)) {
						continue
					}
					v := a[j+1:]
					if v == "" && opt != 'S' && opt != 'u' && i+1 < len(args) {
						i++
						v = args[i]
					}
					switch opt {
					case 'm':
						out = append(out, messageText(v))
					case 'F':
						out = append(out, bodyFile(v, c, written, at))
					}
					break
				}
			}
		}
		return out
	})
}

// messageText returns the part of a message argument this can read as text:
// what precedes the first $( and every heredoc after it. A substitution's
// output is not known before it runs, so its code is not read as the message;
// a heredoc inside one is the text it prints. A backtick is read as text, since
// the lexer does not say whether single quotes made it literal.
func messageText(v string) string {
	i := strings.Index(v, "$(")
	if i < 0 {
		return v
	}
	parts := []string{v[:i]}
	lines := strings.Split(v[i:], "\n")
	for n := 0; n < len(lines); n++ {
		m := heredocStart.FindStringSubmatch(lines[n])
		if m == nil {
			continue
		}
		var body []string
		for n++; n < len(lines) && strings.TrimSpace(lines[n]) != m[1]; n++ {
			body = append(body, lines[n])
		}
		parts = append(parts, strings.Join(body, "\n"))
	}
	return strings.Join(parts, "\n")
}

var heredocStart = regexp.MustCompile(`<<-?[ \t]*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// gitGlobals reads the global options in front of a git subcommand and returns
// the directory the invocation runs in, with the words from the subcommand on.
// -C moves the directory, relative to the last one.
func gitGlobals(args []string, dir string) (string, []string) {
	at := dir
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-C" && i+1 < len(args):
			if p := args[i+1]; filepath.IsAbs(p) {
				at = filepath.Clean(p)
			} else {
				at = filepath.Join(at, p)
			}
			i++
		case a == "-c" || a == "--git-dir" || a == "--work-tree" || a == "--namespace":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			return at, args[i:]
		}
	}
	return at, nil
}

// denyCommitMessage returns the reason to refuse a closing keyword in a git
// commit or merge message, or "". The commit reaches the default branch at a
// push there or at the next promotion, and GitHub closes the issue then, so
// every branch is denied, negated or not, as a merge message is. A pull
// request that is itself the whole proof declares its close in its own body.
func denyCommitMessage(messages []string) string {
	for _, text := range messages {
		m := closing.FindStringSubmatchIndex(text)
		if m == nil {
			continue
		}
		ref := text[m[2*refGroup]:m[2*refGroup+1]]
		return "\"" + strings.Join(strings.Fields(text[m[0]:m[1]]), " ") + "\" in a git commit or merge message: " +
			"GitHub closes " + ref + " when this commit reaches the default branch — at the push if it is pushed " +
			"there, at the next promotion if it lands on a working branch — before any deploy and before the proof " +
			"comment mellions-issue-closure requires is on the record.\n\n" +
			"Write \"Refs " + ref + "\" in the message instead, and close " + ref +
			" by hand once its acceptance is met. A pull request that is itself the whole proof declares the close " +
			"in its own body, never in a commit message."
	}
	return ""
}

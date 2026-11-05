// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package secretread decides whether a tool call would read the content of a
// credential-bearing file into the transcript.
//
// A parser that assumes KEY=value can print a bare credential URI unchanged.
// The check therefore classifies the command and path at the point before any
// file content can enter the transcript.
//
// The classification is deliberately asymmetric. A denial costs one rewritten
// command; a miss puts a live credential in a transcript that is already sent.
// So a command shape this package cannot prove non-printing is denied.
package secretread

import (
	"path"
	"regexp"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// Finding is one credential-bearing path a tool call would read.
type Finding struct {
	// Path is the path as the command wrote it.
	Path string
	// Reader is the command word that would print it — "cat", "awk" — or
	// "" when the tool reads the file directly rather than through a shell.
	Reader string
	// Value is true when Path names a variable assigned a word that contains
	// `$(` or a backtick and a credential file's name.
	Value bool
	// Substituted is true when one of the reader's arguments contains `$(` or a
	// backtick and the credential file name in Path.
	Substituted bool
	// Held is true when Path names a variable assigned a credential's path
	// earlier on the same command line.
	Held bool
}

// Reason is the sentence a denial gives for f. The scan matches names — of
// files, of programs, of variables — and never models what a command does with
// an argument, so the sentence states the names it matched and no more.
func (f Finding) Reason() string {
	switch {
	case f.Reader == "":
		return "`" + f.Path + "` is named like a credential-bearing file, and this tool " +
			"can return a file's content."
	case f.Value:
		return "`" + f.Path + "` was assigned, earlier on this command line, a word " +
			"containing `$(` or a backtick and a credential file's name, and `" + f.Reader + "` is on the " +
			"guard's list of programs that can print what they are given."
	case f.Substituted:
		return "an argument to `" + f.Reader + "` contains `$(` or a backtick and the " +
			"credential file name `" + f.Path + "`, and `" + f.Reader + "` is on the guard's list of programs that can " +
			"print what they are given."
	case f.Held:
		return "`" + f.Path + "` was assigned a path named like a credential file earlier on " +
			"this command line, and `" + f.Reader + "` is not on the guard's list of programs " +
			"that never print a file's content."
	}
	return "`" + f.Reader + " … " + f.Path + "` — `" + f.Path + "` is named like a " +
		"credential file, and `" + f.Reader + "` is not on the guard's list of programs " +
		"that never print a file's content."
}

// safeReaders are the command words that take a path and never write its
// content to stdout. The list is short on purpose: a word is here only when
// reading the bytes out is not among the things it does.
//
// Deliberately absent, and each for a reason a session will otherwise argue
// with: `git`, because `git diff` and `git show` print file content; `find`,
// because `-exec cat` is a command word this scan never sees; `xargs` and
// `env` for the same reason; `diff`, `sort`, `uniq`, `tee` and `jq`, which are
// all printers people think of as inspectors.
var safeReaders = map[string]bool{
	"wc": true, "stat": true, "ls": true, "test": true, "[": true,
	"chmod": true, "chown": true, "chgrp": true, "touch": true,
	"mkdir": true, "cp": true, "mv": true, "rm": true, "shred": true,
	"install": true, "ln": true, "realpath": true, "dirname": true,
	"basename": true, "mktemp": true, "truncate": true,
	// A build tool's arguments are package and module paths, not files it
	// prints. `go test ./internal/secretread/` was denied on the directory's
	// own name, which is a refusal a session can only get past by rephrasing a
	// correct command — and a guard that has to be worked around is one that
	// gets turned off.
	"go": true, "gofmt": true, "make": true,
	// `mellions` is deliberately NOT here, and the reason is worth the line: it
	// looks like a CLI whose arguments are identifiers, and it carries three
	// `-file` flags that read a caller-named path, one of which stores the bytes
	// in a report `mellions report latest` prints straight back out. Exonerating
	// a binary is a claim about every subcommand it will ever have.
}

// consumers read a file into the shell rather than onto stdout.
var consumers = map[string]bool{"source": true, ".": true}

// printers write their input back out. This list gates the two shapes where a
// credential travels as a value rather than as a path — a variable holding it,
// and a substitution that read it — and there the default is the opposite of
// the one above: capturing a credential is the idiom this guard steers toward,
// so ordinary use of the value must stay silent and only printing it is the
// leak. `psql "$DATABASE_URL"` and `psql "$(tail -1 .db_connection)"` expose
// the value identically, to an argument vector and not to the transcript;
// denying one and allowing the other would teach a distinction that is not
// there.
var printers = map[string]bool{
	"echo": true, "printf": true, "print": true, "cat": true, "less": true,
	"more": true, "head": true, "tail": true, "awk": true, "gawk": true,
	"sed": true, "grep": true, "egrep": true, "fgrep": true, "rg": true,
	"ag": true, "cut": true, "tr": true, "xxd": true, "od": true,
	"strings": true, "base64": true, "jq": true, "tee": true, "column": true,
	"fold": true, "nl": true, "rev": true, "sort": true, "uniq": true,
	"diff": true, "yq": true, "hexdump": true,
}

// consumedFlags names, per reader, the option whose operand the command uses
// as a key rather than printing. `ssh -i <key> host` hands the file to the
// crypto and writes it nowhere, so the denial it drew could only be got past by
// rewriting a correct command.
//
// This narrows to the operand and deliberately does NOT exonerate the reader:
// `ssh host cat .env` still prints one, and putting `ssh` in safeReaders would
// miss it.
var consumedFlags = map[string]map[string]bool{
	"ssh":  {"-i": true},
	"scp":  {"-i": true},
	"sftp": {"-i": true},
}

// wrappers stand in front of the real command word without changing what it
// does with its arguments.
var wrappers = map[string]bool{"sudo": true, "command": true, "builtin": true, "nohup": true, "time": true}

// prefixOperands reports how many words a prefix command consumes before the
// real command word begins, for the wrappers that take operands of their own.
// Without this the command word resolves to the prefix's own argument — a bare
// duration — which is in no allowlist, so every argument behind it is scanned
// as though a stranger were printing it. Returning the count rather than
// stripping in place keeps the caller's loop able to see a second wrapper.
func prefixOperands(base string, rest []string) (int, bool) {
	if base != "timeout" {
		return 0, false
	}
	// timeout [OPTION]... DURATION COMMAND [ARG]...
	n := 1
	for n < len(rest) && strings.HasPrefix(rest[n], "-") {
		// Only the exact spellings take their value as a separate word;
		// `-k5s` and `--kill-after=5s` carry it inside the option.
		switch rest[n] {
		case "-k", "--kill-after", "-s", "--signal":
			n++
		}
		n++
	}
	// Consume the DURATION only when it looks like one. Skipping a word that
	// is not a duration would step PAST the real command word and resolve the
	// reader to one of its arguments, which is how a scan finds nothing and
	// reports it as clean.
	if n < len(rest) && durationWord.MatchString(rest[n]) {
		n++
	}
	return n, true
}

// durationWord is timeout's DURATION operand: a number with an optional unit.
var durationWord = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[smhd]?$`)

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// exactSecretNames are basenames that are a credential by their name alone.
var exactSecretNames = map[string]bool{
	".db_connection": true, ".pgpass": true, ".netrc": true, ".my.cnf": true,
	".npmrc": true, ".pypirc": true, ".htpasswd": true, ".dockercfg": true,
	".git-credentials": true, ".env": true, "credentials": true,
	"kubeconfig": true, ".pgservicefile": true, ".pg_service.conf": true,
	"secring.gpg": true,
}

var secretPrefixes = []string{".env.", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "credentials.", "service-account"}

var secretSuffixes = []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".kubeconfig", ".ovpn", "_ed25519", "_rsa", "_ecdsa", ".asc"}

// notSecretSuffixes win over every rule above. A template is published on
// purpose, and a public key is public — refusing to read either would train a
// session to reach for the override.
var notSecretSuffixes = []string{".example", ".sample", ".template", ".dist", ".pub", ".lock"}

// codeSuffixes are where the words "secret" and "credential" name a subject
// rather than hold one. internal/secret/secret.go is source, not a credential.
var codeSuffixes = []string{
	".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".rs", ".java", ".rb", ".c",
	".h", ".cc", ".cpp", ".cs", ".php", ".swift", ".kt", ".scala", ".md",
	".rst", ".proto", ".sql", ".sh", ".bash", ".tf", ".html",
}

// subjectWords are the words that name a credential when one of them IS a
// segment of the basename. `app-secret` and `secrets.yaml` are files; the same
// letters inside a longer identifier are not, and `secretread`, an assignment id
// and a match pattern were all denied for carrying them as a substring.
//
// Equality per segment, not a prefix: `secretread` starts with one of these and
// is a package. The cost of that line is a file named `secret1`, which no rule
// here catches and no rule here caught before.
var subjectWords = map[string]bool{
	"secret": true, "secrets": true, "credential": true, "credentials": true,
}

// namesTheSubject reports whether a basename carries a subject word as a whole
// segment, splitting on the punctuation that separates words inside a filename.
func namesTheSubject(base string) bool {
	for _, seg := range strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	}) {
		if subjectWords[seg] {
			return true
		}
	}
	return false
}

// IsSecretPath reports whether a word names a file whose content is a
// credential. It is a judgement about the name, which is all a PreToolUse hook
// has: the file need not exist, because a command that is about to read one
// that does not exist yet is the same command.
func IsSecretPath(word string) bool {
	return secretByName(word) || secretByGlob(word)
}

// basename returns the lowered basename to classify, or "" when the word is not
// a candidate at all.
func basename(word string) string {
	if word == "" || strings.ContainsAny(word, " \t\n") {
		return ""
	}
	if strings.HasPrefix(word, "-") {
		return ""
	}
	base := strings.ToLower(path.Base(strings.TrimSuffix(word, "/")))
	if base == "." || base == ".." {
		return ""
	}
	return base
}

// secretByName classifies a word by the name it spells out, with no shell
// expansion in between. This is the half that is as true of a fragment carved
// out of another language as it is of a shell word.
func secretByName(word string) bool {
	base := basename(word)
	if base == "" {
		return false
	}
	for _, s := range notSecretSuffixes {
		if strings.HasSuffix(base, s) {
			return false
		}
	}
	if exactSecretNames[base] {
		return true
	}
	for _, p := range secretPrefixes {
		if strings.HasPrefix(base, p) {
			return true
		}
	}
	for _, s := range secretSuffixes {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	if namesTheSubject(base) {
		for _, s := range codeSuffixes {
			if strings.HasSuffix(base, s) {
				return false
			}
		}
		return true
	}
	// A path that names the file by its directory rather than its basename.
	lower := strings.ToLower(word)
	return strings.HasSuffix(lower, ".docker/config.json") || strings.HasSuffix(lower, ".aws/config")
}

// secretByGlob reports whether a word is a pattern the SHELL will expand onto a
// credential's name. `cat .db_conn*` reads the same bytes as naming the file,
// so the stem before the wildcard is tested as a prefix of the names above.
//
// It applies to shell words only. A fragment carved out of another language is
// not something the shell expands, and taking its stem is how `.*` — in a sed
// script, a regex, a series matcher — came to match every dotted name in
// exactSecretNames and deny commands that open no file at all.
func secretByGlob(word string) bool {
	base := basename(word)
	if base == "" {
		return false
	}
	i := strings.IndexAny(base, "*?")
	if i <= 0 {
		return false
	}
	stem := base[:i]
	for name := range exactSecretNames {
		if strings.HasPrefix(name, stem) {
			return true
		}
	}
	for _, p := range secretPrefixes {
		if strings.HasPrefix(p, stem) || strings.HasPrefix(stem, p) {
			return true
		}
	}
	return false
}

// secretsInWord returns the credential paths a single word names.
//
// A word is scanned as a whole first. Where that finds nothing and the word
// carries no whitespace, it is split at the punctuation that wraps a path
// inside another language — `python3 -c "print(open('.db_connection').read())"`
// is one shell word, and the path inside it is read exactly as if it had been
// an argument.
//
// The whitespace condition is what keeps this off prose. A commit message or a
// document that says `cat .db_connection prints it` is a word with spaces in
// it, so it is never fragmented and never matches, while a one-liner that
// actually opens the file is not.
func secretsInWord(word string) []string {
	if secretByGlob(word) {
		return []string{word}
	}
	return namesInWord(word)
}

// namesInWord is secretsInWord without the reading that a shell expands the
// word: the credential names the word spells out, whole or wrapped.
func namesInWord(word string) []string {
	if secretByName(word) {
		return []string{word}
	}
	if word == "" || strings.ContainsAny(word, " \t\n") {
		return nil
	}
	var out []string
	for _, f := range strings.FieldsFunc(word, func(r rune) bool {
		switch r {
		// `|` is deliberately absent. shellsplit has already cut the command at
		// every unquoted pipe, so one that survives into a word is quoted — it is
		// regex alternation, and splitting on it turns `credential\|secret` into
		// the word `secret`, which exactSecretNames closes on purpose.
		case '(', ')', '\'', '"', '`', ',', '=', ';', '{', '}', '[', ']', '<', '>', '$':
			return true
		}
		return false
	}) {
		if secretByName(f) {
			out = append(out, f)
		}
	}
	return out
}

// grepReaders and sed open a word as it is spelled: a quoted * in one is no glob,
// whichever operand it is, unless an option globs its value.
var grepReaders = map[string]bool{"grep": true, "egrep": true, "fgrep": true}

// globValue reports whether the option before a word may glob it: a long option
// with no glued value (`--include '.env*'`), or the -g of the Bash tool's grep.
func globValue(option string) bool {
	if option == "--" {
		return false
	}
	return strings.HasPrefix(option, "--") && !strings.Contains(option, "=") ||
		strings.HasPrefix(option, "-") && strings.HasSuffix(option, "g")
}

// spelled reports whether reader opens or matches args[ai] as it is written.
func spelled(c *shellsplit.Command, reader string, args []string, ai int) bool {
	return (grepReaders[reader] || reader == "sed") && !expands(c, args[ai]) &&
		(ai == 0 || !globValue(args[ai-1]))
}

// ScanPath classifies a path a tool reads directly, with no shell in between —
// Read's file_path, Grep's path. There is no command word to exonerate it, so
// naming a credential is the whole finding.
func ScanPath(p string) []Finding {
	if !IsSecretPath(p) {
		return nil
	}
	return []Finding{{Path: p}}
}

// ScanBash returns every credential-bearing path the command would print.
//
// What is not a finding: a path captured into a variable
// (`URL="$(tail -1 .db_connection)"`), a path consumed by `source`, and a path
// handed to a command that does not emit content. What is: everything else,
// including shapes this scan cannot classify.
//
// Heredoc bodies are data and are never scanned — shellsplit already separates
// them from the words — so writing a document that discusses a credential file
// by name is silent, which is what makes the guard survivable.
func ScanBash(command string) []Finding {
	return shellsplit.Each(command, func(cmds []*shellsplit.Command) []Finding {
		var out []Finding
		// A variable that holds a credential's VALUE, read by a substitution. Using
		// it is the idiom; printing it is the leak.
		holdsValue := map[string]bool{}
		// A variable that holds a credential's PATH, assigned literally. It carries
		// the path default rather than the value default: `F=.db_connection; cat
		// "$F"` opens the file exactly as `cat .db_connection` does.
		holdsPath := map[string]bool{}

		for _, c := range cmds {
			words, argc := commandFirst(c)
			if len(words) == 0 {
				continue
			}

			// A word's text is a heredoc's body where its substitution has one, so
			// what the substitution also runs is read from where the lexer keeps it.
			ran := secretInCommands(c.Runs)

			// Leading NAME=value assignments. One whose value reads a credential is
			// the safe capture form; remember the name so a later print of it in the
			// same command line is still caught.
			i := 0
			for ; i < len(words); i++ {
				if !assignment.MatchString(words[i]) {
					break
				}
				name, value, _ := strings.Cut(words[i], "=")
				switch {
				case ran != "" || containsSecretRead(value):
					holdsValue[name] = true
				case IsSecretPath(value):
					holdsPath[name] = true
				}
			}

			// Everything from here is an ordinary command. Wrappers do not change
			// what it does with its arguments.
			i = pastWrappers(words, i)
			if i >= len(words) {
				continue
			}
			reader := path.Base(words[i])
			args := words[i+1:]

			if consumers[reader] {
				continue
			}

			// A variable holding a credential, printed back out. The capture was
			// safe; handing it to a printer is the same leak one step later.
			consumed := consumedFlags[reader]
			for ai, a := range args {
				for name := range holdsValue {
					if printers[reader] && names(a, name) {
						out = append(out, Finding{Path: "$" + name, Reader: reader, Value: true})
					}
				}
				for name := range holdsPath {
					if !safeReaders[reader] && names(a, name) {
						out = append(out, Finding{Path: "$" + name, Reader: reader, Held: true})
					}
				}
				// An option operand the reader consumes rather than prints. A stdin
				// operand is never one, wherever it was written.
				if ai > 0 && consumed[args[ai-1]] && i+1+ai < argc {
					continue
				}
				found := secretsInWord(a)
				// A name stays read: a grep may walk without saying so, as a directory
				// operand does under the Bash tool's grep, and `grep secret .` prints
				// a .env's lines, while a sed script can name a file it reads.
				if spelled(c, reader, args, ai) {
					found = namesInWord(a)
				}
				if len(found) > 0 {
					// A path, however it is spelled: the command opens the file
					// itself. Only the enumerated non-emitters are exonerated.
					if !safeReaders[reader] {
						for _, f := range found {
							out = append(out, Finding{Path: f, Reader: reader})
						}
					}
					continue
				}
				if containsSecretRead(a) && printers[reader] {
					out = append(out, Finding{Path: secretInside(a), Reader: reader, Substituted: true})
				}
			}
			if ran != "" && printers[reader] {
				out = append(out, Finding{Path: ran, Reader: reader, Substituted: true})
			}
		}
		return dedupe(out)
	})
}

// pastWrappers returns the index of the command word at or after i.
func pastWrappers(words []string, i int) int {
	for i < len(words) {
		w := path.Base(words[i])
		if wrappers[w] {
			i++
			continue
		}
		if n, ok := prefixOperands(w, words[i:]); ok {
			i += n
			continue
		}
		break
	}
	return i
}

// expands reports whether the word is written anywhere in the command with an
// unquoted expansion character.
func expands(c *shellsplit.Command, a string) bool {
	for _, i := range c.Globs {
		if c.Words[i] == a {
			return true
		}
	}
	return false
}

// commandFirst returns a command's words with its stdin operands moved after
// the rest, and how many words precede them. `<wc cat .pgpass` runs cat, not
// wc, and the file it reads on stdin stays an operand the command opens.
func commandFirst(c *shellsplit.Command) ([]string, int) {
	if len(c.Stdin) == 0 {
		return c.Words, len(c.Words)
	}
	stdin := map[int]bool{}
	for _, i := range c.Stdin {
		stdin[i] = true
	}
	var head, tail []string
	for i, w := range c.Words {
		if stdin[i] {
			tail = append(tail, w)
		} else {
			head = append(head, w)
		}
	}
	return append(head, tail...), len(head)
}

// containsSecretRead reports whether a word embeds a command substitution that
// reads a credential — the `$(tail -1 .db_connection)` inside an assignment.
func containsSecretRead(word string) bool {
	return secretInside(word) != ""
}

// secretInside returns the credential path a command substitution in the word
// reads, or "".
//
// Each substitution is lexed as the shell will run it, and its words are
// classified as a bare argument is, then searched as text: a quoted sed
// script, grep pattern, `sh -c` or `python3 -c` program or a path with a space
// is one word the shell hands over whole. Text outside every substitution is
// searched where it may be code the lexer did not read: past a ) that closed one
// early, at a case arm or inside ${…}, or between backquotes that nest. So is
// text glued to a substitution, whose output may split it into a word of its own.
func secretInside(word string) string {
	if !strings.Contains(word, "$(") && !strings.Contains(word, "`") {
		return ""
	}
	var outside strings.Builder
	for i := 0; i < len(word); {
		switch {
		case strings.HasPrefix(word[i:], "$("):
			cmds, next := shellsplit.Substitution(word, i)
			if f := secretInCommands(cmds); f != "" {
				return f
			}
			// Arithmetic runs nothing itself, but a substitution inside it runs.
			if cmds == nil && strings.HasPrefix(word[i:], "$((") {
				if f := secretInside(word[i+3 : next]); f != "" {
					return f
				}
			}
			outside.WriteString(substituted)
			i = next
		case word[i] == '`':
			// The lexer does not group a backquoted command, so each pair of
			// backquotes encloses one.
			body, tail, _ := strings.Cut(word[i+1:], "`")
			for _, cmds := range shellsplit.Readings(body) {
				if f := secretInCommands(cmds); f != "" {
					return f
				}
			}
			outside.WriteString(substituted)
			i = len(word) - len(tail)
		default:
			outside.WriteByte(word[i])
			i++
		}
	}
	text := outside.String()
	if strings.Contains(text, ")") || strings.Count(word, "`") > 2 {
		return secretInText(strings.ReplaceAll(text, substituted, " "))
	}
	for _, tok := range strings.Fields(text) {
		if strings.Contains(tok, substituted) {
			if f := secretInText(strings.ReplaceAll(tok, substituted, " ")); f != "" {
				return f
			}
		}
	}
	return ""
}

// substituted marks where a substitution stood in a word's outside text.
const substituted = "\x00"

// secretInCommands returns the first credential path the commands' words name,
// including one read by a substitution nested in a word. A grep's words are read
// for names alone, since grep runs no text. A sed script is searched in full: its
// e command runs the shell.
func secretInCommands(cmds []*shellsplit.Command) string {
	for _, c := range cmds {
		words, _ := commandFirst(c)
		at := 0
		for at < len(words) && assignment.MatchString(words[at]) {
			at++
		}
		at = pastWrappers(words, at)
		grep := at < len(words) && grepReaders[path.Base(words[at])]
		for wi, w := range words {
			if _, v, ok := strings.Cut(w, "="); grep && ok && strings.HasPrefix(w, "--") && secretByGlob(v) {
				return v
			}
			if grep && wi > at && !strings.HasPrefix(w, "-") && spelled(c, path.Base(words[at]), words[at+1:], wi-at-1) {
				if found := namesInWord(w); len(found) > 0 {
					return found[0]
				}
				if f := nameInText(w); f != "" {
					return f
				}
			} else {
				if found := secretsInWord(w); len(found) > 0 {
					return found[0]
				}
				if f := secretInText(w); f != "" {
					return f
				}
			}
			if f := secretInside(w); f != "" {
				return f
			}
		}
		if f := secretInCommands(c.Runs); f != "" {
			return f
		}
	}
	return ""
}

// secretInText returns a credential path named inside a word's text, or "".
//
// The word is split at whitespace into tokens, and each token at the
// punctuation that wraps a path. A fragment is a credential by name, or by a
// glob stem: the program the text runs may glob it, as `sh -c 'cat .db_conn*'`
// and `find -name '.env*'` do. Two fragments are not:
//   - a subject word that is not itself a credential's name, the `credential`
//     in `grep 'a credential file'`;
//   - a stem of dots in a fragment ending in a backslash, the `.*\` the sed
//     script `s/\(.*\)/\1/` splits into, which is a regex and not a pattern
//     any program expands. Any other `.*` stays a glob: `sh -c 'cat .*;'`
//     prints every dotfile.
func secretInText(word string) string {
	if f := nameInText(word); f != "" {
		return f
	}
	for _, tok := range strings.Fields(word) {
		// A glob is cut at quotes, parentheses, backquotes and the operators a
		// program's shell ends a word at. Not at >, so a stem keeps it: `>.*` in
		// a sed script names no file.
		for _, f := range strings.FieldsFunc(tok, func(r rune) bool {
			switch r {
			case '(', ')', '`', '"', '\'', '<', '|', '&', ';':
				return true
			}
			return false
		}) {
			if secretByGlob(f) && !regexDots(f) {
				return f
			}
		}
	}
	return ""
}

// nameInText returns a credential's name inside a word's text, or "". A subject
// word that is not itself a credential's name is none.
func nameInText(word string) string {
	for _, tok := range strings.Fields(word) {
		for _, f := range strings.FieldsFunc(tok, wrapsAPath) {
			lower := strings.ToLower(f)
			if subjectWords[lower] && !exactSecretNames[lower] {
				continue
			}
			if secretByName(f) {
				return f
			}
		}
	}
	return ""
}

// wrapsAPath reports whether a rune is punctuation that wraps a path inside
// another language's text.
func wrapsAPath(r rune) bool {
	switch r {
	case '(', ')', '\'', '"', '`', ',', '=', ';', '{', '}', '[', ']', '<', '>', '$', '|', '&':
		return true
	}
	return false
}

// regexDots reports whether a fragment is a regex's dot-star rather than a
// glob: its stem is nothing but dots and it ends in the backslash of an
// escaped group, as `\(.*\)` splits into. A backslash before a letter is
// dropped by the shell, so `.*\n` is the glob `.*n`.
func regexDots(f string) bool {
	base := basename(f)
	i := strings.IndexAny(base, "*?")
	return i > 0 && strings.Trim(base[:i], ".") == "" && strings.HasSuffix(f, `\`)
}

// names reports whether an argument references the shell variable.
func names(arg, v string) bool {
	return strings.Contains(arg, "$"+v+" ") || strings.HasSuffix(arg, "$"+v) ||
		arg == "$"+v || strings.Contains(arg, "${"+v+"}")
}

func dedupe(in []Finding) []Finding {
	type key struct{ path, reader string }
	seen := map[key]bool{}
	var out []Finding
	for _, f := range in {
		k := key{f.Path, f.Reader}
		if f.Path == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}

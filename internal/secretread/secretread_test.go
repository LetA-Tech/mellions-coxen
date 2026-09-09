// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A key-name parse is still a content read when the file can contain a bare URI.
func TestScanBash_DangerousCredentialReads(t *testing.T) {
	for _, cmd := range []string{
		`awk '/^[[:space:]]*#/ {print NR": "$0}' .db_connection`,
		`awk '!/^[[:space:]]*#/ && NF {split($0,a,"="); print NR": "a[1]"=<withheld>"}' .db_connection`,
		`cd /home/you/workspace/payments-api/migrations/postgres; awk '{print $0}' .db_connection`,
	} {
		got := ScanBash(cmd)
		if len(got) == 0 {
			t.Fatalf("ScanBash(%q) returned no finding — this is the leak", cmd)
		}
		if got[0].Path != ".db_connection" {
			t.Errorf("ScanBash(%q) path = %q, want .db_connection", cmd, got[0].Path)
		}
	}
}

func TestScanBash(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		deny bool
	}{
		// Reads that reach the transcript.
		{"cat", `cat .db_connection`, true},
		{"bare tail prints", `tail -1 .db_connection`, true},
		{"head an env file", `head -20 /opt/deploy/payments/.env`, true},
		{"grep a key out", `grep POSTGRES_MIN_IDLE_CONNS .env`, true},
		{"sed redaction idiom", `sed -E 's/=.*/=<redacted>/' .db_connection`, true},
		{"private key", `cat ~/.ssh/id_ed25519`, true},
		{"pem", `openssl rsa -in server.key -text`, true},
		{"sudo does not launder", `sudo cat /root/.pgpass`, true},
		{"redirect is still a read", `cat .db_connection > /tmp/x`, true},
		{"secret yaml", `cat k8s/secrets.yaml`, true},
		{"aws credentials", `cat ~/.aws/credentials`, true},

		// The idiom the guard steers toward, and ordinary use of the value.
		{"capture into a variable", `DATABASE_URL="$(tail -1 .db_connection)"`, false},
		{"capture then use", "URL=\"$(tail -1 .db_connection)\"\npsql \"$URL\" -c 'select 1'", false},
		{"substitution as an argument", `psql "$(tail -1 .db_connection)" -c 'select 1'`, false},
		{"source", `source /opt/deploy/payments/.env`, false},
		{"dot form", `. ./.env`, false},

		// The follow-on leak: captured safely, then printed.
		{"echo the captured variable", "URL=\"$(tail -1 .db_connection)\"\necho \"$URL\"", true},
		{"printf the captured variable", "U=\"$(cat .pgpass)\"\nprintf '%s' \"${U}\"", true},

		// A build tool's argument is a package path. The directory holding
		// this guard is named for what it guards, so its own test command was
		// the first false denial.
		{"go test on this package", `go test ./internal/secretread/`, false},
		{"go test on a secrets package", `go test ./internal/secrets/... -run TestX`, false},
		{"make a target named for it", `make secret-check`, false},
		{"cat is still cat", `cat ./internal/secretread/.db_connection`, true},

		// Metadata about the file is not its content.
		{"wc", `wc -c .db_connection`, false},
		{"wc lines", `wc -l migrations/postgres/.db_connection`, false},
		{"stat", `stat -c %a .env`, false},
		{"ls", `ls -la ~/.ssh/id_ed25519`, false},
		{"chmod", `chmod 0600 .db_connection`, false},
		{"test", `test -f .db_connection && echo present`, false},
		{"cp", `cp .env .env.bak`, false},

		// Published-on-purpose files, and source that merely discusses secrets.
		{"env example", `cat .env.example`, false},
		{"deploy env example", `sed -n '120,130p' deploy/.env.example`, false},
		{"public key", `cat ~/.ssh/id_ed25519.pub`, false},
		{"source file named secret", `cat internal/secret/secret.go`, false},
		{"test file", `cat internal/partner/partner_test.go`, false},
		{"a doc about credentials", `cat docs/operator-database-credential-custody.md`, false},

		// Prose that names a credential file is not a command that reads one.
		{"a sentence mentioning the file", `printf -- '- [.db_connection is a bare URI](x.md)\n' >> MEMORY.md`, false},
		{"unrelated", `go test ./...`, false},
		{"empty", ``, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanBash(tt.cmd)
			if tt.deny && len(got) == 0 {
				t.Errorf("ScanBash(%q) allowed; want denied", tt.cmd)
			}
			if !tt.deny && len(got) != 0 {
				t.Errorf("ScanBash(%q) denied on %+v; want allowed", tt.cmd, got)
			}
		})
	}
}

// TestScanBash_HeredocBodyIsData guards the property that makes the check
// survivable: a session writes documents that name credential files by path,
// and denying those would train it to reach for the override.
func TestScanBash_HeredocBodyIsData(t *testing.T) {
	cmd := "cat > memory/note.md <<'EOF'\nThe real .db_connection is a bare URI, so `cat .db_connection` prints it.\nUse `wc -c` instead.\nEOF"
	if got := ScanBash(cmd); len(got) != 0 {
		t.Errorf("a heredoc discussing .db_connection was denied: %+v", got)
	}
}

// TestScanBash_Bypasses collects the shapes an adversarial pass over the first
// implementation found it allowed. Each is a spelling of "open this file" that
// the whole-word path match did not see.
func TestScanBash_Bypasses(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  string
	}{
		{"python one-liner", `python3 -c "print(open('.db_connection').read())"`},
		{"perl one-liner", `perl -e 'open(F,".pgpass");print<F>'`},
		{"node one-liner", `node -e "console.log(require('fs').readFileSync('.env','utf8'))"`},
		{"glob stem", `cat .db_conn*`},
		{"glob on env", `cat .env*`},
		{"path held in a variable", `F=.db_connection; cat "$F"`},
		{"path in a variable, awk", `F=.db_connection
awk '{print}' "$F"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanBash(tt.cmd); len(got) == 0 {
				t.Errorf("ScanBash(%q) allowed a credential read", tt.cmd)
			}
		})
	}
}

// TestScanBash_ProseIsNotACommand pins the property the fragment scan must not
// cost: a message or a document that names a credential file is text, and
// denying it would put the guard in the way of writing this down.
func TestScanBash_ProseIsNotACommand(t *testing.T) {
	for _, cmd := range []string{
		`git commit -m "fix .env handling in the deploy path"`,
		`git commit -m "cat .db_connection prints the password"`,
		`gh issue comment 338 --body "the file at migrations/postgres/.db_connection is a bare URI"`,
		`echo "documented in .env.example"`,
	} {
		if got := ScanBash(cmd); len(got) != 0 {
			t.Errorf("ScanBash(%q) denied prose: %+v", cmd, got)
		}
	}
}

func TestIsSecretPath(t *testing.T) {
	secret := []string{
		".db_connection", "migrations/postgres/.db_connection", ".env",
		"/opt/deploy/payments/.env", ".env.production", "~/.ssh/id_ed25519",
		"server.pem", "tls.key", ".pgpass", ".netrc", "k8s/secrets.yaml",
		"~/.aws/credentials", "cluster.kubeconfig", "service-account.json",
		".git-credentials", "~/.docker/config.json",
	}
	for _, p := range secret {
		if !IsSecretPath(p) {
			t.Errorf("IsSecretPath(%q) = false, want true", p)
		}
	}
	ordinary := []string{
		".env.example", ".env.sample", "deploy/.env.example", "id_ed25519.pub",
		"internal/secret/secret.go", "secrets_test.go", "credential-custody.md",
		"go.mod", "README.md", "main.go", "", "-", "--body-file", ".", "..",
		"package-lock.json",
	}
	for _, p := range ordinary {
		if IsSecretPath(p) {
			t.Errorf("IsSecretPath(%q) = true, want false", p)
		}
	}
}

func TestScanPath(t *testing.T) {
	if got := ScanPath("/home/you/workspace/payments-api/migrations/postgres/.db_connection"); len(got) != 1 {
		t.Fatalf("ScanPath on a credential returned %d findings, want 1", len(got))
	}
	if got := ScanPath("cmd/mellions/main.go"); len(got) != 0 {
		t.Errorf("ScanPath on source returned %+v, want none", got)
	}
}

// A prefix command carries operands before the real command word. Without
// accounting for them the reader resolves to `timeout`'s duration, and a guard
// that has to be worked around is one that gets turned off. The Mellions cases
// are the opposite control: this first-party CLI has file-reading subcommands,
// so resolving through the prefix must not exonerate it.
func TestScanBash_PrefixOperandsAndFirstPartyCLI(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		deny bool
	}{
		{"timeout reaches the build tool", `timeout 600 go test -list '.*' ./internal/guard/...`, false},
		{"timeout with options", `timeout -k 5s --foreground 600 go test -list '.*' ./...`, false},
		{"timeout with a float duration", `timeout 1.5m go build ./...`, false},
		// `mellions` is NOT exonerated. An independent read built the binary and
		// watched this print a fixture's DATABASE_URL and AWS_SECRET_ACCESS_KEY
		// back out, so the shape is locked closed here rather than left to the
		// next reader to rediscover.
		{"a first-party CLI that reads a named file", `mellions report write -id d -file .env`, true},
		{"and behind a prefix", `timeout 60 mellions report write -id d -file .git-credentials`, true},

		// The same command shapes must still deny a real credential read.
		{"timeout does not launder cat", `timeout 5 cat .env`, true},
		{"timeout does not launder tail", `timeout -k 1s 5 tail -1 .db_connection`, true},
		{"mellions does not launder a pipe into cat", `mellions assign get x; cat .pgpass`, true},
		// A malformed timeout does not run at all — `timeout: invalid time
		// interval` — but the scan must not step past the command word on the
		// way to finding that out, or it reports a clean read of nothing.
		{"no duration at all", `timeout cat .env`, true},
		{"a separator where the duration should be", `timeout -- cat .env`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanBash(tt.cmd)
			if tt.deny && len(got) == 0 {
				t.Fatalf("ScanBash(%q) returned no finding — a real credential read went unguarded", tt.cmd)
			}
			if !tt.deny && len(got) > 0 {
				t.Fatalf("ScanBash(%q) = %+v, want no finding — this denies a correct command", tt.cmd, got)
			}
		})
	}
}

// The reader named in the refusal must be the command that would do the
// printing. `timeout 5 cat .env` denied before this change, but blamed
// `timeout`, so the message told the reader to stop using the wrong tool.
func TestScanBash_TimeoutBlamesTheRealReader(t *testing.T) {
	got := ScanBash(`timeout 5 cat .env`)
	if len(got) != 1 {
		t.Fatalf("ScanBash = %+v, want exactly one finding", got)
	}
	if got[0].Reader != "cat" {
		t.Errorf("Reader = %q, want cat — the refusal must name what prints", got[0].Reader)
	}
}

// TestScanBash_FalseDenials collects commands that read no credential and were
// denied anyway. Each was measured in one session's real work; the last one is
// the command that opened the lane for this fix, refused because the assignment
// id named the package.
//
// A denial is not free. It costs the whole tool call — a heredoc paired with the
// publish that consumes it never runs — and a guard that has to be worked around
// is one that gets turned off, which this package already says of the go/make
// case.
func TestScanBash_FalseDenials(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  string
	}{
		// ssh consumes the identity file. It is handed to the crypto and
		// written nowhere.
		{"ssh identity file", `ssh -fN -i ~/.ssh/obsdebug_ed25519 -L 9428:127.0.0.1:9428 obsdebug@159.203.55.241`},
		{"scp identity file", `scp -i ~/.ssh/id_ed25519 build.tar deploy@host:/srv`},

		// `.*` is a regex, not a glob. Split out of a larger word its stem is
		// ".", which prefixes every dotted name in exactSecretNames.
		{"sed script with a wildcard", `git blame -L 110,122 -- goal_pace.go | sed 's/(\(.\{1,22\}\).*[0-9]\{4\})/(\1)/'`},
		{"a series matcher", `curl -sf http://127.0.0.1:8428/api/v1/series --data-urlencode 'match[]={__name__=~".*ratio_not_representable.*"}'`},

		// The two words appear inside an identifier, not a filename.
		{"the words inside a match pattern", `grep -ln -i 'credential\|secret' hooks/*.sh`},
		{"an assignment id naming the package", `mellions assign open cx-secretread-false-denials -repo mellions-coxen`},
		// `git` is deliberately not a safe reader — `git show` prints file
		// content — so this one turns on the name, not on the command word.
		{"git add on the package directory", `git add internal/secretread/`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanBash(tt.cmd); len(got) != 0 {
				t.Errorf("ScanBash(%q) denied a command that reads no credential: %+v", tt.cmd, got)
			}
		})
	}
}

// TestScanBash_NarrowingDidNotWiden is the other direction, and the one that
// decides whether the change above is safe. A revert arm shows what the fix now
// admits; only these show what it must still catch. Each is a shape adjacent to
// something newly allowed.
func TestScanBash_NarrowingDidNotWiden(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  string
	}{
		// The operand is exonerated, never the reader.
		{"ssh runs a printer on the far side", `ssh host cat .env`},
		{"ssh with an identity AND a remote read", `ssh -i ~/.ssh/id_ed25519 host cat /opt/app/.env`},

		// A glob is still a glob when the SHELL is the one expanding it. `.*`
		// as a whole word reads every dotfile, .env among them.
		{"a bare dot-star word", `cat .*`},
		{"the stem that started this", `cat .db_conn*`},
		{"a glob inside a substitution", `echo "$(cat .db_conn*)"`},

		// The subject word as a whole SEGMENT of the name, with or without an
		// extension. A reviewer who never saw the diff predicted the dotless
		// half of this family as the place a narrowing would go wrong, and the
		// first attempt did: `app-secret` was caught before and was not after.
		{"a file named secret", `cat secret`},
		{"a file named secrets", `tail -5 secrets`},
		{"a dotless hyphenated name", `cat app-secret`},
		{"the singular, dotless", `head -1 prod-credential`},
		{"an underscore separator", `cat app_secrets`},
		{"still caught with an extension", `cat my-secrets.yaml`},
		{"and in a directory", `head -1 deploy/app-credentials.json`},

		// A flag letter means what the READER says it means. `-i` is an
		// identity file to ssh and an in-place edit to sed, so the exoneration
		// is keyed by reader and does not travel with the letter.
		{"the same flag on a printer", `sed -i 's/x/y/' .env`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanBash(tt.cmd); len(got) == 0 {
				t.Errorf("ScanBash(%q) allowed a credential read", tt.cmd)
			}
		})
	}
}

// TestScanBash_ShapesThatReachedTheBytes is the set from mellions-coxen#26:
// each was allowed on dev at bcc0059, and each reaches a credential's content.
// They are separated by mechanism because they are three defects, not one — a
// redirect operand lexed as the command word, a shell operand the whitespace
// rule silences, and a safeReader whose exoneration is a claim about a
// destination the command line contradicts.
func TestScanBash_ShapesThatReachedTheBytes(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  string
		path string
	}{
		// The redirect operand is not an argument and not the command word.
		// Only the leading spelling was open on dev: with the redirect after
		// the command the operand was still an argument, so the scan caught
		// it. It is here because the lexer now takes the operand OUT of Words,
		// which would have turned a caught shape into an allowed one had the
		// scan not read In — the arm that neutralises In reds all three.
		{"stdin redirect before the command word", `< .env grep .`, ".env"},
		{"stdin redirect after it", `grep . < .env`, ".env"},
		{"an explicit descriptor", `grep . 0< .db_connection`, ".db_connection"},

		// A shell's -c operand is a command line, not a word of prose.
		{"bash -c", `bash -c 'cat .env'`, ".env"},
		{"sh -c", `sh -c "cat .db_connection"`, ".db_connection"},
		{"bundled option letters", `bash -lc 'tail -1 .env'`, ".env"},
		{"a shell behind a wrapper", `sudo bash -c 'cat /root/.pgpass'`, "/root/.pgpass"},
		{"a shell inside a shell", `bash -c "sh -c 'cat .env'"`, ".env"},
		{"the redirect shape nested in one", `bash -c '< .env grep .'`, ".env"},

		// A destination that prints withdraws the safeReader exoneration.
		{"cp to stdout", `cp .env /dev/stdout`, ".env"},
		{"cp to a numbered descriptor", `cp .db_connection /dev/fd/1`, ".db_connection"},
		{"install to the terminal", `install ~/.ssh/id_ed25519 /dev/tty`, "~/.ssh/id_ed25519"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanBash(tt.cmd)
			if len(got) == 0 {
				t.Fatalf("ScanBash(%q) allowed a credential read", tt.cmd)
			}
			for _, f := range got {
				if f.Path == tt.path {
					return
				}
			}
			t.Errorf("ScanBash(%q) = %+v, wanted a finding on %q", tt.cmd, got, tt.path)
		})
	}
}

// TestScanBash_TheThreeShapesDidNotOverRefuse is the other direction. Each of
// the three fixes above refuses more than the code did, so each needs the
// adjacent shape that must stay silent: a redirect that feeds a non-printer, a
// shell operand that reads nothing, and a safeReader whose destination is a
// file.
func TestScanBash_TheThreeShapesDidNotOverRefuse(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  string
	}{
		// stdin into a command that does not print its input.
		{"wc counts without printing", `wc -l < .env`},
		{"a redirect from an ordinary file", `grep POSTGRES < docker-compose.yml`},
		{"a descriptor duplicate names no file", `grep . <&3`},

		// The shell operand is scanned as a command line, so an innocent one
		// stays innocent.
		{"a build behind bash -c", `bash -c 'go build ./... && make check'`},
		{"a shell with no -c at all", `bash --version`},
		{"a long option is not the -c operand", `bash --norc script.sh`},

		// -c belongs to the reader that declares it: `sort -c` checks order.
		{"a c flag on something that is not a shell", `sort -c notes.txt`},

		// Prose naming the shape is still prose: the reader is git, not a shell.
		{"the issue title in a commit message", `git commit -m "the guard missed bash -c 'cat .env'"`},

		// cp is still exonerated when its destination is a file.
		{"cp to a path", `cp .env /tmp/backup.env`},
		{"cp to the bit bucket", `cp .env /dev/null`},
		{"mv within the tree", `mv deploy/.env deploy/.env.bak`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanBash(tt.cmd); len(got) != 0 {
				t.Errorf("ScanBash(%q) denied a command that reads no credential into the transcript: %+v", tt.cmd, got)
			}
		})
	}
}

// TestScanBash_ShellOperandIsNotPositional closes what an independent read of
// the first commit ran and found: the shell descent read the word after the
// -c-bearing option instead of the first operand, so two characters put the
// script back out of reach. Each case here executes and prints in real bash.
func TestScanBash_ShellOperandIsNotPositional(t *testing.T) {
	for _, cmd := range []string{
		`bash -c -- 'cat .env'`,
		`sh -c -- "cat .db_connection"`,
		`bash -c -x 'cat .env'`,
		`bash -c --norc 'cat .env'`,
		`bash -c 'cat .env' arg0 arg1`,
		`eval 'cat .env'`,
		`eval "tail -1 .db_connection"`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("ScanBash(%q) allowed a credential read", cmd)
		}
	}
	// A heredoc is data to every reader but a shell, for which it is a script.
	heredoc := "bash <<'EOF'\ncat .env\nEOF\n"
	if got := ScanBash(heredoc); len(got) == 0 {
		t.Errorf("ScanBash(a heredoc fed to bash) allowed a credential read")
	}
	// The same body handed to a reader that is not a shell stays data, which
	// is what keeps a pull-request body naming a credential file silent.
	prose := "gh pr create --body-file - <<'EOF'\ncat .env is what the guard refuses\nEOF\n"
	if got := ScanBash(prose); len(got) != 0 {
		t.Errorf("ScanBash(a heredoc fed to gh) denied prose: %+v", got)
	}
}

// TestScanBash_PrefixesDoNotHideTheShell: a prefix that stands in front of the
// real command word must not make the reader resolve to itself or to one of
// its own options, which is how the shell behind it stopped being a shell.
func TestScanBash_PrefixesDoNotHideTheShell(t *testing.T) {
	for _, cmd := range []string{
		`env bash -c 'cat .env'`,
		`env -i bash -c 'cat .env'`,
		`env PATH=/usr/bin sh -c "cat .db_connection"`,
		`timeout 5 env bash -c 'cat .env'`,
		`busybox sh -c 'cat .env'`,
		`env cat .env`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("ScanBash(%q) allowed a credential read", cmd)
		}
	}
	for _, cmd := range []string{
		`env bash -c 'go build ./...'`,
		`env -i HOME=/tmp PATH=/usr/bin wc -l .env`,
		`busybox ls -l .env`,
	} {
		if got := ScanBash(cmd); len(got) != 0 {
			t.Errorf("ScanBash(%q) denied a command that prints no credential: %+v", cmd, got)
		}
	}
}

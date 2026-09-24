// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import (
	"strings"
	"testing"
)

// The scan matches names and never models what a command does with an
// argument, so a denial states the names it matched. A sentence claiming a
// program prints a file is a fact the session reading it cannot check, and on
// `gh -R aws-actions/configure-aws-credentials` or `jq .credentials` it is false.
func TestReasonStatesTheNamesItMatched(t *testing.T) {
	notOnSafeList := "is not on the guard's list of programs that never print a file's content."
	onPrinterList := "is on the guard's list of programs that can print what they are given."
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"a known printer on a credential path", `cat .db_connection`,
			"`cat … .db_connection` — `.db_connection` is named like a credential file, and `cat` " + notOnSafeList},
		{"the guard's own subcommand name", `./bin/mellions secret-check`,
			"`mellions … secret-check` — `secret-check` is named like a credential file, and `mellions` " + notOnSafeList},
		{"a repository name argument", `gh release view v6.2.4 -R aws-actions/configure-aws-credentials --json body`,
			"`gh … aws-actions/configure-aws-credentials` — `aws-actions/configure-aws-credentials` is " +
				"named like a credential file, and `gh` " + notOnSafeList},
		{"a literal path that begins with an expansion", `cat $HOME/.env`,
			"`cat … $HOME/.env` — `$HOME/.env` is named like a credential file, and `cat` " + notOnSafeList},
		{"a variable assigned a credential's path", `F=.db_connection; frob "$F"`,
			"`$F` was assigned a path named like a credential file earlier on this command line, and `frob` " + notOnSafeList},
		{"a variable assigned a substitution naming a credential", `U="$(tail -1 .db_connection)"; echo "$U"`,
			"`$U` was assigned, earlier on this command line, a word containing `$(` or a backtick " +
				"and a credential file's name, and `echo` " + onPrinterList},
		{"a substitution beside a credential name", `echo "$(date) .env"`,
			"an argument to `echo` contains `$(` or a backtick and the credential file name `.env`, and `echo` " + onPrinterList},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanBash(tt.cmd)
			var rs []string
			for _, f := range got {
				rs = append(rs, f.Reason())
			}
			for _, r := range rs {
				if r == tt.want {
					return
				}
			}
			t.Fatalf("ScanBash(%q) gave no finding with reason\n  %s\ngot:\n  %s",
				tt.cmd, tt.want, strings.Join(rs, "\n  "))
		})
	}

	direct := ScanPath("/etc/payments/.env")
	if len(direct) != 1 || direct[0].Reason() !=
		"`/etc/payments/.env` is named like a credential-bearing file, and this tool can return a file's content." {
		t.Errorf("ScanPath(/etc/payments/.env) = %+v", direct)
	}
}

// Every shape here is denied and prints no credential, or is denied on a word
// the scan cannot tell is a file operand. None of their sentences may state
// what a program does with the file.
func TestReasonAssertsNoBehaviour(t *testing.T) {
	for _, cmd := range []string{
		`jq -r .credentials config.json`,
		`grep -rn credentials docs/`,
		`echo hi | tee .env`,
		`sort -o .env input.txt`,
		`grep -c "$(cat .env)" log.txt`,
		`X="$(echo .env)"; echo "$X"`,
		`X="$(cat .env)"; X=foo; echo "$X"`,
		`echo "$(date) .env"`,
		`for s in check-publish-credential-scope; do bash scripts/$s.sh; done`,
	} {
		got := ScanBash(cmd)
		if len(got) == 0 {
			t.Errorf("ScanBash(%q) found nothing, so this case proves nothing about its sentence", cmd)
		}
		for _, f := range got {
			r := f.Reason()
			for _, claim := range []string{"writes", "would", "reads `", "holds a value", "prints the", "substitution"} {
				if strings.Contains(r, claim) {
					t.Errorf("ScanBash(%q) reason claims %q: %s", cmd, claim, r)
				}
			}
		}
	}
}

// A path and the program reading it are one finding however the scan reached
// it, so the count a denial and `mellions secret check` report is the count of
// distinct reads.
func TestFindingsCountDistinctReads(t *testing.T) {
	for cmd, want := range map[string]int{
		`cat .env "$(cat .env)"`:             1,
		`X=.env; X="$(cat .env)"; echo "$X"`: 1,
		`X="$(cat .env)"; X=.env; cat "$X"`:  1,
	} {
		if got := ScanBash(cmd); len(got) != want {
			t.Errorf("ScanBash(%q) = %d findings, want %d: %+v", cmd, len(got), want, got)
		}
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import (
	"strings"
	"testing"
)

// A denial states a program's behaviour only where the scan knows it. Every
// other denial is made on an argument's name, and a sentence claiming that an
// unknown program prints file content is a false fact the session reading it
// cannot check.
func TestReasonClaimsOnlyWhatTheScanKnows(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		definite bool
		want     string
	}{
		{
			"a known printer on a credential path",
			`cat .db_connection`, true,
			"`cat … .db_connection` — `.db_connection` is named like a credential file, " +
				"and `cat` writes a file's content to stdout.",
		},
		{
			"the guard's own subcommand name",
			`./bin/mellions secret-check`, false,
			"`mellions … secret-check` — `secret-check` is named like a credential file. " +
				"`mellions` is not among the programs this guard knows never print a file's " +
				"content, so the denial rests on that name alone; the guard has not " +
				"established that this command opens or prints the file.",
		},
		{
			"a repository name argument",
			`gh release view v6.2.4 -R aws-actions/configure-aws-credentials --json body`, false,
			"`gh … aws-actions/configure-aws-credentials` — " +
				"`aws-actions/configure-aws-credentials` is named like a credential file. " +
				"`gh` is not among the programs this guard knows never print a file's " +
				"content, so the denial rests on that name alone; the guard has not " +
				"established that this command opens or prints the file.",
		},
		{
			"echo prints the name, not the file",
			`echo .env`, false,
			"`echo … .env` — `.env` is named like a credential file. `echo` is not among " +
				"the programs this guard knows never print a file's content, so the denial " +
				"rests on that name alone; the guard has not established that this command " +
				"opens or prints the file.",
		},
		{
			"a literal path that begins with an expansion",
			`cat $HOME/.env`, true,
			"`cat … $HOME/.env` — `$HOME/.env` is named like a credential file, " +
				"and `cat` writes a file's content to stdout.",
		},
		{
			"a variable holding a credential's path",
			`F=.db_connection; frob "$F"`, false,
			"`$F` holds a path named like a credential file, assigned earlier on this " +
				"command line. `frob` is not among the programs this guard knows never print " +
				"a file's content, so the denial rests on that name alone; the guard has not " +
				"established that this command opens or prints the file.",
		},
		{
			"a variable holding a credential's value, printed",
			`U="$(tail -1 .db_connection)"; echo "$U"`, true,
			"`$U` holds a value read from a credential file earlier on this command line, " +
				"and `echo` writes its arguments out.",
		},
		{
			"a substitution reading a credential, printed",
			`echo "$(tail -1 .db_connection)"`, true,
			"a command substitution reads `.db_connection`, and `echo` writes what it " +
				"returns to stdout.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanBash(tt.cmd)
			var match *Finding
			for i := range got {
				if got[i].Reason() == tt.want {
					match = &got[i]
				}
			}
			if match == nil {
				var rs []string
				for _, f := range got {
					rs = append(rs, f.Reason())
				}
				t.Fatalf("ScanBash(%q) gave no finding with reason\n  %s\ngot:\n  %s",
					tt.cmd, tt.want, strings.Join(rs, "\n  "))
			}
			if match.Definite() != tt.definite {
				t.Errorf("Definite() = %v, want %v", match.Definite(), tt.definite)
			}
		})
	}

	direct := ScanPath("/etc/payments/.env")
	if len(direct) != 1 || !direct[0].Definite() || direct[0].Reason() !=
		"`/etc/payments/.env` is named like a credential-bearing file, and this tool "+
			"prints the content of the file it reads." {
		t.Errorf("ScanPath(/etc/payments/.env) = %+v", direct)
	}
}

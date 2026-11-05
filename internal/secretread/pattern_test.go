// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

func TestScanBash_PatternOperandOpensNoFile(t *testing.T) {
	for _, cmd := range []string{
		`grep -E '^hooks/|^cmd/.*(guard|cite|check)' list.txt`,
		`git ls-tree -r origin/dev --name-only | grep -E '^hooks/|^cmd/.*(guard|cite|check)'`,
		`ls pack/ | sed 's/.*\.//' | sort | uniq -c`,
		`sed -n 's/.*\.//p' list.txt`,
		`sed -e 's/.*\.//' -e 's/x/y/' list.txt`,
		`sed -ne 's/.*\.//p' list.txt`,
		`sed -i 's/.*\.//' list.txt`,
		`grep -rn '.*foo' internal/`,
		`grep -rEn --include='*.go' 'func \(.*\) Close|.*_test' internal/`,
		`grep -n -A3 '.*foo' a.go`,
		`grep -nA3 '.*foo' a.go`,
		`grep -A 3 '.*foo' a.go`,
		`grep -n AWS_SECRETS_MANAGER_NAME internal/config/config.go`,
		`grep -n .env Makefile`,
		`grep -c '.pem' README.md`,
		`grep -- .netrc docs/x.md`,
		`grep credentials a.go`,
		`grep -e .env Makefile`,
		`grep -e .env -e .pem Makefile`,
		`grep -ne .env Makefile`,
		`cat Makefile | grep .env`,
		`sudo grep -n .env Makefile`,
		`timeout 5 grep -n .env Makefile`,
		`egrep .env Makefile`,
	} {
		if got := ScanBash(cmd); len(got) != 0 {
			t.Errorf("%s\n  refused: %+v", cmd, got)
		}
	}
}

func TestScanBash_PatternOperandHidesNoRead(t *testing.T) {
	for _, cmd := range []string{
		// The named file is opened, whatever the pattern is.
		`grep x .env`, `grep -n foo .pgpass`, `grep foo .env*`, `grep -l x .env`,
		`grep -- x .env`, `sed -n p .env`, `sed p .env*`, `sed -e p .env`,
		`sed -ne p .env`, `timeout 5 grep x .env`,
		// An unquoted glob or brace is expanded before grep sees a pattern.
		`grep .* x`, `grep .env* x`, `grep {x,.env} y`, `grep {.env,.pgpass} y`,
		`sed .* x`,
		// A walk reaches files no operand names, so the pattern's name still counts.
		`grep -r .env config.d`, `grep -rn .pem deploy.d`, `grep -R .env config.d`,
		`grep --recursive .env config.d`, `grep --dereference-recursive .env config.d`,
		`grep .env -r config.d`, `grep .env config.d -r`, `grep -nr .env config.d`,
		`grep -rn SECRET .`, `grep -rn AWS_SECRET_ACCESS_KEY .`, `sudo grep -r .env config.d`,
		`egrep -r .env config.d`, `grep -r -e .env config.d`, `grep -r -- .env config.d`,
		// An option outside the grammar stops the reading.
		`grep -d recurse .env config.d`, `grep --directories=recurse .env config.d`,
		`grep --recur .env config.d`, `grep -f .env x`, `grep --file=.env x`,
		`grep --include .env x .`, `grep --include=.env -r x .`, `grep --reg=x .env`,
		`grep --regexp=x .env`, `grep -A .env x`,
		// With -e given, the first operand is a file.
		`grep -e x .env`, `grep -ex .env`, `grep .netrc -e x`, `grep -ne x .env`,
		`sed -e p .env`, `sed .pgpass -e p`, `sed --expression=p .env`, `sed -n -f .env x`,
		// A pattern that runs something.
		`grep "$(cat .pgpass)" x`, "grep `cat .pgpass` x", `grep "$(cat<.pgpass)" x`,
		`grep <(cat .env) x`, `grep <(cat<.env) x`, `grep >(cat<.env) x`,
		// A redirection's file is no operand.
		`grep <.env`, `grep x <.env`, `<.env grep x`,
		// Sed's script names a file it reads.
		`sed 'r/x/.env' y`, `sed '/.env' y`,
		// A variable holding the path.
		`F=.env; grep x "$F"`, `F=.env; grep "$F" x`,
		// Other readers keep every reading.
		`cat '.env*'`, `xargs grep .env`, `find . -name '.env*' -exec cat {} +`,
		`rg -uu .env config.d`, `git grep .env -- config.d`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("%s\n  admitted", cmd)
		}
	}
}

func TestScanBash_TextOutsideACaptureIsText(t *testing.T) {
	for _, cmd := range []string{
		`echo "$(date) .env"`,
		`echo "$(date) (see .env)"`,
		`echo ".env $(date)"`,
	} {
		if got := ScanBash(cmd); len(got) != 0 {
			t.Errorf("%s\n  refused: %+v", cmd, got)
		}
	}
}

// Whatever the lexer makes of a substitution's end, what bash runs stays read.
func TestScanBash_CaptureHidesNoRead(t *testing.T) {
	for _, cmd := range []string{
		"X=\"$( [[ x =~ ( (a) #b) ]]; cat .pgpass )\"; echo \"$X\"",
		"X=\"$( echo ${y:-(a) #}; cat .pgpass )\"; echo \"$X\"",
		"X=\"$( (( (1) + 1 #)); cat .pgpass )\"; echo \"$X\"",
		"X=\"$(echo hi # )\ncat .pgpass\n)\"; echo \"$X\"",
		"echo \"$( echo ${y:-(a) #}; cat .pgpass )\"",
		"echo \"$(echo hi # )\ncat .pgpass\n)\"",
		"X=$(cat <<E\n$(cat .pgpass)\nE\n); echo \"$X\"",
		"echo \"$(cat <<E\n$(cat .pgpass)\nE\n)\"",
		"X=`echo \\`cat .pgpass\\``; echo \"$X\"",
		"echo \"$((  $(cat .pgpass) ))\"",
		"X=\"$(( (1+$(cat .pgpass)) ))\"; echo \"$X\"",
		"X=\"`cat .pgpass`$(date)\"; echo \"$X\"",
		"X=\"$(date)`cat .pgpass`\"; echo \"$X\"",
		`X=$(echo $(cat .pgpass|base64)); echo "$X"`,
		`echo "$(echo "$(cat .pgpass|base64)")"`,
		"X=\"$(case x in a) cat .pgpass;; esac)\"; echo \"$X\"",
		"echo \"$(case x in a) cat .pgpass;; esac)\"",
		"X=\"`cat <<E\n$(cat .pgpass)\nE\n`\"; echo \"$X\"",
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("%s\n  admitted", cmd)
		}
	}
}

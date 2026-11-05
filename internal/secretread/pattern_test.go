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
		`grep -n x '.env*'`,
		`grep -- '.*foo' a.go`,
		`grep -e '.*foo' a.go`,
		`grep -ne '.*foo' -e '.*bar' a.go`,
		`cat a.go | egrep '.*foo'`,
		`sudo grep -n '.*foo' a.go`,
		`timeout 5 grep -n '.*foo' a.go`,
		`grep -nA3 '.*foo' a.go`,
		`grep -A 3 '.*foo' a.go`,
		`grep -n '.*foo' "$f"`,
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
		`O=-r; grep .env $O config.d`, `grep .env "$O" config.d`, "grep .env `o` config.d",
		// A name stays read in a pattern: a grep may walk without saying so.
		`grep secret .`, `grep -n .env .`, `grep -in secret d`, `grep "secret" ./`,
		`K=$(grep secret .); echo "$K"`, `K=$(grep .env d); echo "$K"`,
		`grep -n .env Makefile`, `grep -n AWS_SECRETS_MANAGER_NAME a.go`,
		// Permutation may be off, and an option may arrive by expansion.
		`POSIXLY_CORRECT=1 grep API -e .env`, `export POSIXLY_CORRECT=1; grep API -e .env`,
		`K=$(POSIXLY_CORRECT=1 grep API -e .env); echo "$K"`, `POSIXLY_CORRECT=1 egrep API -e .env`,
		`grep .env {-e,API}`, `grep .env {-eAPI,}`, `grep .env {-f,pat}`,
		`K=$(grep .env {-e,API}); echo "$K"`,
		// The first operand may split or glob once expanded.
		`V=' '; grep API$V./.env`, `grep API$(printf ' ')./.env`, `grep $(printf 'API ')./.env`,
		`IFS=,; V=,; grep API$V./.env`, `V='*'; grep .env$V`, `K=$(V=' '; grep API$V./.env); echo "$K"`,
		`grep -rn SECRET .`, `grep -rn AWS_SECRET_ACCESS_KEY .`, `sudo grep -r .env config.d`,
		`egrep -r .env config.d`, `grep -r -e .env config.d`, `grep -r -- .env config.d`,
		// An option outside the grammar stops the reading.
		`grep -d recurse .env config.d`, `grep --directories=recurse .env config.d`,
		`grep --recur .env config.d`, `grep -f .env x`, `grep --file=.env x`,
		`grep -r --include '.db_conn*' x .`, `grep -g '.db_conn*' x .`, `grep -rg '.db_conn*' x .`,
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
		`echo ".env $(date)"`,
		`f=$(grep -rln 'func (.*) ReleaseClaim' internal/x | head -1); sed -n 1,3p $f`,
		`V=$(sudo grep -c 'a credential file' a.go) && echo "$V"`,
		`K=$(grep -o 'event stream kept: .*' run.out); echo $K`,
		`K=$(timeout 5 grep -o '.*foo' run.out); echo $K`,
		`echo "t+$((s*2))s"`,
		`N=$(( (1+2)*3 )); echo "$N"`,
		`echo "$(( (a+b) * (c+d) ))"`,
		"gh pr create --body \"$(cat <<'EOF'\nsee .env\nEOF\n)\"",
		"X=$(cat <<E\nhello\nE\n); echo \"$X\"",
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
		// $(( that no )) closes is a command substitution holding a subshell.
		`X=$((cd . && cat .env) | grep API); echo "$X"`,
		`X=$((cat .env; echo) | head -1); echo "$X"`,
		`X=$((cat .env) | (cat)); echo "$X"`,
		`echo "$((cat .env) | cat)"`,
		`echo $((cat .env) )`,
		`echo "$((cat .en*) )"`,
		// Backquotes that nest.
		"echo \"`echo \\`cat .env\\``\"",
		"X=\"`echo \\`cat .env \\``\"; echo \"$X\"",
		// An early close is not hidden by a ( elsewhere in the word.
		`echo "($(case a in a) cat .env;; esac)"`,
		`X="$(case a in a) echo '(' >/dev/null; cat .env;; esac)"; echo "$X"`,
		`echo "$(echo ${Y:-)} '(' ; cat .env)"`,
		`echo "($(echo ${Y:-)} ; cat .env)"`,
		// A shell operator glued to a name in a program's text.
		`X=$(sh -c 'cat .env|cat'); echo "$X"`,
		`X=$(sh -c 'cat .env&&true'); echo "$X"`,
		`X=$(eval 'cat .env|cat'); echo "$X"`,
		"X=$(bash -c 'cat <.db_conn*'); echo \"$X\"",
		`X=$(bash -c 'cat .en*|/bin/cat'); echo "$X"`,
		"X=$(cat .pgpass; cat <<E\nhello\nE\n); echo \"$X\"",
		"echo \"$(cat .pgpass; cat <<E\nhello\nE\n)\"",
		"echo \"$(cat <<E\nhello\nE\ncat .pgpass\n)\"",
		"X=$(echo \"$(cat .pgpass; cat <<E\nhello\nE\n)\"); echo \"$X\"",
		`X=$(echo $(cat .pgpass|base64)); echo "$X"`,
		`V=$(grep "$(sh -c 'cat .en*')" f) && echo "$V"`,
		`V=$(grep x "my dir/.pgpass") && echo "$V"`,
		`V=$(grep -e x "my dir/.pgpass") && echo "$V"`,
		`V=$(grep -r --include='.db_conn*' '' .) && echo "$V"`,
		`V=$(grep -g '.db_conn*' x .) && echo "$V"`,
		`V=$(grep x .pgpass) && echo "$V"`,
		`V=$(grep -e x .pgpass) && echo "$V"`,
		`V=$(grep .* x) && echo "$V"`,
		`V=$(grep -r .pgpass dir) && echo "$V"`,
		`V=$(sudo grep -r .env dir) && echo "$V"`,
		`V=$(timeout 5 grep -r .env dir) && echo "$V"`,
		`V=$(grep -r --include '.db_conn*' '' .) && echo "$V"`,
		`V=$(<.pgpass grep x) && echo "$V"`,
		`V=$(grep <.pgpass) && echo "$V"`,
		`V=$(grep "$(cat .pgpass)" f) && echo "$V"`,
		`V=$(sed '1e cat .pgpass' f) && echo "$V"`,
		`V=$(F=x sh -c 'cat .pgpass') && echo "$V"`,
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

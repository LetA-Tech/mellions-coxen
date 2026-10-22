// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A substitution is lexed as the shell runs it, and quoted text inside it is
// searched for credential names and glob stems. A subject word that is not a
// credential's name, and a dot-star carrying a regex backslash, are words of
// the text rather than files.
func TestScanBash_QuotedTextInASubstitution(t *testing.T) {
	admitted := []string{
		`V=$(sed -n 's/^go \(.*\)$/go\1/p' go.mod) && echo "$V"`,
		`GV=$(sed -n 's/^go \(.*\)$/go\1/p' go.mod) && echo $GV`,
		`f=$(grep -rln 'a credential file' internal) && grep -n x "$f"`,
		`V=$(grep -rln 'secret token' .) && echo "$V"`,
		`echo "$(grep -c 'no secrets here' notes.md)"`,
		`echo "$(date) .env"`,
		`ARGS=$(sed -n '/run.sh/,/timeout/p' f | sed -E 's/>.*$//') && echo "$ARGS"`,
	}
	for _, cmd := range admitted {
		if got := ScanBash(cmd); len(got) != 0 {
			t.Errorf("ScanBash(%q) = %+v, want no finding: nothing it runs opens a credential", cmd, got)
		}
	}

	caught := []struct{ cmd, path string }{
		{`T=$(tail -1 .db_connection) && echo "$T"`, "$T"},
		{`V=$(cat .db_conn*) && echo "$V"`, "$V"},
		{`V=$(cat ".db_conn"*) && echo "$V"`, "$V"},
		{`echo "$(cat .db_conn*)"`, ".db_conn*"},
		{`echo "$(cat '.db_connection')"`, ".db_connection"},
		{`cat $(echo '.db_conn*')`, ".db_conn*"},
		{`V=$(find . -name '.db_conn*' -exec cat {} \;) && echo "$V"`, "$V"},
		{`find . -name '.db_conn*' -exec cat {} \;`, ".db_conn*"},
		{`grep -r --include '.db_conn*' '' .`, ".db_conn*"},
		{`V=$(python3 -c "print(open('.db_connection').read())") && echo "$V"`, "$V"},
		{`V=$(python3 -c "f = open('.db_connection'); print(f.read())") && echo "$V"`, "$V"},
		{`V=$(sh -c 'cat .pgpass') && echo "$V"`, "$V"},
		{`echo "$(bash -c 'cat .pgpass')"`, ".pgpass"},
		{`V=$(ssh host 'cat .env') && echo "$V"`, "$V"},
		{`V=$(cat "my dir/.pgpass") && echo "$V"`, "$V"},
		{`V=$(echo "$(cat .pgpass)") && echo "$V"`, "$V"},
		{`echo "$(<.pgpass)"`, ".pgpass"},
		{`echo $(( $(cat .pgpass) ))`, ".pgpass"},
		{"echo \"`cat .pgpass`\"", ".pgpass"},
		{"echo \"`date` `cat .pgpass`\"", ".pgpass"},
		{"V=\"`date`-`cat .pgpass`\" && echo \"$V\"", "$V"},
		{`V=$(sh -c 'cat credentials') && echo "$V"`, "$V"},
		{`V=$(sh -c 'cat .db_conn*') && echo "$V"`, "$V"},
		{`V=$(sh -c 'cat .*') && echo "$V"`, "$V"},
		{`echo "$(bash -c "find . -name '.env*' -exec cat {} +")"`, ".env*"},
		{`V=$(ssh host 'cat .env*') && echo "$V"`, "$V"},
		{`V=$(python3 -c "import glob; print(open(glob.glob('.db_conn*')[0]).read())") && echo "$V"`, "$V"},
		{`V=$(python3 -c "print(open(__import__('glob').glob('.db_conn*')[0]).read())") && echo "$V"`, "$V"},
		{`V=$(sh -c 'cat .*;') && echo "$V"`, "$V"},
		{`V=$(sh -c '(cat .*)') && echo "$V"`, "$V"},
		{`V=$(bash -c 'cat "".*') && echo "$V"`, "$V"},
		{`V=$(python3 -c "import glob,os; [print(open(f).read()) for f in glob.glob('.*') if os.path.isfile(f)]") && echo "$V"`, "$V"},
		// A .* in a pattern without a backslash cannot be told from a glob.
		{`K=$(grep -o 'event stream kept: .*' run.out); echo $K`, "$K"},
	}
	for _, c := range caught {
		got := ScanBash(c.cmd)
		hit := false
		for _, f := range got {
			hit = hit || f.Path == c.path
		}
		if !hit {
			t.Errorf("ScanBash(%q) = %+v, want a finding on %s", c.cmd, got, c.path)
		}
	}
}

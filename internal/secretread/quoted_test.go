// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A substitution is lexed as the shell runs it, and a wildcard counts only where
// the shell expands it. A quoted sed script or grep pattern is one literal word:
// neither its `.*` nor the words inside its quotes name a file the command opens.
func TestScanBash_QuotedTextIsOneLiteralWord(t *testing.T) {
	admitted := []string{
		`V=$(sed -n 's/^go \(.*\)$/go\1/p' go.mod) && echo "$V"`,
		`f=$(grep -rln 'a credential file' internal) && grep -n x "$f"`,
		`V=$(grep -c '.*' go.mod) && echo "$V"`,
		`V=$(sed 's/.*//' go.mod) && echo "$V"`,
		`echo "$(grep -c ".*" go.mod)"`,
		`grep -c '.*' go.mod`,
		`grep -c ".*" go.mod`,
		`grep -c \.\* go.mod`,
		`echo "$(date) .env"`,
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
		{`V=$(python3 -c "print(open('.db_connection').read())") && echo "$V"`, "$V"},
		{`V=$(echo "$(cat .pgpass)") && echo "$V"`, "$V"},
		{`echo "$(<.pgpass)"`, ".pgpass"},
		{"echo \"`cat .pgpass`\"", ".pgpass"},
		{`cat .db_conn*`, ".db_conn*"},
		{`cat ".db_conn"*`, ".db_conn*"},
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

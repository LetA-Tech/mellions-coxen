// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

func TestGlobsMarksOnlyAnUnquotedWildcard(t *testing.T) {
	tests := []struct {
		cmd  string
		want []bool
	}{
		{`cat .db_conn*`, []bool{false, true}},
		{`cat ".db_conn"*`, []bool{false, true}},
		{`ls a?b [ab]c`, []bool{false, true, true}},
		{`grep -c '.*' f`, []bool{false, false, false, false}},
		{`grep -c ".*" f`, []bool{false, false, false, false}},
		{`grep -c \.\* f`, []bool{false, false, false, false}},
		{`echo "$(ls *)"`, []bool{false, false}},
	}
	for _, tt := range tests {
		cmds := Split(tt.cmd)
		if len(cmds) != 1 {
			t.Fatalf("Split(%q) = %d commands", tt.cmd, len(cmds))
		}
		got := cmds[0].Globs
		if len(got) != len(tt.want) || len(got) != len(cmds[0].Words) {
			t.Fatalf("Split(%q).Globs = %v for words %q, want %v", tt.cmd, got, cmds[0].Words, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("Split(%q).Globs = %v for words %q, want %v", tt.cmd, got, cmds[0].Words, tt.want)
				break
			}
		}
	}
}

func TestSubstitutionReturnsTheCommandsItRuns(t *testing.T) {
	s := `X=$(sed -n 's/a b/c/p' f; cat .db_conn*) rest`
	cmds, next := Substitution(s, 2)
	if len(cmds) != 2 || s[next:] != " rest" {
		t.Fatalf("Substitution = %d commands, rest %q", len(cmds), s[next:])
	}
	if w := cmds[0].Words; len(w) != 4 || w[2] != "s/a b/c/p" || cmds[0].Globs[2] {
		t.Errorf("first command words %q globs %v", w, cmds[0].Globs)
	}
	if w := cmds[1].Words; len(w) != 2 || !cmds[1].Globs[1] {
		t.Errorf("second command words %q globs %v", w, cmds[1].Globs)
	}
	if cmds, next := Substitution("$((1+2)) x", 0); cmds != nil || next != 8 {
		t.Errorf("arithmetic Substitution = %v, %d", cmds, next)
	}
}

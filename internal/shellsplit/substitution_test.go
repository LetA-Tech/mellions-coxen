// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

func TestSubstitutionReturnsTheCommandsItRuns(t *testing.T) {
	s := `X=$(sed -n 's/a b/c/p' f; cat .db_conn*) rest`
	cmds, next := Substitution(s, 2)
	if len(cmds) != 2 || s[next:] != " rest" {
		t.Fatalf("Substitution = %d commands, rest %q", len(cmds), s[next:])
	}
	if w := cmds[0].Words; len(w) != 4 || w[2] != "s/a b/c/p" {
		t.Errorf("first command words %q", w)
	}
	if w := cmds[1].Words; len(w) != 2 || w[1] != ".db_conn*" {
		t.Errorf("second command words %q", w)
	}
	if cmds, next := Substitution("$((1+2)) x", 0); cmds != nil || next != 8 {
		t.Errorf("arithmetic Substitution = %v, %d", cmds, next)
	}
	// No )) closes it, so bash runs it as a command substitution.
	for _, s := range []string{"$((cat f) | wc)", "$((cat f) | (cat))", "$((cat f) )"} {
		if cmds, _ := Substitution(s, 0); len(cmds) == 0 {
			t.Errorf("Substitution(%q) runs no commands", s)
		}
	}
	for _, s := range []string{"$(( (1+2)*3 ))", "$(( (a+b) * (c+d) ))", "$((1))"} {
		if cmds, _ := Substitution(s, 0); cmds != nil {
			t.Errorf("Substitution(%q) runs %d commands, want arithmetic", s, len(cmds))
		}
	}
}

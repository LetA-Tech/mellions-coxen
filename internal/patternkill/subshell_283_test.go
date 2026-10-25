// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package patternkill

import "testing"

// A kill in a subshell written without spaces is read with the arguments bash
// passes it: the pattern pkill matches, and a pid scope it honours.
func TestAKillInAnUnspacedSubshellIsReadWithItsOwnArguments(t *testing.T) {
	if got, want := Find("(pkill -f foo)"), "pkill -f foo"; got != want {
		t.Errorf(`Find("(pkill -f foo)") = %q, want %q`, got, want)
	}
	if got := Find("(pkill -P $$ sleep)"); got != "" {
		t.Errorf(`Find("(pkill -P $$ sleep)") = %q, want "" (scoped to this shell's children)`, got)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import "testing"

// A number too large for a descriptor, and the 2 of 2>(…), are arguments bash
// passes: -t takes them as the title, and the body after them reaches GitHub.
func TestAWordBashPassesIsNotReadAsADescriptor(t *testing.T) {
	for _, command := range []string{
		`gh pr create -t 2147483648>/dev/null -b "Closes #1"`,
		`gh pr create -t 2>(true) -b "Closes #1"`,
		`gh pr edit 5 -t 2147483648>/dev/null --body "Fixes #1"`,
		`gh pr merge 5 -t 2147483648>/dev/null --body "Fixes #1"`,
	} {
		calls := Calls(command, "/tmp")
		found := false
		for _, c := range calls {
			for _, b := range c.Bodies {
				if b == "Closes #1" || b == "Fixes #1" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("Calls(%q) carry no body; bash passes the closing keyword to gh", command)
		}
	}
}

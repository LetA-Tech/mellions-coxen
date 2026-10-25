// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prmerge

import "testing"

// A merge in a subshell written without spaces names the pull request bash
// merges, so the guard looks up that one rather than standing aside on a
// selector no lookup can find.
func TestAMergeInAnUnspacedSubshellNamesItsPullRequest(t *testing.T) {
	for _, command := range []string{"(gh pr merge 12)", "( gh pr merge 12 )", "(cd x && gh pr merge 12)"} {
		got := Calls(command)
		if len(got) != 1 || got[0].Selector != "12" {
			t.Errorf("Calls(%q) = %+v, want one merge of 12", command, got)
		}
	}
}

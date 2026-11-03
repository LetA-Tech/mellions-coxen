// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prmerge

import "testing"

// A comment's text is no argument and no command: bash merges what the
// command line runs, so the guard reads that.
func TestAMergeIsReadPastAComment(t *testing.T) {
	cases := []struct {
		command  string
		selector []string
	}{
		{"echo ok # it's fine\ngh pr merge 7 --squash", []string{"7"}},
		{"gh pr merge # merge it once it's green", []string{""}},
		{"gh pr merge 7 # (then 8)", []string{"7"}},
		{"echo done # ; gh pr merge 9", nil},
	}
	for _, c := range cases {
		got := Calls(c.command)
		if len(got) != len(c.selector) {
			t.Errorf("Calls(%q) found %d merges, want %d: %+v", c.command, len(got), len(c.selector), got)
			continue
		}
		for i, call := range got {
			if call.Selector != c.selector[i] {
				t.Errorf("Calls(%q)[%d].Selector = %q, want %q", c.command, i, call.Selector, c.selector[i])
			}
		}
	}
}

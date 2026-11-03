// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prmerge

import "testing"

// The guard reads the merge bash runs past a comment, and keeps what a reading
// blind to comments makes of the comment's text.
func TestAMergeIsReadPastAComment(t *testing.T) {
	cases := []struct {
		command  string
		selector []string
	}{
		{"echo ok # it's fine\ngh pr merge 7 --squash", []string{"7"}},
		{"gh pr merge # merge it once it's green", []string{"", "#"}},
		{"gh pr merge 7 # (then 8)", []string{"7"}},
		{"echo done # ; gh pr merge 9", []string{"9"}},
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

// Where the comment rule takes a # bash reads as text, the merge after it is
// still read, from where the command line starts.
func TestAMergeAMisreadCommentWouldHideIsRead(t *testing.T) {
	for _, prefix := range []string{
		"echo $( [[ x =~ ( (a) #b) ]] ); ",
		"echo $( echo ${x:-(a) #} ); ",
		"echo $( (( (1) + 1 #)) ); ",
		"echo ${x:-<(echo }) #)}; ",
	} {
		command := prefix + "gh pr merge 5\ncd /elsewhere"
		found := false
		for _, call := range Calls(command) {
			if call.Selector == "5" && call.Dir == "" {
				found = true
			}
		}
		if !found {
			t.Errorf("Calls(%q) = %+v; bash runs gh pr merge 5 where the command line starts", command, Calls(command))
		}
	}
}

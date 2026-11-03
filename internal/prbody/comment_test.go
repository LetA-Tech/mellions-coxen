// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"slices"
	"testing"
)

// Where the comment rule takes a # bash reads as text, the commit after it is
// still read.
func TestACommitAMisreadCommentWouldHideIsRead(t *testing.T) {
	command := "echo $( echo ${x:-(a) #} ); git commit -m 'Closes #5'"
	if got := CommitMessages(command, "/home/you"); !slices.Contains(got, "Closes #5") {
		t.Errorf("CommitMessages(%q) = %q; bash commits with Closes #5", command, got)
	}
}

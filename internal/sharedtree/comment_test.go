// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// Where the comment rule takes a # bash reads as text, the reset after it is
// still found, in the checkout the command line starts in.
func TestAResetAMisreadCommentWouldHideIsFound(t *testing.T) {
	command := "echo $( echo ${x:-(a) #} ); git reset --hard\ncd /home/you/mellions/assignments/data-42/tree"
	if sharedtree.Find(command, "/home/you/workspace/data-service", estate) == nil {
		t.Errorf("Find(%q) = nil; bash resets the shared data-service checkout", command)
	}
}

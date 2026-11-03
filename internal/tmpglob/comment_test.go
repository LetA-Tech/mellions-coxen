// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package tmpglob

import "testing"

// Where the comment rule takes a # bash reads as text, the delete after it is
// still found.
func TestADeleteAMisreadCommentWouldHideIsFound(t *testing.T) {
	command := "echo $( echo ${x:-(a) #} ); rm -rf /tmp/*"
	if Find(command, "/home/you") == "" {
		t.Errorf("Find(%q) = \"\"; bash deletes /tmp/*", command)
	}
}

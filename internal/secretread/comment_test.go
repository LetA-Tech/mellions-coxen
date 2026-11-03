// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// Where the comment rule takes a # bash reads as text, the read after it is
// still found.
func TestAReadAMisreadCommentWouldHideIsFound(t *testing.T) {
	command := "echo $( echo ${x:-(a) #} ); cat ~/.pgpass"
	if len(ScanBash(command)) == 0 {
		t.Errorf("ScanBash(%q) found nothing; bash prints ~/.pgpass", command)
	}
}

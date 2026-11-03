// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package patternkill

import "testing"

func TestASignalIsReadPastAComment(t *testing.T) {
	for _, command := range []string{
		"true # don't leave it running\npkill -f gate",
		"echo ` #x`; pkill -f gate",
	} {
		if Find(command) == "" {
			t.Errorf("Find(%q) = \"\"; bash runs pkill -f gate", command)
		}
	}
	for _, command := range []string{
		"echo stopping # ; pkill -f gate",
		"true # (pkill -f gate)",
	} {
		if got := Find(command); got != "" {
			t.Errorf("Find(%q) = %q; bash runs no pkill, the comment holds it", command, got)
		}
	}
}

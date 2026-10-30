// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package patternkill

import "testing"

// A redirection's operator bytes do not end the command, so a kill by pattern
// written after one is still found.
func TestFind_PatternKillAfterARedirection(t *testing.T) {
	for _, cmd := range []string{
		`pkill <&- -f foo`,
		`pkill >| /dev/null -f foo`,
		`pkill <&3- -f foo`,
	} {
		if Find(cmd) == "" {
			t.Errorf("Find(%q) found nothing; pkill signals by pattern", cmd)
		}
	}
}

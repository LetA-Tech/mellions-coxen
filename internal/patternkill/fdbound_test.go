// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package patternkill

import "testing"

// A number too large for a descriptor, and the 2 of 2>(…), are arguments bash
// passes, so the option before them takes them and pkill still runs.
func TestAWordBashPassesIsNotReadAsADescriptor(t *testing.T) {
	for _, command := range []string{
		"nice -n 2147483648>/dev/null pkill -f gate",
		"env -u 2>(true) pkill -f gate",
	} {
		if Find(command) == "" {
			t.Errorf("Find(%q) = \"\"; bash runs pkill -f gate", command)
		}
	}
}

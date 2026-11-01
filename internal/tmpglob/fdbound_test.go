// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package tmpglob

import "testing"

// A number too large for a descriptor, and the 2 of 2>(…), are arguments bash
// passes, so the option before them takes them and rm still runs.
func TestAWordBashPassesIsNotReadAsADescriptor(t *testing.T) {
	for _, command := range []string{
		"nice -n 2147483648>/dev/null rm -rf /tmp/x*",
		"env -u 2>(true) rm -rf /tmp/x*",
	} {
		if Find(command, "/home/you") == "" {
			t.Errorf("Find(%q) = \"\"; bash runs rm -rf /tmp/x*", command)
		}
	}
}

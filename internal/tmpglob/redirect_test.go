// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package tmpglob

import "testing"

// A redirection's operator bytes do not end the command, so a glob delete
// written after one is still found.
func TestFind_GlobDeleteAfterARedirection(t *testing.T) {
	for _, cmd := range []string{
		`rm <&- -rf /tmp/*`,
		`rm >| /dev/null -rf /tmp/*`,
		`rm 0<&3- -rf /tmp/*`,
	} {
		if Find(cmd, "/home/you") == "" {
			t.Errorf("Find(%q) found nothing; rm deletes the glob", cmd)
		}
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package tmpglob

import "testing"

func TestFindRefusesARecursiveGlobOverASharedTempRoot(t *testing.T) {
	for _, cmd := range []string{
		`rm -rf /tmp/tmp.* 2>/dev/null; true`,
		`echo done; rm -rf /tmp/*`,
		`rm -r -f /tmp/tmp.??????????`,
		`rm --recursive --force /var/tmp/build-*`,
		`rm -fR /dev/shm/sem.*`,
		`rm -rf "$TMPDIR"/go-build*`,
		`rm -rf ${TMPDIR}/tmp.*`,
		`sudo rm -rf /tmp//tmp.*`,
		`cd /x && rm -rf -- /tmp/[a-z]*`,
	} {
		if Find(cmd) == "" {
			t.Errorf("not refused: %s", cmd)
		}
	}
}

func TestFindLeavesNamedPathsAlone(t *testing.T) {
	for _, cmd := range []string{
		`rm -rf /tmp/tmp.Ab12Cd34Ef`,
		`d=$(mktemp -d); rm -rf "$d"`,
		`rm -rf "$d"/*`,
		`rm -rf /tmp/tmp.Ab12Cd34Ef/*`,
		`rm -f /tmp/tmp.*`,
		`rm -rf ./build/*`,
		`ls /tmp/tmp.*`,
		`grep -r rm /tmp/x`,
	} {
		if got := Find(cmd); got != "" {
			t.Errorf("refused %q (operand %q)", cmd, got)
		}
	}
}

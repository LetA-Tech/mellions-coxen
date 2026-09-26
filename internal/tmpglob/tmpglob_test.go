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
		`cd /tmp && rm -rf tmp.*`,
		`cd /tmp; rm -rf ./tmp.*`,
		`for i in 1; do rm -rf /tmp/tmp.*; done`,
		`if [ -d /tmp ]; then rm -rf /tmp/tmp.*; fi`,
		`{ rm -rf /tmp/tmp.*; }`,
		`(rm -rf /tmp/tmp.*)`,
		`! rm -rf /tmp/tmp.*`,
		`timeout 60 rm -rf /tmp/tmp.*`,
		`time rm -rf /tmp/tmp.*`,
		`nice -n 10 rm -rf /tmp/tmp.*`,
		`sudo -n rm -rf /tmp/tmp.*`,
	} {
		if Find(cmd, "/home/you") == "" {
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
		`cd /tmp/tmp.Ab12 && rm -rf *`,
		`rm -rf tmp.*`,
		`rm -rf "$d"  # not /tmp/tmp.*`,
		`timeout 60 ls /tmp/tmp.*`,
	} {
		if got := Find(cmd, "/home/you"); got != "" {
			t.Errorf("refused %q (operand %q)", cmd, got)
		}
	}
}

// A session standing in a shared temporary root globs it with a relative path.
func TestFindReadsARelativeGlobFromTheSessionDirectory(t *testing.T) {
	if Find(`rm -rf tmp.*`, "/tmp") == "" {
		t.Error("a relative glob run from /tmp was not refused")
	}
	if got := Find(`rm -rf *`, "/tmp/tmp.Ab12Cd34Ef"); got != "" {
		t.Errorf("a glob inside a named scratch directory was refused (operand %q)", got)
	}
}

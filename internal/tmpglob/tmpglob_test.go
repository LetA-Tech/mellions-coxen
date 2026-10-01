// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package tmpglob

import (
	"strings"
	"testing"
)

func TestFindRefusesAGlobDeleteOverASharedTempRoot(t *testing.T) {
	for _, cmd := range []string{
		`rm -rf /tmp/tmp.* 2>/dev/null; true`,
		`echo done; rm -rf /tmp/*`,
		`rm -r -f /tmp/tmp.??????????`,
		`rm --recursive --force /var/tmp/build-*`,
		`rm -fR /dev/shm/sem.*`,
		`rm -f /tmp/tmp.*`,
		`rm /tmp/tmp.*`,
		`rm -f -- /var/tmp/build-*.log`,
		`rm -f "$TMPDIR"/go-build*`,
		`cd /tmp && rm -f tmp.*`,
		`timeout 60 rm -f /dev/shm/sem.*`,
		`rm -f "${TMPDIR:-/tmp}"/tmp.*`,
		`rm -rf ${TMPDIR-/tmp}/tmp.*`,
		`cd $TMPDIR && rm -f tmp.*`,
		`cd "${TMPDIR:-/tmp}" && rm -rf tmp.*`,
		`sudo -u x rm -f /tmp/tmp.*`,
		`timeout -s KILL 60 rm -f /tmp/tmp.*`,
		`env -u FOO rm -rf /tmp/tmp.*`,
		`rm -f /private/tmp/tmp.*`,
		`rm -rf /private/var/tmp/build-*`,
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
		`rm -f /tmp/tmp.Ab12Cd34Ef`,
		`rm -f "$d"/*.log`,
		`rm -f /tmp/tmp.Ab12Cd34Ef/*`,
		`rm -rf ./build/*`,
		`ls /tmp/tmp.*`,
		`grep -r rm /tmp/x`,
		`cd /tmp/tmp.Ab12 && rm -rf *`,
		`rm -rf tmp.*`,
		`rm -rf "$d"  # not /tmp/tmp.*`,
		`timeout 60 ls /tmp/tmp.*`,
		`rm -f "${TMPDIR:-/tmp}"/tmp.Ab12Cd34Ef`,
		`cd $HOME && rm -f tmp.*`,
		`cd $TMPDIR/tmp.Ab12 && rm -f *`,
		`sudo -u x ls /tmp/tmp.*`,
		`timeout -s KILL 60 ls /tmp/tmp.*`,
		`rm -f /private/tmp/tmp.Ab12Cd34Ef`,
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

// The refusal is read by a session that typed `rm -f`: it must say files are
// taken, not only directories.
func TestReasonNamesFilesAndDirectories(t *testing.T) {
	r := Reason("/tmp/tmp.*")
	for _, want := range []string{"/tmp/tmp.*", "files and directories", `f=$(mktemp)`, `"$d"`} {
		if !strings.Contains(r, want) {
			t.Errorf("Reason lacks %q:\n%s", want, r)
		}
	}
}

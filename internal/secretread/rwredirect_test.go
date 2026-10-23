// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A file opened with <> is read on stdin, and cat prints it.
func TestScanBash_ReadWriteRedirectIsARead(t *testing.T) {
	for _, cmd := range []string{
		`cat <> .pgpass`,
		`cat <>.pgpass`,
		`cat 0<>.pgpass`,
		`cat 0<> .db_connection | head -1`,
		`V=$(cat <> .pgpass) && echo "$V"`,
		`cat < .pgpass`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("ScanBash(%q) returned no finding; bash prints the file", cmd)
		}
	}
	if got := ScanBash(`echo x > .pgpass`); len(got) != 0 {
		t.Errorf("ScanBash(echo x > .pgpass) = %+v, want none: a write prints nothing", got)
	}
}

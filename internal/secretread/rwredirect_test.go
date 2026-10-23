// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A file opened with <> is read on stdin, and cat prints it. A stdin operand
// written first does not name the command: `<wc cat .pgpass` runs cat.
func TestScanBash_ReadWriteRedirectIsARead(t *testing.T) {
	for _, cmd := range []string{
		`cat <> .pgpass`,
		`cat <>.pgpass`,
		`cat 0<>.pgpass`,
		`cat 0<> .db_connection | head -1`,
		`V=$(cat <> .pgpass) && echo "$V"`,
		`cat < .pgpass`,
		`<wc cat .pgpass`,
		`< wc cat .pgpass`,
		`<>wc cat .pgpass`,
		`<<<wc cat .pgpass`,
		`<source cat .pgpass`,
		`ssh host 'cat #' < .pgpass -i`,
		`ssh host sh -c cat < .pgpass -i`,
		`< <(cat .pgpass) cat`,
		`< <(cat .env) sort`,
		`cat <(cat .pgpass)`,
		`ssh -i < <(cat .pgpass) KEY HOST cat`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("ScanBash(%q) returned no finding; bash prints the file", cmd)
		}
	}
	if got := ScanBash(`echo x > .pgpass`); len(got) != 0 {
		t.Errorf("ScanBash(echo x > .pgpass) = %+v, want none: a write prints nothing", got)
	}
}

// A stdin operand is still a file the command opens: wc reads it and prints a
// count, so naming a credential there is not a print.
func TestScanBash_StdinOperandKeepsItsReader(t *testing.T) {
	for _, cmd := range []string{`wc -l < .pgpass`, `<.pgpass wc -l`} {
		if got := ScanBash(cmd); len(got) != 0 {
			t.Errorf("ScanBash(%q) = %+v, want none: wc prints a count", cmd, got)
		}
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import (
	"strings"
	"testing"
)

// >| redirects stdout as > does, overriding noclobber, so cat still prints the
// operand written after its target.
func TestScanBash_NoclobberRedirectKeepsTheRead(t *testing.T) {
	for _, cmd := range []string{
		`cat >| /dev/stdout .pgpass`,
		`cat >|/dev/stderr .pgpass`,
		`cat 1>| /dev/stdout .db_connection`,
		`cat >| /dev/stdout .pgpass | head -1`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("ScanBash(%q) returned no finding; bash prints the file", cmd)
		}
	}
	if got := ScanBash(`echo x >| .pgpass`); len(got) != 0 {
		t.Errorf("ScanBash(echo x >| .pgpass) = %+v, want none: a write prints nothing", got)
	}
}

// Whatever the guard decides about a > redirection it decides about >|: the two
// differ only in whether bash honours noclobber, which changes no read.
func TestScanBash_NoclobberMatchesPlainRedirect(t *testing.T) {
	for _, cmd := range []string{
		`cat >| /dev/stdout .pgpass`,
		`cat >| out.txt .pgpass`,
		`cat .pgpass >| out.txt`,
		`echo x >| .pgpass`,
		`grep -c x .env >| /dev/null`,
		`base64 .pgpass >| /dev/stderr`,
	} {
		plain := strings.ReplaceAll(cmd, ">|", ">")
		got, want := ScanBash(cmd), ScanBash(plain)
		if len(got) != len(want) {
			t.Errorf("ScanBash(%q) = %+v, but ScanBash(%q) = %+v", cmd, got, plain, want)
		}
	}
}

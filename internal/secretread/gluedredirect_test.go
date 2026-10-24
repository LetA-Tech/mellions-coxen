// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A redirection operator glued to its file inside a capture still opens that
// file: bash prints the value each of these captures and then echoes.
func TestScanBash_GluedRedirectInsideCaptureIsARead(t *testing.T) {
	for _, cmd := range []string{
		`V=$(cat <>.pgpass) && echo "$V"`,
		`V=$(cat 0<>.pgpass) && echo "$V"`,
		`V=$(cat <.pgpass) && echo "$V"`,
		`V=$(<.pgpass cat) && echo "$V"`,
		`V=$(<.pgpass); echo "$V"`,
		`echo "$(cat <>.pgpass)"`,
		`printf '%s\n' "$(tr a b <>.pgpass)"`,
	} {
		if got := ScanBash(cmd); len(got) == 0 {
			t.Errorf("ScanBash(%q) returned no finding; bash prints the file", cmd)
		}
	}
	if got := ScanBash(`V=$(git log -1 <>/dev/null); echo "$V"`); len(got) != 0 {
		t.Errorf("ScanBash(glued redirect of /dev/null) = %+v, want none: no credential is named", got)
	}
}

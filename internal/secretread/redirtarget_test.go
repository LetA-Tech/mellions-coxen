// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A redirection's target is expanded before the command runs, so a
// substitution in it runs wherever it sits: after <& or >&, after >, glued to
// a descriptor prefix. Each prints the credential to stderr, which the
// transcript carries.
func TestASubstitutionInARedirectionTargetIsStillScanned(t *testing.T) {
	for _, cmd := range []string{
		`cat <&$(cat .pgpass >&2)`,
		`cat <& "$(cat .pgpass >&2)"`,
		`echo x >&$(cat .pgpass >&2)`,
		`echo x 2>&$(cat .pgpass >&2)`,
		"echo x >&`cat .pgpass >&2`",
		`echo x >$(cat .pgpass >&2)`,
		`echo x 2>"$(cat .pgpass >&2)"`,
		`cat 0<.pgpass`,
		`cat 3<>.pgpass <&3`,
	} {
		if len(ScanBash(cmd)) == 0 {
			t.Errorf("ScanBash(%q) found nothing; bash runs the read in it", cmd)
		}
	}
	for _, cmd := range []string{
		`git 2>/dev/null log --oneline`,
		`echo x 1>&2`,
		`echo x >&2 2>/dev/null`,
	} {
		if f := ScanBash(cmd); len(f) != 0 {
			t.Errorf("ScanBash(%q) = %v, want nothing: no credential is read", cmd, f)
		}
	}
}

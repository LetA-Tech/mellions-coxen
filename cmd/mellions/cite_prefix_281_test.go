// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"strings"
	"testing"
)

// TestCiteHook_GradesACallBehindAWrapperOrKeyword drives the real hook with a
// body whose citation quotes a line the session checkout does not hold. A
// wrapper or compound keyword before gh still runs gh, so each must be denied
// for that citation exactly as the bare call is.
func TestCiteHook_GradesACallBehindAWrapperOrKeyword(t *testing.T) {
	session, _ := citeFixture(t)
	wrong := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")
	right := citeBody(t, t.TempDir(), "\thealth, err := neonlink.NewHealthChecker(sdkCfg)")
	call := "gh pr create --base dev --body-file "

	for _, c := range []struct{ name, before, after string }{
		{"bare", "", ""},
		{"timeout", "timeout 60 ", ""},
		{"command", "command ", ""},
		{"env", "env ", ""},
		{"for do", "for x in 1; do ", "; done"},
		{"if then", "if true; then ", "; fi"},
		{"subshell", "( ", " )"},
		{"group", "{ ", "; }"},
	} {
		t.Run(c.name, func(t *testing.T) {
			reason := runCiteHook(t, session, c.before+call+wrong+c.after)
			if reason == "" {
				t.Fatalf("%q published a citation the checkout does not back, ungraded", c.before+call+"…"+c.after)
			}
			if !strings.Contains(reason, "neonlink.go:3") {
				t.Fatalf("denied, but not for the citation under test: %s", reason)
			}
			if reason := runCiteHook(t, session, c.before+call+right+c.after); reason != "" {
				t.Fatalf("a citation the checkout backs was denied behind %q: %s", c.before, reason)
			}
		})
	}
}

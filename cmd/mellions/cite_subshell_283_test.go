// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import "testing"

// A body published from a subshell is graded in the tree the subshell's cd
// moved to, spaced or not: the cd is a command the hook reads, so a citation
// quoting that tree's line is admitted.
func TestCiteHook_GradesASubshellBodyInTheTreeItsCdMovedTo(t *testing.T) {
	session, lane := citeFixture(t)
	body := "`neonlink.go:3`\n```go\n\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)\n```"

	for _, command := range []string{
		"(cd " + lane + " && gh pr comment 5 --body '" + body + "')",
		"( cd " + lane + " && gh pr comment 5 --body '" + body + "' )",
	} {
		if reason := runCiteHook(t, session, command); reason != "" {
			t.Errorf("%q: want admitted, got:\n%s", command, reason)
		}
	}
}

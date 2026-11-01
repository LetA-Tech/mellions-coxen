// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// A redirection written between git and its subcommand changes where output
// goes, never what git runs, so the subcommand read by position is still the
// one bash runs.
func TestARedirectionBeforeTheSubcommandHidesNoWrite(t *testing.T) {
	const checkout = "/home/you/workspace/data-service"
	for _, c := range []struct {
		cwd, command string
	}{
		{checkout, "git 2>/dev/null reset --hard"},
		{checkout, "git </dev/null reset --hard"},
		{checkout, "git <<<x reset --hard"},
		{checkout, "</dev/null git reset --hard"},
		{checkout, "git 2> /dev/null reset --hard"},
		{checkout, "git 2>>/tmp/log reset --hard"},
		{checkout, "git 2>|/tmp/log reset --hard"},
		{checkout, "git 0</dev/null reset --hard"},
		{checkout, "git 0<>/dev/null reset --hard"},
		{checkout, "git 0<<<x reset --hard"},
		{checkout, "git {fd}>/dev/null reset --hard"},
		{checkout, "git <& 0 reset --hard"},
		{checkout, "git >& 2 reset --hard"},
		{checkout, "git 2>& 1 reset --hard"},
		{checkout, "git >&/dev/null reset --hard"},
		{checkout, "git >& /dev/null reset --hard"},
		{checkout, "git <&- reset --hard"},
		{lane, "git 2>/dev/null -C " + checkout + " reset --hard"},
		{lane, "git -C " + checkout + " 2>/dev/null reset --hard"},
	} {
		if sharedtree.Deny(payload("Bash", c.cwd, c.command), estate) == "" {
			t.Errorf("Deny(%q) in %s allowed it; bash runs git reset --hard there", c.command, c.cwd)
		}
	}
	for _, command := range []string{
		"git 2>/dev/null status",
		"git 2>/dev/null log --oneline -5",
		"git >&2 diff",
	} {
		if got := sharedtree.Deny(payload("Bash", checkout, command), estate); got != "" {
			t.Errorf("Deny(%q) refused a read: %s", command, got)
		}
	}
}

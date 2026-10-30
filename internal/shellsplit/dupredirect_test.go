// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

// <& and >& duplicate, close or move a descriptor. Their & is part of the
// operator, so the words after them stay the command's own, and their target
// is no word of it.
func TestDescriptorDuplicationKeepsTheCommandWhole(t *testing.T) {
	for _, tc := range []struct {
		cmd   string
		words []string
	}{
		{`cat <&0 .pgpass`, []string{"cat", ".pgpass"}},
		{`echo <&- -n hi`, []string{"echo", "-n", "hi"}},
		{`rm <&- -rf /tmp/x*`, []string{"rm", "-rf", "/tmp/x*"}},
		{`pkill <&3- -f foo`, []string{"pkill", "-f", "foo"}},
		{`echo x >&2- y`, []string{"echo", "x", "y"}},
		{`echo x >&- y`, []string{"echo", "x", "y"}},
		{`echo x >&2 y`, []string{"echo", "x", "y"}},
	} {
		cmds := Split(tc.cmd)
		if len(cmds) != 1 {
			t.Errorf("Split(%q) gave %d commands, want 1: the & belongs to the redirection", tc.cmd, len(cmds))
			continue
		}
		c := cmds[0]
		if len(c.Words) != len(tc.words) {
			t.Errorf("Split(%q).Words = %q, want %q", tc.cmd, c.Words, tc.words)
			continue
		}
		for i := range tc.words {
			if c.Words[i] != tc.words[i] {
				t.Errorf("Split(%q).Words = %q, want %q", tc.cmd, c.Words, tc.words)
				break
			}
		}
		if len(c.Stdin) != 0 || c.Out != "" {
			t.Errorf("Split(%q) Stdin = %v, Out = %q: a descriptor is no file", tc.cmd, c.Stdin, c.Out)
		}
	}
	for _, tc := range []struct {
		cmd  string
		want int
	}{
		{`cat <&3; wc x`, 2},
		{`cat <&3 && wc x`, 2},
		{`cat <&0 & wc x`, 2},
	} {
		if got := len(Split(tc.cmd)); got != tc.want {
			t.Errorf("Split(%q) gave %d commands, want %d", tc.cmd, got, tc.want)
		}
	}
}

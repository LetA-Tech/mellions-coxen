// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

// <> opens its file for reading on stdin, so the file is an operand the
// command reads, exactly as after <, and never the file stdout is written to.
func TestReadWriteRedirectTargetIsARead(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want []string
	}{
		{`cat <> .pgpass`, []string{"cat", ".pgpass"}},
		{`cat <>.pgpass`, []string{"cat", ".pgpass"}},
		{`cat 0<>.pgpass`, []string{"cat", "0", ".pgpass"}},
		{`cat < .pgpass`, []string{"cat", ".pgpass"}},
	} {
		cmds := Split(tc.cmd)
		if len(cmds) != 1 {
			t.Fatalf("Split(%q) gave %d commands, want 1", tc.cmd, len(cmds))
		}
		c := cmds[0]
		if c.Out != "" {
			t.Errorf("Split(%q).Out = %q, want none: <> does not redirect stdout", tc.cmd, c.Out)
		}
		if len(c.Words) != len(tc.want) {
			t.Errorf("Split(%q).Words = %q, want %q", tc.cmd, c.Words, tc.want)
			continue
		}
		for i := range tc.want {
			if c.Words[i] != tc.want[i] {
				t.Errorf("Split(%q).Words = %q, want %q", tc.cmd, c.Words, tc.want)
				break
			}
		}
	}
	if got := Split(`echo x > out.txt`)[0].Out; got != "out.txt" {
		t.Errorf("Split(echo x > out.txt).Out = %q, want out.txt", got)
	}
}

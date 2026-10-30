// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import "testing"

// >| is one redirection, bash's override of noclobber: its | is no pipe, so the
// words after the target stay operands of the command before it.
func TestNoclobberRedirectIsOneRedirection(t *testing.T) {
	for _, tc := range []struct {
		cmd   string
		words []string
		out   string
	}{
		{`echo x >| out.txt`, []string{"echo", "x"}, "out.txt"},
		{`echo x >|out.txt`, []string{"echo", "x"}, "out.txt"},
		{`cat >| /dev/stdout .pgpass`, []string{"cat", ".pgpass"}, "/dev/stdout"},
		{`cat >|/dev/stderr .pgpass`, []string{"cat", ".pgpass"}, "/dev/stderr"},
	} {
		cmds := Split(tc.cmd)
		if len(cmds) != 1 {
			t.Errorf("Split(%q) gave %d commands, want 1: >| is not a pipe", tc.cmd, len(cmds))
			continue
		}
		c := cmds[0]
		if c.Out != tc.out {
			t.Errorf("Split(%q).Out = %q, want %q", tc.cmd, c.Out, tc.out)
		}
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
	}
}

// Only the | that touches an unquoted single > belongs to it: a pipe after a
// redirection's target, after >>, or after a quoted or escaped > still
// separates two commands.
func TestNoclobberLeavesPipesAlone(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want int
	}{
		{`echo x >out.txt | wc`, 2},
		{`echo x >| out.txt | wc`, 2},
		{`echo x >| out.txt; cat y`, 2},
		{`echo x >> out.txt | wc`, 2},
		{`echo x >&2 | wc`, 2},
		{`echo x \>| cat .pgpass`, 2},
		{`echo x '>'| cat .pgpass`, 2},
		{`echo x ">"| cat .pgpass`, 2},
	} {
		if got := len(Split(tc.cmd)); got != tc.want {
			t.Errorf("Split(%q) gave %d commands, want %d", tc.cmd, got, tc.want)
		}
	}
}

// An fd prefix leaves the operand the command reads, and stdout's target, as
// they are without it.
func TestNoclobberWithDescriptorKeepsTheOperand(t *testing.T) {
	cmds := Split(`cat 1>| /dev/stdout .pgpass`)
	if len(cmds) != 1 {
		t.Fatalf("Split(cat 1>| /dev/stdout .pgpass) gave %d commands, want 1", len(cmds))
	}
	if cmds[0].Out != "/dev/stdout" {
		t.Errorf("Out = %q, want /dev/stdout", cmds[0].Out)
	}
	found := false
	for _, w := range cmds[0].Words {
		found = found || w == ".pgpass"
	}
	if !found {
		t.Errorf("Words = %q, want .pgpass among them", cmds[0].Words)
	}
}

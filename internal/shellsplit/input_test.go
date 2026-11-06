// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"slices"
	"testing"
)

func TestInputIsWhatDescriptorZeroReads(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    []string
	}{
		{"sh <<EOF\nx\nEOF", []string{"x\n"}},
		{"sh 0<<EOF\nx\nEOF", []string{"x\n"}},
		{"sh 3<<EOF\nx\nEOF", nil},
		{"sh {fd}<<EOF\nx\nEOF", nil},
		{"sh 2>e <<EOF\nx\nEOF", []string{"x\n"}},
		{"sh <<<x", []string{"x"}},
		{"sh 3<<<x", nil},
		{"sh <f", []string{"f"}},
		{"sh 3<f <g", []string{"g"}},
		{"sh <<EOF\n\\$a \\`b\\` \\\\c \\d\nEOF", []string{"$a `b` \\c \\d\n"}},
		{"sh <<'EOF'\n\\$a\nEOF", []string{"\\$a\n"}},
		{"sh <<\\EOF\n\\$a\nEOF", []string{"\\$a\n"}},
		{"sh <<\"EOF\"\n\\$a\nEOF", []string{"\\$a\n"}},
	} {
		if got := Split(tc.command)[0].Input; !slices.Equal(got, tc.want) {
			t.Errorf("%q: Input = %q, want %q", tc.command, got, tc.want)
		}
	}
}

func TestPipedIsWhetherStdinIsThePreviousCommand(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{"a | b", true},
		{"a |& b", true},
		{"a || b", false},
		{"a ; b", false},
		{"a && b", false},
		{"a\nb", false},
		{"a |\nb", true},
		{"a | # note\n\nb", true},
		{"a ;\nb", false},
	} {
		if got := Split(tc.command)[1].Piped; got != tc.want {
			t.Errorf("%q: Piped = %v, want %v", tc.command, got, tc.want)
		}
	}
}

func TestAnUnquotedHeredocRunsWhatIsLiveInIt(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    int
	}{
		{"cat <<EOF\n$(a b)\nEOF", 1},
		{"cat <<EOF\n`a b`\nEOF", 1},
		{"cat <<EOF\n\\$(a b)\nEOF", 0},
		{"cat <<EOF\n\\`a b\\`\nEOF", 0},
		{"cat <<EOF\n\\\\$(a b)\nEOF", 1},
		{"cat <<'EOF'\n$(a b)\nEOF", 0},
		{"cat <<EOF\nno substitution\nEOF", 0},
	} {
		if got := len(Split(tc.command)[0].Body); got != tc.want {
			t.Errorf("%q: %d commands run, want %d", tc.command, got, tc.want)
		}
	}
}

func TestProgramFindsTheCommandBehindWhatOpensIt(t *testing.T) {
	for _, tc := range []struct {
		words []string
		want  string
	}{
		{[]string{"if", "rm", "x"}, "rm"},
		{[]string{"elif", "rm"}, "rm"},
		{[]string{"while", "rm"}, "rm"},
		{[]string{"until", "rm"}, "rm"},
		{[]string{"builtin", "rm"}, "rm"},
		{[]string{"case", "a", "in", "a)", "rm"}, "rm"},
		{[]string{"case", "a", "in", "(a)", "rm"}, "rm"},
		{[]string{"a)", "rm"}, "rm"},
		{[]string{"f()", "{", "rm"}, "rm"},
		{[]string{"function", "f", "{", "rm"}, "rm"},
		{[]string{"f(){", "rm"}, "rm"},
		{[]string{"f", "()", "{", "rm"}, "rm"},
		{[]string{"f()(rm"}, "rm"},
		{[]string{"/usr/bin/env", "rm"}, "rm"},
		{[]string{"/usr/bin/sudo", "-u", "x", "rm"}, "rm"},
		{[]string{"sudo", "-iu", "x", "rm"}, "rm"},
		{[]string{"sudo", "--user", "x", "rm"}, "rm"},
		{[]string{"exec", "-a", "n", "rm"}, "rm"},
		{[]string{"(rm)"}, "rm"},
		{[]string{"(rm", "x)"}, "rm"},
		{[]string{"$(which rm)", "x"}, "$(which rm)"},
		{[]string{"$(which rm)"}, "$(which rm)"},
	} {
		got := Program(tc.words)
		if len(got) == 0 || got[0] != tc.want {
			t.Errorf("Program(%q) = %q, want program %q", tc.words, got, tc.want)
		}
	}
	if got := Program([]string{"case", "a"}); len(got) != 0 {
		t.Errorf("a case with no in runs nothing, got %q", got)
	}
}

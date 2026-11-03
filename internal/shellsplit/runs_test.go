// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package shellsplit

import (
	"strings"
	"testing"
)

// TestRunsHoldsOnlyTheSubstitutionsBashRuns: a $(…) bash expands puts its
// commands on Runs; one inside single quotes or escaped is text and puts none.
func TestRunsHoldsOnlyTheSubstitutionsBashRuns(t *testing.T) {
	for _, tc := range []struct {
		line string
		runs string
	}{
		{`ID=$(docker run -d x)`, "docker run -d x"},
		{`echo "id: $(docker create x)"`, "docker create x"},
		{`grep x <<< "$(hostname -f)"`, "hostname -f"},
		{`echo "$(a "$(b c)")"`, "a $(b c)"},
		{`echo 'use $(docker run x)'`, ""},
		{`echo "use \$(docker run x)"`, ""},
		{`echo $((1 + 2))`, ""},
		{"git commit -m \"$(cat <<'M'\nuse $(docker run x)\nM\n)\"", "cat"},
	} {
		var got []string
		for _, c := range Split(tc.line) {
			for _, r := range c.Runs {
				got = append(got, strings.Join(r.Words, " "))
			}
		}
		if strings.Join(got, "; ") != tc.runs {
			t.Errorf("%q: Runs = %q, want %q", tc.line, got, tc.runs)
		}
	}
}

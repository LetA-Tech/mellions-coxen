// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"reflect"
	"testing"
)

// A --body bash expands is graded as its own source text, which cites nothing,
// so every citation in the text gh publishes went unchecked. It is unread.
func TestUnreadNamesEveryBodyBashExpands(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		want          []string
	}{
		{"substitution", `gh pr comment 5 --body "$(cat review.md)"`, []string{"$(cat review.md)"}},
		{"short flag", `gh issue comment 5 -b "$(cat review.md)"`, []string{"$(cat review.md)"}},
		{"glued", `gh pr create --body="$BODY"`, []string{"$BODY"}},
		{"glued short flag", `gh pr edit 3 -b"$BODY"`, []string{"$BODY"}},
		{"variable", `gh pr review 3 --comment --body "$BODY"`, []string{"$BODY"}},
		{"backtick", "gh issue create -t t --body \"`cat review.md`\"", []string{"`cat review.md`"}},
		{"after an assignment prefix", `GH_PAGER= gh pr comment 5 --body "$(cat r.md)"`, []string{"$(cat r.md)"}},
		{"literal", `gh pr comment 5 --body "plain text"`, nil},
		{"single-quoted dollar", `gh pr comment 5 --body 'costs $5'`, nil},
		{"escaped dollar", `gh pr comment 5 --body "costs \$5"`, nil},
		{"substituted quoted heredoc", "gh pr comment 5 --body \"$(cat <<'EOF'\nbody\nEOF\n)\"", nil},
		{"title expands, body literal", `gh pr create --title "$T" --body "plain"`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := Publishing(tc.command, t.TempDir())
			if len(calls) != 1 {
				t.Fatalf("got %d calls", len(calls))
			}
			var got []string
			for _, u := range calls[0].Unread {
				if u.Flag != "--body" {
					t.Errorf("Flag = %q, want --body", u.Flag)
				}
				got = append(got, u.Spec)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Unread = %q, want %q", got, tc.want)
			}
		})
	}
}

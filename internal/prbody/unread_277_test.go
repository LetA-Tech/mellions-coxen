// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUnreadNamesEveryBodyFileSpecThisParseCouldNotRead(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "b.md"), "body\n")
	gone := filepath.Join(dir, "gone.md")
	// A file whose literal name is the unexpanded spec: bash reads what $S
	// expands to, so reading this one would grade a body gh never publishes.
	if err := os.MkdirAll(filepath.Join(dir, "$S"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "$S", "b.md"), "not the body gh reads\n")

	for _, tc := range []struct {
		name, command string
		want          []string
	}{
		{"literal path", "gh pr create --body-file " + filepath.Join(dir, "b.md"), nil},
		{"relative path", "gh pr create -F b.md", nil},
		{"heredoc on stdin", "gh pr create --body-file - <<'EOF'\nbody\nEOF", nil},
		{"heredoc to an expanded path", "cat > $S/b.md <<'EOF'\nbody\nEOF\ngh pr create --body-file $S/b.md", nil},
		{"variable", "gh pr create --body-file $S/b.md", []string{"$S/b.md"}},
		{"glued", "gh pr comment 1 --body-file=$S/b.md", []string{"$S/b.md"}},
		{"braced variable", "gh issue create -F ${S}/b.md", []string{"${S}/b.md"}},
		{"missing path", "gh issue comment 1 --body-file " + gone, []string{gone}},
		{"empty spec", "gh pr edit 1 --body-file ''", []string{""}},
		{"stdin from a pipe", "printf 'body' | gh pr create --body-file -", []string{"-"}},
		{"stdin from a here-string", "gh issue comment 1 -F - <<< 'body'", []string{"-"}},
		{"stdin from a redirect", "gh pr comment 1 --body-file - < b.md", []string{"-"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := Publishing(tc.command, dir)
			if len(calls) != 1 {
				t.Fatalf("got %d calls", len(calls))
			}
			if got := calls[0].Unread; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Unread = %q, want %q", got, tc.want)
			}
		})
	}
}

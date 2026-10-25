// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A body file a subshell writes earlier in the line is the one gh reads, not
// whatever stale copy is on disk when the hook runs.
func TestABodyFileASubshellWritesIsNotReadFromDisk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	calls := Calls("(printf 'Closes #9')>b.md; gh pr create --body-file b.md", dir)
	if len(calls) != 1 || len(calls[0].Unread) != 1 {
		t.Errorf("printf form: want one call with its body unread, got %+v", calls)
	}

	calls = Calls("(cat <<'EOF')>b.md\nfresh body\nEOF\ngh pr create --body-file b.md", dir)
	if len(calls) != 1 || len(calls[0].Bodies) != 1 || !strings.Contains(calls[0].Bodies[0], "fresh body") {
		t.Errorf("heredoc form: want the heredoc as the body, got %+v", calls)
	}
}

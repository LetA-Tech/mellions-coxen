// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// TestShiftIdIsUniquePerShift: the shift id names five files — log, survey,
// prompt, reply and stream. Two shifts launched in the same wall-clock second
// once took the same id, so they appended into one log and truncated each
// other's stream and reply; whichever finished last owned the reply, and the
// other shift's hour was unrecoverable (#35).
//
// The race is made deterministic rather than raced for: a stub `date` earlier
// on PATH returns one fixed second for the id format, so both shifts see the
// same second every run. A correct shift.sh still gives them different ids.
func TestShiftIdIsUniquePerShift(t *testing.T) {
	root := repoRoot(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash: %v", err)
	}

	home := t.TempDir()
	stub := t.TempDir()

	// Fixed only for the id format. Everything else in shift.sh asks date for
	// a clock (elapsed seconds, the deadline, each log line's time), and
	// freezing those would be testing something other than the id.
	write(t, filepath.Join(stub, "date"), `#!/bin/sh
for a in "$@"; do
  [ "$a" = "+%Y%m%d-%H%M%S" ] && { echo 20260828-120000; exit 0; }
done
exec /bin/date "$@"
`)
	// The survey is skipped (MELLIONS_PROMPT is set), so mellions is reached
	// only by the no-reply backstop.
	write(t, filepath.Join(stub, "mellions"), `#!/bin/sh
# The shift asks where this installation's home is and refuses without an
# answer, so a stub that only exits 0 stops the script before it runs.
[ "$1" = config ] && case "$2" in
  home)    echo "$MELLIONS_HOME"; exit 0 ;;
  reports) echo "$MELLIONS_HOME/reports"; exit 0 ;;
esac
exit 0
`)
	// A paragraph and a result: shift-follow.py writes the reply only on the
	// result event, so both are needed for the shift to take its normal path
	// rather than the empty-reply backstop.
	write(t, filepath.Join(stub, "claude"), `#!/bin/sh
cat >/dev/null
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"stub shift"}]}}'
echo '{"type":"result","result":"stub shift"}'
`)

	task := filepath.Join(stub, "task.md")
	if err := os.WriteFile(task, []byte("do nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(),
		"PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"),
		"MELLIONS_HOME="+home,
		"MELLIONS_WORKDIR="+home,
		"MELLIONS_BIN="+filepath.Join(stub, "mellions"),
		"CLAUDE_BIN="+filepath.Join(stub, "claude"),
		"MELLIONS_PROMPT="+task,
	)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command("bash", filepath.Join(root, "scripts", "shift.sh"))
			cmd.Env = env
			cmd.Dir = home
			// The exit status is not the subject: a shift may end unhappily
			// for reasons that have nothing to do with which files it owns.
			out, _ := cmd.CombinedOutput()
			t.Logf("shift output:\n%s", out)
		}()
	}
	wg.Wait()

	ids := stampsIn(t, filepath.Join(home, "shifts"))
	if len(ids) != 2 {
		t.Fatalf("two shifts in one second produced %d distinct ids (%v) — they share a log, a prompt, a stream and a reply, and one shift's record overwrites the other's", len(ids), ids)
	}

	// A distinct id is only half of it: each shift must actually own its five
	// files, so a loser that took a fresh id but kept writing to the old paths
	// is still a failure.
	for _, id := range ids {
		for _, suffix := range []string{".log", ".prompt.md", ".stream.jsonl", ".reply.md"} {
			p := filepath.Join(home, "shifts", id+suffix)
			st, err := os.Stat(p)
			if err != nil {
				t.Errorf("shift %s: %v", id, err)
				continue
			}
			if st.Size() == 0 {
				t.Errorf("shift %s: %s is empty — the shift wrote its %s somewhere else", id, id+suffix, suffix)
			}
		}
	}
}

// stampsIn returns the distinct shift ids that own a log under dir.
func stampsIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	var ids []string
	for _, e := range entries {
		if name := e.Name(); strings.HasSuffix(name, ".log") {
			ids = append(ids, strings.TrimSuffix(name, ".log"))
		}
	}
	sort.Strings(ids)
	return ids
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

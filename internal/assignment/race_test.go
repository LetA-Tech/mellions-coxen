// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

// TestEveryConcurrentFindingSurvives.
//
// Parallel sessions and an overlapping recovery are the situation this record
// exists for, and they are exactly when an unguarded read-modify-write loses
// the change that arrived second. Nothing reports the loss: both callers
// succeed and the file keeps one of them.
//
// A shared staging name adds a second failure on top — two writers renaming the
// same temporary file over the record, one of them onto a file the other has
// already replaced.
func TestEveryConcurrentFindingSurvives(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	s := fakeStore(t)
	a := open(t, s)

	const writers = 100
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if err := s.Record(a.ID, "found", fmt.Sprintf("finding %03d", i)); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		t.Errorf("a concurrent record failed: %v", err)
	}

	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != writers {
		t.Errorf("%d of %d findings survived — the rest were overwritten silently",
			len(got.Findings), writers)
	}
	seen := map[string]bool{}
	for _, f := range got.Findings {
		seen[f.Text] = true
	}
	var lost []string
	for i := 0; i < writers; i++ {
		if want := fmt.Sprintf("finding %03d", i); !seen[want] {
			lost = append(lost, want)
		}
	}
	if len(lost) > 0 {
		t.Errorf("lost %d findings, first few: %v", len(lost), lost[:min(5, len(lost))])
	}

	// Nothing staged may be left in the assignment directory.
	entries, err := os.ReadDir(s.dir(a.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("a write left %s behind", e.Name())
		}
	}
}

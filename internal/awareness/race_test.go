package awareness_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// TestEveryConcurrentDeliverySurvives.
//
// The session-start hooks render every partnership and every program, each in
// its own process, and all of them write this one file. An unguarded
// read-modify-write drops whichever arrived second and reports success for it,
// so a document is silently left with no baseline — which is not a note that
// comes late, it is a document the session is never told about again.
func TestEveryConcurrentDeliverySurvives(t *testing.T) {
	rec := awareness.Delivered{Path: awareness.DeliveredPath(t.TempDir(), "claude", "sess-race")}

	const writers = 64
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	start := make(chan struct{})
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			slug := fmt.Sprintf("doc-%03d", i)
			if err := rec.Record(awareness.Delivery{
				Kind: awareness.KindProgram, Slug: slug, Digest: fmt.Sprintf("v-%03d", i),
			}); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		t.Errorf("a concurrent delivery failed: %v", err)
	}
	all := rec.All()
	if len(all) != writers {
		t.Fatalf("%d of %d deliveries survived: a session was handed a document and holds no record of it",
			len(all), writers)
	}
	for i := range writers {
		key := fmt.Sprintf("%s/doc-%03d", awareness.KindProgram, i)
		if got := all[key].Digest; got != fmt.Sprintf("v-%03d", i) {
			t.Errorf("%s recorded as %q", key, got)
		}
	}
}

// TestConcurrentSettlingKeepsOneRecord.
//
// A prompt hook and a tool-call hook run against the same session at the same
// time, and each folds its own reading into this file. Deciding from one
// process's view and writing it back over another's loses the sighting that was
// part-way through earning its note, which turns the wait into a wait that
// never ends.
func TestConcurrentSettlingKeepsOneRecord(t *testing.T) {
	rec := awareness.Delivered{Path: awareness.DeliveredPath(t.TempDir(), "claude", "sess-settle-race")}
	docs := []string{"alpha", "beta", "gamma", "delta"}
	for _, slug := range docs {
		if err := rec.Record(delivery(awareness.KindProgram, slug, "v1", nil)); err != nil {
			t.Fatal(err)
		}
	}

	at := time.Now()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, slug := range docs {
		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			<-start
			rec.Settle([]awareness.Reading{{
				Kind: awareness.KindProgram, Slug: slug, Digest: "v2",
			}}, at)
		}(slug)
	}
	close(start)
	wg.Wait()

	seen := rec.Seen()
	if len(seen) != len(docs) {
		t.Fatalf("%d of %d readings survived being folded in at once: %+v", len(seen), len(docs), seen)
	}
}

// TestABaselineIsNeverHalfThere.
//
// Every hook firing reads this file, and a truncate-then-write leaves a window
// where it is empty. A reader landing in that window sees a session that was
// handed nothing — silence about every governing document at once, and, if the
// machine stops inside the window, silence that outlives the session.
func TestABaselineIsNeverHalfThere(t *testing.T) {
	rec := awareness.Delivered{Path: awareness.DeliveredPath(t.TempDir(), "claude", "sess-torn-record")}
	if err := rec.Record(delivery(awareness.KindPartnership, "alex", "v1", nil)); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = rec.Record(delivery(awareness.KindProgram, "sample", fmt.Sprintf("v%d", i),
				map[string]string{"Purpose": "p", "Map": "m", "Constraints": "c"}))
		}
	}()

	var empty, reads int64
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if len(rec.All()) == 0 {
					atomic.AddInt64(&empty, 1)
				}
				atomic.AddInt64(&reads, 1)
			}
		}()
	}
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()

	if n := atomic.LoadInt64(&reads); n < 1000 {
		t.Fatalf("only %d reads: the window was never sampled hard enough to prove anything", n)
	}
	if n := atomic.LoadInt64(&empty); n != 0 {
		t.Errorf("%d of %d reads saw a session that had been handed nothing",
			n, atomic.LoadInt64(&reads))
	}
}

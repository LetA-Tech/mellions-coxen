package awareness_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// The windows the mechanism holds a reading for, restated here rather than
// exported: a test that reads the constant it is checking cannot catch the
// constant being changed.
const (
	shortOfTheWindow = 2 * time.Second
	pastTheWindow    = 5 * time.Second
	pastGone         = 45 * time.Second
)

func store(t *testing.T) awareness.Delivered {
	t.Helper()
	rec := awareness.Delivered{Path: awareness.DeliveredPath(t.TempDir(), "claude", "sess-settle")}
	if err := rec.Record(delivery(awareness.KindPartnership, "alex", "v1",
		map[string]string{"Delegated": "d1"})); err != nil {
		t.Fatal(err)
	}
	return rec
}

func reading(digest string, sections ...awareness.SectionStamp) awareness.Reading {
	return awareness.Reading{
		Kind: awareness.KindPartnership, Slug: "alex", Digest: digest, Sections: sections,
	}
}

func missing() awareness.Reading {
	return awareness.Reading{Kind: awareness.KindPartnership, Slug: "alex", Missing: true}
}

// TestOneReadingOfANewVersionIsNotYetAFact.
//
// The whole defect in one assertion: a single unsynchronised read of a file
// somebody may be writing is a sample of a moment. Nothing may be said on it.
func TestOneReadingOfANewVersionIsNotYetAFact(t *testing.T) {
	rec, at := store(t), time.Now()
	if docs := rec.Settle([]awareness.Reading{reading("v2")}, at); len(docs) != 0 {
		t.Fatalf("spoke on a single reading: %+v", docs)
	}
	if got := rec.Seen()["partnership/alex"].State; got != "v2" {
		t.Fatalf("the reading was not held for confirmation: %q", got)
	}
	if docs := rec.Settle([]awareness.Reading{reading("v2")}, at.Add(shortOfTheWindow)); len(docs) != 0 {
		t.Fatalf("spoke before the reading had held for the window: %+v", docs)
	}
	docs := rec.Settle([]awareness.Reading{reading("v2")}, at.Add(pastTheWindow))
	if len(docs) != 1 || docs[0].Digest != "v2" || docs[0].Missing || docs[0].Back {
		t.Fatalf("a reading that held for the window was not confirmed: %+v", docs)
	}
}

// TestARenameStyleSaveNeverSaysThePartnershipIsGone.
//
// `mv f f~; cp f~ f` — an editor's default save, a `rm && cp` deploy — leaves a
// window with no file. It is microseconds to milliseconds wide and the hook
// samples it on every prompt and every tool call. Reproduced against the
// unconfirmed read as one permanent false note that nothing ever retracts.
func TestARenameStyleSaveNeverSaysThePartnershipIsGone(t *testing.T) {
	rec, at := store(t), time.Now()
	for i := range 200 {
		// The window, then the file back exactly as it was, a few ms apart.
		when := at.Add(time.Duration(i) * 20 * time.Millisecond)
		if docs := rec.Settle([]awareness.Reading{missing()}, when); len(docs) != 0 {
			t.Fatalf("save %d: said the partnership was gone: %+v", i, docs)
		}
		if docs := rec.Settle([]awareness.Reading{reading("v1")}, when.Add(5*time.Millisecond)); len(docs) != 0 {
			t.Fatalf("save %d: said something about a document that never changed: %+v", i, docs)
		}
	}
}

// TestADocumentThatIsReallyGoneIsStillReported: the window is a delay, not a
// silence. An absence that outlasts it is the owner's, and is said.
func TestADocumentThatIsReallyGoneIsStillReported(t *testing.T) {
	rec, at := store(t), time.Now()
	rec.Settle([]awareness.Reading{missing()}, at)
	if docs := rec.Settle([]awareness.Reading{missing()}, at.Add(pastTheWindow)); len(docs) != 0 {
		t.Fatalf("an absence was believed on the short window: %+v", docs)
	}
	docs := rec.Settle([]awareness.Reading{missing()}, at.Add(pastGone))
	if len(docs) != 1 || !docs[0].Missing {
		t.Fatalf("a document gone for a minute was never reported: %+v", docs)
	}
}

// TestATornRewriteDeliversNoPhantomChange.
//
// An in-place rewrite is a run of intermediate states, each of which parses,
// digests differently from what was delivered, and appears to have lost every
// section the writer has not reached. Against the unconfirmed read each one was
// a distinct new fact, so the once-per-version discipline gave no protection at
// all: reproduced as four notes to one session, three of them naming sections
// nobody touched.
func TestATornRewriteDeliversNoPhantomChange(t *testing.T) {
	rec, at := store(t), time.Now()
	torn := []string{"chunk1", "chunk2", "chunk3", "chunk4", "chunk5", "chunk6"}
	for i, d := range torn {
		when := at.Add(time.Duration(i) * 8 * time.Millisecond)
		if docs := rec.Settle([]awareness.Reading{reading(d)}, when); len(docs) != 0 {
			t.Fatalf("chunk %d was reported as a change: %+v", i, docs)
		}
	}
	// The writer finishes; the settled document is the one that is spoken about,
	// once, and it is the real one.
	done := at.Add(time.Second)
	if docs := rec.Settle([]awareness.Reading{reading("v2")}, done); len(docs) != 0 {
		t.Fatalf("the finished document was believed on its first reading: %+v", docs)
	}
	docs := rec.Settle([]awareness.Reading{reading("v2")}, done.Add(pastTheWindow))
	if len(docs) != 1 || docs[0].Digest != "v2" {
		t.Fatalf("the real change was lost: %+v", docs)
	}
}

// TestAFileWrittenInsideTheWindowIsNotBelieved: two readings that agree are not
// enough on their own — a writer that repeats itself, or one whose next chunk
// has not landed yet, produces exactly that. A file touched inside the window
// is one somebody may still be writing.
func TestAFileWrittenInsideTheWindowIsNotBelieved(t *testing.T) {
	rec, at := store(t), time.Now()
	fresh := reading("v2")
	fresh.ModTime = at.Add(pastTheWindow)
	rec.Settle([]awareness.Reading{fresh}, at)
	if docs := rec.Settle([]awareness.Reading{fresh}, at.Add(pastTheWindow)); len(docs) != 0 {
		t.Fatalf("spoke about a file written a moment ago: %+v", docs)
	}
	settled := reading("v2")
	settled.ModTime = at
	if docs := rec.Settle([]awareness.Reading{settled}, at.Add(pastTheWindow)); len(docs) != 1 {
		t.Fatalf("a file nobody has touched since was never confirmed: %+v", docs)
	}
}

// TestAnUnreadableFileNeitherSpeaksNorDisturbsTheWait: half a file carries no
// information, so it must not reset a reading that is part-way through earning
// its note.
func TestAnUnreadableFileNeitherSpeaksNorDisturbsTheWait(t *testing.T) {
	rec, at := store(t), time.Now()
	rec.Settle([]awareness.Reading{reading("v2")}, at)
	half := awareness.Reading{Kind: awareness.KindPartnership, Slug: "alex", Unreadable: true}
	if docs := rec.Settle([]awareness.Reading{half}, at.Add(time.Second)); len(docs) != 0 {
		t.Fatalf("reported on a document that would not parse: %+v", docs)
	}
	if docs := rec.Settle([]awareness.Reading{reading("v2")}, at.Add(pastTheWindow)); len(docs) != 1 {
		t.Fatalf("the half-written read threw away a reading that had already held: %+v", docs)
	}
}

// TestADocumentThatComesBackIsSaidToBeBack: an absence that was reported is a
// sentence nothing else ever contradicts. A session left believing its
// partnership is gone believes it holds no delegation at all.
func TestADocumentThatComesBackIsSaidToBeBack(t *testing.T) {
	rec, at := store(t), time.Now()
	rec.Settle([]awareness.Reading{missing()}, at)
	if docs := rec.Settle([]awareness.Reading{missing()}, at.Add(pastGone)); len(docs) != 1 {
		t.Fatalf("the absence was never reported, so there is nothing to retract: %+v", docs)
	}
	back := at.Add(pastGone + time.Second)
	rec.Settle([]awareness.Reading{reading("v1")}, back)
	docs := rec.Settle([]awareness.Reading{reading("v1")}, back.Add(pastTheWindow))
	if len(docs) != 1 || !docs[0].Back {
		t.Fatalf("the session was left believing the partnership was gone: %+v", docs)
	}
	notes := awareness.Notes(awareness.Observation{Documents: docs})
	if len(notes) != 1 || !strings.Contains(notes[0].Because, "on disk again") {
		t.Fatalf("the retraction says nothing a session can act on: %+v", notes)
	}
	// And it is said once: nothing is outstanding afterwards.
	if docs := rec.Settle([]awareness.Reading{reading("v1")}, back.Add(time.Minute)); len(docs) != 0 {
		t.Fatalf("the retraction repeated: %+v", docs)
	}
}

// TestAnUnchangedDocumentNeverEntersTheRecord: silence costs nothing, including
// a write on every turn of every session.
func TestAnUnchangedDocumentNeverEntersTheRecord(t *testing.T) {
	rec, at := store(t), time.Now()
	for i := range 3 {
		if docs := rec.Settle([]awareness.Reading{reading("v1")}, at.Add(time.Duration(i)*time.Minute)); len(docs) != 0 {
			t.Fatalf("said something about a document nobody touched: %+v", docs)
		}
	}
	if n := len(rec.Seen()); n != 0 {
		t.Fatalf("a document with nothing outstanding was kept in the record: %d", n)
	}
}

// TestReDeliveryClearsWhatWasSeen: a resume or a compact hands the session the
// file as it stands, so a reading taken against the previous version is about a
// comparison that no longer exists.
func TestReDeliveryClearsWhatWasSeen(t *testing.T) {
	rec, at := store(t), time.Now()
	rec.Settle([]awareness.Reading{reading("v2")}, at)
	if n := len(rec.Seen()); n != 1 {
		t.Fatalf("the reading was not held: %d", n)
	}
	if err := rec.Record(delivery(awareness.KindPartnership, "alex", "v2", nil)); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.Seen()); n != 0 {
		t.Fatalf("a stale comparison survived the document being handed over again: %d", n)
	}
	if docs := rec.Settle([]awareness.Reading{reading("v2")}, at.Add(time.Hour)); len(docs) != 0 {
		t.Fatalf("the version the session now holds was reported as a change: %+v", docs)
	}
}

// TestADocumentNeverHandedOverIsNeverSpokenAbout.
func TestADocumentNeverHandedOverIsNeverSpokenAbout(t *testing.T) {
	rec, at := store(t), time.Now()
	stranger := awareness.Reading{Kind: awareness.KindProgram, Slug: "nobody", Digest: "x"}
	rec.Settle([]awareness.Reading{stranger}, at)
	if docs := rec.Settle([]awareness.Reading{stranger}, at.Add(time.Hour)); len(docs) != 0 {
		t.Fatalf("a change was claimed against a baseline that was never recorded: %+v", docs)
	}
}

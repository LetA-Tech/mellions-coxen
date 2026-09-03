package awareness_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

func delivery(kind, slug, digest string, sections map[string]string) awareness.Delivery {
	return awareness.Delivery{Kind: kind, Slug: slug, Digest: digest, Sections: sections}
}

func TestARecordedDeliveryIsWhatTheSessionWasHanded(t *testing.T) {
	rec := awareness.Delivered{Path: awareness.DeliveredPath(t.TempDir(), "claude", "sess-1")}
	partnership := delivery(awareness.KindPartnership, "alex", "v1", map[string]string{"Delegated": "d1"})
	if err := rec.Record(partnership); err != nil {
		t.Fatal(err)
	}
	if err := rec.Record(delivery(awareness.KindProgram, "sample", "p1", nil)); err != nil {
		t.Fatal(err)
	}
	all := rec.All()
	if len(all) != 2 {
		t.Fatalf("one document overwrote the other: %+v", all)
	}
	if got := all["partnership/alex"]; got.Digest != "v1" || got.Sections["Delegated"] != "d1" {
		t.Errorf("partnership recorded as %+v", got)
	}
	// A later delivery of the same document replaces the earlier one: a resume
	// or a compact hands the session the file as it stands now.
	if err := rec.Record(delivery(awareness.KindPartnership, "alex", "v2", nil)); err != nil {
		t.Fatal(err)
	}
	if got := rec.All()["partnership/alex"].Digest; got != "v2" {
		t.Errorf("re-delivery did not replace the baseline: %q", got)
	}
}

func TestNoSessionRecordsNothing(t *testing.T) {
	if p := awareness.DeliveredPath(t.TempDir(), "claude", ""); p != "" {
		t.Fatalf("a caller with no session got a record path: %q", p)
	}
	rec := awareness.Delivered{}
	if err := rec.Record(delivery(awareness.KindPartnership, "alex", "v1", nil)); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.All()); n != 0 {
		t.Fatalf("recorded %d deliveries with nowhere to put them", n)
	}
}

func TestADamagedRecordIsNotABaseline(t *testing.T) {
	rec := awareness.Delivered{Path: awareness.DeliveredPath(t.TempDir(), "claude", "sess-1")}
	if err := rec.Record(delivery(awareness.KindPartnership, "alex", "v1", nil)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rec.Path, []byte("{ truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.All()); n != 0 {
		t.Fatalf("half a file was read as a baseline: %d", n)
	}
}

func TestPruneDeliveredLeavesTheMemoryAlone(t *testing.T) {
	root := t.TempDir()
	old := awareness.Delivered{Path: awareness.DeliveredPath(root, "claude", "old")}
	said := awareness.Said{Path: awareness.SaidPath(root, "claude", "old")}
	if err := old.Record(delivery(awareness.KindPartnership, "alex", "v1", nil)); err != nil {
		t.Fatal(err)
	}
	if err := said.Remember([]awareness.Note{{Because: "something"}}); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-10 * 24 * time.Hour)
	for _, p := range []string{old.Path, said.Path} {
		if err := os.Chtimes(p, past, past); err != nil {
			t.Fatal(err)
		}
	}
	awareness.PruneDelivered(root, 7*24*time.Hour, time.Now())
	if _, err := os.Stat(old.Path); err == nil {
		t.Error("an old session's baseline survived")
	}
	if _, err := os.Stat(old.Path + ".lock"); err == nil {
		t.Error("the lock outlived the record it was guarding")
	}
	if _, err := os.Stat(said.Path); err != nil {
		t.Errorf("pruning baselines took the memory of what was said with it: %v", err)
	}
	if _, err := os.Stat(said.Path + ".lock"); err != nil {
		t.Errorf("pruning baselines took another store's lock: %v", err)
	}
}

// changedDoc is a partnership that was delivered as v1 and now reads otherwise.
func changedDoc(sections ...awareness.SectionStamp) awareness.Document {
	was := &awareness.Delivery{
		Kind: awareness.KindPartnership, Slug: "alex", Digest: "v1",
		Sections: map[string]string{"What is delegated to me": "d1", "Rhythm": "r1"},
	}
	return awareness.Document{
		Kind: awareness.KindPartnership, Slug: "alex", Was: was, Digest: "v2", Sections: sections,
	}
}

func TestAChangedPartnershipNamesTheSectionAndHowToReadIt(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{Documents: []awareness.Document{changedDoc(
		awareness.SectionStamp{Heading: "What is delegated to me", Prov: "DECLARED", Digest: "d2"},
		awareness.SectionStamp{Heading: "Rhythm", Prov: "DISCOVERED", Digest: "r1"},
	)}})
	if len(notes) != 1 {
		t.Fatalf("want one note, got %+v", notes)
	}
	n := notes[0]
	for _, want := range []string{"partnership with alex", "changed after it reached this session",
		`"What is delegated to me" {DECLARED}`} {
		if !strings.Contains(n.Because, want) {
			t.Errorf("note does not say %q: %q", want, n.Because)
		}
	}
	if strings.Contains(n.Because, "Rhythm") {
		t.Errorf("a section that did not change was named: %q", n.Because)
	}
	if !strings.Contains(n.Do, "mellions partner show alex") {
		t.Errorf("the note does not say how to read it: %q", n.Do)
	}
	if !strings.Contains(n.Why, "invisible to you") {
		t.Errorf("the note does not say what is lost by ignoring it: %q", n.Why)
	}
	// The changed text itself is deliberately not carried: a diff of a
	// delegation reads without the sentence that qualifies it.
	if strings.Contains(n.Because, "d2") || strings.Contains(n.Do, "d2") {
		t.Errorf("the note carried the changed content: %+v", n)
	}
}

func TestAnUnchangedDocumentSaysNothing(t *testing.T) {
	same := changedDoc(awareness.SectionStamp{Heading: "What is delegated to me", Prov: "DECLARED", Digest: "d1"})
	same.Digest = "v1"
	if n := awareness.Notes(awareness.Observation{Documents: []awareness.Document{same}}); len(n) != 0 {
		t.Fatalf("a document nobody touched was reported changed: %+v", n)
	}
}

func TestADocumentThatNeverReachedThisSessionSaysNothing(t *testing.T) {
	d := changedDoc(awareness.SectionStamp{Heading: "What is delegated to me", Prov: "DECLARED", Digest: "d2"})
	d.Was = nil
	if n := awareness.Notes(awareness.Observation{Documents: []awareness.Document{d}}); len(n) != 0 {
		t.Fatalf("a change was claimed against a baseline that does not exist: %+v", n)
	}
}

func TestAGovernedDocumentComesBeforeEverythingElse(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{
		Idle:      true,
		Documents: []awareness.Document{changedDoc(awareness.SectionStamp{Heading: "X", Prov: "DECLARED", Digest: "x"})},
		Others:    []awareness.Peer{{Describe: "claude session ab12 on r (b)"}},
	})
	if len(notes) != 3 {
		t.Fatalf("want 3 notes, got %+v", notes)
	}
	if !strings.Contains(notes[0].Because, "partnership with alex") {
		t.Errorf("the document change was not first: %q", notes[0].Because)
	}
}

func TestASecondChangeIsANewFactAndTheSameChangeIsNot(t *testing.T) {
	root := t.TempDir()
	said := awareness.Said{Path: awareness.SaidPath(root, "claude", "sess-1")}
	first := awareness.Notes(awareness.Observation{Documents: []awareness.Document{changedDoc(
		awareness.SectionStamp{Heading: "What is delegated to me", Prov: "DECLARED", Digest: "d2"})}})
	if fresh := said.Fresh(first); len(fresh) != 1 {
		t.Fatalf("first delivery: %+v", fresh)
	} else if err := said.Remember(fresh); err != nil {
		t.Fatal(err)
	}
	if again := said.Fresh(first); len(again) != 0 {
		t.Fatalf("the same change was said twice: %+v", again)
	}
	// The same sentence about a later version is a different fact, and must not
	// be swallowed by the memory of the first.
	later := changedDoc(awareness.SectionStamp{Heading: "What is delegated to me", Prov: "DECLARED", Digest: "d3"})
	later.Digest = "v3"
	next := awareness.Notes(awareness.Observation{Documents: []awareness.Document{later}})
	if next[0].Because != first[0].Because {
		t.Fatalf("this test no longer exercises the collision it was written for:\n%q\n%q",
			next[0].Because, first[0].Because)
	}
	if fresh := said.Fresh(next); len(fresh) != 1 {
		t.Fatalf("a second change to the same section was never delivered: %+v", fresh)
	}
}

func TestADocumentThatWentAwaySaysSoAndSaysWhereToLook(t *testing.T) {
	gone := changedDoc()
	gone.Missing, gone.Digest, gone.Sections = true, "", nil
	notes := awareness.Notes(awareness.Observation{Documents: []awareness.Document{gone}})
	if len(notes) != 1 || !strings.Contains(notes[0].Because, "no longer on disk") {
		t.Fatalf("got %+v", notes)
	}
	if !strings.Contains(notes[0].Do, "mellions partner list") {
		t.Errorf("the note does not say where to look: %q", notes[0].Do)
	}
}

func TestAChangedProgramIsAboutWhatTheWorkIsFor(t *testing.T) {
	d := awareness.Document{
		Kind: awareness.KindProgram, Slug: "sample-platform", Digest: "v2",
		Was:      &awareness.Delivery{Kind: awareness.KindProgram, Slug: "sample-platform", Digest: "v1"},
		Sections: []awareness.SectionStamp{{Heading: "Correctness", Prov: "DECLARED", Digest: "c2"}},
	}
	notes := awareness.Notes(awareness.Observation{Documents: []awareness.Document{d}})
	if len(notes) != 1 {
		t.Fatalf("got %+v", notes)
	}
	if !strings.Contains(notes[0].Because, "program sample-platform") {
		t.Errorf("wrong noun: %q", notes[0].Because)
	}
	if !strings.Contains(notes[0].Do, "mellions program show sample-platform") {
		t.Errorf("wrong command: %q", notes[0].Do)
	}
	if !strings.Contains(notes[0].Why, "correctness bar") {
		t.Errorf("a program note reads like a partnership one: %q", notes[0].Why)
	}
}

func TestManySectionsStayReadable(t *testing.T) {
	var now []awareness.SectionStamp
	for _, h := range []string{"A", "B", "C", "D", "E"} {
		now = append(now, awareness.SectionStamp{Heading: h, Prov: "DISCOVERED", Digest: h})
	}
	d := changedDoc(now...)
	d.Was.Sections = map[string]string{}
	notes := awareness.Notes(awareness.Observation{Documents: []awareness.Document{d}})
	if !strings.Contains(notes[0].Because, "and 2 more") {
		t.Errorf("an unbounded list reached the session: %q", notes[0].Because)
	}
	if strings.Contains(notes[0].Because, `"E"`) {
		t.Errorf("the list was not cut: %q", notes[0].Because)
	}
}

func TestASectionTheOwnerRemovedIsNamed(t *testing.T) {
	d := changedDoc(awareness.SectionStamp{Heading: "What is delegated to me", Prov: "DECLARED", Digest: "d1"})
	notes := awareness.Notes(awareness.Observation{Documents: []awareness.Document{d}})
	if len(notes) != 1 || !strings.Contains(notes[0].Because, `"Rhythm" {gone}`) {
		t.Fatalf("a removed section went unmentioned: %+v", notes)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness

import (
	"maps"
	"slices"
	"time"
)

// A governing document is read with no lock against whoever is writing it, so a
// single reading is a sample of a moment rather than a fact about the file.
// Every ordinary way of saving one has a window in which the reading is a lie:
//
//   - a rename-style save — an editor's default, `rm && cp`, a deploy — has a
//     window with no file at all, which reads exactly like the owner deleting it;
//   - an in-place rewrite has windows holding a prefix of the new content, which
//     can parse, digest differently from what was delivered, and appear to have
//     lost every section the writer has not reached yet.
//
// Both are the same mistake: one unsynchronised observation treated as truth. So
// a reading earns a note only by being stable — the same state seen again after
// the window below, with the file itself untouched throughout. A writer is not
// asked to cooperate and none of this can deadlock; the cost is that a real
// change is announced on the first observation a settle window after the save,
// which in a working session is the next tool call.
const (
	// settleWindow is how long a changed document must read the same way.
	// Three orders of magnitude beyond the millisecond-scale window a local
	// save leaves open, and short enough that the next tool call clears it.
	settleWindow = 3 * time.Second
	// settleGone is the same for a document that has disappeared, which is
	// held to a longer window because it is the costlier thing to be wrong
	// about: a session told its partnership is gone believes it holds no
	// delegation at all, and the absence a copy leaves behind is exactly the
	// state this must not mistake for a deletion.
	settleGone = 30 * time.Second
)

// gone is the sighting state of a document that was not there when it was read.
const gone = "gone"

// Reading is one look at a governing document this session holds.
type Reading struct {
	// Kind and Slug are which document, keyed as the delivery was.
	Kind, Slug string
	// Missing says the file was not there.
	Missing bool
	// Unreadable says the file was there and could not be parsed. It carries no
	// information about the document — a half-written file is what a hook firing
	// during a save sees — so it neither says anything nor disturbs what is
	// already being waited on.
	Unreadable bool
	// Digest and Sections are the document as this reading found it.
	Digest   string
	Sections []SectionStamp
	// ModTime is when the file was last written, zero where it could not be
	// established. A file modified inside the settle window is one somebody may
	// still be writing, whatever two readings of it agreed on.
	ModTime time.Time
}

// Sighting is a state of a governing document that has been seen but has not
// yet earned a note.
type Sighting struct {
	// State is the digest that was read, or gone.
	State string `json:"state"`
	// First is when this state was first seen. It does not move while the state
	// holds, so the wait is against the first sighting rather than the last.
	First time.Time `json:"first"`
	// Told is the state the session was last told about, so a document coming
	// back can be told after an absence was reported, and only then.
	Told string `json:"told,omitempty"`
}

func (r Reading) key() string { return r.Kind + "/" + r.Slug }

func (r Reading) state() string {
	if r.Missing {
		return gone
	}
	return r.Digest
}

// settle decides, for each document this session was handed, whether a fresh
// reading of it is stable enough to be spoken about, and what is left waiting.
//
// Pure, and the whole decision: what is on disk arrives as readings and what is
// remembered arrives as sightings, so every window this exists to close can be
// driven from a test without a filesystem or a clock.
func settle(was map[string]Delivery, seen map[string]Sighting, now []Reading, at time.Time) (map[string]Sighting, []Document) {
	next := map[string]Sighting{}
	maps.Copy(next, seen)
	var docs []Document

	for _, r := range now {
		key := r.key()
		d, handed := was[key]
		if !handed {
			// Nothing was recorded as delivered, so there is no version to
			// compare against and a note would be a guess.
			delete(next, key)
			continue
		}
		if r.Unreadable {
			continue
		}
		st := r.state()
		sight, waiting := next[key]
		if !waiting || sight.State != st {
			sight = Sighting{State: st, First: at, Told: sight.Told}
		}
		next[key] = sight

		if !confirmed(sight, r, at) {
			continue
		}
		doc := Document{Kind: r.Kind, Slug: r.Slug, Was: &d}
		switch {
		case st == gone:
			doc.Missing = true
		case st == d.Digest:
			// Back to the version this session holds. Worth a note only where
			// the session was told it had gone, which it would otherwise go on
			// believing: nothing else ever contradicts that sentence.
			if sight.Told != gone {
				continue
			}
			doc.Digest, doc.Sections, doc.Back = r.Digest, r.Sections, true
		default:
			doc.Digest, doc.Sections = r.Digest, r.Sections
		}
		sight.Told = st
		next[key] = sight
		docs = append(docs, doc)
	}

	// A document with nothing outstanding is not worth a line in the record.
	for key, s := range next {
		d, handed := was[key]
		if !handed || (s.State == d.Digest && (s.Told == "" || s.Told == d.Digest)) {
			delete(next, key)
		}
	}
	if len(next) == 0 {
		return nil, docs
	}
	return next, docs
}

// confirmed says the reading may be spoken about: the same state has held for
// the settle window, and the file was not written during it.
func confirmed(s Sighting, r Reading, at time.Time) bool {
	window := settleWindow
	if s.State == gone {
		window = settleGone
	}
	if at.Sub(s.First) < window {
		return false
	}
	if !r.ModTime.IsZero() && at.Sub(r.ModTime) < window {
		return false
	}
	return true
}

func sightingsEqual(a, b map[string]Sighting) bool {
	if len(a) != len(b) {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(a)) {
		y, ok := b[k]
		if !ok || a[k] != y {
			return false
		}
	}
	return true
}

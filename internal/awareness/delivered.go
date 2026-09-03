// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/durable"
)

// Delivered is the governing documents as they reached one session.
//
// A partnership and a program are handed to a session once, by the session-start
// hooks, and nothing between two starts reads either file again. A session that
// outlives an edit to one of them therefore holds a document the owner has
// already changed, and cannot tell: what it holds reads exactly like what is on
// disk. That is how a session keeps acting on a delegation it no longer has, or
// misses one it was just given.
//
// Establishing that needs one fact a session cannot infer — what actually
// reached it. It is recorded at the moment the document is rendered rather than
// at session start, so the record can never claim a delivery that did not
// happen, and a resume or a compact re-delivering the current file re-stamps it.
type Delivered struct {
	// Path is the file for one session. Empty disables the record entirely,
	// which means no change can be established and nothing is ever said — the
	// right behaviour for a caller with no session to key on.
	Path string
}

// Delivery is one governing document as it reached a session.
type Delivery struct {
	// Kind is the noun for the document: KindPartnership or KindProgram.
	Kind string `json:"kind"`
	// Slug names which one, and completes the command that renders it.
	Slug string `json:"slug"`
	// Digest identifies the whole document as delivered.
	Digest string `json:"digest"`
	// Sections is heading to digest, so a later comparison can name what
	// changed rather than only that something did.
	Sections map[string]string `json:"sections,omitempty"`
}

// state is the whole per-session record: what was handed over, and what has
// been seen on disk since that has not yet earned a note.
type state struct {
	Delivered map[string]Delivery `json:"delivered,omitempty"`
	Seen      map[string]Sighting `json:"seen,omitempty"`
}

// The two governing documents Mellions owns and the owner edits in place. The
// identity is deliberately not one of them: it ships inside the plugin and
// changes only when somebody runs `mellions install`, which is a delivery of a
// new installation rather than an edit under a running session.
const (
	KindPartnership = "partnership"
	KindProgram     = "program"
)

// DeliveredPath is where one session's record of what it was handed lives.
//
// Keyed on the session, like the memory of what it has been told, and for the
// same reason: one conversation is several hook firings and a clock-keyed name
// would make every one of them a session that was handed nothing.
func DeliveredPath(root, runtime, session string) string {
	if root == "" || session == "" {
		return ""
	}
	name := sanitize(runtime) + "-" + sanitize(session) + ".delivered"
	return filepath.Join(root, "awareness", name)
}

// Digest identifies a version of some content.
//
// Truncated: this separates two versions of one file the owner wrote, not a
// forgery from an original.
func Digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func (d Delivery) key() string { return d.Kind + "/" + d.Slug }

// Record stores a document as it was rendered, replacing any earlier version.
//
// Recording a delivery also clears whatever was seen on disk before it: the
// session now holds this version, so any reading taken against the previous one
// is about a comparison that no longer exists.
func (d Delivered) Record(v Delivery) error {
	if d.Path == "" || v.Kind == "" || v.Slug == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(d.Path), 0o755); err != nil {
		return err
	}
	// Guarded because the session-start hooks render several documents at once
	// and a lost update here silently drops one document from the record — the
	// same failure as never recording it, and just as invisible.
	return durable.Guard(d.Path, func() error {
		s := d.load()
		if s.Delivered == nil {
			s.Delivered = map[string]Delivery{}
		}
		s.Delivered[v.key()] = v
		delete(s.Seen, v.key())
		return d.write(s)
	})
}

// Settle folds fresh readings of the documents this session holds into the
// record and returns the ones a note may be made about — only those whose
// state has been the same for at least the settle window.
//
// The whole read-modify-write is one operation across processes, because the
// prompt hook and a tool-call hook can run against the same session at once.
func (d Delivered) Settle(now []Reading, at time.Time) []Document {
	if d.Path == "" {
		return nil
	}
	var docs []Document
	_ = durable.Guard(d.Path, func() error {
		s := d.load()
		var seen map[string]Sighting
		seen, docs = settle(s.Delivered, s.Seen, now, at)
		if sightingsEqual(s.Seen, seen) {
			return nil
		}
		s.Seen = seen
		return d.write(s)
	})
	return docs
}

// All is every document this session was handed, keyed by kind and slug.
func (d Delivered) All() map[string]Delivery {
	if s := d.load(); s.Delivered != nil {
		return s.Delivered
	}
	return map[string]Delivery{}
}

// Seen is every sighting this session is still waiting to confirm, keyed the
// same way. Nothing outside a test needs it; it is here so the waiting can be
// asserted on rather than inferred from what was not said.
func (d Delivered) Seen() map[string]Sighting {
	if s := d.load(); s.Seen != nil {
		return s.Seen
	}
	return map[string]Sighting{}
}

func (d Delivered) load() state {
	var s state
	if d.Path == "" {
		return s
	}
	raw, err := os.ReadFile(d.Path)
	if err != nil {
		return state{}
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return state{}
	}
	return s
}

// write replaces the record atomically and durably. A reader on another process
// sees all of the old record or all of the new one; a half-written record reads
// back as a session that was handed nothing, which is silence about every
// document at once.
func (d Delivered) write(s state) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return durable.Write(d.Path, append(raw, '\n'), 0o644)
}

// PruneDelivered removes the record of sessions that ended long enough ago that
// nothing is left to compare against. Best effort, like the memory beside it: a
// file left behind costs a few hundred bytes, and failing a hook over one would
// be the wrong trade.
func PruneDelivered(root string, olderThan time.Duration, now time.Time) {
	pruneSuffix(root, ".delivered", olderThan, now)
}

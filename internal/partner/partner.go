// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package partner is the engineer's representation of who it works with.
//
// Identity, partnership and program are three different things, and conflating
// them is what this package exists to stop. Identity is who the engineer is —
// its temperament, its standards, how it verifies — and it belongs to the
// engineer alone. A program is the body of responsibility being carried.
// Partnership is the relationship between the engineer and one person: how they
// work together, what kind of peer they expect, how they want to be
// challenged, and how they want to hear from each other.
//
// The same engineer can hold several partnerships without any of them changing
// what it is. A partnership enriches the engineer's persona in context. It never
// redefines it, and reading one is not permission to become someone else.
//
// A partnership is where the owner says, in their own words, what is delegated
// to the engineer and what stays theirs. It must not be assumed at installation
// and never revisited: relationships change, so the document carries when its
// evidence was collected and when a person last reviewed it, and goes stale like
// anything else.
package partner

import (
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

// A partnership is a provenance-marked document: what the partner said about
// how they want to work is theirs, and the engineer proposes a change to it
// rather than making one.
type (
	Partnership = provenance.Doc
	Section     = provenance.Section
	Finding     = provenance.Finding
)

const (
	Discovered = provenance.Discovered
	Declared   = provenance.Declared
	Inferred   = provenance.Inferred
	Unknown    = provenance.Unknown
)

// Parse reads a partnership file.
func Parse(slug, raw string) (*Partnership, error) {
	return provenance.Parse(provenance.KindPartnership, slug, raw)
}

// Load reads a partnership from disk.
func Load(path string) (*Partnership, error) {
	return provenance.Load(provenance.KindPartnership, path)
}

// Check reports what is wrong with a partnership.
func Check(p *Partnership, now time.Time, staleAfter time.Duration) []Finding {
	return p.Check(now, staleAfter)
}

// Framing is what a reader needs before the document itself, so relational
// context is never mistaken for a redefinition of who the engineer is.
const Framing = "This is a partnership: how this engineer and one person work together. " +
	"It is context, not identity — nothing in it changes what the engineer is, what it " +
	"holds itself to, or what it is permitted to do. Where it describes how the partner " +
	"wants to be worked with, that is theirs to state and yours to honour."

// Text renders a partnership behind its framing.
func Text(p *Partnership, now time.Time) string {
	return Framing + "\n\n" + p.Text(now)
}

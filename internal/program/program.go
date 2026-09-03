// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package program is the engineer's representation of what it is responsible for.
//
// A program is one coherent body of engineering responsibility — a purpose, a
// boundary, and the repositories that serve it. It is discovered rather than
// written, because requiring the owner to describe their own environment before
// the engineer can be useful contradicts the rule that unknown is not an
// escalation and investigation comes first.
//
// What the engineer cannot discover is intent. It can establish that a
// repository has not changed in six months; it cannot establish whether that
// means abandoned, finished, or deliberately frozen. So a program is a
// provenance-marked document, and the rules for one live in internal/provenance
// — including that what the owner declared is theirs to change, and the
// engineer's only to propose.
//
// A program says what responsibility is being carried. It says nothing about
// who the engineer is, which is agents/mellions.md, or who it works with, which
// is internal/partner.
package program

import (
	"errors"

	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

// A program is a provenance-marked document. These aliases keep that an
// implementation detail at the call sites, which care about programs.
type (
	Program    = provenance.Doc
	Section    = provenance.Section
	Provenance = provenance.Provenance
	Finding    = provenance.Finding
)

const (
	Discovered = provenance.Discovered
	Declared   = provenance.Declared
	Inferred   = provenance.Inferred
	Unknown    = provenance.Unknown
)

// Parse reads a program file.
func Parse(slug, raw string) (*Program, error) {
	return provenance.Parse(provenance.KindProgram, slug, raw)
}

// Load reads a program from disk.
func Load(path string) (*Program, error) {
	return provenance.Load(provenance.KindProgram, path)
}

// ErrNoProgram reports that no program has been discovered yet.
//
// A distinct error because it has a remedy the engineer can run itself, and
// telling it so is the difference between a blocked run and a first step.
var ErrNoProgram = errors.New("no program adopted yet — run `mellions program discover`")

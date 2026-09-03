// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"errors"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

// noteDelivery records a governing document as it reached a session, at the
// moment it is rendered.
//
// Here rather than in the hook that calls it, because this is where the bytes
// actually go to the session: a record written anywhere else could claim a
// delivery that never happened, and the session would then be told nothing when
// the document later changed. Silent when there is no session to key on — a
// person running `mellions partner show` at a terminal has no stale copy to
// worry about.
//
// Before the document is printed rather than after, because the hook bounds its
// own output and a bound reached closes the pipe under the process still writing
// to it. Recording after that would lose the baseline for whichever document was
// truncated, which is silence about every later change to it; recording before
// it can at worst claim a delivery the session only partly saw, which costs one
// note telling it to read a document it should read anyway.
func noteDelivery(cfg *Config, kind string, d *provenance.Doc) {
	session := hookSession()
	if session == "" || d == nil {
		return
	}
	rec := awareness.Delivered{Path: awareness.DeliveredPath(cfg.reportRoot(), runtimeName(), session)}
	_ = rec.Record(awareness.Delivery{
		Kind:     kind,
		Slug:     d.Slug,
		Digest:   awareness.Digest(d.Raw),
		Sections: sectionDigests(d),
	})
}

// hookSession is the session a hook is rendering for, and nothing at all when
// the caller is not a hook.
//
// `partner show` and `program show` are commands a person, a script, a cron job
// and CI run. Reading stdin unconditionally to find a session id makes every one
// of those block on a pipe nobody is going to close, and swallows the rest of a
// `while read` loop's input. The hooks say who they are: hooks/lib.sh sets this
// before it hands the runtime's payload over.
func hookSession() string {
	if os.Getenv("MELLIONS_HOOK") == "" {
		return ""
	}
	session, _ := hookContext(os.Stdin)
	return session
}

func sectionDigests(d *provenance.Doc) map[string]string {
	out := make(map[string]string, len(d.Sections))
	for _, s := range d.Sections {
		out[s.Heading] = awareness.Digest(s.Body)
	}
	return out
}

// governingDocuments is every document this session was handed that has changed
// on disk in a way stable enough to say so. Only what a delivery was recorded
// for: without knowing what reached the session there is no change to establish,
// and guessing one is worse than saying nothing.
//
// The reading is a sample, never a verdict — Settle holds it against the last
// one before anything is said.
func governingDocuments(cfg *Config, runtime, session string) []awareness.Document {
	if session == "" {
		return nil
	}
	rec := awareness.Delivered{Path: awareness.DeliveredPath(cfg.reportRoot(), runtime, session)}
	was := rec.All()
	if len(was) == 0 {
		return nil
	}
	var now []awareness.Reading
	for _, key := range slices.Sorted(maps.Keys(was)) {
		v := was[key]
		var path string
		var kind provenance.Kind
		switch v.Kind {
		case awareness.KindPartnership:
			path, kind = cfg.partnerPath(v.Slug), provenance.KindPartnership
		case awareness.KindProgram:
			path, kind = cfg.programPath(v.Slug), provenance.KindProgram
		default:
			continue
		}
		now = append(now, readDocument(kind, v.Kind, v.Slug, path))
	}
	return rec.Settle(now, time.Now())
}

// readDocument is one look at a governing document on disk, reported as what it
// found rather than as what is true.
func readDocument(kind provenance.Kind, docKind, slug, path string) awareness.Reading {
	r := awareness.Reading{Kind: docKind, Slug: slug}
	if info, err := os.Stat(path); err == nil {
		r.ModTime = info.ModTime()
	}
	d, err := provenance.Load(kind, path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		r.Missing = true
	case err != nil:
		// A document that does not parse is one the engineer cannot report on
		// honestly, and a half-written file is what a hook firing during a save
		// sees. It carries no information either way.
		r.Unreadable = true
	default:
		r.Digest, r.Sections = awareness.Digest(d.Raw), sectionStamps(d)
	}
	return r
}

func sectionStamps(d *provenance.Doc) []awareness.SectionStamp {
	out := make([]awareness.SectionStamp, 0, len(d.Sections))
	for _, s := range d.Sections {
		out = append(out, awareness.SectionStamp{
			Heading: s.Heading, Prov: string(s.Prov), Digest: awareness.Digest(s.Body),
		})
	}
	return out
}

// runtimeName is which runtime is asking, so one session's record is keyed the
// same way wherever it is written.
func runtimeName() string {
	if r := os.Getenv("MELLIONS_RUNTIME"); r != "" {
		return r
	}
	return "claude"
}

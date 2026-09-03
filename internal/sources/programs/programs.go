// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package programs carries the engineer's statement of what it is responsible
// for into a survey.
//
// A program is discovered rather than handed down — see internal/program and
// docs/program.md. This source only reports what exists, including the two
// states that matter most to a reader: no program at all, which has a remedy,
// and an unadopted draft, which must never be mistaken for owner intent.
//
// Nothing here interprets a program, ranks its sections, or infers priority from
// their order. It is carried to the model with its provenance intact.
package programs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/program"
	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Options configures the source.
type Options struct {
	// Path is a single legacy program file, read when present.
	Path string
	// Dir holds one file per program, which is the current shape.
	Dir string
}

// Source reads program responsibility from a markdown file.
type Source struct{ opts Options }

// New returns a configured source.
func New(o Options) *Source { return &Source{opts: o} }

// Name implements signal.Source.
func (s *Source) Name() string { return "programs" }

// Collect emits one signal per program, with its provenance intact.
//
// Having no program is NOT an error. It used to be, on the reasoning that an
// engineer with no statement of responsibility should not choose its own work —
// but that was written when a program could only be handed down. A program is
// discovered now, so the absence of one has a remedy the engineer can run
// itself, and reporting the remedy is more useful than refusing to start.
//
// An unadopted draft is reported as a draft. A draft read as owner intent is
// worse than no program at all: it looks authoritative and nobody checked it.
func (s *Source) Collect(_ context.Context, _ signal.Scope) ([]signal.Signal, error) {
	paths, err := s.paths()
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return []signal.Signal{{
			Kind: signal.KindObjective, Source: "programs",
			ID: "no-program", Title: "No engineering program has been discovered yet",
			Updated: time.Now(),
			Attrs:   map[string]string{"remedy": "mellions program discover"},
			Detail: "Nothing states what this work is for, so nothing here can be judged " +
				"valuable or not. Run `mellions program discover`, draft a program from the " +
				"evidence, and leave the owner's sections as questions for them.",
		}}, nil
	}

	var out []signal.Signal
	for _, path := range paths {
		p, err := program.Load(path)
		if err != nil {
			// A file written before provenance existed is not malformed, it is
			// old. Refusing to start because the format moved is the same
			// fail-closed mistake the authority gate made: the remedy is to
			// re-discover, and saying so is more use than an error.
			if legacy, lerr := os.ReadFile(path); lerr == nil {
				out = append(out, signal.Signal{
					Kind: signal.KindObjective, Source: "programs",
					ID: "legacy:" + filepath.Base(path), Updated: time.Now(),
					Title: "A program file predates provenance marking",
					Attrs: map[string]string{
						"file":   path,
						"remedy": "mellions program discover",
						"parse":  err.Error(),
					},
					Detail: "Nothing in this file is attributed, so none of it can be told apart " +
						"as discovered fact, owner intent or somebody's reading. Re-run discovery " +
						"and rewrite it with provenance. Until then, treat every line below as " +
						"unverified.\n\n" + string(legacy),
				})
				continue
			}
			return nil, fmt.Errorf("programs: %w", err)
		}
		info, _ := os.Stat(path)
		attrs := map[string]string{"file": path, "sections": strconv.Itoa(len(p.Sections))}
		state := "adopted"
		if p.Adopted == "" {
			state = "DRAFT — not reviewed by the owner; its DECLARED sections are prompts, not intent"
		}
		attrs["state"] = state
		if !p.DiscoveredAt.IsZero() {
			attrs["evidence_age"] = fmt.Sprintf("%dd", int(time.Since(p.DiscoveredAt).Hours()/24))
		}
		if f := p.Check(time.Now(), 45*24*time.Hour); len(f) > 0 {
			attrs["findings"] = strconv.Itoa(len(f))
		}

		title := p.Title
		if title == "" {
			title = p.Slug
		}
		var mod time.Time
		if info != nil {
			mod = info.ModTime()
		}
		out = append(out, signal.Signal{
			Kind: signal.KindObjective, Source: "programs",
			ID: p.Slug, Title: title, Updated: mod,
			Attrs: attrs, Detail: p.Text(time.Now()),
		})
	}
	return out, nil
}

// paths lists the program files, preferring the per-program directory.
func (s *Source) paths() ([]string, error) {
	if s.opts.Dir != "" {
		entries, err := os.ReadDir(s.opts.Dir)
		if err == nil {
			var out []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					out = append(out, filepath.Join(s.opts.Dir, e.Name()))
				}
			}
			sort.Strings(out)
			if len(out) > 0 {
				return out, nil
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("programs: read %s: %w", s.opts.Dir, err)
		}
	}
	if s.opts.Path != "" {
		if _, err := os.Stat(s.opts.Path); err == nil {
			return []string{s.opts.Path}, nil
		}
	}
	return nil, nil
}

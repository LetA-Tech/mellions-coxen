// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package provenance is the document machinery behind everything the engineer
// holds that mixes established fact with somebody else's intent.
//
// Two documents use it today. A program says what body of engineering
// responsibility is being carried; a partnership says who the engineer is
// working with and how. They are different subjects with one shared problem:
// part of the content is discoverable and part of it can only be declared by a
// person, and a reader cannot tell those apart once they are adjacent
// paragraphs under one heading.
//
// So every section carries its provenance, and provenance says how far to
// trust it and whose it is to change: what a person declared is theirs, and the
// engineer proposes a change to it rather than making one.
package provenance

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Kind is what sort of document this is, used where a reader needs the noun.
//
// It changes wording and nothing else. The rules — one provenance per section,
// evidence in a discovered section, declared content untouchable — are the same
// for every kind, because the reason for them does not depend on the subject.
type Kind string

const (
	// KindProgram is a statement of engineering responsibility.
	KindProgram Kind = "Program"
	// KindPartnership is a working relationship with one person.
	KindPartnership Kind = "Partnership"
)

func (k Kind) noun() string { return strings.ToLower(string(k)) }

// declaredBy names whose the DECLARED sections are, which differs by kind: a
// program's intent is the owner's, a partnership's is the partner's own account
// of how they want to work.
func (k Kind) declaredBy() string {
	if k == KindPartnership {
		return "the partner's"
	}
	return "the owner's"
}

// Provenance says where a section's content came from, and therefore how far to
// trust it and who owns it.
type Provenance string

const (
	// Discovered is established from evidence, with citations, as of a date.
	Discovered Provenance = "DISCOVERED"
	// Declared is a person's intent. Authoritative, and not the engineer's to change.
	Declared Provenance = "DECLARED"
	// Inferred is the engineer's reading: supported by evidence, not established
	// by it. Marked so it can never be mistaken for a fact.
	Inferred Provenance = "INFERRED"
	// Unknown is a named gap — what discovery could not settle, and what would.
	Unknown Provenance = "UNKNOWN"
)

var provenances = []Provenance{Discovered, Declared, Inferred, Unknown}

// Section is one part of a document, with exactly one provenance.
//
// Exactly one, because mixing them is how a guess becomes a fact: a reading of
// the code and a decision about priorities read identically once they are
// adjacent paragraphs under one heading.
type Section struct {
	Heading string
	Prov    Provenance
	Body    string
	// Repos is this section's own declared boundary, from a `repos:` line
	// directly under the heading. Empty means undeclared, and an undeclared
	// section is about every repository.
	//
	// The default is the opposite of the document's on purpose. A whole
	// document is standing context whose safe default is absent; a section
	// inside one the owner wrote is content, and dropping what they delegated
	// because they did not annotate it is the dangerous direction — a session
	// that cannot see its own authority either over-asks or over-acts.
	Repos []string
}

// Covers reports whether this section is about the repository. A section that
// declared no boundary covers all of them.
func (s Section) Covers(repo string) bool {
	if len(s.Repos) == 0 {
		return true
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return false
	}
	for _, name := range s.Repos {
		if strings.EqualFold(strings.TrimSpace(name), repo) {
			return true
		}
	}
	return false
}

// Doc is one provenance-marked document.
type Doc struct {
	Kind Kind
	Slug string
	// Title is the first heading, as written.
	Title string
	// DiscoveredAt is when the evidence behind it was collected.
	DiscoveredAt time.Time
	// Adopted records who accepted it and when, empty for an unadopted draft.
	Adopted string
	// Repos is the declared boundary: the repositories this document is about,
	// as named on a `repos:` line. Empty means the boundary has not been
	// declared, which is different from declaring it empty — a document that
	// has not said what it covers cannot be asserted over any repository.
	Repos    []string
	Sections []Section
	// Raw is the file as parsed, so a rewrite can preserve anything this model
	// does not represent.
	Raw string
}

var (
	headingRe = regexp.MustCompile(`^##\s+(.+?)\s*\{([A-Z]+)\}\s*$`)
	titleRe   = regexp.MustCompile(`^#\s+(?:[A-Za-z]+:\s*)?(.+?)\s*$`)
	metaRe    = regexp.MustCompile(`^(discovered|adopted|repos):\s*(.+?)\s*$`)
)

// Parse reads a provenance-marked document.
func Parse(kind Kind, slug, raw string) (*Doc, error) {
	d := &Doc{Kind: kind, Slug: slug, Raw: raw}
	var cur *Section
	var body strings.Builder

	flush := func() {
		if cur == nil {
			return
		}
		cur.Body = strings.TrimSpace(body.String())
		d.Sections = append(d.Sections, *cur)
		body.Reset()
	}

	for line := range strings.SplitSeq(raw, "\n") {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Section{Heading: m[1], Prov: Provenance(m[2])}
			continue
		}
		if cur == nil {
			if m := titleRe.FindStringSubmatch(line); m != nil && d.Title == "" {
				d.Title = m[1]
				continue
			}
			if m := metaRe.FindStringSubmatch(line); m != nil {
				switch m[1] {
				case "discovered":
					d.DiscoveredAt, _ = time.Parse(time.RFC3339, m[2])
				case "adopted":
					d.Adopted = m[2]
				case "repos":
					for _, name := range strings.Split(m[2], ",") {
						if name = strings.TrimSpace(name); name != "" {
							d.Repos = append(d.Repos, name)
						}
					}
				}
				continue
			}
			// An untagged `## ` heading is the one parse failure worth
			// refusing: a section with no provenance is a section whose
			// authority and trustworthiness are both undefined.
			if h, ok := strings.CutPrefix(line, "## "); ok {
				return nil, fmt.Errorf("%s %s: section %q has no provenance tag; "+
					"every section must be marked {DISCOVERED}, {DECLARED}, {INFERRED} or {UNKNOWN}",
					kind.noun(), slug, strings.TrimSpace(h))
			}
			continue
		}
		// A `repos:` line under a heading declares that section's boundary.
		// Only before any prose, so the word at the start of a sentence deep
		// in a section is not read as a directive.
		if strings.TrimSpace(body.String()) == "" {
			if m := metaRe.FindStringSubmatch(line); m != nil && m[1] == "repos" {
				for _, name := range strings.Split(m[2], ",") {
					if name = strings.TrimSpace(name); name != "" {
						cur.Repos = append(cur.Repos, name)
					}
				}
				continue
			}
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	flush()

	if len(d.Sections) == 0 {
		return nil, fmt.Errorf("%s %s: no sections", kind.noun(), slug)
	}
	for _, s := range d.Sections {
		if !validProvenance(s.Prov) {
			return nil, fmt.Errorf("%s %s: section %q has unknown provenance %q; "+
				"must be one of %s", kind.noun(), slug, s.Heading, s.Prov, join(provenances))
		}
	}
	return d, nil
}

// Load reads a document from disk.
func Load(kind Kind, path string) (*Doc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: read %s: %w", kind.noun(), path, err)
	}
	return Parse(kind, slugOf(path), string(raw))
}

// Of returns the sections with a given provenance.
func (d *Doc) Of(prov Provenance) []Section {
	var out []Section
	for _, s := range d.Sections {
		if s.Prov == prov {
			out = append(out, s)
		}
	}
	return out
}

// Here is this document with the sections that declared a boundary excluding
// this repository left out, and everything else as it stands.
//
// A partnership belongs in every session — what somebody delegated is what a
// session may do without asking, and it cannot be withheld on the grounds that
// the repository is different. Only the parts that named their own scope narrow.
func (d *Doc) Here(repo string) *Doc {
	if d == nil {
		return nil
	}
	narrowed := *d
	narrowed.Sections = nil
	for _, s := range d.Sections {
		if s.Covers(repo) {
			narrowed.Sections = append(narrowed.Sections, s)
		}
	}
	return &narrowed
}

// Age is how long since the evidence behind this document was collected.
func (d *Doc) Age(now time.Time) time.Duration {
	if d.DiscoveredAt.IsZero() {
		return 0
	}
	return now.Sub(d.DiscoveredAt)
}

// Finding is one thing wrong with a document.
type Finding struct {
	Section string
	Detail  string
}

func (f Finding) String() string {
	if f.Section == "" {
		return f.Detail
	}
	return f.Section + ": " + f.Detail
}

// citation matches the forms evidence actually takes: a path, an issue, a
// commit, or a date. A date counts because not every established fact lives at
// a line of code — when a person last committed, or what a database held on a
// given day, is checkable precisely because it is stamped.
var citation = regexp.MustCompile(
	`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+(?::\d+)?|#\d+|\b[0-9a-f]{7,40}\b|\b\d{4}-\d{2}-\d{2}\b`)

// resolution is what an UNKNOWN section has to contain: some statement of what
// would settle the gap, in any of the ways drafts actually phrase it. The
// check is for a clause, not a word — one literal word passed sections that
// closed nothing and failed sections that closed every gap in other terms.
var resolution = regexp.MustCompile(`(?i)\bwould\b|\bsettl(e|es|ed|ing)\b|\b(resolved|answered|decided|established) by\b`)

// Check reports what is wrong with a document.
//
// The rule that matters: a DISCOVERED section with no evidence in it is an
// INFERRED section wearing the wrong label, and that mislabelling is exactly how
// a reading becomes a fact nobody re-checks.
func (d *Doc) Check(now time.Time, staleAfter time.Duration) []Finding {
	var out []Finding

	seen := map[Provenance]bool{}
	for _, s := range d.Sections {
		seen[s.Prov] = true
		body := strings.TrimSpace(s.Body)

		if body == "" {
			out = append(out, Finding{s.Heading, "is empty"})
			continue
		}
		if s.Prov == Discovered && !citation.MatchString(body) {
			out = append(out, Finding{s.Heading,
				"is marked DISCOVERED but cites no evidence — a path, an issue, a commit or a date. " +
					"If it is a reading rather than a finding, mark it INFERRED"})
		}
		if s.Prov == Unknown && !resolution.MatchString(body) {
			out = append(out, Finding{s.Heading,
				"names a gap without saying what would settle it; an open question nobody can " +
					"close is a complaint"})
		}
	}

	if !seen[Unknown] {
		// A discovery that raised no questions did not look hard enough, or is
		// quietly presenting inference as fact.
		out = append(out, Finding{"", "no UNKNOWN section: discovery that established everything " +
			"either did not look hard, or is presenting inference as fact"})
	}
	if !seen[Declared] {
		out = append(out, Finding{"", fmt.Sprintf(
			"no DECLARED section: nothing here is %s, so this %s states only what evidence "+
				"already says", d.Kind.declaredBy(), d.Kind.noun())})
	}
	if d.DiscoveredAt.IsZero() {
		out = append(out, Finding{"", "no discovered timestamp, so staleness cannot be judged"})
	} else if staleAfter > 0 && d.Age(now) > staleAfter {
		out = append(out, Finding{"", fmt.Sprintf(
			"the evidence is %d days old; re-run discovery and report drift",
			int(d.Age(now).Hours()/24))})
	}
	return out
}

// Text renders a document, leading with what the reader most needs to weigh.
func (d *Doc) Text(now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n\n", d.Kind, d.Title)
	if !d.DiscoveredAt.IsZero() {
		fmt.Fprintf(&b, "Evidence collected %s", d.DiscoveredAt.UTC().Format("2006-01-02"))
		if age := d.Age(now); age > 0 {
			fmt.Fprintf(&b, " (%d days ago)", int(age.Hours()/24))
		}
		b.WriteString(".\n")
	}
	if d.Adopted == "" {
		b.WriteString("**Not adopted.** This is a draft: nobody has reviewed it, and its " +
			"DECLARED sections are prompts rather than intent.\n")
	} else {
		fmt.Fprintf(&b, "Adopted %s.\n", d.Adopted)
	}

	fmt.Fprintf(&b, "\nProvenance is marked per section. DISCOVERED is established from evidence "+
		"as of the date above; DECLARED is %s intent; INFERRED is this engineer's reading "+
		"and may be wrong; UNKNOWN is a gap nobody has closed.\n", d.Kind.declaredBy())

	for _, prov := range []Provenance{Declared, Discovered, Inferred, Unknown} {
		for _, s := range d.Of(prov) {
			fmt.Fprintf(&b, "\n## %s {%s}\n\n%s\n", s.Heading, s.Prov, s.Body)
		}
	}
	return b.String()
}

// Brief renders a document for a session start: the person's own words in
// full, the engineer's sections as their first lines, and where the whole
// document is. Bounded, because the runtime shows a session only a preview of
// hook output past a limit, and a program cut off mid-sentence is worse than a
// short one with a pointer.
func (d *Doc) Brief(now time.Time, path string, limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n", d.Kind, d.Title)
	if d.Adopted == "" {
		b.WriteString("Draft — not yet reviewed by the person whose sections are DECLARED; those are prompts, not intent.")
	} else {
		fmt.Fprintf(&b, "Adopted %s.", d.Adopted)
	}
	if !d.DiscoveredAt.IsZero() {
		fmt.Fprintf(&b, " Evidence %d days old.", int(d.Age(now).Hours()/24))
	}
	fmt.Fprintf(&b, " Whole document: %s\n", path)
	for _, s := range d.Of(Declared) {
		fmt.Fprintf(&b, "\n## %s {DECLARED}\n\n%s\n", s.Heading, s.Body)
	}
	for _, prov := range []Provenance{Discovered, Inferred, Unknown} {
		for _, s := range d.Of(prov) {
			fmt.Fprintf(&b, "\n## %s {%s}\n\n%s\n", s.Heading, s.Prov, firstLines(s.Body, 3, 400))
		}
	}
	out := b.String()
	if limit > 0 && len(out) > limit {
		cut := strings.LastIndexByte(out[:limit], '\n')
		if cut < limit/2 {
			cut = limit
		}
		out = out[:cut] + "\n\n(… cut here; the whole document is " + path + ")\n"
	}
	return out
}

// firstLines is the opening of a section: up to n non-empty lines and max
// bytes, with an ellipsis when there was more.
func firstLines(body string, n, max int) string {
	var kept []string
	more := false
	for _, l := range strings.Split(strings.TrimSpace(body), "\n") {
		if strings.TrimSpace(l) == "" {
			if len(kept) > 0 {
				more = true
				break
			}
			continue
		}
		if len(kept) == n {
			more = true
			break
		}
		kept = append(kept, l)
	}
	out := strings.Join(kept, "\n")
	if len(out) > max {
		out = out[:max]
		more = true
	}
	if more {
		out += " …"
	}
	return out
}

func validProvenance(p Provenance) bool { return slices.Contains(provenances, p) }

func join(ps []Provenance) string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return strings.Join(out, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func slugOf(path string) string {
	base := path
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".md")
}

// Covers reports that this document's declared boundary includes the named
// repository.
//
// An undeclared boundary covers nothing. That is the whole point of the
// distinction: a program whose Boundary section is still a question for the
// owner is not a statement about any repository, and standing context that
// occupies every session's window without being about the work in front of it
// is worse than absent — it reads as relevant because it is there.
func (d *Doc) Covers(repo string) bool {
	// Both sides are trimmed. The stored name comes from a hand-written header
	// and the queried one from a git remote, and a false negative here is
	// silence where the program was relevant, which is the failure this whole
	// distinction exists to avoid in the other direction.
	repo = strings.TrimSpace(repo)
	if d == nil || repo == "" {
		return false
	}
	for _, name := range d.Repos {
		if strings.EqualFold(strings.TrimSpace(name), repo) {
			return true
		}
	}
	return false
}

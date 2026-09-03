// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package provenance

import (
	"strings"
	"testing"
	"time"
)

const sample = `# Program: ledger-correctness
discovered: 2026-08-26T15:40:00Z
adopted: 2026-08-26 by the operator

## Purpose {DECLARED}

Keep the ledger correct and shippable.

## Map {DISCOVERED}

payments-api posts financial transactions — internal/posting/service/posting.go:594

## Reading {INFERRED}

payments looks like the correctness boundary: four siblings read from it and
none write to it.

## Open questions {UNKNOWN}

Does advisor's six quiet months mean abandoned, finished or frozen? the operator saying
which would settle it.
`

func parse(t *testing.T, raw string) *Doc {
	t.Helper()
	p, err := Parse(KindProgram, "ledger-correctness", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestParseSeparatesProvenance(t *testing.T) {
	p := parse(t, sample)
	if p.Title != "ledger-correctness" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Adopted == "" || p.DiscoveredAt.IsZero() {
		t.Errorf("metadata lost: adopted=%q discovered=%v", p.Adopted, p.DiscoveredAt)
	}
	for prov, want := range map[Provenance]int{Declared: 1, Discovered: 1, Inferred: 1, Unknown: 1} {
		if got := len(p.Of(prov)); got != want {
			t.Errorf("%s sections = %d, want %d", prov, got, want)
		}
	}
	if !strings.Contains(p.Of(Discovered)[0].Body, "posting.go:594") {
		t.Error("section body lost its citation")
	}
}

// TestUntaggedSectionIsRefused. A section with no provenance has neither a
// trustworthiness nor an owner, and both are load-bearing.
func TestUntaggedSectionIsRefused(t *testing.T) {
	_, err := Parse(KindProgram, "x", "# Program: x\n\n## Purpose\n\nsomething\n")
	if err == nil {
		t.Fatal("a section with no provenance tag was accepted")
	}
	if !strings.Contains(err.Error(), "provenance") {
		t.Errorf("error does not explain what is missing: %v", err)
	}
}

func TestUnknownProvenanceIsRefused(t *testing.T) {
	_, err := Parse(KindProgram, "x", "# Program: x\n\n## Purpose {ASSUMED}\n\nsomething\n")
	if err == nil {
		t.Fatal("an invented provenance was accepted")
	}
}

// TestDiscoveredWithoutEvidenceIsMislabelled: the failure this taxonomy exists
// to catch. A reading presented as a finding is how a guess becomes a fact.
func TestDiscoveredWithoutEvidenceIsMislabelled(t *testing.T) {
	p := parse(t, strings.Replace(sample,
		"payments-api posts financial transactions — internal/posting/service/posting.go:594",
		"payments is the most important service in the estate.", 1))

	var found bool
	for _, f := range p.Check(time.Now(), 0) {
		if f.Section == "Map" && strings.Contains(f.Detail, "cites no evidence") {
			found = true
			if !strings.Contains(f.Detail, "INFERRED") {
				t.Errorf("the finding does not say what the section should be instead: %s", f.Detail)
			}
		}
	}
	if !found {
		t.Error("a DISCOVERED section with no evidence was accepted as a finding")
	}
}

func TestCheckPassesAWellFormedProgram(t *testing.T) {
	p := parse(t, sample)
	if got := p.Check(p.DiscoveredAt.Add(time.Hour), 30*24*time.Hour); len(got) != 0 {
		t.Errorf("a well-formed program produced findings: %v", got)
	}
}

// TestNoUnknownSectionIsItselfAFinding. Discovery that established everything
// either did not look hard or is presenting inference as fact.
func TestNoUnknownSectionIsItselfAFinding(t *testing.T) {
	p := parse(t, strings.Replace(sample,
		"## Open questions {UNKNOWN}\n\nDoes advisor's six quiet months mean abandoned, finished or frozen? the operator saying\nwhich would settle it.\n", "", 1))
	if !hasFinding(p.Check(time.Now(), 0), "did not look hard") {
		t.Error("a program with no open questions was accepted without comment")
	}
}

func TestUnknownMustSayWhatWouldSettleIt(t *testing.T) {
	p := parse(t, strings.Replace(sample,
		"Does advisor's six quiet months mean abandoned, finished or frozen? the operator saying\nwhich would settle it.",
		"Not sure about advisor.", 1))
	if !hasFinding(p.Check(time.Now(), 0), "a complaint") {
		t.Error("an open question with no route to closing it was accepted")
	}
}

func TestStaleEvidenceIsReported(t *testing.T) {
	p := parse(t, sample)
	late := p.DiscoveredAt.Add(90 * 24 * time.Hour)
	if !hasFinding(p.Check(late, 30*24*time.Hour), "days old") {
		t.Error("evidence three months old was not reported stale")
	}
	if hasFinding(p.Check(p.DiscoveredAt.Add(time.Hour), 30*24*time.Hour), "days old") {
		t.Error("fresh evidence was reported stale")
	}
}

// TestUnadoptedDraftSaysSo. A draft read as owner intent is worse than no
// program: it looks authoritative and nobody checked it.
func TestUnadoptedDraftSaysSo(t *testing.T) {
	p := parse(t, strings.Replace(sample, "adopted: 2026-08-26 by the operator\n", "", 1))
	txt := p.Text(time.Now())
	if !strings.Contains(txt, "Not adopted") || !strings.Contains(txt, "prompts rather than intent") {
		t.Errorf("an unadopted draft does not disclose itself:\n%s", txt)
	}
	if !strings.Contains(parse(t, sample).Text(time.Now()), "Adopted 2026-08-26") {
		t.Error("an adopted program does not say so")
	}
}

func TestTextExplainsProvenanceToTheReader(t *testing.T) {
	txt := parse(t, sample).Text(time.Now())
	for _, want := range []string{"DECLARED is the owner's intent", "may be wrong", "a gap nobody has closed"} {
		if !strings.Contains(txt, want) {
			t.Errorf("rendered program does not explain provenance, missing %q", want)
		}
	}
}

func hasFinding(fs []Finding, substr string) bool {
	for _, f := range fs {
		if strings.Contains(f.Detail, substr) {
			return true
		}
	}
	return false
}

func TestBriefKeepsDeclaredWholeAndBoundsTheRest(t *testing.T) {
	raw := "# Program: p\ndiscovered: 2026-08-26T15:40:00Z\n\n## Purpose {DECLARED}\n\nkeep every word of this\n\n## Map {DISCOVERED}\n\nline one src/a.go:1\nline two\nline three\nline four\nline five\n\n## Open questions {UNKNOWN}\n\nwhat would settle it\n"
	d, err := Parse(KindProgram, "p", raw)
	if err != nil {
		t.Fatal(err)
	}
	out := d.Brief(time.Now(), "/x/p.md", 0)
	if !strings.Contains(out, "keep every word of this") {
		t.Errorf("declared section was cut: %s", out)
	}
	if strings.Contains(out, "line four") || !strings.Contains(out, "line three …") {
		t.Errorf("discovered section was not reduced to its first lines: %s", out)
	}
	if !strings.Contains(out, "/x/p.md") {
		t.Errorf("the whole document is not named: %s", out)
	}
	small := d.Brief(time.Now(), "/x/p.md", 120)
	if len(small) > 200 || !strings.Contains(small, "cut here") {
		t.Errorf("limit not applied: %d bytes: %s", len(small), small)
	}
}

// TestAnUnknownSectionNeedsAResolutionClauseNotAWord: "Settled by: reading the
// migration" closes the gap; "we would like to know" does not.
func TestAnUnknownSectionNeedsAResolutionClauseNotAWord(t *testing.T) {
	head := "# Program: p\ndiscovered: 2026-08-26T15:40:00Z\n\n## Purpose {DECLARED}\n\nq\n\n## Map {DISCOVERED}\n\nsrc/a.go:1\n\n## Open questions {UNKNOWN}\n\n"
	for body, ok := range map[string]bool{
		"Is the table still written? *Settled by:* reading a/b/c.sql.":         true,
		"Is the table still written? What would settle it: the migration log.": true,
		"Is the table still written? Decided by the operator.":                 true,
		"Is the table still written? We do not know.":                          false,
	} {
		d, err := Parse(KindProgram, "p", head+body+"\n")
		if err != nil {
			t.Fatal(err)
		}
		var hit bool
		for _, f := range d.Check(time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), 0) {
			if strings.Contains(f.Detail, "settle") {
				hit = true
			}
		}
		if hit == ok {
			t.Errorf("%q: flagged=%v, want flagged=%v", body, hit, !ok)
		}
	}
}

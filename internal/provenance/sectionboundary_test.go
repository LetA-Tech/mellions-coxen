// Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
package provenance

import "strings"

import "testing"

const scoped = `# Partnership: someone
adopted: 2026-08-28

## What is delegated to me {DECLARED}

Autonomous by default for normal engineering work.

## Maintaining the engineer itself {DECLARED}
repos: mellions-coxen

Fix a defect in yourself mid-work, in a lane of your own.

## How we work together {DECLARED}

Say what you established and what you did not.
`

func doc(t *testing.T) *Doc {
	t.Helper()
	d, err := Parse(KindPartnership, "someone", scoped)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func headings(d *Doc) []string {
	var out []string
	for _, s := range d.Sections {
		out = append(out, s.Heading)
	}
	return out
}

// A section that named its scope is left out where the repository is not in it.
func TestAScopedSectionNarrowsToTheRepositoryItNames(t *testing.T) {
	got := headings(doc(t).Here("policy-service"))
	for _, h := range got {
		if h == "Maintaining the engineer itself" {
			t.Errorf("a section scoped to mellions-coxen rendered in policy-service: %v", got)
		}
	}
	if len(got) != 2 {
		t.Errorf("sections = %v, want the two unscoped ones", got)
	}
	if in := headings(doc(t).Here("mellions-coxen")); len(in) != 3 {
		t.Errorf("in the repository it names, sections = %v, want all three", in)
	}
}

// The direction that must never fail. Withholding what the owner delegated
// makes a session either over-ask or act past its authority, so a section that
// declared no boundary is about every repository.
func TestAnUnscopedSectionIsNeverWithheld(t *testing.T) {
	for _, repo := range []string{"policy-service", "mellions-coxen", "somewhere-else", ""} {
		got := headings(doc(t).Here(repo))
		var found bool
		for _, h := range got {
			if h == "What is delegated to me" {
				found = true
			}
		}
		if !found {
			t.Errorf("in %q the delegation section was dropped: %v", repo, got)
		}
	}
}

// The boundary line is a directive, not content: it must not reach the reader
// as prose, and a `repos:` deep inside a section is prose and must.
func TestTheBoundaryLineLeavesTheBodyAndProseDoesNot(t *testing.T) {
	d := doc(t)
	for _, s := range d.Sections {
		if s.Heading == "Maintaining the engineer itself" && strings.Contains(s.Body, "repos:") {
			t.Errorf("the boundary line was rendered as body: %q", s.Body)
		}
	}
	late, err := Parse(KindPartnership, "x",
		"# Partnership: x\n\n## S {DECLARED}\n\nSome prose.\nrepos: not-a-directive\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(late.Sections[0].Repos) != 0 {
		t.Errorf("a repos: line after prose was read as a boundary: %v", late.Sections[0].Repos)
	}
	if !strings.Contains(late.Sections[0].Body, "repos: not-a-directive") {
		t.Errorf("prose was swallowed as a directive: %q", late.Sections[0].Body)
	}
}

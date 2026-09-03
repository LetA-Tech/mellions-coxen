// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package provenance_test

import (
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/provenance"
)

const bounded = `# Program: demo-platform
discovered: 2026-08-26T15:41:42Z
adopted: 2026-08-28 by the operator
repos: data-service, payments-api, connector-api

## Purpose {DECLARED}

Keep the ledger correct.
`

const unbounded = `# Program: demo-platform
discovered: 2026-08-26T15:41:42Z
adopted: 2026-08-28 by the operator

## Purpose {DECLARED}

Keep the ledger correct.
`

func parseDoc(t *testing.T, raw string) *provenance.Doc {
	t.Helper()
	d, err := provenance.Parse(provenance.KindProgram, "demo-platform", raw)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestADeclaredBoundaryCoversWhatItNames is the ordinary case.
func TestADeclaredBoundaryCoversWhatItNames(t *testing.T) {
	d := parseDoc(t, bounded)
	if len(d.Repos) != 3 {
		t.Fatalf("parsed %v", d.Repos)
	}
	for _, in := range []string{"data-service", "Payments-API", " connector-api "} {
		if !d.Covers(in) {
			t.Errorf("a declared boundary does not cover %q", in)
		}
	}
}

// TestAnUndeclaredBoundaryCoversNothing is the whole point of the distinction.
//
// A program whose boundary is still a question for the owner is not a statement
// about any repository; silence must not be interpreted as "covers everything".
func TestAnUndeclaredBoundaryCoversNothing(t *testing.T) {
	d := parseDoc(t, unbounded)
	if len(d.Repos) != 0 {
		t.Fatalf("an undeclared boundary parsed as %v", d.Repos)
	}
	for _, in := range []string{"data-service", "policy-service", ""} {
		if d.Covers(in) {
			t.Errorf("an undeclared boundary claimed to cover %q", in)
		}
	}
}

// TestARepositoryOutsideTheBoundaryIsNotCovered holds the case the session met.
func TestARepositoryOutsideTheBoundaryIsNotCovered(t *testing.T) {
	if parseDoc(t, bounded).Covers("policy-service") {
		t.Fatal("a program that does not name policy-service claimed to cover it")
	}
}

// TestTheBoundaryDoesNotDisturbTheRestOfTheHeader: a new meta key must not eat
// the ones the document already carries.
func TestTheBoundaryDoesNotDisturbTheRestOfTheHeader(t *testing.T) {
	d := parseDoc(t, bounded)
	if d.Adopted == "" || d.DiscoveredAt.IsZero() || d.Title == "" {
		t.Fatalf("header lost: adopted=%q discovered=%v title=%q", d.Adopted, d.DiscoveredAt, d.Title)
	}
	if len(d.Sections) == 0 {
		t.Fatal("sections lost")
	}
}

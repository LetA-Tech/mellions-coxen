// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package programs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

const draft = `# Program: ledger
discovered: 2026-08-26T15:40:00Z

## Purpose {DECLARED}

WHAT IS THIS FOR? the operator to answer.

## Map {DISCOVERED}

svc-ledger posts transactions — internal/posting/service.go:12

## Open questions {UNKNOWN}

Is svc-old abandoned or frozen? the operator saying which would settle it.
`

func dir(t *testing.T, files map[string]string) string {
	t.Helper()
	d := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(d, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func collect(t *testing.T, o Options) []signal.Signal {
	t.Helper()
	got, err := New(o).Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return got
}

// TestNoProgramReportsTheRemedyRatherThanFailing. It used to be an error, on the
// reasoning that an engineer with no statement of responsibility should not pick
// its own work. That was written when a program could only be handed down.
func TestNoProgramReportsTheRemedyRatherThanFailing(t *testing.T) {
	got := collect(t, Options{Dir: t.TempDir()})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want one saying no program exists", len(got))
	}
	if got[0].Attrs["remedy"] != "mellions program discover" {
		t.Errorf("the absence of a program does not carry its remedy: %+v", got[0].Attrs)
	}
}

// TestDraftIsNeverPresentedAsIntent. A draft read as owner intent is worse than
// no program: it looks authoritative and nobody checked it.
func TestDraftIsNeverPresentedAsIntent(t *testing.T) {
	got := collect(t, Options{Dir: dir(t, map[string]string{"ledger.md": draft})})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if !strings.Contains(got[0].Attrs["state"], "DRAFT") {
		t.Errorf("an unadopted program is not marked a draft: %+v", got[0].Attrs)
	}
	if !strings.Contains(got[0].Detail, "Not adopted") {
		t.Error("the carried text does not disclose that nobody reviewed it")
	}
	// Provenance must survive into what the model reads.
	for _, want := range []string{"{DECLARED}", "{DISCOVERED}", "{UNKNOWN}"} {
		if !strings.Contains(got[0].Detail, want) {
			t.Errorf("provenance %s was lost on the way to the model", want)
		}
	}
}

// TestAdoptedProgramSaysSo.
func TestAdoptedProgramSaysSo(t *testing.T) {
	adopted := strings.Replace(draft, "discovered: 2026-08-26T15:40:00Z",
		"discovered: 2026-08-26T15:40:00Z\nadopted: 2026-08-26 by the operator", 1)
	d := dir(t, map[string]string{"ledger.md": adopted})
	got := collect(t, Options{Dir: d})
	if got[0].Attrs["state"] != "adopted" {
		t.Errorf("state = %q, want adopted", got[0].Attrs["state"])
	}
}

// TestUntaggedProgramIsNeverAcceptedAsAttributed.
//
// An untagged file cannot be told apart from a legacy one, so it is carried
// rather than refused — but it must never read as a program whose provenance is
// known. The content is useful; the attribution is what is missing, and saying
// so is the whole difference.
func TestUntaggedProgramIsNeverAcceptedAsAttributed(t *testing.T) {
	d := dir(t, map[string]string{"bad.md": "# Program: bad\n\n## Purpose\n\nkeep it correct\n"})
	got := collect(t, Options{Dir: d})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1", len(got))
	}
	if got[0].Attrs["state"] == "adopted" {
		t.Error("an untagged file was reported as an adopted program")
	}
	if got[0].Attrs["remedy"] == "" {
		t.Errorf("no remedy offered for an unattributed program: %+v", got[0].Attrs)
	}
	if !strings.Contains(got[0].Detail, "unverified") {
		t.Error("unattributed content is presented without that warning")
	}
}

func TestSeveralProgramsAreSeparateSignals(t *testing.T) {
	got := collect(t, Options{Dir: dir(t, map[string]string{
		"ledger.md":    draft,
		"reporting.md": strings.Replace(draft, "# Program: ledger", "# Program: reporting", 1),
	})})
	if len(got) != 2 {
		t.Fatalf("got %d signals, want one per program", len(got))
	}
	if got[0].ID == got[1].ID {
		t.Error("two programs collapsed into one identity")
	}
}

// TestLegacyFileMigratesRatherThanBreaking. A file written before provenance
// existed is old, not malformed. Refusing to start because the format moved is
// the same fail-closed mistake the authority gate made, and the remedy — re-run
// discovery — is more use than an error.
func TestLegacyFileMigratesRatherThanBreaking(t *testing.T) {
	legacy := "# Program responsibility\n\n## payments-api — ledger correctness\n\nGo service owning posting.\n"
	got := collect(t, Options{Path: filepath.Join(dir(t, map[string]string{"P.md": legacy}), "P.md")})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want one describing the legacy file", len(got))
	}
	if got[0].Attrs["remedy"] != "mellions program discover" {
		t.Errorf("the legacy file does not carry its remedy: %+v", got[0].Attrs)
	}
	// The content is still useful; what it lacks is attribution, and saying so
	// is the whole difference.
	if !strings.Contains(got[0].Detail, "Go service owning posting") {
		t.Error("the legacy content was discarded rather than carried")
	}
	if !strings.Contains(got[0].Detail, "treat every line below as unverified") {
		t.Error("legacy content is presented without warning that nothing in it is attributed")
	}
}

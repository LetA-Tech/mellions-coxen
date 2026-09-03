// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// idShapeConfig writes a configuration whose records live under the test's own
// directory, with no owner, so nothing here reaches the tracker or the disk the
// session is working on.
func idShapeConfig(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	cfg := map[string]any{
		"work_root":        filepath.Dir(source),
		"repos":            []string{"probe-repo"},
		"checkouts":        map[string]string{"probe-repo": source},
		"assignments_root": filepath.Join(root, "assignments"),
		"report_root":      root,
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestEveryVerbTakesTheIdEitherWay.
//
// One argument had three spellings: `assign open` took -id, every other assign
// verb took the id positionally, and `report write` took -assignment. A session
// that had just used `assign open -id` successfully believed it knew the
// convention, so it wrote `assign record -id X` and got "assign record needs an
// id and some text" — a message naming what was missing rather than that the
// spelling was wrong. The cost was three failed invocations per session, paid
// once by every session.
//
// Each verb is run for real here, against a store in the test's own directory,
// because a test of the flag registration alone would not show that the id
// reaches the store.
func TestEveryVerbTakesTheIdEitherWay(t *testing.T) {
	cfg := idShapeConfig(t, claimRepo(t))
	open := []string{"open", "probe-1", "-repo", "probe-repo", "-config", cfg,
		"-objective", "the work", "-because", "the owner asked for it", "-unpublished"}
	if err := cmdAssign(context.Background(), open); err != nil {
		t.Fatalf("assign open: %v", err)
	}

	// Each verb twice: the spelling it has always taken, and the spelling a
	// session carries over from `assign open`. Both must reach the store.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"record positional", []string{"record", "probe-1", "found it positionally"}},
		{"record -id", []string{"record", "-id", "probe-1", "found it by flag"}},
		{"record -id with -kind", []string{"record", "-id", "probe-1", "-kind", "found", "text after a flag"}},
		{"get positional", []string{"get", "probe-1"}},
		{"get -id", []string{"get", "-id", "probe-1"}},
		{"handoff -id", []string{"handoff", "-id", "probe-1", "where it stands"}},
		{"reopen -id", []string{"reopen", "-id", "probe-1"}},
		{"handoff positional", []string{"handoff", "probe-1", "where it stands"}},
		{"reopen positional", []string{"reopen", "probe-1"}},
		{"close -id", []string{"close", "-id", "probe-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := cmdAssign(context.Background(), append(tc.args, "-config", cfg)); err != nil {
				t.Fatalf("mellions assign %s: %v", strings.Join(tc.args, " "), err)
			}
		})
	}

	// The text really is the text, and not swallowed as a second id.
	records, err := filepath.Glob(filepath.Join(filepath.Dir(cfg), "assignments", "probe-1", "*"))
	if err != nil || len(records) == 0 {
		t.Fatalf("no record written for probe-1: %v %v", records, err)
	}
	var body []byte
	for _, r := range records {
		b, err := os.ReadFile(r)
		if err != nil {
			continue
		}
		body = append(body, b...)
	}
	for _, want := range []string{"found it positionally", "found it by flag", "text after a flag"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the record does not hold %q:\n%s", want, body)
		}
	}
}

// TestReportWriteTakesTheIdEitherWay. `report write` spelled the same argument
// -assignment, a third form again, and parsed with flag.Parse, so an id written
// as an argument silently dropped every flag after it — the report was written
// with no body rather than refused.
func TestReportWriteTakesTheIdEitherWay(t *testing.T) {
	cfg := idShapeConfig(t, claimRepo(t))
	// A distinct id per spelling: a report is named for the second it was
	// written in, so three in one second would be one file three times.
	for _, tc := range []struct {
		name string
		id   string
		args []string
	}{
		{"-assignment", "probe-a", []string{"write", "-assignment", "probe-a", "-did", "the original spelling"}},
		{"-id", "probe-b", []string{"write", "-id", "probe-b", "-did", "the spelling assign open teaches"}},
		{"positional", "probe-c", []string{"write", "probe-c", "-did", "the spelling every assign verb teaches"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := cmdReport(append(tc.args, "-config", cfg)); err != nil {
				t.Fatalf("mellions report %s: %v", strings.Join(tc.args, " "), err)
			}
			found, err := filepath.Glob(filepath.Join(filepath.Dir(cfg), "reports", "*-"+tc.id+".md"))
			if err != nil || len(found) != 1 {
				t.Fatalf("one report named for %s, got %v (%v)", tc.id, found, err)
			}
			b, err := os.ReadFile(found[0])
			if err != nil {
				t.Fatal(err)
			}
			// The body is the flag after the id: parsing that stopped at the
			// id would write a report with nothing in it.
			if !strings.Contains(string(b), "spelling") {
				t.Errorf("%s lost its body:\n%s", found[0], b)
			}
		})
	}
}

// TestTheUsageErrorsNameBothSpellings. The message is what a session reads when
// it guesses wrong, and the old ones named the missing input without saying the
// spelling was the problem, so the natural next guess failed the same way.
func TestTheUsageErrorsNameBothSpellings(t *testing.T) {
	t.Setenv("MELLIONS_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
	for _, verb := range []string{"record", "get", "handoff", "close", "abandon", "reopen"} {
		t.Run(verb, func(t *testing.T) {
			err := cmdAssign(context.Background(), []string{verb})
			if err == nil {
				t.Fatalf("assign %s with no id was accepted", verb)
			}
			if !strings.Contains(err.Error(), "-id") {
				t.Errorf("assign %s: %q does not name -id", verb, err)
			}
			if !strings.Contains(err.Error(), "<id>") {
				t.Errorf("assign %s: %q does not name the argument form", verb, err)
			}
		})
	}
}

// TestAssignIDKeepsTheTextWhenTheFlagIsUsed is the trap in accepting both: with
// -id given, everything left is the verb's own text. Reading the first argument
// as the id as well would eat the first word of every recorded note.
func TestAssignIDKeepsTheTextWhenTheFlagIsUsed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flag  string
		rest  []string
		id    string
		extra string
	}{
		{"flag with text", "probe-1", []string{"some", "text"}, "probe-1", "some text"},
		{"positional with text", "", []string{"probe-1", "some", "text"}, "probe-1", "some text"},
		{"flag alone", "probe-1", nil, "probe-1", ""},
		{"positional alone", "", []string{"probe-1"}, "probe-1", ""},
		{"neither", "", nil, "", ""},
		{"flag padded", "  probe-1  ", []string{"text"}, "probe-1", "text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, rest := assignID(tc.flag, tc.rest)
			if id != tc.id {
				t.Errorf("id = %q, want %q", id, tc.id)
			}
			if got := strings.Join(rest, " "); got != tc.extra {
				t.Errorf("rest = %q, want %q", got, tc.extra)
			}
		})
	}
}

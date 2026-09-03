// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package signal

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestSignalCarriesNoJudgement is the mechanical form of the architecture's
// central rule: collection is software's job, ranking is the model's.
//
// It exists because that rule is exactly the kind that erodes by a reasonable
// one-line addition — a Priority here, a Severity there — until the collector
// has quietly taken over the decision it was built to inform. Documentation did
// not hold that line in the system this replaces. This does.
//
// If a genuinely necessary field trips this, the fix is to carry it in Attrs
// where the model reads it and the core cannot act on it.
func TestSignalCarriesNoJudgement(t *testing.T) {
	// The permitted set, not a list of forbidden words. A denylist only refuses
	// the namings somebody thought of: `Priority int` was caught and `Tier
	// string` passed, and a field named `Bucket`, `Band` or `Class` would carry
	// exactly the same judgement past exactly the same guard.
	//
	// Adding a field here is deliberate, and the question to answer while doing
	// it is whether a reader could sort by it. If they could, it belongs in
	// Attrs, where the model reads it and the core cannot act on it.
	permitted := map[string]bool{
		"Kind": true, "Source": true, "ID": true, "Title": true, "URL": true,
		"Repo": true, "Created": true, "Updated": true, "Labels": true,
		"Attrs": true, "Detail": true,
	}
	for f := range reflect.TypeFor[Signal]().Fields() {
		if !permitted[f.Name] {
			t.Errorf("Signal.%s is not in the permitted set. If it is a fact about the item, "+
				"add it here deliberately; if a reader could sort by it, it is judgement and "+
				"belongs in Attrs — see the internal/signal doc comment", f.Name)
		}
	}
	// And the set does not drift the other way: a field removed from the struct
	// and left here would let its replacement in under the old name.
	present := map[string]bool{}
	for f := range reflect.TypeFor[Signal]().Fields() {
		present[f.Name] = true
	}
	for name := range permitted {
		if !present[name] {
			t.Errorf("the permitted set names %s, which Signal no longer has", name)
		}
	}
}

// TestKindsCoversEveryKind guards the renderer against a kind that is declared
// and then never displayed, which would drop evidence silently.
func TestKindsCoversEveryKind(t *testing.T) {
	declared := []Kind{
		KindAssignment, KindWorkItem, KindChangeSet, KindBuild, KindAlert,
		KindCommit, KindStalePremise, KindBlocked, KindObjective, KindFollowUp,
	}
	in := make(map[Kind]bool, len(Kinds))
	for _, k := range Kinds {
		if in[k] {
			t.Fatalf("Kinds lists %q twice", k)
		}
		in[k] = true
	}
	for _, k := range declared {
		if !in[k] {
			t.Errorf("kind %q is declared but absent from Kinds, so a survey would never show it", k)
		}
	}
	if len(Kinds) != len(declared) {
		t.Errorf("Kinds has %d entries, %d kinds are declared", len(Kinds), len(declared))
	}
}

func TestAgePrefersUpdatedThenCreated(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	created := now.Add(-72 * time.Hour)
	updated := now.Add(-2 * time.Hour)

	if got := (Signal{Created: created, Updated: updated}).Age(now); got != 2*time.Hour {
		t.Errorf("with both timestamps, age = %v, want 2h (Updated wins)", got)
	}
	if got := (Signal{Created: created}).Age(now); got != 72*time.Hour {
		t.Errorf("with only Created, age = %v, want 72h", got)
	}
	if got := (Signal{}).Age(now); got != 0 {
		t.Errorf("with no timestamps, age = %v, want 0", got)
	}
}

func TestDedupeKeepsFirstAcrossSources(t *testing.T) {
	// Two sources legitimately see one issue: the work-item collector and the
	// stale-premise scan. They differ in Kind, so both survive — the same fact
	// twice from one source and kind does not.
	in := []Signal{
		{Source: "github", Kind: KindWorkItem, ID: "75", Title: "first"},
		{Source: "github", Kind: KindWorkItem, ID: "75", Title: "duplicate"},
		{Source: "stale", Kind: KindStalePremise, ID: "75", Title: "premise no longer holds"},
	}
	got := Dedupe(in)
	if len(got) != 2 {
		t.Fatalf("Dedupe returned %d signals, want 2: %+v", len(got), got)
	}
	if got[0].Title != "first" {
		t.Errorf("Dedupe kept %q, want the first occurrence", got[0].Title)
	}
	if got[1].Kind != KindStalePremise {
		t.Errorf("Dedupe dropped a different-kind signal about the same id")
	}
}

type fakeSource struct {
	name string
	out  []Signal
	err  error
}

func (f fakeSource) Name() string { return f.name }
func (f fakeSource) Collect(context.Context, Scope) ([]Signal, error) {
	return f.out, f.err
}

func TestRegistryRefusesDuplicateName(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeSource{name: "github"}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := r.Register(fakeSource{name: "github"})
	if err == nil {
		t.Fatal("registering a duplicate name succeeded; Signal.Source provenance would be ambiguous")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("error = %v, want it to name the duplicate", err)
	}
}

func TestRegistryRefusesNamelessAndNil(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Error("registering nil succeeded")
	}
	if err := r.Register(fakeSource{name: "  "}); err == nil {
		t.Error("registering a blank-named source succeeded")
	}
}

func TestRegistryNamesAreStable(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"programs", "github", "assignments"} {
		if err := r.Register(fakeSource{name: n}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"assignments", "github", "programs"}
	if got := r.Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	if _, ok := r.Get("github"); !ok {
		t.Error("Get did not find a registered source")
	}
	if _, ok := r.Get("gitlab"); ok {
		t.Error("Get found an unregistered source")
	}
}

func TestSourceErrorIsDistinctFromEmpty(t *testing.T) {
	// The contract that makes a survey trustworthy: "nothing is failing" and
	// "could not reach CI" must never render the same way.
	boom := errors.New("gh: not authenticated")
	_, err := fakeSource{name: "ci", err: boom}.Collect(context.Background(), Scope{})
	if !errors.Is(err, boom) {
		t.Fatalf("Collect swallowed the source error: %v", err)
	}
	out, err := fakeSource{name: "ci"}.Collect(context.Background(), Scope{})
	if err != nil || len(out) != 0 {
		t.Fatalf("an empty successful collection must not look like a failure: %v %v", out, err)
	}
}

func TestGroupByKindPreservesInputOrder(t *testing.T) {
	in := []Signal{
		{Kind: KindWorkItem, ID: "a"},
		{Kind: KindBuild, ID: "b"},
		{Kind: KindWorkItem, ID: "c"},
	}
	got := GroupByKind(in)
	if len(got[KindWorkItem]) != 2 || got[KindWorkItem][0].ID != "a" || got[KindWorkItem][1].ID != "c" {
		t.Errorf("grouping reordered a bucket: %+v", got[KindWorkItem])
	}
	if len(got[KindBuild]) != 1 {
		t.Errorf("grouping lost a signal: %+v", got)
	}
}

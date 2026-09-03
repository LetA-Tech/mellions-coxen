// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package gitsrc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

func fakeGit(t *testing.T, byVerb map[string]string) Runner {
	t.Helper()
	return func(_ context.Context, _ string, args ...string) (string, error) {
		if out, ok := byVerb[args[0]]; ok {
			return out, nil
		}
		return "", nil
	}
}

func TestBusyRepoIsOneSignalNotManyCommits(t *testing.T) {
	// A busy week is one fact about a repository. Four hundred commit signals
	// would drown every other source in a survey meant to be read.
	var log strings.Builder
	for i := range 30 {
		fmt.Fprintf(&log, "abc%03d 2026-08-2%d fix: thing %d\n", i, i%10, i)
	}
	s := New(Options{
		WorkRoot: t.TempDir(), Repos: []string{"repo"}, MaxPerRepo: 5,
		Run: fakeGit(t, map[string]string{"log": log.String(), "rev-parse": "dev\n"}),
	})
	// The work root has no real checkout, so Collect must refuse — which is
	// itself the behaviour we want, checked below.
	if _, err := s.Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("a missing checkout was accepted")
	}
}

func TestMissingCheckoutIsAnErrorNotSilence(t *testing.T) {
	s := New(Options{WorkRoot: t.TempDir(), Repos: []string{"absent"},
		Run: fakeGit(t, nil)})
	_, err := s.Collect(context.Background(), signal.Scope{})
	if err == nil {
		t.Fatal("a repository with no checkout reported no change rather than failing")
	}
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("the error does not name the missing repository: %v", err)
	}
}

func TestNoWorkRootIsAnError(t *testing.T) {
	if _, err := New(Options{}).Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("collecting with no work root succeeded")
	}
}

func TestQuietRepoProducesNoSignal(t *testing.T) {
	// Nothing changed is not a finding; it is the absence of one.
	dir := t.TempDir()
	s := New(Options{WorkRoot: dir, Repos: []string{"."},
		Run: fakeGit(t, map[string]string{"log": "", "rev-parse": "main\n"})})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a repository with no recent commits produced %d signals", len(got))
	}
}

func TestRecentChangeIsSummarisedWithCounts(t *testing.T) {
	dir := t.TempDir()
	s := New(Options{
		WorkRoot: dir, Repos: []string{"."}, MaxPerRepo: 2,
		Run: fakeGit(t, map[string]string{
			"log":       "aaa 2026-08-25 fix: one\nbbb 2026-08-24 fix: two\nccc 2026-08-23 fix: three\n",
			"rev-parse": "dev\n",
			"status":    " M internal/thing.go\n",
		}),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want one signal per repository, got %d", len(got))
	}
	g := got[0]
	if g.Kind != signal.KindCommit || g.Attrs["commits"] != "3" || g.Attrs["branch"] != "dev" {
		t.Fatalf("unexpected signal: %+v", g)
	}
	if g.Attrs["uncommitted"] != "1" {
		t.Errorf("uncommitted work in a shared checkout was not surfaced: %+v", g.Attrs)
	}
	if !strings.Contains(g.Detail, "and 1 more") {
		t.Errorf("truncation is silent, so the reader cannot tell how much was hidden:\n%s", g.Detail)
	}
}

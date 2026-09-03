// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package stale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// checkout writes a fake repository and returns the name→path map the scan needs.
func checkout(t *testing.T, files map[string]string) map[string]string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "payments-api")
	for name, body := range files {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return map[string]string{"payments-api": repo}
}

func lines(n int, at map[int]string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		if s, ok := at[i]; ok {
			b.WriteString(s)
		} else {
			b.WriteString("// filler")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func runnerFor(t *testing.T, items ...item) Runner {
	t.Helper()
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	return func(context.Context, ...string) ([]byte, error) { return raw, nil }
}

func old() time.Time { return time.Now().Add(-30 * 24 * time.Hour) }

func scan(t *testing.T, co map[string]string, items ...item) []signal.Signal {
	t.Helper()
	s := New(Options{Owner: "example-org", Repos: []string{"payments-api"}, Checkouts: co, Run: runnerFor(t, items...)})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return got
}

// TestCitationThatStillResolvesIsNotStale keeps the scan from crying wolf on
// every open issue, which would make the signal worthless.
func TestCitationThatStillResolvesIsNotStale(t *testing.T) {
	co := checkout(t, map[string]string{"internal/database/drivers/postgres.go": lines(900, nil)})
	got := scan(t, co, item{
		Number: 75, Title: "pool warns at idle", CreatedAt: old(),
		Body: "The monitor at `internal/database/drivers/postgres.go:877` emits every 5 minutes.",
	})
	if len(got) != 0 {
		t.Fatalf("a resolving citation was reported stale: %+v", got)
	}
}

// A citation past the end of a located file is a moved premise.
func TestCitationPastEndOfFileIsStale(t *testing.T) {
	co := checkout(t, map[string]string{"internal/database/drivers/postgres.go": lines(400, nil)})
	got := scan(t, co, item{
		Number: 42, Title: "avg_acquire_duration logs the cumulative total", CreatedAt: old(),
		URL:  "https://example.invalid/42",
		Body: "`internal/database/drivers/postgres.go:820`\n\nlogged under an averaging name.",
	})
	if len(got) != 1 {
		t.Fatalf("got %d signals, want 1: %+v", len(got), got)
	}
	s := got[0]
	if s.Kind != signal.KindStalePremise || s.ID != "#42" || s.Repo != "payments-api" {
		t.Fatalf("unexpected signal: %+v", s)
	}
	if s.Attrs["moved"] != "1" || s.Attrs["citations"] == "" || s.Attrs["unchecked"] != "0" {
		t.Errorf("attrs do not record what was checked: %+v", s.Attrs)
	}
	// The wording matters as much as the detection: a stale premise is a reason
	// to read the code, not a claim that the work is finished.
	if !strings.Contains(s.Detail, "not proof the work is done") {
		t.Errorf("detail overstates the finding:\n%s", s.Detail)
	}
}

// A bare basename absent here may belong to a sibling repository, so it is
// unchecked rather than stale.
func TestBareBasenameNotFoundIsUncheckedNotStale(t *testing.T) {
	co := checkout(t, map[string]string{"internal/keep.go": lines(10, nil)})
	got := scan(t, co, item{
		Number: 43, Title: "close the data-service column guard blind spot", CreatedAt: old(),
		Body: "See `140_credit_limit_provenance.sql:44` and `169_account_balance_orientation.sql:104`.",
	})
	if len(got) != 0 {
		t.Fatalf("a bare filename that may live in a sibling repo was reported as moved: %+v", got)
	}
}

// TestUncheckedCitationsAreDisclosedAlongsideRealEvidence: when a body mixes
// both, the reader must be told how much could not be checked here, or the
// evidence looks more complete than it is.
func TestUncheckedCitationsAreDisclosedAlongsideRealEvidence(t *testing.T) {
	co := checkout(t, map[string]string{"internal/short.go": lines(20, nil)})
	got := scan(t, co, item{
		Number: 500, Title: "mixed citations", CreatedAt: old(),
		Body: "`internal/short.go:900` is the defect; compare `999_elsewhere.sql:12`.",
	})
	if len(got) != 1 {
		t.Fatalf("want 1 signal, got %d: %+v", len(got), got)
	}
	if got[0].Attrs["moved"] != "1" || got[0].Attrs["unchecked"] != "1" {
		t.Errorf("attrs = %+v, want moved=1 unchecked=1", got[0].Attrs)
	}
	if got[0].Attrs["moved_in_located_file"] != "1" || got[0].Attrs["path_absent"] != "0" {
		t.Errorf("evidence classes not separated: %+v", got[0].Attrs)
	}
	if !strings.Contains(got[0].Detail, "unknown rather than moved") {
		t.Errorf("the unchecked citation was not disclosed:\n%s", got[0].Detail)
	}
}

// TestVanishedPathIsStale: a path, unlike a bare name, is a claim about THIS
// repository by the gate's own convention.
func TestVanishedFileIsStale(t *testing.T) {
	co := checkout(t, map[string]string{"internal/keep.go": lines(10, nil)})
	got := scan(t, co, item{
		Number: 400, Title: "balance trigger is statement-scoped", CreatedAt: old(),
		Body: "See `internal/removed/trigger.go:12`.",
	})
	if len(got) != 1 {
		t.Fatalf("a citation into a deleted file was not reported: %+v", got)
	}
	// The reader must be able to tell this apart from a finding inside a file
	// the checkout actually holds: only the second is settled. A path this repo
	// does not have may equally belong to a dependency.
	if got[0].Attrs["path_absent"] != "1" || got[0].Attrs["moved_in_located_file"] != "0" {
		t.Errorf("evidence classes not separated: %+v", got[0].Attrs)
	}
	if !strings.Contains(got[0].Detail, "dependency or sibling repository") {
		t.Errorf("detail does not disclose that this class is ambiguous:\n%s", got[0].Detail)
	}
}

// TestBodyWithoutCitationsIsNotJudged: the scan reports that a claim moved. It
// cannot report that about a body which never made a checkable claim, and
// guessing would flood the survey with prose issues.
func TestBodyWithoutCitationsIsNotJudged(t *testing.T) {
	co := checkout(t, map[string]string{"internal/a.go": lines(10, nil)})
	got := scan(t, co, item{
		Number: 9, Title: "the dashboard feels slow", CreatedAt: old(),
		Body: "Users report the dashboard feels slow in the afternoon.",
	})
	if len(got) != 0 {
		t.Fatalf("an uncitable body was judged: %+v", got)
	}
}

func TestRecentIssuesAreSkipped(t *testing.T) {
	co := checkout(t, map[string]string{"internal/a.go": lines(10, nil)})
	s := New(Options{
		Owner: "example-org", Repos: []string{"payments-api"}, Checkouts: co,
		MinAge: 7 * 24 * time.Hour,
		Run: runnerFor(t, item{
			Number: 500, Title: "filed this morning", CreatedAt: time.Now().Add(-2 * time.Hour),
			Body: "`internal/gone.go:99` is wrong.",
		}),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("a body written today was re-checked against today's tree: %+v", got)
	}
}

// TestMissingCheckoutIsAnErrorNotASilentSkip: reporting "no stale premises" for
// a repository that was never examined is the failure this whole source exists
// to prevent, applied to itself.
func TestMissingCheckoutIsAnErrorNotASilentSkip(t *testing.T) {
	co := checkout(t, map[string]string{"internal/a.go": lines(10, nil)})
	s := New(Options{
		Owner: "example-org",
		Repos: []string{"payments-api", "analytics-service"},
		Run:   runnerFor(t), Checkouts: co,
	})
	_, err := s.Collect(context.Background(), signal.Scope{})
	if err == nil {
		t.Fatal("a repository with no checkout was silently skipped")
	}
	if !strings.Contains(err.Error(), "analytics-service") {
		t.Errorf("error does not name the unexaminable repository: %v", err)
	}
}

func TestNoCheckoutsAtAllIsAnError(t *testing.T) {
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: runnerFor(t)})
	if _, err := s.Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("scanning with no checkouts succeeded")
	}
}

func TestProviderFailurePropagates(t *testing.T) {
	co := checkout(t, map[string]string{"a.go": lines(3, nil)})
	s := New(Options{Owner: "o", Repos: []string{"payments-api"}, Checkouts: co,
		Run: func(context.Context, ...string) ([]byte, error) {
			return nil, fmt.Errorf("gh: not authenticated")
		}})
	if _, err := s.Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("an unreachable tracker read as no stale premises")
	}
}

func TestDiscoverCheckoutsFindsGitDirectories(t *testing.T) {
	root := t.TempDir()
	for _, r := range []string{"payments-api", "analytics-service"} {
		if err := os.MkdirAll(filepath.Join(root, r, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "not-a-repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverCheckouts(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["payments-api"] == "" || got["analytics-service"] == "" {
		t.Fatalf("discovery = %v, want the two git checkouts only", got)
	}
	if _, err := DiscoverCheckouts(filepath.Join(root, "not-a-repo")); err == nil {
		t.Error("a directory with no checkouts returned success")
	}
}

// TestOneUnreadableRepositoryDoesNotSilenceTheScan: the others are still
// examined and returned beside the error that names what was not.
func TestOneUnreadableRepositoryDoesNotSilenceTheScan(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	run := func(_ context.Context, args ...string) ([]byte, error) {
		repo := args[3]
		switch repo {
		case "acme/broken":
			return nil, errors.New("gh: could not resolve to a Repository")
		case "acme/quiet":
			return nil, errors.New("the 'acme/quiet' repository has disabled issues")
		}
		return []byte(`[{"number":7,"title":"t","body":"see a.go:99","url":"u","createdAt":"` + old + `","updatedAt":"` + old + `"}]`), nil
	}
	src := New(Options{Owner: "acme", Repos: []string{"broken", "quiet", "ok"},
		Checkouts: map[string]string{"broken": dir, "quiet": dir, "ok": dir}, Run: run})
	got, err := src.Collect(context.Background(), signal.Scope{})
	if err == nil || !strings.Contains(err.Error(), "broken") || strings.Contains(err.Error(), "quiet") {
		t.Fatalf("err = %v; want the broken repository named and the quiet one not", err)
	}
	if len(got) != 1 || got[0].Repo != "ok" {
		t.Fatalf("the readable repository was not scanned: %+v", got)
	}
}

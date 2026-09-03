// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package partner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const sample = `# Partnership: alex
discovered: 2030-03-02T15:40:00Z
adopted: 2030-03-02 by Alex

## How we work {DECLARED}

Bring me the decision, not the question. Challenge the premise before the code.

## Rhythm {DISCOVERED}

Commits cluster 21:00-02:00 on their own clock, offset -04:00 — payments-api,
42 commits between 2030-01-01 and 2030-03-01

## Reading {INFERRED}

Reviews land in the morning, so a report filed overnight is read before work starts.

## Open questions {UNKNOWN}

How much unfinished thinking is welcome in a report? Them saying which would settle it.
`

func parse(t *testing.T, raw string) *Partnership {
	t.Helper()
	p, err := Parse("alex", raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestCheckPassesAWellFormedPartnership(t *testing.T) {
	p := parse(t, sample)
	if got := Check(p, p.DiscoveredAt.Add(time.Hour), 90*24*time.Hour); len(got) != 0 {
		t.Errorf("a well-formed partnership produced findings: %v", got)
	}
}

// TestDescribingAPersonIsNotRestatingPolicy. The check must not fire on the
// ordinary content of a relationship document, or it teaches people to omit
// the context that makes it useful.
func TestDescribingAPersonIsNotRestatingPolicy(t *testing.T) {
	p := parse(t, strings.Replace(sample,
		"Bring me the decision, not the question. Challenge the premise before the code.",
		"He deploys on Friday afternoons and merges his own work; expect a migration most weeks.", 1))
	if hasFinding(Check(p, time.Now(), 0), "authority list") {
		t.Error("describing what the partner does was reported as restating policy")
	}
}

// TestTextIsFramedAsContextNotIdentity. A partnership enriches the engineer's
// persona in context; it never redefines it, and the reader is told so before
// reading a word of it.
func TestTextIsFramedAsContextNotIdentity(t *testing.T) {
	txt := Text(parse(t, sample), time.Now())
	if !strings.Contains(txt, "context, not identity") {
		t.Errorf("a partnership is rendered without its framing:\n%s", txt)
	}
	if strings.Index(txt, "context, not identity") > strings.Index(txt, "How we work") {
		t.Error("the framing arrives after the content it is supposed to frame")
	}
}

const log = "Alex\talex@example.com\t2030-02-20T23:14:02-04:00\tAlex\n" +
	"Alex\talex@example.com\t2030-02-21T01:02:11-04:00\tAlex\n" +
	"Morgan\tmorgan@example.com\t2030-02-21T09:30:00+01:00\tAlex\n"

func TestDiscoverEstablishesWhereAndWhenNotWho(t *testing.T) {
	ev, err := Discover(context.Background(), DiscoverOptions{
		WorkRoot: "/w", Repos: []string{"svc"}, WindowDays: 30,
		Run: func(_ context.Context, _, _ string, _ ...string) (string, error) { return log, nil },
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(ev.People) != 2 {
		t.Fatalf("got %d identities, want 2: %+v", len(ev.People), ev.People)
	}

	alex := ev.People[0]
	if alex.Name != "Alex" || alex.Commits != 2 {
		t.Errorf("busiest identity = %s with %d commits", alex.Name, alex.Commits)
	}
	// The hour is theirs, not UTC: 23:14-04:00 is late evening where they were,
	// and converting it would report them as a morning person.
	if alex.HoursLocal[23] != 1 || alex.HoursLocal[1] != 1 {
		t.Errorf("local hours lost the commit's own offset: %v", alex.HoursLocal)
	}
	if len(alex.Offsets) != 1 || alex.Offsets[0] != "-04:00" {
		t.Errorf("offsets = %v, want [-04:00]", alex.Offsets)
	}
	if alex.Landed != 1 {
		t.Errorf("landed = %d, want 1 — they committed Morgan's work", alex.Landed)
	}

	txt := ev.Text()
	if !strings.Contains(txt, "not how they want to be worked with") {
		t.Error("evidence does not disclaim what it cannot establish")
	}
}

func TestDiscoverNarrowsToOnePerson(t *testing.T) {
	ev, err := Discover(context.Background(), DiscoverOptions{
		WorkRoot: "/w", Repos: []string{"svc"}, Person: "morgan@example.com",
		Run: func(_ context.Context, _, _ string, _ ...string) (string, error) { return log, nil },
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(ev.People) != 1 || ev.People[0].Name != "Morgan" {
		t.Errorf("narrowing by email returned %+v", ev.People)
	}
}

// TestAnUnreadableRepositoryIsReportedNotSwallowed: a thin picture must never
// be mistaken for a quiet estate.
func TestAnUnreadableRepositoryIsReportedNotSwallowed(t *testing.T) {
	ev, err := Discover(context.Background(), DiscoverOptions{
		WorkRoot: "/w", Repos: []string{"good", "broken"},
		Run: func(_ context.Context, dir, _ string, _ ...string) (string, error) {
			if strings.HasSuffix(dir, "broken") {
				return "", errors.New("not a git repository")
			}
			return log, nil
		},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(ev.Failures) != 1 || !strings.Contains(ev.Failures[0], "broken") {
		t.Errorf("failures = %v, want the unreadable repository named", ev.Failures)
	}
	if !strings.Contains(ev.Text(), "Could not examine") {
		t.Error("the rendered evidence hides what it could not read")
	}
}

// TestSharedAddressIsReportedNotMerged. One address under two names is either
// one person whose git config drifted or two people sharing a credential, and
// merging the second invents a person. The evidence asks rather than decides.
func TestSharedAddressIsReportedNotMerged(t *testing.T) {
	shared := "Alex\tshared@example.com\t2030-02-20T23:14:02-04:00\tAlex\n" +
		"A. Example\tshared@example.com\t2030-02-21T01:02:11-04:00\tAlex\n" +
		"Morgan\tshared@example.com\t2030-02-21T09:30:00-04:00\tMorgan\n"
	ev, err := Discover(context.Background(), DiscoverOptions{
		WorkRoot: "/w", Repos: []string{"svc"},
		Run: func(_ context.Context, _, _ string, _ ...string) (string, error) { return shared, nil },
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(ev.People) != 3 {
		t.Errorf("identities were merged on a shared address: %d remain", len(ev.People))
	}
	if len(ev.Overlaps) != 1 || ev.Overlaps[0].Email != "shared@example.com" {
		t.Fatalf("overlaps = %+v, want the shared address named", ev.Overlaps)
	}
	if len(ev.Overlaps[0].Names) != 3 {
		t.Errorf("overlap names = %v, want all three", ev.Overlaps[0].Names)
	}
	if !strings.Contains(ev.Text(), "must not be resolved by guessing") {
		t.Error("the rendered evidence presents an unresolved identity as settled")
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

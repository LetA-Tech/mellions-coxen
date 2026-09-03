// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return at
}

func TestAHostWhoseOwnerNeverSaidIsUnknownRatherThanAttended(t *testing.T) {
	got, err := readOwner(filepath.Join(t.TempDir(), "owner"))
	if err != nil {
		t.Fatalf("a missing marker is a state, not a failure: %v", err)
	}
	if got != nil {
		t.Fatalf("a missing marker was read as a recorded state: %+v", got)
	}
	if p := ownerPresence(nil, time.Now()); p != nil {
		t.Fatalf("nothing recorded became a presence the session would act on: %+v", p)
	}
}

func TestTheMarkerSurvivesBeingWrittenAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner")
	want := ownerState{
		State:   ownerAway,
		Since:   mustParse(t, "2026-08-29T22:10:00Z"),
		Until:   mustParse(t, "2026-08-30T08:00:00Z"),
		Because: "overnight",
	}
	if err := writeOwner(path, want); err != nil {
		t.Fatalf("writeOwner: %v", err)
	}
	got, err := readOwner(path)
	if err != nil {
		t.Fatalf("readOwner: %v", err)
	}
	if got.State != want.State || !got.Since.Equal(want.Since) ||
		!got.Until.Equal(want.Until) || got.Because != want.Because {
		t.Fatalf("the marker did not come back as it went in: %+v", got)
	}
}

func TestAMarkerNothingWroteIsRefusedRatherThanReadAsAttended(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner")
	if err := writeFile(t, path, "state: possibly\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := readOwner(path); err == nil {
		t.Fatal("a marker with no state I recognise was accepted; every reader would then infer one")
	}
}

func TestAnAwayWindowThatRanOutIsAttendedAgain(t *testing.T) {
	s := &ownerState{
		State: ownerAway,
		Since: mustParse(t, "2026-08-29T22:10:00Z"),
		Until: mustParse(t, "2026-08-30T08:00:00Z"),
	}
	if p := ownerPresence(s, mustParse(t, "2026-08-30T07:59:00Z")); !p.Away || p.Lapsed {
		t.Fatalf("inside the window the host was not away: %+v", p)
	}
	p := ownerPresence(s, mustParse(t, "2026-08-30T08:00:00Z"))
	if p.Away {
		t.Fatalf("the window ran out and the host was still away: %+v", p)
	}
	if !p.Lapsed {
		t.Fatalf("a lapsed window read as the owner having said they were back: %+v", p)
	}
}

func TestUntilTakesTheThreeFormsSomebodyTypesOnTheWayOut(t *testing.T) {
	now := mustParse(t, "2026-08-29T22:10:00Z")
	cases := []struct {
		in   string
		want string
	}{
		{"8h", "2026-08-30T06:10:00Z"},
		{"08:00", "2026-08-30T08:00:00Z"}, // the next occurrence, not this morning's
		{"23:30", "2026-08-29T23:30:00Z"}, // still to come today
		{"2026-08-30T09:00:00Z", "2026-08-30T09:00:00Z"},
	}
	for _, c := range cases {
		got, err := parseUntil(c.in, now)
		if err != nil {
			t.Errorf("-until %q: %v", c.in, err)
			continue
		}
		if stampUTC(got) != c.want {
			t.Errorf("-until %q became %s, not %s", c.in, stampUTC(got), c.want)
		}
	}
	if _, err := parseUntil("2026-08-29T20:00:00Z", now); err == nil {
		t.Error("a time already past was accepted; the away state would read as attended the moment it was written")
	}
	if _, err := parseUntil("tomorrow", now); err == nil {
		t.Error("a form nothing parses was accepted, and would have been written as a zero time — no window at all")
	}
}

func writeFile(t *testing.T, path, body string) error {
	t.Helper()
	return os.WriteFile(path, []byte(body), 0o644)
}

// The marker is what three readers agree on, so the shape the runner parses in
// shell is asserted here rather than left to the shell test alone: `state:` and
// `until:` at the start of a line, and a stamp whose string order is its time
// order, which is what the runner's `[[ "$now" < "$until" ]]` stands on.
func TestTheMarkerIsWrittenInTheFormTheRunnerParses(t *testing.T) {
	body := ownerState{
		State: ownerAway,
		Since: mustParse(t, "2026-08-29T22:10:00Z"),
		Until: mustParse(t, "2026-08-30T08:00:00Z"),
	}.String()
	for _, want := range []string{"state: away\n", "until: 2026-08-30T08:00:00Z\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("the runner's sed would not find %q in:\n%s", want, body)
		}
	}
	early, late := stampUTC(mustParse(t, "2026-08-30T08:00:00Z")), stampUTC(mustParse(t, "2026-08-30T09:00:00Z"))
	if !(early < late) {
		t.Errorf("%s does not sort before %s, so the runner's window check is comparing the wrong way", early, late)
	}
}

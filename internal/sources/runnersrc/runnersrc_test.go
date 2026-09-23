// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package runnersrc

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

func home(t *testing.T, log string) string {
	t.Helper()
	h := t.TempDir()
	if err := os.MkdirAll(filepath.Join(h, "shifts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if log != "" {
		if err := os.WriteFile(filepath.Join(h, "shifts", "runner.log"), []byte(log), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func collect(t *testing.T, h string) []signal.Signal {
	t.Helper()
	got, err := New(h, "").Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// Lines copied from a runner.log in which three failures followed a landed
// update, interleaved with the shift lines that are not update attempts.
const failingAfterLanded = `2026-09-17T03:36:56Z update ok: 33ba51e pulled, built and checked; the binary is at /srv/bin/mellions
2026-09-17T03:37:01Z shift start: 20260917-033701
2026-09-18T00:01:02Z update: 33ba51e is what runs already
2026-09-22T20:43:10Z update failed at git pull --ff-only in /srv/mellions-coxen; the binary that runs stays — /srv/mellions/shifts/runner-update.log
2026-09-23T00:35:43Z update failed at make -C /srv/mellions-coxen check; the binary that runs stays — /srv/mellions/shifts/runner-update.log
2026-09-23T00:36:00Z shift start: 20260923-003600
2026-09-23T01:20:21Z update failed at make -C /srv/mellions-coxen check; the binary that runs stays — /srv/mellions/shifts/runner-update.log
`

func TestAFailedLatestUpdateIsReported(t *testing.T) {
	h := home(t, failingAfterLanded)
	if err := os.WriteFile(filepath.Join(h, "shifts", "runner.installed"), []byte("33ba51e\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := collect(t, h)
	if len(got) != 1 {
		t.Fatalf("want one signal for a runner whose latest update failed, got %d", len(got))
	}
	s := got[0]
	if s.Kind != signal.KindBuild || s.Source != "runner" || s.ID != "runner-update" {
		t.Errorf("signal is %s/%s/%s", s.Kind, s.Source, s.ID)
	}
	for k, want := range map[string]string{
		"failed_since_landed": "3",
		"installed":           "33ba51e",
		"last_landed":         "2026-09-18T00:01:02Z update: 33ba51e is what runs already",
		"last_failure":        "update failed at make -C /srv/mellions-coxen check; the binary that runs stays — /srv/mellions/shifts/runner-update.log",
	} {
		if s.Attrs[k] != want {
			t.Errorf("attr %s = %q, want %q", k, s.Attrs[k], want)
		}
	}
	if !strings.Contains(s.Title, "last 3 update(s)") || !strings.Contains(s.Title, "2026-09-18T00:01:02Z") {
		t.Errorf("title does not say how many failed and when one last landed: %q", s.Title)
	}
	if s.Updated.UTC().Format("2006-01-02T15:04:05Z") != "2026-09-23T01:20:21Z" {
		t.Errorf("Updated = %s, want the latest failure", s.Updated)
	}
}

func TestALandedLatestUpdateIsSilent(t *testing.T) {
	for name, tail := range map[string]string{
		"built":   "2026-09-23T01:52:16Z update ok: adfbff0 pulled, built and checked; the binary is at /srv/bin/mellions\n",
		"already": "2026-09-23T02:10:00Z update: adfbff0 is what runs already\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := collect(t, home(t, failingAfterLanded+tail)); len(got) != 0 {
				t.Fatalf("a runner whose latest update landed reported %d signal(s): %+v", len(got), got)
			}
		})
	}
}

func TestFailuresWithNothingLandedSaySo(t *testing.T) {
	log := "2026-09-06T00:01:23Z update failed at make -C /x check; the binary that runs stays — /y\n"
	got := collect(t, home(t, log))
	if len(got) != 1 || !strings.Contains(got[0].Title, "no update has landed") || got[0].Attrs["failed_since_landed"] != "1" {
		t.Fatalf("got %+v", got)
	}
}

func TestNoRunnerLogIsNoSignal(t *testing.T) {
	if got := collect(t, home(t, "")); len(got) != 0 {
		t.Fatalf("a host with no runner reported %+v", got)
	}
}

func TestAnUnreadableLogIsAnError(t *testing.T) {
	h := home(t, "")
	if err := os.Mkdir(filepath.Join(h, "shifts", "runner.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(h, "").Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("a runner.log that cannot be read was reported as a healthy runner")
	}
}

// TestEveryUpdateEventTheRunnerWritesIsRead reads the update lines out of
// scripts/shifts.sh itself, so a new or reworded event cannot fall outside
// parse unnoticed.
func TestEveryUpdateEventTheRunnerWritesIsRead(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "shifts.sh"))
	if err != nil {
		t.Fatal(err)
	}
	events := regexp.MustCompile(`(?m)^\s*log "(update[^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(events) < 6 {
		t.Fatalf("found %d update events in shifts.sh, expected at least 6", len(events))
	}
	for _, m := range events {
		u, found := parse("2026-09-23T00:00:00Z " + m[1])
		if !found {
			t.Errorf("shifts.sh writes %q and parse does not read it as an update attempt", m[1])
			continue
		}
		if want := !strings.HasPrefix(m[1], "update failed"); u.ok != want {
			t.Errorf("%q read as landed=%v", m[1], u.ok)
		}
	}
}

func TestARunningBinaryTheRunnerDidNotInstallIsReported(t *testing.T) {
	landed := failingAfterLanded + "2026-09-23T01:52:16Z update ok: adfbff0 pulled, built and checked; the binary is at /srv/bin/mellions\n"
	h := home(t, landed)
	if err := os.WriteFile(filepath.Join(h, "shifts", "runner.installed"), []byte("adfbff0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for running, want := range map[string]int{
		"34cfbb0":     1,
		"adfbff0":     0,
		"adfbff04a9d": 0,
		"":            0,
		"unknown":     0,
	} {
		got, err := New(h, running).Collect(context.Background(), signal.Scope{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want {
			t.Errorf("running %q: %d signal(s), want %d: %+v", running, len(got), want, got)
			continue
		}
		if want == 1 && (got[0].ID != "runner-divergence" || got[0].Attrs["running"] != running || got[0].Attrs["installed"] != "adfbff0") {
			t.Errorf("running %q: %+v", running, got[0])
		}
	}
}

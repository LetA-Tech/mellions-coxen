// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

func TestClockLineReadsTheDeadlineRatherThanEstimating(t *testing.T) {
	now := time.Date(2026, 8, 28, 19, 12, 21, 0, time.UTC)
	deadline := now.Add(35 * time.Minute)
	got := clockLine(now, " "+itoa(deadline.Unix())+"\n")
	want := "clock: 19:12 UTC · deadline 19:47 UTC · 35 min left"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestClockLineSaysWhenTheDeadlinePassed(t *testing.T) {
	now := time.Date(2026, 8, 28, 19, 50, 0, 0, time.UTC)
	got := clockLine(now, itoa(now.Add(-3*time.Minute).Unix()))
	if !strings.Contains(got, "passed 3 min ago") || !strings.Contains(got, "write where the work stands") {
		t.Fatalf("got %q", got)
	}
}

func TestClockLineIsSilentWithoutADeadline(t *testing.T) {
	for _, env := range []string{"", "soon", "0", "-5"} {
		if got := clockLine(time.Now(), env); got != "" {
			t.Fatalf("MELLIONS_DEADLINE=%q said %q; nothing should be said", env, got)
		}
	}
}

func TestStateTextCarriesTheClockWithAndWithoutNotes(t *testing.T) {
	clock := "clock: 19:12 UTC · deadline 19:47 UTC · 35 min left"
	alone := stateText(nil, clock)
	if alone != "<mellions-state>\n"+clock+"\n</mellions-state>\n" {
		t.Fatalf("clock alone: %q", alone)
	}
	with := stateText([]awareness.Note{{Because: "a peer arrived", Do: "run mellions who"}}, clock)
	if !strings.Contains(with, "a peer arrived\n  run mellions who\n\n"+clock+"\n") {
		t.Fatalf("clock after notes: %q", with)
	}
	if stateText(nil, "") != "<mellions-state>\n</mellions-state>\n" {
		t.Fatalf("no notes and no clock still printed a body")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

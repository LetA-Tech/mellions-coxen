// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/pluginreg"
)

// A session is told which of three states it is in, and the middle one is the
// reason this exists: hooks are matched by the name the runtime records, so two
// registrations declaring the same names are indistinguishable by name. What
// separates them is when the session ran them.
func TestStaleAgainst(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	reg := func(s string) pluginreg.Registration {
		if s == "" {
			return pluginreg.Registration{}
		}
		return pluginreg.Registration{Registered: at(s)}
	}

	cases := []struct {
		name       string
		last       pluginreg.Event
		reg        pluginreg.Registration
		wantStale  bool
		wantReport time.Time
	}{{
		name:       "SessionStart before the registration is carrying the earlier copy",
		last:       pluginreg.Event{At: at("2026-08-28T16:58:07Z")},
		reg:        reg("2026-08-28T17:15:09Z"),
		wantStale:  true,
		wantReport: at("2026-08-28T16:58:07Z"),
	}, {
		name: "SessionStart after the registration is on it",
		last: pluginreg.Event{At: at("2026-08-28T17:20:00Z")},
		reg:  reg("2026-08-28T17:15:09Z"),
	}, {
		// Equal is not before: a session launched at the instant of the
		// registration got that registration.
		name: "SessionStart at the registration is on it",
		last: pluginreg.Event{At: at("2026-08-28T17:15:09Z")},
		reg:  reg("2026-08-28T17:15:09Z"),
	}, {
		// Neither of these may answer "stale". A registry missing its
		// timestamp, or a transcript entry without one, would otherwise report
		// every session on the machine as behind — a claim nothing observed.
		name: "no registration time is unknown, not stale",
		last: pluginreg.Event{At: at("2026-08-28T16:58:07Z")},
		reg:  reg(""),
	}, {
		name: "no SessionStart time is unknown, not stale",
		last: pluginreg.Event{},
		reg:  reg("2026-08-28T17:15:09Z"),
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stale, when := staleAgainst(c.last, c.reg)
			if stale != c.wantStale {
				t.Fatalf("stale = %v, want %v", stale, c.wantStale)
			}
			if c.wantStale && !when.Equal(c.wantReport) {
				t.Errorf("reported time = %s, want %s", when, c.wantReport)
			}
		})
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// Unattended is a state the owner enters and leaves, not a property of how a
// session was started.
//
// One file records which state this host is in, beside the pause and stop
// markers the runner already reads, so one act on the way out of the room
// reaches every session on the host, the runner and the digest at once. It is
// key: value lines, written by `mellions away` and `mellions back` and read by
// three places that must not disagree — this command, the awareness hook, and
// the runner, which parses it in shell so a stale binary cannot change what the
// host thinks the owner is doing.
//
//	state: away
//	since: 2026-08-29T22:10:00Z
//	until: 2026-08-30T08:00:00Z
//	because: overnight
//
// Absent is not "attended": a host whose owner has never said either is a host
// that has never been asked, and reading presence out of a missing file would
// be inventing an answer. Every reader treats absent as unknown and says so.

// ownerAway and ownerBack are the two states the marker records.
const (
	ownerAway = "away"
	ownerBack = "back"
)

// ownerMarker is where this host records where its owner is.
func (c *Config) ownerMarker() string { return filepath.Join(c.home(), "owner") }

// ownerState is the marker as written. Until is zero where the owner named no
// time to be back by.
type ownerState struct {
	State   string
	Since   time.Time
	Until   time.Time
	Because string
}

func (s ownerState) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "state: %s\n", s.State)
	fmt.Fprintf(&b, "since: %s\n", stampUTC(s.Since))
	if !s.Until.IsZero() {
		fmt.Fprintf(&b, "until: %s\n", stampUTC(s.Until))
	}
	if s.Because != "" {
		fmt.Fprintf(&b, "because: %s\n", strings.TrimSpace(s.Because))
	}
	return b.String()
}

// stampUTC is the one form every reader parses, the runner by string
// comparison — so seconds, UTC and Z are not cosmetic.
func stampUTC(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

// readOwner reads the marker. A missing file is (nil, nil): unknown, which is
// what every caller must distinguish from attended.
func readOwner(path string) (*ownerState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	s := &ownerState{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "state":
			s.State = val
		case "since":
			s.Since, _ = time.Parse(time.RFC3339, val)
		case "until":
			s.Until, _ = time.Parse(time.RFC3339, val)
		case "because":
			s.Because = val
		}
	}
	if s.State != ownerAway && s.State != ownerBack {
		return nil, fmt.Errorf("%s records no state I recognise (%q); `mellions away` or `mellions back` rewrites it", path, s.State)
	}
	return s, nil
}

func writeOwner(path string, s ownerState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s.String()), 0o644)
}

// ownerPresence is what the marker means now: an away window that has run out
// is attended again, because the owner said when they would be back and nothing
// has said otherwise. Nil in, nil out — unknown stays unknown.
func ownerPresence(s *ownerState, now time.Time) *awareness.OwnerPresence {
	if s == nil {
		return nil
	}
	p := &awareness.OwnerPresence{
		Away:    s.State == ownerAway,
		Since:   s.Since,
		Until:   s.Until,
		Because: s.Because,
	}
	if p.Away && !s.Until.IsZero() && !now.Before(s.Until) {
		p.Away, p.Lapsed = false, true
	}
	return p
}

// cmdAway marks this host away: nobody is reachable until someone says
// otherwise, and every session, the runner and the digest read it from the
// marker rather than guessing from how they were started.
func cmdAway(args []string) error {
	fs := newFlagSet("away", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	until := fs.String("until", "", "when you expect to be back: 8h, 22:30 (UTC, the next one), or a full 2006-01-02T15:04:05Z stamp. The away state lapses then")
	because := fs.String("because", "", "where you are going, for the sessions that read this")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	s := ownerState{State: ownerAway, Since: now, Because: *because}
	if *until != "" {
		if s.Until, err = parseUntil(*until, now); err != nil {
			return err
		}
	}
	if err := writeOwner(cfg.ownerMarker(), s); err != nil {
		return err
	}
	fmt.Printf("away since %s", stampUTC(s.Since))
	if !s.Until.IsZero() {
		fmt.Printf(", until %s", stampUTC(s.Until))
	}
	fmt.Printf(" — %s\n", cfg.ownerMarker())
	fmt.Println("Sessions on this host are told at their next turn; the runner starts shifts back to back. `mellions back` ends it.")
	return nil
}

// cmdBack marks this host attended again and prints what happened while it was
// away, from the marker's own since — the digest is where that already lives,
// and the return is what points at it.
func cmdBack(args []string) error {
	fs := newFlagSet("back", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	was, err := readOwner(cfg.ownerMarker())
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := writeOwner(cfg.ownerMarker(), ownerState{State: ownerBack, Since: now}); err != nil {
		return err
	}
	fmt.Printf("back at %s — %s\n", stampUTC(now), cfg.ownerMarker())
	fmt.Println("Sessions on this host are told at their next turn; the runner starts no shift until you are away again.")
	// The whole of it, unbounded, and the brief form's marker is left alone: a
	// person reading this is not the session-start hook, and consuming the
	// marker here would silence the next session that has not seen it.
	var since time.Time
	if was != nil && was.State == ownerAway {
		since = was.Since
	}
	fmt.Println()
	return digestSince(cfg, since, now, os.Stdout)
}

// digestSince is `report digest` from a moment the caller names rather than
// from the marker: what happened while the owner was away, which is what they
// want on the way back in.
func digestSince(cfg *Config, since, now time.Time, w io.Writer) error {
	lines := append(shiftLines(filepath.Join(cfg.home(), "shifts"), since),
		reportLines(cfg.reportsDir(), since)...)
	sortDigest(lines)
	owed := lanesNamingOwner(cfg)
	if len(lines) == 0 && owed == 0 {
		fmt.Fprintf(w, "nothing for the owner since %s\n", sinceText(since))
		return nil
	}
	_, err := io.WriteString(w, renderDigest(lines, owed, since, 0))
	return err
}

// parseUntil takes the three forms somebody types on the way out of the room:
// a duration from now, a UTC clock time meaning its next occurrence, and a full
// stamp. A time already past is refused rather than written, because an away
// window that has lapsed before it is recorded reads as attended everywhere.
func parseUntil(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	var at time.Time
	switch {
	case strings.Contains(s, "T"):
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("-until %q is not a 2006-01-02T15:04:05Z stamp: %w", s, err)
		}
		at = t.UTC()
	case strings.Contains(s, ":"):
		t, err := time.Parse("15:04", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("-until %q is not a HH:MM time: %w", s, err)
		}
		at = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
		if !at.After(now) {
			at = at.AddDate(0, 0, 1)
		}
	default:
		d, err := time.ParseDuration(s)
		if err != nil {
			return time.Time{}, fmt.Errorf("-until %q is not a duration, a HH:MM time or a stamp: %w", s, err)
		}
		at = now.Add(d)
	}
	if !at.After(now) {
		return time.Time{}, fmt.Errorf("-until %s is already past; an away window that has lapsed reads as attended everywhere", stampUTC(at))
	}
	return at, nil
}

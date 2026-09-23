// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package runnersrc reports whether the shift runner's last update of this
// host's Mellions landed.
//
// scripts/shifts.sh pulls, builds and checks the load-path checkout before
// every shift and writes one line per attempt to shifts/runner.log. A failed
// attempt leaves the previous binary running and is written nowhere else, so
// without this source a deploy can fail on every shift and no survey says so.
package runnersrc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Name is the source's name in configuration and in Signal.Source.
const Name = "runner"

// Source reads the runner's log under a Mellions home.
type Source struct{ shifts, running string }

// New reads home/shifts. running is the commit of the binary collecting, as
// `mellions version` prints it; "" or "unknown" skips the comparison with it.
func New(home, running string) *Source {
	return &Source{shifts: filepath.Join(home, "shifts"), running: running}
}

// Name implements signal.Source.
func (s *Source) Name() string { return Name }

// update is one attempt the runner logged.
type update struct {
	at   time.Time
	ok   bool
	text string
}

// Collect emits a build signal when the runner's latest update attempt failed,
// and another when the binary collecting is not the commit the runner last
// installed; nothing when no runner has logged here.
func (s *Source) Collect(_ context.Context, _ signal.Scope) ([]signal.Signal, error) {
	path := filepath.Join(s.shifts, "runner.log")
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	defer f.Close()

	var last, lastOK *update
	failed := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		u, found := parse(sc.Text())
		if !found {
			continue
		}
		last = &u
		if u.ok {
			lastOK, failed = &u, 0
		} else {
			failed++
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("runner: read %s: %w", path, err)
	}
	if last == nil {
		return nil, nil
	}
	installed := ""
	if b, err := os.ReadFile(filepath.Join(s.shifts, "runner.installed")); err == nil {
		installed = strings.TrimSpace(string(b))
	}
	var out []signal.Signal
	if !last.ok {
		out = append(out, failedSignal(path, last, lastOK, failed, installed))
	}
	if sig, ok := s.divergence(path, installed); ok {
		out = append(out, sig)
	}
	return out, nil
}

func failedSignal(path string, last, lastOK *update, failed int, installed string) signal.Signal {
	attrs := map[string]string{
		"log":                 path,
		"failed_since_landed": strconv.Itoa(failed),
		"last_failure":        last.text,
	}
	since := "no update has landed in this log"
	if lastOK != nil {
		attrs["last_landed"] = lastOK.at.UTC().Format(time.RFC3339) + " " + lastOK.text
		since = "the last one that landed was at " + lastOK.at.UTC().Format(time.RFC3339)
	}
	if installed != "" {
		attrs["installed"] = installed
	}
	return signal.Signal{
		Kind: signal.KindBuild, Source: Name, ID: "runner-update",
		Title: fmt.Sprintf("The shift runner's last %d update(s) of this host's Mellions failed; %s",
			failed, since),
		Updated: last.at,
		Attrs:   attrs,
		Detail: "The runner keeps the previous binary when an update fails, and the checkout it " +
			"pulled is already what sessions load. Latest: " + last.text +
			". The step's output is in shifts/runner-update.log.",
	}
}

// divergence reports a binary collecting that is not the commit the runner
// installed: the runner writes one path, and a session's PATH can resolve
// another that nothing updates. Short hashes of different lengths compare by
// prefix.
func (s *Source) divergence(path, installed string) (signal.Signal, bool) {
	running := strings.TrimSpace(s.running)
	if installed == "" || running == "" || running == "unknown" ||
		strings.HasPrefix(running, installed) || strings.HasPrefix(installed, running) {
		return signal.Signal{}, false
	}
	marker := filepath.Join(s.shifts, "runner.installed")
	var updated time.Time
	if fi, err := os.Stat(marker); err == nil {
		updated = fi.ModTime()
	}
	return signal.Signal{
		Kind: signal.KindBuild, Source: Name, ID: "runner-divergence",
		Title: fmt.Sprintf("The mellions running this survey is %s; the shift runner last installed %s",
			running, installed),
		Updated: updated,
		Attrs:   map[string]string{"log": path, "installed": installed, "running": running},
		Detail: "The runner updates the binary at the path it resolved; this one came from another " +
			"path, or was built by hand, and nothing the runner does moves it. `which -a mellions` " +
			"lists the candidates.",
	}, true
}

// parse reads one runner.log line, "<RFC3339> <event>", and reports whether it
// is an update attempt. "update: X is what runs already" is a landed update.
func parse(line string) (update, bool) {
	stamp, event, ok := strings.Cut(line, " ")
	if !ok {
		return update{}, false
	}
	at, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return update{}, false
	}
	switch {
	case strings.HasPrefix(event, "update failed"):
		return update{at: at, ok: false, text: event}, true
	case strings.HasPrefix(event, "update ok:"), strings.HasPrefix(event, "update: "):
		return update{at: at, ok: true, text: event}, true
	}
	return update{}, false
}

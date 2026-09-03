// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// The digest is what needs the owner, delivered where they already are. They
// read the tracker and the session in front of them, not the reports
// directory, so the session-start hook prints this — once, for every session
// on the host, with the marker's mtime saying when it was last said.

// digestWindow is how long one saying stands for every session on the host.
const digestWindow = 8 * time.Hour

// digestBudget bounds the hook's form, under the runtime's preview limit.
const digestBudget = 6 * 1024

// digestMarker is touched when a digest is said; its mtime is what "since"
// means for the next one.
func (c *Config) digestMarker() string { return filepath.Join(c.home(), "digest-seen") }

// digestLine is one thing the owner has not been told, and when it happened.
type digestLine struct {
	At   time.Time
	Text string
}

// reportDigest prints what needs the owner since the marker was last touched.
//
// The brief form is the hook's: silent inside the window, bounded, and the
// marker is touched only once something was said — a silent digest moves
// nothing, so the next session reads the same interval again. Without -brief
// it prints everything since the marker, unbounded, and leaves the marker
// alone: the form a person runs to see the whole of it.
func reportDigest(cfg *Config, brief bool, now time.Time, w io.Writer) error {
	var since time.Time
	if fi, err := os.Stat(cfg.digestMarker()); err == nil {
		since = fi.ModTime()
		if brief && now.Sub(since) < digestWindow {
			return nil
		}
	}
	lines := append(shiftLines(filepath.Join(cfg.home(), "shifts"), since),
		reportLines(cfg.reportsDir(), since)...)
	sortDigest(lines)
	owed := lanesNamingOwner(cfg)
	if len(lines) == 0 && owed == 0 {
		if !brief {
			fmt.Fprintf(w, "nothing for the owner since %s\n", sinceText(since))
		}
		return nil
	}
	budget := 0
	if brief {
		budget = digestBudget
	}
	if _, err := io.WriteString(w, renderDigest(lines, owed, since, budget)); err != nil {
		return err
	}
	if !brief {
		return nil
	}
	if err := os.MkdirAll(cfg.home(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfg.digestMarker(), []byte(now.UTC().Format(time.RFC3339)+"\n"), 0o644)
}

// sortDigest puts the most recent first, for whichever "since" the caller
// established — the marker's, or the moment the owner stepped away.
func sortDigest(lines []digestLine) {
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].At.After(lines[j].At) })
}

func renderDigest(lines []digestLine, owed int, since time.Time, budget int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# For the owner — since %s\n\n", sinceText(since))
	b.WriteString("What the unattended work produced that a person may need to see. The engineer " +
		"reads it as context, never as a task.\n\n")
	if owed > 0 {
		lanes, names := "lanes", "name"
		if owed == 1 {
			lanes, names = "lane", "names"
		}
		fmt.Fprintf(&b, "- %d handed-off %s %s you in the handoff — `mellions assign list`\n", owed, lanes, names)
	}
	shown := 0
	for _, l := range lines {
		// Room is left for the closing line, so the cut is said rather than
		// made by the hook's byte bound mid-line.
		if budget > 0 && b.Len()+len(l.Text)+80 > budget {
			break
		}
		b.WriteString(l.Text)
		b.WriteString("\n")
		shown++
	}
	if rest := len(lines) - shown; rest > 0 {
		fmt.Fprintf(&b, "… and %d more — `mellions report digest` has them all\n", rest)
	}
	return b.String()
}

func sinceText(since time.Time) string {
	if since.IsZero() {
		return "the beginning of the record"
	}
	return since.UTC().Format("2006-01-02 15:04 UTC")
}

// shiftLines is one line per finished shift: its stamp, the host, and what
// its reply led with. The shift files carry no host and the store is one
// machine's, so the host is this one.
func shiftLines(dir string, since time.Time) []digestLine {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	host, _ := os.Hostname()
	var out []digestLine
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".reply.md") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().After(since) {
			continue
		}
		stamp := strings.TrimSuffix(name, ".reply.md")
		raw, _ := os.ReadFile(filepath.Join(dir, name))
		said := replyGist(string(raw))
		if said == "" {
			said = "said nothing — " + stamp + ".log has the session"
		}
		out = append(out, digestLine{At: info.ModTime(), Text: fmt.Sprintf("- shift %s on %s: %s", stamp, host, said)})
	}
	return out
}

// replyGist is the first heading of a reply, or its first line where it has
// none. The reply is the session's own summary: its headings say what it did,
// where its first line is usually "Done".
func replyGist(reply string) string {
	first := ""
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			return clip(strings.TrimSpace(strings.TrimLeft(line, "#")), 140)
		}
		if first == "" {
			first = line
		}
	}
	return clip(first, 140)
}

// reportLines is one line per report that stopped on the owner: what its
// "Needs you" section opens with, or its "Blocked" one. Only the sections the
// flags write count; a body read from a file is one document, and nothing
// inside it says whether it needs a person.
func reportLines(dir string, since time.Time) []digestLine {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []digestLine
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().After(since) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var text string
		switch doc := string(raw); {
		case section(doc, "## Needs you") != "":
			text = "needs you: " + clip(section(doc, "## Needs you"), 200)
		case section(doc, "## Blocked") != "":
			text = "blocked: " + clip(section(doc, "## Blocked"), 200)
		default:
			continue
		}
		out = append(out, digestLine{At: info.ModTime(),
			Text: fmt.Sprintf("- report %s %s", strings.TrimSuffix(name, ".md"), text)})
	}
	return out
}

// section is the first non-empty line under a heading, or empty.
func section(doc, heading string) string {
	_, rest, ok := strings.Cut(doc, "\n"+heading+"\n")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			return ""
		}
		if line != "" {
			return line
		}
	}
	return ""
}

// lanesNamingOwner counts the handed-off lanes whose handoff names the owner.
//
// A heuristic, and said so: a handoff is prose, and the words a session uses
// when it stops at the owner's decision — "owner", "decision package", the
// partner's own name — are the only signal there is. It over- and
// under-counts, so the count is a reason to read `mellions assign list`,
// never a fact about any one lane.
func lanesNamingOwner(cfg *Config) int {
	store, err := assignment.NewStore(cfg.assignmentsRoot())
	if err != nil {
		return 0
	}
	open, err := store.List(false)
	if err != nil {
		return 0
	}
	words := []string{"owner", "decision package"}
	if slugs, err := slugsIn(cfg.partnersDir()); err == nil {
		words = append(words, slugs...)
	}
	n := 0
	for _, a := range open {
		if a.State != assignment.StateHandedOff {
			continue
		}
		handoff := strings.ToLower(a.Handoff)
		for _, w := range words {
			if strings.Contains(handoff, strings.ToLower(w)) {
				n++
				break
			}
		}
	}
	return n
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/durable"
)

// A report is what the owner reads instead of the session. It is derived from
// the record — the assignment, the branch, the pull request — and never
// authoritative over it: deleting every report loses nothing.
func cmdReport(args []string) error {
	if len(args) == 0 {
		return errors.New("report needs a verb: write, latest or digest")
	}
	fs := newFlagSet("report", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	assignment := fs.String("assignment", "", "assignment this concerns; -id and the first argument do the same")
	idFlag := fs.String("id", "", "assignment this concerns")
	did := fs.String("did", "", "what happened, with links to the durable artifacts")
	established := fs.String("established", "", "what changed about what the owner believes")
	blocked := fs.String("blocked", "", "what stopped, and on whom")
	next := fs.String("next", "", "what to pick up next, and why that")
	needsOwner := fs.String("needs-owner", "", "what genuinely needs the owner, with the recommendation")
	file := fs.String("file", "", "read the 'did' body from a file, or - for stdin")
	n := fs.Int("n", 3, "how many recent reports to show")
	brief := fs.Bool("brief", false, "digest: the session-start form — silent inside eight hours of the last one, bounded, and marked as said")
	rest, err := parsePositional(fs, args[1:])
	if err != nil {
		return err
	}
	named, extra := assignID(firstNonEmpty(*assignment, *idFlag), rest)
	if len(extra) > 0 {
		return fmt.Errorf("report %s takes one assignment id and its text in flags, and was given %d loose arguments: %s",
			args[0], len(extra)+1, strings.Join(rest, " "))
	}
	assignment = &named
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	dir := cfg.reportsDir()
	switch args[0] {
	case "write":
		body := *did
		if *file != "" {
			if body, err = readInput(*file); err != nil {
				return err
			}
		}
		if strings.TrimSpace(body) == "" && strings.TrimSpace(*needsOwner) == "" {
			return errors.New("a report says what happened (-did) or what needs the owner (-needs-owner)")
		}
		now := time.Now().UTC()
		name := now.Format("20060102-150405")
		if *assignment != "" {
			name += "-" + *assignment
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# %s", now.Format("2006-01-02 15:04 UTC"))
		if *assignment != "" {
			fmt.Fprintf(&b, " — %s", *assignment)
		}
		b.WriteString("\n")
		for _, sec := range []struct{ h, v string }{
			{"Needs you", *needsOwner},
			{"What changed about what you believe", *established},
			{"What I did", body},
			{"Blocked", *blocked},
			{"Next, and why that", *next},
		} {
			if strings.TrimSpace(sec.v) == "" {
				continue
			}
			fmt.Fprintf(&b, "\n## %s\n\n%s\n", sec.h, strings.TrimSpace(sec.v))
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path, err := claimReportPath(dir, name)
		if err != nil {
			return err
		}
		if err := durable.Write(path, []byte(b.String()), 0o644); err != nil {
			releaseUnwrittenClaim(path)
			return err
		}
		fmt.Println(path)
		// Only the flags say what a report carries. A body read from -file is
		// one document, and the reassurance would be computed from flags that
		// are empty because they were never the input — printing it there tells
		// a report whose first section is "Needs you" that nothing needs the
		// owner, which is worse than saying nothing at all.
		//
		// -blocked counts for the same reason: it is what stopped and on whom,
		// so a report carrying one is not a run with nothing in it for the
		// owner, whatever the other flags hold.
		if *file == "" && strings.TrimSpace(*needsOwner) == "" && strings.TrimSpace(*established) == "" && strings.TrimSpace(*blocked) == "" {
			fmt.Println("\n(nothing here needs the owner)")
		}
		return nil
	case "latest":
		paths, err := latestReports(dir, *n)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			fmt.Println("no reports yet")
			return nil
		}
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n\n---\n\n", strings.TrimSpace(string(raw)))
		}
		return nil
	case "digest":
		return reportDigest(cfg, *brief, time.Now(), os.Stdout)
	default:
		return fmt.Errorf("report: unknown verb %q", args[0])
	}
}

func (c *Config) reportsDir() string { return filepath.Join(c.reportRoot(), "reports") }

// reportSuffixes bounds the disambiguation loop. A hundred reports written into
// one UTC second is far outside anything the shift script or a person produces,
// and refusing loudly is the only honest end to the loop: the alternative is
// choosing some report to destroy.
const reportSuffixes = 100

// reportStamp is the UTC second every report name starts with.
const reportStamp = "20060102-150405"

// claimReportPath reserves a free name under dir and returns it, unwritten.
//
// The name is a UTC second, so two reports written inside one second ask for
// the same path. O_EXCL makes the filesystem settle it instead of a clock: the
// creator owns the name, and a writer that loses takes the next suffix rather
// than replacing the winner. That holds across two processes, which a finer
// timestamp would only have made less likely.
//
// The reservation is an empty file, which durable.Write then replaces, so the
// content write keeps its stage-fsync-rename contract. A crash in between
// leaves an empty report. It costs a directory entry and nothing more only
// because both readers refuse it: the digest skips it for carrying no section,
// and latestReports skips it for its size. A reservation this returns an error
// for is removed, because the caller cannot; one the machine dies inside of
// cannot be, which is why the refusal has to live in the readers.
func claimReportPath(dir, name string) (string, error) {
	for n := 1; n <= reportSuffixes; n++ {
		candidate := name
		if n > 1 {
			candidate = fmt.Sprintf("%s-%d", name, n)
		}
		path := filepath.Join(dir, candidate+".md")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if err := f.Close(); err != nil {
				os.Remove(path)
				return "", fmt.Errorf("report: claim %s: %w", path, err)
			}
			return path, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("report: claim %s: %w", path, err)
		}
	}
	return "", fmt.Errorf("report: %s and its next %d suffixes are all taken in %s",
		name+".md", reportSuffixes-1, dir)
}

// releaseUnwrittenClaim gives a claimed name back when nothing was written
// under it, so a failed write does not leave an empty report behind.
//
// It removes the file only while it is still the empty one claimReportPath
// created. durable.Write commits by renaming and then flushes the directory,
// and the flush is what it returns — so it reports failure on a path whose
// content is already on disk and correct. Removing unconditionally would
// destroy a written report to tidy up after a directory flush, which is the
// defect the claim exists to prevent, reintroduced in the cleanup.
//
// A non-empty file is therefore kept and the error is what tells the writer
// the report may not have survived a crash.
func releaseUnwrittenClaim(path string) {
	if fi, err := os.Stat(path); err != nil || fi.Size() > 0 {
		return
	}
	os.Remove(path)
}

// reportOrder is a report name split into the second it was claimed for and the
// suffix claimReportPath gave it, so the two sort as one key.
//
// The name is still the ordering key. Sorting it as a plain string stopped
// working when collisions began producing siblings: "-" (0x2D) sorts below "."
// (0x2E), so 20260829-033306.md ranks above its own 20260829-033306-2 successor
// and `latest -n 1` answers with the older of the two. Only the suffix needs
// understanding; everything else about the order is unchanged, which is the
// point — every report written before this existed sorts exactly as it did.
//
// Modification time is not the key, and the reason is worth keeping: mtime is
// the last time the bytes changed, not the order they were written in. Reports
// get edited — a superseding banner added to one already on disk — and ordering
// by mtime promotes the edited report over the one that superseded it.
//
// A trailing -N counts as a suffix only where what precedes it is exactly the
// timestamp, and only for 2 through reportSuffixes with no leading zero.
//
// The narrowness is the point: "<name>-<suffix>" and "<timestamp>-<id ending
// in -N>" are the same string, so an id like "review-16" would otherwise be
// read as suffix 16 and sort a lane's report above sixteen it was written
// before. Requiring the bare timestamp makes that unreachable, at the price of
// ordering ten or more same-second reports under one id as text — which needs
// a tenth collision inside one second inside one lane, where the id case needs
// only a branch named the way this repository names branches.
func reportOrder(name string) (string, int) {
	base := strings.TrimSuffix(name, ".md")
	cut := strings.LastIndex(base, "-")
	if cut != len(reportStamp) {
		return base, 1
	}
	digits := base[cut+1:]
	if len(digits) == 0 || strings.HasPrefix(digits, "0") {
		return base, 1
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 2 || n > reportSuffixes {
		return base, 1
	}
	return base[:cut], n
}

func latestReports(dir string, n int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type report struct {
		name, base string
		suffix     int
	}
	var found []report
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		// Every report holds at least its own dated heading, so a zero-byte
		// file is a reservation whose content write never landed. latest
		// prints a report verbatim, and a reservation claimed after the
		// report it stands in front of sorts ahead of it — so it would answer
		// the owner's first command with a blank page while that report
		// stays on disk, unread.
		if info, err := e.Info(); err != nil || info.Size() == 0 {
			continue
		}
		base, suffix := reportOrder(e.Name())
		found = append(found, report{name: e.Name(), base: base, suffix: suffix})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].base != found[j].base {
			return found[i].base > found[j].base
		}
		return found[i].suffix > found[j].suffix
	})
	if n > 0 && len(found) > n {
		found = found[:n]
	}
	var out []string
	for _, r := range found {
		out = append(out, filepath.Join(dir, r.name))
	}
	return out, nil
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package survey assembles situational awareness from configured sources and
// renders it as the context an engineer reasons over.
//
// It runs sources concurrently, keeps what each one returned separate from what
// it failed to return, and renders both. It does not rank, filter by importance,
// or recommend. The output is evidence; the decision it informs belongs to the
// model reading it.
//
// The failure record is not incidental. A survey that silently drops a source
// tells the engineer "nothing is failing in CI" when the truth was "CI could not
// be reached", and those lead to opposite decisions.
package survey

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Failure records a source that could not be collected.
type Failure struct {
	Source string
	Err    error
}

// Result is one survey run.
type Result struct {
	// At is when the run started.
	At time.Time
	// Scope is what it was asked to look at.
	Scope signal.Scope
	// Signals is everything collected, deduplicated, in source order.
	Signals []signal.Signal
	// Failures is every source that could not be collected. A non-empty
	// Failures means the picture is incomplete and the reader must be told.
	Failures []Failure
	// Ran is the sources that were asked, in order.
	Ran []string
	// Elapsed is how long collection took.
	Elapsed time.Duration
}

// Complete reports whether every source answered.
func (r Result) Complete() bool { return len(r.Failures) == 0 }

// Runner collects from a set of sources.
type Runner struct {
	sources []signal.Source
	// Timeout bounds one source. A provider that hangs must not hold the whole
	// survey: partial evidence delivered on time beats complete evidence that
	// arrives after the decision was needed.
	Timeout time.Duration
}

// NewRunner returns a runner over the named sources, resolved from reg.
// An unknown name is an error rather than a skip, because a survey silently
// missing the source that would have shown the fire is worse than no survey.
func NewRunner(reg *signal.Registry, names []string) (*Runner, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("survey: no sources configured; available: %s",
			strings.Join(reg.Names(), ", "))
	}
	r := &Runner{Timeout: 60 * time.Second}
	for _, n := range names {
		s, ok := reg.Get(n)
		if !ok {
			return nil, fmt.Errorf("survey: unknown source %q; available: %s",
				n, strings.Join(reg.Names(), ", "))
		}
		r.sources = append(r.sources, s)
	}
	return r, nil
}

// Run collects from every source concurrently.
func (r *Runner) Run(ctx context.Context, scope signal.Scope) Result {
	started := time.Now()
	res := Result{At: started, Scope: scope}

	type outcome struct {
		idx     int
		name    string
		signals []signal.Signal
		err     error
	}
	out := make(chan outcome, len(r.sources))

	var wg sync.WaitGroup
	for i, src := range r.sources {
		res.Ran = append(res.Ran, src.Name())
		wg.Go(func() {
			sctx := ctx
			if r.Timeout > 0 {
				var cancel context.CancelFunc
				sctx, cancel = context.WithTimeout(ctx, r.Timeout)
				defer cancel()
			}
			// A source that panics is a bug in that source, not a reason to
			// lose every other source's evidence.
			defer func() {
				if p := recover(); p != nil {
					out <- outcome{idx: i, name: src.Name(), err: fmt.Errorf("panicked: %v", p)}
				}
			}()
			sigs, err := src.Collect(sctx, scope)
			out <- outcome{idx: i, name: src.Name(), signals: sigs, err: err}
		})
	}
	wg.Wait()
	close(out)

	// Reassemble in configured source order so two runs over unchanged state
	// render identically; concurrent completion order would make every survey
	// look different from the last one for no reason.
	collected := make([][]signal.Signal, len(r.sources))
	var failures []Failure
	for o := range out {
		if o.err != nil {
			failures = append(failures, Failure{Source: o.name, Err: o.err})
		}
		// Kept even alongside a failure. A source that reached nineteen
		// repositories and could not reach the twentieth has nineteen
		// repositories' worth of real signal, and discarding it turns one
		// unreachable corner into an estate that looks quiet.
		collected[o.idx] = o.signals
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].Source < failures[j].Source })

	var all []signal.Signal
	for _, batch := range collected {
		all = append(all, batch...)
	}

	res.Signals = signal.Dedupe(all)
	res.Failures = failures
	res.Elapsed = time.Since(started)
	return res
}

// Detail chooses how much of each signal a render prints.
type Detail int

const (
	// Brief prints one line per signal: repository, identifier, age, title.
	Brief Detail = iota
	// Full prints every field the source returned.
	Full
)

// Options bounds what a render prints. It bounds printing only. Every signal
// collected is still counted in the summary whatever these say, and the JSON
// form is not affected by any of them — so a bound can hide which item, never
// that items exist.
type Options struct {
	// Detail is how much of each signal to print.
	Detail Detail
	// PerRepo caps how many signals of one kind, from one repository, are
	// listed. What the cap held back is stated, with the command that prints
	// it. Zero means no cap.
	PerRepo int
	// Kinds, when non-empty, prints only these kinds. The summary still counts
	// every kind, so narrowing cannot make the rest look absent.
	Kinds []signal.Kind
	// CollectionLimit is the per-repository cap the sources ran under. A group
	// sitting exactly on it was truncated at collection, which is a different
	// and worse thing than a render cap: those items are not in this result at
	// all. Zero means no cap was applied.
	CollectionLimit int
}

// briefPerRepo is enough of a repository's list of one kind to see what sort of
// work it holds, and few enough that a twenty-repository estate stays readable
// in one pass. What it holds back is stated and one command away.
const briefPerRepo = 10

// Default is the form a session reads when it is choosing what to work on.
func Default() Options { return Options{Detail: Brief, PerRepo: briefPerRepo} }

// Everything prints every field of every signal collected.
func Everything() Options { return Options{Detail: Full} }

// Text renders the result in the default form.
func (r Result) Text() string { return r.Render(Default()) }

// Render writes the structured context a session reads.
//
// Grouped by kind for legibility, and explicitly not ordered by importance —
// the header says so, because a reader who assumes the first group matters most
// has been given a ranking nobody computed. Within a group the source's own
// order is preserved and nothing is reordered, scored or selected for merit.
//
// What Options bounds is how much of each signal is printed and how long a
// single repository's list may run. That is rendering, not collection: the
// counts in every heading and in the summary are of what was collected, and a
// bound that held something back says so and names the command that prints it.
// A survey nobody can read in the moment it is needed has failed at the one
// thing it exists for, and a survey that quietly shows less than it counted
// would be worse than either.
func (r Result) Render(o Options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Engineering state — %s\n\n", r.At.UTC().Format(time.RFC3339))

	fmt.Fprintf(&b, "Collected %d signals from %d sources in %s.\n",
		len(r.Signals), len(r.Ran), r.Elapsed.Round(time.Millisecond))
	if len(r.Scope.Repos) > 0 {
		fmt.Fprintf(&b, "Scope: %s\n", strings.Join(r.Scope.Repos, ", "))
	}
	b.WriteString("\nGrouped for reading only. Nothing here is ranked, scored or " +
		"recommended: deciding what matters now is yours.\n")

	if !r.Complete() {
		b.WriteString("\n## INCOMPLETE — sources that did not answer\n\n")
		b.WriteString("Treat these as unknown, never as empty. An absent signal here is " +
			"missing evidence, not the absence of a problem.\n\n")
		for _, f := range r.Failures {
			fmt.Fprintf(&b, "- **%s**: %v\n", f.Source, f.Err)
		}
	}

	byKind := signal.GroupByKind(r.Signals)
	b.WriteString(r.summary(o, byKind))

	for _, k := range signal.Kinds {
		batch := byKind[k]
		if len(batch) == 0 || !o.prints(k) {
			continue
		}
		fmt.Fprintf(&b, "\n## %s (%d)\n\n", kindHeading(k), len(batch))
		shown, held := bound(batch, o.PerRepo)
		for _, s := range shown {
			if o.Detail == Full {
				b.WriteString(renderSignal(s, r.At))
				continue
			}
			b.WriteString(renderBrief(s, r.At))
		}
		for _, h := range held {
			fmt.Fprintf(&b, "- … %d more collected in `%s`, not printed here: `mellions survey -full -repos %s -kind %s`\n",
				h.held, h.repo, h.repo, k)
		}
	}

	if len(r.Signals) == 0 && r.Complete() {
		b.WriteString("\nEvery source answered and none reported anything.\n")
	}
	return b.String()
}

// summary is what a reader orients on before reading any list: how much of what,
// where it is, what this form left out, and where the picture is bounded by
// something other than reality.
func (r Result) summary(o Options, byKind map[signal.Kind][]signal.Signal) string {
	if len(r.Signals) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## What was collected\n\n")

	// Named by the kind itself rather than by its heading, because that is the
	// token `-kind` takes: the count and the way to read more of it are the
	// same word.
	var kinds []string
	for _, k := range signal.Kinds {
		if n := len(byKind[k]); n > 0 {
			kinds = append(kinds, fmt.Sprintf("%d %s", n, k))
		}
	}
	fmt.Fprintf(&b, "By kind: %s.\n", strings.Join(kinds, " · "))

	byRepo := map[string]int{}
	for _, s := range r.Signals {
		if s.Repo != "" {
			byRepo[s.Repo]++
		}
	}
	if len(byRepo) > 0 {
		var repos []string
		for _, name := range slices.Sorted(maps.Keys(byRepo)) {
			repos = append(repos, fmt.Sprintf("%s %d", name, byRepo[name]))
		}
		fmt.Fprintf(&b, "\nBy repository, alphabetically: %s.\n", strings.Join(repos, " · "))
	}

	if at := r.atCollectionLimit(o.CollectionLimit, byKind); len(at) > 0 {
		fmt.Fprintf(&b, "\nSources ran with a limit of %d per repository. %s came back exactly "+
			"at it, so more of that exists than this run collected — those items are absent "+
			"from every count above, not merely unprinted. Ask the tracker directly before "+
			"concluding a repository is nearly clear.\n", o.CollectionLimit, strings.Join(at, ", "))
	}

	if o.Detail == Brief {
		b.WriteString("\nOne line per signal: repository, identifier, age, title. Labels, URLs, " +
			"attributes and bodies were collected and are not printed in this form — " +
			"`mellions survey -full` prints them, `-repos <name>` and `-kind <kind>` narrow to " +
			"a slice, `-json` gives everything a machine can read.\n")
	}
	if len(o.Kinds) > 0 {
		b.WriteString("\nNarrowed to " + kindList(o.Kinds) + ". The counts above are of the whole run.\n")
	}
	return b.String()
}

// atCollectionLimit names the groups sitting exactly on the source's own cap.
func (r Result) atCollectionLimit(limit int, byKind map[signal.Kind][]signal.Signal) []string {
	if limit <= 0 {
		return nil
	}
	var out []string
	for _, k := range signal.Kinds {
		perRepo := map[string]int{}
		for _, s := range byKind[k] {
			perRepo[s.Repo]++
		}
		for _, repo := range slices.Sorted(maps.Keys(perRepo)) {
			if repo != "" && perRepo[repo] >= limit {
				out = append(out, fmt.Sprintf("%s (%s)", repo, strings.ToLower(kindHeading(k))))
			}
		}
	}
	return out
}

func (o Options) prints(k signal.Kind) bool {
	return len(o.Kinds) == 0 || slices.Contains(o.Kinds, k)
}

// heldBack is one repository's list truncated by the render cap.
type heldBack struct {
	repo string
	held int
}

// bound caps how many signals one repository contributes to one section,
// preserving the source's order in what it keeps and reporting per repository
// what it did not keep. A repository with a long list cannot push every other
// repository's work off the end of a survey somebody has to read.
func bound(in []signal.Signal, perRepo int) ([]signal.Signal, []heldBack) {
	if perRepo <= 0 {
		return in, nil
	}
	count := map[string]int{}
	var kept []signal.Signal
	var order []string
	held := map[string]int{}
	for _, s := range in {
		if _, seen := count[s.Repo]; !seen {
			order = append(order, s.Repo)
		}
		count[s.Repo]++
		if count[s.Repo] <= perRepo {
			kept = append(kept, s)
			continue
		}
		held[s.Repo]++
	}
	var out []heldBack
	for _, repo := range order {
		if n := held[repo]; n > 0 {
			out = append(out, heldBack{repo: repo, held: n})
		}
	}
	return kept, out
}

// renderBrief is one signal as one line: what distinguishes it from every other
// signal, and nothing that a reader can derive or fetch. On this estate the
// fields it leaves out — labels, URL, attributes, body — were four fifths of the
// survey's bytes and none of them was what told one item from another.
func renderBrief(s signal.Signal, now time.Time) string {
	var b strings.Builder
	b.WriteString("- ")
	if s.Repo != "" {
		fmt.Fprintf(&b, "`%s` ", s.Repo)
	}
	if s.ID != "" {
		fmt.Fprintf(&b, "**%s** ", s.ID)
	}
	if age := s.Age(now); age > 0 {
		fmt.Fprintf(&b, "%s · ", humanAge(age))
	}
	// A title carrying a newline would break the list it sits in, and a source
	// is free to hand one over.
	b.WriteString(strings.Join(strings.Fields(s.Title), " "))
	if tail := briefReadiness(s); tail != "" {
		b.WriteString(tail)
	}
	b.WriteString("\n")
	return b.String()
}

// briefReadiness is the one thing a change set's title cannot say: whether it is
// finished work waiting for a reader, a stub, or one another lane is holding.
// Two change sets are told apart by their draft state, how much body they carry,
// whether anyone has reviewed them and whether a lane still holds them — so by
// this rule these belong in the brief line, not in the attributes a brief render
// drops. It costs a few bytes on change sets only; every other kind
// is untouched.
func briefReadiness(s signal.Signal) string {
	if s.Kind != signal.KindChangeSet {
		return ""
	}
	var parts []string
	if s.Attrs["draft"] == "true" {
		parts = append(parts, "draft")
	}
	if a := s.Attrs["author"]; a != "" {
		parts = append(parts, a)
	}
	if b := s.Attrs["body"]; b != "" {
		parts = append(parts, "body "+b)
	}
	if r := s.Attrs["reviews"]; r != "" {
		noun := " reviews"
		if r == "1" {
			noun = " review"
		}
		parts = append(parts, r+noun)
	}
	// Printed beside the review count, and only when there is something to
	// print, because "0 reviews" alone is read as "nobody has looked at this"
	// and in this estate that inference is wrong: repositories whose merge
	// gate is a recorded verdict POST it as a comment, so a twice-reviewed
	// change set still reports zero reviews. This does not claim a comment is
	// a review — it says the review count is not the whole record, which is
	// the one thing a reader choosing work off this line needs to know.
	if c := s.Attrs["comments"]; c != "" && c != "0" {
		noun := " comments"
		if c == "1" {
			noun = " comment"
		}
		parts = append(parts, c+noun)
	}
	// A held change set belongs here rather than in the attributes, for the
	// same reason the rest of this line does: a shift chooses from the brief
	// render, which drops attributes, and a marker only the full render prints
	// is invisible to the one reader it was built for.
	if s.Attrs["claimed"] != "" {
		parts = append(parts, "CLAIMED")
	}
	if len(parts) == 0 {
		return ""
	}
	return " · " + strings.Join(parts, " · ")
}

func renderSignal(s signal.Signal, now time.Time) string {
	var b strings.Builder
	b.WriteString("- ")
	if s.Repo != "" {
		fmt.Fprintf(&b, "`%s` ", s.Repo)
	}
	if s.ID != "" {
		fmt.Fprintf(&b, "**%s** ", s.ID)
	}
	b.WriteString(strings.TrimSpace(s.Title))

	var meta []string
	if age := s.Age(now); age > 0 {
		meta = append(meta, "age "+humanAge(age))
	}
	if len(s.Labels) > 0 {
		meta = append(meta, strings.Join(s.Labels, ", "))
	}
	for _, k := range slices.Sorted(maps.Keys(s.Attrs)) {
		meta = append(meta, k+"="+s.Attrs[k])
	}
	meta = append(meta, "via "+s.Source)
	fmt.Fprintf(&b, "  \n  _%s_", strings.Join(meta, " · "))
	if s.URL != "" {
		fmt.Fprintf(&b, "  \n  %s", s.URL)
	}
	if d := strings.TrimSpace(s.Detail); d != "" {
		fmt.Fprintf(&b, "  \n  %s", indent(trimDetail(s.Kind, d)))
	}
	b.WriteString("\n")
	return b.String()
}

// trimDetail bounds how much of a signal's body is rendered.
//
// Collection stays neutral: every source returns everything it found, in the
// order it found it, and nothing here reorders or drops a signal. What this
// bounds is the rendering, which is a different thing. A live run returned
// thirteen signals of which twelve were repository change, each carrying eight
// lines of raw commit log, and the log spent a large share of a context window
// restating what the signal's own title and attributes already say — the count,
// the branch, the window, whether anything is uncommitted.
//
// A repository change is the one kind whose body is a list rather than a
// statement, so it is the one kind bounded here. That is a fact the source
// declares about the signal, not a judgement made about it.
func trimDetail(k signal.Kind, detail string) string {
	if k != signal.KindCommit {
		return detail
	}
	lines := strings.Split(detail, "\n")
	if len(lines) <= commitLinesRendered {
		return detail
	}
	kept := lines[:commitLinesRendered]
	rest := len(lines) - commitLinesRendered
	return strings.Join(kept, "\n") +
		fmt.Sprintf("\n… %d more line(s) collected; `git log` in the repository has them", rest)
}

// commitLinesRendered is enough to see what a repository has been doing and not
// enough to bury the signals whose body is a statement.
const commitLinesRendered = 2

func kindList(ks []signal.Kind) string {
	out := make([]string, 0, len(ks))
	for _, k := range ks {
		out = append(out, string(k))
	}
	return strings.Join(out, ", ")
}

func indent(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n  ")
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func kindHeading(k signal.Kind) string {
	switch k {
	case signal.KindObjective:
		return "Program objectives"
	case signal.KindAssignment:
		return "Work already in flight"
	case signal.KindStalePremise:
		return "Stale premises — recorded claims the current tree contradicts"
	case signal.KindAlert:
		return "Runtime alerts"
	case signal.KindBuild:
		return "Failing checks"
	case signal.KindBlocked:
		return "Waiting on the owner"
	case signal.KindChangeSet:
		return "Changes under review"
	case signal.KindWorkItem:
		return "Tracked work items"
	case signal.KindFollowUp:
		return "Found while working"
	case signal.KindCommit:
		return "Recent repository change"
	default:
		return string(k)
	}
}

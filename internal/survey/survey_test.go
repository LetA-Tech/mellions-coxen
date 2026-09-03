// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package survey

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

type stub struct {
	name  string
	out   []signal.Signal
	err   error
	delay time.Duration
	panic bool
}

func (s stub) Name() string { return s.name }
func (s stub) Collect(ctx context.Context, _ signal.Scope) ([]signal.Signal, error) {
	if s.panic {
		panic("source bug")
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.out, s.err
}

func regWith(t *testing.T, sources ...signal.Source) *signal.Registry {
	t.Helper()
	r := signal.NewRegistry()
	for _, s := range sources {
		if err := r.Register(s); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestUnknownSourceIsAnErrorNotASkip(t *testing.T) {
	// A survey quietly missing the source that would have shown the fire is
	// worse than no survey at all.
	reg := regWith(t, stub{name: "github"})
	if _, err := NewRunner(reg, []string{"github", "gitlab"}); err == nil {
		t.Fatal("NewRunner accepted an unknown source")
	} else if !strings.Contains(err.Error(), "gitlab") {
		t.Errorf("error should name the unknown source, got: %v", err)
	}
}

func TestNoSourcesIsAnError(t *testing.T) {
	if _, err := NewRunner(regWith(t, stub{name: "a"}), nil); err == nil {
		t.Fatal("NewRunner accepted an empty source list")
	}
}

func TestFailureIsRecordedAndOtherSourcesStillCollect(t *testing.T) {
	boom := errors.New("gh: not authenticated")
	reg := regWith(t,
		stub{name: "ci", err: boom},
		stub{name: "github", out: []signal.Signal{{Kind: signal.KindWorkItem, Source: "github", ID: "1", Title: "a bug"}}},
	)
	r, err := NewRunner(reg, []string{"ci", "github"})
	if err != nil {
		t.Fatal(err)
	}
	res := r.Run(context.Background(), signal.Scope{})

	if res.Complete() {
		t.Error("Complete() is true despite a failed source")
	}
	if len(res.Failures) != 1 || res.Failures[0].Source != "ci" {
		t.Fatalf("failures = %+v, want one for ci", res.Failures)
	}
	if len(res.Signals) != 1 {
		t.Errorf("a failing source suppressed a healthy one: %+v", res.Signals)
	}

	txt := res.Text()
	if !strings.Contains(txt, "INCOMPLETE") {
		t.Error("rendered survey does not announce that it is incomplete")
	}
	if !strings.Contains(txt, "not authenticated") {
		t.Error("rendered survey hides why the source failed")
	}
	if !strings.Contains(txt, "never as empty") {
		t.Error("rendered survey does not warn that a missing source is unknown, not empty")
	}
}

// TestEmptyAndFailedRenderDifferently is the property the whole failure record
// exists for: "nothing is failing" and "could not reach CI" must never look the
// same to a reader.
func TestEmptyAndFailedRenderDifferently(t *testing.T) {
	quiet, err := NewRunner(regWith(t, stub{name: "ci"}), []string{"ci"})
	if err != nil {
		t.Fatal(err)
	}
	broken, err := NewRunner(regWith(t, stub{name: "ci", err: errors.New("unreachable")}), []string{"ci"})
	if err != nil {
		t.Fatal(err)
	}
	q := quiet.Run(context.Background(), signal.Scope{}).Text()
	b := broken.Run(context.Background(), signal.Scope{}).Text()

	if !strings.Contains(q, "none reported anything") {
		t.Errorf("a clean survey does not say so:\n%s", q)
	}
	if strings.Contains(q, "INCOMPLETE") {
		t.Error("a clean survey claims to be incomplete")
	}
	if !strings.Contains(b, "INCOMPLETE") {
		t.Errorf("a broken survey reads as clean:\n%s", b)
	}
}

func TestPanickingSourceIsContainedAsAFailure(t *testing.T) {
	reg := regWith(t,
		stub{name: "bad", panic: true},
		stub{name: "good", out: []signal.Signal{{Kind: signal.KindBuild, Source: "good", ID: "b1", Title: "red"}}},
	)
	r, err := NewRunner(reg, []string{"bad", "good"})
	if err != nil {
		t.Fatal(err)
	}
	res := r.Run(context.Background(), signal.Scope{})
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Err.Error(), "panicked") {
		t.Fatalf("panic was not contained as a failure: %+v", res.Failures)
	}
	if len(res.Signals) != 1 {
		t.Error("a panicking source lost another source's evidence")
	}
}

func TestSlowSourceTimesOutWithoutBlockingTheRest(t *testing.T) {
	reg := regWith(t,
		stub{name: "slow", delay: 2 * time.Second},
		stub{name: "fast", out: []signal.Signal{{Kind: signal.KindAlert, Source: "fast", ID: "a1", Title: "alarm"}}},
	)
	r, err := NewRunner(reg, []string{"slow", "fast"})
	if err != nil {
		t.Fatal(err)
	}
	r.Timeout = 50 * time.Millisecond

	start := time.Now()
	res := r.Run(context.Background(), signal.Scope{})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("a slow source held the survey for %v", elapsed)
	}
	if len(res.Failures) != 1 || res.Failures[0].Source != "slow" {
		t.Errorf("timeout was not recorded as a failure: %+v", res.Failures)
	}
	if len(res.Signals) != 1 {
		t.Errorf("the fast source's evidence was lost: %+v", res.Signals)
	}
}

// TestOutputIsDeterministicInSourceOrder guards against a survey that reads
// differently on every run purely because goroutines finished in a new order.
func TestOutputIsDeterministicInSourceOrder(t *testing.T) {
	mk := func() *Runner {
		reg := regWith(t,
			stub{name: "a", delay: 20 * time.Millisecond, out: []signal.Signal{{Kind: signal.KindWorkItem, Source: "a", ID: "1", Title: "from a"}}},
			stub{name: "b", out: []signal.Signal{{Kind: signal.KindWorkItem, Source: "b", ID: "2", Title: "from b"}}},
		)
		r, err := NewRunner(reg, []string{"a", "b"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := mk().Run(context.Background(), signal.Scope{})
	second := mk().Run(context.Background(), signal.Scope{})

	if first.Signals[0].Source != "a" || first.Signals[1].Source != "b" {
		t.Fatalf("signals are not in configured source order: %+v", first.Signals)
	}
	for i := range first.Signals {
		if first.Signals[i].Key() != second.Signals[i].Key() {
			t.Fatalf("two runs over identical state ordered differently: %v vs %v",
				first.Signals[i].Key(), second.Signals[i].Key())
		}
	}
}

// TestRenderedSurveyDisclaimsRanking is the user-visible half of the no-ranking
// invariant: the reader must not infer a priority order from the layout.
func TestRenderedSurveyDisclaimsRanking(t *testing.T) {
	reg := regWith(t, stub{name: "s", out: []signal.Signal{
		{Kind: signal.KindWorkItem, Source: "s", ID: "1", Title: "x"},
	}})
	r, err := NewRunner(reg, []string{"s"})
	if err != nil {
		t.Fatal(err)
	}
	txt := r.Run(context.Background(), signal.Scope{}).Text()
	for _, want := range []string{"Grouped for reading only", "deciding what matters now is yours"} {
		if !strings.Contains(txt, want) {
			t.Errorf("survey output is missing %q, so its grouping could be read as a ranking", want)
		}
	}
}

func TestRenderIncludesProvenanceAndDetail(t *testing.T) {
	now := time.Now()
	reg := regWith(t, stub{name: "stale", out: []signal.Signal{{
		Kind: signal.KindStalePremise, Source: "stale", ID: "223", Repo: "payments-api",
		Title:   "premise no longer holds",
		URL:     "https://example.invalid/223",
		Labels:  []string{"type:bug"},
		Attrs:   map[string]string{"cited": "postgres.go:820", "now": "postgres.go:934"},
		Detail:  "the cited line no longer matches",
		Updated: now.Add(-72 * time.Hour),
	}}})
	r, err := NewRunner(reg, []string{"stale"})
	if err != nil {
		t.Fatal(err)
	}
	txt := r.Run(context.Background(), signal.Scope{}).Render(Everything())
	for _, want := range []string{
		"payments-api", "223", "premise no longer holds", "via stale",
		"cited=postgres.go:820", "now=postgres.go:934",
		"https://example.invalid/223", "the cited line no longer matches", "3d",
	} {
		if !strings.Contains(txt, want) {
			t.Errorf("rendered signal is missing %q:\n%s", want, txt)
		}
	}
}

// TestARepositoryChangeDoesNotBuryTheRest.
//
// A live run returned thirteen signals, twelve of them repository change, each
// carrying eight lines of raw commit log. The one signal whose body was a
// statement sat underneath ninety-six lines of log that restated what the
// signals' own attributes already said.
func TestARepositoryChangeDoesNotBuryTheRest(t *testing.T) {
	var log []string
	for i := 0; i < 8; i++ {
		log = append(log, fmt.Sprintf("abc%d 2026-08-27 a commit subject", i))
	}
	commit := signal.Signal{
		Kind: signal.KindCommit, Source: "git", ID: "repo-a",
		Title: "8 commits in the last 7d", Repo: "repo-a",
		Attrs:  map[string]string{"commits": "8", "branch": "dev"},
		Detail: strings.Join(log, "\n"),
	}
	// The kind whose body is a statement is rendered whole.
	blocked := signal.Signal{
		Kind: signal.KindBlocked, Source: "assignments", ID: "d-1",
		Title: "waiting on the owner", Detail: strings.Join(log, "\n"),
	}

	out := Result{Signals: []signal.Signal{commit, blocked}}.Render(Everything())

	if strings.Count(out, "a commit subject") != commitLinesRendered+len(log) {
		t.Errorf("repository change was not bounded, or another kind was:\n%s", out)
	}
	if !strings.Contains(out, "more line(s) collected") {
		t.Errorf("what was collected and not rendered is not stated:\n%s", out)
	}
	// Collection is untouched: the signal is still there, still in order.
	if !strings.Contains(out, "8 commits in the last 7d") {
		t.Errorf("bounding the body dropped the signal:\n%s", out)
	}
}

// mkSignals builds n work items for one repository, in source order.
func mkSignals(repo string, n int) []signal.Signal {
	out := make([]signal.Signal, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, signal.Signal{
			Kind: signal.KindWorkItem, Source: "github", Repo: repo,
			ID:     fmt.Sprintf("#%d", i),
			Title:  fmt.Sprintf("item %d", i),
			URL:    fmt.Sprintf("https://example.invalid/%s/%d", repo, i),
			Labels: []string{"type:bug", "priority:p2", "repo:" + repo},
			Attrs:  map[string]string{"type": "issue"},
			Detail: "a body nobody needs to choose between items",
		})
	}
	return out
}

// TestBriefLeavesOutOnlyWhatItSaysItLeavesOut.
//
// The brief line keeps the fields that distinguish items while the header names
// the command that prints all omitted labels, URLs, attributes and bodies.
func TestBriefLeavesOutOnlyWhatItSaysItLeavesOut(t *testing.T) {
	out := Result{Signals: mkSignals("repo-a", 1)}.Render(Default())

	for _, want := range []string{"repo-a", "#1", "item 1", "mellions survey -full"} {
		if !strings.Contains(out, want) {
			t.Errorf("brief render is missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"https://example.invalid", "priority:p2", "type=issue", "a body nobody needs"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("brief render printed %q, which it exists not to print:\n%s", unwanted, out)
		}
	}
	if got := strings.Count(out, "\n- "); got != 1 {
		t.Errorf("one signal rendered as %d list lines, want 1:\n%s", got, out)
	}
}

// TestCapStatesWhatItHeldBackAndTheHeadingStaysHonest is the whole safety
// property of the render cap: it may decide how much of a repository's list is
// printed, never what the survey claims to have found.
func TestCapStatesWhatItHeldBackAndTheHeadingStaysHonest(t *testing.T) {
	res := Result{Signals: mkSignals("repo-a", 15)}
	out := res.Render(Options{Detail: Brief, PerRepo: 10})

	if !strings.Contains(out, "Tracked work items (15)") {
		t.Errorf("the heading does not state what was collected:\n%s", out)
	}
	if n := strings.Count(out, "\n- `repo-a`"); n != 10 {
		t.Errorf("cap printed %d items, want 10:\n%s", n, out)
	}
	if !strings.Contains(out, "5 more collected in `repo-a`") {
		t.Errorf("what the cap held back is not stated:\n%s", out)
	}
	if !strings.Contains(out, "-repos repo-a -kind work_item") {
		t.Errorf("the command that prints the rest is not given:\n%s", out)
	}
	// What it keeps is the source's own order, unreordered: the cap chooses how
	// many, never which are worth printing.
	for i := 1; i <= 10; i++ {
		if !strings.Contains(out, fmt.Sprintf("**#%d** item %d\n", i, i)) {
			t.Errorf("item %d is missing, so the cap did not keep the source's leading order:\n%s", i, out)
		}
	}
	for i := 11; i <= 15; i++ {
		if strings.Contains(out, fmt.Sprintf("item %d\n", i)) {
			t.Errorf("item %d was printed past the cap:\n%s", i, out)
		}
	}
	// Uncapped, the same result prints everything.
	if n := strings.Count(res.Render(Options{Detail: Brief}), "\n- `repo-a`"); n != 15 {
		t.Errorf("an uncapped render printed %d of 15", n)
	}
}

// TestCapIsPerRepositorySoOneBacklogCannotCrowdOutTheRest.
func TestCapIsPerRepositorySoOneBacklogCannotCrowdOutTheRest(t *testing.T) {
	sigs := append(mkSignals("loud", 40), mkSignals("quiet", 2)...)
	out := Result{Signals: sigs}.Render(Options{Detail: Brief, PerRepo: 3})

	if n := strings.Count(out, "\n- `quiet`"); n != 2 {
		t.Errorf("a small repository lost its work to a large one: %d of 2 printed\n%s", n, out)
	}
	if n := strings.Count(out, "\n- `loud`"); n != 3 {
		t.Errorf("the cap did not hold: %d printed, want 3", n)
	}
	if strings.Contains(out, "more collected in `quiet`") {
		t.Errorf("a repository under the cap was reported as held back:\n%s", out)
	}
}

// TestNarrowingByKindStillCountsEveryKind: a filter must never make the rest of
// the estate look absent.
func TestNarrowingByKindStillCountsEveryKind(t *testing.T) {
	sigs := append(mkSignals("repo-a", 2),
		signal.Signal{Kind: signal.KindBuild, Source: "github", Repo: "repo-a", ID: "CI", Title: "red"})
	out := Result{Signals: sigs}.Render(Options{Detail: Brief, Kinds: []signal.Kind{signal.KindBuild}})

	if strings.Contains(out, "item 1") {
		t.Errorf("a narrowed render printed a kind it was not asked for:\n%s", out)
	}
	if !strings.Contains(out, "2 work_item") {
		t.Errorf("the summary stopped counting a kind it did not print:\n%s", out)
	}
	if !strings.Contains(out, "Narrowed to build") {
		t.Errorf("the render does not say it was narrowed:\n%s", out)
	}
}

// TestARepositoryAtTheCollectionLimitIsNamed.
//
// A repository whose issue list came back exactly at the source's per-repository
// limit was truncated before the survey ever saw it. Rendering "50" as a count
// tells the reader the repository holds fifty; it holds at least fifty, and the
// difference is what makes an estate look nearly clear when it is not.
func TestARepositoryAtTheCollectionLimitIsNamed(t *testing.T) {
	res := Result{Signals: mkSignals("repo-a", 50)}

	at := res.Render(Options{Detail: Brief, PerRepo: 10, CollectionLimit: 50})
	if !strings.Contains(at, "limit of 50 per repository") || !strings.Contains(at, "repo-a") {
		t.Errorf("a repository sitting on the collection limit is not named:\n%s", at)
	}
	if !strings.Contains(at, "absent from every count above") {
		t.Errorf("the render does not distinguish a collection cap from a render cap:\n%s", at)
	}

	under := Result{Signals: mkSignals("repo-a", 49)}.Render(Options{Detail: Brief, PerRepo: 10, CollectionLimit: 50})
	if strings.Contains(under, "limit of 50 per repository") {
		t.Errorf("a repository under the limit was reported as truncated:\n%s", under)
	}
}

// TestSummaryCountsAreOfWhatWasCollected guards the one number a reader must be
// able to trust while every list below it is bounded.
func TestSummaryCountsAreOfWhatWasCollected(t *testing.T) {
	sigs := append(mkSignals("repo-a", 30), mkSignals("repo-b", 5)...)
	out := Result{Signals: sigs}.Render(Options{Detail: Brief, PerRepo: 2})

	for _, want := range []string{"Collected 35 signals", "35 work_item", "repo-a 30", "repo-b 5"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}
}

// TestBriefSurvivesATitleWithANewline: a source may hand over anything, and a
// title carrying a line break would silently split one signal into two lines.
func TestBriefSurvivesATitleWithANewline(t *testing.T) {
	out := Result{Signals: []signal.Signal{{
		Kind: signal.KindWorkItem, Source: "s", Repo: "r", ID: "#1",
		Title: "first line\nsecond line",
	}}}.Render(Default())
	if n := strings.Count(out, "\n- "); n != 1 {
		t.Errorf("a multi-line title produced %d list lines:\n%s", n, out)
	}
	if !strings.Contains(out, "first line second line") {
		t.Errorf("flattening the title lost part of it:\n%s", out)
	}
}

// The shift hands the session `mellions survey -save`, which is the brief
// render — so an attribute a brief line drops does not exist as far as an
// unattended session is concerned. A change set's readiness has to survive it.
func TestBriefChangeSetSaysWhetherItIsReadyForAReader(t *testing.T) {
	now := time.Date(2026, 8, 28, 20, 0, 0, 0, time.UTC)
	evidenced := signal.Signal{
		Kind: signal.KindChangeSet, Source: "github", Repo: "analytics-service",
		ID: "PR #916", Title: "fix(freshness): serve computed_at from the compute instant",
		Created: now.Add(-3 * time.Hour), Updated: now.Add(-3 * time.Hour),
		Attrs: map[string]string{"draft": "true", "author": "sample-user", "body": "5.9kB", "reviews": "0", "type": "pull_request"},
	}
	stub := evidenced
	stub.ID = "PR #737"
	stub.Attrs = map[string]string{"draft": "true", "author": "sample-user", "body": "0B", "reviews": "0", "type": "pull_request"}

	got := renderBrief(evidenced, now)
	for _, want := range []string{"draft", "sample-user", "body 5.9kB", "0 reviews"} {
		if !strings.Contains(got, want) {
			t.Errorf("brief change-set line lost %q — the shift render is what an unattended session reads.\ngot: %s", want, got)
		}
	}
	if a, b := renderBrief(evidenced, now), renderBrief(stub, now); a == b {
		t.Error("an evidenced draft and an empty one render identically in brief; the session cannot tell them apart")
	}

	// A change set whose merge-gate verdicts were POSTED AS COMMENTS reports
	// zero submitted GitHub reviews, and that is the shape that misdirects a
	// shift: advisor-service#846 carried two full independent reviews, a
	// merge-result gate and eleven comments, and the brief line said
	// "0 reviews" — so a session chose it as the unreviewed work nobody had
	// read. The two counts must both reach the line; neither substitutes for
	// the other.
	reviewedByComment := evidenced
	reviewedByComment.ID = "PR #846"
	reviewedByComment.Attrs = map[string]string{
		"draft": "false", "author": "sample-user", "body": "10.0kB",
		"reviews": "0", "comments": "11", "type": "pull_request",
	}
	got846 := renderBrief(reviewedByComment, now)
	if !strings.Contains(got846, "11 comments") {
		t.Errorf("brief change-set line drops the comment count, so a twice-reviewed change set reads as unread.\ngot: %s", got846)
	}
	if !strings.Contains(got846, "0 reviews") {
		t.Errorf("the review count must survive beside the comment count; a comment is not a review.\ngot: %s", got846)
	}
	silent := reviewedByComment
	silent.Attrs = map[string]string{
		"draft": "false", "author": "sample-user", "body": "10.0kB",
		"reviews": "0", "type": "pull_request",
	}
	if renderBrief(silent, now) == got846 {
		t.Error("a change set with eleven comments and one with none render identically; the count reaches nothing")
	}
	if strings.Contains(renderBrief(silent, now), "comments") {
		t.Errorf("a change set nobody has written on prints a comment noun anyway.\ngot: %s", renderBrief(silent, now))
	}

	// Every other kind keeps the line it had: this is a change-set rule, not a
	// licence for attributes generally to leak into the brief render.
	other := signal.Signal{
		Kind: signal.KindWorkItem, Source: "github", Repo: "r", ID: "#1", Title: "t",
		Created: now.Add(-time.Hour), Updated: now.Add(-time.Hour),
		Attrs: map[string]string{"draft": "true", "author": "sample-user", "body": "9kB", "reviews": "3"},
	}
	if s := renderBrief(other, now); strings.Contains(s, "sample-user") || strings.Contains(s, "body") {
		t.Errorf("a non-change-set brief line grew attributes: %s", s)
	}
}

// Whether the claim marker reaches a reader depends on which render it reaches
// them through, and the two renders differ: the full one prints every attribute
// generically, the brief one prints none of them. A shift chooses from the
// brief render — so asserting only the full one would pass while the reader
// #186 is about still sees nothing.
func TestAClaimedChangeSetSaysSoInBothRenders(t *testing.T) {
	held := signal.Signal{
		Kind: signal.KindChangeSet, Source: "github", ID: "PR #178", Title: "held draft", Repo: "r",
		Attrs: map[string]string{"type": "pull_request", "draft": "true", "reviews": "0",
			"claimed": "a Mellions lane holds this — read the claim on the pull request before merging it"},
	}
	free := signal.Signal{
		Kind: signal.KindChangeSet, Source: "github", ID: "PR #179", Title: "free draft", Repo: "r",
		Attrs: map[string]string{"type": "pull_request", "draft": "true", "reviews": "0"},
	}
	r := Result{At: time.Now(), Signals: []signal.Signal{held, free}}

	brief := lineFor(t, r.Render(Default()), "PR #178")
	if !strings.Contains(brief, "CLAIMED") {
		t.Errorf("the brief line a shift chooses from is %q and does not say the change set is held", brief)
	}
	if other := lineFor(t, r.Render(Default()), "PR #179"); strings.Contains(other, "CLAIMED") {
		t.Errorf("an unheld change set reads as held: %q", other)
	}
	full := lineFor(t, r.Render(Everything()), "PR #178")
	if !strings.Contains(full, "claimed=") {
		t.Errorf("the full render drops the claim attribute: %q", full)
	}
}

// lineFor returns the rendered block for one signal id, so a test asserts on
// what a reader sees rather than on the whole document. A block, not a line:
// the full render puts a signal's attributes on a continuation line, and a
// line-only match would report them missing when they are simply below.
func lineFor(t *testing.T, out, id string) string {
	t.Helper()
	for _, block := range strings.Split(out, "\n- ") {
		if strings.Contains(block, id) {
			return block
		}
	}
	t.Fatalf("%s does not appear in the render at all", id)
	return ""
}

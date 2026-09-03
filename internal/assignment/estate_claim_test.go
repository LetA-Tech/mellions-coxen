// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
)

// fakeTracker is the estate's tracker, in memory. Claims are keyed the way the
// real one keys them — host and lane id — so a test can put another machine's
// claim on an issue without pretending to be that machine.
type fakeTracker struct {
	mu     sync.Mutex
	host   string
	now    func() time.Time
	claims map[string][]claim.Claim
	// fail, when set, is returned by every call: the tracker is unreachable.
	fail error
	// failPublish fails only writes: the estate is readable, and this lane's
	// hold still cannot be put where anyone else can see it.
	failPublish error
	// swept records what Sweep removed, so a test can assert a stale claim was
	// cleared rather than merely ignored.
	swept []claim.Claim
	// prs is what the tracker says the branch has, keyed by branch, and comments
	// is what was posted, keyed by ref — the two halves a handoff travels on.
	prs      map[string][]claim.PullRequest
	comments map[string][]string
}

func newFakeTracker() *fakeTracker {
	return &fakeTracker{host: "here", now: time.Now, claims: map[string][]claim.Claim{},
		prs: map[string][]claim.PullRequest{}, comments: map[string][]string{}}
}

func key(repo, issue string) string {
	return strings.ToLower(strings.TrimSpace(repo)) + "#" + strings.TrimPrefix(strings.TrimSpace(issue), "#")
}

func (f *fakeTracker) Claims(_ context.Context, repo, issue string) ([]claim.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	return append([]claim.Claim(nil), f.claims[key(repo, issue)]...), nil
}

func (f *fakeTracker) Publish(_ context.Context, repo, issue, id, state string) (claim.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return claim.Claim{}, f.fail
	}
	if f.failPublish != nil {
		return claim.Claim{}, f.failPublish
	}
	c := claim.Claim{ID: id, Host: f.host, State: state, At: f.now().UTC()}
	k := key(repo, issue)
	kept := f.claims[k][:0:0]
	for _, e := range f.claims[k] {
		if e.Host == c.Host && e.ID == c.ID {
			continue
		}
		kept = append(kept, e)
	}
	f.claims[k] = append(kept, c)
	return c, nil
}

func (f *fakeTracker) Release(_ context.Context, repo, issue, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	k := key(repo, issue)
	kept := f.claims[k][:0:0]
	for _, e := range f.claims[k] {
		if e.Host == f.host && e.ID == id {
			continue
		}
		kept = append(kept, e)
	}
	f.claims[k] = kept
	return nil
}

func (f *fakeTracker) PullRequests(_ context.Context, _, branch string) ([]claim.PullRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	return append([]claim.PullRequest(nil), f.prs[branch]...), nil
}

func (f *fakeTracker) Comment(_ context.Context, repo, ref, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	if f.comments == nil {
		f.comments = map[string][]string{}
	}
	k := key(repo, ref)
	f.comments[k] = append(f.comments[k], body)
	return nil
}

func (f *fakeTracker) Sweep(_ context.Context, repo, issue string) ([]claim.Claim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	k := key(repo, issue)
	now := f.now()
	kept := f.claims[k][:0:0]
	var swept []claim.Claim
	for _, e := range f.claims[k] {
		if e.Stale(now) {
			swept = append(swept, e)
			continue
		}
		kept = append(kept, e)
	}
	f.claims[k] = kept
	f.swept = append(f.swept, swept...)
	return swept, nil
}

// newStoreT is the store every test in this package opens: one with a tracker,
// because a store without one refuses to open a lane on an issue and that
// refusal is the behaviour under test, not the fixture.
func newStoreT(dir string) (*Store, error) {
	s, err := NewStore(dir)
	if err != nil {
		return nil, err
	}
	s.Tracker = newFakeTracker()
	return s, nil
}

func trackerOf(t *testing.T, s *Store) *fakeTracker {
	t.Helper()
	f, ok := s.Tracker.(*fakeTracker)
	if !ok {
		t.Fatalf("store has no fake tracker: %T", s.Tracker)
	}
	return f
}

// The local guard is this machine's disk. Two hosts surveying one tracker both
// see the same open issue, and the store on either one has nothing to compare
// against — which is how one issue acquired two pull requests four minutes
// apart, on two machines the guard could not see across.
func TestALaneIsRefusedWhenAnotherMachineHoldsTheIssue(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("payments-api", "#43")] = []claim.Claim{{
		ID: "payments-claimed", Host: "build-host", State: "active", At: time.Now().Add(-time.Hour),
	}}

	_, err = s.Open(OpenOptions{
		ID: "payments-new", Repo: "payments-api", Issue: "#43", Source: src,
		Objective: "the --version banner claims schema:1 it does not carry",
		Because:   "the survey showed it open and unheld on this machine",
	})
	if err == nil {
		t.Fatal("a lane opened on an issue another machine holds; both hosts now carry the same work")
	}
	for _, want := range []string{"payments-claimed", "build-host", claim.Label, "-alongside"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q, so the session cannot act on it: %v", want, err)
		}
	}
	if _, err := s.Get("payments-new"); err == nil {
		t.Error("the refused lane left a record behind")
	}
}

// A host that lost power mid-lane leaves a claim nothing can release. Honouring
// it forever would make the issue permanently unclaimable by anyone, and an
// engineer that needs a person to clear a lock it invented is worse than no
// lock at all.
func TestAStaleClaimIsSweptRatherThanObeyed(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	f.claims[key("svc", "#7")] = []claim.Claim{{
		ID: "old", Host: "dead-host", State: "active", At: time.Now().Add(-claim.StaleAfter - time.Hour),
	}}

	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "take work whose claim expired", Because: "the holder is gone",
	}); err != nil {
		t.Fatalf("a stale claim blocked the work it can no longer be doing: %v", err)
	}
	if len(f.swept) != 1 || f.swept[0].ID != "old" {
		t.Fatalf("the stale claim was ignored rather than swept: %+v", f.swept)
	}
}

// The failure this exists for is not a network error; it is a lane that
// believes it holds an issue nothing else can see. Degrading silently to a
// local-only claim reproduces exactly the state the claim was built to end.
func TestAClaimThatCannotBePublishedRefusesTheOpen(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trackerOf(t, s).fail = errors.New("gh: could not resolve host api.github.com")

	_, err = s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "work on an unreachable tracker", Because: "the survey said it was open",
	})
	if err == nil {
		t.Fatal("a lane opened with a claim no other machine can see, and said nothing about it")
	}
	if !strings.Contains(err.Error(), "-unpublished") {
		t.Errorf("the refusal does not name the one flag that accepts it: %v", err)
	}
	if _, err := s.Get("svc-7"); err == nil {
		t.Error("the refused lane left a record behind")
	}
}

// Accepting a local-only claim is legitimate — the tracker is down and the work
// is not — but the lane has to carry that everywhere it is printed, or it reads
// as an ordinary claim to the next session.
func TestAnUnpublishedLaneSaysSoOnItsRecord(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trackerOf(t, s).fail = errors.New("gh: could not resolve host api.github.com")

	a, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src, Unpublished: true,
		Objective: "work while the tracker is down", Because: "the work does not need GitHub",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if a.Claim == nil || a.Claim.Published() {
		t.Fatalf("an unpublishable claim was recorded as published: %+v", a.Claim)
	}
	if !strings.Contains(a.Text(time.Now()), "LOCAL ONLY") {
		t.Error("the lane prints as an ordinary claim, so the next session cannot tell it is invisible")
	}
}

func TestOpeningPublishesAndClosingReleases(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)

	a, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "publish a claim", Because: "the owner asked for it",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !a.Claim.Published() {
		t.Fatalf("opening did not publish the claim: %+v", a.Claim)
	}
	if got := f.claims[key("svc", "#7")]; len(got) != 1 || got[0].ID != "svc-7" {
		t.Fatalf("the tracker does not carry this lane's claim: %+v", got)
	}

	if err := s.Handoff("svc-7", "done and pushed"); err != nil {
		t.Fatal(err)
	}
	// Handed off is finished and waiting on a person: the state a survey on
	// another machine still shows as an open issue, so the claim stands.
	if got := f.claims[key("svc", "#7")]; len(got) != 1 || got[0].State != StateHandedOff {
		t.Fatalf("handing off did not restate the claim: %+v", got)
	}

	if err := s.Close("svc-7"); err != nil {
		t.Fatal(err)
	}
	if got := f.claims[key("svc", "#7")]; len(got) != 0 {
		t.Fatalf("closing left the issue looking taken: %+v", got)
	}
}

// A release that fails must not block a finished lane from closing: the residue
// it leaves goes stale and is swept, and refusing to close would trade a
// self-healing residue for a stuck one. It is recorded, not silent.
func TestAReleaseThatFailsIsRecordedAndDoesNotBlockClosing(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "publish a claim", Because: "the owner asked for it",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Handoff("svc-7", "done"); err != nil {
		t.Fatal(err)
	}
	f.fail = errors.New("gh: could not resolve host api.github.com")

	if err := s.Close("svc-7"); err != nil {
		t.Fatalf("a finished lane could not close because the tracker was unreachable: %v", err)
	}
	a, err := s.Get("svc-7")
	if err != nil {
		t.Fatal(err)
	}
	if a.Claim == nil || a.Claim.Stranded == "" {
		t.Fatalf("the claim left on the tracker is not recorded anywhere: %+v", a.Claim)
	}
}

// Abandoning releases the same way closing does. A lane that ends by throwing
// its work away has released the issue as completely as one that finished it.
func TestAbandoningReleasesTheClaim(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)
	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "abandon this", Because: "the owner asked for it",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Abandon("svc-7", "the hypothesis was wrong", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.claims[key("svc", "#7")]; len(got) != 0 {
		t.Fatalf("an abandoned lane still holds the issue: %+v", got)
	}
}

// -alongside is the reconciliation case and has to reach the estate guard too:
// a session sent to reconcile two lanes on two machines cannot do it if the
// tracker claim refuses it the way the local one would.
func TestAlongsideTakesWorkAnotherMachineHolds(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trackerOf(t, s).claims[key("svc", "#7")] = []claim.Claim{{
		ID: "theirs", Host: "build-host", State: "active", At: time.Now(),
	}}
	if _, err := s.Open(OpenOptions{
		ID: "reconcile-7", Repo: "svc", Issue: "#7", Source: src, Alongside: true,
		Objective: "reconcile the two lanes", Because: "two hosts carried the same issue",
	}); err != nil {
		t.Fatalf("-alongside was refused, so two lanes on two machines cannot be reconciled: %v", err)
	}
}

// The estate read succeeding and the publish failing is the case the estate
// guard cannot cover: nothing holds the issue, and this lane still cannot say
// that it does.
func TestAPublishThatFailsOnAReadableTrackerRefusesTheOpen(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	trackerOf(t, s).failPublish = errors.New("gh: HTTP 403 (issues are disabled)")

	_, err = s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "claim an issue that cannot be commented on", Because: "the survey said it was open",
	})
	if err == nil {
		t.Fatal("a lane opened holding a claim the tracker never accepted")
	}
	if !strings.Contains(err.Error(), "-unpublished") {
		t.Errorf("the refusal does not name the one flag that accepts it: %v", err)
	}
	if _, err := s.Get("svc-7"); err == nil {
		t.Error("the refused lane left a record behind")
	}
}

// #186: a draft with an independent review in flight said so only in a comment,
// and a peer on another host merged it. The hold has to be on the change set
// itself, in the channel both hosts already read.
func TestClaimingAPullRequestPutsTheHoldWhereAPeerReadsIt(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)

	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "hold a draft", Because: "a review of it is in flight",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ClaimPullRequest(context.Background(), "svc-7", "178"); err != nil {
		t.Fatalf("ClaimPullRequest: %v", err)
	}

	a, err := s.Get("svc-7")
	if err != nil {
		t.Fatal(err)
	}
	// The canonical spelling, because it is the one the tracker parses and the
	// one a survey prints as the change set's id.
	if a.PullRequest != "PR #178" {
		t.Fatalf("recorded pull request is %q, want %q", a.PullRequest, "PR #178")
	}
	held := f.claims[key("svc", "PR #178")]
	if len(held) != 1 || held[0].ID != "svc-7" {
		t.Fatalf("the tracker does not carry a hold on the change set: %+v", held)
	}
	if !strings.Contains(a.Text(time.Now()), "PR #178") {
		t.Error("the lane's own record does not say which change set it holds")
	}

	// Both refs are released together: a lane that let go of its issue and kept
	// its pull request leaves a draft looking held by a lane that is finished.
	if err := s.Handoff("svc-7", "done and pushed"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close("svc-7"); err != nil {
		t.Fatal(err)
	}
	if got := f.claims[key("svc", "PR #178")]; len(got) != 0 {
		t.Fatalf("closing left the change set looking held: %+v", got)
	}
	if got := f.claims[key("svc", "#7")]; len(got) != 0 {
		t.Fatalf("closing left the issue looking taken: %+v", got)
	}
}

// A claim the tracker refused is not a claim, and recording it would put the
// lane back in exactly the state this exists to end: believing it holds
// something nothing else can see.
func TestAPullRequestClaimTheTrackerRefusedIsNotRecorded(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Source: src,
		Objective: "hold a draft", Because: "a review of it is in flight",
	}); err != nil {
		t.Fatal(err)
	}
	trackerOf(t, s).failPublish = errors.New("gh: 403 label creation forbidden")

	if err := s.ClaimPullRequest(context.Background(), "svc-7", "178"); err == nil {
		t.Fatal("an unpublishable hold was accepted; a peer would still read the draft as unheld")
	}
	a, err := s.Get("svc-7")
	if err != nil {
		t.Fatal(err)
	}
	if a.PullRequest != "" {
		t.Fatalf("the lane records a hold nothing else can see: %q", a.PullRequest)
	}
}

// #132: the handoff is written to one machine's disk and the peer deciding
// whether the draft is ready is on the other one. It has to travel, and it has
// to travel even when the lane never recorded a pull request — the branch is
// enough to find it.
func TestHandingOffPutsTheHandoffOnThePullRequest(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := trackerOf(t, s)

	a, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Issue: "#7", Source: src,
		Objective: "hand off across hosts", Because: "the peer is elsewhere",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.prs[a.Branch] = []claim.PullRequest{{Number: 178, State: "OPEN"}}

	if err := s.Handoff("svc-7", "waits on the cold read dispatched at 04:45"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("svc-7")
	if err != nil {
		t.Fatal(err)
	}
	if got.PullRequest != "PR #178" {
		t.Fatalf("the branch's pull request was not found: %q", got.PullRequest)
	}
	posted := f.comments[key("svc", "PR #178")]
	if len(posted) != 1 {
		t.Fatalf("want the handoff posted once on the change set, got %d comments", len(posted))
	}
	// The opening line is what a peer reads first: whose lane, which host, what
	// state. Without it the comment is prose in a column of prose.
	for _, want := range []string{"Handoff", "svc-7", "here", StateHandedOff,
		"waits on the cold read dispatched at 04:45"} {
		if !strings.Contains(posted[0], want) {
			t.Errorf("the handoff comment does not say %q:\n%s", want, posted[0])
		}
	}
	// Finding the pull request also claims it: the handoff and the hold arrive
	// together or a peer reads a handed-off draft as free.
	if held := f.claims[key("svc", "PR #178")]; len(held) != 1 {
		t.Fatalf("handing off did not restate the hold on the change set: %+v", held)
	}
}

// A tracker that is down must not cost the session the handoff it just wrote.
// The record is the thing that must survive; the comment is best effort.
func TestAHandoffSurvivesATrackerThatCannotTakeIt(t *testing.T) {
	src := gitFixture(t)
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(OpenOptions{
		ID: "svc-7", Repo: "svc", Source: src,
		Objective: "hand off with the tracker down", Because: "the work does not need GitHub",
	}); err != nil {
		t.Fatal(err)
	}
	trackerOf(t, s).fail = errors.New("gh: could not resolve host api.github.com")

	if err := s.Handoff("svc-7", "stands at the failing test"); err != nil {
		t.Fatalf("Handoff refused because the tracker was down: %v", err)
	}
	a, err := s.Get("svc-7")
	if err != nil {
		t.Fatal(err)
	}
	if a.Handoff != "stands at the failing test" || a.State != StateHandedOff {
		t.Fatalf("the handoff was lost with the network: %+v", a)
	}
}

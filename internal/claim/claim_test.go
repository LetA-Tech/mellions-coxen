// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package claim

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// fake records what gh was asked for and answers issue views from a script.
type fake struct {
	calls [][]string
	view  string
	err   error
}

func (f *fake) run(ctx context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	if f.err != nil {
		return nil, f.err
	}
	if len(args) > 1 && (args[0] == "issue" || args[0] == "pr") && args[1] == "view" {
		return []byte(f.view), nil
	}
	return []byte(""), nil
}

func (f *fake) saw(verb ...string) bool {
	for _, c := range f.calls {
		if len(c) < len(verb) {
			continue
		}
		match := true
		for i, v := range verb {
			if c[i] != v {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (f *fake) sawArg(arg string) bool {
	for _, c := range f.calls {
		for _, a := range c {
			if a == arg {
				return true
			}
		}
	}
	return false
}

func body(id, host, state string, at time.Time) string {
	c := Claim{ID: id, Host: host, State: state, At: at}
	raw, _ := json.Marshal(c)
	return "Claimed by Mellions lane\n\n" + marker + string(raw) + " -->\n"
}

func view(comments ...ghComment) string {
	g := struct {
		Comments []ghComment `json:"comments"`
	}{comments}
	raw, _ := json.Marshal(g)
	return string(raw)
}

func tracker(f *fake, now time.Time) *Tracker {
	return &Tracker{Owner: "example-org", Host: "here", Run: f.run, now: func() time.Time { return now }}
}

func TestClaimsReadsPublishedClaims(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view(
		ghComment{ID: "c1", Body: "an ordinary comment about the bug"},
		ghComment{ID: "c2", Body: body("advisor-51", "build-host", "active", at)},
	)}
	got, err := tracker(f, at).Claims(context.Background(), "advisor-service", "#51")
	if err != nil {
		t.Fatalf("Claims: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 claim, got %d: %+v", len(got), got)
	}
	if got[0].ID != "advisor-51" || got[0].Host != "build-host" || got[0].Ref != "c2" {
		t.Fatalf("claim not read back: %+v", got[0])
	}
}

// A claim read out of a tracker that could not be reached is the failure this
// whole mechanism exists to prevent: empty means "nobody holds this", and
// reporting that from a failed read is how two machines take one issue.
func TestClaimsRefusesRatherThanReportingUnclaimed(t *testing.T) {
	f := &fake{err: errors.New("gh: could not resolve host")}
	got, err := tracker(f, time.Now()).Claims(context.Background(), "advisor-service", "#51")
	if err == nil {
		t.Fatalf("a tracker that cannot be read reported %d claims and no error", len(got))
	}
}

func TestStaleAfterElapses(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	c := Claim{At: at}
	if c.Stale(at.Add(StaleAfter - time.Minute)) {
		t.Fatal("a claim restated within the window is not stale")
	}
	if !c.Stale(at.Add(StaleAfter + time.Minute)) {
		t.Fatal("a claim unrestated past the window is stale")
	}
}

func TestPublishLabelsCommentsAndSupersedes(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view(ghComment{ID: "old", Body: body("task-a", "here", "active", at.Add(-time.Hour))})}
	c, err := tracker(f, at).Publish(context.Background(), "mellions-coxen", "#40", "task-a", "active")
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if c.Host != "here" || !c.At.Equal(at) {
		t.Fatalf("published claim not returned: %+v", c)
	}
	if !f.saw("label", "create", Label) {
		t.Error("the label was not created, so a repository that has never seen it cannot be labelled")
	}
	if !f.saw("issue", "comment", "40") {
		t.Error("no claim comment was posted")
	}
	if !f.sawArg("--add-label") {
		t.Error("the issue was not labelled, so no survey can filter on it")
	}
	if !f.sawArg("id=old") {
		t.Error("the superseded claim comment was left behind")
	}
}

// Publishing must not delete this lane's previous claim before the new one is
// up: a delete that runs first and a publish that then fails leaves the issue
// unclaimed while the lane believes it holds it.
func TestPublishPostsBeforeItDeletes(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view(ghComment{ID: "old", Body: body("task-a", "here", "active", at.Add(-time.Hour))})}
	if _, err := tracker(f, at).Publish(context.Background(), "mellions-coxen", "#40", "task-a", "active"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	post, del := -1, -1
	for i, c := range f.calls {
		if len(c) > 1 && c[0] == "issue" && c[1] == "comment" {
			post = i
		}
		if len(c) > 1 && c[0] == "api" && del == -1 {
			del = i
		}
	}
	if post == -1 || del == -1 || del < post {
		t.Fatalf("delete of the superseded claim ran at %d, the post at %d", del, post)
	}
}

func TestReleaseWithdrawsAndUnlabels(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view(ghComment{ID: "mine", Body: body("task-a", "here", "active", at)})}
	if err := tracker(f, at).Release(context.Background(), "mellions-coxen", "#40", "task-a"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if !f.sawArg("id=mine") {
		t.Error("the claim comment was not withdrawn")
	}
	if !f.sawArg("--remove-label") {
		t.Error("the label stayed on an issue nothing holds")
	}
}

// The label says "some lane holds this", not "this lane held it". Taking it off
// while another machine still holds the issue tells every survey in the estate
// the work is free.
func TestReleaseKeepsLabelWhileAnotherLaneHolds(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view(
		ghComment{ID: "mine", Body: body("task-a", "here", "active", at)},
		ghComment{ID: "theirs", Body: body("task-b", "build-host", "active", at)},
	)}
	if err := tracker(f, at).Release(context.Background(), "mellions-coxen", "#40", "task-a"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if !f.sawArg("id=mine") {
		t.Error("this lane's claim was not withdrawn")
	}
	if f.sawArg("--remove-label") {
		t.Error("the label was removed while another machine still holds the issue")
	}
}

func TestSweepRemovesStaleClaimsAndKeepsLive(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view(
		ghComment{ID: "dead", Body: body("task-a", "gone-host", "active", now.Add(-StaleAfter-time.Hour))},
		ghComment{ID: "live", Body: body("task-b", "build-host", "active", now.Add(-time.Minute))},
	)}
	swept, err := tracker(f, now).Sweep(context.Background(), "mellions-coxen", "#40")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 1 || swept[0].ID != "task-a" {
		t.Fatalf("want the stale claim swept, got %+v", swept)
	}
	if !f.sawArg("id=dead") {
		t.Error("the stale claim comment was left on the issue")
	}
	if f.sawArg("id=live") {
		t.Error("a live claim was swept")
	}
	if f.sawArg("--remove-label") {
		t.Error("the label was removed while a live claim remains")
	}
}

func TestNumberReadsTheSpellingsSessionsWrite(t *testing.T) {
	for _, in := range []string{"#43", "43", " #43 "} {
		n, err := Number(in)
		if err != nil || n != 43 {
			t.Errorf("Number(%q) = %d, %v", in, n, err)
		}
	}
	if _, err := Number("dev"); err == nil {
		t.Error("Number accepted a branch name as an issue")
	}
}

func TestHeldNamesTheMachine(t *testing.T) {
	c := Claim{ID: "task-a", Host: "build-host", State: "handed_off",
		At: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)}
	got := c.Held()
	for _, want := range []string{"task-a", "build-host", "handed_off"} {
		if !strings.Contains(got, want) {
			t.Errorf("Held() = %q, missing %q — a refusal that cannot name the holder cannot be acted on", got, want)
		}
	}
}

// TestPullRequestsReadsWhatGhSays: the sweep's question, asked in gh's own
// terms and read back whole — a merge time where there is one, none where
// there is not, and a failure as a failure.
func TestPullRequestsReadsWhatGhSays(t *testing.T) {
	var asked []string
	tr := &Tracker{Owner: "example-org", Host: "here", Run: func(_ context.Context, args ...string) ([]byte, error) {
		asked = args
		return []byte(`[{"mergedAt":"2026-08-28T19:54:26Z","number":118,"state":"MERGED"},{"mergedAt":null,"number":120,"state":"OPEN"}]`), nil
	}}
	prs, err := tr.PullRequests(context.Background(), "mellions-coxen", "mellions/x")
	if err != nil {
		t.Fatal(err)
	}
	want := "pr list --repo example-org/mellions-coxen --head mellions/x --state all --json number,state,mergedAt"
	if got := strings.Join(asked, " "); got != want {
		t.Errorf("asked gh %q, want %q", got, want)
	}
	if len(prs) != 2 || prs[0].Number != 118 || prs[0].State != "MERGED" || prs[0].MergedAt.IsZero() {
		t.Errorf("merged pull request read as %+v", prs)
	}
	if prs[1].Number != 120 || prs[1].State != "OPEN" || !prs[1].MergedAt.IsZero() {
		t.Errorf("open pull request read as %+v", prs)
	}

	failing := &Tracker{Owner: "example-org", Run: func(context.Context, ...string) ([]byte, error) {
		return nil, errors.New("gh pr list: HTTP 502")
	}}
	if _, err := failing.PullRequests(context.Background(), "r", "b"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Errorf("a failing gh returned %v, want its error", err)
	}
	if _, err := tr.PullRequests(context.Background(), "r", ""); err == nil {
		t.Error("an empty branch was asked about; gh would list every pull request")
	}
}

// A pull request is addressed through gh's pull-request verbs, not its issue
// verbs. The two number spaces overlap and gh is asymmetric about it: `gh pr
// view` refuses an issue number, but `gh issue view` answers for a pull
// request — so a claim routed through the wrong verb fails loudly in one
// direction and silently succeeds against the wrong thing in the other. These
// assert the verb, because the verb is the whole correctness of the routing.
func TestPublishOnAPullRequestUsesThePullRequestVerbs(t *testing.T) {
	at := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	f := &fake{view: view()}
	if _, err := tracker(f, at).Publish(context.Background(), "mellions-coxen", "PR #51", "task-a", "handed_off"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	for _, want := range [][]string{{"pr", "view", "51"}, {"pr", "comment", "51"}, {"pr", "edit", "51"}} {
		if !f.saw(want...) {
			t.Errorf("want gh %v; a pull request claim must not go through the issue verbs", want)
		}
	}
	if f.saw("issue", "comment") || f.saw("issue", "edit") || f.saw("issue", "view") {
		t.Error("a pull request was addressed through gh issue")
	}
	if !f.saw("label", "create", Label) {
		t.Error("the label was not created")
	}
}

func TestClaimsReleaseAndSweepRoutePullRequestsToThePullRequestVerbs(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	stale := body("task-a", "gone-host", "active", now.Add(-StaleAfter-time.Hour))

	f := &fake{view: view(ghComment{ID: "mine", Body: body("task-a", "here", "active", now)})}
	if err := tracker(f, now).Release(context.Background(), "mellions-coxen", "PR #51", "task-a"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if !f.saw("pr", "edit", "51") || !f.sawArg("--remove-label") {
		t.Error("releasing a pull request claim did not unlabel it through gh pr edit")
	}
	if f.saw("issue", "edit") {
		t.Error("a pull request was unlabelled through gh issue edit")
	}

	// A claim on a pull request expires on exactly the same clock: a host that
	// died mid-review must not hold a draft shut forever.
	g := &fake{view: view(ghComment{ID: "dead", Body: stale})}
	swept, err := tracker(g, now).Sweep(context.Background(), "mellions-coxen", "PR #51")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 1 || swept[0].ID != "task-a" {
		t.Fatalf("want the stale pull request claim swept, got %+v", swept)
	}
	if !g.saw("pr", "view", "51") || !g.saw("pr", "edit", "51") {
		t.Error("sweeping a pull request claim did not go through the pull request verbs")
	}

	// And a live one is still honoured.
	h := &fake{view: view(ghComment{ID: "live", Body: body("task-b", "build-host", "active", now.Add(-time.Minute))})}
	claims, err := tracker(h, now).Claims(context.Background(), "mellions-coxen", "PR #51")
	if err != nil {
		t.Fatalf("Claims: %v", err)
	}
	if len(claims) != 1 || claims[0].Stale(now) {
		t.Fatalf("want one live claim read off the pull request, got %+v", claims)
	}
}

func TestParseRefTellsAPullRequestFromAnIssue(t *testing.T) {
	for _, tc := range []struct {
		in   string
		verb string
		n    int
	}{
		{"#12", "issue", 12},
		{"12", "issue", 12},
		{" #12 ", "issue", 12},
		{"PR #12", "pr", 12},
		{"pr#12", "pr", 12},
		{"pr 12", "pr", 12},
		{"PR12", "pr", 12},
	} {
		verb, n, err := parseRef(tc.in)
		if err != nil || verb != tc.verb || n != tc.n {
			t.Errorf("parseRef(%q) = %q, %d, %v; want %q, %d", tc.in, verb, n, err, tc.verb, tc.n)
		}
	}
	for _, bad := range []string{"dev", "", "#0", "pr", "prime-42"} {
		if verb, n, err := parseRef(bad); err == nil {
			t.Errorf("parseRef(%q) = %q, %d; want a refusal", bad, verb, n)
		}
	}
}

// A handed-off lane is finished work whose worktree is kept because a reviewer
// may still need it, and unattended that reviewer is the only one there is. The
// claim comment is the whole of what such a reader sees before deciding, so a
// blanket refusal there is the claim withholding work that is waiting to be
// taken. The live-lane refusal has to survive that fix, which is what the third
// case is for: without it, deleting the sentence outright would pass.
func TestAHandedOffClaimAsksForTheReaderItsWorktreeIsKeptFor(t *testing.T) {
	at := time.Date(2026, 9, 4, 5, 0, 0, 0, time.UTC)

	published := func(state string) string {
		f := &fake{view: view()}
		if _, err := tracker(f, at).Publish(context.Background(), "mellions-coxen", "PR #10", "lane-a", state); err != nil {
			t.Fatalf("Publish(%s): %v", state, err)
		}
		for _, c := range f.calls {
			for i, a := range c {
				if a == "--body" && i+1 < len(c) {
					return c[i+1]
				}
			}
		}
		t.Fatalf("Publish(%s) published no body", state)
		return ""
	}

	handed := published("handed_off")
	if strings.Contains(handed, "the lane is live") {
		t.Error("a handed-off claim tells the reader the lane is live, which its own state line contradicts")
	}
	if strings.Contains(handed, "is not yours while the claim stands") {
		t.Error("a handed-off claim refuses the reviewer its worktree is kept for; unattended there is no other reader")
	}
	if !strings.Contains(handed, "handed_off") {
		t.Error("a handed-off claim does not say which state it is in")
	}

	live := published("active")
	if !strings.Contains(live, "is not yours while the claim stands") {
		t.Error("an active lane's claim stopped refusing, so nothing holds work in flight against a second session")
	}
}

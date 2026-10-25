// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package claim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// thread is a gh that keeps a pull request's conversation: a comment adds one,
// the GraphQL delete removes one by id, and a view returns what is left.
type thread struct {
	comments []ghComment
	next     int
	// failView, failComment and failDelete fail that one call kind only;
	// failDeleteID fails the delete of that one comment.
	failView, failComment, failDelete error
	failDeleteID                      string
}

func (th *thread) run(_ context.Context, args ...string) ([]byte, error) {
	switch {
	case len(args) > 1 && args[1] == "view":
		if th.failView != nil {
			return nil, th.failView
		}
		raw, _ := json.Marshal(struct {
			Comments []ghComment `json:"comments"`
		}{th.comments})
		return raw, nil
	case len(args) > 1 && args[1] == "comment":
		if th.failComment != nil {
			return nil, th.failComment
		}
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--body" {
				th.next++
				th.comments = append(th.comments, ghComment{ID: fmt.Sprintf("IC_%d", th.next), Body: args[i+1]})
				return nil, nil
			}
		}
		return nil, errors.New("comment without --body")
	case len(args) > 0 && args[0] == "api":
		if th.failDelete != nil {
			return nil, th.failDelete
		}
		for _, a := range args {
			if id, ok := strings.CutPrefix(a, "id="); ok {
				if id == th.failDeleteID {
					return nil, errors.New("gh: HTTP 502")
				}
				kept := th.comments[:0]
				for _, c := range th.comments {
					if c.ID != id {
						kept = append(kept, c)
					}
				}
				th.comments = kept
			}
		}
		return nil, nil
	}
	return nil, nil
}

func (th *thread) bodies() []string {
	var out []string
	for _, c := range th.comments {
		out = append(out, c.Body)
	}
	return out
}

// countHolding is how many comments on the thread contain s.
func (th *thread) countHolding(s string) int {
	n := 0
	for _, c := range th.comments {
		if strings.Contains(c.Body, s) {
			n++
		}
	}
	return n
}

func onThread(th *thread, host string) *Tracker {
	return &Tracker{Owner: "example-org", Host: host, Run: th.run, now: time.Now}
}

// legacyHandoff is a handoff as posted before it carried a marker: the header
// line alone names the lane and its host.
func legacyHandoff(id, host, text string) string {
	return fmt.Sprintf("**Handoff — Mellions lane `%s` on `%s`, now handed_off.**\n\n"+
		"This is the lane's own statement of where the work stands. Read it before merging.\n\n"+
		"## Handoff\n\n%s\n", id, host, text)
}

// A lane that hands off, takes a review round and hands off again leaves one
// handoff on its pull request, and it is the second. Every other comment on the
// thread — another lane's handoff, the same lane id from another machine, a
// reviewer's prose — is left exactly where it was.
func TestASecondHandoffReplacesTheLanesFirst(t *testing.T) {
	th := &thread{}
	th.comments = []ghComment{
		{ID: "R1", Body: "Independent read: NOT READY at 0ac472b."},
	}
	other := onThread(th, "here")
	if err := other.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-b", "lane-b stands at its arms"); err != nil {
		t.Fatalf("lane-b Handoff: %v", err)
	}
	elsewhere := onThread(th, "the-mac")
	if err := elsewhere.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "lane-a on the mac"); err != nil {
		t.Fatalf("lane-a on the mac Handoff: %v", err)
	}

	tr := onThread(th, "here")
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "first: head 0ac472b, NOT READY"); err != nil {
		t.Fatalf("first Handoff: %v", err)
	}
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "second: head b9440dd, PASS"); err != nil {
		t.Fatalf("second Handoff: %v", err)
	}

	if n := th.countHolding("first: head 0ac472b"); n != 0 {
		t.Errorf("the superseded handoff is still on the pull request (%d copies):\n%s", n, strings.Join(th.bodies(), "\n---\n"))
	}
	if n := th.countHolding("second: head b9440dd, PASS"); n != 1 {
		t.Errorf("want the second handoff on the pull request once, got %d:\n%s", n, strings.Join(th.bodies(), "\n---\n"))
	}
	for _, kept := range []string{"lane-b stands at its arms", "lane-a on the mac", "Independent read: NOT READY"} {
		if n := th.countHolding(kept); n != 1 {
			t.Errorf("a comment that is not this lane's handoff was touched: %q now appears %d times", kept, n)
		}
	}
	if len(th.comments) != 4 {
		t.Errorf("want 4 comments (reviewer, lane-b, lane-a on the mac, lane-a's second), got %d:\n%s",
			len(th.comments), strings.Join(th.bodies(), "\n---\n"))
	}
}

// Lanes handed off before handoffs carried a marker have their earlier handoff
// on the pull request already. The next handoff of the same lane replaces it;
// another lane's stays.
func TestAHandoffReplacesTheLanesHandoffPostedWithoutAMarker(t *testing.T) {
	th := &thread{comments: []ghComment{
		{ID: "L1", Body: legacyHandoff("lane-a", "here", "old: head 4d3c7ff7")},
		{ID: "L2", Body: legacyHandoff("lane-a2", "here", "a lane whose id extends this one's")},
		{ID: "L3", Body: legacyHandoff("lane-a", "the-mac", "same id, other machine")},
		{ID: "Q1", Body: "> " + legacyHandoff("lane-a", "here", "quoted by a reviewer")},
	}}
	if err := onThread(th, "here").Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "new: head ae4a74b6"); err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	if th.countHolding("old: head 4d3c7ff7") != 0 {
		t.Errorf("the lane's handoff posted without a marker survived:\n%s", strings.Join(th.bodies(), "\n---\n"))
	}
	for _, kept := range []string{"a lane whose id extends this one's", "same id, other machine", "quoted by a reviewer", "new: head ae4a74b6"} {
		if n := th.countHolding(kept); n != 1 {
			t.Errorf("want %q on the pull request once, got %d", kept, n)
		}
	}
}

// A handoff's marker is the line Handoff writes last, so text inside a handoff
// that shows the marker's form cannot hide the real one, and a comment that
// carries a marker anywhere but its own last line is not a handoff.
func TestOnlyAHandoffsOwnLastLineMarksIt(t *testing.T) {
	th := &thread{}
	tr := onThread(th, "here")
	quoting := "the marker reads `<!-- mellions:handoff {\"id\":…,\"host\":…} -->` and names the lane\n" +
		"<!-- mellions:handoff {\"id\":\"lane-b\",\"host\":\"here\"} -->\nis lane-b's form"
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "first\n\n"+quoting); err != nil {
		t.Fatal(err)
	}
	th.comments = append(th.comments,
		ghComment{ID: "Q1", Body: "Pasting the handoff:\n\n> first\n> <!-- mellions:handoff {\"id\":\"lane-a\",\"host\":\"here\"} -->\n"},
		ghComment{ID: "Q2", Body: "<!-- mellions:handoff {\"id\":\"lane-a\",\"host\":\"here\"} -->\nand a reviewer's words after it"},
	)
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "second"); err != nil {
		t.Fatal(err)
	}
	if n := th.countHolding("first\n\nthe marker reads"); n != 0 {
		t.Errorf("a handoff whose text shows the marker form was not replaced:\n%s", strings.Join(th.bodies(), "\n---\n"))
	}
	for _, kept := range []string{"Pasting the handoff", "and a reviewer's words after it", "second"} {
		if n := th.countHolding(kept); n != 1 {
			t.Errorf("want %q on the pull request once, got %d:\n%s", kept, n, strings.Join(th.bodies(), "\n---\n"))
		}
	}
}

// The new handoff goes up before the old one comes down: a post that fails
// leaves the lane's last statement standing rather than none.
func TestAHandoffThatCannotPostKeepsTheLanesPrevious(t *testing.T) {
	th := &thread{}
	tr := onThread(th, "here")
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "first"); err != nil {
		t.Fatal(err)
	}
	th.failComment = errors.New("gh: HTTP 502")
	err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "second")
	if err == nil {
		t.Fatal("a handoff that never posted reported success")
	}
	if errors.Is(err, ErrHandoffNotReplaced) {
		t.Errorf("a handoff that never posted is reported as posted with its predecessor left: %v", err)
	}
	if th.countHolding("first") != 1 {
		t.Errorf("the lane's previous handoff was removed although its replacement never posted")
	}
}

// A handoff that posted but could not clear its predecessor says so: the stale
// one is the defect, and silence would leave a merger reading it.
func TestAHandoffThatCannotClearItsPredecessorSaysSo(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*thread)
	}{
		{"the thread cannot be read", func(th *thread) { th.failView = errors.New("gh: could not resolve host") }},
		{"the old comment cannot be deleted", func(th *thread) { th.failDelete = errors.New("gh: HTTP 403") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := &thread{}
			tr := onThread(th, "here")
			if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "first"); err != nil {
				t.Fatal(err)
			}
			tc.set(th)
			err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "second")
			if !errors.Is(err, ErrHandoffNotReplaced) {
				t.Fatalf("want ErrHandoffNotReplaced, got %v", err)
			}
			if th.countHolding("second") != 1 {
				t.Errorf("the new handoff did not post")
			}
		})
	}
}

// A claim opens with the line naming its lane and ends with its marker. A
// comment that carries a claim pasted among other words — even the current one,
// so the same instant — is not a claim, and no restate, release or sweep deletes it.
func TestAPastedClaimMarkerIsNotAClaim(t *testing.T) {
	at := time.Date(2026, 10, 25, 2, 0, 0, 0, time.UTC)
	pasted := body("lane-a", "here", "active", at.Add(-time.Hour))
	for _, tc := range []struct {
		name string
		act  func(*Tracker) error
	}{
		{"restating", func(tr *Tracker) error {
			_, err := tr.Publish(context.Background(), "mellions-coxen", "#40", "lane-a", "active")
			return err
		}},
		{"releasing", func(tr *Tracker) error {
			return tr.Release(context.Background(), "mellions-coxen", "#40", "lane-a")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := &thread{comments: []ghComment{
				{ID: "REAL", Body: pasted},
				{ID: "REVIEWER", Body: "The claim's source, pasted:\n\n" + pasted + "\nI disagree with this lane holding the issue."},
				{ID: "ENDPASTE", Body: "My objection to this hold, then the claim itself:\n\n" + pasted},
				{ID: "OPENS", Body: "Claimed by Mellions lane `lane-a` is the wrong holder; its claim:\n\n" + pasted + "\nwhich I dispute."},
				{ID: "QUOTED", Body: "> " + strings.ReplaceAll(strings.TrimRight(pasted, "\n"), "\n", "\n> ") + "\n"},
			}}
			tr := &Tracker{Owner: "example-org", Host: "here", Run: th.run, now: func() time.Time { return at }}
			if err := tc.act(tr); err != nil {
				t.Fatal(err)
			}
			for _, kept := range []string{"I disagree with this lane", "> Claimed by", "My objection to this hold", "which I dispute"} {
				if th.countHolding(kept) != 1 {
					t.Errorf("a comment that is not the lane's claim was deleted (%q):\n%s", kept, strings.Join(th.bodies(), "\n---\n"))
				}
			}
			for _, c := range th.comments {
				if c.ID == "REAL" {
					t.Errorf("the lane's own claim comment was left behind")
				}
			}
		})
	}
}

// A stale claim is swept, a hand-written one naming its lane first included,
// and a comment that merely carries one pasted is not a claim, however old.
func TestASweepLeavesAPastedClaim(t *testing.T) {
	at := time.Date(2026, 10, 25, 2, 0, 0, 0, time.UTC)
	old := body("lane-a", "elsewhere", "active", at.Add(-StaleAfter-time.Hour))
	handwritten := "Claimed by Mellions lane `lane-b` for issue40. The lane's branch holds the work.\n\n" +
		marker + `{"id":"lane-b","host":"elsewhere","state":"active","at":"` + at.Add(-StaleAfter-time.Hour).Format(time.RFC3339) + `"} -->` + "\n"
	th := &thread{comments: []ghComment{
		{ID: "STALE", Body: old},
		{ID: "ENDPASTE", Body: "Why was this held so long?\n\n" + old},
		{ID: "HANDWRITTEN", Body: handwritten},
	}}
	tr := &Tracker{Owner: "example-org", Host: "here", Run: th.run, now: func() time.Time { return at }}
	swept, err := tr.Sweep(context.Background(), "mellions-coxen", "#40")
	if err != nil {
		t.Fatal(err)
	}
	if len(swept) != 2 {
		t.Errorf("want the stale claim and the stale hand-written claim swept, got %+v", swept)
	}
	if len(th.comments) != 1 || th.comments[0].ID != "ENDPASTE" {
		t.Errorf("want only the comment quoting the claim left, got:\n%s", strings.Join(th.bodies(), "\n---\n"))
	}
}

// A handoff opens and ends with the lane's marker. A comment that pastes the
// handoff word for word above or below somebody's own words is not that handoff.
func TestAHandoffPastedWithOtherWordsIsNotTheHandoff(t *testing.T) {
	th := &thread{}
	tr := onThread(th, "here")
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "first: NOT READY"); err != nil {
		t.Fatal(err)
	}
	posted := th.comments[0].Body
	quoted := "> " + strings.ReplaceAll(strings.TrimRight(posted, "\n"), "\n", "\n> ") + "\n"
	th.comments = append(th.comments,
		ghComment{ID: "QUOTED", Body: quoted},
		ghComment{ID: "ABOVE", Body: "Reviewer: this handoff overstates the arms.\n\n" + posted},
		ghComment{ID: "BELOW", Body: posted + "\nReviewer: the arms above are fine."},
	)
	if err := tr.Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "second: READY"); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"Reviewer: this handoff overstates", "Reviewer: the arms above are fine", "> first: NOT READY", "second: READY"} {
		if n := th.countHolding(kept); n != 1 {
			t.Errorf("want %q on the pull request once, got %d:\n%s", kept, n, strings.Join(th.bodies(), "\n---\n"))
		}
	}
	if len(th.comments) != 4 {
		t.Errorf("want the lane's first handoff replaced and both reviewer comments kept, got %d comments:\n%s",
			len(th.comments), strings.Join(th.bodies(), "\n---\n"))
	}
}

// One delete that fails does not stop the rest: every earlier handoff that can
// come down does, and the one that could not is reported.
func TestAFailedDeleteDoesNotStopTheRest(t *testing.T) {
	th := &thread{comments: []ghComment{
		{ID: "L1", Body: legacyHandoff("lane-a", "here", "oldest")},
		{ID: "L2", Body: legacyHandoff("lane-a", "here", "older")},
	}, failDeleteID: "L1"}
	err := onThread(th, "here").Handoff(context.Background(), "mellions-coxen", "PR #51", "lane-a", "newest")
	if !errors.Is(err, ErrHandoffNotReplaced) {
		t.Fatalf("want ErrHandoffNotReplaced, got %v", err)
	}
	if th.countHolding("older") != 0 {
		t.Errorf("a failed delete stopped the next one:\n%s", strings.Join(th.bodies(), "\n---\n"))
	}
}

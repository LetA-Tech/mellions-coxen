// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
)

// PullRequests asks the tracker for every pull request whose head is branch.
//
// An error is unknown, never absent. A branch with no pull request and a
// tracker that could not be asked are the same silence and opposite answers
// to whether the lane is finished.
type PullRequests func(ctx context.Context, repo, branch string) ([]claim.PullRequest, error)

// SweepOptions says what a sweep reads and whether it acts.
type SweepOptions struct {
	// Repo narrows the sweep to one repository; empty is every one.
	Repo string
	// Apply closes what is closable. Without it the sweep only says what it
	// would close.
	Apply bool
	// PullRequests reads the tracker. Nil is an installation with no tracker,
	// and every handed-off lane is then kept: unknown is not closable.
	PullRequests PullRequests
	// Live describes the running session holding a lane, by the session id
	// the lane records. A lane with a live session is being worked whatever
	// its state says, and the sweep says so instead of asking for a handoff.
	Live map[string]string
}

// Swept is the sweep's reading of one lane.
type Swept struct {
	ID string
	// Verdict is closed, closable or kept for a handed-off lane, and the
	// lane's own state where the sweep never touches it.
	Verdict string
	Why     string
}

// Sweep closes the handed-off lanes whose pull request the tracker says is
// merged or closed, and says what it did, or would do, with every open lane.
//
// The session that merges a pull request is almost never the one that opened
// the lane, so lanes were handed off and nobody closed them: continuity, the
// session-start hook and the survey carried dozens of finished lanes as work
// in flight, and their worktrees stayed on disk. Finished work waiting to be
// noticed is not finished.
//
// What closes a lane is what the tracker establishes, nothing inferred: a
// tracker that could not be asked and a pull request still open both keep the
// lane, each saying why. A branch the tracker has no pull request for in any
// state is the case a pull request can never settle — most of this engineer's
// work never opens one — so there it is what the lane still holds that decides:
// commits waiting to be published keep it, and a lane holding nothing does not
// stay open on a pull request that will never exist. An active lane is
// never closed here — the handoff is the session's act — and a blocked or
// suspended one is resting on purpose. Closing is the ordinary close: the
// worktree goes, the branch and the record stay, and the record says the
// sweep did it and on what evidence.
func (s *Store) Sweep(ctx context.Context, o SweepOptions) ([]Swept, error) {
	open, err := s.List(false)
	if err != nil {
		return nil, err
	}
	var out []Swept
	for _, a := range open {
		if o.Repo != "" && !strings.EqualFold(strings.TrimSpace(o.Repo), strings.TrimSpace(a.Repo)) {
			continue
		}
		out = append(out, s.sweepOne(ctx, a, o))
	}
	return out, nil
}

func (s *Store) sweepOne(ctx context.Context, a *Assignment, o SweepOptions) Swept {
	v := Swept{ID: a.ID, Verdict: a.State}
	switch a.State {
	case StateActive:
		if who, ok := liveOn(a, o.Live); ok {
			v.Why = "held by " + who + "; the sweep never closes active work"
		} else {
			v.Why = "`mellions assign handoff " + a.ID + "` first; the sweep never closes active work"
		}
		return v
	case StateBlocked, StateSuspended:
		v.Why = "set down on purpose; reopen it or hand it off"
		return v
	case StateHandedOff:
	default:
		v.Why = "not a state the sweep reads"
		return v
	}
	v.Verdict = "kept"
	if strings.TrimSpace(a.Handoff) == "" {
		v.Why = "no handoff on record"
		return v
	}
	if o.PullRequests == nil {
		v.Why = "no tracker is configured, so whether " + a.Branch + " has a pull request is unknown"
		return v
	}
	prs, err := o.PullRequests(ctx, a.Repo, a.Branch)
	if err != nil {
		v.Why = "the tracker could not say whether " + a.Branch + " has a pull request: " + err.Error()
		return v
	}
	pr, ok := decisive(prs)
	var why string
	switch {
	case !ok:
		// A lane whose branch never had a pull request in any state could not
		// become closable by the rule above, ever: there is nothing that will
		// merge. Most of this engineer's work is that shape — an independent
		// read, an investigation, a decision package — and its whole product
		// is the record the handoff already holds. Kept on the tracker's
		// silence alone, 70 of 99 open lanes were finished and unclosable, and
		// continuity, the session-start hook and the survey carried every one
		// of them as work in flight.
		//
		// What is still at stake decides it rather than the silence. Commits
		// the branch carries are work waiting to be published, and a pull
		// request is what publishes it; Unsaved below keeps a lane holding
		// files or unpublished commits.
		if n, known := s.laneCommits(a); known && n > 0 {
			v.Why = fmt.Sprintf("no pull request for %s, which carries %d commit%s — open one, or `mellions assign abandon %s -discarding \"...\"`",
				a.Branch, n, plural(n), a.ID)
			return v
		}
		why = "no pull request was ever opened for " + a.Branch + " and the lane carries nothing to publish"
	case pr.State == "OPEN":
		v.Why = fmt.Sprintf("pull request #%d is open", pr.Number)
		return v
	case pr.State != "MERGED" && pr.State != "CLOSED":
		v.Why = fmt.Sprintf("pull request #%d is %s, which the sweep does not read as finished", pr.Number, pr.State)
		return v
	default:
		why = finished(pr)
	}
	u, err := s.Unsaved(a)
	if err != nil {
		v.Why = why + ", but the worktree could not be read: " + err.Error()
		return v
	}
	if u.Any() {
		v.Why = fmt.Sprintf("%s, but the worktree %s holds %s — commit it, or `mellions assign abandon %s -discarding \"...\"`",
			why, a.Worktree, u, a.ID)
		return v
	}
	if !o.Apply {
		v.Verdict, v.Why = "closable", why
		return v
	}
	if err := s.close(a.ID, "closed by `mellions assign sweep`: "+why); err != nil {
		v.Why = why + ", but closing failed: " + err.Error()
		return v
	}
	v.Verdict, v.Why = "closed", why
	return v
}

// laneCommits counts the commits the lane's branch carries over its recorded
// base, and says whether it could be counted at all.
//
// Unknown rather than zero wherever the question cannot be put: no worktree on
// the record, no base to bound the range with, a tree already removed, or a
// base that no longer resolves. Zero would read as "this lane produced
// nothing", which is the answer that closes it, and a tree that is gone is
// exactly a lane that holds nothing on disk — its branch and its record
// survive the close either way, so the count is not what protects it.
//
// This is the lane's own commits, not Unsaved's: Unsaved counts what no
// remote-tracking ref holds, so a pushed branch reads as zero there and this
// still sees the work.
func (s *Store) laneCommits(a *Assignment) (int, bool) {
	if a.Worktree == "" || a.Base == "" {
		return 0, false
	}
	if _, err := os.Stat(a.Worktree); err != nil {
		return 0, false
	}
	out, err := s.Git(a.Worktree, "rev-list", "--count", a.Base+"..HEAD")
	if err != nil {
		return 0, false
	}
	n := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// decisive is the pull request that settles the lane. An open one wins
// whatever happened to earlier attempts; then a merged one; then the latest
// closed one.
func decisive(prs []claim.PullRequest) (claim.PullRequest, bool) {
	rank := func(state string) int {
		switch state {
		case "OPEN":
			return 3
		case "MERGED":
			return 2
		case "CLOSED":
			return 1
		}
		return 0
	}
	var pick claim.PullRequest
	found := false
	for _, p := range prs {
		if !found || rank(p.State) > rank(pick.State) ||
			(rank(p.State) == rank(pick.State) && p.Number > pick.Number) {
			pick, found = p, true
		}
	}
	return pick, found
}

// finished says what the tracker established, in the words the record keeps.
func finished(pr claim.PullRequest) string {
	if pr.State == "MERGED" {
		if pr.MergedAt.IsZero() {
			return fmt.Sprintf("pull request #%d merged", pr.Number)
		}
		return fmt.Sprintf("pull request #%d merged %s", pr.Number, pr.MergedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	return fmt.Sprintf("pull request #%d closed without merging", pr.Number)
}

// liveOn is the running session holding this lane, if one is. Every session
// the lane records is checked, not only the newest: a dead session that
// touched it last does not end a live one that touched it earlier.
func liveOn(a *Assignment, live map[string]string) (string, bool) {
	for _, s := range a.Sessions {
		if d, ok := live[s.ID]; ok {
			return d, true
		}
	}
	return "", false
}

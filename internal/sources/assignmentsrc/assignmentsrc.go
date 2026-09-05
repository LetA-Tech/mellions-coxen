// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package assignmentsrc reports the engineer's own unfinished work.
//
// It is the first thing a returning engineer should see and the easiest to
// forget, because unfinished work is invisible from outside: a branch nobody
// opened a pull request for and a question nobody answered look exactly like
// nothing happening. A survey that lists every open issue and omits the two
// assignments already in flight will reliably suggest starting a third.
package assignmentsrc

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Source reports open assignments and the follow-ups they recorded.
type Source struct {
	store *assignment.Store
	// Remote reports whether a branch still exists on the source checkout's
	// remote. known is false when the remote could not be asked, which is a
	// different answer from absent. Replaced in tests.
	Remote func(ctx context.Context, source, branch string) (present, known bool)
	// Commits reports how many commits a branch carries beyond the base it was
	// cut from. known is false when the checkout could not be asked, which is a
	// different answer from none. Replaced in tests.
	Commits func(ctx context.Context, source, base, branch string) (n int, known bool)
}

// New returns a source over an assignment store.
func New(s *assignment.Store) *Source {
	return &Source{store: s, Remote: remoteBranch, Commits: branchCommits}
}

// remoteBranch asks the remote itself rather than the local refs: a checkout
// with a restricted fetch refspec reports branches absent that the remote still
// has, and this installation has already been bitten by exactly that.
func remoteBranch(ctx context.Context, source, branch string) (present, known bool) {
	if source == "" || branch == "" {
		return false, false
	}
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, "git", "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	cmd.Dir = source
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(out)) != "", true
}

// branchCommits counts what a lane actually produced, against the commit its
// record says it was cut from — Base, the commit; BasePin is the prose that
// says how that commit was chosen. A lane that reviews rather than writes never
// commits, so its branch stands where it was cut and its absence from the
// remote is nothing to act on; a lane holding commits is the case a missing
// remote branch is worth reporting for.
func branchCommits(ctx context.Context, source, base, branch string) (int, bool) {
	if source == "" || base == "" || branch == "" {
		return 0, false
	}
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, "git", "rev-list", "--count", base+".."+branch)
	cmd.Dir = source
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false
	}
	return n, true
}

// Name implements signal.Source.
func (s *Source) Name() string { return "assignments" }

// Collect reports work in flight, and separately anything found while doing it.
//
// It also says when an open assignment's own premise has expired: its branch
// is gone from the remote, so the work was merged, deleted, or never pushed.
// The session that merges a pull request is almost never the session that
// opened the assignment, and closing is a manual act; on one installation
// fifteen finished lanes sat open for days and every session start listed
// them as work in flight. Both halves were already collected here and in the
// tracker source, and nothing compared them. Reported, never acted on: whether
// the lane is finished is for the reader to establish.
func (s *Source) Collect(ctx context.Context, scope signal.Scope) ([]signal.Signal, error) {
	list, err := s.store.List(false)
	if err != nil {
		return nil, fmt.Errorf("assignmentsrc: %w", err)
	}
	now := time.Now()
	var out []signal.Signal
	for _, a := range list {
		if len(scope.Repos) > 0 && a.Repo != "" && !slices.Contains(scope.Repos, a.Repo) {
			continue
		}
		attrs := map[string]string{
			"state":    a.State,
			"branch":   a.Branch,
			"worktree": a.Worktree,
		}
		// An elapsed budget is a fact about the work, not a judgement about it:
		// the engineer decides whether to finish, hand off or stop.
		if a.Overdue(now) {
			attrs["budget"] = "elapsed — a written status is owed"
		}
		if a.Handoff != "" {
			attrs["handoff"] = "written"
		}
		detail := a.Objective
		// Suspended work looks identical to stalled work without this. One was
		// a decision and the other is a failure, and only the record separates
		// them at the moment somebody is deciding what to pick up.
		if n := len(a.Suspensions); n > 0 {
			last := a.Suspensions[n-1]
			if last.Open() {
				attrs["set-down-for"] = last.For
				detail += "\nset down for: " + last.For + "\nstood at: " + last.Stands
			}
			if n > 1 {
				attrs["interruptions"] = fmt.Sprintf("%d", n)
			}
		}
		if a.Because != "" {
			detail += "\nchosen because: " + a.Because
		}
		if last, ok := lastFinding(a); ok {
			detail += fmt.Sprintf("\nlast note (%s): %s", last.Kind, last.Text)
		}
		gone := false
		if s.Remote != nil && a.Source != "" && a.Branch != "" {
			switch present, known := s.Remote(ctx, a.Source, a.Branch); {
			case !known:
				attrs["remote_branch"] = "unknown"
			case present:
				attrs["remote_branch"] = "present"
			default:
				attrs["remote_branch"] = "gone"
				gone = true
			}
			// A lane that never committed has nothing that could have been
			// pushed and nothing that could have merged, so its branch missing
			// from the remote is not the record falling behind the world — it
			// is a review lane looking exactly as it should. Half of what this
			// signal reported was that. Not knowing keeps the signal: a
			// checkout that cannot be asked is not evidence that no work exists.
			if gone && s.Commits != nil {
				if n, known := s.Commits(ctx, a.Source, a.Base, a.Branch); known && n == 0 {
					gone = false
					attrs["remote_branch"] = "gone (nothing was ever committed on it)"
				}
			}
		}

		out = append(out, signal.Signal{
			Kind: signal.KindAssignment, Source: "assignments",
			ID: a.ID, Title: firstLine(a.Objective), Repo: a.Repo,
			Created: a.OpenedAt, Updated: a.UpdatedAt,
			Attrs: attrs, Detail: detail,
		})
		if gone {
			out = append(out, signal.Signal{
				Kind: signal.KindStalePremise, Source: "assignments",
				ID: a.ID, Repo: a.Repo,
				Title: "assignment " + a.ID + " is open on the record, but its branch " + a.Branch +
					" is not on the remote",
				Created: a.OpenedAt, Updated: a.UpdatedAt,
				Attrs: map[string]string{"state": a.State, "branch": a.Branch},
				Detail: "The branch was merged and deleted, deleted without merging, or never pushed. " +
					"The record is behind the world either way: `mellions continue` says what the " +
					"tracker holds for it, and closing or abandoning it is the reader's call.",
			})
		}

		// Follow-ups are carried separately because they are candidate work,
		// not the work in flight, and merging them would hide both.
		for _, f := range a.Findings {
			if f.Kind != "found" {
				continue
			}
			out = append(out, signal.Signal{
				Kind: signal.KindFollowUp, Source: "assignments",
				ID:    a.ID + ":" + f.At.UTC().Format("0102-1504"),
				Title: firstLine(f.Text), Repo: a.Repo,
				Created: f.At, Updated: f.At,
				Attrs:  map[string]string{"found_during": a.ID},
				Detail: f.Text,
			})
		}
	}
	return out, nil
}

func lastFinding(a *assignment.Assignment) (assignment.Finding, bool) {
	if len(a.Findings) == 0 {
		return assignment.Finding{}, false
	}
	return a.Findings[len(a.Findings)-1], true
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

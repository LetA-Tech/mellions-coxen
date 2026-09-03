// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
)

// ghSays is a faked gh: it answers `pr list --head <branch>` from a script
// and records what it was asked.
type ghSays struct {
	byBranch map[string]string
	err      error
	asked    [][]string
}

func (g *ghSays) run(_ context.Context, args ...string) ([]byte, error) {
	g.asked = append(g.asked, args)
	if g.err != nil {
		return nil, g.err
	}
	for i, a := range args {
		if a == "--head" && i+1 < len(args) {
			if out, ok := g.byBranch[args[i+1]]; ok {
				return []byte(out), nil
			}
		}
	}
	return []byte("[]"), nil
}

// fakePullRequests uses the same runner boundary as the GitHub claim tracker.
func fakePullRequests(g *ghSays) PullRequests {
	tr := &claim.Tracker{Owner: "example-org", Host: "here", Run: g.run}
	return tr.PullRequests
}

func handedOff(t *testing.T, s *Store, id, repo, source, handoff string) *Assignment {
	t.Helper()
	a := mustOpen(t, s, OpenOptions{ID: id, Repo: repo, Source: source,
		Objective: "finish " + id, Because: "the owner asked for it"})
	if err := s.Handoff(id, handoff); err != nil {
		t.Fatal(err)
	}
	return a
}

func verdicts(sw []Swept) map[string]Swept {
	out := map[string]Swept{}
	for _, v := range sw {
		out[v.ID] = v
	}
	return out
}

// TestSweepClosesOnlyWhatTheTrackerEstablishes is the sweep's whole contract
// on one store: a merged pull request closes the lane, an open one and a
// missing one keep it, and an active lane is never touched. The dry run and
// the apply read the same evidence; only the apply acts on it.
func TestSweepClosesOnlyWhatTheTrackerEstablishes(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	merged := handedOff(t, s, "fx-merged", "rates-service", repo, "Done and merged as PR #7.")
	open := handedOff(t, s, "fx-open", "rates-service", repo, "Draft PR #8 is up for review.")
	none := handedOff(t, s, "fx-none", "rates-service", repo, "Branch pushed; no PR yet.")
	active := mustOpen(t, s, OpenOptions{ID: "fx-active", Repo: "rates-service", Source: repo,
		Objective: "still working", Because: "the owner asked for it"})

	gh := &ghSays{byBranch: map[string]string{
		merged.Branch: `[{"number":7,"state":"MERGED","mergedAt":"2026-08-28T19:54:26Z"}]`,
		open.Branch:   `[{"number":8,"state":"OPEN","mergedAt":null}]`,
		none.Branch:   `[]`,
		// The tracker would say merged; the lane is active, so it must not matter.
		active.Branch: `[{"number":9,"state":"MERGED","mergedAt":"2026-08-28T19:54:26Z"}]`,
	}}

	dry, err := s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(gh)})
	if err != nil {
		t.Fatal(err)
	}
	got := verdicts(dry)
	if len(got) != 4 {
		t.Fatalf("dry run printed %d lines for 4 open lanes: %+v", len(got), dry)
	}
	if v := got["fx-merged"]; v.Verdict != "closable" || !strings.Contains(v.Why, "pull request #7 merged 2026-08-28 19:54 UTC") {
		t.Errorf("merged PR: %+v, want closable naming #7 and when it merged", v)
	}
	if v := got["fx-open"]; v.Verdict != "kept" || !strings.Contains(v.Why, "pull request #8 is open") {
		t.Errorf("open PR: %+v, want kept naming the open #8", v)
	}
	if v := got["fx-none"]; v.Verdict != "kept" || !strings.Contains(v.Why, "no pull request for "+none.Branch) {
		t.Errorf("no PR: %+v, want kept saying there is none", v)
	}
	if v := got["fx-active"]; v.Verdict != StateActive || !strings.Contains(v.Why, "`mellions assign handoff fx-active` first") {
		t.Errorf("active lane: %+v, want its state and the handoff nudge", v)
	}
	// A dry run acts on nothing.
	for _, id := range []string{"fx-merged", "fx-open", "fx-none"} {
		if a, _ := s.Get(id); a.State != StateHandedOff {
			t.Errorf("dry run changed %s to %s", id, a.State)
		}
	}
	if a, _ := s.Get("fx-active"); a.State != StateActive {
		t.Errorf("dry run changed the active lane to %s", a.State)
	}
	// The faked gh was asked the question the sweep says it asks, per lane.
	askedFor := func(branch string) bool {
		for _, call := range gh.asked {
			if len(call) > 1 && call[0] == "pr" && call[1] == "list" && strings.Contains(strings.Join(call, " "),
				"--repo example-org/rates-service --head "+branch+" --state all") {
				return true
			}
		}
		return false
	}
	for _, a := range []*Assignment{merged, open, none} {
		if !askedFor(a.Branch) {
			t.Errorf("gh was never asked `pr list --repo example-org/rates-service --head %s --state all`; asked: %v", a.Branch, gh.asked)
		}
	}
	if askedFor(active.Branch) {
		t.Errorf("the tracker was asked about an active lane, whose answer cannot matter")
	}

	applied, err := s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(gh), Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	got = verdicts(applied)
	if v := got["fx-merged"]; v.Verdict != "closed" || !strings.Contains(v.Why, "pull request #7 merged") {
		t.Errorf("apply on the merged lane: %+v, want closed", v)
	}
	done, err := s.Get("fx-merged")
	if err != nil {
		t.Fatal(err)
	}
	if done.State != StateClosed {
		t.Errorf("record says %s after the sweep closed it", done.State)
	}
	if _, err := os.Stat(merged.Worktree); !os.IsNotExist(err) {
		t.Errorf("worktree %s survived the close: %v", merged.Worktree, err)
	}
	if _, err := s.Git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+merged.Branch); err != nil {
		t.Errorf("closing deleted branch %s; a close keeps the branch", merged.Branch)
	}
	// The record says who closed it and on what evidence, or a reader cannot
	// tell a swept lane from one a session finished by hand.
	said := false
	for _, f := range done.Findings {
		if f.Kind == "note" && strings.Contains(f.Text, "closed by `mellions assign sweep`") &&
			strings.Contains(f.Text, "pull request #7 merged 2026-08-28 19:54 UTC") {
			said = true
		}
	}
	if !said {
		t.Errorf("the closed record does not say the sweep closed it and why; findings: %+v", done.Findings)
	}
	for _, id := range []string{"fx-open", "fx-none"} {
		if a, _ := s.Get(id); a.State != StateHandedOff {
			t.Errorf("apply closed %s (%s) on evidence that does not close a lane", id, a.State)
		}
	}
	if a, _ := s.Get("fx-active"); a.State != StateActive {
		t.Errorf("apply changed the active lane to %s; the sweep never closes active work", a.State)
	}
}

// TestSweepKeepsWhatItCannotEstablish: a tracker that fails and a store with
// no tracker are both unknown, and unknown is not closable.
func TestSweepKeepsWhatItCannotEstablish(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	handedOff(t, s, "fl-1", "payments-api", repo, "Done and merged.")

	failing := &ghSays{err: errors.New("gh pr list: HTTP 502 from api.github.com")}
	sw, err := s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(failing), Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	v := verdicts(sw)["fl-1"]
	if v.Verdict != "kept" || !strings.Contains(v.Why, "could not say") || !strings.Contains(v.Why, "HTTP 502") {
		t.Errorf("tracker error: %+v, want kept, saying the tracker could not answer and why", v)
	}
	if a, _ := s.Get("fl-1"); a.State != StateHandedOff {
		t.Errorf("a tracker failure closed the lane (%s)", a.State)
	}

	sw, err = s.Sweep(context.Background(), SweepOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	v = verdicts(sw)["fl-1"]
	if v.Verdict != "kept" || !strings.Contains(v.Why, "no tracker is configured") {
		t.Errorf("no tracker: %+v, want kept saying so", v)
	}
	if a, _ := s.Get("fl-1"); a.State != StateHandedOff {
		t.Errorf("a store with no tracker closed the lane (%s)", a.State)
	}
}

// TestSweepSaysWhenALiveSessionHoldsActiveWork: the handoff nudge is for a
// lane nobody is in. Telling a running session to hand off the lane it is
// working is noise, and the sweep can tell the two apart.
func TestSweepSaysWhenALiveSessionHoldsActiveWork(t *testing.T) {
	const sess = "0d4c1a2e-live-session"
	t.Setenv("CLAUDE_CODE_SESSION_ID", sess)
	repo := realRepo(t)
	s := newStore(t)
	mustOpen(t, s, OpenOptions{ID: "me-live", Repo: "mellions-coxen", Source: repo,
		Objective: "in progress", Because: "the owner asked for it"})

	live := map[string]string{sess: "a live claude session 0d4c1a2e (pid 4242) in /tmp/tree"}
	sw, err := s.Sweep(context.Background(), SweepOptions{Live: live, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	v := verdicts(sw)["me-live"]
	if v.Verdict != StateActive || !strings.Contains(v.Why, "held by a live claude session 0d4c1a2e") {
		t.Errorf("live holder: %+v, want the lane's state and who holds it", v)
	}
	if strings.Contains(v.Why, "assign handoff") {
		t.Errorf("a lane a live session holds was told to hand off: %s", v.Why)
	}
	if a, _ := s.Get("me-live"); a.State != StateActive {
		t.Errorf("apply changed a held active lane to %s", a.State)
	}
}

// TestSweepKeepsALaneWhoseWorktreeHoldsWork: a merged pull request does not
// license destroying what only the worktree has. The ordinary close refuses
// this; the sweep says so in the dry run rather than discovering it on apply.
func TestSweepKeepsALaneWhoseWorktreeHoldsWork(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := handedOff(t, s, "fs-1", "advisor-service", repo, "Done and merged as PR #3.")
	if err := os.WriteFile(filepath.Join(a.Worktree, "repro.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gh := &ghSays{byBranch: map[string]string{a.Branch: `[{"number":3,"state":"MERGED","mergedAt":"2026-08-28T10:00:00Z"}]`}}
	for _, apply := range []bool{false, true} {
		sw, err := s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(gh), Apply: apply})
		if err != nil {
			t.Fatal(err)
		}
		v := verdicts(sw)["fs-1"]
		if v.Verdict != "kept" || !strings.Contains(v.Why, "1 untracked file") || !strings.Contains(v.Why, "assign abandon fs-1") {
			t.Errorf("apply=%v: %+v, want kept, naming the untracked file and the way out", apply, v)
		}
	}
	if got, _ := s.Get("fs-1"); got.State != StateHandedOff {
		t.Errorf("the lane was closed over unsaved work (%s)", got.State)
	}
	if _, err := os.Stat(filepath.Join(a.Worktree, "repro.sh")); err != nil {
		t.Errorf("the unsaved file is gone: %v", err)
	}
}

// TestSweepClosesOnAClosedPullRequestToo: closed without merging is finished
// as far as the lane is concerned — the work was rejected, and the branch
// stays for whoever wants to read it.
func TestSweepClosesOnAClosedPullRequestToo(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	a := handedOff(t, s, "fa-1", "frontend-app", repo, "PR #12 up.")
	gh := &ghSays{byBranch: map[string]string{
		// An earlier attempt was closed and a later one merged: the merged
		// one decides. A still-open one would win over both.
		a.Branch: `[{"number":12,"state":"CLOSED","mergedAt":null},{"number":15,"state":"MERGED","mergedAt":"2026-08-28T11:00:00Z"}]`,
	}}
	sw, err := s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(gh)})
	if err != nil {
		t.Fatal(err)
	}
	if v := verdicts(sw)["fa-1"]; v.Verdict != "closable" || !strings.Contains(v.Why, "#15 merged") {
		t.Errorf("closed then merged: %+v, want closable on #15", v)
	}
	gh.byBranch[a.Branch] = `[{"number":12,"state":"CLOSED","mergedAt":null}]`
	sw, _ = s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(gh)})
	if v := verdicts(sw)["fa-1"]; v.Verdict != "closable" || !strings.Contains(v.Why, "#12 closed without merging") {
		t.Errorf("closed only: %+v, want closable saying it closed without merging", v)
	}
	gh.byBranch[a.Branch] = `[{"number":12,"state":"CLOSED","mergedAt":null},{"number":16,"state":"OPEN","mergedAt":null}]`
	sw, _ = s.Sweep(context.Background(), SweepOptions{PullRequests: fakePullRequests(gh)})
	if v := verdicts(sw)["fa-1"]; v.Verdict != "kept" || !strings.Contains(v.Why, "#16 is open") {
		t.Errorf("closed then reopened as another: %+v, want kept on the open #16", v)
	}
}

// TestSweepNarrowsToARepository: -repo is a filter on what is read, and a
// blocked lane is reported as resting rather than swept.
func TestSweepNarrowsToARepository(t *testing.T) {
	repo := realRepo(t)
	s := newStore(t)
	handedOff(t, s, "a-1", "frontend-app", repo, "done")
	handedOff(t, s, "b-1", "advisor-service", repo, "done")
	if err := s.SetState("b-1", StateBlocked); err != nil {
		t.Fatal(err)
	}
	sw, err := s.Sweep(context.Background(), SweepOptions{Repo: "ADVISOR-SERVICE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sw) != 1 || sw[0].ID != "b-1" {
		t.Fatalf("-repo advisor-service read %+v, want b-1 alone", sw)
	}
	if sw[0].Verdict != StateBlocked || !strings.Contains(sw[0].Why, "set down on purpose") {
		t.Errorf("blocked lane: %+v", sw[0])
	}
}

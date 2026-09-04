// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package claim publishes a lane's hold on an issue or a pull request where
// every machine can see it.
//
// The assignment store is one machine's disk. Two sessions on two hosts that
// both survey the same tracker both see the same open issue, and neither can
// see that the other has already taken it — which is how one issue acquired two
// pull requests four minutes apart. The tracker is the only thing both sessions
// already read, so the claim goes there: a label a survey can filter on, and a
// comment carrying who holds it, so a session that finds the label can find the
// lane behind it.
//
// A pull request needs the same channel for the same reason: a draft is one
// bit — draft or not — and that bit means "unfinished", "finished but
// unreviewed" and "finished, reviewed, blocked on something in flight" alike.
// A peer on another host choosing work cannot tell them apart, and merges the
// third. The claim is what tells them apart without opening the pull request.
//
// Which of the two an item is travels in the reference, because the two number
// spaces overlap and gh's verbs are not interchangeable: `gh pr view` refuses
// an issue number outright, while `gh issue view` answers for a pull request
// and would let a mislabelled reference succeed against the wrong kind of
// thing. So: "PR #12" (also "pr#12", "pr 12") is a pull request, and "#12" or
// "12" is an issue. "PR #12" is the spelling githubsrc already prints as a
// change set's id, so a session can pass a survey line straight back in.
//
// A claim is a statement that expires. Nothing here treats a published claim as
// proof that work is still in flight — a host can lose power mid-lane, and the
// claim it left behind would otherwise block that item forever. A claim not
// restated within StaleAfter is swept by whoever next reads it.
package claim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Label marks an issue or a pull request a lane holds. It is what a survey on
// any machine can filter on without reading a single comment body.
const Label = "mellions:claimed"

// StaleAfter is how long a claim stands without being restated.
//
// A lane being worked writes to its record — a finding, a handoff, a state
// change — and every one of those restates the claim. A lane that has said
// nothing for a day is not evidence of work in flight; it is evidence of a
// session that ended without releasing, which is the case this has to survive.
const StaleAfter = 24 * time.Hour

// marker opens the machine-readable payload inside a claim comment. An HTML
// comment renders as nothing, so the comment reads as prose to a person and
// parses exactly to a machine.
const marker = "<!-- mellions:claim "

var markerPattern = regexp.MustCompile(`(?s)<!-- mellions:claim (\{.*?\}) -->`)

// Claim is one lane's hold on one issue or pull request, as published.
type Claim struct {
	// ID is the assignment id on the holding machine.
	ID string `json:"id"`
	// Host is the machine the lane lives on. It is what makes a claim
	// actionable from somewhere else: the lane's record is not reachable, but
	// the machine holding it is nameable.
	Host string `json:"host"`
	// State is the lane's state when the claim was last restated, and it
	// decides what the published comment asks of whoever finds it.
	//
	// Handed-off lanes still hold, for the reason they always did: the work is
	// done and a session that saw the item still open would otherwise redo it.
	// What a handed-off claim does not do is refuse a reader. That state is
	// finished work whose worktree is kept because a reviewer may still need
	// it, and the review — and the merge, where the reader establishes it
	// ready — is what it is waiting for.
	State string `json:"state"`
	// At is when the claim was last restated, and the only input to staleness.
	At time.Time `json:"at"`
	// Ref is the tracker's id for the comment carrying this claim, so it can be
	// released or swept without searching. Never published inside the payload —
	// it is read off the comment itself.
	Ref string `json:"-"`
}

// Stale reports whether the claim has gone unrestated long enough that it is no
// longer evidence of work in flight.
func (c Claim) Stale(now time.Time) bool { return now.Sub(c.At) > StaleAfter }

// Held renders the claim the way a refusal has to name it.
func (c Claim) Held() string {
	return fmt.Sprintf("%s on %s (%s), last restated %s",
		c.ID, c.Host, c.State, c.At.Format(time.RFC3339))
}

// Runner executes the gh CLI and returns stdout. Replaced in tests.
//
// Shelling out to gh matches githubsrc and for the same reason: gh holds the
// authentication and tracks GitHub's API changes, and the boundary worth having
// is this package, not the transport underneath it.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

func execRunner(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), firstLine(msg))
	}
	return out, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Tracker publishes and reads claims against one GitHub owner.
type Tracker struct {
	// Owner is the organisation or user the repositories live under.
	Owner string
	// Host names this machine in everything it publishes.
	Host string
	// Run executes gh; nil uses the real CLI.
	Run Runner
	now func() time.Time
}

// NewTracker returns a tracker for owner. The host is this machine's name,
// because a claim that does not say where the lane lives cannot be chased.
func NewTracker(owner string) *Tracker {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	return &Tracker{Owner: owner, Host: host, Run: execRunner, now: time.Now}
}

func (t *Tracker) clock() time.Time {
	if t.now == nil {
		return time.Now()
	}
	return t.now()
}

func (t *Tracker) run(ctx context.Context, args ...string) ([]byte, error) {
	if t.Run == nil {
		t.Run = execRunner
	}
	return t.Run(ctx, args...)
}

// slug is owner/repo as gh wants it.
func (t *Tracker) slug(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("claim: no repository named")
	}
	if strings.Contains(repo, "/") {
		return repo, nil
	}
	if strings.TrimSpace(t.Owner) == "" {
		return "", fmt.Errorf("claim: %q is not owner/repo and no owner is configured", repo)
	}
	return t.Owner + "/" + repo, nil
}

// Number reads the item number out of the spellings a session actually writes:
// "#405", "405", " #405 ".
func Number(ref string) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(ref), "#"))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("claim: %q is not an issue or pull request number", ref)
	}
	return n, nil
}

// prefix matches the pull-request half of a reference: "PR #12", "pr#12",
// "pr 12". Anchored, so an issue whose number is written plainly can never be
// read as a pull request.
var prefix = regexp.MustCompile(`(?i)^pr\s*#?\s*`)

// parseRef splits a reference into the gh verb that addresses it and its
// number. Every method routes through this, so the kind is decided once.
func parseRef(ref string) (verb string, n int, err error) {
	s := strings.TrimSpace(ref)
	verb = "issue"
	if loc := prefix.FindStringIndex(s); loc != nil {
		verb, s = "pr", s[loc[1]:]
	}
	if n, err = Number(s); err != nil {
		return "", 0, err
	}
	return verb, n, nil
}

// Addressable reports whether the tracker can address a reference at all.
// A work unit a repository keeps in its own register — "IMP-016", "REM-024" —
// is a real reference to real work and is not a tracker item, so asking gh
// about it is a category error rather than a typo.
func Addressable(ref string) bool {
	_, _, err := parseRef(ref)
	return err == nil
}

// PullRequestRef is the canonical spelling of a pull request reference: the one
// this package parses and the one githubsrc prints as a change set's id. A bare
// number is a pull request here — the caller has already said which kind it
// holds — so "12", "#12" and "PR #12" all normalise to "PR #12".
func PullRequestRef(ref string) (string, error) {
	_, n, err := parseRef(ref)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PR #%d", n), nil
}

// name renders a reference the way this package's errors have to name it: an
// issue as owner/repo#12, a pull request as owner/repo PR #12, because
// owner/repo#12 alone does not say which of the two was addressed.
func name(slug, verb string, n int) string {
	if verb == "pr" {
		return fmt.Sprintf("%s PR #%d", slug, n)
	}
	return fmt.Sprintf("%s#%d", slug, n)
}

type ghComment struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type ghIssue struct {
	Comments []ghComment `json:"comments"`
	Labels   []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// Claims returns every claim published on an issue, newest restatement first
// per lane.
//
// A tracker that cannot be read returns an error rather than an empty list.
// Empty means "nobody holds this", and reporting that from a failed read is how
// a session concludes an issue is free because the network was down.
func (t *Tracker) Claims(ctx context.Context, repo, ref string) ([]Claim, error) {
	slug, err := t.slug(repo)
	if err != nil {
		return nil, err
	}
	verb, n, err := parseRef(ref)
	if err != nil {
		return nil, err
	}
	out, err := t.run(ctx, verb, "view", strconv.Itoa(n), "--repo", slug, "--json", "comments,labels")
	if err != nil {
		return nil, fmt.Errorf("claim: read claims on %s: %w", name(slug, verb, n), err)
	}
	var got ghIssue
	if err := json.Unmarshal(out, &got); err != nil {
		return nil, fmt.Errorf("claim: read claims on %s: %w", name(slug, verb, n), err)
	}
	// Later comments supersede earlier ones for the same lane: restating a
	// claim posts a new comment and deletes the old, but a delete that failed
	// must not resurrect a claim the lane has moved past.
	byLane := map[string]Claim{}
	for _, c := range got.Comments {
		m := markerPattern.FindStringSubmatch(c.Body)
		if m == nil {
			continue
		}
		var parsed Claim
		if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
			continue // a payload we cannot read is not a claim we can honour
		}
		parsed.Ref = c.ID
		key := parsed.Host + "\x00" + parsed.ID
		if prev, ok := byLane[key]; ok && prev.At.After(parsed.At) {
			continue
		}
		byLane[key] = parsed
	}
	claims := make([]Claim, 0, len(byLane))
	for _, c := range byLane {
		claims = append(claims, c)
	}
	return claims, nil
}

// Publish states that this machine's lane id holds the issue or pull request.
//
// It removes whatever this lane published before, so a restated claim is one
// comment rather than a growing column of them, and the label is created if the
// repository has never seen it.
// HandedOffState is assignment.StateHandedOff. It is duplicated rather than
// imported because internal/assignment imports this package, and the two are
// held equal by a guard in the external test package, which can import both.
const HandedOffState = "handed_off"

// audience is what the claim asks of whoever finds it, which is not the same
// question for every state. A handed-off lane is finished work whose worktree
// is kept precisely because a reviewer may still need it
// (assignment.StateHandedOff), so refusing that reader refuses the only thing
// the state is waiting for — and unattended there is no other reader. Every
// other state is a lane still holding the work, where the refusal is the point.
func audience(state string) string {
	if state == HandedOffState {
		return "the lane has finished and written its handoff, and its worktree is kept for exactly this — " +
			"reading it, and merging what it establishes is ready, is available to you rather than taken. " +
			"The record is on that machine, so read the handoff before deciding; " +
			"what is still the owner's it says so."
	}
	return "the lane is live and its record is on that machine, " +
		"so taking this work — or merging this change set — is not yours while the claim stands."
}

func (t *Tracker) Publish(ctx context.Context, repo, ref, id, state string) (Claim, error) {
	slug, err := t.slug(repo)
	if err != nil {
		return Claim{}, err
	}
	verb, n, err := parseRef(ref)
	if err != nil {
		return Claim{}, err
	}
	existing, err := t.Claims(ctx, repo, ref)
	if err != nil {
		return Claim{}, err
	}
	c := Claim{ID: id, Host: t.Host, State: state, At: t.clock().UTC()}
	payload, err := json.Marshal(c)
	if err != nil {
		return Claim{}, fmt.Errorf("claim: %w", err)
	}
	body := fmt.Sprintf("Claimed by Mellions lane `%s` on `%s` (%s).\n\n"+
		"Another session reading this: %s "+
		"A claim not restated within %s is stale and is swept rather than trusted.\n\n"+
		"%s%s -->\n", id, t.Host, state, audience(state), StaleAfter, marker, payload)

	if _, err := t.run(ctx, "label", "create", Label, "--repo", slug,
		"--color", "5319E7", "--description", "A Mellions lane holds this issue or pull request", "--force"); err != nil {
		return Claim{}, fmt.Errorf("claim: create label on %s: %w", slug, err)
	}
	if _, err := t.run(ctx, verb, "comment", strconv.Itoa(n), "--repo", slug, "--body", body); err != nil {
		return Claim{}, fmt.Errorf("claim: publish claim on %s: %w", name(slug, verb, n), err)
	}
	if _, err := t.run(ctx, verb, "edit", strconv.Itoa(n), "--repo", slug, "--add-label", Label); err != nil {
		return Claim{}, fmt.Errorf("claim: label %s: %w", name(slug, verb, n), err)
	}
	// Only after the new claim is up: a delete that runs first and a publish
	// that then fails leaves the issue unclaimed while the lane believes it
	// holds it.
	for _, old := range existing {
		if old.Host == t.Host && old.ID == id && old.Ref != "" {
			t.deleteComment(ctx, slug, old.Ref)
		}
	}
	return c, nil
}

// Release withdraws this machine's claim on an issue or pull request, and takes
// the label off when no other lane still holds it.
func (t *Tracker) Release(ctx context.Context, repo, ref, id string) error {
	slug, err := t.slug(repo)
	if err != nil {
		return err
	}
	verb, n, err := parseRef(ref)
	if err != nil {
		return err
	}
	claims, err := t.Claims(ctx, repo, ref)
	if err != nil {
		return err
	}
	remaining := 0
	for _, c := range claims {
		if c.Host == t.Host && c.ID == id {
			if c.Ref != "" {
				if err := t.deleteComment(ctx, slug, c.Ref); err != nil {
					return fmt.Errorf("claim: release %s on %s: %w", id, name(slug, verb, n), err)
				}
			}
			continue
		}
		if !c.Stale(t.clock()) {
			remaining++
		}
	}
	if remaining > 0 {
		return nil
	}
	if _, err := t.run(ctx, verb, "edit", strconv.Itoa(n), "--repo", slug, "--remove-label", Label); err != nil {
		return fmt.Errorf("claim: unlabel %s: %w", name(slug, verb, n), err)
	}
	return nil
}

// Sweep removes the claims on an issue or pull request that have gone stale,
// and returns them.
//
// Sweeping rather than warning is the whole point: a claim nobody can release —
// the machine is off, the session is gone — would otherwise make the issue
// permanently unclaimable, and an engineer that has to ask a person to clear a
// lock it invented is worse than no lock.
func (t *Tracker) Sweep(ctx context.Context, repo, ref string) ([]Claim, error) {
	slug, err := t.slug(repo)
	if err != nil {
		return nil, err
	}
	verb, n, err := parseRef(ref)
	if err != nil {
		return nil, err
	}
	claims, err := t.Claims(ctx, repo, ref)
	if err != nil {
		return nil, err
	}
	now := t.clock()
	var swept []Claim
	live := 0
	for _, c := range claims {
		if !c.Stale(now) {
			live++
			continue
		}
		if c.Ref != "" {
			if err := t.deleteComment(ctx, slug, c.Ref); err != nil {
				return swept, fmt.Errorf("claim: sweep %s on %s: %w", c.ID, name(slug, verb, n), err)
			}
		}
		swept = append(swept, c)
	}
	if len(swept) > 0 && live == 0 {
		if _, err := t.run(ctx, verb, "edit", strconv.Itoa(n), "--repo", slug, "--remove-label", Label); err != nil {
			return swept, fmt.Errorf("claim: unlabel %s: %w", name(slug, verb, n), err)
		}
	}
	return swept, nil
}

// Comment posts body on an issue or pull request and touches no claim.
//
// It exists so a lane's handoff can travel to the other host by the only route
// both machines already read. The assignment store is one machine's disk; the
// pull request is not.
func (t *Tracker) Comment(ctx context.Context, repo, ref, body string) error {
	slug, err := t.slug(repo)
	if err != nil {
		return err
	}
	verb, n, err := parseRef(ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("claim: nothing to say on %s", name(slug, verb, n))
	}
	if _, err := t.run(ctx, verb, "comment", strconv.Itoa(n), "--repo", slug, "--body", body); err != nil {
		return fmt.Errorf("claim: comment on %s: %w", name(slug, verb, n), err)
	}
	return nil
}

func (t *Tracker) deleteComment(ctx context.Context, slug, ref string) error {
	// gh returns the GraphQL node id for a comment; the REST delete wants the
	// numeric one. Both reach the same object through the GraphQL mutation, so
	// that is what this uses rather than guessing which id it was handed. A
	// pull request's conversation comments are IssueComment nodes too, and
	// `gh pr view --json comments` hands back the same IC_ ids, so one mutation
	// covers both kinds.
	_, err := t.run(ctx, "api", "graphql",
		"-f", "query=mutation($id:ID!){deleteIssueComment(input:{id:$id}){clientMutationId}}",
		"-f", "id="+ref)
	return err
}

// PullRequest is what the tracker says about one pull request on a branch.
type PullRequest struct {
	Number int `json:"number"`
	// State is the tracker's own word: OPEN, MERGED or CLOSED.
	State    string    `json:"state"`
	MergedAt time.Time `json:"mergedAt"`
}

// PullRequests reads every pull request whose head is branch, in every state.
//
// An error is unknown, never an empty list. A branch with no pull request and
// a tracker that could not be asked are the same silence and opposite
// instructions to whoever decides whether the lane behind the branch is done.
func (t *Tracker) PullRequests(ctx context.Context, repo, branch string) ([]PullRequest, error) {
	slug, err := t.slug(repo)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(branch) == "" {
		return nil, fmt.Errorf("claim: no branch to look for pull requests on")
	}
	out, err := t.run(ctx, "pr", "list", "--repo", slug, "--head", branch, "--state", "all",
		"--json", "number,state,mergedAt")
	if err != nil {
		return nil, fmt.Errorf("claim: read pull requests for %s on %s: %w", branch, slug, err)
	}
	var prs []PullRequest
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("claim: read pull requests for %s on %s: %w", branch, slug, err)
	}
	return prs, nil
}

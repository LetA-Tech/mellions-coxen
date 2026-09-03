// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package assignment is continuity for one piece of work.
//
// An engineer carrying remediation end to end has continuity at the level of
// the job, not of its career and not of the day. Coming back on Wednesday to a
// defect it opened on Monday, it reloads that job — not a ledger of everything
// it has ever done.
//
// So an assignment is a worktree, a branch, the issue it concerns, and a record
// of where the work stands. Native worktrees give isolation for free: two
// assignments are two directories with no shared mutable state, which is the
// thing a previous design spent an entire subsystem preventing.
//
// The record lives OUTSIDE the target repository, under the assignments root.
// Keeping it in the worktree — even ignored — puts the engineer's working
// memory in `git status` during real work and invites it into a commit. What
// another person needs belongs on the issue or the pull request; what the
// engineer needs to resume belongs here; and the two are not the same thing.
package assignment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
	"github.com/LetA-Tech/mellions-coxen/internal/durable"
)

// State is where an assignment stands.
const (
	// StateActive is work in progress.
	StateActive = "active"
	// StateBlocked is stopped at something the engineer cannot resolve — an
	// owner decision, or a dependency. It is a legitimate resting place, not a
	// failure, and it carries what is needed to unblock.
	StateBlocked = "blocked"
	// StateHandedOff is finished work whose handoff has been written but whose
	// worktree is kept, because a reviewer may still need it.
	StateHandedOff = "handed_off"
	// StateSuspended is work deliberately set down because something more
	// important appeared, with its worktree and branch kept intact.
	//
	// Distinct from blocked, which is stopped on something external. This one
	// is a choice the engineer made, and the difference matters: blocked work
	// resumes when the world changes, suspended work resumes when the engineer
	// decides it is the most valuable thing again.
	StateSuspended = "suspended"
	// StateClosed is done and released.
	StateClosed = "closed"
	// StateAbandoned is released with material destroyed rather than finished.
	StateAbandoned = "abandoned"
)

var states = []string{StateActive, StateBlocked, StateSuspended, StateHandedOff, StateClosed, StateAbandoned}

// ErrNotFound is returned when an id names no assignment.
var ErrNotFound = errors.New("assignment: not found")

// ErrDamaged is a record that exists and cannot be read. It is distinct from
// ErrNotFound on purpose: absent means the work was never here, and damaged
// means it was and the record of it did not survive. Treating the second as the
// first is how a session concludes that unfinished work was finished.
var ErrDamaged = errors.New("assignment: unreadable record")

// Budget bounds one assignment.
//
// It exists to force a written status rather than to stop work: an engineer
// that grinds silently past its budget and one that abandons a worktree are
// the same failure seen from different ends.
type Budget struct {
	// Wall is how long the assignment may run before a status is owed. Zero
	// means unbounded, which is a deliberate choice and not a default.
	Wall time.Duration `json:"wall_seconds_ns,omitempty"`
	// Note records why an unusual budget was set.
	Note string `json:"note,omitempty"`
}

// Finding is something learned while working.
//
// Kept here only until it reaches a durable home. A finding that another person
// needs belongs on the issue; this is the engineer's notebook between sessions,
// not the record of the work.
type Finding struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"` // hypothesis | found | next | note
	Text string    `json:"text"`
}

// Assignment is one piece of work and everything needed to resume it.
type Assignment struct {
	ID string `json:"id"`
	// Repo is the repository this concerns.
	Repo string `json:"repo"`
	// Issue is the tracked item, when there is one — or the work unit in the
	// repository's own register, where that is where its work is recorded.
	Issue string `json:"issue,omitempty"`
	// Register is where this repository records its own work, when that is not
	// the tracker. Held on the record so every surface that prints the lane can
	// say where the rows are, including after the store that resolved it is
	// gone.
	Register string `json:"register,omitempty"`
	// PullRequest is the lane's change set on the tracker, as "PR #12". Held
	// separately from Issue because it is claimed separately and usually later:
	// a lane opens on an issue and acquires a pull request halfway through.
	PullRequest string `json:"pull_request,omitempty"`
	// Objective is what this assignment exists to achieve, in the engineer's
	// own words at the moment it chose the work.
	Objective string `json:"objective"`
	// Because records why this work rather than the alternatives. It is the
	// counterfactual that makes selection quality measurable later.
	Because string `json:"because,omitempty"`
	// NotChosen names what was passed over, and why.
	NotChosen string `json:"not_chosen,omitempty"`

	Branch   string `json:"branch"`
	Worktree string `json:"worktree"`
	// Source is the checkout the worktree was cut from.
	Source string `json:"source"`
	// Base is the commit the branch started at, so a falsification can revert
	// to it without guessing.
	Base string `json:"base"`
	// BasePin says where that commit came from, in the words a later session
	// needs when it asks whether the premise was re-verified against the tree
	// everyone else has: the ref, when it was fetched, and any local head that
	// was declined for being behind it.
	BasePin string `json:"base_pin,omitempty"`
	// Adopted marks a worktree and branch that something else created. They
	// are recorded, never removed: close and abandon leave them in place.
	Adopted bool `json:"adopted,omitempty"`

	State     string    `json:"state"`
	OpenedAt  time.Time `json:"opened_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ClosedAt  time.Time `json:"closed_at,omitzero"`

	Budget   Budget    `json:"budget,omitzero"`
	Findings []Finding `json:"findings,omitempty"`
	// Handoff is the written statement of where the work stands. Required
	// before closing: an assignment that ends without one destroys whatever the
	// session knew and had not committed.
	Handoff string `json:"handoff,omitempty"`
	// Suspensions is every time this work was set down for something more
	// important. A list rather than a flag: an assignment interrupted four
	// times is telling you something a boolean cannot.
	Suspensions []Suspension `json:"suspensions,omitempty"`
	// Discarded is what abandoning this work destroyed. Present only where the
	// worktree was removed with material in it.
	Discarded *Discarded `json:"discarded,omitempty"`
	// Sessions is every runtime session that touched this work. It is how
	// recovery reaches for the runtime's own resume before rebuilding anything
	// from the record.
	Sessions []Session `json:"sessions,omitempty"`
	// Claim is this lane's hold on the issue as it stands on the tracker.
	// Absent on a lane with no issue; present and saying so on a lane whose
	// claim could not be published.
	Claim *ClaimState `json:"claim,omitempty"`
}

// ClaimState is what this machine believes about its published hold on an
// issue.
//
// It is belief, not truth: the tracker is the record, and this exists so a
// session can tell a lane whose claim other machines can see from one whose
// claim never left this disk. The two look identical without it, and the second
// is the one that lets a second machine open the same work.
type ClaimState struct {
	// Host is the machine named in the published claim.
	Host string `json:"host,omitempty"`
	// At is when the claim was last restated on the tracker.
	At time.Time `json:"at,omitzero"`
	// Unpublished says why this lane's hold is not on the tracker, in the
	// tracker's own words. Empty means published. A lane carrying this is
	// visible to nothing but this machine, and every surface that prints a lane
	// says so rather than letting it read as an ordinary claim.
	Unpublished string `json:"unpublished,omitempty"`
	// Stranded says a release failed and the claim is still on the tracker.
	// It is not a failure to act on: an unreleased claim goes stale and is
	// swept by whoever next reads the issue.
	Stranded string `json:"stranded,omitempty"`
}

// Published reports whether other machines can see this lane's hold.
func (c *ClaimState) Published() bool { return c != nil && c.Unpublished == "" }

// Tracker publishes a lane's hold on an issue where every machine can see it.
//
// An interface rather than the concrete tracker so the store can be exercised
// against a tracker that fails, and so an installation on a different tracker
// is a new implementation rather than a change here.
type Tracker interface {
	Claims(ctx context.Context, repo, ref string) ([]claim.Claim, error)
	Publish(ctx context.Context, repo, ref, id, state string) (claim.Claim, error)
	Release(ctx context.Context, repo, ref, id string) error
	Sweep(ctx context.Context, repo, ref string) ([]claim.Claim, error)
	// PullRequests finds the lane's change set from its branch, for a lane
	// that never recorded one.
	PullRequests(ctx context.Context, repo, branch string) ([]claim.PullRequest, error)
	// Comment posts prose where both hosts read it.
	Comment(ctx context.Context, repo, ref, body string) error
}

// claimRefs is every tracker item this lane holds.
//
// One ClaimState covers both: it is one lane, on one host, publishing at one
// moment, and the two refs cannot disagree about any of those.
func (a *Assignment) claimRefs() []string {
	var refs []string
	if r := strings.TrimSpace(a.Issue); r != "" {
		refs = append(refs, r)
	}
	if r := strings.TrimSpace(a.PullRequest); r != "" {
		refs = append(refs, r)
	}
	return refs
}

// Suspension is one interruption.
//
// A real engineer does not finish a low-value task while something materially
// more important is happening. What separates that from abandoning work is
// entirely what gets written down at the moment of setting it down — so both
// fields are required, and neither can be reconstructed afterwards.
type Suspension struct {
	At time.Time `json:"at"`
	// For names what took priority, so the judgement can be reviewed later
	// against what that work turned out to be worth.
	For string `json:"for"`
	// Stands is where this work was, in the terms a resuming session needs.
	Stands string `json:"stands"`
	// ResumedAt is when it was picked up again. Zero while still set down.
	ResumedAt time.Time `json:"resumed_at,omitzero"`
}

// Open reports whether this suspension is still in force.
func (s Suspension) Open() bool { return s.ResumedAt.IsZero() }

// Overdue reports whether the budget has elapsed without a handoff.
func (a Assignment) Overdue(now time.Time) bool {
	if a.Budget.Wall <= 0 || a.State == StateClosed || a.State == StateHandedOff {
		return false
	}
	// Suspended work is not overrunning; it was put down on purpose. Counting
	// the time it sits would make every interruption look like an abandonment
	// and teach the engineer to finish low-value work rather than set it aside.
	if a.State == StateSuspended {
		return false
	}
	return now.Sub(a.OpenedAt) > a.Budget.Wall
}

// Age is how long the assignment has been open.
func (a Assignment) Age(now time.Time) time.Duration { return now.Sub(a.OpenedAt) }

// Store holds assignments on disk, outside every target repository.
type Store struct {
	// Root is the assignments directory.
	Root string
	// Git runs git; replaced in tests so worktree behaviour can be asserted
	// without a real repository where that is not the point.
	Git Runner
	// Tracker publishes claims where other machines see them. Nil is a store
	// that cannot publish, and a lane with an issue opened against one is
	// refused rather than quietly becoming local-only.
	Tracker Tracker
	// Registers maps a repository to the path, inside it, where that
	// repository records its own work: the rows, the decisions and the open
	// remediations. Not every repository's work register is the tracker, and a
	// reference into one is not a number gh can address.
	Registers map[string]string
	now       func() time.Time
}

// register is where a repository records its own work, or "" where the tracker
// is the register.
func (s *Store) register(repo string) string {
	if s.Registers == nil {
		return ""
	}
	return s.Registers[strings.ToLower(strings.TrimSpace(repo))]
}

// inOwnRegister reports whether this reference names work in the repository's
// own register rather than an item on the tracker.
//
// The distinction has to exist because refusing the reference is worse than
// either answer. A repository whose work lives in a document has real work
// units with real identifiers, and a lane on one was pushed to -unpublished —
// which says "no other session can see this" and disables the local collision
// check nothing else provides. Two lanes then re-derived the same open
// remediation row on one afternoon, in the one repository where lanes actually
// collide.
func (s *Store) inOwnRegister(repo, issue string) bool {
	return s.register(repo) != "" && strings.TrimSpace(issue) != "" && !claim.Addressable(issue)
}

// Runner executes a command in a directory and returns combined output.
type Runner func(dir string, args ...string) ([]byte, error)

// NewStore returns a store rooted at dir, creating it if absent.
func NewStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("assignment: no assignments root configured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("assignment: create %s: %w", dir, err)
	}
	return &Store{Root: dir, Git: gitRunner, now: time.Now}, nil
}

func (s *Store) clock() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

func (s *Store) dir(id string) string  { return filepath.Join(s.Root, id) }
func (s *Store) file(id string) string { return filepath.Join(s.dir(id), "assignment.json") }

// OpenOptions describes work about to start.
type OpenOptions struct {
	ID        string
	Repo      string
	Issue     string
	Objective string
	Because   string
	NotChosen string
	// Source is the checkout to cut a worktree from.
	Source string
	// Branch defaults to a name derived from the id and issue.
	Branch string
	// BaseRef is what the branch is cut from; empty uses the source's HEAD.
	BaseRef string
	// Worktree, when set, is an existing working tree of Source to adopt
	// instead of cutting one: a repository's own process or a person made it,
	// and the lane records it without owning it.
	Worktree string
	// Alongside opens this lane even though another live lane already claims
	// the same issue. Reconciling two lanes is the case that needs it.
	Alongside bool
	// Unpublished opens a lane on an issue whose claim could not reach the
	// tracker, accepting that no other machine can see it. Saying it is the
	// point: the failure it covers is the one that silently produces two lanes
	// on two hosts, and the lane it opens is marked local-only everywhere it is
	// printed.
	Unpublished bool
	Budget      Budget
}

// Open creates an assignment and its worktree.
//
// The worktree is created before the record is written, and the record is
// removed if the worktree fails: a record pointing at a directory that does not
// exist is worse than no record, because a later session will trust it.
func (s *Store) Open(o OpenOptions) (*Assignment, error) {
	id := strings.TrimSpace(o.ID)
	if id == "" {
		return nil, errors.New("assignment: an assignment needs an id")
	}
	if strings.ContainsAny(id, "/\\ ") {
		return nil, fmt.Errorf("assignment: id %q must not contain a path separator or space", id)
	}
	if strings.TrimSpace(o.Objective) == "" {
		return nil, errors.New("assignment: an assignment needs an objective — what this work is for")
	}
	// Required, not optional. Selection agreement is the measure that decides
	// whether this is an engineer or a tool, and it can only be judged against a
	// stated reason. A measure that depends on someone choosing to supply it
	// stops being collected, quietly, at exactly the moment it matters.
	//
	// "The owner asked for it" is a perfectly good reason. Having none is not.
	if strings.TrimSpace(o.Because) == "" {
		return nil, errors.New("assignment: say why this work rather than the alternatives (-because). " +
			"It is what makes the choice judgeable later, and \"the owner asked for it\" counts")
	}
	if o.Source == "" {
		return nil, errors.New("assignment: no source checkout to cut a worktree from")
	}
	// Everything from here is one operation across processes, because the
	// alternative is what two agents told to take the same work actually do.
	// Unguarded, both pass the existence check, both create git state, and the
	// loser's cleanup can remove the winner's directory — so neither ends up
	// with a lane and the branch one of them cut survives with nothing pointing
	// at it. The id is then unusable forever: every later open fails on a branch
	// no command in this product mentions.
	// The lock lives beside the record, which means the record's directory has
	// to exist before it can be taken. Making it here rather than in create
	// keeps one lock file per assignment — the same one every other mutator
	// uses — instead of a second one somewhere else that guards a different
	// thing.
	if err := os.MkdirAll(s.dir(id), 0o755); err != nil {
		return nil, fmt.Errorf("assignment: create %s: %w", s.dir(id), err)
	}
	var opened *Assignment
	err := durable.Guard(s.file(id), func() error {
		a, err := s.create(id, o)
		if err != nil {
			return err
		}
		opened = a
		return nil
	})
	if err != nil {
		// The directory had to exist before the lock beside the record could be
		// taken, so a create that refused left one behind holding nothing but
		// that lock. It does not block a later open — every check keys on the
		// record, not the directory — but an id with a directory and no record
		// is exactly the shape a session reads as a half-made lane. Removed
		// non-recursively and unchecked: if anything else is in there, create
		// got further than a refusal and the directory is not litter.
		os.Remove(filepath.Join(s.dir(id), "assignment.json.lock"))
		os.Remove(s.dir(id))
		return nil, err
	}
	return opened, nil
}

// sameIssue reports whether two claims name the same tracked item.
//
// The spellings a session actually uses are compared, not required to match:
// "#43", "43" and " #43 " are one issue, and a repository name differing
// only in case is one repository. An assignment with no issue claims no issue
// and collides with nothing.
func sameIssue(aRepo, aIssue, bRepo, bIssue string) bool {
	norm := func(repo, issue string) (string, string) {
		return strings.ToLower(strings.TrimSpace(repo)),
			strings.TrimPrefix(strings.TrimSpace(issue), "#")
	}
	ar, ai := norm(aRepo, aIssue)
	br, bi := norm(bRepo, bIssue)
	return ai != "" && ai == bi && ar == br
}

// issueUnclaimed refuses to open a second live lane on an issue this store
// already holds one for.
//
// Until this existed, Open guarded uniqueness by assignment id alone. The issue
// was recorded on the record and never read back, so two ids naming one issue
// both opened, and the second session had no way to learn the first had already
// taken it — it duplicated the work and found out at the pull request.
//
// Handed-off lanes count as live on purpose. Handed off means finished and
// waiting on a person, which is the state most likely to be re-claimed by a
// session that surveyed the issue and saw it still open; closed and abandoned
// lanes have released the work and do not collide.
//
// Refusing is the default rather than warning because a warning printed to a
// session that has already decided is not a control. Taking the work anyway is
// a legitimate choice — reconciling two lanes needs exactly that — so it stays
// available, but it has to be said.
//
// The scan is not serialised against a concurrent open of a different id: the
// lock Open holds is the one beside this assignment's own record. Two opens
// racing on one issue within the same instant can still both pass. This catches
// the case that actually happens, which is minutes apart, and the collision it
// cannot catch is the one where no record existed to read yet.
func (s *Store) issueUnclaimed(o OpenOptions) error {
	if o.Alongside || strings.TrimSpace(o.Issue) == "" {
		return nil
	}
	live, _, err := s.ListWithDamage(false)
	if err != nil {
		return nil // a store that cannot be listed is not evidence the issue is claimed
	}
	for _, a := range live {
		if a.ID == o.ID || !sameIssue(a.Repo, a.Issue, o.Repo, o.Issue) {
			continue
		}
		return fmt.Errorf("assignment: %s %s is already claimed by %s (%s), opened %s.\n\n"+
			"Read what it established before repeating it:\n"+
			"  mellions assign get %s\n\n"+
			"If this is deliberate — reconciling the two, or the earlier lane is finished and\n"+
			"you know it — say so:\n"+
			"  mellions assign open %s ... -alongside",
			o.Repo, o.Issue, a.ID, a.State, a.OpenedAt.Format(time.RFC3339), a.ID, o.ID)
	}
	return nil
}

// estateUnclaimed refuses to open a lane on an issue another machine holds.
//
// The local scan above is this disk. Every session in the estate surveys the
// same tracker, so two hosts reading the same open issue each see it unheld and
// each take it — the local guard cannot see the other machine at all, and the
// first thing either learns is a second pull request. This asks the one thing
// both machines already read.
//
// Stale claims are swept, not obeyed. A host that lost power mid-lane leaves a
// claim nothing can release, and honouring it would make the issue permanently
// unclaimable by anyone; the sweep is what keeps a lock this engineer invented
// from becoming something a person has to clear by hand.
//
// A tracker that cannot be read is not evidence the issue is free, and this
// says so rather than proceeding. Publishing will fail in the same breath
// anyway — refusing here just names the reason before a worktree exists.
func (s *Store) estateUnclaimed(ctx context.Context, o OpenOptions) error {
	// -unpublished has already said the tracker is unreachable and a local-only
	// lane is accepted. Refusing here for the same unreachability would leave
	// no way through at all.
	if o.Alongside || o.Unpublished || strings.TrimSpace(o.Issue) == "" || s.Tracker == nil {
		return nil
	}
	if s.inOwnRegister(o.Repo, o.Issue) {
		return nil
	}
	// Nothing here declares where this repository records its work, so a
	// reference the tracker cannot address is a typo rather than a work unit.
	// Opening it would be a lane holding a claim that reaches nothing, which is
	// the state the claim exists to end; the refusal names both ways forward
	// rather than only the flag that gives up.
	if !claim.Addressable(o.Issue) {
		return fmt.Errorf("assignment: %s %q is not a reference the tracker can address, and nothing "+
			"says %s records its work anywhere else.\n\nEither give the issue or pull request number, "+
			"or declare the register in the configuration so a work unit there is a real claim:\n"+
			"  \"work_registers\": {%q: \"docs/…/tracker.md\"}",
			o.Repo, o.Issue, o.Repo, o.Repo)
	}
	unreadable := func(err error) error {
		return fmt.Errorf("assignment: %s %s: the estate's claims could not be read, so whether "+
			"another machine holds this is unknown: %w\n\nOpen it local-only, knowing a session "+
			"elsewhere may hold or take the same work:\n  mellions assign open %s ... -unpublished",
			o.Repo, o.Issue, err, o.ID)
	}
	if swept, err := s.Tracker.Sweep(ctx, o.Repo, o.Issue); err != nil {
		return unreadable(err)
	} else if len(swept) > 0 {
		for _, c := range swept {
			fmt.Fprintf(os.Stderr, "assignment: swept a stale claim on %s %s — %s\n",
				o.Repo, o.Issue, c.Held())
		}
	}
	claims, err := s.Tracker.Claims(ctx, o.Repo, o.Issue)
	if err != nil {
		return unreadable(err)
	}
	now := s.clock()
	for _, c := range claims {
		if c.ID == o.ID || c.Stale(now) {
			continue
		}
		return fmt.Errorf("assignment: %s %s is claimed on the tracker by %s.\n\n"+
			"That lane is on another machine, so its record is not readable from here — the issue's\n"+
			"own %s claim is. Read it before repeating the work.\n\n"+
			"If this is deliberate — reconciling the two, or that lane is finished and you know it:\n"+
			"  mellions assign open %s ... -alongside",
			o.Repo, o.Issue, c.Held(), claim.Label, o.ID)
	}
	return nil
}

// publishClaim puts this lane's hold where every machine can see it, and
// settles what happens when it cannot.
//
// The failure this exists for is not a network error; it is a lane that
// believes it holds an issue nothing else can see. Silently degrading to a
// local-only claim reproduces exactly the state the claim was built to end, so
// an unpublishable claim refuses the open and names the one flag that accepts
// it. A lane opened that way carries the reason on its record and prints it
// everywhere the lane is printed.
func (s *Store) publishClaim(ctx context.Context, a *Assignment, unpublished bool) error {
	refs := a.claimRefs()
	if len(refs) == 0 {
		return nil
	}
	// A reference into the repository's own register cannot be published to the
	// tracker and is not a failure to publish. The lane says where its hold
	// does reach, which is this machine's store and the local collision check,
	// and says it everywhere the lane is printed.
	if s.inOwnRegister(a.Repo, a.Issue) {
		a.Claim = &ClaimState{Unpublished: a.Repo + " records " + a.Issue + " in " +
			s.register(a.Repo) + " rather than on the tracker, so this hold is on this machine"}
		return nil
	}
	if s.Tracker == nil {
		if unpublished {
			a.Claim = &ClaimState{Unpublished: "no tracker is configured for this store"}
			return nil
		}
		return fmt.Errorf("assignment: %s %s cannot be claimed: no tracker is configured, so no other "+
			"machine could see this lane.\n\nOpen it local-only, knowing a session elsewhere may take "+
			"the same work:\n  mellions assign open %s ... -unpublished", a.Repo, a.Issue, a.ID)
	}
	for _, ref := range refs {
		c, err := s.Tracker.Publish(ctx, a.Repo, ref, a.ID, a.State)
		if err != nil {
			if unpublished {
				a.Claim = &ClaimState{Unpublished: err.Error()}
				return nil
			}
			return fmt.Errorf("assignment: %s %s could not be claimed on the tracker, so no other machine "+
				"can see this lane: %w\n\nA claim that cannot be published is not a claim. Open it "+
				"local-only, knowing a session elsewhere may take the same work:\n"+
				"  mellions assign open %s ... -unpublished", a.Repo, ref, err, a.ID)
		}
		a.Claim = &ClaimState{Host: c.Host, At: c.At}
	}
	return nil
}

// ClaimPullRequest publishes this lane's hold on a pull request and records it.
//
// A draft carries one bit of meaning and it is overloaded: unfinished, finished
// but unreviewed, and finished with a review in flight all read as "draft". A
// peer on another host choosing work merges the third believing it is the
// second, which is exactly what happened. The claim is what separates them, and
// it is on the tracker because the tracker is the only thing both hosts read.
//
// Publishing first, recording second: a claim the tracker refused is not a
// claim, and a record saying the lane holds a pull request nothing else can see
// reproduces the failure this exists to end.
func (s *Store) ClaimPullRequest(ctx context.Context, id, pr string) error {
	ref, err := claim.PullRequestRef(pr)
	if err != nil {
		return err
	}
	a, err := s.Get(id)
	if err != nil {
		return err
	}
	if a.State == StateClosed || a.State == StateAbandoned {
		return fmt.Errorf("assignment: %s is %s; a finished lane does not hold %s", id, a.State, ref)
	}
	if s.Tracker == nil {
		return fmt.Errorf("assignment: %s cannot claim %s: no tracker is configured, so no other "+
			"machine could see the hold", id, ref)
	}
	c, err := s.Tracker.Publish(ctx, a.Repo, ref, a.ID, a.State)
	if err != nil {
		return fmt.Errorf("assignment: %s %s could not be claimed on the tracker, so a peer would "+
			"still read this change set as unheld: %w", a.Repo, ref, err)
	}
	_, err = s.update(id, func(a *Assignment) error {
		a.PullRequest = ref
		if a.Claim == nil {
			a.Claim = &ClaimState{Host: c.Host}
		}
		a.Claim.At = c.At
		return nil
	})
	return err
}

// restateClaim pushes the lane's current state back onto the tracker, which is
// what keeps the claim from going stale under it.
//
// Every write to the record restates it, because a lane being worked writes to
// its record and a lane that has not is the one whose claim should expire.
// Failure is not fatal here: the claim is already published and the work is
// already recorded, and the worst case is a claim that goes stale early and is
// swept — which is the designed behaviour, not a defect.
func (s *Store) restateClaim(a *Assignment) {
	refs := a.claimRefs()
	// A lane that already said its claim is local-only is left alone. A lane
	// with refs and no ClaimState yet is one that just acquired a pull request:
	// it gets one here rather than waiting for a claim it will never have.
	if s.Tracker == nil || len(refs) == 0 || (a.Claim != nil && a.Claim.Unpublished != "") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, ref := range refs {
		c, err := s.Tracker.Publish(ctx, a.Repo, ref, a.ID, a.State)
		if err != nil {
			continue
		}
		if a.Claim == nil {
			a.Claim = &ClaimState{Host: c.Host}
		}
		a.Claim.At = c.At
	}
}

// releaseClaim withdraws the lane's hold when the work is closed or abandoned.
//
// A release that fails is recorded, never fatal. The lane is finished either
// way, and the claim it left behind stops being obeyed once it goes stale —
// refusing to close a finished lane because GitHub was briefly unreachable
// would trade a self-healing residue for a stuck one.
func (s *Store) releaseClaim(a *Assignment) {
	refs := a.claimRefs()
	if len(refs) == 0 || a.Claim == nil || !a.Claim.Published() {
		return
	}
	if s.Tracker == nil {
		a.Claim.Stranded = "no tracker is configured; the published claim will be swept when it goes stale"
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Every ref is attempted even after one fails: a lane that let go of its
	// issue and kept its pull request would leave a draft looking held.
	stranded := ""
	for _, ref := range refs {
		if err := s.Tracker.Release(ctx, a.Repo, ref, a.ID); err != nil {
			stranded = ref + ": " + err.Error()
		}
	}
	if stranded != "" {
		a.Claim.Stranded = stranded
		return
	}
	a.Claim = nil
}

// create is Open's body, run under the lock.
//
// Its failure paths undo what it made. A branch cut for a lane that then failed
// to open is not harmless litter: it is the thing that makes the id permanently
// unopenable, and it is invisible to every command here.
func (s *Store) create(id string, o OpenOptions) (*Assignment, error) {
	if _, err := os.Stat(s.file(id)); err == nil {
		return nil, fmt.Errorf("assignment: %s already exists", id)
	}
	if err := s.issueUnclaimed(o); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.estateUnclaimed(ctx, o); err != nil {
		return nil, err
	}

	if o.Worktree != "" {
		return s.adopt(id, o)
	}

	branch := o.Branch
	if branch == "" {
		branch = "mellions/" + id
	}
	worktree := filepath.Join(s.dir(id), "tree")

	if err := os.MkdirAll(s.dir(id), 0o755); err != nil {
		return nil, fmt.Errorf("assignment: create %s: %w", s.dir(id), err)
	}

	base, pin := s.baseFor(o.Source, o.BaseRef)
	if base == "" {
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: read HEAD of %s: nothing to cut %s from", o.Source, branch)
	}

	// Whether the branch is ours to undo is decided before it is made, not
	// after. A branch that was already there belongs to something else — an
	// earlier lane, a person, a leaked ref from a failure like this one — and
	// removing it on the way out of our own failure would be a worse defect
	// than the one this cleanup exists for.
	if _, err := s.Git(o.Source, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: %s cannot be opened: branch %s already exists in %s and no "+
			"assignment holds it. Something left it behind, or somebody else is using it. Look before "+
			"removing it: `git -C %s log --oneline %s`",
			id, branch, o.Source, o.Source, branch)
	}

	if _, err := s.Git(o.Source, "worktree", "add", "-b", branch, worktree, base); err != nil {
		// git writes the ref before it registers the working tree, so a failure
		// between the two leaves a branch with no lane — invisible here, and
		// enough to make the id unopenable for good. This is ours to undo
		// because the check above established the branch was not there.
		s.Git(o.Source, "worktree", "remove", "--force", worktree)
		s.Git(o.Source, "branch", "-D", branch)
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: create worktree for %s: %w", id, err)
	}

	now := s.clock()
	a := &Assignment{
		ID: id, Repo: o.Repo, Issue: o.Issue, Register: s.register(o.Repo),
		Objective: o.Objective, Because: o.Because, NotChosen: o.NotChosen,
		Branch: branch, Worktree: worktree, Source: o.Source, Base: base, BasePin: pin,
		State: StateActive, OpenedAt: now, UpdatedAt: now, Budget: o.Budget,
	}
	undo := func() {
		s.Git(o.Source, "worktree", "remove", "--force", worktree)
		s.Git(o.Source, "branch", "-D", branch)
		os.RemoveAll(s.dir(id))
	}
	if err := s.publishClaim(ctx, a, o.Unpublished); err != nil {
		undo()
		return nil, err
	}
	if err := s.save(a); err != nil {
		s.releaseClaim(a)
		undo()
		return nil, err
	}
	return a, nil
}

// baseFor settles what a lane is cut from, and says so in words a later session
// can check.
//
// The default is the remote-tracking head, fetched now — not the source
// checkout's local branch head. The source is a long-lived checkout that
// nothing updates: lanes work in worktrees, so nobody runs `git pull` in it,
// and its branch ref is stale by default. Cutting from it starts the lane
// behind the tree everyone else shares, and the first thing a lane does is
// re-verify its work item's premise at base HEAD and state the pin. Against a
// stale base that pin is precisely wrong in the way that reads right — "premise
// confirmed at dev @ <sha>", where the sha is not what dev is — and a session
// reading a line number that moved will "correct" an issue against a head
// nobody shares. It also widens every pull request's diff base.
//
// Nor is the branch the source checkout sits on the answer. A long-lived
// checkout may sit on a release branch while work starts from a newer
// development branch. The working branch is therefore resolved in order: the
// repository declaration, then `dev`, and only then the tracked branch.
//
// Nothing here is fatal. A checkout tracking no remote, an unreachable one, a
// fetch that fails: each falls back to the local head and says which it used,
// because a lane that cannot start is worse than one told plainly where it is.
func (s *Store) baseFor(source, ref string) (base, pin string) {
	local := ""
	if out, err := s.Git(source, "rev-parse", "HEAD"); err == nil {
		local = strings.TrimSpace(string(out))
	}
	// An explicit -base is the session saying it means this one. It is resolved
	// so the record holds a commit rather than a name that moves under it, and
	// nothing is fetched.
	if strings.TrimSpace(ref) != "" {
		out, err := s.Git(source, "rev-parse", ref+"^{commit}")
		if err != nil {
			return "", ""
		}
		return strings.TrimSpace(string(out)), "given as -base " + ref
	}

	upstream := ""
	if out, err := s.Git(source, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		upstream = strings.TrimSpace(string(out))
	}

	remote := "origin"
	if r, _, ok := strings.Cut(upstream, "/"); ok && r != "" {
		remote = r
	}

	for _, want := range workingBranchCandidates(source) {
		if want == "" || remote+"/"+want == upstream {
			continue
		}
		if base, pin = s.resolveRemoteBranch(source, local, remote, want); base != "" {
			return base, pin
		}
	}

	if upstream == "" {
		return local, "local HEAD — " + short(local) + "; the checkout tracks no remote branch"
	}
	_, branch, _ := strings.Cut(upstream, "/")
	if base, pin = s.resolveRemoteBranch(source, local, remote, branch); base != "" {
		return base, pin
	}
	return local, "local HEAD — " + short(local) + "; " + upstream + " could not be read"
}

// workingBranchCandidates names the branches a lane should be cut from, best
// first. The repository's own declaration wins over the estate convention,
// because a repository saying where its work happens is evidence and a
// convention is an assumption.
func workingBranchCandidates(source string) []string {
	declared := declaredWorkingBranch(source)
	if declared == "dev" {
		return []string{"dev"}
	}
	return []string{declared, "dev"}
}

// declaredWorkingBranch reads .claude/repo-binding.yaml's development_branch.
// One key is read with one pattern rather than a YAML dependency: an absent
// file, an absent key or a shape this does not match all answer "not
// declared", which is the same answer and costs the caller nothing.
func declaredWorkingBranch(source string) string {
	raw, err := os.ReadFile(filepath.Join(source, ".claude", "repo-binding.yaml"))
	if err != nil {
		return ""
	}
	m := developmentBranchKey.FindSubmatch(raw)
	if m == nil {
		return ""
	}
	return string(m[1])
}

var developmentBranchKey = regexp.MustCompile(`(?m)^development_branch:[ \t]*["']?([A-Za-z0-9._/-]+)`)

// resolveRemoteBranch fetches one branch and resolves it, returning "" when the
// remote does not carry it. The fetch is what makes the pin current; a branch
// resolved out of a stale remote-tracking ref is the defect this whole function
// exists to avoid, so a failed fetch is reported in the pin rather than hidden.
func (s *Store) resolveRemoteBranch(source, local, remote, branch string) (base, pin string) {
	if branch == "" {
		return "", ""
	}
	ref := remote + "/" + branch
	fetched := "fetched"
	if _, err := s.Git(source, "fetch", remote, branch); err != nil {
		fetched = "NOT fetched — fetch failed, so this is the last state seen locally"
	}
	out, err := s.Git(source, "rev-parse", ref+"^{commit}")
	if err != nil {
		return "", ""
	}
	base = strings.TrimSpace(string(out))
	if base == "" {
		return "", ""
	}
	pin = ref + " " + fetched + " " + s.clock().UTC().Format(time.RFC3339)
	if local != "" && local != base {
		behind := ""
		if n, err := s.Git(source, "rev-list", "--count", local+".."+base); err == nil {
			behind = ", " + strings.TrimSpace(string(n)) + " behind"
		}
		pin += "; declined the local head " + short(local) + " in " + source + behind
	}
	return base, pin
}

// adopt records a lane in a working tree that already exists. A repository's
// own process may dictate where its work happens and what the branch is
// called; forcing a second tree beside it makes the record point at the wrong
// place, and a record that lies about where the work is is worse than none.
//
// Nothing adopted is ours to destroy. The tree and the branch stay where they
// are whatever happens to the lane.
func (s *Store) adopt(id string, o OpenOptions) (*Assignment, error) {
	tree, err := filepath.Abs(o.Worktree)
	if err == nil {
		tree, err = filepath.EvalSymlinks(tree)
	}
	if err != nil {
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: adopt %s: %w", o.Worktree, err)
	}
	top, err := s.Git(tree, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(strings.TrimSpace(string(top)), tree) {
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: %s is not the top of a git working tree", tree)
	}
	if !sameGitDir(s, tree, o.Source) {
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: %s is not a working tree of %s", tree, o.Source)
	}
	out, err := s.Git(tree, "rev-parse", "--abbrev-ref", "HEAD")
	branch := strings.TrimSpace(string(out))
	if err != nil || branch == "" || branch == "HEAD" {
		os.RemoveAll(s.dir(id))
		return nil, fmt.Errorf("assignment: %s is not on a branch; a lane needs one to record", tree)
	}
	head, _ := s.Git(tree, "rev-parse", "HEAD")
	base := strings.TrimSpace(string(head))
	if mb, err := s.Git(o.Source, "merge-base", "HEAD", base); err == nil && strings.TrimSpace(string(mb)) != "" {
		base = strings.TrimSpace(string(mb))
	}
	now := s.clock()
	a := &Assignment{
		ID: id, Repo: o.Repo, Issue: o.Issue, Register: s.register(o.Repo),
		Objective: o.Objective, Because: o.Because, NotChosen: o.NotChosen,
		Branch: branch, Worktree: tree, Source: o.Source, Base: base, Adopted: true,
		State: StateActive, OpenedAt: now, UpdatedAt: now, Budget: o.Budget,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.publishClaim(ctx, a, o.Unpublished); err != nil {
		os.RemoveAll(s.dir(id))
		return nil, err
	}
	if err := s.save(a); err != nil {
		s.releaseClaim(a)
		os.RemoveAll(s.dir(id))
		return nil, err
	}
	return a, nil
}

// sameGitDir reports whether two directories share one repository.
func sameGitDir(s *Store, a, b string) bool {
	common := func(dir string) string {
		out, err := s.Git(dir, "rev-parse", "--git-common-dir")
		if err != nil {
			return ""
		}
		p := strings.TrimSpace(string(out))
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		return p
	}
	ca, cb := common(a), common(b)
	return ca != "" && ca == cb
}

func samePath(a, b string) bool {
	if r, err := filepath.EvalSymlinks(a); err == nil {
		a = r
	}
	if r, err := filepath.EvalSymlinks(b); err == nil {
		b = r
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func (s *Store) save(a *Assignment) error {
	a.UpdatedAt = s.clock()
	a.stamp(a.UpdatedAt)
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("assignment: encode %s: %w", a.ID, err)
	}
	// Staged under a unique name, flushed and renamed: a crash mid-write must
	// not leave a truncated record that a resuming session reads as the truth,
	// and a shared staging name would make two concurrent writers collide on
	// the one file that says what this session is carrying.
	if err := durable.Write(s.file(a.ID), append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("assignment: write %s: %w", a.ID, err)
	}
	return nil
}

// update applies fn to one assignment under an exclusive lock.
//
// Every change here is a read, a decision taken from what was read, and a write
// back. Unguarded, two of those lose one change and keep the other with no
// error anywhere — and overlapping sessions are exactly the situation the
// record exists to survive.
func (s *Store) update(id string, fn func(*Assignment) error) (*Assignment, error) {
	var out *Assignment
	err := durable.Guard(s.file(id), func() error {
		a, err := s.Get(id)
		if err != nil {
			return err
		}
		if err := fn(a); err != nil {
			return err
		}
		// Every write to the record restates the lane's hold, which is what
		// keeps a lane being worked from expiring under it. A lane that has
		// released — closed or abandoned — is not restated back onto the issue
		// it just let go of.
		if a.State != StateClosed && a.State != StateAbandoned {
			s.restateClaim(a)
		}
		if err := s.save(a); err != nil {
			return err
		}
		out = a
		return nil
	})
	return out, err
}

// Get loads one assignment.
func (s *Store) Get(id string) (*Assignment, error) {
	raw, err := os.ReadFile(s.file(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("assignment: read %s: %w", id, err)
	}
	var a Assignment
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("%w: %s (%v).\n\n"+
			"A record is written whole and renamed into place, so a truncated one means the "+
			"write did not survive — a crash, a full disk, an interrupted copy. What it held is "+
			"not recoverable from here.\n\n"+
			"The branch and its commits are the durable part and are untouched. Read them, and "+
			"open a new assignment against the same work rather than repairing this file by "+
			"hand: a record edited into shape asserts things nobody established.",
			ErrDamaged, id, err)
	}
	return &a, nil
}

// List returns every assignment, newest first. Without includeClosed it
// omits what is finished — closed and abandoned alike: an abandoned lane listed
// as work in flight is the record contradicting itself.
func (s *Store) List(includeClosed bool) ([]*Assignment, error) {
	out, _, err := s.ListWithDamage(includeClosed)
	return out, err
}

// ListWithDamage lists what is readable and names what is not.
//
// One unreadable record used to fail the whole listing, which took `assign
// list` and `mellions continue` with it — and continuity is the command whose
// entire purpose is recovering after a session died, which is exactly when a
// record gets truncated. The one command that must work in that state was the
// one that could not.
//
// So a damaged record is a finding about that record, never a failure of
// everything beside it. It is returned rather than skipped, because silently
// listing four of five assignments is how somebody concludes the fifth was
// closed.
func (s *Store) ListWithDamage(includeClosed bool) ([]*Assignment, []string, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("assignment: read %s: %w", s.Root, err)
	}
	var out []*Assignment
	var damaged []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		a, err := s.Get(e.Name())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if errors.Is(err, ErrDamaged) {
			damaged = append(damaged, e.Name())
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if !includeClosed && (a.State == StateClosed || a.State == StateAbandoned) {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt.After(out[j].OpenedAt) })
	sort.Strings(damaged)
	return out, damaged, nil
}

// Record appends a finding to the engineer's working notes.
func (s *Store) Record(id, kind, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("assignment: a finding needs text")
	}
	switch kind {
	case "hypothesis", "found", "next", "note":
	default:
		return fmt.Errorf("assignment: finding kind %q must be hypothesis, found, next or note", kind)
	}
	_, err := s.update(id, func(a *Assignment) error {
		a.Findings = append(a.Findings, Finding{At: s.clock(), Kind: kind, Text: strings.TrimSpace(text)})
		return nil
	})
	return err
}

// SetState moves an assignment between states.
func (s *Store) SetState(id, state string) error {
	if !slices.Contains(states, state) {
		return fmt.Errorf("assignment: state %q must be one of %s", state, strings.Join(states, ", "))
	}
	_, err := s.update(id, func(a *Assignment) error {
		a.State = state
		return nil
	})
	return err
}

// Suspend sets work down for something more important, keeping the worktree and
// the branch.
//
// Both reasons are required. An interruption without what preempted it cannot
// be judged later, and one without where the work stood is abandonment with a
// better name — the resuming session inherits a branch and no idea what it was
// in the middle of.
func (s *Store) Suspend(id, forWhat, stands string) error {
	if strings.TrimSpace(forWhat) == "" {
		return errors.New("assignment: say what took priority. An interruption nobody can " +
			"justify later is indistinguishable from losing interest in the work")
	}
	if strings.TrimSpace(stands) == "" {
		return errors.New("assignment: say where the work stands. Setting it down without " +
			"that is abandonment — whoever resumes it inherits a branch and no idea what " +
			"it was in the middle of")
	}
	_, err := s.update(id, func(a *Assignment) error {
		switch a.State {
		case StateActive, StateBlocked:
		case StateSuspended:
			return fmt.Errorf("assignment: %s is already suspended", id)
		default:
			return fmt.Errorf("assignment: %s is %s — there is nothing in progress to set down", id, a.State)
		}
		a.Suspensions = append(a.Suspensions, Suspension{
			At: s.clock(), For: strings.TrimSpace(forWhat), Stands: strings.TrimSpace(stands),
		})
		a.State = StateSuspended
		return nil
	})
	return err
}

// Resume picks suspended work back up.
func (s *Store) Resume(id string) (*Assignment, error) {
	return s.update(id, func(a *Assignment) error {
		if a.State != StateSuspended {
			return fmt.Errorf("assignment: %s is %s, not suspended", id, a.State)
		}
		for i := range a.Suspensions {
			if a.Suspensions[i].Open() {
				a.Suspensions[i].ResumedAt = s.clock()
			}
		}
		a.State = StateActive
		return nil
	})
}

// Reopen takes handed-off or blocked work back into progress, re-cutting the
// worktree from the branch when a cleanup removed it.
//
// A handed-off lane is finished only provisionally: a review comes back, a
// device check fails, the owner asks for one more thing. Without this the only
// way back in was a new assignment with a new id, and the record of what had
// been established stayed behind on the old one.
func (s *Store) Reopen(id string) (*Assignment, error) {
	return s.update(id, func(a *Assignment) error {
		switch a.State {
		case StateHandedOff, StateBlocked, StateSuspended:
		case StateActive:
			return fmt.Errorf("assignment: %s is already active", id)
		default:
			return fmt.Errorf("assignment: %s is %s; a %s lane is not reopened, a new assignment is opened", id, a.State, a.State)
		}
		if a.Worktree != "" && a.Source != "" {
			if _, err := os.Stat(filepath.Join(a.Worktree, ".git")); err != nil {
				os.RemoveAll(a.Worktree)
				s.Git(a.Source, "worktree", "prune")
				if _, err := s.Git(a.Source, "worktree", "add", a.Worktree, a.Branch); err != nil {
					return fmt.Errorf("assignment: %s: re-cut the worktree from %s: %w", id, a.Branch, err)
				}
			}
		}
		a.State = StateActive
		return nil
	})
}

// Handoff records where the work stands.
//
// Required before closing. The stopping rule in the architecture says work ends
// when the claim is established and further iteration changes no decision — and
// the only way a later session can tell that happened is if someone wrote it
// down while they still knew.
// A handoff also travels: it is written to one machine's disk, and the peer
// deciding whether a draft is ready to merge is on the other one. Where the lane
// has a pull request, the same text goes there as a comment, which is the only
// surface both hosts read.
func (s *Store) Handoff(id, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("assignment: a handoff needs to say where the work stands")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a, err := s.update(id, func(a *Assignment) error {
		a.Handoff = strings.TrimSpace(text)
		if a.State == StateActive {
			a.State = StateHandedOff
		}
		// Recorded inside the write so the restate that follows it publishes
		// the claim on the pull request too, in the same act.
		if a.PullRequest == "" {
			a.PullRequest = s.findPullRequest(ctx, a)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.postHandoff(ctx, a)
	return nil
}

// findPullRequest asks the tracker which pull request the lane's branch has.
//
// Silence is the answer where there is none and where the tracker could not be
// asked alike: the handoff is already on the record either way, and refusing a
// handoff because GitHub was unreachable would destroy the one thing the lane
// knew. An open pull request wins over a closed one; a lane whose branch has
// several is telling you something this cannot resolve, so it takes none.
func (s *Store) findPullRequest(ctx context.Context, a *Assignment) string {
	if s.Tracker == nil || strings.TrimSpace(a.Branch) == "" {
		return ""
	}
	prs, err := s.Tracker.PullRequests(ctx, a.Repo, a.Branch)
	if err != nil {
		return ""
	}
	var open []claim.PullRequest
	for _, pr := range prs {
		if strings.EqualFold(pr.State, "OPEN") {
			open = append(open, pr)
		}
	}
	if len(open) != 1 {
		return ""
	}
	return fmt.Sprintf("PR #%d", open[0].Number)
}

// postHandoff puts the lane's handoff on its pull request.
//
// It opens by naming itself, the host and the state, because a reader arriving
// at a comment column needs to know in one line that this is the lane's own
// statement of where the work stands and which machine it stands on — that
// machine's record is not readable from here.
//
// Best effort by design. The handoff is recorded before this runs; a tracker
// that is down must not cost the session the text it just wrote.
func (s *Store) postHandoff(ctx context.Context, a *Assignment) {
	if s.Tracker == nil || a == nil || strings.TrimSpace(a.PullRequest) == "" {
		return
	}
	host := "this machine"
	if a.Claim != nil && a.Claim.Host != "" {
		host = a.Claim.Host
	}
	body := fmt.Sprintf("**Handoff — Mellions lane `%s` on `%s`, now %s.**\n\n"+
		"This is the lane's own statement of where the work stands, posted here because the "+
		"lane's record is on that machine and nothing else can read it. Read it before merging.\n\n"+
		"## Handoff\n\n%s\n", a.ID, host, a.State, a.Handoff)
	if err := s.Tracker.Comment(ctx, a.Repo, a.PullRequest, body); err != nil {
		fmt.Fprintf(os.Stderr, "assignment: %s handed off, but the handoff did not reach %s %s: %v\n"+
			"A peer on another host reads this change set as though nothing had been said about it.\n",
			a.ID, a.Repo, a.PullRequest, err)
	}
}

// Unsaved is what the worktree holds beyond what is committed and published.
//
// Modified, staged, untracked and conflicted material is destroyed with the
// worktree. Unpushed commits are not — the branch survives — and are reported
// so a reader knows the work was never published.
type Unsaved struct {
	Modified   int
	Staged     int
	Untracked  int
	Conflicted int
	Unpushed   int
}

// Any reports whether removing the worktree would destroy anything.
//
// Unpushed commits are counted but are not part of this. Removing a worktree
// keeps its branch, so a commit survives where an edit that was never committed
// does not — refusing on unpushed work would block the ordinary case, which is
// exactly how a safety check gets a flag that turns it off.
func (u Unsaved) Any() bool {
	return u.Modified+u.Staged+u.Untracked+u.Conflicted > 0
}

// String names what is there, in the order a person would want to hear it.
func (u Unsaved) String() string {
	var parts []string
	for _, p := range []struct {
		n    int
		what string
	}{
		{u.Conflicted, "conflicted file"}, {u.Staged, "staged change"},
		{u.Modified, "modified file"}, {u.Untracked, "untracked file"},
		{u.Unpushed, "unpushed commit"},
	} {
		if p.n == 0 {
			continue
		}
		plural := ""
		if p.n != 1 {
			plural = "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s%s", p.n, p.what, plural))
	}
	return strings.Join(parts, ", ")
}

// Unsaved reports what removing this assignment's worktree would destroy.
//
// Untracked files count. The evidence a session generates while investigating —
// a reproduction script, a captured payload, a failing fixture — is untracked
// by definition, and it is the material most expensive to reconstruct.
func (s *Store) Unsaved(a *Assignment) (Unsaved, error) {
	var u Unsaved
	if a.Worktree == "" {
		return u, nil
	}
	// A worktree that is already gone — pruned, wiped, cleaned up by hand —
	// holds nothing to destroy. Refusing to close over it left every lane
	// whose tree had been removed externally unclosable forever.
	if _, err := os.Stat(a.Worktree); err != nil {
		return u, nil
	}
	// Porcelain v1 is stable and says staged, unstaged, untracked and
	// unmerged in one pass. --ignored is deliberately absent: ignored build
	// output is not work.
	out, err := s.Git(a.Worktree, "status", "--porcelain")
	if err != nil {
		return u, fmt.Errorf("assignment: %s: read worktree state: %w", a.ID, err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		switch {
		case x == '?' && y == '?':
			u.Untracked++
		case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
			u.Conflicted++
		default:
			if x != ' ' {
				u.Staged++
			}
			if y != ' ' {
				u.Modified++
			}
		}
	}
	// Commits on the lane that no remote-tracking ref in this checkout holds.
	// Reported rather than refused: the branch outlives the worktree, so these
	// are recoverable and a reader still wants to know they were never
	// published.
	//
	// `--not --remotes` is every remote-tracking ref, not the one @{upstream}
	// names. A lane pushed under a different name, or with no upstream
	// configured, is published and must not be counted; measuring one ref
	// answered a question about all of them, and the fallback that ran when
	// there was no upstream counted the lane against its local base, which
	// consults no remote at all.
	//
	// What this measures is the local remote-tracking refs, not the remotes.
	// A push nobody has fetched reads as unpushed; a remote branch deleted
	// upstream whose local ref survives reads as pushed. The second direction
	// reassures wrongly, so `fetch --prune` is what keeps it honest — this is
	// not a check that errs safe in both directions.
	//
	// Bounded below by the lane's base so the count is of the lane. Without a
	// base there is nothing to subtract but `--remotes`, and a checkout that
	// holds no remote-tracking ref then reports the repository's whole
	// history — so that combination reports nothing rather than everything.
	if a.Base != "" {
		if out, err := s.Git(a.Worktree, "rev-list", "--count", a.Base+"..HEAD", "--not", "--remotes"); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &u.Unpushed)
			return u, nil
		}
		// The recorded base no longer resolves — rewritten, or pruned by gc.
	}
	// No base to bound the range with, so the only thing subtracting anything
	// is `--remotes`. Where the checkout holds no remote-tracking ref at all
	// that subtracts nothing and the count becomes the repository's whole
	// history, which is a louder wrong answer than the silence below.
	if out, err := s.Git(a.Worktree, "for-each-ref", "--count=1", "refs/remotes"); err != nil ||
		strings.TrimSpace(string(out)) == "" {
		return u, nil
	}
	if out, err := s.Git(a.Worktree, "rev-list", "--count", "HEAD", "--not", "--remotes"); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &u.Unpushed)
	}
	return u, nil
}

// Close ends an assignment and removes its worktree.
//
// Refused without a handoff, and refused while anything exists only in the
// worktree. A worktree removed with the reasoning still in a dead session's
// context is how expensive uncommitted state becomes fourteen files and no
// explanation; a worktree removed with the files still in it is worse, because
// completion is what it reports while it does it.
//
// There is no force here. Discarding work is a different act with a different
// name — see Abandon — and the reason to separate them is that one of the two
// destroys something and the record has to say which happened.
func (s *Store) Close(id string) error { return s.close(id, "") }

// close is Close with a note written in the same update, so a lane ended by
// something other than the session that worked it — the sweep — says so on
// the record itself, not in a second write a crash could lose.
func (s *Store) close(id, note string) error {
	_, err := s.update(id, func(a *Assignment) error {
		if a.Handoff == "" {
			return fmt.Errorf("assignment: %s has no handoff; write where the work stands "+
				"before closing", id)
		}
		u, err := s.Unsaved(a)
		if err != nil {
			return err
		}
		if u.Any() {
			return fmt.Errorf("assignment: %s still holds %s that exist only in %s.\n\n"+
				"Closing removes that worktree, and none of that is recoverable afterwards. "+
				"Commit it, or say what is being thrown away:\n\n"+
				"  mellions assign abandon %s -discarding \"...\"",
				id, u, a.Worktree, id)
		}
		if err := s.release(a, StateClosed); err != nil {
			return err
		}
		if note != "" {
			a.Findings = append(a.Findings, Finding{At: s.clock(), Kind: "note", Text: note})
		}
		return nil
	})
	return err
}

// Abandon ends an assignment and destroys what only its worktree holds.
//
// The description is required and is not a formality: it is the only record
// that will exist of material nothing else can reproduce. Abandoning is
// recorded as abandonment rather than as completion, because a reader deciding
// whether the work was finished cannot tell the two apart afterwards.
func (s *Store) Abandon(id, discarding string, unreviewed []string) error {
	if strings.TrimSpace(discarding) == "" {
		return errors.New("assignment: say what is being discarded. Removing a worktree is the " +
			"one operation here nothing can undo, and an unexplained one leaves a reader " +
			"unable to tell abandoned work from finished work")
	}
	var branchErr error
	_, err := s.update(id, func(a *Assignment) error {
		held := "unreadable worktree — what it held could not be established"
		// A worktree git can no longer read — its registration pruned, its
		// gitdir gone — is exactly the lane that most needs abandoning, and
		// refusing over the unreadable state made it unabandonable as well as
		// uncloseable. What it held is recorded as unknown rather than guessed.
		if u, err := s.Unsaved(a); err == nil {
			held = u.String()
		}
		d := &Discarded{At: s.clock(), What: strings.TrimSpace(discarding), Held: held,
			Unreviewed: unreviewed}
		if tip, err := s.Git(a.Source, "rev-parse", a.Branch); err == nil {
			d.Tip = strings.TrimSpace(string(tip))
		}
		if n, err := s.Git(a.Source, "rev-list", "--count", a.Base+".."+a.Branch); err == nil {
			d.Commits, _ = strconv.Atoi(strings.TrimSpace(string(n)))
		}
		a.Discarded = d
		if err := s.release(a, StateAbandoned); err != nil {
			// Abandoning destroys on purpose. A tree git will not detach is
			// removed directly, and git's stale registration pruned.
			if a.Worktree != "" {
				os.RemoveAll(a.Worktree)
				s.Git(a.Source, "worktree", "prune")
			}
			a.State = StateAbandoned
			a.ClosedAt = s.clock()
		}
		// The record settles as abandoned whatever happens to the branch: a
		// branch that would not delete is worth reporting, and it used to
		// cancel the state change, which left a lane the reader had abandoned
		// still listed as work in flight.
		branchErr = s.discardBranch(a, d)
		return nil
	})
	if err != nil {
		return err
	}
	return branchErr
}

// discardBranch deletes the branch the abandoned work was on.
//
// Closing keeps a branch: the work is finished and may still be merged, and
// removing a worktree deliberately leaves it. Abandoning is the other verb and
// meant the same thing, which made it the way past every check closing performs
// — a change that required an independent view could not be closed and could be
// abandoned, and the commit stayed on a branch that merged like any other.
//
// The tip is recorded before the branch goes, and git keeps it reachable in the
// reflog, so this is recoverable by somebody who reads the record. That is a
// narrower promise than "nothing is lost" and it is the true one.
func (s *Store) discardBranch(a *Assignment, d *Discarded) error {
	if a.Source == "" || a.Branch == "" || a.Adopted {
		return nil
	}
	// A branch that is already gone has nothing to delete, and is not a
	// failure to report.
	if _, err := s.Git(a.Source, "rev-parse", "--verify", "--quiet", "refs/heads/"+a.Branch); err != nil {
		return nil
	}
	if _, err := s.Git(a.Source, "branch", "-D", a.Branch); err != nil {
		// Not fatal: the worktree is already gone and the assignment is
		// abandoned. A branch that will not delete is worth reporting.
		return fmt.Errorf("assignment: %s abandoned, but branch %s could not be deleted: %w\n\n"+
			"It still carries %d commit(s) and still merges like any other branch. "+
			"Delete it, or say why it is being kept", a.ID, a.Branch, err, d.Commits)
	}
	d.Branch = a.Branch
	return nil
}

// release removes the worktree and settles the assignment into state.
func (s *Store) release(a *Assignment, state string) error {
	// Before the worktree, because this is the half other machines can see: a
	// lane that ends holding a claim leaves the issue looking taken until the
	// claim goes stale.
	s.releaseClaim(a)
	if a.Adopted {
		a.State = state
		a.ClosedAt = s.clock()
		return nil
	}
	if a.Worktree != "" && a.Source != "" {
		if _, err := os.Stat(a.Worktree); err != nil {
			// Already gone; only git's registration of it may remain.
			s.Git(a.Source, "worktree", "prune")
			a.State = state
			a.ClosedAt = s.clock()
			return nil
		}
		if _, err := s.Git(a.Source, "worktree", "remove", "--force", a.Worktree); err != nil {
			// The branch and its commits are the durable part and survive; a
			// worktree that will not detach is worth reporting, not fatal.
			return fmt.Errorf("assignment: %s: remove worktree: %w (the branch %s is kept)", a.ID, err, a.Branch)
		}
	}
	a.State = state
	a.ClosedAt = s.clock()
	return nil
}

// Text renders an assignment as the context a resuming session reads.
func (a Assignment) Text(now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Assignment %s — %s\n\n", a.ID, a.State)
	fmt.Fprintf(&b, "%s\n\n", a.Objective)
	if a.Issue != "" {
		fmt.Fprintf(&b, "- issue: %s %s\n", a.Repo, a.Issue)
		switch {
		case a.Claim == nil:
			fmt.Fprintf(&b, "- claim: none published — no other machine can see this lane\n")
		case a.Claim.Unpublished != "":
			fmt.Fprintf(&b, "- claim: LOCAL ONLY — %s. A session on another machine can take this same work\n",
				a.Claim.Unpublished)
		default:
			fmt.Fprintf(&b, "- claim: %s on %s, restated %s\n",
				claim.Label, a.Claim.Host, a.Claim.At.Format(time.RFC3339))
		}
	}
	if a.PullRequest != "" {
		fmt.Fprintf(&b, "- change set: %s %s (claimed; a peer reads the claim before merging it)\n",
			a.Repo, a.PullRequest)
	}
	if a.Adopted {
		fmt.Fprintf(&b, "- worktree: %s (adopted, not cut: it and its branch are left in place when this lane ends)\n- branch: %s (base %s)\n", a.Worktree, a.Branch, short(a.Base))
	} else {
		fmt.Fprintf(&b, "- worktree: %s\n- branch: %s (cut from %s)\n", a.Worktree, a.Branch, short(a.Base))
		if a.BasePin != "" {
			fmt.Fprintf(&b, "- base: %s\n", a.BasePin)
		}
	}
	fmt.Fprintf(&b, "- open: %s\n", humanDuration(a.Age(now)))
	if a.Register != "" {
		fmt.Fprintf(&b, "- register: %s records its work in %s — read the rows there, the open remediations included, before reporting anything found here as new\n", a.Repo, a.Register)
	}
	// A method listed in a catalog at minute zero is not one a session reaches
	// for at minute twenty: across a hundred shift streams the remediation and
	// territory Skills were loaded in one, while the two Skills named by the
	// text of their own moment were loaded in fifty. The lane is the moment, so
	// its record names what is here.
	//
	// It names rather than commands. An unconditional "load this before the
	// first edit" is attached to a lane whose size it cannot see; against a
	// change small enough to hold in one reading it is out of proportion, and
	// the session that overrides it once has learned the line is not about this
	// work — which costs the channel every later lane where it was right. An
	// adopted tree is the exception, because that is a fact about this lane and
	// not an estimate of its size.
	if a.State == StateActive {
		if a.Adopted {
			fmt.Fprintf(&b, "- method: this tree was not cut for you — `Skill(skill: \"mellions:mellions-territory\")` before touching what you did not write")
		} else {
			fmt.Fprintf(&b, "- method: `mellions skills <what you are doing>` says what this installation carries; `mellions:mellions-issue-remediation` where the work is larger than an edit you can hold in one reading, `mellions:mellions-territory` where another lane may hold what you are about to change")
		}
		b.WriteString("\n")
	}
	if a.Because != "" {
		fmt.Fprintf(&b, "\n**Chosen because** %s\n", a.Because)
	}
	if a.NotChosen != "" {
		fmt.Fprintf(&b, "\n**Passed over** %s\n", a.NotChosen)
	}
	if a.Overdue(now) {
		fmt.Fprintf(&b, "\n**BUDGET ELAPSED** (%s). Write where this stands: what holds, what is "+
			"unresolved, what finishing would cost. Do not continue silently.\n", humanDuration(a.Budget.Wall))
	}
	if len(a.Findings) > 0 {
		last := a.Findings[len(a.Findings)-1].At
		fmt.Fprintf(&b, "\n## Working notes — as they stood %s ago\n\n"+
			"Every line below was true when it was written. Whether it is still true is a "+
			"separate question, and the ones that describe the world rather than the "+
			"reasoning have to be re-established before they are acted on.\n\n",
			humanDuration(now.Sub(last)))
		for _, f := range a.Findings {
			fmt.Fprintf(&b, "- **%s** (%s) %s\n", f.Kind, f.At.UTC().Format("01-02 15:04"), f.Text)
		}
	}
	if len(a.Suspensions) > 0 {
		b.WriteString("\n## Set down\n\n")
		for _, sp := range a.Suspensions {
			state := "resumed " + sp.ResumedAt.UTC().Format("01-02 15:04")
			if sp.Open() {
				state = "still set down"
			}
			fmt.Fprintf(&b, "- %s — for %s (%s)\n  stood at: %s\n",
				sp.At.UTC().Format("01-02 15:04"), sp.For, state, sp.Stands)
		}
		if len(a.Suspensions) > 2 {
			fmt.Fprintf(&b, "\nInterrupted %d times. Work that keeps being the second most "+
				"important thing is usually not worth resuming again — finish it or close it.\n",
				len(a.Suspensions))
		}
	}
	if a.Handoff != "" {
		fmt.Fprintf(&b, "\n## Handoff\n\n%s\n", a.Handoff)
	}
	b.WriteString("\nThese are working notes, not the record of the work. Anything another " +
		"person needs belongs on the issue or the pull request.\n")
	if s, ok := a.Latest(); ok && s.Resume() != "" {
		fmt.Fprintf(&b, "\nLast worked in %s session %s. If it still resumes, resume it — it "+
			"holds the reasoning these notes are a summary of:\n\n  %s\n",
			s.Runtime, short(s.ID), s.Resume())
	}
	fmt.Fprintf(&b, "\nWhat the world says now is not in this file. `mellions continue` "+
		"establishes it.\n")
	return b.String()
}

func short(ref string) string {
	if len(ref) > 12 {
		return ref[:12]
	}
	return ref
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// Session is one runtime session that worked this assignment.
//
// Recorded so recovery can try the runtime's own resume before reconstructing
// anything. A session that still resumes carries reasoning no record
// reconstructs — the discarded hypotheses, the file read three times, why the
// obvious fix was wrong. Rebuilding from the assignment when `claude --resume`
// would have worked throws away the better evidence and calls it recovery.
type Session struct {
	Runtime string    `json:"runtime"`
	ID      string    `json:"id"`
	First   time.Time `json:"first"`
	Last    time.Time `json:"last"`
}

// Resume is the command that reopens this session.
//
// Empty where the runtime is not one whose resume is known, which is honest:
// naming a command that does not exist is worse than saying nothing, because
// the reader spends the attempt before finding out.
func (s Session) Resume() string {
	switch s.Runtime {
	case "claude":
		return "claude --resume " + s.ID
	case "codex":
		return "codex resume " + s.ID
	}
	return ""
}

// Here reads the runtime sessions this process is running inside.
//
// Both runtimes export their session id to the processes they start, so this
// needs no hook, no plumbing and no cooperation from the model.
//
// It returns every runtime that claims the process rather than one, because a
// session started from inside another session inherits the outer runtime's
// variable alongside its own, and nothing in the environment says which is
// which. Picking one would name a resume command that reopens a different
// conversation — worse than naming two and letting the reader try both.
//
// Outside a session — a terminal, a timer, CI — there is nothing to record, and
// that is a supported way to run rather than a fault.
func Here() []Session {
	var out []Session
	for _, r := range []struct{ runtime, env string }{
		{"claude", "CLAUDE_CODE_SESSION_ID"},
		{"codex", "CODEX_SESSION_ID"},
	} {
		if id := strings.TrimSpace(os.Getenv(r.env)); id != "" {
			out = append(out, Session{Runtime: r.runtime, ID: id})
		}
	}
	return out
}

// SessionsByRecency is every session that touched this work, newest first.
//
// All of them rather than the last: recovery reaches for a resume and the newest
// handle is the one most likely to fail — a transcript swept on its retention
// sweep, a machine that is not this one, a runtime that is not the one running
// now. The one before it is then the best thing left, and a caller given only
// the newest has a dead command and nothing else to try.
func (a Assignment) SessionsByRecency() []Session {
	out := slices.Clone(a.Sessions)
	slices.SortStableFunc(out, func(x, y Session) int {
		switch {
		case x.Last.After(y.Last):
			return -1
		case y.Last.After(x.Last):
			return 1
		}
		return 0
	})
	return out
}

// Latest is the most recently active session, and whether there was one.
func (a Assignment) Latest() (Session, bool) {
	if len(a.Sessions) == 0 {
		return Session{}, false
	}
	best := a.Sessions[0]
	for _, s := range a.Sessions[1:] {
		if s.Last.After(best.Last) {
			best = s
		}
	}
	return best, true
}

// stamp records the runtime session doing the writing.
//
// Called from every save rather than from the commands, because the point is to
// hold when a session dies without finishing anything — and a session that dies
// mid-thought is exactly the one that never reached the call it was supposed to
// make.
func (a *Assignment) stamp(now time.Time) {
	for _, here := range Here() {
		found := false
		for i := range a.Sessions {
			if a.Sessions[i].Runtime == here.Runtime && a.Sessions[i].ID == here.ID {
				a.Sessions[i].Last = now
				found = true
				break
			}
		}
		if !found {
			here.First, here.Last = now, now
			a.Sessions = append(a.Sessions, here)
		}
	}
}

// Discarded is the record of work destroyed rather than finished.
type Discarded struct {
	At time.Time `json:"at"`
	// What the engineer said was being thrown away.
	What string `json:"what"`
	// Held is what the worktree actually contained when it went, so the claim
	// and the state can be compared rather than taken on trust.
	Held string `json:"held,omitempty"`
	// Branch and Tip are what the branch was when it was deleted. Recorded
	// because deleting it is the point — abandoned work that is still mergeable
	// was not abandoned — and because git's reflog will hold the tip for a
	// while, so a mistake is recoverable by somebody who knows the hash.
	Branch string `json:"branch,omitempty"`
	Tip    string `json:"tip,omitempty"`
	// Commits is how many the branch carried past its base.
	Commits int `json:"commits,omitempty"`
	// Unreviewed names the review triggers that were outstanding when the work
	// went.
	//
	// Deleting the branch removes the hazard: unreviewed work that is still
	// mergeable was not abandoned. It does not record that a review was owed,
	// and the difference matters to whoever reads this afterwards — "abandoned"
	// and "abandoned rather than reviewed" are different accounts of the same
	// state, and only one of them can be argued with.
	Unreviewed []string `json:"unreviewed,omitempty"`
}

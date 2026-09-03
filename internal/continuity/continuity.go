// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package continuity gives a session that did not attend the one before it
// enough trustworthy state to work out what it is in the middle of.
//
// It stores nothing of its own and concludes nothing. Two things it will not do
// are worth stating because both are tempting and both are wrong.
//
// It does not decide whether continuing is safe. That question is answered by
// comparing what a record claims against what the world says, discarding
// hypotheses that no longer hold, and judging what a difference means — which
// is engineering reasoning, and a frontier model does it better than any
// sequence of conditions written here. Every attempt to encode it produces a
// state machine that is confidently wrong about the case nobody anticipated.
//
// It does not narrate. What it emits is two columns and a boundary: what the
// record says and when it was written, what was read from the world and when,
// and what the configuration permits right now. The comparison belongs to the
// reader.
//
// What it does enforce, because none of it survives being reasoned about:
//
// The record and the world are never printed in the same voice. "PR #421 is
// open" was true when it was written and is a claim now.
//
// Unknown is not absent. A tracker that could not be reached and a branch with
// no pull request are the same silence here and opposite instructions outside.
//
// What may be done is whatever the partnership says at the moment of asking,
// never what an earlier session was told; nothing recovered may soften that.
package continuity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// Git runs git in a directory and returns its output.
type Git func(dir string, args ...string) ([]byte, error)

// Tracker answers what the configured tracker says about work in flight.
//
// Both functions may be nil and both return known=false rather than an empty
// answer when they could not ask. That distinction is why this is a pair of
// functions and not two strings.
type Tracker struct {
	PullRequest func(ctx context.Context, repo, branch string) (string, bool)
	Issue       func(ctx context.Context, repo, ref string) (string, bool)
}

// Fact is one thing read from the world, with where it came from.
//
// The provenance is not decoration. A branch name read from the worktree and a
// branch name read from an assignment record are different kinds of claim, and
// a reader that cannot tell them apart cannot reconcile them.
type Fact struct {
	Name  string
	Value string
	From  string
}

// Observed is what was read from the world about one assignment, and when.
type Observed struct {
	At    time.Time
	Facts []Fact
	// Unestablished is what could not be answered, each line saying why it is
	// unknown rather than absent.
	Unestablished []string
}

// Look reads what the world currently says about one assignment.
//
// It compares nothing. Every failure produces an unestablished line rather than
// an error: a reconciliation that stops at the first unreachable thing reports
// nothing about the rest, and the rest is usually where the answer was.
func Look(ctx context.Context, a *assignment.Assignment, git Git, tr Tracker) Observed {
	o := Observed{At: time.Now().UTC()}
	add := func(name, value, from string) {
		o.Facts = append(o.Facts, Fact{Name: name, Value: value, From: from})
	}
	miss := func(why string) { o.Unestablished = append(o.Unestablished, why) }

	present := false
	switch {
	case a.Worktree == "":
		miss("the record names no worktree, so nothing local could be read")
	case exists(filepath.Join(a.Worktree, ".git")):
		present = true
		add("worktree", "present at "+a.Worktree, "the filesystem")
	case exists(a.Worktree):
		add("worktree", a.Worktree+" exists but holds no .git", "the filesystem")
	default:
		add("worktree", "absent — "+a.Worktree+" does not exist", "the filesystem")
	}

	if present && git != nil {
		where := "git in " + a.Worktree
		run := func(args ...string) (string, bool) {
			out, err := git(a.Worktree, args...)
			if err != nil {
				return "", false
			}
			return strings.TrimSpace(string(out)), true
		}
		if v, ok := run("rev-parse", "--abbrev-ref", "HEAD"); ok {
			add("branch", v, where)
		} else {
			miss("the worktree's current branch could not be read")
		}
		if v, ok := run("rev-parse", "HEAD"); ok {
			add("head", v, where)
		}
		if v, ok := run("status", "--porcelain"); ok {
			add("uncommitted", strconv.Itoa(len(nonEmptyLines(v)))+" path(s)", where)
		} else {
			miss("the worktree's uncommitted state could not be read")
		}
		if a.Base != "" {
			if v, ok := run("rev-list", "--count", a.Base+"..HEAD"); ok {
				add("commits since the base", v, where)
			}
		}
		if v, ok := unpublished(run, a.Base, "HEAD"); ok {
			add("unpublished", v, where)
		}
	}

	// The worktree is gone. That is a fact about the worktree, and it used to be
	// the end of the reading — one line saying "absent" and nothing else, for
	// the most ordinary disruption there is: a wiped machine, a cleared /tmp, a
	// restarted container, somebody's prune. Every question that matters is
	// still answerable from the repository the record names, and a session told
	// only that the worktree is missing reads total loss where nothing was lost.
	if !present && git != nil && a.Source != "" && a.Branch != "" {
		where := "git in " + a.Source
		run := func(args ...string) (string, bool) {
			out, err := git(a.Source, args...)
			if err != nil {
				return "", false
			}
			return strings.TrimSpace(string(out)), true
		}
		tip, alive := run("rev-parse", "--verify", "--quiet", "refs/heads/"+a.Branch)
		switch {
		case alive && tip != "":
			add("branch "+a.Branch, "still in "+a.Source+", tip "+shortSHA(tip), where)
			if a.Base != "" {
				if v, ok := run("rev-list", "--count", a.Base+".."+a.Branch); ok {
					add("commits since the base", v+" — recoverable with `git -C "+
						a.Source+" worktree add <dir> "+a.Branch+"`", where)
				}
			}
			if v, ok := unpublished(run, a.Base, a.Branch); ok {
				add("unpublished", v, where)
			}
		case alive:
			add("branch "+a.Branch, "gone from "+a.Source, where)
			miss("the branch is not in the source repository either; whether its commits are " +
				"still reachable is not established here — `git -C " + a.Source +
				" reflog` and `git fsck --lost-found` are where that is answered")
		default:
			miss("the source repository " + a.Source + " could not be read, so whether the " +
				"branch survived the worktree is unknown rather than answered")
		}
		if v, ok := run("worktree", "list", "--porcelain"); ok && strings.Contains(v, a.Worktree) {
			add("worktree registration", "git still records "+a.Worktree+
				", prunable — `git -C "+a.Source+" worktree prune` clears it", where)
		}
	}

	branch := a.Branch
	for _, f := range o.Facts {
		if f.Name == "branch" {
			branch = f.Value
		}
	}
	switch {
	case tr.PullRequest == nil:
		miss("no tracker was reachable, so whether a pull request exists for " + branch +
			" is unknown rather than no")
	case a.Repo == "" || branch == "":
		miss("no repository or branch to ask the tracker about")
	default:
		if v, known := tr.PullRequest(ctx, a.Repo, branch); known {
			add("pull request", v, "the tracker")
		} else {
			miss("the tracker could not say whether a pull request exists for " + branch)
		}
	}
	if a.Issue != "" {
		if tr.Issue == nil {
			miss("no tracker was reachable, so the state of " + a.Issue + " is unknown")
		} else if v, known := tr.Issue(ctx, a.Repo, a.Issue); known {
			add(a.Issue, v, "the tracker")
		} else {
			miss("the tracker could not say what state " + a.Issue + " is in")
		}
	}
	return o
}

// unpublished counts the commits on ref that no remote-tracking ref in this
// checkout holds, and says which it found. Empty and false where the question
// cannot be answered, so the caller records nothing rather than a guess.
//
// It reads remote-tracking refs rather than the upstream configuration because
// configuring an upstream and publishing a branch are different acts:
// `git push origin HEAD:refs/heads/<name>` without `-u` publishes and
// configures nothing, which is the ordinary shape of a lane branch here. The
// absence of an upstream is a fact about this checkout's config and says
// nothing about any remote — and a reader deciding whether a dead lane's work
// survived is the last reader who should be handed a claim of total loss
// derived from it.
//
// What this measures is the local remote-tracking refs, not the remotes. A push
// nobody has fetched reads as unpublished; a remote branch deleted upstream
// whose local ref survives reads as published. The value names the boundary so
// the reader can weigh it: this is not a reading that errs safe both ways.
//
// Bounded below by base so the count is of the lane. With no base to subtract
// and no remote-tracking ref in the checkout, `--not --remotes` subtracts
// nothing and the count becomes the repository's whole history — so that
// combination answers nothing rather than everything.
//
// The sentence names the range it was taken over, and never a wider one. The
// bounded count is of `base..ref`, and base itself can hold commits no remote
// holds: `mellions assign open` falls back to a local HEAD as the base whenever
// the source checkout tracks no remote branch. A count of zero over that range
// says every commit the lane added is published; said about ref it would tell a
// session its work is safe when it is not, which is this defect's own mirror
// image in the direction that loses work rather than duplicating it.
func unpublished(run func(...string) (string, bool), base, ref string) (string, bool) {
	say := func(v, scope string) (string, bool) {
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", false
		}
		if n == 0 {
			return "none in " + scope + " — every commit that range holds is on a " +
				"remote-tracking ref in this checkout, which a missing fetch can make stale", true
		}
		return strconv.Itoa(n) + " commit(s) in " + scope + " that no remote-tracking ref " +
			"in this checkout holds, which a missing fetch can make stale", true
	}
	if base != "" {
		if v, ok := run("rev-list", "--count", base+".."+ref, "--not", "--remotes"); ok {
			return say(v, base+".."+ref)
		}
		// The recorded base no longer resolves — rewritten, or pruned by gc.
	}
	if v, ok := run("for-each-ref", "--count=1", "refs/remotes"); !ok || v == "" {
		return "", false
	}
	if v, ok := run("rev-list", "--count", ref, "--not", "--remotes"); ok {
		return say(v, ref)
	}
	return "", false
}

// Work pairs what the record claims with what the world said when asked.
//
// Deliberately a pair rather than a merge. Merging them requires deciding which
// wins, and which wins depends on what the difference is — a branch that moved,
// a pull request somebody merged, and a worktree a cleanup removed all read the
// same to a comparison and mean entirely different things.
type Work struct {
	Recorded *assignment.Assignment
	Observed Observed
}

// Standing is the whole slate handed to the reader.
type Standing struct {
	At time.Time
	// In is every runtime session claiming this process. More than one means a
	// session was started from inside another and both handles are live.
	In   []assignment.Session
	Work []Work
	// Unreadable names records that exist and did not survive their write. They
	// are neither recorded nor observed: what they held is gone, and saying so
	// is the only honest thing available. Absent from this list is not the same
	// as never having existed.
	Unreadable []string
	Notes      []string
}

// Assemble collects the slate.
func Assemble(ctx context.Context, as *assignment.Store, git Git, tr Tracker) (Standing, error) {
	s := Standing{At: time.Now().UTC()}
	s.In = assignment.Here()

	// Damaged records are named rather than fatal. This is the command for a
	// session that did not attend the one before it, so a record truncated by
	// whatever ended that session is the ordinary case here — and refusing to
	// assemble a slate at all would fail precisely when the slate is needed.
	open, damaged, err := as.ListWithDamage(false)
	if err != nil {
		return s, err
	}
	for _, id := range damaged {
		s.Unreadable = append(s.Unreadable, id)
	}
	for _, a := range open {
		s.Work = append(s.Work, Work{Recorded: a, Observed: Look(ctx, a, git, tr)})
	}
	return s, nil
}

// Text renders the slate.
//
// It ends without a conclusion on purpose. The reader has the record, the
// world, the boundary and the gaps; what those mean for this piece of work is
// the judgement it was reinstated to make, and a verdict printed here would be
// answered before it was asked.
func (s Standing) Text() string {
	var b strings.Builder
	b.WriteString("# Continuity slate\n\n")
	b.WriteString("Who you are, who you work with and what you are responsible for reached you " +
		"at session start from files no session writes. They are current, not recovered.\n\n")
	b.WriteString("Everything below is either **recorded** — what an earlier session wrote, " +
		"true when written and a claim now — or **observed**, read from the world at the time " +
		"stamped on it. They are never merged here. Deciding what a difference between them " +
		"means is the work.\n\n")

	if len(s.In) > 0 {
		for _, in := range s.In {
			fmt.Fprintf(&b, "Running inside %s, session `%s`.\n", in.Runtime, in.ID)
		}
		if len(s.In) > 1 {
			b.WriteString("Two runtimes claim this process — one session was started " +
				"from inside the other, and the environment does not say which.\n")
		}
	} else {
		b.WriteString("Not running inside a coding-agent session — a terminal, a timer or CI. " +
			"Supported, and it means nothing here can be resumed natively.\n")
	}

	if len(s.Unreadable) > 0 {
		b.WriteString("\n## Records that did not survive\n\n")
		for _, id := range s.Unreadable {
			fmt.Fprintf(&b, "- **%s** — the record exists and cannot be read.\n", id)
		}
		b.WriteString("\nWhat those held is gone. Their branches and commits are the durable part " +
			"and are untouched: read those. An absent assignment is not the same as a finished " +
			"one, and neither is an unreadable one.\n")
	}

	b.WriteString("\n## Work open on the record\n\n")
	if len(s.Work) == 0 {
		b.WriteString("None. An empty list is the state after finishing, not after forgetting — " +
			"nothing here was lost. `mellions survey` is where the next work comes from.\n")
	}
	for _, w := range s.Work {
		b.WriteString(workText(w, s.At))
	}

	for _, n := range s.Notes {
		fmt.Fprintf(&b, "\n%s\n", n)
	}

	b.WriteString("\n---\n\n" +
		"The slate stops here. What survived, what has to be re-established, which " +
		"hypotheses no longer hold and what the next safe action is are yours to work " +
		"out from this and from whatever else you need to go and look at. What you may " +
		"do is what the partnership says now, not what an earlier session was told. " +
		"The `mellions-continuity` Skill carries the method.\n")
	return b.String()
}

func workText(w Work, now time.Time) string {
	a, o := w.Recorded, w.Observed
	var b strings.Builder
	fmt.Fprintf(&b, "\n### %s — %s on the record, opened %s ago\n\n%s\n", a.ID, a.State,
		humanDuration(now.Sub(a.OpenedAt)), firstLine(a.Objective))

	// Every session, newest first, not only the last one. A transcript that has
	// been swept, a machine that is not this one and a runtime that is not the
	// one running now all make the newest handle fail — and when it does, the
	// one before it is the next best thing in the room. Printing only the
	// latest leaves a reader with a dead command and no alternative.
	if by := a.SessionsByRecency(); len(by) > 0 {
		b.WriteString("\nWorked in, newest first. Each holds what was thought and discarded, " +
			"which no record keeps; the newest that still opens is worth more than " +
			"rebuilding from below.\n\n")
		for _, s := range by {
			cmd := s.Resume()
			if cmd == "" {
				cmd = "(no known resume for " + s.Runtime + ")"
			}
			fmt.Fprintf(&b, "- %s ago — `%s`\n", humanDuration(now.Sub(s.Last)), cmd)
		}
	}

	fmt.Fprintf(&b, "\n**Recorded** — written by an earlier session:\n\n")
	fmt.Fprintf(&b, "- branch `%s`, cut from %s\n- worktree %s\n", a.Branch, shortRef(a.Base), a.Worktree)
	if a.Because != "" {
		fmt.Fprintf(&b, "- chosen because %s\n", firstLine(a.Because))
	}
	if n := len(a.Suspensions); n > 0 && a.Suspensions[n-1].Open() {
		last := a.Suspensions[n-1]
		fmt.Fprintf(&b, "- set down %s ago for %s; stood at: %s\n",
			humanDuration(now.Sub(last.At)), last.For, last.Stands)
	}
	for _, f := range a.Findings {
		fmt.Fprintf(&b, "- `%s` (%s ago) %s\n", f.Kind, humanDuration(now.Sub(f.At)), firstLine(f.Text))
	}
	if a.Handoff != "" {
		fmt.Fprintf(&b, "- `handoff` %s\n", firstLine(a.Handoff))
	}

	fmt.Fprintf(&b, "\n**Observed** — read %s:\n\n", o.At.Format("15:04:05 UTC"))
	if len(o.Facts) == 0 {
		b.WriteString("- nothing could be read\n")
	}
	for _, f := range o.Facts {
		fmt.Fprintf(&b, "- %s: %s _(%s)_\n", f.Name, f.Value, f.From)
	}

	if len(o.Unestablished) > 0 {
		b.WriteString("\n**Unestablished** — unknown, not absent:\n\n")
		for _, u := range o.Unestablished {
			fmt.Fprintf(&b, "- %s\n", u)
		}
	}
	return b.String()
}

// shortSHA is the readable form of a commit, for a line a person scans.
func shortSHA(v string) string {
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func shortRef(r string) string {
	if len(r) > 8 {
		return r[:8]
	}
	return r
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
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

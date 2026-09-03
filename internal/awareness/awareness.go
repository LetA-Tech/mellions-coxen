// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package awareness decides what a working session should be told right now.
//
// It answers one question — what is true about this session's situation that
// it cannot infer and would act on — as a pure function of an observation, so
// what it decides to say is testable without a filesystem or a runtime.
//
// What arrives without being asked for gets used; what has to be reached for
// does not. The risk is volume: a hook that speaks on every turn is a hook
// somebody turns off. So a note names something actionable, and Said keeps it
// from being repeated once it has been delivered.
package awareness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Note is one thing worth telling a session, and what it can do about it.
type Note struct {
	// Because is the observation, in one line.
	Because string
	// Do is what to do about it, ready to run or to act on.
	Do string
	// Why is the consequence of doing nothing, where it is not obvious.
	Why string
	// Ident is what the note is about, where the sentence alone does not say.
	// A document that changes twice produces the same words about two different
	// versions, and the second one must not be swallowed as already said.
	Ident string
}

// Key identifies the fact a note is about, so the same fact is not repeated.
func (n Note) Key() string {
	what := n.Ident
	if what == "" {
		what = n.Because
	}
	sum := sha256.Sum256([]byte(what))
	return hex.EncodeToString(sum[:8])
}

// ownerNote tells a session that the owner left the room, and tells it again
// when they come back.
//
// Both halves matter and they are separate facts, so each is said once: a
// session told only the leaving goes on filing decision packages at somebody
// sitting in front of it. Ident carries the moment the state began, so a second
// departure in the same session is not swallowed as the first one already said.
//
// Silence where nothing was recorded. Absence of a marker is absence of a
// statement, and a session behaving as it always has is the right answer to it.
func ownerNote(p *OwnerPresence) (Note, bool) {
	if p == nil {
		return Note{}, false
	}
	if p.Away {
		because := "The owner stepped away at " + stamp(p.Since) + " and is not reachable on this host"
		if !p.Until.IsZero() {
			because += ", until " + stamp(p.Until)
		}
		if p.Because != "" {
			because += " — " + p.Because
		}
		return Note{
			Because: because + ".",
			Do: "Carry the program on your own judgment and produce only reversible artifacts — branches, " +
				"commits, evidence, draft pull requests, written decision packages. Where the decision is " +
				"genuinely theirs, stop with the package on the issue or the pull request rather than asking: " +
				"that is a finished outcome, not a failure. `mellions report write -needs-owner \"...\"` is what " +
				"they read on the way back in.",
			Why: "A question asked into an empty room stops the work until they return, and an irreversible " +
				"act taken as though they were watching is what somebody finds at breakfast.",
			Ident: "owner-away-" + stamp(p.Since),
		}, true
	}
	because, at := "The owner is back as of "+stamp(p.Since)+": this host is attended again.", p.Since
	if p.Lapsed {
		because, at = "The away window the owner set ran out at "+stamp(p.Until)+
			": treat this host as attended again until they say otherwise.", p.Until
	}
	return Note{
		Because: because,
		Do: "Work as a peer again: a decision that is genuinely theirs is worth asking now rather than " +
			"filing, and `mellions report digest` is what the unattended work left for them.",
		Why: "A decision package written for somebody who is in the room is a question nobody was asked, " +
			"and the work behind it waits exactly as long.",
		Ident: "owner-back-" + stamp(at),
	}, true
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

// Peer is another session, already rendered.
type Peer struct {
	Describe string
	// Resume reopens it once it has ended; empty where nothing recorded a way.
	Resume string
}

// SectionStamp is one section of a governing document: its heading, the
// provenance tag that says whose it is, and a digest of its content.
type SectionStamp struct {
	Heading string
	Prov    string
	Digest  string
}

// Document is a governing document as it stands on disk now, beside the version
// that reached this session.
type Document struct {
	// Kind and Slug are which document: KindPartnership or KindProgram, and
	// which one of that kind.
	Kind, Slug string
	// Was is the version this session was handed. Nil where nothing recorded a
	// delivery — a session older than the record, or one with no session id to
	// key on. Nil means silence: a change cannot be established without knowing
	// what was delivered, and a guess here is worse than nothing.
	Was *Delivery
	// Digest and Sections are the document as it stands now.
	Digest   string
	Sections []SectionStamp
	// Missing says the document is no longer where it was.
	Missing bool
	// Back says the document is on disk again, holding the version this session
	// was handed, after the session was told it had gone.
	Back bool
}

// OwnerPresence is where the owner of this host says they are.
//
// Unattended is a state they enter and leave, not a property of how a session
// was started: a session at the keyboard becomes unattended the moment they say
// so, and no session can see them leave the room. Nil means nothing has been
// recorded either way — unknown, never attended, because a host whose owner has
// never said is not a host they are sitting at.
type OwnerPresence struct {
	// Away says nobody is reachable here now.
	Away bool
	// Since is when the state now recorded began.
	Since time.Time
	// Until is when an away window lapses; zero where none was named.
	Until time.Time
	// Because is what they said about where they were going.
	Because string
	// Lapsed says they are not away because the window they named ran out,
	// rather than because they said they were back.
	Lapsed bool
}

// Observation is what the caller established by looking.
type Observation struct {
	// Owner is where the owner of this host says they are; nil where nothing
	// has been recorded.
	Owner *OwnerPresence
	// Tree is the working tree the session is in, empty outside a repository.
	Tree string
	// Documents are the governing documents this session was handed, as they
	// stand now. Only the ones a delivery was recorded for are here.
	Documents []Document
	// Repo and Branch are what the tree is, as far as the caller established.
	Repo, Branch string
	// Source reports that Tree is the long-lived checkout this installation
	// cuts lanes from, rather than a lane worktree of its own.
	Source bool
	// Lane is the worktree this session should be working in for Repo, empty
	// where it has none open.
	Lane string
	// Reaching is a checkout every lane is cut from that this session's
	// command line steps into while standing somewhere else. Empty where it
	// steps into none. Repo names it; Tree is the directory.
	Reaching, ReachingRepo string
	// ReachingLane is this session's own worktree for ReachingRepo, empty
	// where it has none.
	ReachingLane string
	// Tracking reports that Branch has an upstream. In a source checkout it is
	// the difference between resting where lanes are cut from and sitting on a
	// lane branch somebody left behind.
	Tracking bool
	// Others are the live sessions registered in this same tree.
	Others []Peer
	// Elsewhere are live sessions on this same repository from another tree.
	Elsewhere []Peer
	// ContextBytes is how much transcript this session has accumulated since
	// the runtime last compacted it (or since it began); RenewAt is the size at
	// which a session is told to renew. Zero RenewAt says nothing.
	ContextBytes int64
	RenewAt      int64
	// CompactAt is the size this host's runtime has compacted at on its own,
	// measured from its transcripts, over CompactSamples compactions; zero when
	// none has been observed.
	CompactAt      int64
	CompactSamples int
	// Command is the Bash command line this session is about to run, empty
	// where the observation was not made at a tool call. It is read only to
	// recognise a situation the session is entering, never to decide anything
	// about the command itself — the guards that deny a call are their own
	// programs and live elsewhere.
	Command string
	// Idle reports that nothing is in flight anywhere on this installation.
	Idle bool
	// Survey is the path of a fresh estate survey, empty when there is none.
	Survey string
	// SurveyBrief is its one-line summary.
	SurveyBrief string
}

// Notes is everything worth telling this session, in the order it matters.
func Notes(o Observation) []Note {
	var out []Note

	// First of all, because whether anybody can answer a question decides what
	// the session may do with everything else it is about to be told.
	if n, ok := ownerNote(o.Owner); ok {
		out = append(out, n)
	}

	// Then, because a governing document that moved changes what the session
	// may do, and everything else it might be told is downstream of that.
	for _, d := range o.Documents {
		if n, ok := documentNote(d); ok {
			out = append(out, n)
		}
	}

	if n, ok := sandboxNote(o.Command); ok {
		out = append(out, n)
	}

	if n := len(o.Others); n > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "%s in this working tree:", plural(n, "Another session is", "Other sessions are"))
		for _, p := range o.Others {
			fmt.Fprintf(&b, " %s.", p.Describe)
		}
		out = append(out, Note{
			Because: b.String(),
			Do: "Coordinate before switching branch, merging, rebasing or removing files here: " +
				"`ListAgents` names them, `SendMessage(to: <name>, message: ...)` reaches them — say what " +
				"you are doing in this tree and ask what they are" + resumeHint(o.Others),
			Why: "A branch switch or a deleted untracked file changes what the other session's " +
				"next command means, and it will not be told.",
		})
	}

	if n := len(o.Elsewhere); n > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "%s on %s from another tree:", plural(n, "Another session is", "Other sessions are"), o.Repo)
		for _, p := range o.Elsewhere {
			fmt.Fprintf(&b, " %s.", p.Describe)
		}
		out = append(out, Note{
			Because: b.String(),
			Do: "Open the conversation now, not when it is needed: `ListAgents` names them, then " +
				"`SendMessage(to: <name>, message: \"<your lane, its objective, the files and contracts you " +
				"expect to touch> — what are you on?\")`. From then on tell them what you establish about " +
				"this repository as you establish it — a defect, a premise that moved, a contract — and ask " +
				"when you are stuck; a peer who cannot see your record is a colleague, not a hazard. " +
				"What is yours to change, delete or revert on a repository you share is " +
				"`Skill(skill: \"mellions:mellions-territory\")` — load it before touching what you did not write" +
				resumeHint(o.Elsewhere),
			// Read while choosing what to work on, this said a session was in
			// the repository and nothing about what that reserved, and survey
			// shifts filled the gap in: they passed over every issue in the
			// repository as territory. Overlapping work is the thing to check;
			// the repository is not the unit anybody takes.
			Why: "A session in a repository does not reserve it, and is not a reason to pass " +
				"over its work: every lane is a worktree of its own, and what refuses a second " +
				"lane is the claim on the issue, which `mellions assign open` checks for you.",
		})
	}

	if o.Source {
		where := o.Repo
		if where == "" {
			where = "this repository"
		}
		n := Note{
			Because: "This tree is the " + where + " source checkout every lane on this " +
				"installation is cut from, not a worktree of your own.",
			Do: "Commit in a lane: `mellions assign open -id <id> -repo " + where +
				" -objective \"...\" -because \"...\"` cuts a worktree and a branch, and leaves " +
				"this checkout where it is. Read it only through git plumbing or that lane — the rules " +
				"of a tree several lanes share are `Skill(skill: \"mellions:mellions-territory\")`.",
			Why: "`git checkout -b` here leaves the shared checkout on a lane branch. The next " +
				"deploy from it fails to fast-forward, and the next lane is cut from your commits.",
		}
		if !o.Tracking {
			n.Because = "This tree is the " + where + " source checkout every lane is cut from, " +
				"and it is on " + branchName(o.Branch) + ", which tracks no remote branch — " +
				"the state a lane left behind."
			n.Do = "Return it to its tracking branch before cutting anything from it, then work " +
				"in a lane: `mellions assign open -id <id> -repo " + where + " ...`."
			n.Why = "`mellions assign open` reads the upstream of whatever this checkout has " +
				"checked out. With none it falls back to this branch's HEAD, so the next lane is " +
				"cut from it and pinned \"local HEAD\" — a pin that reads right and is not."
		}
		// The tree by name, because a session reads this while typing a path
		// and "this tree" is the one thing it cannot check against.
		if o.Tree != "" {
			n.Because = strings.Replace(n.Because, "This tree is the ", "This tree, "+o.Tree+", is the ", 1)
		}
		if o.Lane != "" {
			n.Do = "Work in your lane, " + o.Lane + " — it is a worktree of your own. " + n.Do
		}
		out = append(out, n)
	}

	// A session may stand outside a shared checkout while a compound command
	// reaches into it, so awareness must inspect the command target as well as
	// the session's current directory.
	if o.Reaching != "" {
		repo := o.ReachingRepo
		if repo == "" {
			repo = "another repository"
		}
		n := Note{
			Because: "This command line steps into " + o.Reaching + ", the " + repo +
				" checkout every lane on this host is cut from, not a worktree of your own.",
			Do: "Read it without writing to it — `git -C " + o.Reaching + " show <rev>:<path>`, or " +
				"`git -C " + o.Reaching + " archive <rev> | tar -x -C \"$(mktemp -d)\"` for a whole tree.",
			Why: "Its working tree carries whatever the owner or another session has not committed, " +
				"and a command that writes it overwrites that in place, with no reflog entry and " +
				"nothing that reports what was there.",
		}
		if o.Lane != "" && o.ReachingRepo == o.Repo {
			n.Do = "Work in your lane, " + o.Lane + ", which is already at its own commit. " + n.Do
		} else if o.ReachingLane != "" {
			n.Do = "Your lane for " + repo + " is " + o.ReachingLane + ". " + n.Do
		}
		out = append(out, n)
	}

	if o.Idle {
		switch {
		case o.Survey != "":
			out = append(out, Note{
				Because: "Nothing is in flight. " + o.SurveyBrief,
				Do:      "Read " + o.Survey + " and decide what is worth doing; `mellions assign open` claims it.",
			})
		default:
			out = append(out, Note{
				Because: "Nothing is in flight here.",
				Do:      "mellions survey",
				Why:     "What needs attention across the estate, collected rather than remembered.",
			})
		}
	}
	// A session cannot compact itself and cannot see its own context, so the
	// transcript stands in: bytes since the last compaction, against the size
	// this host's runtime has actually compacted at. The note states the
	// measurement and what renewal is; when to renew is the session's judgment
	// at a boundary in the work — never a reason to stop and ask. Said once per
	// multiple of RenewAt, so a session that carries on is told again, not
	// nagged.
	if o.RenewAt > 0 && o.ContextBytes >= o.RenewAt {
		bucket := o.ContextBytes / o.RenewAt
		host := "no automatic compaction of a session of this model has been observed on this host yet, so the runtime's size here is unknown"
		if o.CompactAt > 0 {
			host = fmt.Sprintf("this host's runtime has compacted sessions of this model on its own at about %.1f MB (median of %d)", float64(o.CompactAt)/1e6, o.CompactSamples)
		}
		// Ending the shift renews the work only where the runner starts
		// another, so the clause is offered only against a recorded away
		// marker. Attended, shifts.sh starts none until `mellions away`.
		// Nil is unknown rather than attended, and shifts.sh does run back to
		// back on a host that has never recorded either — `shift_allowed`
		// admits that state beside away, deliberately — so this withholds a
		// clause that is true there. That way round is the cheap one: a restart
		// promised and not delivered stops the lane, a true one withheld only
		// routes the next phase through a dispatched successor, which continues
		// the work.
		shift := ""
		if o.Owner != nil && o.Owner.Away {
			shift = ", or end the shift and let the runner start the next one"
		}
		out = append(out, Note{
			Because: fmt.Sprintf("Context: %.1f MB of transcript since the last compaction; %s.", float64(o.ContextBytes)/1e6, host),
			Do: "Renewal changes which context does the work, never whether the work continues. At the next " +
				"boundary — a piece finished and its state written — decide whether to renew rather than carry " +
				"finished-work noise into a phase that needs fresh reasoning: write where it stands " +
				"(`mellions assign record -kind next \"...\"` — not the handoff, which records the lane " +
				"finished), then hand the next phase to a session you dispatch with that slate" + shift +
				". Never ask the owner to `/compact` and never wait: renewal is yours. If you carry on instead, " +
				"the runtime compacts on its own and the renewal hook carries the slate you wrote.",
			Why: "What the runtime's summary drops is what was not written down; what a fresh context gains is " +
				"the absence of everything that no longer matters.",
			Ident: fmt.Sprintf("context-%d", bucket),
		})
	}
	return out
}

// branchName renders a branch for a sentence, naming the gap when the caller
// could not establish one rather than reading as though there were none.
func branchName(b string) string {
	if strings.TrimSpace(b) == "" {
		return "a branch that could not be read"
	}
	return "`" + b + "`"
}

// documentNote is what a session is told when a governing document changed
// after it reached it.
//
// It names the document, the sections that differ and the one command that
// renders the current whole — not the changed text itself. Two reasons. A diff
// of a delegation is a trap: a withdrawn grant reads as an absence, an added one
// reads without the sentence that qualifies it, and a session acting on the
// changed lines alone can end up more wrong than one acting on the old document.
// And the change is unbounded, while a note arriving on a turn nobody asked for
// has to stay small enough to be read rather than skipped.
func documentNote(d Document) (Note, bool) {
	if d.Was == nil {
		return Note{}, false
	}
	noun, verb := docNoun(d.Kind), docVerb(d.Kind)
	ident := "doc:" + d.Kind + "/" + d.Slug + ":"
	if d.Back {
		// A session was told this document was gone and nothing else would ever
		// contradict that sentence: an unretracted false claim about what
		// governs the work is worse than the note it took to make it.
		return Note{
			Ident: ident + "back:" + d.Digest,
			Because: fmt.Sprintf("The %s %s is on disk again, and holds the version this session was handed. "+
				"It was reported gone; that no longer stands.", noun, d.Slug),
			Do: fmt.Sprintf("Nothing. What you hold is current — `mellions %s show %s` if you want to see it.",
				verb, ShellQuote(d.Slug)),
		}, true
	}
	if d.Missing {
		return Note{
			Ident: ident + "gone",
			Because: fmt.Sprintf("The %s %s is no longer on disk. It was there when this session was handed it.",
				noun, d.Slug),
			Why: missedBy(d.Kind),
			Do: fmt.Sprintf("`mellions %s list` says what is there now — establish what governs before "+
				"acting on what you were handed.", verb),
		}, true
	}
	if d.Digest == "" || d.Digest == d.Was.Digest {
		return Note{}, false
	}
	because := fmt.Sprintf("The %s %s changed after it reached this session", noun, d.Slug)
	if changed := changedSections(d.Was.Sections, d.Sections); changed != "" {
		because += ": " + changed
	}
	return Note{
		Ident:   ident + d.Digest,
		Because: because + ".",
		Why:     missedBy(d.Kind),
		Do: fmt.Sprintf("`mellions %s show %s` — read it before %s.",
			verb, ShellQuote(d.Slug), beforeActing(d.Kind)),
	}, true
}

func docNoun(kind string) string {
	if kind == KindPartnership {
		return "partnership with"
	}
	return "program"
}

func docVerb(kind string) string {
	if kind == KindPartnership {
		return "partner"
	}
	return "program"
}

// missedBy says what is lost by doing nothing, which is the whole point of the
// note: the session cannot see that what it holds is a previous version.
func missedBy(kind string) string {
	const once = " reaches a session once, at session start, and nothing between two starts re-reads it, " +
		"so what you hold is the version from before the change"
	if kind == KindPartnership {
		return "A partnership" + once + " — a grant made or withdrawn since is invisible to you."
	}
	return "A program" + once + " — a priority, a boundary or a correctness bar set since is invisible to you."
}

func beforeActing(kind string) string {
	if kind == KindPartnership {
		return "acting on what you believe is delegated"
	}
	return "deciding what the work is for"
}

// changedSections names what differs, the owner's own sections first, because a
// change to what they declared is the one that changes what the session may do.
func changedSections(was map[string]string, now []SectionStamp) string {
	const show = 3
	var declared, rest []string
	for _, s := range now {
		old, had := was[s.Heading]
		if had && old == s.Digest {
			continue
		}
		named := fmt.Sprintf("%q {%s}", s.Heading, s.Prov)
		if s.Prov == "DECLARED" {
			declared = append(declared, named)
		} else {
			rest = append(rest, named)
		}
	}
	here := map[string]bool{}
	for _, s := range now {
		here[s.Heading] = true
	}
	for heading := range was {
		if !here[heading] {
			rest = append(rest, fmt.Sprintf("%q {gone}", heading))
		}
	}
	slices.Sort(rest)
	all := append(declared, rest...)
	switch {
	case len(all) == 0:
		return ""
	case len(all) <= show:
		return strings.Join(all, ", ")
	default:
		return fmt.Sprintf("%s, and %d more", strings.Join(all[:show], ", "), len(all)-show)
	}
}

func resumeHint(peers []Peer) string {
	for _, p := range peers {
		if p.Resume != "" {
			return "; if it has ended, `" + p.Resume + "` reopens it."
		}
	}
	return "."
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ShellQuote makes a value safe to paste into a command line.
func ShellQuote(s string) string {
	if strings.ContainsAny(s, " \t\"'$`\\") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// startsContainer recognises a session bringing a container up. The forms are
// the ones a session actually types; a `docker ps` or a `docker logs` is not
// one of them, and neither is a `docker rm`, which is the act this note exists
// to ask for rather than to interrupt.
var startsContainer = regexp.MustCompile(
	`(?i)\bdocker\s+(run|create|start|compose\s+up)\b|\bdocker-compose\s+up\b|\bpodman\s+run\b`)

// sandboxNote is said once, the first time a session brings a container up.
//
// This host is shared, and resources a lane starts are borrowed. Teardown
// belongs to the turn that created them rather than to a later cleanup guess.
//
// It names the capability rather than the rule, because the rule is the Skill's
// and the judgement about whether the sandbox suits this work is the session's.
func sandboxNote(command string) (Note, bool) {
	if command == "" || !startsContainer.MatchString(command) {
		return Note{}, false
	}
	return Note{
		Because: "This session is starting a container on a host it shares with other lanes.",
		Do: "What you start is yours to give back in the same turn — containers, volumes, " +
			"networks and the anonymous volumes an image declares. " +
			"`Skill(skill: \"mellions:mellions-sandbox\")` carries the disposable-sandbox " +
			"options and the teardown check; `mellions skills <what you are doing>` finds " +
			"the rest of the toolbox at the moment you want it.",
		Why: "A stopped container still holds disk and a left-up one still holds RAM, and " +
			"neither is visible from the handoff being written hours later.",
		Ident: "sandbox-capability",
	}, true
}

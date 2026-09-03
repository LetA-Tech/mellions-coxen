// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/claim"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

func cmdAssign(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("assign needs a verb: " + assignVerbs)
	}
	switch args[0] {
	case "open":
		return assignOpen(args[1:])
	case "list":
		return assignList(args[1:])
	// show, because `program show` and `partner show` are the same act on the
	// other two records, and a session that has written one reaches for it here.
	case "get", "show":
		return assignGet(args[1:])
	case "record":
		return assignRecord(args[1:])
	case "handoff":
		return assignHandoff(args[1:])
	case "claim":
		return assignClaim(ctx, args[1:])
	case "close":
		return assignClose(args[1:])
	case "abandon":
		return assignAbandon(args[1:])
	case "reopen":
		return assignReopen(args[1:])
	case "sweep":
		return assignSweep(ctx, args[1:])
	case "-h", "--help", "help":
		fmt.Println("mellions assign open <id> -repo R -objective \"...\" -because \"...\" [-issue \"#N\"] [-branch b] [-base ref] [-worktree dir] [-alongside] [-unpublished] [-budget 4h]\n" +
			"mellions assign list [-all] | get <id> | record <id> <text> [-kind found|hypothesis|next|note]\n" +
			"mellions assign claim <id> -pr N            # this lane holds that change set; a peer reads the claim before merging\n" +
			"mellions assign handoff <id> [-file f|-] | reopen <id> | close <id> | abandon <id> -discarding \"...\"\n" +
			"mellions assign sweep [-repo R] [-apply]   # close the handed-off lanes whose pull request is merged or closed\n" +
			"Each verb takes -h for its flags.")
		return nil
	// Older verbs, kept so a session following an older method is told what
	// replaced them rather than handed an unknown-command error.
	case "suspend", "resume":
		return errors.New("assign " + args[0] + " is gone. Setting work down is a note:\n\n" +
			"  mellions assign record <id> -kind next \"set down for <what took priority>; stands at <where it is>\"\n\n" +
			"and picking it back up is simply working in its worktree again")
	// The kinds a recording can carry are not verbs, and reaching for one as a
	// verb is a near miss rather than a misunderstanding: say where it belongs.
	case "note", "found", "hypothesis", "next":
		return fmt.Errorf("assign %s is a kind of recording, not a verb:\n\n"+
			"  mellions assign record <id> -kind %s \"…\"", args[0], args[0])
	default:
		return fmt.Errorf("assign: unknown verb %q. The verbs are: %s", args[0], assignVerbs)
	}
}

// assignVerbs is what a refusal has to name. The top-level unknown-command
// error prints the whole usage; this one printed the rejected word and nothing
// else, so a session that guessed wrong was told only that it had guessed. It
// guessed "show" four times across three sessions — the word the sibling
// records use for the same act — and the refusal that could have said so said
// nothing, which is how one wrong guess becomes four.
const assignVerbs = "open, list, get, record, claim, handoff, reopen, close, abandon, sweep"

func assignStore(cfgPath string) (*assignment.Store, *Config, error) {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	s, err := assignment.NewStore(cfg.assignmentsRoot())
	if err != nil {
		return nil, nil, err
	}
	// Every lane with an issue publishes its claim where the other machines
	// look. An installation with no owner configured has no tracker to publish
	// to, and a lane on an issue is refused there rather than opened as a
	// claim only this disk can see.
	if strings.TrimSpace(cfg.Owner) != "" {
		s.Tracker = claim.NewTracker(cfg.Owner)
	}
	s.Registers = cfg.workRegisters()
	return s, cfg, nil
}

func assignOpen(args []string) error {
	cfgPath, o, err := parseOpen(args)
	if err != nil {
		return err
	}
	store, cfg, err := assignStore(cfgPath)
	if err != nil {
		return err
	}
	a, claimed, err := claimExisting(store, o.ID)
	if err != nil {
		return err
	}
	if !claimed {
		if o.Source, err = cfg.checkout(o.Repo); err != nil {
			return err
		}
		if a, err = store.Open(o); err != nil {
			return err
		}
	}
	noteWorking(cfg, a)
	fmt.Print(a.Text(time.Now()))
	return nil
}

// noteWorking tells the presence record what this session now works in.
//
// A record is written once, at session start, from the directory the runtime
// handed over — and an unattended shift is handed its home directory, which is
// no repository. Without this the record never names the repository the shift
// spends its life in, so `who` on that repository does not list it and the
// awareness note about a peer on the same repository has nothing to fire on:
// two shifts on one repository are each invisible to the other. Opening,
// adopting or reopening a lane is the moment the session learns what it works
// in, and this is where the record learns it.
//
// Best effort: a lane opens whether or not the record can be written, and there
// is no session to record for when the CLI is run from a terminal.
func noteWorking(cfg *Config, a *assignment.Assignment) {
	_, id := presence.Here()
	if id == "" {
		return
	}
	_ = cfg.presences().Working(id, presence.Work{
		Tree: a.Worktree, Repo: a.Repo, Branch: a.Branch, Assignment: a.ID,
	}, time.Now())
}

// claimExisting takes up work whose record already exists, and reports whether
// it handled the call.
//
// It runs before the repository is resolved and before Open validates, because
// everything those two demand — a checkout, an objective, a reason — is already
// in the record. A session told to claim a handed-off lane was refused three
// times in sequence for a repository it had given, then an objective the record
// held, and only then told the record existed; every message named a missing
// input, so the only reading was "supply more" when no amount of flags would
// have worked. Every dispatch says "claim it with mellions assign open <id>",
// which made that instruction unexecutable for any lane that had been handed
// off.
//
// Reopen decides what may be taken up: it re-cuts a worktree that has gone, and
// it refuses a closed or abandoned lane in the words that say a new assignment
// is the answer. Active is not a refusal — a lane you already hold is claimed,
// and the record prints who last worked it, so a second session meets the
// collision rather than an exit code.
func claimExisting(store *assignment.Store, id string) (*assignment.Assignment, bool, error) {
	a, err := store.Get(id)
	if errors.Is(err, assignment.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	if a.State != assignment.StateActive {
		if a, err = store.Reopen(id); err != nil {
			return nil, true, err
		}
	}
	return a, true, nil
}

// parseOpen reads what `assign open` was asked for. Separate from opening it so
// the argument handling is checkable without a checkout to cut a worktree from.
// Source is left empty here: resolving a repository to one is the caller's.
func parseOpen(args []string) (string, assignment.OpenOptions, error) {
	fs := newFlagSet("assign open", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	id := fs.String("id", "", "short assignment id; the first argument does the same")
	repo := fs.String("repo", "", "repository this concerns")
	issue := fs.String("issue", "", "tracked item, e.g. #223")
	objective := fs.String("objective", "", "what this work is for")
	because := fs.String("because", "", "why this work rather than the alternatives")
	notChosen := fs.String("not-chosen", "", "what was passed over, and why")
	branch := fs.String("branch", "", "branch name; default mellions/<id>")
	baseRef := fs.String("base", "", "commit to cut from; default the source HEAD")
	worktree := fs.String("worktree", "", "adopt this existing working tree instead of cutting one; its branch is recorded and it is never removed")
	alongside := fs.Bool("alongside", false, "open even though another live lane already claims this issue; reconciling two lanes is the case that needs it")
	unpublished := fs.Bool("unpublished", false, "open even though the claim cannot reach the tracker, accepting that no other machine can see this lane")
	budget := fs.Duration("budget", 0, "wall-clock budget before a written status is owed")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return "", assignment.OpenOptions{}, err
	}
	given, err := openID(*id, rest)
	if err != nil {
		return "", assignment.OpenOptions{}, err
	}
	return *cfgPath, assignment.OpenOptions{
		ID: given, Repo: *repo, Issue: *issue, Objective: *objective,
		Because: *because, NotChosen: *notChosen,
		Branch: *branch, BaseRef: *baseRef, Worktree: *worktree, Alongside: *alongside,
		Unpublished: *unpublished,
		Budget:      assignment.Budget{Wall: *budget},
	}, nil
}

// openID settles which id was asked for. Every other assign verb takes it as an
// argument, so open takes it either way, and -id remains the named form that
// wins. Two ids naming different assignments have no correct reading — choosing
// one silently would open a lane, a branch and a worktree under a name nobody
// typed — so they are refused rather than resolved.
func openID(flagID string, rest []string) (string, error) {
	if len(rest) > 1 {
		return "", fmt.Errorf("assign open takes one id, and was given %d: %s",
			len(rest), strings.Join(rest, " "))
	}
	id := strings.TrimSpace(flagID)
	if len(rest) == 0 {
		return id, nil
	}
	positional := strings.TrimSpace(rest[0])
	if id == "" {
		return positional, nil
	}
	if id != positional {
		return "", fmt.Errorf("assign open was given two ids: -id %q and %q. Name one", id, positional)
	}
	return id, nil
}

// assignID settles which id a verb was given, and what is left over. Every
// verb here has always taken the id positionally and `assign open` has always
// taken -id, so each takes both spellings: a session that learned one on one
// verb is not wrong on the next. The flag wins when it is present, and then
// every positional is the verb's own text rather than a second id.
func assignID(flagID string, rest []string) (string, []string) {
	if id := strings.TrimSpace(flagID); id != "" {
		return id, rest
	}
	if len(rest) == 0 {
		return "", nil
	}
	return strings.TrimSpace(rest[0]), rest[1:]
}

func assignList(args []string) error {
	fs := newFlagSet("assign list", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	all := fs.Bool("all", false, "include closed and abandoned assignments")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	list, damaged, err := store.ListWithDamage(*all)
	if err != nil {
		return err
	}
	for _, id := range damaged {
		fmt.Printf("%-14s UNREADABLE — the record exists and did not survive its write. The branch\n"+
			"%-14s and its commits are untouched; read those rather than repairing the file.\n", id, "")
	}
	if len(list) == 0 {
		if len(damaged) == 0 {
			fmt.Println("no assignments")
		}
		return nil
	}
	now := time.Now()
	for _, a := range list {
		flag := ""
		if a.Overdue(now) {
			flag = "  BUDGET ELAPSED — a written status is owed"
		}
		fmt.Printf("%-28s %-10s %-22s %s%s\n", a.ID, a.State, a.Repo+" "+a.Issue, firstLineOf(a.Objective), flag)
	}
	return nil
}

func assignGet(args []string) error {
	fs := newFlagSet("assign get", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, extra := assignID(*idFlag, rest)
	if id == "" || len(extra) > 0 {
		return errors.New(`assign get needs one id: mellions assign get <id>   (-id <id> does the same)`)
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	a, err := store.Get(id)
	if err != nil {
		return err
	}
	fmt.Print(a.Text(time.Now()))
	return nil
}

// laneHere is the open assignment whose worktree holds the working directory,
// or nil where none does. Paths are compared after symlink resolution because
// a worktree reached through one is the same tree.
func laneHere(store *assignment.Store) *assignment.Assignment {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(wd); err == nil {
		wd = resolved
	}
	open, err := store.List(false)
	if err != nil {
		return nil
	}
	var best *assignment.Assignment
	for _, a := range open {
		if a.Worktree == "" {
			continue
		}
		tree := a.Worktree
		if resolved, err := filepath.EvalSymlinks(tree); err == nil {
			tree = resolved
		}
		if wd != tree && !strings.HasPrefix(wd, tree+string(filepath.Separator)) {
			continue
		}
		// Nested worktrees are not expected, but the innermost is the lane
		// actually being worked if they ever occur.
		if best == nil || len(tree) > len(best.Worktree) {
			best = a
		}
	}
	return best
}

func assignRecord(args []string) error {
	fs := newFlagSet("assign record", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	kind := fs.String("kind", "note", "hypothesis, found, next or note")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, text := assignID(*idFlag, rest)
	if len(rest) == 0 {
		return assignRecordUsageError()
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	here := laneHere(store)
	// The id in hand is usually the last one opened rather than the one being
	// worked: a session that opens a second lane keeps writing to the first,
	// and the record of the work ends up on the lane that did not do it.
	//
	// Inside a lane the id may be left out, and then the first argument is the
	// text rather than the id — which is only decidable against the store,
	// since both are just words. An argument that names no assignment is prose.
	if here != nil {
		if id == "" {
			id = here.ID
		} else if _, err := store.Get(id); err != nil {
			text = append([]string{id}, text...)
			id = here.ID
			fmt.Fprintf(os.Stderr, "mellions: no assignment is called %q; recorded on %s, whose worktree this is.\n",
				text[0], id)
		}
	}
	if id == "" || len(text) == 0 {
		return assignRecordUsageError()
	}
	if err := store.Record(id, *kind, strings.Join(text, " ")); err != nil {
		return err
	}
	if here != nil && here.ID != id {
		fmt.Fprintf(os.Stderr,
			"mellions: recorded on %s, but this tree is %s's lane.\n"+
				"A record on a lane you are not working is working memory the next session reads under the wrong objective.\n",
			id, here.ID)
	}
	return nil
}

func assignRecordUsageError() error {
	return errors.New(`assign record needs an id and some text:` + "\n\n" +
		`  ` + usageLine("assign record") + "\n" +
		`  mellions assign record -id <id> -kind found "…"` + "\n\n" +
		`The text is the argument after the id; there is no flag for it.` + "\n" +
		`Inside a lane's worktree the id is optional and that lane is used.`)
}

// assignClaim publishes this lane's hold on a pull request.
//
// A lane that dispatches a review of its own draft claims the draft first:
// "draft" alone means unfinished, unreviewed and blocked-on-a-review-in-flight
// alike, and a peer on another host that reads the second where the third is
// true merges work a review is about to reject.
func assignClaim(ctx context.Context, args []string) error {
	fs := newFlagSet("assign claim", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	pr := fs.String("pr", "", "the pull request this lane holds, as a number")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, _ := assignID(*idFlag, rest)
	if id == "" || strings.TrimSpace(*pr) == "" {
		return errors.New("assign claim needs a lane and a pull request: " +
			"mellions assign claim <id> -pr <n>")
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	if err := store.ClaimPullRequest(ctx, id, *pr); err != nil {
		return err
	}
	a, err := store.Get(id)
	if err != nil {
		return err
	}
	fmt.Printf("%s holds %s %s — it carries `%s`, so a survey on any host shows it held.\n\n"+
		"Release it by closing the lane. A claim not restated within 24 hours is swept, not obeyed.\n",
		id, a.Repo, a.PullRequest, claim.Label)
	return nil
}

func assignHandoff(args []string) error {
	fs := newFlagSet("assign handoff", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	file := fs.String("file", "", "read the handoff from a file, or - for stdin")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	reconciled := fs.String("reconciled", "",
		"the closed obligation set completion was checked against, and where it is stated")
	residual := fs.String("residual", "", "what is still owed, where the work is not finished")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, body := assignID(*idFlag, rest)
	if id == "" {
		return errors.New(`assign handoff needs an id: mellions assign handoff <id> "…"   (-id <id> does the same)`)
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	text := strings.Join(body, " ")
	if *file != "" {
		if text, err = readInput(*file); err != nil {
			return err
		}
	}
	// The challenge comes before the write, so a claim that cannot name what it
	// enumerated is not on the record while the question is being answered.
	if challenge := assignment.ChallengeCompletion(text, *reconciled, *residual); challenge != "" {
		fmt.Fprintf(os.Stderr, "%s\n", challenge)
		return fmt.Errorf("%s not handed off: the completion claim is unreconciled", id)
	}
	// What was answered belongs in the handoff, where the next reader sees the
	// claim and its support together rather than the claim alone.
	stored := text
	if strings.TrimSpace(*reconciled) != "" {
		stored += "\n\nReconciled against: " + strings.TrimSpace(*reconciled)
	}
	if strings.TrimSpace(*residual) != "" {
		stored += "\n\nStill owed: " + strings.TrimSpace(*residual)
	}
	if err := store.Handoff(id, stored); err != nil {
		return err
	}
	where := ""
	if a, err := store.Get(id); err == nil && a.PullRequest != "" {
		where = fmt.Sprintf(" The handoff is on %s %s, where the other host reads it.", a.Repo, a.PullRequest)
	}
	outcome := fmt.Sprintf("%s handed off — %d bytes stored.%s", id, len(stored), where)
	// Said at both ends. The outcome used to be the first line under twelve
	// lines of standing text, so the ordinary way to read a verbose command —
	// tail — showed the lecture and hid the result, and the session ran it
	// three times to find out whether it had worked.
	fmt.Printf("%s\n\n%s\n\n%s\n\n%s\n", outcome, renewalHandoffNote, learningPrompt, outcome)
	return nil
}

// learningPrompt is the last question of finishing, asked at the one moment
// somebody is guaranteed to be there to answer it.
const learningPrompt = `Before this ends: what should change about how the next piece of work is
done? A method that misled you, a check the repository should carry, a fact
the program got wrong, something the owner corrected. Usually nothing — and
then say nothing. Where there is something, put it where it binds: a test in
the repository, a Skill, the program's INFERRED or UNKNOWN sections. The
mellions-self-learning Skill is the method. If a Skill you used changed
recently (git log -3 -- skills/<name>, where the Mellions checkout is), you were its
held-out test: say whether the change changed what you did.`

func assignClose(args []string) error {
	fs := newFlagSet("assign close", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, extra := assignID(*idFlag, rest)
	if id == "" || len(extra) > 0 {
		return errors.New(`assign close needs one id: mellions assign close <id>   (-id <id> does the same)`)
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	if err := store.Close(id); err != nil {
		return err
	}
	if done, err := store.Get(id); err == nil && done.Adopted {
		fmt.Printf("%s closed; its adopted worktree %s and branch %s are left in place.\n", id, done.Worktree, done.Branch)
		return nil
	}
	fmt.Printf("%s closed; its worktree is removed and its branch kept.\n", id)
	return nil
}

func assignAbandon(args []string) error {
	fs := newFlagSet("assign abandon", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	discarding := fs.String("discarding", "", "what is being thrown away, and why it is acceptable to lose")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, extra := assignID(*idFlag, rest)
	if id == "" || len(extra) > 0 {
		return errors.New(`assign abandon needs one id: mellions assign abandon <id> -discarding "…"   (-id <id> does the same)`)
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	a, err := store.Get(id)
	if err != nil {
		return err
	}
	u, uerr := store.Unsaved(a)
	if err := store.Abandon(id, *discarding, nil); err != nil {
		fmt.Fprintf(os.Stderr, "mellions: %v\n", err)
	}
	done, err := store.Get(id)
	if err != nil {
		return err
	}
	switch {
	case done.Adopted:
		fmt.Printf("%s abandoned. Its worktree %s and branch %s were adopted, not cut, and are left in place.\n", a.ID, a.Worktree, a.Branch)
	case uerr != nil:
		fmt.Printf("%s abandoned. What its worktree held could not be established (%v); the directory is gone.\n", a.ID, uerr)
	case u.Any():
		fmt.Printf("%s abandoned. %s destroyed with %s.\n", a.ID, u, a.Worktree)
	default:
		fmt.Printf("%s abandoned. The worktree held nothing unsaved.\n", a.ID)
	}
	if d := done.Discarded; d != nil && d.Commits > 0 {
		fmt.Printf("Branch %s deleted with %d commit(s) on it. Its tip was %s,\n"+
			"which git keeps reachable for now if this was a mistake.\n",
			d.Branch, d.Commits, shortSHA(d.Tip))
	}
	return nil
}

func assignReopen(args []string) error {
	fs := newFlagSet("assign reopen", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	idFlag := fs.String("id", "", "short assignment id; the first argument does the same")
	rest, err := parsePositional(fs, args)
	if err != nil {
		return err
	}
	id, extra := assignID(*idFlag, rest)
	if id == "" || len(extra) > 0 {
		return errors.New(`assign reopen needs one id: mellions assign reopen <id>   (-id <id> does the same)`)
	}
	store, _, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	a, err := store.Reopen(id)
	if err != nil {
		return err
	}
	fmt.Print(a.Text(time.Now()))
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

// readInput reads a file, or stdin for "-".
func readInput(path string) (string, error) {
	if path == "" {
		return "", errors.New("no input file given (use - for stdin)")
	}
	if path == "-" {
		raw, err := readAll(os.Stdin)
		return string(raw), err
	}
	raw, err := os.ReadFile(path)
	return string(raw), err
}

// assignSweep closes the lanes the tracker says are finished, or says which
// it would close. One line per open lane either way, so what the sweep read
// is on the screen next to what it did.
func assignSweep(ctx context.Context, args []string) error {
	fs := newFlagSet("assign sweep", flag.ExitOnError)
	cfgPath := fs.String("config", "", "config file")
	repo := fs.String("repo", "", "only lanes on this repository")
	apply := fs.Bool("apply", false, "close what is closable; without it the sweep only says what it would close")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, cfg, err := assignStore(*cfgPath)
	if err != nil {
		return err
	}
	o := assignment.SweepOptions{Repo: *repo, Apply: *apply, Live: liveSessions(cfg)}
	if strings.TrimSpace(cfg.Owner) != "" {
		tr := claim.NewTracker(cfg.Owner)
		o.PullRequests = func(ctx context.Context, repo, branch string) ([]claim.PullRequest, error) {
			c, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			return tr.PullRequests(c, repo, branch)
		}
	}
	swept, err := store.Sweep(ctx, o)
	if err != nil {
		return err
	}
	if len(swept) == 0 {
		fmt.Println("no open assignments")
		return nil
	}
	counts := map[string]int{}
	for _, v := range swept {
		counts[v.Verdict]++
		fmt.Printf("%-28s %-9s %s\n", v.ID, v.Verdict, v.Why)
	}
	fmt.Println()
	switch {
	case *apply:
		fmt.Printf("%d closed — worktree removed, branch and record kept; %d kept.\n", counts["closed"], counts["kept"])
	case counts["closable"] > 0:
		fmt.Printf("%d closable — a dry run; `mellions assign sweep -apply` closes them: worktree removed, "+
			"branch and record kept, the record saying the sweep did it. %d kept.\n", counts["closable"], counts["kept"])
	default:
		fmt.Printf("nothing closable; %d kept.\n", counts["kept"])
	}
	return nil
}

// liveSessions is every running session on this host, by the id a lane
// records, each described as a sweep line needs it. This session is included
// on purpose: a lane it holds is being worked, and the sweep is the one
// reader that must not take its own lane for unattended work.
func liveSessions(cfg *Config) map[string]string {
	_, me := presence.Here()
	mine := presence.SelfPID()
	out := map[string]string{}
	for _, s := range cfg.presences().Live() {
		d := fmt.Sprintf("a live %s session %s (pid %d) in %s", s.Runtime, s.Short(), s.PID, s.Cwd)
		if isSelf(s, me, mine) {
			d = "this session (" + s.Short() + ")"
		}
		out[s.ID] = d
	}
	return out
}

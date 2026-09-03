// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// `mellions renew` is what a session says to its own compaction. The runtime
// decides when to compact — nothing the model can call starts one, established
// from the Claude Code binary at 2.1.250: the compaction path runs on a token
// threshold, `/compact` is a local command rather than a Skill, and the Skill
// tool resolves only against the Skill registry. So renewal is not something a
// session triggers; it is something a session is ready for. This command is the
// readiness: run from the PreCompact hook, its stdout becomes the runtime's
// custom compaction instructions verbatim (executePreCompactHooks joins each
// succeeding hook's stdout and appends it to the summary request), so it is the
// one place that says what the summary must keep and what it may let go.
//
// It names the lane rather than restating it. The established facts already
// live on the assignment record, outside the conversation and outside the
// repository, and a summary asked to carry them would be carrying a copy of the
// durable thing. What the summary has to keep is the engineering responsibility
// — which lane, what the claim is, where the work stands, what is next — and
// the address of the record that holds the rest.
func cmdRenew(args []string) error {
	fs := newFlagSet("renew", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "config file")
	trigger := fs.String("trigger", "", "the runtime's compaction trigger: auto or manual")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	as, err := assignment.NewStore(cfg.assignmentsRoot())
	if err != nil {
		return err
	}
	_, cwd := hookContext(os.Stdin)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	lane := laneAt(as, cwd)
	fmt.Print(renewalInstructions(lane, *trigger))
	return nil
}

// laneAt is the assignment whose worktree the session is standing in. A session
// works in one lane's tree at a time, so the tree is the addressing: matching on
// the repository instead would name three lanes and pick the wrong one. Nil when
// the session is not in any lane's worktree, which is ordinary — a session
// surveying the estate or reading a checkout directly holds no lane, and a
// summary still has to be told what to keep.
func laneAt(as *assignment.Store, cwd string) *assignment.Assignment {
	if cwd == "" {
		return nil
	}
	// The runtime reports the working directory as the shell sees it; a record
	// stores the path it created. On this host /var and /tmp are symlinks, so a
	// literal comparison misses a match that is the same directory. Resolve both
	// and fall back to the unresolved form rather than to nothing.
	target := resolved(cwd)
	open, _, err := as.ListWithDamage(false)
	if err != nil {
		return nil
	}
	var best *assignment.Assignment
	for _, a := range open {
		if a.Worktree == "" {
			continue
		}
		w := resolved(a.Worktree)
		if !under(target, w) {
			continue
		}
		// Nested worktrees are not expected, but the deepest match is the one
		// the session is actually in, so ambiguity resolves rather than races.
		if best == nil || len(resolved(best.Worktree)) < len(w) {
			best = a
		}
	}
	return best
}

func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// under reports whether path is dir or lives inside it, on separator
// boundaries: /a/tree-2 is not inside /a/tree.
func under(path, dir string) bool {
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(dir, string(filepath.Separator))+string(filepath.Separator))
}

// renewalInstructions is the text the runtime is handed. It is split out from
// the command so a test reads what a session would be told rather than what the
// command happened to print.
func renewalInstructions(lane *assignment.Assignment, trigger string) string {
	var b strings.Builder
	b.WriteString("This summary is a working engineer's context being renewed, not a chat being shortened.\n")
	b.WriteString("It is read next by the same engineer, mid-work, with no person to ask.\n\n")

	if lane != nil {
		fmt.Fprintf(&b, "THE LANE THIS SESSION IS IN — carry every line of this through verbatim:\n")
		fmt.Fprintf(&b, "  assignment %s · repository %s · branch %s\n", lane.ID, lane.Repo, lane.Branch)
		if lane.Worktree != "" {
			fmt.Fprintf(&b, "  worktree %s\n", lane.Worktree)
		}
		if lane.Issue != "" {
			fmt.Fprintf(&b, "  issue %s\n", lane.Issue)
		}
		fmt.Fprintf(&b, "  objective: %s\n", firstLineOf(lane.Objective))
		fmt.Fprintf(&b, "  the record holding the established facts: `mellions assign get %s`\n\n", lane.ID)
	} else {
		b.WriteString("THE LANE THIS SESSION IS IN: none — the working directory is not a lane's worktree.\n")
		b.WriteString("Say so rather than inventing one, and keep whatever the session was actually doing.\n\n")
	}

	b.WriteString("KEEP, in full, and prefer keeping too much of this to too little:\n")
	b.WriteString("  · the lane above, and any other repository or branch this session has open work in\n")
	b.WriteString("  · what has been ESTABLISHED and how — the file and line, the command run, what it printed;\n")
	b.WriteString("    a fact whose citation is dropped becomes an assumption, which is a downgrade, not a saving\n")
	b.WriteString("  · every decision taken and the reason it beat the alternative, including decisions not to act\n")
	b.WriteString("  · what is still UNRESOLVED, and what would settle each one\n")
	b.WriteString("  · what needs the owner and why, verbatim — an unattended session cannot re-derive that\n")
	b.WriteString("  · anything started on this machine and not yet given back: a container, a tunnel, a\n")
	b.WriteString("    background process, a worktree that was never this session's\n")
	b.WriteString("  · what was tried and failed, and why — otherwise it is tried again after this summary\n")
	b.WriteString("  · the exact next step, as an action rather than a topic\n\n")

	b.WriteString("LET GO — it is recoverable from the repository, the record or a re-run:\n")
	b.WriteString("  · file contents, command output and search results quoted at length; keep the conclusion\n")
	b.WriteString("    and the citation that reaches them again\n")
	b.WriteString("  · the order things were discovered in, and the false starts that led nowhere and changed nothing\n")
	b.WriteString("  · tool-call mechanics, retries, and anything already written to a commit, a pull request,\n")
	b.WriteString("    an issue comment or the assignment record — that is where it durably lives\n\n")

	b.WriteString("Label what survives the way it was held: established / inferred / assumed / unknown.\n")
	b.WriteString("A claim that arrives without its label arrives stronger than it was, and the session after\n")
	b.WriteString("this one has no way to tell.\n")
	if trigger == "auto" {
		b.WriteString("\nThis compaction was the runtime's, on its own threshold, mid-work. Nothing is finished\n")
		b.WriteString("because it happened, and the next step above is still the next step.\n")
	}
	return b.String()
}

// renewalHandoffNote is what the handoff says about renewal. The handoff is the
// completion point — the moment a session has moved everything that must survive
// onto the record — and it is the only moment the session can know that. Nothing
// lets it compact there, so what it can be told is that from here the
// conversation is disposable, which is the difference between a compaction that
// costs nothing and one that costs the work.
const renewalHandoffNote = "This is the renewal point. What must survive is now on the record and " +
	"outside this conversation, so a compaction from here — the runtime's own, on its threshold, " +
	"which is the only kind there is — costs nothing. Take the next work up in this session " +
	"without carrying this one's transcript into it: what the next session needs, the record has."

// renewUsage is the line in the top-level usage.
const renewUsage = `  mellions renew [-trigger auto|manual]
        What a session says to its own compaction: the instructions that decide
        what the runtime's summary keeps and what it lets go, anchored on the
        lane the working directory is in. Run from the PreCompact hook, where
        stdout becomes those instructions. Nothing in either runtime lets a
        session start a compaction, so this shapes the one the runtime starts.
`

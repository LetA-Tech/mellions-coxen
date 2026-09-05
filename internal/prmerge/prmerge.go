// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package prmerge decides whether a Bash tool call merges a pull request whose
// state cannot support the decision to merge it.
//
// Merging is where an older branch silently rolls a base branch backward. Git
// resolving the merge says nothing about whether it should: the branch may have
// diverged before a fix landed on the base, and re-applying its side of a file
// the base has since changed reverts that fix under a green suite. Conflict-free
// is not regression-free.
//
// A note is already delivered at this command, and a note is not enough for two
// reasons that have nothing to do with how well it is written. It is advisory,
// so nothing has to answer it — a denial becomes a tool result the session must
// address. And it is said once per session, keyed by the Skill it names, so the
// eighth merge of a long session is reached in silence, which is exactly the
// merge where discipline has decayed.
//
// So this refuses, and only on a state established rather than guessed:
//
//   - mergeability GitHub has not finished computing. `UNKNOWN` is not an
//     answer that a decision can rest on; it is the absence of one, and it
//     appears for seconds after a push, which is when a session is most likely
//     to be looking.
//   - a branch behind its base where the base's commits since the divergence
//     touch files this pull request also changes. That intersection is the
//     hazard stated concretely: those are the files where one side is about to
//     be written over the other.
//
// Being behind on its own is not refused. It is ordinary, usually harmless, and
// a guard that fires on correct work is turned off and then protects nothing.
//
// What this establishes is narrower than what it is for, and the refusal says
// so: it sees a textual overwrite of newer work on the base, never a semantic
// regression. A pull request that changes A while the base changed B, where A is
// wrong given the new B, has no file in common and passes here.
//
// Anything it cannot read — the tracker unreachable, the comparison too large to
// enumerate, a selector naming no pull request — leaves it silent, because a
// deny on a guess blocks legitimate work and is how a guard gets removed.
package prmerge

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/ghcmd"
	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// overlapNamed is how many overlapping paths the refusal lists before it counts
// the rest. The list is the evidence; a screen of paths is not more evidence.
const overlapNamed = 10

// Call is one `gh pr merge` in a command.
type Call struct {
	// Selector is the number, URL or branch the command names, empty where it
	// merges whatever pull request the current branch has.
	Selector string
	// Repo is what --repo named, empty for the checkout's own.
	Repo string
}

// State is what the tracker says about the pull request a call names.
type State struct {
	// Number and URL identify it in the refusal.
	Number int
	URL    string
	// Base is the branch it would merge into.
	Base string
	// MergeState is gh's mergeStateStatus. "UNKNOWN" means GitHub has not
	// finished computing mergeability, which is the absence of an answer.
	MergeState string
	// BehindBy is how many commits the base has that the head does not. It is
	// counted from the comparison rather than read from mergeStateStatus,
	// because gh only reports BEHIND where branch protection requires the
	// branch to be current, so that field is silent on most repositories.
	BehindBy int
	// Overlap is the files changed both by this pull request and by the base
	// since the divergence, sorted.
	Overlap []string
	// Truncated says the comparison could not enumerate every base-side file,
	// so an empty Overlap does not establish that there is none.
	Truncated bool
}

// Deny returns the reason to refuse a PreToolUse payload, or "" to stay silent.
// look answers with the state of the pull request a call names; an error, or a
// zero Number, means it could not be established, and unknown is silence.
func Deny(payload []byte, look func(cwd string, call Call) (State, error)) string {
	var ev struct {
		ToolName string `json:"tool_name"`
		Cwd      string `json:"cwd"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" {
		return ""
	}
	for _, call := range Calls(ev.Input.Command) {
		state, err := look(ev.Cwd, call)
		if err != nil || state.Number == 0 {
			continue
		}
		if reason := refuse(state); reason != "" {
			return reason
		}
	}
	return ""
}

// Calls returns every `gh pr merge` the command makes. A command line that
// mixes one with other work yields only the merge itself, so a pull request
// number written in some other command's text is not read as a merge.
func Calls(command string) []Call {
	var out []Call
	for _, c := range shellsplit.Split(command) {
		args, ok := ghcmd.Args(c.Words, func(noun, verb string) bool {
			return noun == "pr" && verb == "merge"
		})
		if !ok {
			continue
		}
		var call Call
		for i := 0; i < len(args); i++ {
			name, glued, hasGlued := ghcmd.SplitFlag(args[i])
			if name == "--repo" || name == "-R" {
				if hasGlued {
					call.Repo = glued
				} else if i+1 < len(args) {
					i++
					call.Repo = args[i]
				}
				continue
			}
			if strings.HasPrefix(name, "-") {
				// A flag that takes a value must not have its value read as
				// the selector. These are gh's value-taking merge flags; a
				// valueless flag falls through and consumes nothing.
				switch name {
				case "--body", "-b", "--body-file", "-F", "--subject", "-t",
					"--match-head-commit", "--author-email":
					if !hasGlued && i+1 < len(args) {
						i++
					}
				}
				continue
			}
			if call.Selector == "" {
				call.Selector = args[i]
			}
		}
		out = append(out, call)
	}
	return out
}

// refuse is the decision on one pull request's state.
func refuse(s State) string {
	where := "#" + strconv.Itoa(s.Number)
	if s.URL != "" {
		where = s.URL
	}
	switch {
	case strings.EqualFold(s.MergeState, "UNKNOWN"):
		return "Merging " + where + " while GitHub still reports its mergeability as UNKNOWN.\n\n" +
			"That is not a state the branch is in — it is GitHub not having finished " +
			"computing one, which is what it reports for a while after a push. Anything " +
			"read about this merge right now, including a clean-looking answer, was read " +
			"before the answer existed.\n\n" +
			"Ask again until it is not UNKNOWN, then decide:\n\n" +
			"    gh pr view " + strconv.Itoa(s.Number) + " --json mergeable,mergeStateStatus"

	case s.BehindBy > 0 && s.Truncated:
		return "Merging " + where + ", which is " + plural(s.BehindBy, "commit") + " behind " + s.Base +
			", and the comparison is too large to enumerate — so whether those commits touch " +
			"files this pull request also changes could not be established here.\n\n" +
			"A branch this far behind is reconciled rather than merged on the strength of git " +
			"being able to resolve it. Rebase it on " + s.Base + ", or read the divergence and " +
			"say why it does not overlap:\n\n" +
			"    git fetch origin && git diff --name-only origin/" + s.Base + "...HEAD"

	case s.BehindBy > 0 && len(s.Overlap) > 0:
		return "Merging " + where + ", which is " + plural(s.BehindBy, "commit") + " behind " + s.Base +
			", and " + s.Base + " has changed " + plural(len(s.Overlap), "file") +
			" that this pull request also changes:\n\n" + list(s.Overlap) + "\n\n" +
			"This is the case where a merge rolls the base backward: the branch diverged, " +
			"work landed on " + s.Base + " in these files, and this side of them is about to be " +
			"written over it. Git resolving the merge says nothing about whether it should.\n\n" +
			"Read both sides of each file before merging, and reconcile rather than accepting " +
			"either whole — rebasing on " + s.Base + " is usually the honest way to do that:\n\n" +
			"    git fetch origin && git log --oneline HEAD..origin/" + s.Base + " -- " + s.Overlap[0] + "\n\n" +
			"This guard reads files, not meaning: it cannot see a change that regresses " + s.Base +
			" without touching the same file, so a clean answer here is not a review."
	}
	return ""
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// list renders the overlap, bounded, saying how many it did not name rather
// than trailing off.
func list(paths []string) string {
	sort.Strings(paths)
	shown := paths
	var rest int
	if len(shown) > overlapNamed {
		rest = len(shown) - overlapNamed
		shown = shown[:overlapNamed]
	}
	var b strings.Builder
	for _, p := range shown {
		b.WriteString("    " + p + "\n")
	}
	if rest > 0 {
		b.WriteString("    … and " + plural(rest, "more") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

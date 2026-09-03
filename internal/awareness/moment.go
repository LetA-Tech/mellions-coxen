// Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
package awareness

import "strings"

// Moment is a command shape that is unmistakably the situation one Skill
// names, and the Skill it names.
//
// A catalog delivered at session start is read once, against work that has not
// happened yet, and a trigger phrased as a state — "whenever remediating" — is
// true for most of a long session and so registers as nothing. These are
// phrased as the opposite: a single command, at the instant it is run, once.
type Moment struct {
	// Skill is the bare name, rendered as a Skill call for the runtime.
	Skill string
	// Because names the situation in the session's own terms.
	Because string
	// match reports whether a command is this moment.
	match func(cmd string) bool
}

// Moments is the closed set. It is deliberately small: every entry costs every
// session that runs its command, and a trigger that fires on work it does not
// bear on is the wallpaper this exists to avoid.
func Moments() []Moment {
	return []Moment{
		{
			Skill:   "mellions-delegation",
			Because: "publishing or merging a change set is the moment its evidence has to be judged, and merging a peer's is not the same act as merging your own",
			match: func(c string) bool {
				return ghSub(c, "pr", "create") || ghSub(c, "pr", "merge") ||
					ghSub(c, "pr", "review") || ghSub(c, "pr", "ready")
			},
		},
		{
			Skill:   "mellions-issue-closure",
			Because: "closing is a claim that the work is done and on the record, and which event it closes on is not always yours to decide",
			match:   func(c string) bool { return ghSub(c, "issue", "close") },
		},
		{
			Skill:   "mellions-issue-creation",
			Because: "an issue is a work contract, and one filed on a premise that has already moved sends the next session at nothing",
			match:   func(c string) bool { return ghSub(c, "issue", "create") },
		},
		{
			Skill:   "mellions-territory",
			Because: "removing or reverting what you did not write is the act that cannot be undone by the session that finds it missing",
			match: func(c string) bool {
				return gitSub(c, "worktree", "remove") || gitSub(c, "branch", "-D") ||
					gitSub(c, "push", "--delete") || gitSub(c, "rm", "") ||
					gitSub(c, "reset", "--hard") || gitSub(c, "checkout", "--")
			},
		},
	}
}

// For is the moments a command is, in declaration order.
func For(command string) []Moment {
	c := strings.Join(strings.Fields(command), " ")
	if c == "" {
		return nil
	}
	var found []Moment
	for _, m := range Moments() {
		if m.match(c) {
			found = append(found, m)
		}
	}
	return found
}

// MomentNotes renders moments as notes, so the once-per-session memory that
// already stops a note repeating stops these too. Ident is the Skill, so each
// is said once however many times its command is run.
func MomentNotes(command string) []Note {
	var notes []Note
	for _, m := range For(command) {
		notes = append(notes, Note{
			Because: m.Because,
			Do:      `Skill(skill: "mellions:` + m.Skill + `")`,
			Ident:   "moment:" + m.Skill,
		})
	}
	return notes
}

// ghSub reports whether a command invokes `gh <noun> <verb>`. The words are
// matched in order rather than adjacently, so global flags between them do not
// hide the call.
func ghSub(c, noun, verb string) bool { return ordered(c, "gh", noun, verb) }

// gitSub is the same for git. An empty verb matches the subcommand alone.
func gitSub(c, noun, verb string) bool {
	if verb == "" {
		return ordered(c, "git", noun)
	}
	return ordered(c, "git", noun, verb)
}

// ordered reports whether one command in the line invokes the tool with these
// words after it, in order.
//
// Segmenting first is what keeps it honest: matching words in order across the
// whole line makes `git status && rm -rf x` read as `git rm`, and a trigger
// that fires on work it does not bear on is the wallpaper this exists to
// avoid. The tool must lead its own segment, so a path or a message that
// happens to contain the word is not the call.
func ordered(c string, words ...string) bool {
	for _, seg := range segments(c) {
		fields := strings.Fields(seg)
		for len(fields) > 0 && strings.Contains(fields[0], "=") {
			fields = fields[1:] // leading NAME=value assignments
		}
		if len(fields) == 0 || fields[0] != words[0] {
			continue
		}
		i := 1
		for _, f := range fields[1:] {
			if i < len(words) && f == words[i] {
				i++
			}
		}
		if i == len(words) {
			return true
		}
	}
	return false
}

// segments splits a command line on the operators that start a new command.
// Quoting is not honoured: an operator inside a string produces an extra
// segment, which can only lose a match, never invent one.
func segments(c string) []string {
	out := []string{c}
	for _, op := range []string{"&&", "||", ";", "|", "\n"} {
		var next []string
		for _, seg := range out {
			next = append(next, strings.Split(seg, op)...)
		}
		out = next
	}
	return out
}

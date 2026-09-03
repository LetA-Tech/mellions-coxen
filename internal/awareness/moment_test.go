// Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
package awareness

import "testing"

func skillsFor(t *testing.T, cmd string) []string {
	t.Helper()
	var got []string
	for _, m := range For(cmd) {
		got = append(got, m.Skill)
	}
	return got
}

func hasSkill(got []string, want string) bool {
	for _, g := range got {
		if g == want {
			return true
		}
	}
	return false
}

// The moments a session actually reaches. Each is a command shape taken from a
// real session rather than invented, because a matcher is only ever wrong on
// the cases nobody would think to write.
func TestTheCommandThatIsTheMomentNamesItsSkill(t *testing.T) {
	for _, c := range []struct{ cmd, skill string }{
		{"gh pr create --base dev --title x --body-file b.md", "mellions-delegation"},
		{"gh pr merge 115 --merge --delete-branch", "mellions-delegation"},
		{"cd /some/tree && gh pr merge 113 --merge", "mellions-delegation"},
		{"gh issue close 42 --comment done", "mellions-issue-closure"},
		{"gh issue create --title x", "mellions-issue-creation"},
		{"git worktree remove /home/you/mellions/assignments/x/tree", "mellions-territory"},
		{"git push origin --delete mellions/some-lane", "mellions-territory"},
		{"git checkout -- internal/foo.go", "mellions-territory"},
		{"git reset --hard origin/dev", "mellions-territory"},
		{"GIT_PAGER=cat git rm docs/old.md", "mellions-territory"},
	} {
		if got := skillsFor(t, c.cmd); !hasSkill(got, c.skill) {
			t.Errorf("%q named %v, want it to name %s", c.cmd, got, c.skill)
		}
	}
}

// The half that decides whether this is a trigger or wallpaper. Every case
// here is a command a long session runs constantly; one that fires on these
// gets ignored, and then it protects nothing.
func TestOrdinaryWorkNamesNothing(t *testing.T) {
	for _, cmd := range []string{
		"git status --short",
		"git log --oneline -5",
		"git diff origin/dev...HEAD",
		"gh pr view 113 --json files",
		"gh issue list --state open",
		"go test ./... -count=1",
		"make verify",
		"rm -rf /tmp/scratch",
		"git add internal/foo.go",
		"git commit -q -F -",
		// The defect this test exists for: words in order across a compound
		// command are not the call.
		"git status && rm -rf /tmp/scratch",
		"git log --oneline | head -5",
		"echo 'gh pr merge' >> notes.md",
		"grep -rn 'git worktree remove' docs/",
	} {
		if got := skillsFor(t, cmd); len(got) != 0 {
			t.Errorf("%q named %v, want nothing — a trigger on ordinary work is ignored, and then it protects nothing", cmd, got)
		}
	}
}

// Ident is what the once-per-session memory keys on, so the same moment
// reached twice is said once.
func TestOneMomentIsSaidOncePerSkill(t *testing.T) {
	first := MomentNotes("gh pr merge 1 --merge")
	again := MomentNotes("gh pr merge 2 --merge")
	if len(first) == 0 || len(again) == 0 {
		t.Fatalf("expected notes for both, got %d and %d", len(first), len(again))
	}
	if first[0].Key() != again[0].Key() {
		t.Errorf("two runs of the same moment key differently (%s vs %s); the second would be said again",
			first[0].Key(), again[0].Key())
	}
	other := MomentNotes("gh issue close 1")
	if len(other) == 0 || other[0].Key() == first[0].Key() {
		t.Errorf("different moments must key differently, or the second is swallowed as already said")
	}
}

// A note with no Do is a note that names a problem and no action.
func TestEveryMomentCarriesTheCallThatLoadsIt(t *testing.T) {
	for _, m := range Moments() {
		notes := MomentNotes(exampleFor(t, m))
		if len(notes) == 0 {
			t.Fatalf("%s: no example command in this test matches its own moment", m.Skill)
		}
		if notes[0].Do == "" || notes[0].Because == "" {
			t.Errorf("%s: note carries Do=%q Because=%q; both are owed", m.Skill, notes[0].Do, notes[0].Because)
		}
	}
}

func exampleFor(t *testing.T, m Moment) string {
	t.Helper()
	switch m.Skill {
	case "mellions-delegation":
		return "gh pr create"
	case "mellions-issue-closure":
		return "gh issue close 1"
	case "mellions-issue-creation":
		return "gh issue create"
	case "mellions-territory":
		return "git worktree remove x"
	}
	t.Fatalf("a moment was added with no example here: %s", m.Skill)
	return ""
}

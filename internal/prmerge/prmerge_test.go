// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prmerge

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func payloadFor(command string) []byte {
	b, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        "/repo",
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		panic(err)
	}
	return b
}

func silent(t *testing.T) func(string, Call) (State, error) {
	t.Helper()
	return func(string, Call) (State, error) { return State{}, nil }
}

func answering(s State) func(string, Call) (State, error) {
	return func(string, Call) (State, error) { return s, nil }
}

// TestTheMergeIsFoundWhateverItIsWrittenBeside is the parse, not the decision.
// A merge buried in a compound command is the one that gets typed at the end of
// a long session, and a matcher that only sees a bare `gh pr merge N` is silent
// exactly there.
func TestTheMergeIsFoundWhateverItIsWrittenBeside(t *testing.T) {
	for _, c := range []struct {
		name     string
		command  string
		want     int
		selector string
		repo     string
	}{
		{"bare", "gh pr merge 177 --squash", 1, "177", ""},
		{"no selector", "gh pr merge --squash --delete-branch", 1, "", ""},
		{"repo flag", "gh pr merge --repo LetA-Tech/hipsys 5", 1, "5", "LetA-Tech/hipsys"},
		{"glued repo", "gh pr merge --repo=LetA-Tech/hipsys 5", 1, "5", "LetA-Tech/hipsys"},
		{"url", "gh pr merge https://github.com/o/r/pull/12 -m", 1, "https://github.com/o/r/pull/12", ""},
		{"after a fetch", "git fetch origin && gh pr merge 8 --merge", 1, "8", ""},
		{"env and absolute path", "GH_TOKEN=x /usr/bin/gh pr merge 7", 1, "7", ""},
		{"not a merge", "gh pr view 5 --json mergeable", 0, "", ""},
		{"merge in someone else's text", `echo "remember to gh pr merge 9"`, 0, "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Calls(c.command)
			if len(got) != c.want {
				t.Fatalf("Calls(%q) found %d merges, want %d", c.command, len(got), c.want)
			}
			if c.want == 0 {
				return
			}
			if got[0].Selector != c.selector {
				t.Errorf("selector = %q, want %q", got[0].Selector, c.selector)
			}
			if got[0].Repo != c.repo {
				t.Errorf("repo = %q, want %q", got[0].Repo, c.repo)
			}
		})
	}
}

// TestAFlagsValueIsNotReadAsTheSelector guards the parse against asking the
// tracker about the wrong pull request. `--subject "merge 999"` carries a
// number, and a parser that takes the first word not starting with a dash
// would look up 999 and then decide about a pull request nobody is merging.
func TestAFlagsValueIsNotReadAsTheSelector(t *testing.T) {
	for _, command := range []string{
		`gh pr merge --subject "999" 5`,
		`gh pr merge -t 999 5`,
		`gh pr merge --body 999 5`,
		`gh pr merge --match-head-commit 999 5`,
	} {
		got := Calls(command)
		if len(got) != 1 {
			t.Fatalf("Calls(%q) found %d merges, want 1", command, len(got))
		}
		if got[0].Selector != "5" {
			t.Errorf("Calls(%q) selector = %q, want \"5\"", command, got[0].Selector)
		}
	}
}

// TestOnlyABashCallIsExamined: the payload for another tool carries no command,
// and reading one out of it would be a decision about nothing.
func TestOnlyABashCallIsExamined(t *testing.T) {
	b, _ := json.Marshal(map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"command": "gh pr merge 1"},
	})
	if got := Deny(b, answering(State{Number: 1, MergeState: "UNKNOWN"})); got != "" {
		t.Fatalf("an Edit payload was refused: %q", got)
	}
}

// TestMergeabilityGitHubHasNotComputedIsRefused is the failure this guard was
// written for. UNKNOWN is not a state the branch is in; it is the absence of an
// answer, reported for a while after a push, which is when a session is most
// likely to be asking.
func TestMergeabilityGitHubHasNotComputedIsRefused(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 177 --squash"),
		answering(State{Number: 177, Base: "dev", MergeState: "UNKNOWN"}))
	if got == "" {
		t.Fatal("a merge with mergeability UNKNOWN was allowed")
	}
	if !strings.Contains(got, "UNKNOWN") {
		t.Errorf("the refusal does not name the state it refuses on: %q", got)
	}
	if !strings.Contains(got, "gh pr view 177") {
		t.Errorf("the refusal does not say how to get a real answer: %q", got)
	}
}

// TestABranchBehindItsBaseInSharedFilesIsRefused is the hazard stated
// concretely: those are the files where one side is about to be written over
// the other.
func TestABranchBehindItsBaseInSharedFilesIsRefused(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 42"), answering(State{
		Number:     42,
		URL:        "https://github.com/o/r/pull/42",
		Base:       "dev",
		MergeState: "CLEAN",
		BehindBy:   10,
		Overlap:    []string{"internal/app/source/recertify.go"},
	}))
	if got == "" {
		t.Fatal("a stale merge overlapping newer work on the base was allowed")
	}
	for _, want := range []string{
		"10 commits behind",
		"internal/app/source/recertify.go",
		"https://github.com/o/r/pull/42",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not carry %q: %q", want, got)
		}
	}
}

// TestBeingBehindOnItsOwnIsNotRefused is the negative that keeps the guard
// alive. Behind is ordinary and usually harmless, and a guard that fires on
// correct work is turned off — after which it protects nothing at all.
func TestBeingBehindOnItsOwnIsNotRefused(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 177"), answering(State{
		Number:     177,
		Base:       "dev",
		MergeState: "CLEAN",
		BehindBy:   10,
	}))
	if got != "" {
		t.Fatalf("a branch behind its base in no shared file was refused: %q", got)
	}
}

// TestAnOverlapWithoutDivergenceIsNotRefused: the same files changed on both
// sides of a branch that is not behind is what a branch cut from the current
// tip looks like. There is nothing older about to overwrite anything.
func TestAnOverlapWithoutDivergenceIsNotRefused(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 8"), answering(State{
		Number:     8,
		Base:       "dev",
		MergeState: "CLEAN",
		BehindBy:   0,
		Overlap:    []string{"internal/cite/cite.go"},
	}))
	if got != "" {
		t.Fatalf("a current branch was refused: %q", got)
	}
}

// TestACleanMergeIsSilent: the ordinary case has to cost nothing, or the guard
// becomes the wallpaper it exists to avoid.
func TestACleanMergeIsSilent(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 3 --squash --delete-branch"),
		answering(State{Number: 3, Base: "dev", MergeState: "CLEAN"}))
	if got != "" {
		t.Fatalf("a clean, current merge was refused: %q", got)
	}
}

// TestWhatCannotBeEstablishedIsSilent. A deny on a guess blocks legitimate work
// in a repository this cannot read, and that is how a guard gets removed.
func TestWhatCannotBeEstablishedIsSilent(t *testing.T) {
	t.Run("the tracker could not answer", func(t *testing.T) {
		look := func(string, Call) (State, error) { return State{}, errNotFound{} }
		if got := Deny(payloadFor("gh pr merge 1"), look); got != "" {
			t.Fatalf("refused on an unreadable tracker: %q", got)
		}
	})
	t.Run("no pull request", func(t *testing.T) {
		if got := Deny(payloadFor("gh pr merge 1"), silent(t)); got != "" {
			t.Fatalf("refused on a selector naming nothing: %q", got)
		}
	})
}

// TestATruncatedComparisonIsRefusedRatherThanReadAsClean. GitHub pages the
// files in a comparison, so at the cap an empty overlap is not evidence of
// none — and this is the one unknown that is refused rather than passed,
// because a branch that far behind is reconciled rather than merged on the
// strength of git being able to resolve it.
func TestATruncatedComparisonIsRefusedRatherThanReadAsClean(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 4"), answering(State{
		Number:     4,
		Base:       "main",
		MergeState: "CLEAN",
		BehindBy:   400,
		Truncated:  true,
	}))
	if got == "" {
		t.Fatal("a comparison too large to enumerate was read as no overlap")
	}
	if !strings.Contains(got, "could not be established") {
		t.Errorf("the refusal claims more than it established: %q", got)
	}
}

// TestTheRefusalSaysWhatItCannotSee. The matcher is files; the hazard is
// meaning. A reader who takes a clean answer here for a review has been misled
// by this guard, so the refusal that does fire says so in its own text.
func TestTheRefusalSaysWhatItCannotSee(t *testing.T) {
	got := Deny(payloadFor("gh pr merge 42"), answering(State{
		Number: 42, Base: "dev", MergeState: "CLEAN",
		BehindBy: 2, Overlap: []string{"a.go"},
	}))
	if !strings.Contains(got, "not a review") {
		t.Errorf("the refusal does not say a clean answer is not a review: %q", got)
	}
}

// TestTheOverlapListIsBoundedAndSaysSo: a screen of paths is not more evidence
// than ten of them, but silently showing ten of two hundred would be.
func TestTheOverlapListIsBoundedAndSaysSo(t *testing.T) {
	var many []string
	for i := 0; i < overlapNamed+7; i++ {
		many = append(many, "pkg/file"+strconv.Itoa(i)+".go")
	}
	got := Deny(payloadFor("gh pr merge 9"), answering(State{
		Number: 9, Base: "dev", MergeState: "CLEAN", BehindBy: 3, Overlap: many,
	}))
	if strings.Count(got, "pkg/file") > overlapNamed+1 {
		t.Errorf("the refusal named more than %d paths", overlapNamed)
	}
	if !strings.Contains(got, "7 mores") && !strings.Contains(got, "and 7 more") {
		t.Errorf("the refusal does not say how many it did not name: %q", got)
	}
}

type errNotFound struct{}

func (errNotFound) Error() string { return "no pull request found" }

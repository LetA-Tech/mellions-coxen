// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShiftPromptAsksForSubagents: a runtime may tell a session not to call the
// Agent tool unless the user requested it. The shift prompt is the user turn,
// so it is where the request has to be, and it has to be in the part both a
// survey shift and a dispatched-task shift receive — the preamble before
// `if [ -n "$TASK" ]`, not either branch.
func TestShiftPromptAsksForSubagents(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "shift.sh"))
	if err != nil {
		t.Fatalf("scripts/shift.sh: %v", err)
	}
	src := string(b)

	// The same `if [ -n "$TASK" ]` guards the survey step earlier in the
	// script, so the preamble is bounded from where the prompt itself starts.
	start := strings.Index(src, "You are running UNATTENDED")
	if start < 0 {
		t.Fatal(`scripts/shift.sh: no "You are running UNATTENDED" — the prompt moved, and this guard no longer knows where it begins`)
	}
	rest := strings.Index(src[start:], `if [ -n "$TASK" ]; then`)
	if rest < 0 {
		t.Fatal(`scripts/shift.sh: no 'if [ -n "$TASK" ]; then' after the prompt's opening — the two branches moved, and this guard no longer knows where the common preamble ends`)
	}
	preamble := src[start : start+rest]

	for _, want := range []string{
		"Subagents are yours to dispatch",
		"Agent(*)",
		"unless the user requested it",
	} {
		if !strings.Contains(preamble, want) {
			t.Errorf("the shift prompt's common preamble does not contain %q — an unattended session is left to infer whether it may dispatch, and on a host whose runtime says not to unless asked, it will not", want)
		}
	}
}

// TestShiftPromptRefusesOwnLaneWhenChoosing: the prompt hands the review method
// to `mellions-delegation`, which is loaded before the diff is opened — later
// than the point where a session picks which pull request to review. Every
// other review rule can arrive with the Skill, because none of them can bind
// before a diff is open. This one binds earlier, so the choose-your-own-work
// branch has to carry it itself.
//
// Scoped to that branch: a shift given a task does not choose, and the survey
// attributes every Mellions pull request to one author, so nothing else in the
// prompt distinguishes a session's own lane from a peer's.
func TestShiftPromptRefusesOwnLaneWhenChoosing(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "shift.sh"))
	if err != nil {
		t.Fatalf("scripts/shift.sh: %v", err)
	}
	src := string(b)

	start := strings.Index(src, "You are running UNATTENDED")
	if start < 0 {
		t.Fatal(`scripts/shift.sh: no "You are running UNATTENDED" — the prompt moved, and this guard no longer knows where it begins`)
	}
	open := strings.Index(src[start:], `if [ -n "$TASK" ]; then`)
	if open < 0 {
		t.Fatal(`scripts/shift.sh: no 'if [ -n "$TASK" ]; then' after the prompt's opening — the two branches moved, and this guard no longer knows where they are`)
	}
	rel := strings.Index(src[start+open:], "\n  else\n")
	if rel < 0 {
		t.Fatal(`scripts/shift.sh: the prompt's task branch has no '  else' — this guard no longer knows where the choose-your-own-work branch begins`)
	}
	end := strings.Index(src[start+open+rel:], "\n  fi\n")
	if end < 0 {
		t.Fatal(`scripts/shift.sh: the prompt's choose-your-own-work branch is unterminated — this guard no longer knows where it ends`)
	}
	choosing := src[start+open+rel : start+open+rel+end]

	at := strings.Index(choosing, "own lane")
	if at < 0 {
		t.Error("the shift prompt's choose-your-own-work branch does not say \"own lane\" — an unattended session picks a pull request before it loads `mellions-delegation`, so nothing tells it at that moment that its own lane's draft is not a peer's work; it claims the lane, opens the diff, and only then reads the rule that says it may not be the one to judge it")
		return
	}

	// The words are not the rule; the refusal is. A guard that demands only
	// "own lane" is satisfied by a sentence that mentions the lane and permits
	// it, so the sentence carrying the phrase has to withhold it.
	sentence := choosing
	if from := strings.LastIndex(choosing[:at], ". "); from >= 0 {
		sentence = choosing[from+2:]
	}
	if to := strings.Index(sentence, "."); to >= 0 {
		sentence = sentence[:to+1]
	}
	lower := strings.ToLower(sentence)
	if !strings.Contains(lower, "never") && !strings.Contains(lower, "not") {
		t.Errorf("the shift prompt's choose-your-own-work branch mentions the own lane without refusing it — %q withholds nothing, and a session reading it picks its own draft anyway; the sentence carrying \"own lane\" has to say the session may not take one", sentence)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"strings"
	"testing"
)

// The roster is read at the moment work is chosen, and saying only who is here
// left what that reserved to be filled in. Three survey shifts on this host
// filled it in with the repository in one afternoon — 20260828-152536 passed
// over mellions-coxen #35/#40/#42/#46/#47 as "live peer session in that repo
// — territory", 20260828-142752 over "mellions-coxen and frontend-app work
// (both repos had live sessions)", 20260828-181004 over both as "held by live
// sessions" — while directed shifts took that same work with the same peer
// present, and 20260828-164416 took #35 itself.
//
// This is the block those sessions actually read: the awareness note is wired
// to UserPromptSubmit and PreToolUse, so a headless shift never renders it, and
// none of the four streams contains one.
func TestTheRosterSaysWhatAPeerDoesNotReserve(t *testing.T) {
	// Literals, not a second call into the renderer: what a session reads is
	// the thing under test, so the oracle must not move with it.
	for _, want := range []string{
		"None of this reserves a repository",
		"worktree of its own",
		"listed with no assignment holds no lane",
		"claim on the issue",
		"conclusion that stops the work",
	} {
		if !strings.Contains(whoTrailer, want) {
			t.Errorf("the roster trailer does not say %q, so passing over a repository\n"+
				"because somebody is in it stays the available reading:\n%s", want, whoTrailer)
		}
	}
	// What it must not lose: the older warning it is appended to.
	if !strings.Contains(whoTrailer, "absence is not proof of an empty tree") {
		t.Error("the trailer dropped the warning that an unregistered session is not an empty tree")
	}
}

// The claim the trailer makes about the assignment clause has to be true of the
// code that writes it, or the sentence teaches a session to read an absence
// that means nothing. cmdHere sets Assignment only when the session's tree is
// an open assignment's worktree (who.go), and Describe prints the clause only
// when it is set (internal/presence).
func TestASessionListedWithNoAssignmentReallyHoldsNoLane(t *testing.T) {
	if !strings.Contains(whoTrailer, "listed with no assignment holds no lane") {
		t.Skip("the trailer no longer makes this claim")
	}
	for _, want := range []string{"rec.Assignment = a.ID", "sameTree(a.Worktree, here)"} {
		if !strings.Contains(whoRegistrationSource(t), want) {
			t.Errorf("cmdHere no longer binds the assignment to the worktree (%q missing),\n"+
				"so the trailer's claim about an absent assignment clause is no longer true", want)
		}
	}
}

func whoRegistrationSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("who.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

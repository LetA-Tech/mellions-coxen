// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// claimRepo builds an actual git repository, because Open cuts a real worktree
// from it and a fake would test the fake.
func claimRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	return dir
}

// claimed opens a lane and leaves it in the given state.
func claimed(t *testing.T, state string) (*assignment.Store, string) {
	t.Helper()
	store, err := assignment.NewStore(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(assignment.OpenOptions{
		ID: "probe-1", Repo: "probe-repo", Objective: "the work",
		Because: "the owner asked for it", Source: claimRepo(t),
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if state != assignment.StateActive {
		if err := store.Handoff("probe-1", "where it stands"); err != nil {
			t.Fatalf("Handoff: %v", err)
		}
	}
	if state == assignment.StateClosed {
		if err := store.Close("probe-1"); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
	return store, "probe-1"
}

// TestAssignOpenClaimsWorkThatAlreadyExists.
//
// Every dispatch says "claim it with mellions assign open <id>", and for a lane
// that had been handed off that instruction could not be executed at all. Open
// checked the record last, so it refused three times in sequence — for a
// repository that was on the command line, then for an objective the record
// already held, and only then for existing. Each message named a missing input,
// which reads as "supply more" when no set of flags would have worked.
func TestAssignOpenClaimsWorkThatAlreadyExists(t *testing.T) {
	store, id := claimed(t, assignment.StateHandedOff)

	// No repository, no objective, no reason: everything the record holds.
	_, handled, err := claimExisting(store, id)
	if err != nil {
		t.Fatalf("claiming handed-off work: %v", err)
	}
	if !handled {
		t.Fatal("claimExisting passed a handed-off lane through to Open, which refuses it")
	}
	a, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != assignment.StateActive {
		t.Errorf("state = %q after claiming it, want %q", a.State, assignment.StateActive)
	}
}

// TestAssignOpenOnActiveWorkIsNotARefusal. A lane you already hold is claimed.
// The record prints who last worked it, so a second session meets the collision
// in the text rather than as an exit code that says a flag is missing.
func TestAssignOpenOnActiveWorkIsNotARefusal(t *testing.T) {
	store, id := claimed(t, assignment.StateActive)

	_, handled, err := claimExisting(store, id)
	if err != nil {
		t.Fatalf("claiming active work: %v", err)
	}
	if !handled {
		t.Fatal("claimExisting passed an active lane through to Open, which refuses it")
	}
}

// TestAssignOpenSaysWhyClosedWorkCannotBeClaimed. Reopen owns which states may
// be taken up; claiming must not route around it and must not report a closed
// lane as a missing objective.
func TestAssignOpenSaysWhyClosedWorkCannotBeClaimed(t *testing.T) {
	store, id := claimed(t, assignment.StateClosed)

	_, handled, err := claimExisting(store, id)
	if !handled {
		t.Fatal("claimExisting passed a closed lane through to Open")
	}
	if err == nil {
		t.Fatal("claiming closed work succeeded; a closed lane is not reopened")
	}
	if !strings.Contains(err.Error(), assignment.StateClosed) {
		t.Errorf("error does not say the lane is closed: %v", err)
	}
	if strings.Contains(err.Error(), "objective") {
		t.Errorf("error names a missing input rather than the state: %v", err)
	}
}

// TestAssignOpenStillOpensNewWork. The claim path must not swallow an id that
// has no record: that is the ordinary open, and it needs the repository, the
// objective and the reason it always did.
func TestAssignOpenStillOpensNewWork(t *testing.T) {
	store, err := assignment.NewStore(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	_, handled, err := claimExisting(store, "never-opened")
	if err != nil {
		t.Fatalf("looking up work that does not exist: %v", err)
	}
	if handled {
		t.Fatal("claimExisting handled an id with no record, so no new lane can be opened")
	}
}

// An unknown assign verb names the valid alternatives so the caller can recover.
func TestAssignRefusalsNameTheVerbs(t *testing.T) {
	err := cmdAssign(context.Background(), []string{"quixotic"})
	if err == nil {
		t.Fatal("an unknown verb was accepted")
	}
	for _, verb := range strings.Split(assignVerbs, ", ") {
		if !strings.Contains(err.Error(), verb) {
			t.Errorf("refusal does not name %q: %v", verb, err)
		}
	}
}

// Show is accepted as the common read synonym for get.
func TestAssignShowIsGet(t *testing.T) {
	if err := cmdAssign(context.Background(), []string{"show"}); err != nil && strings.Contains(err.Error(), "unknown verb") {
		t.Fatalf("show is still refused as unknown: %v", err)
	}
}

// TestAssignKindsAreNotVerbs. One session wrote `assign note`, which is a -kind
// value; the refusal should say where it belongs rather than list verbs it is
// not among.
func TestAssignKindsAreNotVerbs(t *testing.T) {
	for _, kind := range []string{"note", "found", "hypothesis", "next"} {
		err := cmdAssign(context.Background(), []string{kind})
		if err == nil {
			t.Fatalf("assign %s was accepted as a verb", kind)
		}
		if !strings.Contains(err.Error(), "-kind "+kind) {
			t.Errorf("assign %s does not say where the kind belongs: %v", kind, err)
		}
	}
}

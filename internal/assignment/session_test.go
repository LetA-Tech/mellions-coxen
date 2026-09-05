// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestTheRuntimeSessionIsRecordedWithoutBeingAskedTo is the property that makes
// native resume reachable at all.
//
// It has to happen on every write rather than on a command, because the session
// this matters for is the one that died mid-thought — and that one never
// reached whatever call it was supposed to make.
func TestTheRuntimeSessionIsRecordedWithoutBeingAskedTo(t *testing.T) {
	noAmbientSession(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "a5993f61-648d-4fa7-aa63-95993c63aee5")
	t.Setenv("CODEX_SESSION_ID", "")

	s := fakeStore(t)
	a := open(t, s)
	if err := s.Record(a.ID, "found", "the dedup key drops the value date"); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1 — two writes from one session are one session", len(got.Sessions))
	}
	sess, ok := got.Latest()
	if !ok {
		t.Fatal("no latest session")
	}
	if sess.Runtime != "claude" || sess.ID != "a5993f61-648d-4fa7-aa63-95993c63aee5" {
		t.Fatalf("session = %+v", sess)
	}
	if want := "claude --resume a5993f61-648d-4fa7-aa63-95993c63aee5"; sess.Resume() != want {
		t.Errorf("Resume() = %q, want %q", sess.Resume(), want)
	}
	if !sess.Last.After(sess.First) && !sess.Last.Equal(sess.First) {
		t.Error("Last must move forward as the session keeps working")
	}
}

// TestASecondRuntimeIsASecondSession: the same work picked up in Codex after
// Claude died is exactly the case §8 names, and both handles have to survive.
func TestASecondRuntimeIsASecondSession(t *testing.T) {
	noAmbientSession(t)
	s := fakeStore(t)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-one")
	a := open(t, s)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "019fbfeb-86f8-7710-a4f1-6b473eb44fb7")
	if err := s.Record(a.ID, "next", "falsify against the three known rows"); err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(a.ID)
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(got.Sessions))
	}
	latest, _ := got.Latest()
	if latest.Runtime != "codex" {
		t.Fatalf("latest runtime = %q, want codex", latest.Runtime)
	}
	if !strings.HasPrefix(latest.Resume(), "codex resume ") {
		t.Errorf("Resume() = %q", latest.Resume())
	}
}

// TestOutsideASessionNothingIsInvented. A terminal, a timer and CI are
// supported ways to run. Recording a session id there would mean recording a
// resume command that does not resume anything.
func TestOutsideASessionNothingIsInvented(t *testing.T) {
	noAmbientSession(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	s := fakeStore(t)
	a := open(t, s)
	got, _ := s.Get(a.ID)
	if len(got.Sessions) != 0 {
		t.Fatalf("sessions = %+v, want none outside a runtime session", got.Sessions)
	}
	if _, ok := got.Latest(); ok {
		t.Error("Latest reported a session that does not exist")
	}
}

// TestWorkingNotesAreRenderedAsHistory is §5 at the smallest scale.
//
// The notes are the engineer's own words about the world, written at a moment
// that has passed. Rendering them in the same voice as current fact is how "PR
// #421 is open" becomes a premise nobody checked.
func TestWorkingNotesAreRenderedAsHistory(t *testing.T) {
	noAmbientSession(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s-1")
	s := fakeStore(t)
	a := open(t, s)
	if err := s.Record(a.ID, "found", "PR #421 is open against dev"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(a.ID)
	text := got.Text(time.Now().Add(3 * time.Hour))

	for _, want := range []string{
		"as they stood",
		"true when it was written",
		"re-established",
		"mellions continue",
		"claude --resume s-1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the rendered assignment never says %q:\n%s", want, text)
		}
	}
}

// noAmbientSession removes both runtime handles before a test declares the ones
// it means to test with.
//
// These tests run inside a real Claude or Codex session, which exports its own
// handle. Setting only the variable a case cares about leaves the other one
// live, and the assertion then counts the parent session as a recorded one —
// which reads as a production defect in session recording and is not.
func noAmbientSession(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
}

// fakeStore avoids touching a real repository: worktree mechanics are not what
// these assert, and a test that creates worktrees in the developer's checkout
// leaves branches behind.
func fakeStore(t *testing.T) *Store {
	t.Helper()
	s, err := newStoreT(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Command-aware rather than one answer for everything. A double that
	// reports every ref as present tells Open that the branch it is about to
	// cut already exists, which is both false and the opposite of what these
	// fixtures mean.
	s.Git = func(_ string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--verify" {
			return nil, errors.New("fatal: Needed a single revision")
		}
		return []byte("abc1234\n"), nil
	}
	return s
}

func open(t *testing.T, s *Store) *Assignment {
	t.Helper()
	a, err := s.Open(OpenOptions{
		ID: "fg-1", Repo: "analytics-service", Issue: "#340",
		Objective: "duplicate settlement rows",
		Because:   "it is the only defect reaching a balance sheet",
		Source:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// TestEverySessionSurvivesNewestFirst.
//
// The newest handle is the one most likely to fail — a transcript past its
// retention sweep, a machine that is not this one, a runtime that is not the
// one running now. A reader handed only that gets a dead command and nothing
// to try next.
func TestEverySessionSurvivesNewestFirst(t *testing.T) {
	noAmbientSession(t)
	s := fakeStore(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "first")
	a := open(t, s)

	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "second")
	if err := s.Record(a.ID, "note", "picked up in codex"); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "third")
	if err := s.Record(a.ID, "note", "and back again"); err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(a.ID)
	by := got.SessionsByRecency()
	if len(by) != 3 {
		t.Fatalf("sessions = %d, want all three kept", len(by))
	}
	want := []string{"third", "second", "first"}
	for i, id := range want {
		if by[i].ID != id {
			t.Fatalf("position %d = %q, want %q — newest first", i, by[i].ID, id)
		}
	}
	if by[0].Resume() != "claude --resume third" || by[1].Resume() != "codex resume second" {
		t.Errorf("resume commands do not follow the runtime: %q, %q",
			by[0].Resume(), by[1].Resume())
	}
}

// TestANestedSessionKeepsBothHandles.
//
// Starting Codex from inside a Claude session — which is how cross-runtime work
// actually happens — leaves both variables set in the same process. Nothing in
// the environment says which runtime is the inner one, so recording a single
// handle means recording, half the time, a resume command that reopens a
// different conversation.
func TestANestedSessionKeepsBothHandles(t *testing.T) {
	noAmbientSession(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "outer-claude")
	t.Setenv("CODEX_SESSION_ID", "inner-codex")

	s := fakeStore(t)
	a := open(t, s)
	got, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions = %+v, want both handles", got.Sessions)
	}
	seen := map[string]string{}
	for _, sess := range got.Sessions {
		seen[sess.Runtime] = sess.Resume()
	}
	if seen["claude"] != "claude --resume outer-claude" || seen["codex"] != "codex resume inner-codex" {
		t.Fatalf("resume commands = %v", seen)
	}
}

package awareness_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

func TestASessionInTheSameTreeComesFirst(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{
		Tree: "/w/a", Repo: "r", Branch: "b", Idle: true,
		Others:    []awareness.Peer{{Describe: "claude session ab12 on r (b)", Resume: "claude --resume ab12"}},
		Elsewhere: []awareness.Peer{{Describe: "claude session cd34 on r (c)"}},
	})
	if len(notes) != 3 {
		t.Fatalf("want 3 notes, got %+v", notes)
	}
	if !strings.Contains(notes[0].Because, "in this working tree") || !strings.Contains(notes[0].Because, "ab12") {
		t.Errorf("first note = %q", notes[0].Because)
	}
	if !strings.Contains(notes[0].Do, "claude --resume ab12") {
		t.Errorf("the note does not say how to reach the peer: %q", notes[0].Do)
	}
	if !strings.Contains(notes[1].Because, "from another tree") {
		t.Errorf("second note = %q", notes[1].Because)
	}
}

func TestAQuietSituationSaysNothing(t *testing.T) {
	if n := awareness.Notes(awareness.Observation{Tree: "/w", Repo: "r"}); len(n) != 0 {
		t.Fatalf("a session with nothing to be told was told %+v", n)
	}
}

func TestIdleHandsTheSurveyWhenOneIsFresh(t *testing.T) {
	with := awareness.Notes(awareness.Observation{Idle: true, Survey: "/m/survey.md", SurveyBrief: "12 signals."})
	if len(with) != 1 || !strings.Contains(with[0].Do, "/m/survey.md") {
		t.Fatalf("got %+v", with)
	}
	without := awareness.Notes(awareness.Observation{Idle: true})
	if len(without) != 1 || without[0].Do != "mellions survey" {
		t.Fatalf("got %+v", without)
	}
}

func TestSaidDeliversEachFactOnce(t *testing.T) {
	root := t.TempDir()
	said := awareness.Said{Path: awareness.SaidPath(root, "claude", "sess-1")}
	notes := awareness.Notes(awareness.Observation{Idle: true})
	fresh := said.Fresh(notes)
	if len(fresh) != 1 {
		t.Fatalf("first delivery: %d", len(fresh))
	}
	if err := said.Remember(fresh); err != nil {
		t.Fatal(err)
	}
	if again := said.Fresh(notes); len(again) != 0 {
		t.Fatalf("delivered twice: %+v", again)
	}
	// A different session is told afresh.
	other := awareness.Said{Path: awareness.SaidPath(root, "claude", "sess-2")}
	if n := other.Fresh(notes); len(n) != 1 {
		t.Fatalf("another session was silenced by the first: %d", len(n))
	}
}

func TestPruneRemovesOnlyWhatIsOldEnough(t *testing.T) {
	root := t.TempDir()
	old := awareness.Said{Path: awareness.SaidPath(root, "claude", "old")}
	recent := awareness.Said{Path: awareness.SaidPath(root, "claude", "recent")}
	notes := awareness.Notes(awareness.Observation{Idle: true})
	for _, s := range []awareness.Said{old, recent} {
		if err := s.Remember(notes); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(old.Path, past, past); err != nil {
		t.Fatal(err)
	}
	awareness.Prune(root, 7*24*time.Hour, time.Now())
	if _, err := os.Stat(old.Path); err == nil {
		t.Error("an old session's memory survived")
	}
	if _, err := os.Stat(filepath.Join(root, "awareness")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(recent.Path); err != nil {
		t.Error("a recent session's memory was removed")
	}
}

// A session standing in the source checkout is told so. Three sessions on this
// host ran `git checkout -b` there in one day, left it on a lane branch, and
// nothing in the tool observed it: Observation carried Tree and Branch and
// Notes read neither.
func TestSourceCheckoutIsAnnounced(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{Tree: "/src", Repo: "mellions-coxen", Branch: "dev",
		Source: true, Tracking: true})
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1 naming the source checkout", len(notes))
	}
	// The oracle is a literal, not another call into the renderer: a test that
	// asserted against Notes' own output could not fail for any change to it.
	for _, want := range []string{"source checkout", "mellions assign open", "-repo mellions-coxen"} {
		if !strings.Contains(notes[0].Because+notes[0].Do+notes[0].Why, want) {
			t.Errorf("source-checkout note does not mention %q:\n%+v", want, notes[0])
		}
	}
}

// The already-broken state — source checkout sitting on a branch with no
// upstream — is named for what it costs, because that is what baseFor cuts the
// next lane from.
func TestSourceCheckoutOnALaneBranchNamesTheCost(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{Tree: "/src", Repo: "mellions-coxen",
		Branch: "mellions/oracle-independence-0828", Source: true, Tracking: false})
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	all := notes[0].Because + notes[0].Do + notes[0].Why
	for _, want := range []string{"mellions/oracle-independence-0828", "tracks no remote branch", "local HEAD"} {
		if !strings.Contains(all, want) {
			t.Errorf("lane-branch note does not mention %q:\n%+v", want, notes[0])
		}
	}
}

// A lane worktree is not the source checkout, and must stay silent.
func TestLaneWorktreeSaysNothing(t *testing.T) {
	if n := awareness.Notes(awareness.Observation{Tree: "/lane", Repo: "mellions-coxen",
		Branch: "mellions/x", Source: false}); len(n) != 0 {
		t.Errorf("a lane worktree got %d notes, want 0: %+v", len(n), n)
	}
}

// A peer in the repository is not the repository being taken. Survey shifts on
// this host passed over every mellions-coxen issue for a whole afternoon —
// "live peer session in that repo — territory" — while the peer sat in the
// source checkout holding nothing any lane needs, and directed shifts took the
// same work with the same peer present. The note said a session was there and
// said nothing about what that reserved, so the reading that stops the work was
// the one available to make.
func TestThePeerNoteSaysAPeerDoesNotReserveTheRepository(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{Tree: "/lane", Repo: "mellions-coxen",
		Elsewhere: []awareness.Peer{{Describe: "claude session 2cdc on mellions-coxen (dev)"}}})
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1 naming the peer", len(notes))
	}
	whole := notes[0].Because + notes[0].Do + notes[0].Why
	// Literals, not another call into the renderer: what a session reads is the
	// thing under test, so the oracle cannot be allowed to move with it.
	for _, want := range []string{
		"does not reserve",
		"not a reason to pass",
		"worktree of its own",
		"claim on the issue",
	} {
		if !strings.Contains(whole, want) {
			t.Errorf("the peer note does not say %q, so passing over the repository stays\n"+
				"the available reading:\n%+v", want, notes[0])
		}
	}
}

// The tree a session is told about is one it is typing a path into, so the
// note names the path rather than "this tree", and names the lane it should
// have been in when it has one.
func TestTheSourceNoteNamesTheTreeAndTheLane(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{Tree: "/home/you/workspace/data-service",
		Repo: "data-service", Branch: "dev", Source: true, Tracking: true,
		Lane: "/home/you/mellions/assignments/data-42/tree"})
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1", len(notes))
	}
	all := notes[0].Because + notes[0].Do + notes[0].Why
	for _, want := range []string{"/home/you/workspace/data-service", "/home/you/mellions/assignments/data-42/tree"} {
		if !strings.Contains(all, want) {
			t.Errorf("the source-checkout note does not name %q:\n%+v", want, notes[0])
		}
	}
}

// A command can reach a shared checkout while its cwd remains elsewhere.
func TestReachingIntoASharedCheckoutIsAnnounced(t *testing.T) {
	notes := awareness.Notes(awareness.Observation{Tree: "/home/you/mellions",
		Reaching: "/home/you/workspace/data-service", ReachingRepo: "data-service",
		ReachingLane: "/home/you/mellions/assignments/data-42/tree"})
	if len(notes) != 1 {
		t.Fatalf("got %d notes, want 1 naming the tree reached into: %+v", len(notes), notes)
	}
	all := notes[0].Because + notes[0].Do + notes[0].Why
	for _, want := range []string{
		"/home/you/workspace/data-service",
		"every lane on this host is cut from",
		"git -C /home/you/workspace/data-service show <rev>:<path>",
		"/home/you/mellions/assignments/data-42/tree",
		"has not committed",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("the reaching note does not say %q:\n%+v", want, notes[0])
		}
	}
}

// Reaching nowhere says nothing: a session working in its own lane must not
// collect a note for every command it runs.
func TestReachingNowhereSaysNothing(t *testing.T) {
	if n := awareness.Notes(awareness.Observation{Tree: "/home/you/mellions/assignments/x/tree",
		Repo: "data-service", Branch: "mellions/x"}); len(n) != 0 {
		t.Errorf("got %d notes, want 0: %+v", len(n), n)
	}
}

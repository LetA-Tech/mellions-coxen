package presence

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestLiveReportsOnlySessionsWhoseProcessExists(t *testing.T) {
	root := t.TempDir()
	s := Store{Root: root}
	now := time.Now().UTC()
	me := Session{ID: "live-1", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a", Repo: "r", Seen: now}
	gone := Session{ID: "gone-1", Runtime: "claude", PID: 1 << 22, Cwd: "/w/b", Repo: "r", Seen: now}
	for _, sess := range []Session{me, gone} {
		if err := s.Register(sess); err != nil {
			t.Fatal(err)
		}
	}
	live := s.Live()
	if len(live) != 1 || live[0].ID != "live-1" {
		t.Fatalf("live = %+v, want only live-1", live)
	}
	if n := len(s.All()); n != 2 {
		t.Fatalf("all = %d, want 2", n)
	}
}

func TestRegisterKeepsTheFirstStart(t *testing.T) {
	s := Store{Root: t.TempDir()}
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.Register(Session{ID: "x", PID: os.Getpid(), Seen: first}); err != nil {
		t.Fatal(err)
	}
	later := first.Add(time.Hour)
	if err := s.Register(Session{ID: "x", PID: os.Getpid(), Seen: later, Branch: "b"}); err != nil {
		t.Fatal(err)
	}
	got := s.All()
	if len(got) != 1 || !got[0].Started.Equal(first) || got[0].Branch != "b" {
		t.Fatalf("got %+v", got)
	}
}

func TestPruneRemovesOnlyEndedSessions(t *testing.T) {
	s := Store{Root: t.TempDir()}
	old := time.Now().Add(-48 * time.Hour)
	s.Register(Session{ID: "ended", PID: 1 << 22, Seen: old})
	s.Register(Session{ID: "running", PID: os.Getpid(), Seen: old})
	s.Prune(time.Now(), 24*time.Hour)
	all := s.All()
	if len(all) != 1 || all[0].ID != "running" {
		t.Fatalf("after prune: %+v", all)
	}
}

// A pid outlives the process that held it. The store is full of records whose
// pid was reissued — every one of them, after a reboot — and judging liveness
// on the number alone reports an ended session as a working peer for as long as
// anything holds its number.
func TestLiveIgnoresARecordWhosePidWasReissued(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Now().UTC()
	mine := procStarts()[os.Getpid()]
	if mine.IsZero() {
		t.Fatal("cannot read this process's start time")
	}
	if err := s.Register(Session{ID: "running", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a", Seen: now}); err != nil {
		t.Fatal(err)
	}
	// An ended session that registered eight hours before this process began,
	// and whose pid this process was later given.
	ended := Session{ID: "reissued", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/b",
		ProcStarted: mine.Add(-8 * time.Hour), Started: now.Add(-8 * time.Hour), Seen: now.Add(-8 * time.Hour)}
	if err := s.Register(ended); err != nil {
		t.Fatal(err)
	}
	live := s.Live()
	if len(live) != 1 || live[0].ID != "running" {
		t.Fatalf("live = %+v, want only running", live)
	}
}

// The same is true with nothing but the old fields to go on: a process that
// began after a record was written cannot be the process that wrote it.
func TestLiveIgnoresAReissuedPidInARecordWrittenBeforeProcessesWereIdentified(t *testing.T) {
	s := Store{Root: t.TempDir()}
	legacy := Session{ID: "legacy", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/b",
		Started: time.Now().UTC().Add(-8 * time.Hour), Seen: time.Now().UTC().Add(-8 * time.Hour)}
	raw, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file(legacy.ID), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.All(); len(got) != 1 || !got[0].ProcStarted.IsZero() {
		t.Fatalf("fixture is not a record without a process start: %+v", got)
	}
	if live := s.Live(); len(live) != 0 {
		t.Fatalf("live = %+v, want none", live)
	}
}

// A reopened conversation is handed a second session id for the same process.
// Two records for one session make it its own peer, count it twice, and split
// the record the turn hooks refresh from the id a person is told to resume.
func TestOneProcessHoldsOneRecordUnderTheEarlierId(t *testing.T) {
	s := Store{Root: t.TempDir()}
	first := time.Now().UTC().Add(-8 * time.Hour)
	if err := s.Register(Session{ID: "first-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a", Seen: first}); err != nil {
		t.Fatal(err)
	}
	// The runtime reopens the conversation and the session-start hook registers
	// the same process under a fresh id.
	if err := s.Register(Session{ID: "second-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a",
		Branch: "b", Seen: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	all := s.All()
	if len(all) != 1 {
		t.Fatalf("all = %+v, want one record for one process", all)
	}
	if all[0].ID != "first-id" {
		t.Fatalf("surviving id = %q, want first-id — the id the runtime keeps using and a person resumes", all[0].ID)
	}
	if all[0].Branch != "b" {
		t.Fatalf("record was not refreshed: %+v", all[0])
	}
	if !all[0].Started.Equal(first) {
		t.Fatalf("started = %s, want %s", all[0].Started, first)
	}
	if len(s.Live()) != 1 {
		t.Fatalf("live = %+v, want one", s.Live())
	}
}

// Registration order is the runtime's, not ours: the fresh id may arrive first.
func TestFoldingDoesNotDependOnWhichIdRegistersFirst(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Now().UTC()
	if err := s.Register(Session{ID: "second-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a", Seen: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.Register(Session{ID: "first-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a",
		Started: now.Add(-8 * time.Hour), Seen: now}); err != nil {
		t.Fatal(err)
	}
	all := s.All()
	if len(all) != 1 || all[0].ID != "first-id" {
		t.Fatalf("all = %+v, want only first-id", all)
	}
}

// Prune only removes what it can see has ended. A record kept alive by a
// reissued pid is never reclaimed, so the store grows a permanent population of
// sessions that finished.
func TestPruneReclaimsARecordWhosePidWasReissued(t *testing.T) {
	s := Store{Root: t.TempDir()}
	mine := procStarts()[os.Getpid()]
	old := time.Now().Add(-48 * time.Hour)
	if err := s.Register(Session{ID: "reissued", PID: os.Getpid(), ProcStarted: mine.Add(-72 * time.Hour), Seen: old}); err != nil {
		t.Fatal(err)
	}
	s.Prune(time.Now(), 24*time.Hour)
	if all := s.All(); len(all) != 0 {
		t.Fatalf("after prune: %+v, want empty", all)
	}
}

// writeLegacy puts a record in the store as it was written before a record
// could say which process wrote it: a pid, and nothing that says whose.
func writeLegacy(t *testing.T, s Store, sess Session) {
	t.Helper()
	sess.ProcStarted = time.Time{}
	raw, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file(sess.ID), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A store that has been in use already holds pairs: one process, two ids, both
// records written before either could say which process wrote it. Registration
// cannot fold those — at the second the newer id was written, the older record
// was indistinguishable from a reissued pid — so the heartbeat that finally
// identifies the process is the one that has to fold, and the store has to stay
// folded across the heartbeats that follow.
func TestAPairAlreadyInTheStoreConvergesAtTheNextHeartbeat(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Now().UTC()
	began := procStarts()[os.Getpid()]
	if began.IsZero() {
		t.Fatal("cannot read this process's start time")
	}
	// The conversation's own id, written long before the process now serving it
	// began and refreshed by every turn, beside the id its last resume wrote.
	writeLegacy(t, s, Session{ID: "resumed-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a",
		Started: began.Add(-8 * time.Hour), Seen: now})
	writeLegacy(t, s, Session{ID: "restart-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a",
		Started: began.Add(time.Second), Seen: began.Add(time.Second)})
	if got := s.All(); len(got) != 2 {
		t.Fatalf("fixture = %+v, want two records for one process", got)
	}
	if live := s.Live(); len(live) != 1 || live[0].ID != "restart-id" {
		t.Fatalf("before any heartbeat: live = %+v, want only restart-id", live)
	}

	if err := s.Touch("resumed-id", now); err != nil {
		t.Fatal(err)
	}
	if all := s.All(); len(all) != 1 || all[0].ID != "resumed-id" {
		t.Fatalf("after one heartbeat: %+v, want one record, under the id the turns refresh", all)
	}

	for i := 1; i <= 3; i++ {
		if err := s.Touch("resumed-id", now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if all := s.All(); len(all) != 1 || all[0].ID != "resumed-id" {
			t.Fatalf("after heartbeat %d: %+v, want one record still", i+1, all)
		}
	}
	if live := s.Live(); len(live) != 1 || live[0].ID != "resumed-id" {
		t.Fatalf("live = %+v, want only resumed-id", live)
	}
}

// Which of a process's two ids the runtime keeps sending is the runtime's
// choice, not a contract. A heartbeat arriving under the id folding removed is
// still that process saying it is working, and belongs on the record it holds.
func TestAHeartbeatUnderAFoldedIdRefreshesTheRecordThatProcessHolds(t *testing.T) {
	s := Store{Root: t.TempDir()}
	t.Setenv("CLAUDE_PID", strconv.Itoa(os.Getpid()))
	first := time.Now().UTC().Add(-8 * time.Hour)
	if err := s.Register(Session{ID: "first-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a", Seen: first}); err != nil {
		t.Fatal(err)
	}
	if err := s.Register(Session{ID: "second-id", Runtime: "claude", PID: os.Getpid(), Cwd: "/w/a", Seen: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if all := s.All(); len(all) != 1 || all[0].ID != "first-id" {
		t.Fatalf("all = %+v, want second-id folded into first-id", all)
	}

	beat := time.Now().UTC().Add(time.Minute)
	if err := s.Touch("second-id", beat); err != nil {
		t.Fatal(err)
	}
	all := s.All()
	if len(all) != 1 || all[0].ID != "first-id" {
		t.Fatalf("all = %+v, want one record under first-id", all)
	}
	if !all[0].Seen.Equal(beat) {
		t.Fatalf("seen = %s, want the heartbeat at %s — the beat was dropped", all[0].Seen, beat)
	}
}

// A heartbeat for a session that never registered is still not a registration.
func TestAHeartbeatForAnUnregisteredSessionWritesNothing(t *testing.T) {
	s := Store{Root: t.TempDir()}
	t.Setenv("CLAUDE_PID", strconv.Itoa(os.Getpid()))
	if err := s.Touch("never-registered", time.Now().UTC()); err == nil {
		t.Fatal("Touch on an unregistered session returned no error")
	}
	if all := s.All(); len(all) != 0 {
		t.Fatalf("all = %+v, want empty", all)
	}
}

func TestParseElapsed(t *testing.T) {
	for _, c := range []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"00:00", 0, true},
		{"20:06", 20*time.Minute + 6*time.Second, true},
		{"01:02:03", time.Hour + 2*time.Minute + 3*time.Second, true},
		{"3-04:05:06", 3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second, true},
		{"", 0, false},
		{"nonsense", 0, false},
	} {
		got, ok := parseElapsed(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseElapsed(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestWorkingNamesTheRepositoryTheLaneIsIn.
//
// A shift registers from its home directory, which is no repository, so its
// record names none — and two shifts on one repository are then invisible to
// each other because the only thing a peer is matched on is a field neither
// ever carried.
func TestWorkingNamesTheRepositoryTheLaneIsIn(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Now().UTC()
	shift := Session{ID: "shift-1", Runtime: "claude", PID: os.Getpid(), Cwd: "/Users/you/.mellions", Seen: now}
	if err := s.Register(shift); err != nil {
		t.Fatal(err)
	}
	if err := s.Working("shift-1", Work{
		Tree: "/lanes/task-42/tree", Repo: "mellions-coxen", Branch: "mellions/task-42", Assignment: "task-42",
	}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got := s.All()
	if len(got) != 1 {
		t.Fatalf("all = %+v, want one record", got)
	}
	if got[0].Repo != "mellions-coxen" || got[0].Branch != "mellions/task-42" || got[0].Assignment != "task-42" {
		t.Errorf("record = %+v, want the lane's repo, branch and assignment", got[0])
	}
	if got[0].Cwd != "/lanes/task-42/tree" {
		t.Errorf("cwd = %q, want the lane's worktree", got[0].Cwd)
	}
	if !got[0].Started.Equal(shift.Seen) {
		t.Errorf("started = %v, want the session's own start %v", got[0].Started, shift.Seen)
	}
}

// TestADirectoryNeverDisplacesTheLaneASessionHolds. The heartbeat says where a
// session stands on every tool call. A session that cut a worktree works in it
// whatever directory its process sits in, and a shift's process never leaves
// the home directory it was started in.
func TestADirectoryNeverDisplacesTheLaneASessionHolds(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Now().UTC()
	if err := s.Register(Session{ID: "shift-1", Runtime: "claude", PID: os.Getpid(), Cwd: "/home", Seen: now}); err != nil {
		t.Fatal(err)
	}
	lane := Work{Tree: "/lanes/task-42/tree", Repo: "mellions-coxen", Branch: "mellions/task-42", Assignment: "task-42"}
	if err := s.Working("shift-1", lane, now); err != nil {
		t.Fatal(err)
	}
	beat := now.Add(time.Minute)
	if err := s.Working("shift-1", Work{Tree: "/home", Repo: "", Branch: ""}, beat); err != nil {
		t.Fatal(err)
	}
	got := s.All()[0]
	if got.Repo != "mellions-coxen" || got.Assignment != "task-42" || got.Cwd != "/lanes/task-42/tree" {
		t.Errorf("record = %+v, want the lane untouched by the directory it stands in", got)
	}
	// It still has to be a heartbeat: declining the directory must not stop the
	// record saying the session is working.
	if !got.Seen.Equal(beat) {
		t.Errorf("seen = %v, want the heartbeat %v", got.Seen, beat)
	}
}

// TestWorkingFollowsASessionThatMovedIntoARepository. A session holding no lane
// registers wherever the runtime started it; one that has since moved would
// otherwise be looked for in the tree it left.
func TestWorkingFollowsASessionThatMovedIntoARepository(t *testing.T) {
	s := Store{Root: t.TempDir()}
	now := time.Now().UTC()
	if err := s.Register(Session{ID: "free-1", Runtime: "claude", PID: os.Getpid(), Cwd: "/home", Seen: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.Working("free-1", Work{Tree: "/w/repo", Repo: "frontend-app", Branch: "dev"}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got := s.All()[0]
	if got.Repo != "frontend-app" || got.Branch != "dev" || got.Cwd != "/w/repo" {
		t.Errorf("record = %+v, want the repository it moved into", got)
	}
	if got.Assignment != "" {
		t.Errorf("assignment = %q, want none invented", got.Assignment)
	}
}

// TestWorkingForAnUnregisteredSessionWritesNothing. Naming the work is not a
// registration, exactly as a heartbeat is not.
func TestWorkingForAnUnregisteredSessionWritesNothing(t *testing.T) {
	s := Store{Root: t.TempDir()}
	t.Setenv("CLAUDE_PID", strconv.Itoa(os.Getpid()))
	if err := s.Working("never-registered", Work{Repo: "r", Tree: "/w"}, time.Now().UTC()); err == nil {
		t.Fatal("Working on an unregistered session returned no error")
	}
	if all := s.All(); len(all) != 0 {
		t.Fatalf("all = %+v, want empty", all)
	}
}

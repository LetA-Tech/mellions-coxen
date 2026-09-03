// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package presence records which coding-agent sessions are working where.
//
// A session registers when it starts and says which tree, repository, branch
// and assignment it is in. Every other session on the machine can then be told
// who else is on the same repository before it changes something they depend
// on. Nothing here refuses anything: the runtime already carries messaging
// between sessions, and the engineer decides what to do about a peer.
//
// Liveness is the runtime process. A record whose process is gone is a session
// that ended, however it ended, and is not reported as present. A pid alone
// cannot establish that: the operating system reissues a pid as soon as its
// process exits, and reissues every pid on the machine after a reboot. So a
// record carries when its process began as well as which pid it was, and
// liveness asks whether the process holding that pid is the one that
// registered.
package presence

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/durable"
)

// Session is one registered coding-agent session.
type Session struct {
	ID      string `json:"id"`
	Runtime string `json:"runtime"`
	// PID is the runtime process, which is what liveness is judged on.
	PID int `json:"pid"`
	// ProcStarted is when that process began. It is what separates the process
	// that registered from a later one holding the same reissued pid; without
	// it a record survives its session and is reported live for as long as any
	// process happens to hold its number.
	ProcStarted time.Time `json:"proc_started,omitempty"`
	Cwd         string    `json:"cwd"`
	Repo        string    `json:"repo,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	Assignment  string    `json:"assignment,omitempty"`
	Started     time.Time `json:"started"`
	Seen        time.Time `json:"seen"`
}

// reuseSlack absorbs the second of rounding in an elapsed-time reading and any
// gap between a process starting and registering. It is far below the interval
// that separates a reissued pid from the record it collided with.
const reuseSlack = 2 * time.Minute

// Alive reports whether the session's runtime process is still running.
func (s Session) Alive() bool {
	if s.PID <= 0 {
		return false
	}
	return s.running(procStarts())
}

// running answers Alive against a start time already read for every pid in the
// store, so a whole store costs one process listing rather than one per record.
func (s Session) running(starts map[int]time.Time) bool {
	if s.PID <= 0 {
		return false
	}
	began, ok := starts[s.PID]
	if !ok {
		return false
	}
	if !s.ProcStarted.IsZero() {
		return began.Sub(s.ProcStarted).Abs() < reuseSlack
	}
	// A record written before processes were identified says only when its
	// session registered. A process that began after that cannot be the one
	// that registered — which is the reuse this catches — and a process older
	// than the record is taken as still running, because nothing left in the
	// record can say otherwise.
	return !began.After(s.Started.Add(reuseSlack))
}

// procStarts reads when every running process began.
//
// The whole table in one `ps`, rather than a lookup per record: this runs on
// the way to a session's turn, and asking per record would put an exec per
// registered session in front of every prompt — and a single pid the kernel no
// longer knows makes `ps` reject the request, which would read as "no session
// is running" rather than "one record is stale".
func procStarts() map[int]time.Time {
	out := map[int]time.Time{}
	now := time.Now()
	raw, err := exec.Command("ps", "-A", "-o", "pid=,etime=").Output()
	if err != nil && len(raw) == 0 {
		return out
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		age, ok := parseElapsed(f[1])
		if !ok {
			continue
		}
		out[pid] = now.Add(-age)
	}
	return out
}

// parseElapsed reads ps's [[dd-]hh:]mm:ss elapsed time.
func parseElapsed(s string) (time.Duration, bool) {
	var days int
	if i := strings.IndexByte(s, '-'); i >= 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, false
		}
		days, s = d, s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	units := []time.Duration{time.Second, time.Minute, time.Hour}
	total := time.Duration(days) * 24 * time.Hour
	for i := range parts {
		n, err := strconv.Atoi(parts[len(parts)-1-i])
		if err != nil {
			return 0, false
		}
		total += time.Duration(n) * units[i]
	}
	return total, true
}

// Short is the session id as a person reads it.
func (s Session) Short() string {
	if len(s.ID) > 8 {
		return s.ID[:8]
	}
	return s.ID
}

// Resume is the command that reopens this session once it has ended.
func (s Session) Resume() string {
	switch s.Runtime {
	case "claude":
		return "claude --resume " + s.ID
	case "codex":
		return "codex resume " + s.ID
	}
	return ""
}

// Describe is one line about what the session is doing.
func (s Session) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s session %s", s.Runtime, s.Short())
	if s.Repo != "" {
		fmt.Fprintf(&b, " on %s", s.Repo)
	}
	if s.Branch != "" {
		fmt.Fprintf(&b, " (%s)", s.Branch)
	}
	if s.Assignment != "" {
		fmt.Fprintf(&b, ", assignment %s", s.Assignment)
	}
	fmt.Fprintf(&b, ", in %s", s.Cwd)
	return b.String()
}

// Store is one file per session under a directory.
type Store struct{ Root string }

func (s Store) file(id string) string {
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, id)
	return filepath.Join(s.Root, safe+".json")
}

// Register writes or refreshes a session's record.
//
// One runtime process holds one record. A runtime that hands the same process a
// second session id — which is what reopening a conversation does — would
// otherwise leave two records for one session: the session is reported as its
// own peer, counted twice in a machine-wide listing, and the id whose record
// the turn hooks keep refreshing is not the id anybody is told to resume. The
// record that survives is the one whose session started first, so which id
// registers first does not decide the outcome.
func (s Store) Register(sess Session) error {
	if s.Root == "" || sess.ID == "" {
		return nil
	}
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return fmt.Errorf("presence: create %s: %w", s.Root, err)
	}
	prev, prevErr := s.get(sess.ID)
	if prevErr == nil {
		if !prev.Started.IsZero() {
			sess.Started = prev.Started
		}
		// Carried only while the pid is unchanged: a reopened conversation
		// registers again from a new process, and inheriting the old process's
		// start would make the new one look like a reused pid.
		if sess.ProcStarted.IsZero() && prev.PID == sess.PID {
			sess.ProcStarted = prev.ProcStarted
		}
	}
	if sess.ProcStarted.IsZero() {
		sess.ProcStarted = procStarts()[sess.PID]
	}
	if sess.Started.IsZero() {
		sess.Started = sess.Seen
	}
	// Folding reads the whole store, and Register is also the heartbeat on
	// every prompt and every tool call, so the scan runs only when what it
	// depends on has changed: the record is new, its pid moved, or its process
	// was identified where it was not before. That last one is what converges a
	// store already holding a pair — two records written for one process before
	// this code existed share only a number, so neither is foldable until a
	// heartbeat establishes which process wrote it, and the heartbeat that
	// establishes it is the one that must fold.
	was := sess.ID
	if prevErr != nil || prev.PID != sess.PID || !prev.ProcStarted.Equal(sess.ProcStarted) {
		sess = s.fold(sess)
	}
	raw, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	if err := durable.Write(s.file(sess.ID), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	if sess.ID != was {
		os.Remove(s.file(was))
	}
	return nil
}

// fold merges a registration into the record this process already has under
// another id, and returns the record to write. Nothing is folded unless the
// process itself is identified, because without that the only thing two records
// share is a number the operating system reuses.
func (s Store) fold(sess Session) Session {
	if sess.ProcStarted.IsZero() {
		return sess
	}
	starts := map[int]time.Time{sess.PID: sess.ProcStarted}
	for _, other := range s.All() {
		if other.ID == sess.ID || other.PID != sess.PID || !other.running(starts) {
			continue
		}
		if other.Started.Before(sess.Started) {
			sess.ID, sess.Runtime, sess.Started = other.ID, other.Runtime, other.Started
			continue
		}
		os.Remove(s.file(other.ID))
	}
	return sess
}

func (s Store) get(id string) (Session, error) {
	raw, err := os.ReadFile(s.file(id))
	if err != nil {
		return Session{}, err
	}
	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// All reads every record, live or not, oldest first.
func (s Store) All() []Session {
	if s.Root == "" {
		return nil
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.Root, e.Name()))
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(raw, &sess); err != nil || sess.ID == "" {
			continue
		}
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out
}

// Touch records that a session is still working: its Seen moves to now and
// nothing else changes. Absent records are left absent — a heartbeat is not a
// registration — unless this process holds a record under a different id, which
// is what folding leaves behind. The heartbeat is that record's: dropping it
// would freeze a working session's activity at the second it was folded, and
// which of the two ids the runtime keeps sending is the runtime's to choose.
func (s Store) Touch(id string, now time.Time) error {
	sess, err := s.get(id)
	if err != nil {
		held, ok := s.heldHere()
		if !ok {
			return err
		}
		sess = held
	}
	sess.Seen = now
	return s.Register(sess)
}

// Work is what a session turned out to be working in.
type Work struct {
	// Tree is the working tree the work is in.
	Tree string
	// Repo and Branch are what that tree is.
	Repo, Branch string
	// Assignment is the lane, empty where the session holds none.
	Assignment string
}

// Working rewrites a registered session's record to name the work it holds.
//
// A record is written once at session start, from the directory the runtime
// handed over. For an unattended shift that directory is its home, which is no
// repository at all, so the record never names the repository the shift spends
// its whole life in — and two shifts on one repository are each invisible to
// the other. Opening a lane is when a session learns what it works in, and this
// is where its record learns it.
//
// A lane outranks a directory. Work carrying no assignment is only where the
// session's process stands, and it never displaces a lane: a session that cut a
// worktree works in that worktree whatever directory its process sits in.
//
// A session that never registered stays unregistered — a heartbeat is not a
// registration here either.
func (s Store) Working(id string, w Work, now time.Time) error {
	sess, err := s.get(id)
	if err != nil {
		held, ok := s.heldHere()
		if !ok {
			return err
		}
		sess = held
	}
	if w.Assignment == "" && sess.Assignment != "" {
		sess.Seen = now
		return s.Register(sess)
	}
	if w.Assignment != "" {
		sess.Assignment = w.Assignment
	}
	if w.Tree != "" {
		sess.Cwd = w.Tree
	}
	sess.Repo, sess.Branch = w.Repo, w.Branch
	sess.Seen = now
	return s.Register(sess)
}

// heldHere is the record this runtime process holds, under whatever id.
func (s Store) heldHere() (Session, bool) {
	pid := SelfPID()
	if pid <= 0 {
		return Session{}, false
	}
	starts := procStarts()
	for _, sess := range s.All() {
		if sess.PID == pid && sess.running(starts) {
			return sess, true
		}
	}
	return Session{}, false
}

// Live is every session whose runtime process is still running.
func (s Store) Live() []Session {
	all := s.All()
	starts := procStarts()
	var out []Session
	for _, sess := range all {
		if sess.running(starts) {
			out = append(out, sess)
		}
	}
	return out
}

// SelfPID is the runtime process this code runs under, as the runtime exports
// it, or zero where it exports nothing.
//
// It is how a session recognises its own record. The session id cannot do it
// alone: a reopened conversation is handed a second id, and a session reading a
// record written under the other one has no way to tell itself from a peer.
func SelfPID() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("CLAUDE_PID"))); err == nil && v > 0 {
		return v
	}
	return 0
}

// Prune removes the records of sessions that have ended and were last seen
// longer ago than keep. Best effort.
func (s Store) Prune(now time.Time, keep time.Duration) {
	all := s.All()
	starts := procStarts()
	for _, sess := range all {
		if !sess.running(starts) && now.Sub(sess.Seen) > keep {
			os.Remove(s.file(sess.ID))
		}
	}
}

// Here reads the session this process is running inside, from what the runtime
// exports. Empty outside a session — a terminal, a timer, CI — which is a
// supported way to run rather than a fault.
func Here() (runtime, id string) {
	for _, p := range []struct{ rt, env string }{
		{"claude", "CLAUDE_CODE_SESSION_ID"},
		{"codex", "CODEX_SESSION_ID"},
	} {
		if v := strings.TrimSpace(os.Getenv(p.env)); v != "" {
			return p.rt, v
		}
	}
	return "", ""
}

// SelfStarted is when the runtime process this code runs under began, or the
// zero time where the runtime exports no pid or the kernel no longer knows the
// one it exports.
//
// A session id can be shared by two processes — a resumed conversation is
// handed the original's id — so where the question is about the running
// process rather than the conversation, this is what separates them.
func SelfStarted() time.Time {
	pid := SelfPID()
	if pid <= 0 {
		return time.Time{}
	}
	return procStarts()[pid]
}

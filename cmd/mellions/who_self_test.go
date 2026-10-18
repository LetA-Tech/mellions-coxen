// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
	"github.com/LetA-Tech/mellions-coxen/internal/presence"
)

// The shape two shift sessions met on leta-server: started in the same second,
// in the same tree, each told only "another session is in this working tree:
// claude session <peer>", and each reading the peer's id as its own.
const (
	readerID = "ff16449a-b4e5-4c87-b016-e80240a3f75a"
	peerID   = "8eb893f9-4c84-4adf-a04b-0fb3a2832f63"
)

// twoSessionsInOneTree registers the reader (this test process) and a peer
// (its parent, so both pids are running) in one git tree, exports the reader's
// identity the way the runtime does, and returns the config path and the tree.
func twoSessionsInOneTree(t *testing.T) (cfgPath, tree string) {
	t.Helper()
	root := t.TempDir()
	tree = filepath.Join(root, "work")
	if out, err := exec.Command("git", "init", "-q", tree).CombinedOutput(); err != nil {
		t.Skipf("git init: %v: %s", err, out)
	}
	cfg := map[string]string{
		"report_root":      filepath.Join(root, "reports"),
		"assignments_root": filepath.Join(root, "assignments"),
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(root, "config.json")
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	store := presence.Store{Root: filepath.Join(cfg["report_root"], "sessions")}
	for _, s := range []presence.Session{
		{ID: readerID, Runtime: "claude", PID: os.Getpid(), Cwd: tree, Repo: "work"},
		{ID: peerID, Runtime: "claude", PID: os.Getppid(), Cwd: tree, Repo: "work"},
	} {
		if err := store.Register(s); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(store.Live()); n != 2 {
		t.Fatalf("%d live records, want both sessions live — the test would not exercise a peer", n)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", readerID)
	t.Setenv("CLAUDE_PID", fmt.Sprint(os.Getpid()))
	t.Setenv("CODEX_SESSION_ID", "")
	return cfgPath, tree
}

// TestWhoNamesTheReaderWhereItListsAPeer: `who` printed the peer and a header
// naming the tree, and nothing naming the reader, so "claude session 8eb893f9 —
// THIS tree" was read by 8eb893f9's peer as itself.
func TestWhoNamesTheReaderWhereItListsAPeer(t *testing.T) {
	cfgPath, tree := twoSessionsInOneTree(t)
	var err error
	out := captureStdout(t, func() { err = cmdWho([]string{"-config", cfgPath, "-C", tree}) })
	if err != nil {
		t.Fatalf("who: %v", err)
	}
	if !strings.Contains(out, "claude session 8eb893f9") || !strings.Contains(out, "THIS tree") {
		t.Fatalf("positive control: the peer in this tree is not listed, so nothing below is tested; got:\n%s", out)
	}
	want := fmt.Sprintf("You are claude session ff16449a (pid %d).", os.Getpid())
	if !strings.Contains(out, want) {
		t.Errorf("who lists a peer and does not name the reader (%q); got:\n%s", want, out)
	}
	if strings.Contains(out, "- claude session ff16449a") {
		t.Errorf("the reader is listed as a peer of itself; got:\n%s", out)
	}
}

// TestTheSameTreeNoteNamesTheReader: the hook's note carried the peer's id and
// no other, which is what both sessions took as a description of themselves.
func TestTheSameTreeNoteNamesTheReader(t *testing.T) {
	cfgPath, tree := twoSessionsInOneTree(t)
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var note string
	for _, n := range awareness.Notes(observe(cfg, tree, readerID, "claude")) {
		if strings.Contains(n.Because, "in this working tree") {
			note = n.Because
		}
	}
	if !strings.Contains(note, "claude session 8eb893f9") {
		t.Fatalf("positive control: no same-tree note naming the peer, so nothing below is tested; got %q", note)
	}
	want := fmt.Sprintf("You are claude session ff16449a (pid %d).", os.Getpid())
	if !strings.Contains(note, want) {
		t.Errorf("the same-tree note names the peer and not the reader (%q); got %q", want, note)
	}
}

// TestTheHeldLineNamesTheReader: the session-start brief says a lane is held
// by a live session; with no id beside it, the holder reads as the reader.
func TestTheHeldLineNamesTheReader(t *testing.T) {
	const peer = "645d8967-0000-4000-8000-000000000000"
	t.Setenv("CLAUDE_CODE_SESSION_ID", peer)
	store, err := assignment.NewStore(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(assignment.OpenOptions{
		ID: "held-7", Repo: "mellions-coxen", Objective: "o", Because: "b", Source: claimRepo(t),
	}); err != nil {
		t.Fatal(err)
	}
	out := brief(t, store, map[string]presence.Session{peer: {
		ID: peer, Runtime: "claude", PID: 510077, Cwd: "/tmp/held-7/tree",
	}})
	if !strings.Contains(out, "HELD RIGHT NOW") {
		t.Fatalf("positive control: the lane is not reported held; got:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "HELD RIGHT NOW") && !strings.Contains(line, "You are claude session self0001 (pid 111).") {
			t.Errorf("the held line names the holder and not the reader; got %q", line)
		}
	}
}

// TestWhoFromAHookNamesTheReaderFromThePayload: the session-start hook pipes
// its payload to `who`; where the hook's environment carries no session id, the
// payload's is the reader's, and saying "not in your environment" would be
// false about a session that has one.
func TestWhoFromAHookNamesTheReaderFromThePayload(t *testing.T) {
	cfgPath, tree := twoSessionsInOneTree(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("MELLIONS_HOOK", "1")
	payload := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payload, []byte(`{"session_id":"`+readerID+`","cwd":"`+tree+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(payload)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	saved := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = saved }()
	out := captureStdout(t, func() { err = cmdWho([]string{"-config", cfgPath, "-C", tree}) })
	if err != nil {
		t.Fatalf("who: %v", err)
	}
	if !strings.Contains(out, "claude session 8eb893f9") {
		t.Fatalf("positive control: the peer is not listed; got:\n%s", out)
	}
	want := fmt.Sprintf("You are claude session ff16449a (pid %d).", os.Getpid())
	if !strings.Contains(out, want) {
		t.Errorf("who under a hook does not name the reader from the payload (%q); got:\n%s", want, out)
	}
}

// TestWhoWithNoIdentityDoesNotCallTheListOthers: with neither a session id nor
// a pid, nothing can filter the reader out of the listing, so saying every
// session below is another one would be false about the one that is the reader.
func TestWhoWithNoIdentityDoesNotCallTheListOthers(t *testing.T) {
	cfgPath, tree := twoSessionsInOneTree(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_PID", "")
	t.Setenv("MELLIONS_HOOK", "")
	var err error
	out := captureStdout(t, func() { err = cmdWho([]string{"-config", cfgPath, "-C", tree}) })
	if err != nil {
		t.Fatalf("who: %v", err)
	}
	if !strings.Contains(out, "claude session 8eb893f9") || !strings.Contains(out, "not in your environment") {
		t.Fatalf("positive control: the peer or the no-identity sentence is missing, so nothing below is tested; got:\n%s", out)
	}
	if strings.Contains(out, "Every session listed below is another one.") {
		t.Errorf("who cannot filter the reader and still says every listed session is another one; got:\n%s", out)
	}
}

// TestTheResumeLineSaysWhoTheReaderIs: an assignment's text names the session
// that last worked it and says to resume it. Read by that same session, it is
// told to resume itself; read by another, the id beside it reads as its own.
func TestTheResumeLineSaysWhoTheReaderIs(t *testing.T) {
	const worker = "aaaa1111-0000-4000-8000-000000000000"
	const other = "bbbb2222-0000-4000-8000-000000000000"
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", worker)
	store, err := assignment.NewStore(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.Open(assignment.OpenOptions{
		ID: "resume-8", Repo: "mellions-coxen", Objective: "o", Because: "b", Source: claimRepo(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := a.Latest(); !ok || s.ID != worker {
		t.Fatalf("positive control: the opening session is not the lane's latest (%+v, %v), so nothing below is tested", s, ok)
	}

	own := a.Text(time.Now())
	if !strings.Contains(own, "Last worked in claude session aaaa1111-000, which is your own session id") {
		t.Errorf("read by the session that worked it, the text does not say so; got:\n%s", own)
	}
	if strings.Contains(own, "claude --resume "+worker) {
		t.Errorf("the session that worked the lane is told to resume itself; got:\n%s", own)
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", other)
	theirs := a.Text(time.Now())
	if !strings.Contains(theirs, "claude --resume "+worker) {
		t.Fatalf("positive control: another session is not offered the resume; got:\n%s", theirs)
	}
	if !strings.Contains(theirs, "not you: you are claude session bbbb2222-000") {
		t.Errorf("another session is offered the resume with nothing naming it; got:\n%s", theirs)
	}
}

// TestTheBriefSaysWhenTheLastWorkerIsTheReader: the session-start brief drops
// the reader from the live holders, so its own lane falls through to "last
// worked in <its own id> — claude --resume <its own id>".
func TestTheBriefSaysWhenTheLastWorkerIsTheReader(t *testing.T) {
	const worker = "cccc3333-0000-4000-8000-000000000000"
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", worker)
	store, err := assignment.NewStore(filepath.Join(t.TempDir(), "assignments"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(assignment.OpenOptions{
		ID: "own-9", Repo: "mellions-coxen", Objective: "o", Because: "b", Source: claimRepo(t),
	}); err != nil {
		t.Fatal(err)
	}
	run := func(me string) string {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout := os.Stdout
		os.Stdout = w
		err = continueBrief(store, "", nil, me, "You are claude session "+me[:8]+".")
		os.Stdout = stdout
		_ = w.Close()
		out, _ := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatalf("continueBrief: %v", err)
		}
		return string(out)
	}

	own := run(worker)
	if !strings.Contains(own, "own-9") {
		t.Fatalf("positive control: the lane is not in the brief; got:\n%s", own)
	}
	if strings.Contains(own, "claude --resume "+worker) || !strings.Contains(own, "your own session id — nothing to resume") {
		t.Errorf("the brief tells the session that worked the lane to resume itself; got:\n%s", own)
	}

	other := run("dddd4444-0000-4000-8000-000000000000")
	if !strings.Contains(other, "claude --resume "+worker) {
		t.Fatalf("positive control: another reader is not offered the resume; got:\n%s", other)
	}
	if !strings.Contains(other, "if it still opens. You are claude session dddd4444.") {
		t.Errorf("another reader is offered the resume with nothing naming it; got:\n%s", other)
	}
}

// TestTheHeldElsewhereNoteNamesTheReader: the collision note names the lane's
// holder; with nothing beside it, the holder's id reads as the reader's own.
func TestTheHeldElsewhereNoteNamesTheReader(t *testing.T) {
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", readerID)
	t.Setenv("CLAUDE_PID", fmt.Sprint(os.Getpid()))
	note := heldElsewhere("lane-1", presence.Session{Runtime: "claude", ID: peerID})
	if !strings.Contains(note, peerID) || !strings.Contains(note, "not taken up") {
		t.Fatalf("positive control: the note does not name the holder; got %q", note)
	}
	want := fmt.Sprintf("You are claude session ff16449a (pid %d).", os.Getpid())
	if !strings.Contains(note, want) {
		t.Errorf("the collision note names the holder and not the reader (%q); got %q", want, note)
	}
}

// TestBehindOfDoesNotListTheReader: install lists the sessions launched before
// it as needing a new process. A reader whose runtime exports no pid was
// matched by pid only, so it was listed as one of them.
func TestBehindOfDoesNotListTheReader(t *testing.T) {
	launched := time.Date(2026, 10, 18, 5, 0, 0, 0, time.UTC)
	live := []presence.Session{
		{ID: readerID, Runtime: "claude", PID: 4242, Started: launched},
		{ID: peerID, Runtime: "claude", PID: 4343, Started: launched},
	}
	got := behindOf(live, launched.Add(time.Hour), readerID, 0)
	var ids []string
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if !slices.Contains(ids, peerID) {
		t.Fatalf("positive control: the peer launched before the install is not listed; got %v", ids)
	}
	if slices.Contains(ids, readerID) {
		t.Errorf("the reader, known by session id with no pid, is listed as a session behind the install; got %v", ids)
	}
}

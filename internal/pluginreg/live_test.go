// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package pluginreg

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// hooks is the set of hook names a registration declares, as ReadLive is given
// them.
var hooks = []string{"SessionStart:identity"}

func at(t *testing.T, ts string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("bad fixture time %q: %v", ts, err)
	}
	return v
}

// touch dates a transcript, because ReadLive skips files last written before
// the process began rather than reading every transcript in a busy tree.
func touch(t *testing.T, home, project, session string, mod time.Time) {
	t.Helper()
	p := filepath.Join(home, ".claude", "projects", project, session+".jsonl")
	must(t, os.Chtimes(p, mod, mod))
}

// The one #80 is about. After `claude --resume` the runtime keeps exporting the
// original conversation's id while the process writes a new transcript, so
// reading the exported id's file reports a dead file's last SessionStart as
// this session's.
func TestReadLiveFindsTheResumedProcessesOwnTranscript(t *testing.T) {
	home := t.TempDir()
	started := at(t, "2026-08-28T13:13:00Z")

	// The conversation the runtime still names: its last SessionStart is from
	// yesterday, hours before this process existed.
	transcript(t, home, "-p", "old",
		hookLine("2026-08-27T23:30:37Z", "clear", "t1", "mellions identity", "hook_result"))
	touch(t, home, "-p", "old", at(t, "2026-08-27T23:31:00Z"))

	// The file this process is writing: forked, so it carries the old events
	// too, and then its own launch.
	transcript(t, home, "-p", "new",
		hookLine("2026-08-27T23:30:37Z", "clear", "t1", "mellions identity", "hook_result"),
		hookLine("2026-08-28T13:13:04Z", "compact", "t2", "mellions identity", "hook_result"))
	touch(t, home, "-p", "new", at(t, "2026-08-28T13:20:00Z"))

	load, ok := ReadLive(home, "", "old", started, hooks)
	if !ok {
		t.Fatal("both transcripts exist; ReadLive found neither")
	}
	if load.SessionID != "new" {
		t.Fatalf("SessionID = %q, want the transcript this process is writing (%q)", load.SessionID, "new")
	}
	if filepath.Base(load.Transcript) != "new.jsonl" {
		t.Fatalf("Transcript = %q, want new.jsonl", load.Transcript)
	}
	if load.Exported != "old" {
		t.Fatalf("Exported = %q, want %q — doctor has to be able to say the ids differ", load.Exported, "old")
	}
	last, has := load.Latest()
	if !has || last.At.Format(time.RFC3339) != "2026-08-28T13:13:04Z" {
		t.Fatalf("Latest = %v, want the 13:13:04Z compaction from the live file", last)
	}
}

// The ordinary session, which is every session that was not resumed: the
// exported id's own transcript carries this process's launch, and ReadLive must
// not go looking past it.
func TestReadLiveKeepsTheExportedIDWhenItsTranscriptIsTheLiveOne(t *testing.T) {
	home := t.TempDir()
	started := at(t, "2026-08-28T17:46:51Z")

	transcript(t, home, "-p", "mine",
		hookLine("2026-08-28T17:46:53Z", "startup", "t1", "mellions identity", "hook_result"))
	touch(t, home, "-p", "mine", at(t, "2026-08-28T17:50:00Z"))
	// A peer in the same tree, launched later. Nothing may make doctor read it.
	transcript(t, home, "-p", "peer",
		hookLine("2026-08-28T18:30:00Z", "startup", "t2", "mellions identity", "hook_result"))
	touch(t, home, "-p", "peer", at(t, "2026-08-28T18:31:00Z"))

	load, ok := ReadLive(home, "", "mine", started, hooks)
	if !ok {
		t.Fatal("the transcript exists; ReadLive did not find it")
	}
	if load.SessionID != "mine" || load.Exported != "" {
		t.Fatalf("SessionID = %q Exported = %q, want %q and empty", load.SessionID, load.Exported, "mine")
	}
}

// Two sessions launched into one tree inside the same window cannot be told
// apart by time. Naming one of them would put a peer's evidence under this
// session's verdict, which is worse than the stale file it replaced.
func TestReadLiveRefusesToGuessBetweenPeersLaunchedTogether(t *testing.T) {
	home := t.TempDir()
	started := at(t, "2026-08-28T13:13:00Z")

	transcript(t, home, "-p", "old",
		hookLine("2026-08-27T23:30:37Z", "clear", "t1", "mellions identity", "hook_result"))
	touch(t, home, "-p", "old", at(t, "2026-08-27T23:31:00Z"))
	transcript(t, home, "-p", "a",
		hookLine("2026-08-28T13:13:04Z", "compact", "t2", "mellions identity", "hook_result"))
	touch(t, home, "-p", "a", at(t, "2026-08-28T13:20:00Z"))
	transcript(t, home, "-p", "b",
		hookLine("2026-08-28T13:14:40Z", "startup", "t3", "mellions identity", "hook_result"))
	touch(t, home, "-p", "b", at(t, "2026-08-28T13:20:00Z"))

	load, ok := ReadLive(home, "", "old", started, hooks)
	if !ok {
		t.Fatal("the exported id's transcript exists; ReadLive returned nothing")
	}
	if load.SessionID != "old" {
		t.Fatalf("SessionID = %q, want the exported id back — the live one is not established", load.SessionID)
	}
	if len(load.Ambiguous) != 2 {
		t.Fatalf("Ambiguous = %v, want both candidates named", load.Ambiguous)
	}
}

// No pid exported — a hook, a timer, a terminal, or a runtime that does not
// publish one. Nothing about the process is establishable, so the exported id
// is used unchanged rather than guessed past.
func TestReadLiveFallsBackWhereTheProcessStartIsUnknown(t *testing.T) {
	home := t.TempDir()
	transcript(t, home, "-p", "old",
		hookLine("2026-08-27T23:30:37Z", "clear", "t1", "mellions identity", "hook_result"))
	transcript(t, home, "-p", "new",
		hookLine("2026-08-28T13:13:04Z", "compact", "t2", "mellions identity", "hook_result"))

	load, ok := ReadLive(home, "", "old", time.Time{}, hooks)
	if !ok || load.SessionID != "old" || load.Exported != "" {
		t.Fatalf("ReadLive(zero start) = %+v ok=%v, want the exported id unchanged", load, ok)
	}
}

// The exported id's transcript is gone — swept, or the fork left nothing behind
// — so the directory has to come from the working directory instead.
func TestReadLiveDerivesTheProjectDirectoryFromTheWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	started := at(t, "2026-08-28T13:13:00Z")
	transcript(t, home, "-home-you-mellions", "new",
		hookLine("2026-08-28T13:13:04Z", "compact", "t2", "mellions identity", "hook_result"))
	touch(t, home, "-home-you-mellions", "new", at(t, "2026-08-28T13:20:00Z"))

	load, ok := ReadLive(home, "/home/you/mellions", "gone", started, hooks)
	if !ok {
		t.Fatal("the live transcript exists under the derived directory; ReadLive found nothing")
	}
	if load.SessionID != "new" || load.Exported != "gone" {
		t.Fatalf("SessionID = %q Exported = %q, want %q and %q", load.SessionID, load.Exported, "new", "gone")
	}
}

// A derived directory that does not exist reports unestablished rather than
// resolving to some other tree's transcripts: the runtime's mangling is not
// documented, so a rule that is wrong for a path must fail safe.
func TestProjectDirRequiresTheDirectoryToExist(t *testing.T) {
	home := t.TempDir()
	if got := projectDir(home, "/nowhere/at/all"); got != "" {
		t.Fatalf("projectDir = %q, want empty for a directory that is not there", got)
	}
	must(t, os.MkdirAll(filepath.Join(home, ".claude", "projects", "-nowhere-at-all"), 0o755))
	if got := projectDir(home, "/nowhere/at/all"); got == "" {
		t.Fatal("the directory exists now; projectDir must find it")
	}
}

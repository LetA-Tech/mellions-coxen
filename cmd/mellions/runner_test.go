// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The doctor's runner line answers three things from the files the runner
// leaves and the process the lock names: whether a runner is alive here, which
// shifts.sh that live pid is executing, and when the last shift ended. A lock
// naming a dead pid is reported as stale, never as a runner.
func TestRunnerState(t *testing.T) {
	root := t.TempDir()
	loadPath := filepath.Join(root, "loadpath")
	shifts := filepath.Join(root, "shifts")
	expect := func(wantState, wantDetail string) {
		t.Helper()
		state, detail := runnerState(root, loadPath)
		if state != wantState || !strings.Contains(detail, wantDetail) {
			t.Fatalf("got %q %q, want %q containing %q", state, detail, wantState, wantDetail)
		}
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(shifts, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	expect("absent", "no shift has run here")

	if err := os.MkdirAll(shifts, 0o755); err != nil {
		t.Fatal(err)
	}
	write("20260828-120000.log", "12:00:00 shift starting\n12:30:00 session exited 0, reply 10 bytes\n")
	write("20260828-130000.log", "13:00:00 shift starting\n")
	expect("absent", "shift 20260828-130000 in progress")
	write("20260828-130000.log", "13:00:00 shift starting\n13:20:00 session exited 1, reply 0 bytes\n")
	expect("absent", "last shift 20260828-130000 ended")

	// A pid that has exited: the lock is stale.
	gone := exec.Command("true")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	write("runner.lock", strconv.Itoa(gone.Process.Pid)+"\n")
	expect("absent", "stale lock names pid "+strconv.Itoa(gone.Process.Pid))

	// A live process whose command line names a shifts.sh under the load path:
	// the runner, deploying what the load path deploys.
	start := func(script string) int {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(script, []byte("sleep 30\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		live := exec.Command("sh", script)
		if err := live.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = live.Process.Kill(); _ = live.Wait() })
		write("runner.lock", strconv.Itoa(live.Process.Pid)+"\n")
		return live.Process.Pid
	}

	inside := start(filepath.Join(loadPath, "scripts", "shifts.sh"))
	expect("present", "alive, pid "+strconv.Itoa(inside))
	expect("present", filepath.Join(loadPath, "scripts", "shifts.sh")+", which is inside the load path")

	// The split installation: alive, producing shifts, and executing scripts/
	// out of a checkout the load path's merges never reach. Reported as the
	// load path having stopped deploying, because that is what it is.
	other := filepath.Join(root, "superseded", "scripts", "shifts.sh")
	outside := start(other)
	expect("STOPPED", "alive, pid "+strconv.Itoa(outside)+" running "+other)
	expect("STOPPED", "which is outside the load path "+loadPath)

	// A live pid whose command line mentions the script without executing it —
	// a backup, an editor, a grep — is not a runner. The lock is stale.
	mention := exec.Command("sh", "-c", "sleep 30", "shifts.sh.orig")
	if err := mention.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mention.Process.Kill(); _ = mention.Wait() })
	write("runner.lock", strconv.Itoa(mention.Process.Pid)+"\n")
	expect("absent", "stale lock names pid "+strconv.Itoa(mention.Process.Pid))
}

// scriptOrigin decides the runner line's state word, and the two states it must
// keep apart are "established to be a different checkout" and "could not be
// established at all". Only the first is a split; reporting the second as one
// would red every host whose runner was started by a relative path.
func TestScriptOrigin(t *testing.T) {
	root := t.TempDir()
	load := filepath.Join(root, "loadpath")
	if err := os.MkdirAll(filepath.Join(load, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(load, "scripts", "shifts.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name, script, load, want string
		split                    bool
	}{
		{"inside", script, load, "which is inside the load path", false},
		{"load path missing", script, filepath.Join(root, "elsewhere"), "not compared", false},
		{"relative", "shifts.sh", load, "is relative", false},
		{"no load path", script, "", "not compared: no checkout", false},
		{"gone", filepath.Join(load, "scripts", "gone.sh"), load, "does not resolve", false},
	} {
		where, split := scriptOrigin(c.script, c.load)
		if split != c.split || !strings.Contains(where, c.want) {
			t.Errorf("%s: got %q split=%v, want %q split=%v", c.name, where, split, c.want, c.split)
		}
	}

	// The established split: a script that resolves, a load path that resolves,
	// and one not under the other.
	superseded := filepath.Join(root, "superseded", "scripts")
	if err := os.MkdirAll(superseded, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(superseded, "shifts.sh")
	if err := os.WriteFile(old, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	where, split := scriptOrigin(old, load)
	if !split || !strings.Contains(where, "outside the load path") {
		t.Fatalf("split: got %q split=%v, want outside the load path, split=true", where, split)
	}

	// A load path reached through a symlink is the same repository under
	// another name, so comparing one resolved side against one unresolved side
	// reports a split that is not there. Either side can be the symlinked one:
	// a checkout under a linked home, a script invoked through a linked path.
	link := filepath.Join(root, "link-to-loadpath")
	if err := os.Symlink(load, link); err != nil {
		t.Fatal(err)
	}
	if where, split := scriptOrigin(script, link); split {
		t.Fatalf("symlinked load path read as a split: %q", where)
	}
	if where, split := scriptOrigin(filepath.Join(link, "scripts", "shifts.sh"), load); split {
		t.Fatalf("symlinked script path read as a split: %q", where)
	}
}

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

	// The same live runner with nothing to place it against is not the same
	// claim, and "present" for both is the fault this row exists to remove.
	if state, detail := runnerState(root, ""); state != "partial" {
		t.Fatalf("state = %q, want %q: nothing established where the script came from — %s", state, "partial", detail)
	}

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

// A checkout path containing a space is the case that separates reading the
// kernel's argument vector from re-splitting the line ps prints. ps flattens
// argv into one space-joined string; splitting it back apart cuts the path in
// two, and the fragment that ends in shifts.sh is relative, so a live split
// installation is reported as "not compared" rather than as the split it is —
// the row reading green through the state it exists to catch.
func TestRunnerStatePlacesAScriptWhosePathHasASpace(t *testing.T) {
	root := t.TempDir()
	loadPath := filepath.Join(root, "loadpath")
	if err := os.MkdirAll(filepath.Join(loadPath, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	shifts := filepath.Join(root, "shifts")
	if err := os.MkdirAll(shifts, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "a superseded checkout", "scripts", "shifts.sh")
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
	if err := os.WriteFile(filepath.Join(shifts, "runner.lock"), []byte(strconv.Itoa(live.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	state, detail := runnerState(root, loadPath)
	if state != "STOPPED" {
		t.Fatalf("state = %q, want %q: the runner executes %s, which is outside %s — %s", state, "STOPPED", script, loadPath, detail)
	}
	if !strings.Contains(detail, "running "+script+",") {
		t.Fatalf("detail = %q, want it to name %q whole", detail, script)
	}
}

// MELLIONS_SHIFT and MELLIONS_SETTINGS are the other half of #13's own
// falsification clause: they substitute the shift script and the deny list for
// the ones beside shifts.sh, so placing shifts.sh alone leaves a runner taking
// its deploy files from a superseded checkout reporting present.
func TestRunnerStatePlacesTheEnvironmentsOverrides(t *testing.T) {
	root := t.TempDir()
	loadPath := filepath.Join(root, "loadpath")
	shifts := filepath.Join(root, "shifts")
	for _, d := range []string{filepath.Join(loadPath, "scripts"), shifts, filepath.Join(root, "superseded", "scripts"), filepath.Join(root, "superseded", "deploy")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(loadPath, "scripts", "shifts.sh")
	if err := os.WriteFile(script, []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldShift := filepath.Join(root, "superseded", "scripts", "shift.sh")
	oldSettings := filepath.Join(root, "superseded", "deploy", "unattended-settings.json")
	for _, f := range []string{oldShift, oldSettings} {
		if err := os.WriteFile(f, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := func(env ...string) int {
		t.Helper()
		live := exec.Command("sh", script)
		live.Env = append(os.Environ(), env...)
		if err := live.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = live.Process.Kill(); _ = live.Wait() })
		if err := os.WriteFile(filepath.Join(shifts, "runner.lock"), []byte(strconv.Itoa(live.Process.Pid)), 0o644); err != nil {
			t.Fatal(err)
		}
		return live.Process.Pid
	}

	// The shifts.sh is inside the load path and every other row is green: the
	// only thing that has stopped deploying is the file the environment names.
	start("MELLIONS_SHIFT=" + oldShift)
	state, detail := runnerState(root, loadPath)
	if state != "STOPPED" || !strings.Contains(detail, "$MELLIONS_SHIFT names "+oldShift) {
		t.Fatalf("got %q %q, want STOPPED naming $MELLIONS_SHIFT %s", state, detail, oldShift)
	}

	start("MELLIONS_SETTINGS=" + oldSettings)
	state, detail = runnerState(root, loadPath)
	if state != "STOPPED" || !strings.Contains(detail, "$MELLIONS_SETTINGS names "+oldSettings) {
		t.Fatalf("got %q %q, want STOPPED naming $MELLIONS_SETTINGS %s", state, detail, oldSettings)
	}

	// An override inside the load path is not a split, and an unset one is not
	// an override at all: the file beside shifts.sh is the one in use.
	inside := filepath.Join(loadPath, "scripts", "shift.sh")
	if err := os.WriteFile(inside, []byte("true\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start("MELLIONS_SHIFT=" + inside)
	if state, detail := runnerState(root, loadPath); state != "present" {
		t.Fatalf("state = %q, want present: %s is inside %s — %s", state, inside, loadPath, detail)
	}
	start()
	if state, detail := runnerState(root, loadPath); state != "present" {
		t.Fatalf("state = %q, want present: no override is set — %s", state, detail)
	}
}

// The row keeps the word shifts.sh earned where the environment cannot be read
// at all, and says so: a check that reds on what it could not measure is the
// same fault as one that reads green through what it was built to catch.
func TestRunnerOverridesStatesTheBoundaryWithoutAProcFilesystem(t *testing.T) {
	restore := procRoot
	procRoot = filepath.Join(t.TempDir(), "no-proc")
	t.Cleanup(func() { procRoot = restore })

	detail, split, unplaced := runnerOverrides(os.Getpid(), t.TempDir())
	if split || unplaced {
		t.Fatalf("split=%v unplaced=%v, want both false: an unreadable environment is not a split", split, unplaced)
	}
	if !strings.Contains(detail, "MELLIONS_SHIFT") || !strings.Contains(detail, "unread") {
		t.Fatalf("detail = %q, want it to name the variables it could not read", detail)
	}
}

// processArgs answers on a host with no process filesystem, which is the only
// state Darwin is ever in. Driven by pointing procRoot at nothing.
func TestProcessArgsFallsBackToPS(t *testing.T) {
	restore := procRoot
	procRoot = filepath.Join(t.TempDir(), "no-proc")
	t.Cleanup(func() { procRoot = restore })

	live := exec.Command("sleep", "30")
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Process.Kill(); _ = live.Wait() })

	args, ok := processArgs(live.Process.Pid)
	if !ok || len(args) == 0 || filepath.Base(args[0]) != "sleep" {
		t.Fatalf("processArgs = %q %v, want the ps fallback to name sleep", args, ok)
	}
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
		established, split       bool
	}{
		{"inside", script, load, "which is inside the load path", true, false},
		{"load path missing", script, filepath.Join(root, "elsewhere"), "not compared", false, false},
		{"relative", "shifts.sh", load, "is relative", false, false},
		{"no load path", script, "", "not compared: no checkout", false, false},
		{"gone", filepath.Join(load, "scripts", "gone.sh"), load, "does not resolve", false, false},
	} {
		where, established, split := scriptOrigin(c.script, c.load)
		if split != c.split || established != c.established || !strings.Contains(where, c.want) {
			t.Errorf("%s: got %q established=%v split=%v, want %q established=%v split=%v",
				c.name, where, established, split, c.want, c.established, c.split)
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
	where, established, split := scriptOrigin(old, load)
	if !split || !established || !strings.Contains(where, "outside the load path") {
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
	if where, _, split := scriptOrigin(script, link); split {
		t.Fatalf("symlinked load path read as a split: %q", where)
	}
	if where, _, split := scriptOrigin(filepath.Join(link, "scripts", "shifts.sh"), load); split {
		t.Fatalf("symlinked script path read as a split: %q", where)
	}
}

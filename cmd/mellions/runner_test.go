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

// The doctor's runner line answers two things from the files the runner
// leaves: whether a runner is alive here, and when the last shift ended. A
// lock naming a dead pid is reported as stale, never as a runner.
func TestRunnerState(t *testing.T) {
	root := t.TempDir()
	shifts := filepath.Join(root, "shifts")
	expect := func(wantState, wantDetail string) {
		t.Helper()
		state, detail := runnerState(root)
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

	// A live process whose command line names shifts.sh: the runner.
	script := filepath.Join(root, "shifts.sh")
	if err := os.WriteFile(script, []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := exec.Command("sh", script)
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Process.Kill(); _ = live.Wait() }()
	write("runner.lock", strconv.Itoa(live.Process.Pid)+"\n")
	expect("present", "alive, pid "+strconv.Itoa(live.Process.Pid))
}

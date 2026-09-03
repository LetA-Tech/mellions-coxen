// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// runnerState reads what scripts/shifts.sh leaves under root/shifts: its lock
// and the latest shift's log. Alive means the pid in the lock is a live process
// running shifts.sh, the runner's own test of a lock — a pid is reused, and a
// lock naming whatever got the number next is stale, not held.
func runnerState(root string) (state, detail string) {
	shifts := filepath.Join(root, "shifts")
	last := lastShift(shifts)
	pid, held := lockPID(filepath.Join(shifts, "runner.lock"))
	switch {
	case held && runnerAlive(pid):
		return "present", fmt.Sprintf("alive, pid %d; %s", pid, last)
	case held:
		return "absent", fmt.Sprintf("stale lock names pid %d, not a live runner; %s", pid, last)
	}
	return "absent", "none on this host; " + last
}

func lockPID(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func runnerAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && strings.Contains(string(out), "shifts.sh")
}

// lastShift says when the latest shift ended, or that it is still running.
// Shift ids are UTC stamps, so the greatest id is the latest shift, and
// "session exited" is the line shift.sh writes once the session has ended,
// whatever it said.
func lastShift(dir string) string {
	paths, _ := filepath.Glob(filepath.Join(dir, "[0-9]*.log"))
	if len(paths) == 0 {
		return "no shift has run here"
	}
	ids := make([]string, 0, len(paths))
	for _, p := range paths {
		ids = append(ids, strings.TrimSuffix(filepath.Base(p), ".log"))
	}
	sort.Strings(ids)
	id := ids[len(ids)-1]
	path := filepath.Join(dir, id+".log")
	st, err := os.Stat(path)
	if err != nil {
		return "the latest shift is " + id
	}
	age := humanAge(time.Since(st.ModTime()))
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte("session exited")) {
		return fmt.Sprintf("last shift %s ended %s ago", id, age)
	}
	return fmt.Sprintf("shift %s in progress, its log written %s ago", id, age)
}

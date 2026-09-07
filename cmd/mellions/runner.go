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
//
// A live pid is two questions, not one, and the second is the one a reader
// relies on: which shifts.sh it is executing. The runner supplies scripts/ and
// deploy/ from its own checkout while the load path supplies hooks, Skills,
// commands and the agent from another, so the two can be different
// repositories with every other line green — a state in which merges to
// scripts/ deploy nothing and nobody is told. That is the load path having
// stopped deploying, not a runner being present, so the state word says
// STOPPED and the detail names both paths.
func runnerState(root, loadPath string) (state, detail string) {
	shifts := filepath.Join(root, "shifts")
	last := lastShift(shifts)
	pid, held := lockPID(filepath.Join(shifts, "runner.lock"))
	script, alive := runnerScript(pid)
	switch {
	case held && alive:
		where, split := scriptOrigin(script, loadPath)
		if split {
			return "STOPPED", fmt.Sprintf("alive, pid %d running %s, %s; merges to scripts/ and deploy/ there do not reach it; %s",
				pid, script, where, last)
		}
		return "present", fmt.Sprintf("alive, pid %d running %s, %s; %s", pid, script, where, last)
	case held:
		return "absent", fmt.Sprintf("stale lock names pid %d, not a live runner; %s", pid, last)
	}
	return "absent", "none on this host; " + last
}

// scriptOrigin says whether the script the runner executes comes out of the
// load path's checkout, and split is true only where it is established that it
// does not. An unresolvable script path — a relative argv, a deleted file — and
// an unknown load path are each reported as not compared rather than as a
// split: this line exists because a check that answers a narrower question than
// its wording implies reads green through the state it was built to catch, and
// one that reds on what it could not measure is the same fault mirrored.
func scriptOrigin(script, loadPath string) (where string, split bool) {
	switch {
	case loadPath == "":
		return "not compared: no checkout the runner and the runtime both read", false
	case !filepath.IsAbs(script):
		return "not compared: " + script + " is relative, so which checkout it came from is unestablished", false
	}
	s, err := filepath.EvalSymlinks(script)
	if err != nil {
		return "not compared: " + script + " does not resolve on disk", false
	}
	// Both sides are resolved before they are compared: a load path or a
	// checkout reached through a symlink is the same repository under another
	// name, and comparing one resolved side against one unresolved side reports
	// a split that is not there.
	root, err := filepath.EvalSymlinks(loadPath)
	if err != nil {
		return "not compared: the load path " + loadPath + " does not resolve on disk", false
	}
	rel, err := filepath.Rel(root, s)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "which is outside the load path " + loadPath, true
	}
	return "which is inside the load path " + loadPath, false
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

// runnerScript names the shifts.sh a live pid is executing, and is the whole
// aliveness test: a pid that no longer exists, or that a reused number now
// gives to something else, names no shifts.sh.
//
// The name is taken as a whole argument rather than as a substring of the
// command line, so a process that merely mentions the script — a grep, an
// editor, the shell running the search — is not read as the runner. A bare
// `shifts.sh` is returned as it stands: it is as legitimate an invocation as an
// absolute one, and which checkout it came from is then a question ps cannot
// answer, which scriptOrigin says out loud rather than guesses at.
func runnerScript(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return "", false
	}
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", false
	}
	for _, arg := range strings.Fields(string(out)) {
		if filepath.Base(arg) == "shifts.sh" {
			return arg, true
		}
	}
	return "", false
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

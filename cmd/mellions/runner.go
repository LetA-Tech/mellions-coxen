// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
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
	if !held {
		return "absent", "none on this host; " + last
	}
	script, exact, alive := runnerScript(pid)
	if !alive {
		return "absent", fmt.Sprintf("stale lock names pid %d, not a live runner; %s", pid, last)
	}
	where, established, split := "", false, false
	if exact {
		where, established, split = scriptOrigin(script, loadPath)
	} else {
		// A flattened argument vector is not a path, so no path is named: a row
		// that prints a fragment of a real one names a file that is not there,
		// and the reader cannot tell that from a script that moved.
		script = "shifts.sh"
		where = "not compared: ps joins the argument vector with the byte it would be split on, so the path it came from is unrecoverable here"
	}
	head := fmt.Sprintf("alive, pid %d running %s, %s", pid, script, where)
	if split {
		return "STOPPED", fmt.Sprintf("%s; merges to scripts/ and deploy/ there do not reach it; %s", head, last)
	}
	over, overSplit, overUnplaced := runnerOverrides(pid, loadPath)
	if over != "" {
		head += "; " + over
	}
	switch {
	case overSplit:
		return "STOPPED", fmt.Sprintf("%s; merges to that file do not reach it; %s", head, last)
	case !established || overUnplaced:
		// A live runner whose script could not be placed is not the same claim
		// as one placed inside the load path, and saying "present" for both is
		// the fault this row exists to remove.
		return "partial", fmt.Sprintf("%s; %s", head, last)
	}
	return "present", fmt.Sprintf("%s; %s", head, last)
}

// runnerOverrides places the deploy files the runner's own environment
// substitutes for the ones beside its shifts.sh. MELLIONS_SHIFT selects the
// shift script (scripts/shifts.sh:50) and MELLIONS_SETTINGS the deny list
// (scripts/shift.sh:30), so placing shifts.sh places neither: a runner whose
// shifts.sh sits inside the load path can still take its shift script or its
// unattended settings from a superseded checkout, which is the same stopped
// deployment by another route.
//
// An unset variable is not an override — the file beside shifts.sh is then the
// one in use, and placing shifts.sh already placed it. Where the environment
// cannot be read at all the row keeps the word shifts.sh earned and names the
// boundary in its detail: a check that reds on what it could not measure is the
// same fault as one that reads green through the state it was built to catch.
//
// Only MELLIONS_SHIFT moves the state word. It selects the runner's own code,
// which has no reason to come from anywhere but the checkout that is deployed,
// so a copy outside the load path is the stopped deployment this row exists to
// name. MELLIONS_SETTINGS selects configuration: docs/cli.md:445 defaults it to
// the checkout's own deploy/unattended-settings.json, so setting it at all
// means naming a different file, and a host's settings living outside every
// checkout is the ordinary reason to do that rather than a superseded copy.
// Which of the two it is here is not established, so the file is named in the
// detail — a reader needs it either way — and the word stays what the rest of
// the row earned.
//
// Every variable is examined. Returning at the first one that could not be
// placed would let an unplaced neighbour hide an established split behind it,
// which is the shape of #13 itself.
func runnerOverrides(pid int, loadPath string) (detail string, split, unplaced bool) {
	env, err := runnerEnv(pid)
	if err != nil {
		return "$MELLIONS_SHIFT and $MELLIONS_SETTINGS unread — " + unreadReason(err) +
			", so a shift script or deny list substituted through them is outside what this row places", false, false
	}
	var notes []string
	for _, o := range []struct {
		name  string
		binds bool
	}{
		{"MELLIONS_SHIFT", true},
		{"MELLIONS_SETTINGS", false},
	} {
		path := env[o.name]
		if path == "" {
			continue
		}
		where, established, isSplit := scriptOrigin(path, loadPath)
		notes = append(notes, fmt.Sprintf("$%s names %s, %s", o.name, path, where))
		if !o.binds {
			continue
		}
		split = split || isSplit
		unplaced = unplaced || !established
	}
	if split {
		unplaced = false
	}
	return strings.Join(notes, "; "), split, unplaced
}

// scriptOrigin says whether the script the runner executes comes out of the
// load path's checkout, and split is true only where it is established that it
// does not. An unresolvable script path — a relative argv, a deleted file — and
// an unknown load path are each reported as not compared rather than as a
// split: this line exists because a check that answers a narrower question than
// its wording implies reads green through the state it was built to catch, and
// one that reds on what it could not measure is the same fault mirrored.
func scriptOrigin(script, loadPath string) (where string, established, split bool) {
	switch {
	case loadPath == "":
		return "not compared: no checkout the runner and the runtime both read", false, false
	case !filepath.IsAbs(script):
		return "not compared: " + script + " is relative, so which checkout it came from is unestablished", false, false
	}
	s, err := filepath.EvalSymlinks(script)
	if err != nil {
		return "not compared: " + script + " does not resolve on disk", false, false
	}
	// Both sides are resolved before they are compared: a load path or a
	// checkout reached through a symlink is the same repository under another
	// name, and comparing one resolved side against one unresolved side reports
	// a split that is not there.
	root, err := filepath.EvalSymlinks(loadPath)
	if err != nil {
		return "not compared: the load path " + loadPath + " does not resolve on disk", false, false
	}
	rel, err := filepath.Rel(root, s)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "which is outside the load path " + loadPath, true, true
	}
	return "which is inside the load path " + loadPath, true, false
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
// gives to something else, names no shifts.sh. The pid is the one lockPID
// established, so it is positive — signal 0 to a group or to every process a
// caller may signal is not a question this asks.
//
// The name is taken as a whole argument rather than as a substring of the
// command line, so a process that merely mentions the script — a grep, an
// editor, the shell running the search — is not read as the runner. A bare
// `shifts.sh` is returned as it stands: it is as legitimate an invocation as an
// absolute one, and which checkout it came from is then a question ps cannot
// answer, which scriptOrigin says out loud rather than guesses at.
// exact is false where the reading cannot bear a path at all. A flattened
// vector is joined with the byte it is then split on, so a token that is not
// absolute is either a relative invocation or the tail of an absolute path
// containing a space, and nothing in the line separates them. Under the process
// filesystem the ambiguity does not exist and every token is exact.
func runnerScript(pid int) (script string, exact, alive bool) {
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return "", false, false
	}
	args, whole, ok := processArgs(pid)
	if !ok {
		return "", false, false
	}
	for _, arg := range args {
		if filepath.Base(arg) == "shifts.sh" {
			return arg, whole || filepath.IsAbs(arg), true
		}
	}
	return "", false, false
}

// procRoot is where the process filesystem is mounted, indirected so a test on
// a host that has one can still drive the ps fallback.
var procRoot = "/proc"

// processArgs returns the pid's argument vector as the kernel holds it.
// /proc/<pid>/cmdline is NUL-delimited, so an argument that contains a space
// survives it whole. `ps -o args=` flattens the vector into a single
// space-joined line instead, and splitting that back apart cuts a checkout path
// containing a space into two fragments: the row would then print a fragment of
// a real path, which resolves nowhere and is reported as "not compared" — a
// split installation reading as unmeasured is the fault this row exists to
// remove. ps stays as the fallback for hosts with no process filesystem, where
// the flattened line is the only reading available.
// whole is true only for the NUL-delimited reading, where each element is the
// argument the kernel holds; the ps line is reported as what it is.
func processArgs(pid int) (args []string, whole, ok bool) {
	raw, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cmdline"))
	if err == nil && len(raw) > 0 {
		return splitNUL(raw), true, true
	}
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, false, false
	}
	return strings.Fields(string(out)), false, true
}

// runnerEnv reads the environment the live runner was given. It returns why it
// could not rather than an empty environment: an unset override and an unread
// one are different claims, and only the first means the file beside shifts.sh
// is the one in use.
func runnerEnv(pid int) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "environ"))
	if err != nil {
		return nil, err
	}
	env := make(map[string]string)
	for _, entry := range splitNUL(raw) {
		if k, v, ok := strings.Cut(entry, "="); ok {
			env[k] = v
		}
	}
	return env, nil
}

// unreadReason keeps the row's detail to what it established. /proc/<pid>/environ
// is readable by the user that owns the process and nobody else, and runnerScript
// already tolerates EPERM from the liveness signal, so a runner another user owns
// is a state this reaches — reporting it as "no process filesystem" would be a
// false sentence in shipped output, and the two have different remedies.
func unreadReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "the runner's environment is readable only by the user that owns it, and this is not that user"
	case errors.Is(err, fs.ErrNotExist):
		// ENOENT on /proc/<pid>/environ is two states, and the host is the one
		// this can establish: a pid that exited between the argv read and this
		// one gives the same errno on a host whose process filesystem is right
		// there. Saying "no process filesystem" for that is the same class of
		// false sentence this function exists to keep out.
		if _, statErr := os.Stat(procRoot); statErr != nil {
			return "no process filesystem on this host"
		}
		return "the runner's process ended between reading its command line and its environment"
	}
	return "the runner's environment did not read: " + err.Error()
}

func splitNUL(raw []byte) []string {
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
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

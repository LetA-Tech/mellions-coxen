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
	requireProcFS(t)
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
// requireProcFS skips where another process's argument vector and environment
// cannot be read at all. Darwin is a deploy target (deploy/README.md:129) and
// has no process filesystem, so a test that asserts what the kernel holds for
// another pid states a condition that host cannot produce — and `make check`
// red on every Mac is the class of open issue #39, not a finding about this
// code. What the row does on such a host is asserted, without /proc, by
// TestRunnerOverridesStatesTheBoundaryWithoutAProcFilesystem and
// TestProcessArgsFallsBackToPS.
func requireProcFS(t *testing.T) {
	t.Helper()
	if _, err := os.ReadFile(filepath.Join(procRoot, "self", "environ")); err != nil {
		t.Skipf("no readable process filesystem at %s (%v)", procRoot, err)
	}
}

func TestRunnerStatePlacesTheEnvironmentsOverrides(t *testing.T) {
	requireProcFS(t)
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

	// MELLIONS_SETTINGS is named and does not move the word. docs/cli.md:445
	// defaults it to the checkout's own deploy/unattended-settings.json, so
	// setting it at all means naming a different file, and a host's settings
	// file living outside every checkout is the ordinary reason to do that.
	// A settings file outside the load path is two different states and the row
	// asks which. In a checkout of this repository it is a deploy/ that stopped
	// receiving merges — the highest-consequence split on this row, the deny
	// list an unattended runtime is given — so STOPPED. Anywhere else it is
	// this host's own file, which is the documented ordinary use
	// (docs/cli.md:445 defaults the variable to the checkout's own copy, so
	// setting it means naming another), and moves nothing. Neither a permanent
	// exit 1 nor a permanent partial: the question is answerable.
	if err := os.WriteFile(filepath.Join(root, "superseded", "scripts", "shifts.sh"), []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start("MELLIONS_SETTINGS=" + oldSettings)
	state, detail = runnerState(root, loadPath)
	if state != "STOPPED" || !strings.Contains(detail, "a checkout of this repository at "+filepath.Join(root, "superseded")) {
		t.Fatalf("got %q %q, want STOPPED naming the superseded checkout", state, detail)
	}

	hostOwn := filepath.Join(root, "etc", "settings.json")
	if err := os.MkdirAll(filepath.Dir(hostOwn), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostOwn, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start("MELLIONS_SETTINGS=" + hostOwn)
	state, detail = runnerState(root, loadPath)
	if state != "present" || !strings.Contains(detail, "no checkout of this repository") {
		t.Fatalf("got %q %q, want present: %s is in no checkout, so it is this host's own file", state, detail, hostOwn)
	}

	// Every variable is examined. A MELLIONS_SHIFT that cannot be placed is not
	// a reason to stop reading, or an unplaced neighbour hides what comes after
	// it — the shape of #13 itself, inside the fix for #13.
	start("MELLIONS_SHIFT=shifts.sh", "MELLIONS_SETTINGS="+hostOwn)
	state, detail = runnerState(root, loadPath)
	if state != "partial" || !strings.Contains(detail, "$MELLIONS_SETTINGS names "+hostOwn) {
		t.Fatalf("got %q %q, want partial still naming $MELLIONS_SETTINGS %s behind an unplaced $MELLIONS_SHIFT", state, detail, hostOwn)
	}
	if !strings.Contains(detail, "$MELLIONS_SHIFT names shifts.sh") {
		t.Fatalf("detail = %q, want it to name the unplaced $MELLIONS_SHIFT too", detail)
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
	if !strings.Contains(detail, "no process filesystem on this host") {
		t.Fatalf("detail = %q, want the reason it could not read them", detail)
	}
}

// A runner another user owns is a state this reaches — runnerScript already
// tolerates EPERM from the liveness signal — and /proc/<pid>/environ is readable
// by the owning user alone. Reporting that as "no process filesystem" would put
// a false sentence in doctor's output, so the two reasons stay apart.
func TestUnreadReasonSeparatesPermissionFromAbsence(t *testing.T) {
	dir := t.TempDir()
	_, missing := os.ReadFile(filepath.Join(dir, "no-such-file"))
	if missing == nil {
		t.Fatal("a file that is not there read")
	}

	// ENOENT is two states and only the host separates them. With no process
	// filesystem the absence is the host's; with one mounted, the same errno on
	// /proc/<pid>/environ is a pid that ended between the two reads — and
	// naming the host there is the false sentence this function exists to keep
	// out, one errno over from the one it already keeps out.
	restore := procRoot
	t.Cleanup(func() { procRoot = restore })

	procRoot = filepath.Join(dir, "no-proc")
	if got := unreadReason(missing); !strings.Contains(got, "no process filesystem on this host") {
		t.Fatalf("unreadReason(not-exist, no procfs) = %q, want the absence reason", got)
	}
	procRoot = dir
	if got := unreadReason(missing); !strings.Contains(got, "process ended") {
		t.Fatalf("unreadReason(not-exist, procfs present) = %q, want the exited-process reason", got)
	}
	procRoot = restore

	locked := filepath.Join(dir, "environ")
	if err := os.WriteFile(locked, []byte("A=b"), 0o000); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		// Only the permission half needs a non-root reader; the arms above ran.
		t.Skip("root reads a 0000 file, so this host cannot produce the permission case")
	}
	_, err := os.ReadFile(locked)
	if err == nil {
		t.Fatal("a 0000 file read: this host cannot produce the permission case")
	}
	if got := unreadReason(err); !strings.Contains(got, "readable only by the user that owns it") {
		t.Fatalf("unreadReason(permission) = %q, want the ownership reason", got)
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

	args, whole, ok := processArgs(live.Process.Pid)
	if !ok || len(args) == 0 || filepath.Base(args[0]) != "sleep" {
		t.Fatalf("processArgs = %q %v, want the ps fallback to name sleep", args, ok)
	}
	if whole {
		t.Fatal("the ps line was reported as the kernel's argument vector; it is a flattened reading")
	}
}

// On a host with no process filesystem the row must refuse a path it cannot
// read rather than print a fragment of one. ps joins argv with the byte the
// line is then split on, so the tail of "/…/a superseded checkout/scripts/
// shifts.sh" comes back as the relative "checkout/scripts/shifts.sh" — a file
// that is nowhere on disk, which the row would have named as the script the
// runner executes.
func TestRunnerStateNamesNoPathWhenPSCannotBearOne(t *testing.T) {
	restore := procRoot
	procRoot = filepath.Join(t.TempDir(), "no-proc")
	t.Cleanup(func() { procRoot = restore })

	root := t.TempDir()
	loadPath := filepath.Join(root, "loadpath")
	shifts := filepath.Join(root, "shifts")
	script := filepath.Join(root, "a superseded checkout", "scripts", "shifts.sh")
	for _, d := range []string{filepath.Join(loadPath, "scripts"), shifts, filepath.Dir(script)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
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
	if state != "partial" {
		t.Fatalf("state = %q, want partial: nothing about the script is established here — %s", state, detail)
	}
	if strings.Contains(detail, "checkout/scripts/shifts.sh") {
		t.Fatalf("detail = %q, want no fabricated path: that fragment is not a file on disk", detail)
	}
	if !strings.Contains(detail, "unrecoverable") {
		t.Fatalf("detail = %q, want it to say the path could not be recovered", detail)
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

	// A file that could not be walked is not a file that is not there. A
	// settings file under a service user's 0700 home is read by its owner and
	// refused to the operator running doctor, and "does not resolve on disk" is
	// a cause the row never established — the class this row exists to keep out.
	if os.Geteuid() != 0 {
		shut := filepath.Join(load, "shut")
		if err := os.MkdirAll(shut, 0o755); err != nil {
			t.Fatal(err)
		}
		hidden := filepath.Join(shut, "unattended-settings.json")
		if err := os.WriteFile(hidden, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(shut, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(shut, 0o755) })
		where, established, split := scriptOrigin(hidden, load)
		// Readable again before anything can fail. GOTMPDIR is shared with
		// whatever else runs on the host, and scripts/shift.sh walks it to
		// collect what earlier shifts left, so an assertion that exits between
		// the chmod and the cleanup would leave a directory nothing can walk.
		if err := os.Chmod(shut, 0o755); err != nil {
			t.Fatal(err)
		}
		if established || split {
			t.Fatalf("got %q established=%v split=%v, want neither: it was never read", where, established, split)
		}
		if strings.Contains(where, "does not resolve on disk") {
			t.Fatalf("where = %q: the file is there and inside the load path; it was not readable", where)
		}
		if !strings.Contains(where, "cannot be read from here") {
			t.Fatalf("where = %q, want the permission reason", where)
		}
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

// A stable name outside every checkout that points into one is how an operator
// names a deploy file, and it is the highest-consequence split on this row: the
// deny list an unattended runtime is given, taken from a checkout that stopped
// receiving merges. The row decides "outside the load path" from the resolved
// path, so it has to ask which checkout from the resolved path too — asking the
// raw one answers "no checkout of this repository, so it is this host's own
// file", which is this row reading green through the state it exists to catch.
func TestRunnerStatePlacesASettingsSymlinkByWhereItResolves(t *testing.T) {
	requireProcFS(t)
	root := t.TempDir()
	loadPath := filepath.Join(root, "loadpath")
	shifts := filepath.Join(root, "shifts")
	superseded := filepath.Join(root, "superseded")
	for _, d := range []string{
		filepath.Join(loadPath, "scripts"), shifts,
		filepath.Join(superseded, "scripts"), filepath.Join(superseded, "deploy"),
		filepath.Join(root, "etc"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(loadPath, "scripts", "shifts.sh")
	if err := os.WriteFile(script, []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// scripts/shifts.sh is what makes superseded a checkout to checkoutOf.
	if err := os.WriteFile(filepath.Join(superseded, "scripts", "shifts.sh"), []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(superseded, "deploy", "unattended-settings.json")
	if err := os.WriteFile(settings, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "etc", "unattended-settings.json")
	if err := os.Symlink(settings, link); err != nil {
		t.Fatal(err)
	}

	live := exec.Command("sh", script)
	live.Env = append(os.Environ(), "MELLIONS_SETTINGS="+link)
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Process.Kill(); _ = live.Wait() })
	if err := os.WriteFile(filepath.Join(shifts, "runner.lock"), []byte(strconv.Itoa(live.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	state, detail := runnerState(root, loadPath)
	if state != "STOPPED" || !strings.Contains(detail, "a checkout of this repository at "+superseded) {
		t.Fatalf("got %q %q, want STOPPED naming the superseded checkout %s", state, detail, superseded)
	}
	if strings.Contains(detail, "this host's own file") {
		t.Fatalf("a superseded checkout's deploy/ reported as this host's own file: %s", detail)
	}
}

// The tail of a path containing a space is itself absolute when the directory
// before the space ends there, so `filepath.IsAbs` on a token re-split out of a
// flattened ps line is not a test of exactness: the token resolves, sits inside
// the load path, and is not what the pid is executing. The row would then print
// a path the runner is not running and call it present — the split this row
// exists to name, certified. The /proc reading of the same process is the
// control: it holds the argument whole and says STOPPED.
func TestRunnerStateDoesNotBelieveAnAbsoluteFragmentOfAPSLine(t *testing.T) {
	root := t.TempDir()
	loadPath := filepath.Join(root, "loadpath")
	shifts := filepath.Join(root, "shifts")
	inside := filepath.Join(loadPath, "scripts", "shifts.sh")
	script := filepath.Join(root, "backup ") + inside
	for _, d := range []string{filepath.Dir(inside), shifts, filepath.Dir(script)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{inside, script} {
		if err := os.WriteFile(f, []byte("sleep 30\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	live := exec.Command("sh", script)
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Process.Kill(); _ = live.Wait() })
	if err := os.WriteFile(filepath.Join(shifts, "runner.lock"), []byte(strconv.Itoa(live.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := procRoot
	t.Cleanup(func() { procRoot = restore })
	procRoot = filepath.Join(root, "no-proc")

	state, detail := runnerState(root, loadPath)
	if state == "present" {
		t.Fatalf("state = %q for a pid executing %s: a fragment of the ps line was believed — %s", state, script, detail)
	}
	if !strings.Contains(detail, "unrecoverable") {
		t.Fatalf("detail = %q, want it to say the path could not be recovered", detail)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stockPATH is what a shift launched by launchd, cron or a systemd timer
// actually gets: the system directories, with no Homebrew and so no GNU
// coreutils. An interactive shell on this machine has more, which is the whole
// reason both defects here stayed invisible.
const stockPATH = "/usr/bin:/bin:/usr/sbin:/sbin"

// TestShiftRunsWithoutGNUCoreutils runs the shift end to end with GNU
// coreutils off PATH, under the bash the shebang actually resolves to.
//
// Two constructs used to need GNU tools and neither said so. `"${TIMEOUT[@]}"`
// with TIMEOUT empty is an unbound-variable death under `set -u` in bash 3.2 —
// which is /bin/bash on macOS — so with no timeout(1) the shift died at the
// line that launches the session. `tail --pid=` is GNU-only, so with the BSD
// tail macOS ships the follower produced nothing, the reply stayed empty, and
// an hour of work was filed as a session that said nothing.
func TestShiftRunsWithoutGNUCoreutils(t *testing.T) {
	root := repoRoot(t)

	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		t.Skipf("%s: %v", shell, err)
	}

	stub := t.TempDir()
	home := t.TempDir()
	path := stub + string(os.PathListSeparator) + stockPATH

	// The premise, asserted rather than assumed: on a host where PATH still
	// carries GNU coreutils this test would pass without exercising either
	// defect, so it must not silently claim to have proven anything.
	if p, err := lookIn(path, "timeout"); err == nil {
		t.Skipf("timeout(1) is on the stripped PATH at %s — this host cannot pose the question the test asks", p)
	}
	if p, err := lookIn(path, "gtimeout"); err == nil {
		t.Skipf("gtimeout(1) is on the stripped PATH at %s — this host cannot pose the question the test asks", p)
	}
	if tailAcceptsPID(t, path) {
		t.Skip("tail on the stripped PATH accepts --pid — this host has GNU tail, and cannot pose the question the test asks")
	}

	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3: %v", err)
	}

	write(t, filepath.Join(stub, "mellions"), `#!/bin/sh
# The shift asks where this installation's home is and refuses without an
# answer, so a stub that only exits 0 stops the script before it runs.
[ "$1" = config ] && case "$2" in
  home)    echo "$MELLIONS_HOME"; exit 0 ;;
  reports) echo "$MELLIONS_HOME/reports"; exit 0 ;;
esac
exit 0
`)
	// A paragraph and a result: the follower writes the reply only on the
	// result event, so both are needed for the shift to take its normal path
	// rather than the empty-reply backstop.
	write(t, filepath.Join(stub, "claude"), `#!/bin/sh
cat >/dev/null
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"stub shift"}]}}'
echo '{"type":"result","result":"the session did speak"}'
`)

	task := filepath.Join(stub, "task.md")
	if err := os.WriteFile(task, []byte("do nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(shell, filepath.Join(root, "scripts", "shift.sh"))
	cmd.Dir = home
	cmd.Env = []string{
		"PATH=" + path,
		"HOME=" + home,
		"MELLIONS_HOME=" + home,
		"MELLIONS_WORKDIR=" + home,
		"MELLIONS_BIN=" + filepath.Join(stub, "mellions"),
		"CLAUDE_BIN=" + filepath.Join(stub, "claude"),
		// python3 is located from the test's own environment rather than the
		// stripped PATH: which interpreter runs the follower is not what this
		// test is about, and a host that keeps python3 outside /usr/bin would
		// otherwise fail for an unrelated reason.
		"MELLIONS_PYTHON=" + python,
		"MELLIONS_PROMPT=" + task,
	}
	out, err := cmd.CombinedOutput()
	t.Logf("shift output:\n%s", out)
	if err != nil {
		t.Fatalf("shift.sh exited %v without GNU coreutils on PATH — a shift launched by launchd or a timer gets exactly this environment", err)
	}

	if got := string(out); strings.Contains(got, "unbound variable") {
		t.Error(`shift.sh died on an unbound variable: an empty array expanded as "${X[@]}" is fatal under set -u in bash 3.2, and must be written ${X[@]+"${X[@]}"}`)
	}
	if strings.Contains(string(out), "said nothing") {
		t.Error("the shift filed the session as having said nothing, but the session emitted a result event — the follower lost the reply")
	}

	ids := stampsIn(t, filepath.Join(home, "shifts"))
	if len(ids) != 1 {
		t.Fatalf("expected one shift, got %d (%v)", len(ids), ids)
	}
	reply, err := os.ReadFile(filepath.Join(home, "shifts", ids[0]+".reply.md"))
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if strings.TrimSpace(string(reply)) != "the session did speak" {
		t.Errorf("reply is %q, want the session's result — a shift that worked was recorded as silent", string(reply))
	}
}

// TestShiftFollowerStopsWhenTheSessionIsGone: `tail --pid=` ended the follower
// when the session process died, and dropping it for portability has to keep
// that. A follower that outlives its session leaks a process per shift; one
// that exits before draining loses the last thing the session said.
func TestShiftFollowerStopsWhenTheSessionIsGone(t *testing.T) {
	root := repoRoot(t)
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3: %v", err)
	}

	dir := t.TempDir()
	stream := filepath.Join(dir, "stream.jsonl")
	reply := filepath.Join(dir, "reply.md")

	// A stand-in for the session: it holds a pid, says nothing, and ends
	// without ever emitting a result event — a session killed by its timeout,
	// which is precisely when the follower has no result to stop on.
	session := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := session.Start(); err != nil {
		t.Fatal(err)
	}
	pid := session.Process.Pid

	follower := exec.Command(python, "-u", filepath.Join(root, "scripts", "shift-follow.py"),
		reply, stream, strconv.Itoa(pid))
	var progress strings.Builder
	follower.Stdout = &progress
	follower.Stderr = &progress
	if err := follower.Start(); err != nil {
		t.Fatal(err)
	}

	// Written after the follower starts, so it is following a growing file
	// rather than reading a finished one.
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(stream, []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	_ = session.Process.Kill()
	_ = session.Wait() // reap, so the pid stops existing for the follower

	done := make(chan error, 1)
	go func() { done <- follower.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("follower exited %v: %s", err, progress.String())
		}
	case <-time.After(15 * time.Second):
		_ = follower.Process.Kill()
		t.Fatal("the follower outlived its session — without --pid it needs its own reason to stop, and a shift that leaves one behind leaks a process per run")
	}

	if !strings.Contains(progress.String(), "go test ./...") {
		t.Errorf("the follower rendered no progress for a line written while it was watching; got %q", progress.String())
	}
}

// lookIn resolves name against an explicit PATH rather than the test process's.
func lookIn(path, name string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

// tailAcceptsPID reports whether the tail on this PATH is GNU tail.
func tailAcceptsPID(t *testing.T, path string) bool {
	t.Helper()
	tail, err := lookIn(path, "tail")
	if err != nil {
		return false
	}
	f := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Without -f/-F, GNU tail accepts --pid, prints the file and exits 0; BSD
	// tail rejects the option outright. Neither form follows, so neither hangs.
	cmd := exec.Command(tail, "-n", "+1", "--pid=1", f)
	cmd.Env = []string{"PATH=" + path}
	return cmd.Run() == nil
}

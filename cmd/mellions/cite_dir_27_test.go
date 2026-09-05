package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// #27. The hook built one resolver from the session's cwd, so a body published
// by `cd <worktree> && gh pr create` was checked against the shared checkout.
// Both directions of that are wrong and only one of them is loud:
//
//   - loud:      a citation right for the lane is DENIED, and the session sees it.
//   - dangerous: a citation right only for the shared checkout is ACCEPTED, and
//     nobody sees it. That one publishes a wrong claim under a passing check,
//     and it is the arm these tests exist for.
//
// The fixture is two real checkouts whose line 3 differs in both directions, so
// neither verdict can be reached by accident.

func citeFixture(t *testing.T) (session, lane string) {
	t.Helper()
	root := t.TempDir()
	session = filepath.Join(root, "shared")
	lane = filepath.Join(root, "lane")

	for dir, line3 := range map[string]string{
		session: "\thealth, err := neonlink.NewHealthChecker(sdkCfg)",
		lane:    "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v: %s", args, err, out)
			}
		}
		body := "package broker\n\n" + line3 + "\n"
		if err := os.WriteFile(filepath.Join(dir, "neonlink.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return session, lane
}

// runCiteHook drives the real entry point: a PreToolUse payload on stdin, the
// decision on stdout. It returns the deny reason, or "" for silence.
func runCiteHook(t *testing.T, sessionCwd, command string) string {
	t.Helper()
	t.Setenv("MELLIONS_HOOK", "1")

	payload, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        sessionCwd,
		"tool_input": map[string]string{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}

	in := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(in, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	stdinFile, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer stdinFile.Close()

	outPath := filepath.Join(t.TempDir(), "out.json")
	stdoutFile, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinFile, stdoutFile
	err = cmdCiteCheck(context.Background(), nil)
	os.Stdin, os.Stdout = oldIn, oldOut
	stdoutFile.Close()
	if err != nil {
		t.Fatalf("cite-check: %v", err)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return ""
	}
	var d struct {
		Output struct {
			Decide string `json:"permissionDecision"`
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("decode decision %q: %v", raw, err)
	}
	if d.Output.Decide != "deny" {
		return ""
	}
	return d.Output.Reason
}

func citeBody(t *testing.T, dir, quoted string) string {
	t.Helper()
	path := filepath.Join(dir, "body.md")
	doc := "Root cause.\n\n`neonlink.go:3`\n\n```go\n" + quoted + "\n```\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// THE ARM THAT COUNTS. A body quoting the SHARED checkout's line 3 is wrong for
// the tree it is published from. Before #27 the hook read that same shared
// checkout, found the quote matched, and approved it — a wrong citation
// published under a passing check.
func TestCiteHook_RejectsACitationRightOnlyForTheSessionCheckout(t *testing.T) {
	session, lane := citeFixture(t)
	body := citeBody(t, t.TempDir(), "\thealth, err := neonlink.NewHealthChecker(sdkCfg)")

	reason := runCiteHook(t, session, "cd "+lane+" && gh pr create --base dev --body-file "+body)

	if reason == "" {
		t.Fatal("the hook approved a citation whose quoted line is right only for the session " +
			"checkout and wrong in the worktree the body is published from — #27's dangerous " +
			"direction, which publishes a false claim with nothing to show for it")
	}
	if !strings.Contains(reason, "neonlink.go:3") {
		t.Fatalf("denied, but not for the citation under test: %s", reason)
	}
}

// The loud direction, and the control that stops the fix from being "deny more".
// A body quoting the LANE's line 3 is correct for the tree it is published from
// and must pass.
func TestCiteHook_AcceptsACitationCorrectForTheWorktreeItPublishesFrom(t *testing.T) {
	session, lane := citeFixture(t)
	body := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")

	if reason := runCiteHook(t, session, "cd "+lane+" && gh pr create --base dev --body-file "+body); reason != "" {
		t.Fatalf("a citation correct for the worktree was denied: %s", reason)
	}
}

// Without a cd the session directory is still the subject, so the fix does not
// change what a plain publish is checked against.
func TestCiteHook_WithoutACdStillChecksTheSessionCheckout(t *testing.T) {
	session, _ := citeFixture(t)
	wrong := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")
	right := citeBody(t, t.TempDir(), "\thealth, err := neonlink.NewHealthChecker(sdkCfg)")

	if reason := runCiteHook(t, session, "gh pr create --base dev --body-file "+wrong); reason == "" {
		t.Fatal("a citation wrong for the session checkout passed a publish that never left it")
	}
	if reason := runCiteHook(t, session, "gh pr create --base dev --body-file "+right); reason != "" {
		t.Fatalf("a citation correct for the session checkout was denied: %s", reason)
	}
}

// A cd to a directory this host does not hold must not become a root that
// resolves nothing: a resolver that resolves nothing reports every citation
// unbacked-but-not-a-citation and passes the body silently, which would turn
// the fix into a wider hole than the defect.
func TestCiteHook_UnreachableCdFallsBackToTheSessionCheckout(t *testing.T) {
	session, _ := citeFixture(t)
	wrong := citeBody(t, t.TempDir(), "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)")

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if reason := runCiteHook(t, session, "cd "+missing+" && gh pr create --base dev --body-file "+wrong); reason == "" {
		t.Fatal("a cd to a nonexistent directory silenced the checker instead of falling back " +
			"to the session checkout")
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestAGuardRunByHandDoesNotWaitOnAPipe is the regression for the defect that
// took pr-body-check and shared-tree-check out of use for a whole production
// session.
//
// The payload read was guarded only against a terminal, so an inherited
// descriptor nobody closes — which is what a tool harness, a script and CI all
// hand a command — blocked the read for good. The read now happens only where a
// hook says a payload is coming.
func TestAGuardRunByHandDoesNotWaitOnAPipe(t *testing.T) {
	// A pipe with a writer that never writes and never closes is exactly the
	// descriptor the hang needed. Reading it must return rather than block.
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = read.Close(); _ = write.Close() })
	t.Setenv("MELLIONS_HOOK", "")

	done := make(chan []byte, 1)
	go func() { raw, _ := readPayload(read); done <- raw }()
	select {
	case got := <-done:
		if len(got) != 0 {
			t.Fatalf("a hand-run guard read %d bytes from a pipe no hook wrote", len(got))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readPayload blocked on an open pipe with no hook set; this is the hang")
	}
}

// TestAHookStillGetsItsPayload holds the other direction: the fix must not
// deafen the guards, which would silently disable every PreToolUse denial and
// look exactly like a tree with nothing to deny.
func TestAHookStillGetsItsPayload(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = read.Close() })
	t.Setenv("MELLIONS_HOOK", "1")

	const body = `{"tool_name":"Bash","tool_input":{"command":"gh pr create"}}`
	go func() { _, _ = write.WriteString(body); _ = write.Close() }()

	done := make(chan []byte, 1)
	go func() { raw, _ := readPayload(read); done <- raw }()
	select {
	case got := <-done:
		if string(got) != body {
			t.Fatalf("a hook was handed %q, want %q", got, body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readPayload did not return a payload a hook wrote")
	}
}

// TestATerminalCarriesNoPayload keeps the original guard: a person at a prompt
// hands nothing over even inside a hook environment.
func TestATerminalCarriesNoPayload(t *testing.T) {
	t.Setenv("MELLIONS_HOOK", "1")
	tty, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		t.Skip("no /dev/null to stand in for a descriptor")
	}
	t.Cleanup(func() { _ = tty.Close() })
	got, err := readPayload(tty)
	if len(got) != 0 || err != nil {
		t.Fatalf("a character device gave %d bytes and err %v, want no payload expected", len(got), err)
	}
}

// everyGuard is each command that answers a PreToolUse payload. A new guard that
// reads its call any way other than guardPayload belongs in this table.
var everyGuard = map[string]func() error{
	"secret-check":      func() error { return cmdSecretCheck(nil) },
	"pr-body-check":     func() error { return cmdPRBodyCheck(context.Background(), nil) },
	"shared-tree-check": func() error { return cmdSharedTreeCheck(nil) },
	"pr-merge-check":    func() error { return cmdPRMergeCheck(context.Background(), nil) },
	"cite-check":        func() error { return cmdCiteCheck(context.Background(), nil) },
}

func cmdSecretCheckNoArgs() error { return cmdSecretCheck(nil) }

// runGuard hands payload to a guard the way hooks/lib.sh does and returns what
// it printed on stdout, which is all the runtime reads.
func runGuard(t *testing.T, guard func() error, hook, payload string) string {
	t.Helper()
	t.Setenv("MELLIONS_HOOK", hook)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MELLIONS_CONFIG", filepath.Join(t.TempDir(), "none.json"))
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, stdout, stderr := os.Stdin, os.Stdout, os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin, os.Stdout, os.Stderr = inR, outW, devnull
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = stdin, stdout, stderr
		_ = devnull.Close()
		_ = inR.Close()
	}()
	go func() { _, _ = inW.WriteString(payload); _ = inW.Close() }()
	printed := make(chan string, 1)
	go func() { b, _ := io.ReadAll(outR); printed <- string(b) }()
	runErr := guard()
	_ = outW.Close()
	if runErr != nil {
		t.Fatalf("guard returned %v", runErr)
	}
	return <-printed
}

// A decision the runtime can act on, read as the runtime reads it.
func decided(t *testing.T, out string) (deny bool, reason string) {
	t.Helper()
	if out == "" {
		return false, ""
	}
	var d decision
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("stdout is not a decision the runtime can parse: %v\n%s", err, out)
	}
	return d.Output.Decide == "deny", d.Output.Reason
}

// The runtime reads a guard's silence as allow, so a payload a guard cannot read
// whole is refused by every guard and is never silence. Over the cap is the one
// case that needs no late writer: the read used to cut it to invalid JSON.
func TestEveryGuardRefusesAPayloadItCannotRead(t *testing.T) {
	whole := bashPayload(t, "cat .db_connection")
	over := bashPayload(t, "echo "+strings.Repeat("x", maxGuardPayload))
	for name, c := range map[string]string{
		"cut by two bytes":     whole[:len(whole)-2],
		"cut in half":          whole[:len(whole)/2],
		"not json":             "cat .db_connection",
		"nothing":              "",
		"an array":             `[]`,
		"a string":             `"cat .db_connection"`,
		"null":                 `null`,
		"names no tool":        `{"tool_input":{"command":"cat .db_connection"}}`,
		"a command number":     `{"tool_name":"Bash","tool_input":{"command":7}}`,
		"input as a word":      `{"tool_name":"Bash","tool_input":"cat .db_connection"}`,
		"over the bound":       over,
		"a read's path a list": `{"tool_name":"Read","tool_input":{"file_path":["a","b"]}}`,
	} {
		for guard, run := range everyGuard {
			t.Run(name+"/"+guard, func(t *testing.T) {
				deny, reason := decided(t, runGuard(t, run, "1", c))
				if !deny || !strings.Contains(reason, "not examined") {
					t.Errorf("want the unread refusal, got deny=%v reason=%q", deny, reason)
				}
				if name == "over the bound" && !strings.Contains(reason, "16 MiB") {
					t.Errorf("an oversized payload was not refused as one: %q", reason)
				}
				if strings.Contains(reason, ".db_connection") {
					t.Errorf("the refusal echoed the payload: %q", reason)
				}
			})
		}
	}
}

// A call cut short may be one too large to read in the hook's window, where
// retrying it unchanged only meets the same refusal; other garbage is not.
func TestACutPayloadIsToldToSplitOnlyWhereThatCouldHelp(t *testing.T) {
	whole := bashPayload(t, "echo hi")
	for name, c := range map[string]struct {
		payload string
		split   bool
	}{
		"cut":      {whole[:len(whole)/2], true},
		"not json": {"echo hi", false},
		"mistyped": {`{"tool_name":"Bash","tool_input":{"command":7}}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			_, reason := decided(t, runGuard(t, cmdSecretCheckNoArgs, "1", c.payload))
			if got := strings.Contains(reason, "split one that large"); got != c.split {
				t.Errorf("split advice %v, want %v: %q", got, c.split, reason)
			}
		})
	}
}

// guardEvent is the one decode the credential guard reads a path through, so each
// field it names has to reach the decision.
func TestTheCredentialGuardReadsEveryPathTheEventCarries(t *testing.T) {
	for name, payload := range map[string]string{
		"Read file_path":        `{"tool_name":"Read","tool_input":{"file_path":"/srv/.db_connection"}}`,
		"Grep path":             `{"tool_name":"Grep","tool_input":{"pattern":"x","path":"/srv/.db_connection"}}`,
		"NotebookRead notebook": `{"tool_name":"NotebookRead","tool_input":{"notebook_path":"/srv/.db_connection"}}`,
		"Bash command":          `{"tool_name":"Bash","tool_input":{"command":"cat /srv/.db_connection"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			deny, reason := decided(t, runGuard(t, cmdSecretCheckNoArgs, "1", payload))
			if !deny || strings.Contains(reason, "not examined") {
				t.Errorf("want the guard's own denial, got deny=%v reason=%q", deny, reason)
			}
		})
	}
}

// The other direction: the refusal must not spread to a payload a guard can
// read. Every guard stays silent on an ordinary call, on a tool it has no field
// for, and when no hook is running at all.
func TestEveryGuardStaysSilentOnWhatItCanRead(t *testing.T) {
	for name, c := range map[string]struct{ hook, payload string }{
		"an ordinary command": {"1", bashPayload(t, "echo hi")},
		"a tool with no command": {"1",
			`{"tool_name":"Agent","cwd":"/home/you","tool_input":{"prompt":"look","count":3}}`},
		"a field no guard reads": {"1",
			`{"tool_name":"Bash","extra":[1,2],"tool_input":{"command":"echo hi","timeout":5}}`},
		"a field of a guarded name on a tool no guard reads": {"1",
			`{"tool_name":"mcp__fs__ReadMany","tool_input":{"path":["a","b"],"command":7}}`},
		"run by hand, a payload piped": {"", bashPayload(t, "cat .db_connection")},
		"run by hand, garbage piped":   {"", "not json"},
	} {
		for guard, run := range everyGuard {
			t.Run(name+"/"+guard, func(t *testing.T) {
				if out := runGuard(t, run, c.hook, c.payload); out != "" {
					t.Errorf("want silence, got:\n%s", out)
				}
			})
		}
	}
}

// A payload past the old 1 MiB cut is decided on what it says, by the guard's own
// reason rather than the unread refusal.
func TestAGuardDecidesALargePayloadWhole(t *testing.T) {
	pad := "; : " + strings.Repeat("x", 1100*1024)
	for guard, c := range map[string]struct {
		run     func() error
		command string
		says    string
	}{
		"secret-check":      {everyGuard["secret-check"], "cat .db_connection" + pad, "read a credential"},
		"pr-body-check":     {everyGuard["pr-body-check"], `git commit -m "Closes #7"` + pad, "#7"},
		"shared-tree-check": {everyGuard["shared-tree-check"], "pkill -f worker" + pad, "pkill"},
	} {
		t.Run(guard, func(t *testing.T) {
			deny, reason := decided(t, runGuard(t, c.run, "1", bashPayload(t, c.command)))
			if !deny || strings.Contains(reason, "not examined") || !strings.Contains(reason, c.says) {
				t.Errorf("want the guard's own denial naming %q, got deny=%v reason=%.200q", c.says, deny, reason)
			}
		})
	}
}

// A hook the runtime kills is allowed by the runtime, so hooks/lib.sh's own
// bounds (2 s of silence, 8 s for the binary) must end first for each guard.
func TestTheRuntimeOutwaitsEveryGuardsOwnBound(t *testing.T) {
	lib, err := os.ReadFile(filepath.Join("..", "..", "hooks", "lib.sh"))
	if err != nil {
		t.Fatal(err)
	}
	window := regexp.MustCompile(`(?m)^payload_window=(\d+)$`).FindSubmatch(lib)
	limit := regexp.MustCompile(`timeout -k (\d+) (\d+) "\$mellions"`).FindSubmatch(lib)
	if window == nil || limit == nil {
		t.Fatalf("lib.sh no longer states its bounds where this reads them: window %q, timeout %q", window, limit)
	}
	// SECONDS counts whole seconds, so the read can run a second past its window.
	own := 1
	for _, n := range [][]byte{window[1], limit[1], limit[2]} {
		v, err := strconv.Atoi(string(n))
		if err != nil {
			t.Fatal(err)
		}
		own += v
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
					Timeout int    `json:"timeout"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	seen, registered := map[string]bool{}, map[string]bool{}
	for _, group := range m.Hooks.PreToolUse {
		for _, h := range group.Hooks {
			for _, script := range []string{"secret-check.sh", "pr-reference.sh", "shared-tree.sh"} {
				if strings.Contains(h.Command, script) {
					seen[script] = true
					for _, tool := range strings.Split(group.Matcher, "|") {
						registered[tool] = true
					}
					if h.Timeout <= own {
						t.Errorf("%s: the runtime kills it at %d s, inside lib.sh's own bound of %d s", script, h.Timeout, own)
					}
				}
			}
		}
	}
	if len(seen) != 3 {
		t.Errorf("found %v, want all three guard scripts registered", seen)
	}
	for tool := range registered {
		if !guardedTools[tool] {
			t.Errorf("hooks.json registers a guard for %s, which guardedTools does not name", tool)
		}
	}
	for tool := range guardedTools {
		if !registered[tool] {
			t.Errorf("guardedTools names %s, for which hooks.json registers no guard", tool)
		}
	}
}

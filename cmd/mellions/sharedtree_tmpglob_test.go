// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// runSharedTreeCheck hands payload to shared-tree-check the way hooks/lib.sh
// does — on stdin, with MELLIONS_HOOK set — and returns what it printed.
func runSharedTreeCheck(t *testing.T, payload string) string {
	t.Helper()
	t.Setenv("MELLIONS_HOOK", "1")
	t.Setenv("HOME", t.TempDir())
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW
	t.Cleanup(func() { os.Stdin, os.Stdout = stdin, stdout })
	go func() { _, _ = inW.WriteString(payload); _ = inW.Close() }()
	runErr := cmdSharedTreeCheck(nil)
	_ = outW.Close()
	os.Stdin, os.Stdout = stdin, stdout
	out, _ := io.ReadAll(outR)
	_ = inR.Close()
	if runErr != nil {
		t.Fatalf("shared-tree-check: %v", runErr)
	}
	return string(out)
}

func bashPayload(t *testing.T, command string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        "/home/you",
		"tool_input": map[string]string{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The glob-delete refusal is reached through the command a hook runs, not only
// through the package: a payload that globs over /tmp is denied, recursive or
// not, and one naming an exact path is silence.
func TestSharedTreeCheckDeniesAGlobDeleteOverTmp(t *testing.T) {
	for _, cmd := range []string{`rm -f /tmp/tmp.*`, `rm -rf /tmp/tmp.*`} {
		out := runSharedTreeCheck(t, bashPayload(t, cmd))
		if !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(out, "/tmp/tmp.*") {
			t.Errorf("%s: want a deny naming the glob, got %q", cmd, out)
		}
	}
	if out := runSharedTreeCheck(t, bashPayload(t, `rm -f /tmp/tmp.Ab12Cd34Ef`)); out != "" {
		t.Errorf("an exact path was refused: %q", out)
	}
}

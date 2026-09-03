// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
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
	go func() { done <- readPayload(read) }()
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
	go func() { done <- readPayload(read) }()
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
	if got := readPayload(tty); len(got) != 0 {
		t.Fatalf("read %d bytes from an empty descriptor", len(got))
	}
}

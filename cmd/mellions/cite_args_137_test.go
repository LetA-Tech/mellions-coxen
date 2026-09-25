package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A body named without -file must not be silently replaced by stdin, and an
// empty document must not pass as a clean one.
func TestCiteCheckRefusesAnUnreadDocument(t *testing.T) {
	body := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(body, []byte("See `cmd/mellions/cite.go:1`:\n```go\nnot that line\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := cmdCite(context.Background(), []string{"check", body})
	if err == nil || !strings.Contains(err.Error(), "-file") {
		t.Errorf("a positional document: err = %v, want a refusal naming -file", err)
	}
	err = cmdCite(context.Background(), []string{"check", "-file", empty})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("an empty document: err = %v, want a refusal", err)
	}
	// Positive control: the same body named with -file is read and refused.
	err = cmdCite(context.Background(), []string{"check", "-file", body, "-dir", "."})
	if err == nil || !strings.Contains(err.Error(), "does not back") {
		t.Errorf("the body read with -file: err = %v, want its unbacked citation reported", err)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The posting hook grades the paragraph after a list whose item holds a fence
// the item's end closed: a wrong quotation denies and the true one is silent.
func TestCiteHook_AParagraphAfterAnItemsFenceIsGraded(t *testing.T) {
	root := citeGitRepo(t)
	for name, c := range map[string]struct {
		quote string
		deny  bool
	}{
		"true quote":  {"package a", false},
		"wrong quote": {"package aXX", true},
	} {
		body := filepath.Join(root, "b.md")
		doc := "- a\n  ```\n  code\n\nAfter the list, `internal/a.go:1`: `" + c.quote + "`\n"
		if err := os.WriteFile(body, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		var out string
		runCiteHookRaw(t, root, "gh pr create --base dev --body-file "+body, &out)
		if got := strings.Contains(out, `"permissionDecision":"deny"`); got != c.deny {
			t.Errorf("%s: denied = %v, want %v.\ngot: %s", name, got, c.deny, out)
		}
	}
}

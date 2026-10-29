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

// The posting hook grades a citation written on a list line indented four
// spaces, as it does one indented two: a wrong quotation denies and the true
// one is silent.
func TestCiteHook_ACitationOnAFourSpaceNestedLineIsGraded(t *testing.T) {
	root := citeGitRepo(t)
	for name, c := range map[string]struct {
		indent string
		quote  string
		deny   bool
	}{
		"two spaces, true quote":   {"  ", "package a", false},
		"two spaces, wrong quote":  {"  ", "package aXX", true},
		"four spaces, true quote":  {"    ", "package a", false},
		"four spaces, wrong quote": {"    ", "package aXX", true},
		"a tab, wrong quote":       {"\t", "package aXX", true},
	} {
		body := filepath.Join(root, "b.md")
		doc := "- parent\n" + c.indent + "- read, `internal/a.go:1`: `" + c.quote + "`\n"
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

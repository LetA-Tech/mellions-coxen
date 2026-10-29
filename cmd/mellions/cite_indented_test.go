// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mellions cite check, and its PreToolUse form cite-check, grade a citation
// written on a list line indented four spaces as they do one indented two: a
// wrong quotation is refused and the true one passes.
func TestCite_ACitationOnAFourSpaceNestedLineIsGraded(t *testing.T) {
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
		err := cmdCite(context.Background(), []string{"check", "-file", body, "-dir", root})
		if got := err != nil; got != c.deny {
			t.Errorf("%s: cite check refused = %v, want %v (err: %v)", name, got, c.deny, err)
		}
		var out string
		runCiteHookRaw(t, root, "gh pr create --base dev --body-file "+body, &out)
		if got := strings.Contains(out, `"permissionDecision":"deny"`); got != c.deny {
			t.Errorf("%s: cite-check denied = %v, want %v.\ngot: %s", name, got, c.deny, out)
		}
	}
}

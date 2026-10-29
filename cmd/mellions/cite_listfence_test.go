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

// mellions cite check, and its PreToolUse form cite-check, grade the paragraph
// after a list whose item holds a fence the item's end closed: a wrong
// quotation is refused and the true one passes.
func TestCite_AParagraphAfterAnItemsFenceIsGraded(t *testing.T) {
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

// A deny made of lines the body shows outside any quotation says the markup
// changes if the number and checkout are right, and claims neither; beside an
// ordinary refusal it names a third case.
func TestCiteHook_AnUnquotedLineDenyClaimsOnlyWhatTheCheckKnows(t *testing.T) {
	root := citeGitRepo(t)
	for file, text := range map[string]string{
		"f.go":    "package f\nfunc f() {\n\treturn\n}\n",
		"docs.md": "## Scope\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	margin := "- Read `internal/a.go:1`:\n  ```go\npackage a\n  ```\n"
	wrong := "Read `top.txt:1`: `y`\n"
	only := []string{"outside any quotation", "If the number and the checkout are right", "content column", "fence or blockquote", "settle that first"}
	claims := []string{"aragraph", "renders", "The number and the checkout are right", "Two things produce this"}
	for name, c := range map[string]struct {
		doc     string
		want    []string
		notWant []string
	}{
		"fence code at the margin":     {margin, only, claims},
		"a brace in a pasted function": {"- Read `f.go:4`:\n  ```go\nfunc f() {\n\treturn\n}\n  ```\n", only, claims},
		"a heading":                    {"Read `docs.md:1`:\n\n## Scope\n", only, claims},
		"an ordinary refusal": {wrong, []string{"The number may be wrong", "wrong subject"},
			[]string{"outside any quotation", "third case"}},
		"both": {wrong + "\n" + margin, []string{"outside any quotation", "The number may be wrong", "third case beside the two below"}, []string{"aragraph", "The number and the checkout are right"}},
	} {
		body := filepath.Join(root, "b.md")
		if err := os.WriteFile(body, []byte(c.doc), 0o644); err != nil {
			t.Fatal(err)
		}
		reason := runCiteHook(t, root, "gh pr create --base dev --body-file "+body)
		if reason == "" {
			t.Errorf("%s: not denied", name)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(reason, w) {
				t.Errorf("%s: the deny does not say %q:\n%s", name, w, reason)
			}
		}
		for _, w := range c.notWant {
			if strings.Contains(reason, w) {
				t.Errorf("%s: the deny says %q:\n%s", name, w, reason)
			}
		}
	}
}

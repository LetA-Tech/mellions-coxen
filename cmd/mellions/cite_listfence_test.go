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

// A deny made of quotes GitHub renders as paragraph text says to fix the markup,
// not the number or the checkout; beside an ordinary refusal it names both.
func TestCiteHook_AParagraphTextDenyAdvisesTheMarkup(t *testing.T) {
	root := citeGitRepo(t)
	paragraph := "- Read `internal/a.go:1`:\n  ```go\npackage a\n  ```\n"
	wrong := "Read `top.txt:1`: `y`\n"
	for name, c := range map[string]struct {
		doc     string
		want    []string
		notWant []string
	}{
		"only paragraph text": {paragraph, []string{"paragraph text", "content column", "fence or blockquote", "The number and the checkout are right"},
			[]string{"The number may be wrong", "wrong subject"}},
		"an ordinary refusal": {wrong, []string{"The number may be wrong", "wrong subject"},
			[]string{"paragraph text", "neither case below"}},
		"both": {wrong + "\n" + paragraph, []string{"paragraph text", "The number may be wrong", "neither case below"}, nil},
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

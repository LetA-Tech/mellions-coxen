// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/prmerge"
)

// A merge lands the body the tracker holds at that moment, so the body's
// citations are graded at the pull request's head commit — not at the working
// tree the merge runs from, and not at all where that checkout lacks the head.
// The branch is current, so nothing but the body can refuse.
func TestPRMergeRefusesABodyItsHeadDoesNotBack(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		base = "dev"
	)
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	file := filepath.Join(dir, "internal", "x", "a.go")
	write := func(word string) {
		t.Helper()
		src := "package x\n\nfunc A() string {\n\treturn \"" + word + "\"\n}\n"
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", base)
	write("head")
	git("add", ".")
	git("commit", "-q", "-m", "head")
	head := git("rev-parse", "HEAD")
	// The checkout the merge runs from has moved on from the head under review.
	write("tree")

	payload, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        dir,
		"tool_input": map[string]string{"command": "gh pr merge 97 --repo " + repo + " --merge --match-head-commit " + head},
	})
	if err != nil {
		t.Fatal(err)
	}
	current := `{"ahead":0,"files":[]}`
	decide := func(body, at string) string {
		t.Helper()
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		prView := `{"number":97,"url":"https://github.com/` + repo + `/pull/97",` +
			`"baseRefName":"` + base + `","headRefOid":"` + at + `",` +
			`"mergeStateStatus":"CLEAN","state":"OPEN","body":` + string(b) + `}`
		stub := stubLook(t, prView, current, "", at, base, repo)
		// gh answers only the fields it is asked for: a view that does not ask
		// for the body gets none.
		read := func(ctx context.Context, d, name string, args ...string) (string, error) {
			out, err := stub(ctx, d, name, args...)
			if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "view" &&
				!strings.Contains(","+args[len(args)-1]+",", ",body,") {
				out = strings.Replace(out, `,"body":`+string(b), "", 1)
			}
			return out, err
		}
		return prmerge.Deny(payload, func(cwd string, call prmerge.Call) (prmerge.State, error) {
			return mergeStateFrom(context.Background(), cwd, call, read)
		})
	}
	quoting := func(line string) string {
		return "The fix is at `internal/x/a.go:4`:\n\n```go\n" + line + "\n```\n"
	}

	if reason := decide(quoting("\treturn \"head\""), head); reason != "" {
		t.Errorf("a body its head backs was refused:\n%s", reason)
	}

	reason := decide(quoting("\treturn \"tree\""), head)
	if reason == "" {
		t.Fatal("a body quoting a line the head does not have was merged")
	}
	for _, want := range []string{"internal/x/a.go:4", `return \"head\"`, head} {
		if !strings.Contains(reason, want) {
			t.Errorf("refusal does not name %q:\n%s", want, reason)
		}
	}

	if reason := decide("Past the end: `internal/x/a.go:40`.\n", head); !strings.Contains(reason, "internal/x/a.go:40") {
		t.Errorf("a citation past the end of the file at the head was not refused by name:\n%s", reason)
	}

	absent := "fedcba9876543210fedcba9876543210fedcba98"
	if reason := decide(quoting("\treturn \"tree\""), absent); reason != "" {
		t.Errorf("a head this checkout does not hold graded the body anyway:\n%s", reason)
	}

	if reason := decide("", head); reason != "" {
		t.Errorf("an empty body was refused:\n%s", reason)
	}
}

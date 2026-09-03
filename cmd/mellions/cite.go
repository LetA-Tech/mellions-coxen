// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/cite"
	"github.com/LetA-Tech/mellions-coxen/internal/prbody"
)

// citeLimit bounds a document read. A body past this is not a body.
const citeLimit = 1 << 20

// cmdCite checks a document's citations against the tree they name and prints
// the ones it cannot back. Exit 1 where there are findings, so a script or a
// person can gate on it; the hook form is cite-check.
func cmdCite(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New("cite: the subcommand is `check`")
	}
	fs := newFlagSet("cite check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("file", "-", "document to check, or - for stdin")
	dir := fs.String("dir", ".", "the checkout the citations are relative to")
	commit := fs.String("commit", "", "resolve citations at this commit rather than the working tree")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	doc, err := readDoc(*file)
	if err != nil {
		return err
	}
	findings := cite.Check(doc, resolver(ctx, *dir, *commit))
	if len(findings) == 0 {
		fmt.Println("cite: every citation this checkout can resolve is quoted in the body.")
		return nil
	}
	for _, f := range findings {
		fmt.Println("  " + f.Reason())
	}
	return fmt.Errorf("%d citation(s) the body does not back", len(findings))
}

// cmdCiteCheck reads a PreToolUse payload on stdin and denies a `gh` command
// that publishes a body carrying a citation the tree does not back. Everything
// else is silence.
//
// The rule this enforces is mellions-deep-research's own — "Open every
// citation before it is filed" — and it is enforced here rather than in the
// Skill because the Skill states it correctly and it kept failing to bind. The
// moment a body is handed to `gh` is the last one at which the claim is still
// retractable, and a denial becomes a tool result the session must answer.
func cmdCiteCheck(ctx context.Context, args []string) error {
	fs := newFlagSet("cite-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload := readPayload(os.Stdin)
	if len(payload) == 0 {
		guardUsage("cite-check", "It denies a `gh` command publishing a body that cites a "+
			"path:line this checkout resolves and the body does not quote. To check a body "+
			"by hand: `mellions cite check -file <path>`.")
		return nil
	}
	var ev struct {
		ToolName string `json:"tool_name"`
		Cwd      string `json:"cwd"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" {
		return nil
	}
	// This runs on every Bash call, so nothing costs anything until the command
	// turns out to publish: building the resolver runs git, and the overwhelming
	// majority of tool calls are not a gh body.
	calls := prbody.Publishing(ev.Input.Command, ev.Cwd)
	if len(calls) == 0 {
		return nil
	}
	cwd := ev.Cwd
	if cwd == "" {
		cwd = "."
	}
	read := resolver(ctx, cwd, "")
	var reasons []string
	for _, call := range calls {
		for _, body := range call.Bodies {
			for _, f := range cite.Check(body, read) {
				reasons = append(reasons, "  "+f.Reason())
			}
		}
	}
	if len(reasons) == 0 {
		return nil
	}
	// A deny is read, so it is bounded: a body that got its anchoring wrong
	// throughout wants the first several and a count, not a wall the session
	// scrolls past.
	reasons = dedupe(reasons)
	if n := len(reasons); n > 8 {
		reasons = append(reasons[:8], "  … and "+strconv.Itoa(n-8)+" more.")
	}
	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = "This body publishes a citation the checkout does not back:\n\n" +
		strings.Join(reasons, "\n") + "\n\n" +
		"A reader cannot tell a citation that landed one line off from one that landed " +
		"on the code it claims, so both read as evidence and one is not. Quote the line " +
		"under the citation — the form mellions-deep-research already asks for, and what " +
		"makes the claim checkable at all.\n\n" +
		"Two things produce this and the remedies differ. The number may be wrong, and " +
		"opening the line fixes it. Or this checkout is the wrong subject: a body about a " +
		"branch, a pull request or an older commit is right about that ref and wrong here, " +
		"and re-deriving the numbers against this tree would make it wrong there instead. " +
		"Check which before editing:\n\n" +
		"  mellions cite check -file body.md -dir <checkout> [-commit <ref>]\n\n" +
		"`-commit` resolves every citation at that ref — the branch under review, not the " +
		"one checked out — and reports the same thing without publishing anything."
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// resolver answers with a file's lines, at a commit where one is named and
// from the working tree otherwise. An error means the path is not a file this
// checkout holds, which the checker reads as "not a citation" rather than as a
// finding — a URL host, another repository's path, or prose that happens to
// carry a colon.
func resolver(ctx context.Context, dir, commit string) func(string) ([]string, error) {
	root := repoRoot(ctx, dir)
	return func(path string) ([]string, error) {
		if commit != "" {
			out, err := exec.CommandContext(ctx, "git", "-C", root, "show", commit+":"+path).Output()
			if err != nil {
				return nil, err
			}
			return split(string(out)), nil
		}
		// A path in a body is not this checker's to trust: it is read only
		// inside the checkout, and only as a regular file.
		full := filepath.Join(root, filepath.Clean("/"+path))
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("not a file in this checkout")
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		return split(string(b)), nil
	}
}

func split(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// repoRoot is the checkout a relative citation is relative to. Where dir is
// not in a work tree it is used as it stands, which is what a document checked
// outside a repository wants.
func repoRoot(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	if root := strings.TrimSpace(string(out)); root != "" {
		return root
	}
	return dir
}

func readDoc(file string) (string, error) {
	if file == "-" {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, citeLimit))
		return string(b), err
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, citeLimit))
	return string(b), err
}

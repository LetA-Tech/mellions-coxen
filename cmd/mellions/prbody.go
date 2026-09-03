// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/prbody"
)

// decision is the PreToolUse answer the runtime reads: one JSON object on
// stdout. Anything else on stdout is not a decision, and silence is consent.
type decision struct {
	Output struct {
		Event  string `json:"hookEventName"`
		Decide string `json:"permissionDecision"`
		Reason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// cmdPRBodyCheck reads a PreToolUse payload on stdin and denies a `gh pr
// create` or `gh pr edit` whose body declares that it closes an issue on a
// base branch where GitHub will not resolve it. Everything else is silence.
func cmdPRBodyCheck(ctx context.Context, args []string) error {
	fs := newFlagSet("pr-body-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload := readPayload(os.Stdin)
	if len(payload) == 0 {
		guardUsage("pr-body-check", "It denies a `gh pr create` whose body declares a closing "+
			"keyword on a base that is not the default branch, where GitHub resolves none.")
		return nil
	}
	reason := prbody.Deny(payload, func(cwd, repo string) string {
		return defaultBranch(ctx, cwd, repo)
	})
	if reason == "" {
		return nil
	}
	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = reason
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// readPayload reads the runtime's payload, bounded, and only where a hook says
// one is coming.
//
// The guards this serves are PreToolUse hooks, and a person, a script, cron or
// CI runs the same binary. `!isatty` is true both for a pipe the runtime wrote
// and for an inherited descriptor nobody will ever close, and on the second the
// read does not return: the command hangs, and a hang inside a process group
// the caller did not create outlives the caller's own timeout. That is not
// hypothetical — it took two of these commands out of use for a whole session.
//
// hooks/lib.sh has always documented the contract that fixes it ("MELLIONS_HOOK
// says the payload is there to be read"), and hookSession implements it for the
// delivery path. The guards did not, which is a comment asserting a property
// the code did not have. They do now: without the variable there is no payload,
// so a hand-run guard returns rather than waiting on a pipe.
func readPayload(r *os.File) []byte {
	if os.Getenv("MELLIONS_HOOK") == "" {
		return nil
	}
	info, err := r.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return nil
	}
	return raw
}

// guardUsage is what a guard says when a person runs it with no payload.
//
// Silence and exit 0 reads as "checked, nothing wrong", which is the one answer
// a guard that examined nothing must not give. It goes to stderr so a hook's
// stdout stays exactly the decision the runtime parses.
func guardUsage(name, what string) {
	fmt.Fprintf(os.Stderr,
		"mellions %s is a PreToolUse guard: it reads the runtime's tool payload and "+
			"decides whether to deny the call. It examined nothing here, because no payload "+
			"was handed to it.\n%s\n", name, what)
}

// defaultBranch is the branch GitHub resolves a closing keyword on: the
// checkout's own origin/HEAD where the command runs, else the tracker's answer.
// Empty means unknown, and unknown means the hook stays silent — a deny on a
// guess blocks a legitimate close in a repository it cannot read.
func defaultBranch(ctx context.Context, cwd, repo string) string {
	// One budget for the whole answer rather than one per command: the hook
	// that calls this has a few seconds before the runtime kills it, and a
	// decision that arrives after that is no decision.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	dir := cwd
	if dir == "" {
		dir = "."
	}
	if repo == "" && cwd != "" {
		if out, err := lookup(ctx, dir, "git", "-C", cwd, "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); err == nil {
			if b := strings.TrimPrefix(out, "origin/"); b != "" {
				return b
			}
		}
	}
	args := []string{"repo", "view"}
	if repo != "" {
		args = append(args, repo)
	}
	args = append(args, "--json", "defaultBranchRef", "-q", ".defaultBranchRef.name")
	out, err := lookup(ctx, dir, "gh", args...)
	if err != nil {
		return ""
	}
	return out
}

// lookup runs one read-only command under the caller's budget.
func lookup(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

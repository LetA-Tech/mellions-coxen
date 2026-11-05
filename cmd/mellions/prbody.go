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
// create` or `gh pr edit` whose body closes an issue by keyword on a base
// whose default branch it can compare with, and a title, `gh pr merge` message
// or `git commit`/`git merge` message that does on any base; prbody.Deny is the
// predicate.
func cmdPRBodyCheck(ctx context.Context, args []string) error {
	fs := newFlagSet("pr-body-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload, _, done, err := guardPayload(os.Stdin, "pr-body-check", "It denies a `gh pr create` whose body declares a closing "+
		"keyword: on a base that is not the default branch, where GitHub resolves none, and on the "+
		"default branch, where it closes before the proof, unless the body says the diff is the whole proof. "+
		"A closing keyword in a title is denied on every base, since the merge commit carries it to the "+
		"default branch, with the same whole-proof exception on the default branch. A closing keyword in a "+
		"`gh pr merge` --subject or --body is denied on every base with no exception, since it is the merge "+
		"commit's message, and so is one in a `git commit` or `git merge` message, which reaches the default "+
		"branch at a push or a promotion.")
	if done {
		return err
	}
	reason := prbody.Deny(payload, func(cwd, repo string) string {
		return defaultBranch(ctx, cwd, repo)
	})
	if reason == "" {
		return nil
	}
	var d decision
	d.Output.Event = preToolUseEvent
	d.Output.Decide = "deny"
	d.Output.Reason = reason
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// maxGuardPayload bounds a guard's read. It sits far above any tool call a model
// can emit; what matters is that a payload past it is refused, not cut.
const maxGuardPayload = 16 << 20

// readPayload reads the runtime's payload, bounded, and only where a hook says
// one is coming. A nil payload and a nil error mean none was to be expected; an
// error means a hook said one was coming and this is not it.
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
func readPayload(r *os.File) ([]byte, error) {
	if os.Getenv("MELLIONS_HOOK") == "" {
		return nil, nil
	}
	info, err := r.Stat()
	if err != nil {
		return nil, fmt.Errorf("its descriptor could not be read: %w", err)
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxGuardPayload+1))
	switch {
	case err != nil:
		return nil, fmt.Errorf("reading it failed: %w", err)
	case len(raw) == 0:
		return nil, errors.New("none arrived")
	case len(raw) > maxGuardPayload:
		return nil, fmt.Errorf("it is over the %d MiB a guard reads", maxGuardPayload>>20)
	}
	return raw, nil
}

// guardEvent is every field of a runtime payload any guard reads. Each guard's
// own decode reads a subset of it, so a payload that decodes here decodes there.
type guardEvent struct {
	ToolName string `json:"tool_name"`
	Cwd      string `json:"cwd"`
	Session  string `json:"session_id"`
	Input    struct {
		Command  string `json:"command"`
		File     string `json:"file_path"`
		Path     string `json:"path"`
		Notebook string `json:"notebook_path"`
	} `json:"tool_input"`
}

// guardPayload is what every guard reads its call through. It returns the raw
// payload and its decoded event, or done where the guard has nothing to decide:
// it has printed its usage to a person, or refused the call.
//
// The runtime reads a guard's silence as allow, so a payload the guard cannot
// read whole is refused here: truncated JSON looks like nothing to deny.
func guardPayload(r *os.File, name, what string) (raw []byte, ev guardEvent, done bool, err error) {
	raw, why := readPayload(r)
	if why == nil && raw == nil {
		guardUsage(name, what)
		return nil, ev, true, nil
	}
	if why == nil {
		why = json.Unmarshal(raw, &ev)
	}
	if why == nil && ev.ToolName == "" {
		why = errors.New("it names no tool")
	}
	if why == nil {
		return raw, ev, false, nil
	}
	var d decision
	d.Output.Event = preToolUseEvent
	d.Output.Decide = "deny"
	d.Output.Reason = "This tool call was not examined: mellions " + name + " could not read its hook " +
		"payload (" + why.Error() + "), so the guard refused it rather than let it through unread. " +
		"Nothing about the call itself was judged. Retry it."
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return nil, ev, true, enc.Encode(d)
}

// guardUsage is what a guard says when a person runs it with no payload.
//
// Silence and exit 0 reads as "checked, nothing wrong", which is the one answer
// a guard that examined nothing must not give. A payload piped without
// MELLIONS_HOOK is not read, so the message names the variable: otherwise it
// tells someone who did pipe one that none was handed. It goes to stderr so a
// hook's stdout stays exactly the decision the runtime parses.
func guardUsage(name, what string) {
	fmt.Fprintf(os.Stderr,
		"mellions %s is a PreToolUse guard: it reads the runtime's tool payload and "+
			"decides whether to deny the call. It examined nothing here: it reads a payload "+
			"only where MELLIONS_HOOK is set, as the runtime's hook sets it. To drive it by "+
			"hand, pipe the payload to `MELLIONS_HOOK=1 mellions %s`.\n%s\n", name, name, what)
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

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

// emitDeny is how every guard here answers "no": one decision object on
// stdout, carrying the guard's own reason and what the refusal discarded
// alongside the command the guard was aimed at.
//
// The second part is not the guard's business to know, which is why it lives
// once at the boundary they all pass through rather than five times inside
// them. Whether a call is refused rightly or wrongly does not change what the
// refusal costs, so no guard has to opt in.
func emitDeny(payload []byte, reason string) error {
	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = reason + discarded(payload)
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// discarded is what a refusal owes a session about the rest of the call.
//
// A PreToolUse denial rejects the whole tool call, so a command line that
// writes a file and then consumes it loses the write too. The failure then
// surfaces one call later as `no such file or directory`, in a command no
// guard touched, which reads as a mistake in the publish rather than as the
// refusal it is. Naming it here costs one paragraph and no change to how any
// guard decides.
//
// It says only what the command line itself shows. A write performed inside a
// program the line invokes is not visible to a lexer, so where a heredoc is
// handed to an interpreter the sentence names the heredoc and stops short of
// claiming which file it would have written.
//
// Empty for every call this cannot say something true about: a non-Bash tool,
// an unparseable payload, a command line that only reads.
func discarded(payload []byte) string {
	var ev struct {
		ToolName string `json:"tool_name"`
		Input    struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil || ev.ToolName != "Bash" || ev.Input.Command == "" {
		return ""
	}
	if files := shellsplit.Writes(ev.Input.Command); len(files) > 0 {
		return "\n\nThe refusal rejects the whole call, not the part that tripped it, so " +
			writesLost(files) + " Write the file in one call and run what consumes it in " +
			"the next, so a refusal of the second costs only the second."
	}
	for _, c := range shellsplit.Split(ev.Input.Command) {
		if len(c.Heredocs) > 0 {
			return "\n\nThe refusal rejects the whole call, not the part that tripped it, so " +
				"the heredoc on this command line was handed to nothing: anything the program " +
				"reading it would have written does not exist. Write the file in one call and " +
				"run what consumes it in the next, so a refusal of the second costs only the second."
		}
	}
	return ""
}

// writesLost names the redirect targets, bounded: a refusal is read, and a
// generated command line redirecting thirty times wants the first several and a
// count rather than a wall.
func writesLost(files []string) string {
	const limit = 6
	if len(files) == 1 {
		return "the write to `" + files[0] + "` did not happen."
	}
	more := ""
	if len(files) > limit {
		more = " and " + strconv.Itoa(len(files)-limit) + " more"
		files = files[:limit]
	}
	return "the writes to `" + strings.Join(files, "`, `") + "`" + more +
		" did not happen."
}

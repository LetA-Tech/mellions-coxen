// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/LetA-Tech/mellions-coxen/internal/secretread"
)

// cmdSecret checks a command for reads that would put a credential in the
// transcript and prints the ones it finds. Exit 1 where there are findings, so
// a script or a person can gate on it; the hook form is secret-check.
func cmdSecret(args []string) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New("secret: the subcommand is `check`")
	}
	fs := newFlagSet("secret check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	path := fs.String("path", "", "a path a tool would read directly, rather than a command")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	var findings []secretread.Finding
	if *path != "" {
		findings = secretread.ScanPath(*path)
	} else {
		findings = secretread.ScanBash(strings.Join(fs.Args(), " "))
	}
	if len(findings) == 0 {
		fmt.Println("secret: nothing here reads a credential into the transcript.")
		return nil
	}
	for _, f := range findings {
		fmt.Println("  " + reason(f))
	}
	return fmt.Errorf("%d credential read(s)", len(findings))
}

// cmdSecretCheck reads a PreToolUse payload on stdin and denies a tool call
// that would read a credential-bearing file's content into the transcript.
// Everything else is silence.
//
// It is a denial rather than a Skill sentence because the rule already existed
// as a Skill sentence and as a header comment in the file that leaked, and
// neither bound at the moment of action. A transcript is sent as it is
// written, so this is the last point at which the credential is still secret.
func cmdSecretCheck(args []string) error {
	fs := newFlagSet("secret-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload := readPayload(os.Stdin)
	if len(payload) == 0 {
		guardUsage("secret-check", "It denies a read that would put credential material "+
			"into the transcript.")
		return nil
	}
	var ev struct {
		ToolName string `json:"tool_name"`
		Input    struct {
			Command  string `json:"command"`
			File     string `json:"file_path"`
			Path     string `json:"path"`
			Notebook string `json:"notebook_path"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(payload, &ev) != nil {
		return nil
	}

	var findings []secretread.Finding
	switch ev.ToolName {
	case "Bash":
		findings = secretread.ScanBash(ev.Input.Command)
	case "Read", "Grep", "NotebookRead":
		for _, p := range []string{ev.Input.File, ev.Input.Path, ev.Input.Notebook} {
			findings = append(findings, secretread.ScanPath(p)...)
		}
	default:
		return nil
	}
	if len(findings) == 0 {
		return nil
	}

	var reasons []string
	for _, f := range findings {
		reasons = append(reasons, "  "+reason(f))
	}
	if n := len(reasons); n > 6 {
		reasons = append(reasons[:6], "  … and "+strconv.Itoa(n-6)+" more.")
	}

	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = "This would read a credential into the transcript:\n\n" +
		strings.Join(reasons, "\n") + "\n\n" +
		"A transcript is sent as it is written, so a credential printed here is " +
		"disclosed before it can be unprinted, and the fix afterwards is a rotation " +
		"rather than an edit. A redaction parse can disclose the value when its " +
		"assumption about the file format is wrong, so do not print credential files " +
		"in order to inspect or transform them.\n\n" +
		"Use the value without seeing it — `URL=\"$(tail -1 <file>)\"` then `\"$URL\"`, " +
		"or `source <file>`. Confirm a file's shape with `wc -c` / `stat`, never by " +
		"printing part of it. If you genuinely need a key's value, you need the program " +
		"that consumes it to read the file, not the transcript to carry it."
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

func reason(f secretread.Finding) string {
	if f.Reader == "" {
		return f.Path + " is a credential-bearing file; reading it prints its content."
	}
	if strings.HasPrefix(f.Path, "$") {
		return f.Path + " holds a credential read earlier on this command line, and `" +
			f.Reader + "` writes its argument out."
	}
	return "`" + f.Reader + " … " + f.Path + "` — " + f.Reader +
		" writes the file's content to stdout."
}

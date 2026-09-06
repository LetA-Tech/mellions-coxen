// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// The two shapes measured on this host, both of which cost a session a call
// spent debugging a file the refusal silently discarded.
func TestDiscardedNamesTheWriteTheRefusalThrewAway(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat > body.md <<'EOF'\n# title\nEOF\ngh issue create --body-file body.md"}}`
	got := discarded([]byte(payload))
	if !strings.Contains(got, "`body.md`") {
		t.Errorf("the refusal does not name body.md, the file the denial discarded:\n%s", got)
	}
	if !strings.Contains(got, "did not happen") {
		t.Errorf("the refusal does not say the write is absent:\n%s", got)
	}
}

func TestDiscardedNamesAHeredocNoInterpreterReceived(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"python3 <<'PY'\nopen('arm.sh','w').write('x')\nPY\nbash arm.sh"}}`
	got := discarded([]byte(payload))
	if !strings.Contains(got, "heredoc") {
		t.Errorf("a heredoc handed to an interpreter is not named:\n%s", got)
	}
	// The lexer cannot see inside python3, so the sentence must not claim to
	// know which file it would have written.
	if strings.Contains(got, "arm.sh") {
		t.Errorf("the refusal names a file it cannot have read off the command line:\n%s", got)
	}
}

func TestDiscardedNamesEveryRedirectTarget(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo a > one.md; echo b >> two.md; gh pr create --body-file one.md"}}`
	got := discarded([]byte(payload))
	for _, want := range []string{"`one.md`", "`two.md`"} {
		if !strings.Contains(got, want) {
			t.Errorf("%s is not named:\n%s", want, got)
		}
	}
}

// Silence is what a refusal owes a call that discards nothing. A sentence
// appended to every denial is noise, and noise is how a refusal stops being
// read at all.
func TestDiscardedSaysNothingWhereNothingWasDiscarded(t *testing.T) {
	for _, quiet := range []string{
		`{"tool_name":"Bash","tool_input":{"command":"git checkout dev"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"go test ./... > /dev/null"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"gh pr merge 12 --squash"}}`,
		// A `>` that is data, not a redirection. Naming a file the call was
		// never going to write is the one failure that costs more than the
		// silence this change exists to end: it sends a session looking for
		// something that was never coming.
		`{"tool_name":"Bash","tool_input":{"command":"echo \"a > b\"; gh pr create"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"grep -n '>' README.md"}}`,
		// A `>` the shell does not read as a redirection either. Inside `[[ ]]`
		// and `(( ))` it compares; `>(` opens a process substitution; a `>`
		// inside `${…}` is part of the expansion. The lexer sets Out for all
		// four, and a refusal naming `100`, `$best`, `(tee` or `fallback}`
		// states as fact that a file it invented was not written.
		`{"tool_name":"Bash","tool_input":{"command":"if (( $(git rev-list --count HEAD) > 100 )); then echo hi; fi"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"[[ \"$f\" > \"$best\" ]] && best=$f"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"make build > >(tee build.log)"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"echo x > ${OUT:->fallback}"}}`,
		`{"tool_name":"Read","tool_input":{"file_path":"/etc/payments/.env"}}`,
		`{"tool_name":"Bash","tool_input":{"command":""}}`,
		`not json`,
	} {
		if got := discarded([]byte(quiet)); got != "" {
			t.Errorf("said something about a call that discarded nothing:\n%s\n%s", quiet, got)
		}
	}
}

// A refusal is read. A generated command line redirecting thirty times gets
// the first several and a count, not a wall.
func TestDiscardedIsBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"tool_name":"Bash","tool_input":{"command":"`)
	for i := 0; i < 30; i++ {
		b.WriteString("echo x > f")
		b.WriteByte(byte('0' + i%10))
		b.WriteString(string(rune('a'+i/10)) + ".md; ")
	}
	b.WriteString(`gh issue create"}}`)
	got := discarded([]byte(b.String()))
	if !strings.Contains(got, "and 24 more") {
		t.Errorf("30 targets were not bounded to 6 and a count:\n%s", got)
	}
}

// The reason the guard wrote is what the session must answer; the discarded
// writes are added after it, never in place of it.
func TestEmitDenyKeepsTheGuardsOwnReasonFirst(t *testing.T) {
	payload := []byte(`{"tool_name":"Bash","tool_input":{"command":"cat > body.md <<'EOF'\nx\nEOF\ngh issue create --body-file body.md"}}`)
	out := "This would read a credential into the transcript:" + discarded(payload)
	if !strings.HasPrefix(out, "This would read a credential into the transcript:") {
		t.Fatalf("the guard's reason no longer leads the refusal:\n%s", out)
	}
	if !strings.Contains(out, "`body.md`") {
		t.Fatalf("the discarded write is not on the refusal:\n%s", out)
	}
}

// An append that was refused leaves the file exactly as it was, which is not
// the same as absent — a session told a file "was not created" acts on that.
func TestDiscardedDoesNotClaimAnExistingFileIsAbsent(t *testing.T) {
	got := discarded([]byte(`{"tool_name":"Bash","tool_input":{"command":"echo x >> already.md; gh pr create"}}`))
	if strings.Contains(got, "not created") {
		t.Errorf("an append is reported as a file that does not exist:\n%s", got)
	}
	if !strings.Contains(got, "`already.md`") {
		t.Errorf("the append target is not named:\n%s", got)
	}
}

// emitDeny is one line reached by five guards, and what it puts on stdout is
// the whole contract with the runtime. Nothing else here executes it.
func TestEmitDenyWritesTheDecisionTheRuntimeReads(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	emitErr := emitDeny(
		[]byte(`{"tool_name":"Bash","tool_input":{"command":"cat > body.md <<'EOF'\nx\nEOF\ngh issue create --body-file body.md"}}`),
		"Use the value without seeing it — `URL=\"$(tail -1 <file>)\"`.")
	os.Stdout = saved
	w.Close()
	out, _ := io.ReadAll(r)
	if emitErr != nil {
		t.Fatalf("emitDeny returned %v", emitErr)
	}
	var got struct {
		Output struct {
			Event  string `json:"hookEventName"`
			Decide string `json:"permissionDecision"`
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("stdout is not the decision the runtime parses: %v\n%s", err, out)
	}
	if got.Output.Event != "PreToolUse" || got.Output.Decide != "deny" {
		t.Errorf("wrong decision: %+v", got.Output)
	}
	// SetEscapeHTML(false). With it on, the `<file>` these reasons carry
	// ships as \u003cfile\u003e and the refusal reads as machine noise at
	// the point it has to persuade.
	if bytes.Contains(out, []byte(`\u003c`)) {
		t.Errorf("the refusal is HTML-escaped:\n%s", out)
	}
	if !bytes.Contains(out, []byte("<file>")) {
		t.Errorf("the reason lost its literal angle brackets:\n%s", out)
	}
	if !strings.HasPrefix(got.Output.Reason, "Use the value without seeing it") {
		t.Errorf("the guard's own reason no longer leads:\n%s", got.Output.Reason)
	}
	if !strings.Contains(got.Output.Reason, "`body.md`") {
		t.Errorf("the discarded write is not on the refusal:\n%s", got.Output.Reason)
	}
}

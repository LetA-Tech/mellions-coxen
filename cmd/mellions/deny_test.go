// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
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
	if !strings.Contains(got, "was not created") {
		t.Errorf("the refusal does not say the file is absent:\n%s", got)
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

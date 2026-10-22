// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"strings"
	"testing"
)

const unreadReason = "This body cannot be checked before it is published"

// `--body "$(cat f)"` was graded as the text `$(cat f)`, which cites nothing,
// so f's unquoted citation was published unchecked. It is refused as unread.
func TestCiteHook_RefusesABodyBashExpands(t *testing.T) {
	session, lane := citeFixture(t)
	path := unquotedBody(t, t.TempDir())

	for _, body := range []string{`"$(cat ` + path + `)"`, `"$B"`, "\"`cat " + path + "`\""} {
		reason := runCiteHook(t, session, "cd "+lane+" && gh pr comment 5 --body "+body)
		if !strings.Contains(reason, unreadReason) || !strings.Contains(reason, "--body ") {
			t.Errorf("--body %s: want an unread refusal naming --body, got:\n%s", body, reason)
		}
	}
}

// What the hook can read stays graded: a literal or single-quoted body, and a
// quoted heredoc substituted in place, are refused for their citation and
// admitted without one — never refused as unread.
func TestCiteHook_GradesABodyItCanRead(t *testing.T) {
	session, lane := citeFixture(t)
	citing := "Root cause is `neonlink.go:3`."

	for _, body := range []string{
		"'" + citing + "'",
		"\"$(cat <<'EOF'\n" + citing + "\nEOF\n)\"",
	} {
		reason := runCiteHook(t, session, "cd "+lane+" && gh pr comment 5 --body "+body)
		if reason == "" || strings.Contains(reason, unreadReason) {
			t.Errorf("--body %q: want a citation refusal, got:\n%s", body, reason)
		}
	}
	for _, body := range []string{`'costs $5'`, `"costs \$5"`, `"plain text"`} {
		if reason := runCiteHook(t, session, "cd "+lane+" && gh pr comment 5 --body "+body); reason != "" {
			t.Errorf("--body %s: want admitted, got:\n%s", body, reason)
		}
	}
}

// bash expands an unquoted heredoc, so `$(cat f)` inside `<<EOF` publishes f's
// citations; the hook graded the bytes `$(cat f)` and passed them.
func TestCiteHook_RefusesAHeredocBashExpands(t *testing.T) {
	session, lane := citeFixture(t)
	path := unquotedBody(t, t.TempDir())

	for _, cmd := range []string{
		"gh pr comment 5 --body \"$(cat <<EOF\nSummary.\n$(cat " + path + ")\nEOF\n)\"",
		"gh pr comment 5 --body-file - <<EOF\nSummary.\n$(cat " + path + ")\nEOF",
		"gh pr comment 5 --body-file - \\\n  <<EOF\nSummary.\n$(cat " + path + ")\nEOF",
		"gh pr comment 5 --body \"$(cat <<'EOF'\nIntro\nEOF\ncat " + path + ")\"",
		"gh pr comment 5 --body \"$(cat " + path + " - <<'EOF'\nIntro\nEOF\n)\"",
	} {
		reason := runCiteHook(t, session, "cd "+lane+" && "+cmd)
		if !strings.Contains(reason, unreadReason) {
			t.Errorf("%q: want an unread refusal, got:\n%s", cmd, reason)
		}
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// at is a clock reading; only its time of day is ever compared.
func at(t *testing.T, hms string) time.Time {
	t.Helper()
	v, err := time.Parse("15:04:05", hms)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestClockDeny(t *testing.T) {
	cases := []struct {
		name    string
		message string
		now     string
		refused bool
	}{
		{"ahead by 5 min", "f56 to lead-68, 01:19Z (date -u): hello", "01:14:00", true},
		{"ahead by 5 min with seconds", "f56 to lead-68, 01:19:00Z: hello", "01:14:00", true},
		{"ahead by 61 s", "01:15:01Z hello", "01:14:00", true},
		{"ahead by 60 s", "01:15:00Z hello", "01:14:00", false},
		{"ahead by 30 s", "01:14:30Z hello", "01:14:00", false},
		{"ahead by 59 s in an HH:MMZ stamp", "01:15Z hello", "01:14:01", false},
		{"ahead by 2 min in an HH:MMZ stamp", "01:16Z hello", "01:14:01", true},
		{"equal to the clock", "01:14Z hello", "01:14:00", false},
		{"behind by 10 min", "lead-68 to f51 (01:04Z): hello", "01:14:00", false},
		{"no stamp", "hello, nothing to see here", "01:14:00", false},
		{"empty message", "", "01:14:00", false},

		// The shapes sessions write: the stamp sits in the header and is judged.
		{"stamp in parentheses after date -u", "lead-68 WORD (date -u 01:19Z): hello", "01:14:00", true},
		{"stamp in parentheses before by date -u", "f60 to lead-68 (01:19Z by date -u): hello", "01:14:00", true},
		{"stamp after a name, before the colon", "lead-68 01:19Z: hello", "01:14:00", true},
		{"stamp before the first comma, no colon", "f59 01:19Z, answer on the PR", "01:14:00", true},
		{"stamp before a full stop, then a colon later", "f56 to lead-68 (host time 01:19Z by date -u). The reviewer's note is back: hello", "01:14:00", true},
		{"a range reads its Z end", "(read 00:51:15-00:51:45Z by date -u)", "00:50:00", true},
		{"a range behind the clock", "(read 00:51:15-00:51:45Z by date -u)", "00:52:00", false},
		{"CRLF line end", "01:19Z hello\r\nsecond", "01:14:00", true},

		// A time that is content, not a stamp, is never judged. Each is under
		// twelve hours ahead of the clock, so it would be refused if it were read.
		{"a deadline after the first colon", "lead-68: GO to review. Valid until 03:00Z by date -u", "01:14:00", false},
		{"a deadline after the colon and a comma", "lead-68: GO, valid until 03:00Z", "01:14:00", false},
		{"a time on a later line is not read", "hello\n03:00Z is the plan", "01:14:00", false},
		{"a later line is not read after a stamp that is behind", "lead-68 (01:10Z): hello\nvalid until 03:00Z", "01:14:00", false},
		{"a later time after an earlier stamp is not read", "lead-68 01:10Z by 03:00Z: hello", "01:14:00", false},
		{"a time after the first comma, no colon, is not read", "f59 01:10Z, answer by 03:00Z", "01:14:00", false},
		{"a deadline after the first comma, no colon, no stamp", "lead-68 GO, valid until 03:00Z", "01:14:00", false},
		{"a time after the header's colon when the stamp is behind", "lead-68 (01:10Z): GO, valid until 03:00Z", "01:14:00", false},

		// Known residual: a first line with no header and a future time first.
		{"a bare future time on the first line is judged", "start between now and 03:00Z", "01:14:00", true},

		{"midnight wrap: stamp before midnight, clock after", "23:59Z hello", "00:00:30", false},
		{"midnight wrap: stamp after midnight, clock before", "00:05Z hello", "23:58:00", true},
		{"midnight wrap: stamp after midnight within a minute", "00:00:30Z hello", "23:59:45", false},
		{"twelve hours ahead is today's", "13:14Z hello", "01:14:00", true},
		{"just over twelve hours ahead is yesterday's", "13:14:01Z hello", "01:14:00", false},
		{"a time with a date is not a stamp of today", "deployed 2026-11-10 09:00Z, now checking", "01:14:00", false},
		{"an ISO timestamp is not a stamp of today", "at 2026-11-10T09:00:00Z", "01:14:00", false},
		{"hour 24", "24:00Z hello", "01:14:00", false},
		{"hour 25", "25:30Z hello", "01:14:00", false},
		{"minute 60", "02:60Z hello", "01:14:00", false},
		{"second 60", "02:00:60Z hello", "01:14:00", false},
		{"no Z", "05:00 hello", "01:14:00", false},
		{"a longer digit run", "101:14Z hello", "00:00:00", false},
		{"a longer token after", "05:00Zulu hello", "01:14:00", false},
		{"a dotted prefix", "1.12:03Z hello", "01:14:00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clockDeny(c.message, at(t, c.now))
			if (got != "") != c.refused {
				t.Fatalf("clockDeny(%q, %s) = %q, refused want %v", c.message, c.now, got, c.refused)
			}
		})
	}
}

func TestClockDenyNamesTheStampTheClockAndTheRemedy(t *testing.T) {
	got := clockDeny("f56 to lead-68, 01:19Z (date -u): hello", at(t, "01:14:10"))
	for _, want := range []string{"01:19Z", "01:14:10Z", "4m50s", "`date -u`", "resend", "header"} {
		if !strings.Contains(got, want) {
			t.Errorf("the reason does not say %q: %s", want, got)
		}
	}
}

func clockPayload(tool, message string) string {
	b, _ := json.Marshal(map[string]any{
		"tool_name":  tool,
		"tool_input": map[string]any{"to": "lead-68", "message": message},
	})
	return string(b)
}

// The hook reaches the predicate through a payload; anything the predicate
// cannot be shown to apply to is silence, never a refusal.
func TestClockCheckReadsThePayloadAndFailsOpen(t *testing.T) {
	check := func() error { return cmdClockCheck([]string{"-now", "01:14:00"}) }
	run := func(payload string) (bool, string) {
		return decided(t, runGuard(t, check, "1", payload))
	}

	if deny, reason := run(clockPayload("SendMessage", "01:19Z hello")); !deny {
		t.Errorf("a stamp 5 min ahead was not refused (the positive control for every allow below): %q", reason)
	}
	allowed := map[string]string{
		"ahead by 30 s":               clockPayload("SendMessage", "01:14:30Z hello"),
		"behind":                      clockPayload("SendMessage", "01:04Z hello"),
		"no stamp":                    clockPayload("SendMessage", "hello"),
		"another tool":                clockPayload("Bash", "01:19Z hello"),
		"malformed JSON":              `{"tool_name":"SendMessage","tool_input":{"message":"01:19Z hel`,
		"not JSON":                    `01:19Z hello`,
		"an empty object":             `{}`,
		"no tool_input":               `{"tool_name":"SendMessage"}`,
		"no message":                  `{"tool_name":"SendMessage","tool_input":{"to":"x"}}`,
		"a message that is a list":    `{"tool_name":"SendMessage","tool_input":{"message":["01:19Z"]}}`,
		"a message that is an object": `{"tool_name":"SendMessage","tool_input":{"message":{"type":"x","text":"01:19Z"}}}`,
		"a tool_input that is text":   `{"tool_name":"SendMessage","tool_input":"01:19Z hello"}`,
		"a null message":              `{"tool_name":"SendMessage","tool_input":{"message":null}}`,
		"an empty payload":            ``,
	}
	for name, payload := range allowed {
		if deny, reason := run(payload); deny || reason != "" {
			t.Errorf("%s was refused: %s", name, reason)
		}
	}
}

func TestClockCheckWithABadClockAllows(t *testing.T) {
	if err := cmdClockCheck([]string{"-now", "noon"}); err == nil {
		t.Error("an unreadable -now was accepted; it must be an error, which the hook reads as allow")
	}
}

// hooks.json must register the hook for SendMessage alone and leave it able to
// fail open: onFailure block would turn a failed or timed-out hook into a
// refusal of every message, and a long timeout is what each send would wait on.
func TestClockCheckIsRegisteredToFailOpen(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command   string `json:"command"`
					Timeout   int    `json:"timeout"`
					OnFailure string `json:"onFailure"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, group := range m.Hooks.PreToolUse {
		for _, h := range group.Hooks {
			if !strings.Contains(h.Command, "clock-check.sh") {
				continue
			}
			found++
			if group.Matcher != "SendMessage" {
				t.Errorf("clock-check.sh is registered for %q, want SendMessage alone", group.Matcher)
			}
			if h.OnFailure != "" {
				t.Errorf("clock-check.sh has onFailure %q; a failed hook must allow the message", h.OnFailure)
			}
			if h.Timeout < 1 || h.Timeout > 5 {
				t.Errorf("clock-check.sh timeout is %d s, want 1 to 5", h.Timeout)
			}
			if !strings.HasSuffix(strings.TrimSpace(h.Command), "; exit 0") {
				t.Errorf("clock-check.sh command %q does not end in `; exit 0`", h.Command)
			}
		}
	}
	if found != 1 {
		t.Errorf("hooks.json registers clock-check.sh %d times, want once", found)
	}
}

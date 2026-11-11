// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// clockSlack is how far ahead of the host clock a stamp may read before it is
// refused: a minute written rounded up (01:15Z at 01:14:30) is not a slip.
const clockSlack = 60 * time.Second

// clockTime finds a UTC time written HH:MMZ or HH:MM:SSZ. The optional first
// group is a date and separator in front of it: a time that belongs to a date
// is not a stamp of today, so it is not compared.
var clockTime = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T ]+)?(\d{2}):(\d{2})(?::(\d{2}))?Z`)

// cmdClockCheck reads a PreToolUse payload on stdin and denies a SendMessage
// whose stamp is more than a minute ahead of the host clock. The stamp is the
// first time in the message's header, which clockHeader defines. Everything
// else is silence.
//
// Unlike the other guards it fails open. A refusal here stops one session
// reaching another, so only a stamp the clock confirms is ahead may cause one;
// a payload that is absent, unreadable, another tool's or of an unexpected
// shape is allowed, and so is a clock that cannot be read. -now (HH:MM:SS UTC)
// replaces the host clock so a test needs neither a sleep nor the wall time.
func cmdClockCheck(args []string) error {
	fs := newFlagSet("clock-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	at := fs.String("now", "", "the time to compare with, HH:MM:SS UTC; the host clock when empty")
	if err := fs.Parse(args); err != nil {
		return err
	}
	now := time.Now().UTC()
	if *at != "" {
		t, err := time.Parse("15:04:05", *at)
		if err != nil {
			return fmt.Errorf("clock-check: -now %q is not HH:MM:SS", *at)
		}
		now = t
	}
	raw, err := readPayload(os.Stdin)
	if raw == nil {
		if err == nil {
			guardUsage("clock-check", "It denies a SendMessage whose first line opens with a time more than "+
				"a minute ahead of the host clock, and allows everything else.")
		}
		return nil
	}

	var ev struct {
		ToolName string `json:"tool_name"`
		Input    struct {
			Message any `json:"message"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(raw, &ev) != nil || ev.ToolName != "SendMessage" {
		return nil
	}
	msg, _ := ev.Input.Message.(string)
	reason := clockDeny(msg, now)
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

// clockHeader is the part of a message that can carry its stamp: the first line
// up to its first ": ", or, when it has none, up to its first ", ". A time
// after that, or on a later line, is content (a deadline, a schedule) and is
// never judged.
func clockHeader(message string) string {
	line, _, _ := strings.Cut(message, "\n")
	if head, _, ok := strings.Cut(line, ": "); ok {
		return head
	}
	head, _, _ := strings.Cut(line, ", ")
	return head
}

// clockDeny returns the reason to refuse message, or "" to allow it. Only the
// time-of-day of now is read: the stamp is taken as today in UTC, and one more
// than twelve hours from the clock is the neighbouring day's, so a stamp just
// either side of midnight is compared across it.
func clockDeny(message string, now time.Time) string {
	line := clockHeader(message)
	loc := clockTime.FindStringSubmatchIndex(line)
	if loc == nil || loc[2] >= 0 {
		return ""
	}
	// A time inside a longer token (v1.2:03Z, 101:14Z, 01:14Zulu) is not a stamp.
	if loc[0] > 0 && (isASCIIAlnum(line[loc[0]-1]) || line[loc[0]-1] == ':' || line[loc[0]-1] == '.') {
		return ""
	}
	if loc[1] < len(line) && isASCIIAlnum(line[loc[1]]) {
		return ""
	}
	h, _ := strconv.Atoi(line[loc[4]:loc[5]])
	m, _ := strconv.Atoi(line[loc[6]:loc[7]])
	s := 0
	if loc[8] >= 0 {
		s, _ = strconv.Atoi(line[loc[8]:loc[9]])
	}
	if h > 23 || m > 59 || s > 59 {
		return ""
	}

	now = now.UTC()
	nowOfDay := time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute +
		time.Duration(now.Second())*time.Second + time.Duration(now.Nanosecond())
	stamp := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
	ahead := stamp - nowOfDay
	switch {
	case ahead > 12*time.Hour:
		ahead -= 24 * time.Hour
	case ahead < -12*time.Hour:
		ahead += 24 * time.Hour
	}
	if ahead <= clockSlack {
		return ""
	}
	text := line[loc[0]:loc[1]]
	return fmt.Sprintf("The header of this message (its first line, up to the first \": \") carries the "+
		"time %s, which is %s ahead of the host clock: `date -u` reads %s. A stamp is the clock at "+
		"the moment of sending, so write it from `date -u`, not from memory. Re-read `date -u` and "+
		"resend with that time. If %s is a time you mean to schedule rather than a stamp, put it "+
		"after the first \": \" or on a later line; only the header is judged.",
		text, ahead.Round(time.Second), now.Format("15:04:05")+"Z", text)
}

func isASCIIAlnum(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

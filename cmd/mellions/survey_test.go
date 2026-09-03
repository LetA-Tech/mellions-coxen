// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sig "github.com/LetA-Tech/mellions-coxen/internal/signal"
	"github.com/LetA-Tech/mellions-coxen/internal/survey"
)

// TestSurveyBriefReadsTheHeadingCount.
//
// The one-line brief a session is handed at the start of a turn used to be
// produced by counting bullets under each heading. A rendered section now holds
// lines that are not signals — what a render cap held back, and the command
// that prints it — so counting bullets would report more work than exists. The
// heading states the collected count; that is what the brief reads.
func TestSurveyBriefReadsTheHeadingCount(t *testing.T) {
	res := survey.Result{
		At:  time.Now(),
		Ran: []string{"github"},
		Signals: []sig.Signal{
			{Kind: sig.KindWorkItem, Source: "github", Repo: "r", ID: "#1", Title: "one"},
			{Kind: sig.KindWorkItem, Source: "github", Repo: "r", ID: "#2", Title: "two"},
			{Kind: sig.KindWorkItem, Source: "github", Repo: "r", ID: "#3", Title: "three"},
			{Kind: sig.KindBuild, Source: "github", Repo: "r", ID: "CI", Title: "red"},
		},
	}
	path := filepath.Join(t.TempDir(), "survey-latest.md")
	// Capped hard, so the rendered list is shorter than what was collected.
	if err := os.WriteFile(path, []byte(res.Render(survey.Options{PerRepo: 1})), 0o644); err != nil {
		t.Fatal(err)
	}

	brief, age, ok := surveyBrief(path)
	if !ok {
		t.Fatal("surveyBrief could not read a survey it was just handed")
	}
	if age > time.Minute {
		t.Errorf("age of a file written now is %v", age)
	}
	if !strings.Contains(brief, "3 tracked work items") {
		t.Errorf("brief reports the printed count rather than the collected one: %q", brief)
	}
	if !strings.Contains(brief, "1 failing checks") {
		t.Errorf("brief lost a section: %q", brief)
	}
}

func TestSurveyBriefReportsIncomplete(t *testing.T) {
	res := survey.Result{
		At: time.Now(), Ran: []string{"github"},
		Failures: []survey.Failure{{Source: "github", Err: os.ErrPermission}},
	}
	path := filepath.Join(t.TempDir(), "survey-latest.md")
	if err := os.WriteFile(path, []byte(res.Text()), 0o644); err != nil {
		t.Fatal(err)
	}
	if brief, _, _ := surveyBrief(path); !strings.Contains(brief, "INCOMPLETE") {
		t.Errorf("a survey that could not reach a source reads as complete: %q", brief)
	}
}

// TestUnknownKindIsRefused: a mistyped filter that renders an empty survey
// reads exactly like an estate with nothing in it.
func TestUnknownKindIsRefused(t *testing.T) {
	if _, err := parseKinds("work_item,workitem"); err == nil {
		t.Fatal("parseKinds accepted a kind that does not exist")
	} else if !strings.Contains(err.Error(), "workitem") || !strings.Contains(err.Error(), "work_item") {
		t.Errorf("the error names neither the mistake nor the alternatives: %v", err)
	}
	got, err := parseKinds("build, stale_premise")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != sig.KindBuild || got[1] != sig.KindStalePremise {
		t.Errorf("parseKinds = %v", got)
	}
}

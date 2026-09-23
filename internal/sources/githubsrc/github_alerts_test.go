// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package githubsrc

import (
	"context"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

const oneHighAlert = `[{"number":30,"html_url":"https://github.com/o/r/security/dependabot/30",
"dependency":{"package":{"ecosystem":"go","name":"google.golang.org/grpc"},"manifest_path":"sdk/go/go.mod"},
"security_advisory":{"ghsa_id":"GHSA-aaaa-bbbb-cccc","severity":"high","summary":"DoS via missing authority"},
"security_vulnerability":{"vulnerable_version_range":"< 1.82.2","first_patched_version":{"identifier":"1.82.2"}}}]`

func alertSignals(sigs []signal.Signal) []signal.Signal {
	var out []signal.Signal
	for _, s := range sigs {
		if s.Kind == signal.KindAlert && s.Attrs["type"] == "security_alert" {
			out = append(out, s)
		}
	}
	return out
}

func TestAnOpenSecurityAlertReachesTheSurvey(t *testing.T) {
	run, _ := fakeGH(t, map[string]string{"alerts": oneHighAlert})
	got := alertSignals(collect(t, New(Options{Owner: "o", Repos: []string{"r"}, Run: run}), signal.Scope{}))
	if len(got) != 1 {
		t.Fatalf("want 1 security alert signal, got %d", len(got))
	}
	a := got[0]
	for k, want := range map[string]string{
		"severity": "high", "package": "google.golang.org/grpc", "manifest": "sdk/go/go.mod",
		"vulnerable": "< 1.82.2", "first_patched": "1.82.2", "advisory": "GHSA-aaaa-bbbb-cccc",
	} {
		if a.Attrs[k] != want {
			t.Errorf("attr %s = %q, want %q", k, a.Attrs[k], want)
		}
	}
	if a.Repo != "r" || a.ID != "dependabot-30" || !strings.Contains(a.Title, "high") {
		t.Errorf("signal = %+v", a)
	}
}

func TestEveryPaginatedPageOfAlertsIsRead(t *testing.T) {
	two := oneHighAlert + "\n" + strings.Replace(oneHighAlert, `"number":30`, `"number":31`, 1)
	run, _ := fakeGH(t, map[string]string{"alerts": two})
	got := alertSignals(collect(t, New(Options{Owner: "o", Repos: []string{"r"}, Run: run}), signal.Scope{}))
	if len(got) != 2 {
		t.Fatalf("want 2 alerts across two pages, got %d", len(got))
	}
}

func TestNoOpenAlertsYieldsNoAlertSignal(t *testing.T) {
	run, _ := fakeGH(t, map[string]string{"alerts": "[]"})
	if got := alertSignals(collect(t, New(Options{Owner: "o", Repos: []string{"r"}, Run: run}), signal.Scope{})); len(got) != 0 {
		t.Fatalf("want no alert signals, got %d", len(got))
	}
}

// An alert endpoint the token cannot read must say so, and must not cost the
// repository the issues already collected.
func TestUnreadableAlertsAreReportedWithoutDroppingIssues(t *testing.T) {
	issue := `[{"number":7,"title":"t","url":"u","labels":[],"assignees":[],"comments":[]}]`
	run, _ := fakeGH(t, map[string]string{"alerts": "403", "issue": issue})
	got, err := New(Options{Owner: "o", Repos: []string{"r"}, Run: run}).Collect(context.Background(), signal.Scope{})
	if err == nil || !strings.Contains(err.Error(), "r: security alerts") {
		t.Fatalf("want an error naming r's security alerts, got %v", err)
	}
	var issues int
	for _, s := range got {
		if s.Kind == signal.KindWorkItem && s.ID == "#7" {
			issues++
		}
	}
	if issues != 1 {
		t.Fatalf("the issue collected before the alert read was dropped: %+v", got)
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package claim

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// exclusive is the sentence a holding lane's claim tells every other session.
// An alongside lane holds nothing, so its claim must never carry it.
const exclusive = "merging this change set — is not yours while the claim stands"

func postedBody(t *testing.T, f *fake) string {
	t.Helper()
	for _, c := range f.calls {
		if len(c) < 2 || c[1] != "comment" {
			continue
		}
		for i, a := range c {
			if a == "--body" && i+1 < len(c) {
				return c[i+1]
			}
		}
	}
	t.Fatal("no claim comment was posted")
	return ""
}

func payloadOf(t *testing.T, body string) Claim {
	t.Helper()
	m := markerPattern.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("posted claim carries no payload:\n%s", body)
	}
	var c Claim
	if err := json.Unmarshal([]byte(m[1]), &c); err != nil {
		t.Fatalf("payload: %v", err)
	}
	return c
}

func alongsideBody(id, host string, at time.Time) string {
	raw, _ := json.Marshal(Claim{ID: id, Host: host, State: "active", Alongside: true, At: at})
	return "Alongside another lane's hold\n\n" + marker + string(raw) + " -->\n"
}

func TestABesideClaimDefersToALaneStillWorkingTheItem(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view(ghComment{ID: "holder", Body: body("owner-lane", "build-host", "active", at)})}
	c, err := tracker(f, at).PublishBeside(context.Background(), "mellions-coxen", "#1494", "read-lane", "active")
	if err != nil {
		t.Fatalf("PublishBeside: %v", err)
	}
	if !c.Alongside {
		t.Error("the returned claim does not say it defers")
	}
	posted := postedBody(t, f)
	if strings.Contains(posted, exclusive) {
		t.Errorf("a lane beside a live holder tells that holder its own merge is not its own:\n%s", posted)
	}
	if !strings.Contains(posted, "defers to it") || !strings.Contains(posted, "taking the work and merging it stay with that lane") {
		t.Errorf("a deferring claim does not say who keeps the work and the merge:\n%s", posted)
	}
	if !strings.Contains(posted, `"alongside":true`) {
		t.Errorf("the payload does not carry the alongside field another machine reads:\n%s", posted)
	}
}

// -alongside is also how a session takes over work whose lane is finished or
// gone. Beside nobody still working the item, the claim is an ordinary hold, or
// a third lane would open on work this one is doing.
func TestABesideClaimHoldsWhenNobodyElseIsWorkingTheItem(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	for name, comments := range map[string][]ghComment{
		"no other claim":       nil,
		"holder handed off":    {{ID: "h", Body: body("owner-lane", "build-host", "handed_off", at)}},
		"holder stale":         {{ID: "h", Body: body("owner-lane", "build-host", "active", at.Add(-2*StaleAfter))}},
		"only a deferring one": {{ID: "r", Body: alongsideBody("other-reader", "build-host", at)}},
		"only this lane's own": {{ID: "me", Body: body("read-lane", "here", "active", at)}},
	} {
		f := &fake{view: view(comments...)}
		c, err := tracker(f, at).PublishBeside(context.Background(), "mellions-coxen", "#1494", "read-lane", "active")
		if err != nil {
			t.Fatalf("%s: PublishBeside: %v", name, err)
		}
		posted := postedBody(t, f)
		if c.Alongside || !strings.Contains(posted, exclusive) || payloadOf(t, posted).Alongside {
			t.Errorf("%s: a lane taking over work nobody is still doing published a claim that holds nothing:\n%s", name, posted)
		}
	}
}

func TestAHoldingClaimStillReservesTheWorkAndTheMerge(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view(ghComment{ID: "holder", Body: body("owner-lane", "build-host", "active", at)})}
	if _, err := tracker(f, at).Publish(context.Background(), "mellions-coxen", "#1494", "next-lane", "active"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	posted := postedBody(t, f)
	if !strings.Contains(posted, exclusive) {
		t.Errorf("a live lane's claim no longer reserves the work:\n%s", posted)
	}
	if payloadOf(t, posted).Alongside || strings.Contains(posted, `"alongside"`) {
		t.Error("a holding claim's payload changed")
	}
	if !f.sawArg("--add-label") {
		t.Error("a holding claim did not label the item")
	}
}

// A deferring claim is still a lane working the item, so the label stays on
// while it lives: the survey and every peer still see the item as worked.
func TestADeferringClaimKeepsTheLabelAfterTheHolderReleases(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view(
		ghComment{ID: "holder", Body: body("owner-lane", "here", "active", at)},
		ghComment{ID: "reader", Body: alongsideBody("read-lane", "build-host", at)},
	)}
	if err := tracker(f, at).Release(context.Background(), "mellions-coxen", "#1494", "owner-lane"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if f.sawArg("--remove-label") {
		t.Error("the label came off an item a live lane is still on")
	}
}

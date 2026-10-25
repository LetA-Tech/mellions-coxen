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

func TestAnAlongsideClaimReservesNeitherTheWorkNorTheMerge(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view(ghComment{ID: "holder", Body: body("owner-lane", "build-host", "active", at)})}
	c, err := tracker(f, at).PublishAlongside(context.Background(), "mellions-coxen", "#1494", "read-lane", "active")
	if err != nil {
		t.Fatalf("PublishAlongside: %v", err)
	}
	if !c.Alongside {
		t.Error("the returned claim does not say it is alongside")
	}
	posted := postedBody(t, f)
	if strings.Contains(posted, exclusive) {
		t.Errorf("an alongside lane's claim tells the holding lane its own merge is not its own:\n%s", posted)
	}
	if !strings.Contains(posted, "reserves nothing") || !strings.Contains(posted, "neither the work nor the merge") {
		t.Errorf("an alongside claim does not say what it reserves:\n%s", posted)
	}
	if got := payloadOf(t, posted); !got.Alongside || got.ID != "read-lane" {
		t.Errorf("payload %+v: another machine reading it cannot tell it reserves nothing", got)
	}
	if f.sawArg("--add-label") || f.saw("label", "create") {
		t.Error("an alongside claim put the claimed label on an item it does not hold")
	}
}

func TestAHoldingClaimStillReservesTheWorkAndTheMerge(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view()}
	if _, err := tracker(f, at).Publish(context.Background(), "mellions-coxen", "#1494", "owner-lane", "active"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	posted := postedBody(t, f)
	if !strings.Contains(posted, exclusive) {
		t.Errorf("a live lane's claim no longer reserves the work:\n%s", posted)
	}
	if payloadOf(t, posted).Alongside {
		t.Error("a holding claim's payload says alongside")
	}
	if !f.sawArg("--add-label") {
		t.Error("a holding claim did not label the item")
	}
}

func TestAnAlongsideClaimDoesNotKeepTheLabelOnAfterTheHolderReleases(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view(
		ghComment{ID: "holder", Body: body("owner-lane", "here", "active", at)},
		ghComment{ID: "reader", Body: alongsideBody("read-lane", "build-host", at)},
	)}
	if err := tracker(f, at).Release(context.Background(), "mellions-coxen", "#1494", "owner-lane"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if !f.sawArg("--remove-label") {
		t.Error("the label stayed on an item only an alongside lane is left on, so a survey prints it as held")
	}
}

func TestASweepLeavingOnlyAnAlongsideClaimTakesTheLabelOff(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 45, 0, 0, time.UTC)
	f := &fake{view: view(
		ghComment{ID: "gone", Body: body("owner-lane", "build-host", "active", now.Add(-2*StaleAfter))},
		ghComment{ID: "reader", Body: alongsideBody("read-lane", "here", now)},
	)}
	swept, err := tracker(f, now).Sweep(context.Background(), "mellions-coxen", "#1494")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(swept) != 1 || swept[0].ID != "owner-lane" {
		t.Fatalf("swept %+v, want the stale holder only", swept)
	}
	if !f.sawArg("--remove-label") {
		t.Error("the label stayed on after its last holder was swept, held up by an alongside claim")
	}
}

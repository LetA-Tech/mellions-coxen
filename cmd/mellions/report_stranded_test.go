// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claimReportPath creates a report's name as an empty file before durable.Write
// replaces it with the document, so a kill or a power loss between the two
// leaves a zero-byte .md behind. latest prints a report verbatim and orders by
// name, so a reservation claimed for a later second stands in front of every
// report before it: `report latest -n 1` answered with a blank page while the
// report it stood in front of — one carrying "## Needs you" — was still on disk
// and went unread. The digest already refused it for carrying no section;
// latest had no such filter.
func TestLatestSkipsAStrandedReservation(t *testing.T) {
	cfgPath, dir := collisionConfig(t)
	const asking = "the RLS decision on finrate.schema_migrations is yours"

	real := writeReport(t, cfgPath, "-needs-owner", asking)

	// The residue exactly as claimReportPath leaves it — the name claimed, the
	// content write never arrived — under a name that sorts after every real
	// report, so it is the one latest would lead with.
	stranded, err := claimReportPath(dir, "20991231-235959")
	if err != nil {
		t.Fatal(err)
	}
	strandedInfo, err := os.Stat(stranded)
	if err != nil {
		t.Fatal(err)
	}
	if strandedInfo.Size() != 0 {
		t.Fatalf("a reservation is meant to be empty; %s holds %d bytes", stranded, strandedInfo.Size())
	}
	// Without this the test can pass having proved nothing: a reservation
	// that sorts below the report it is supposed to hide never led the
	// ordering. The order is the name's, so the check is on the name.
	strandedBase, _ := reportOrder(filepath.Base(stranded))
	realBase, _ := reportOrder(filepath.Base(real))
	if strandedBase <= realBase {
		t.Fatalf("the reservation does not sort ahead of the report, so it was never in front of it: %s, %s",
			stranded, real)
	}

	out := captureStdout(t, func() {
		if err := cmdReport([]string{"latest", "-config", cfgPath, "-n", "1"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, asking) {
		t.Errorf("report latest -n 1 did not reach the report asking for the owner\n"+
			"a stranded reservation at %s stood in front of %s\nasked: %q\nlatest printed:\n%s",
			stranded, real, asking, out)
	}
}

// A zero-byte file is the reservation's shape, so latest must refuse it by size
// rather than by name: nothing about the name says the write never landed, and
// the next release's residue may carry a different one.
func TestLatestStillAnswersWhenEveryReportIsReal(t *testing.T) {
	cfgPath, _ := collisionConfig(t)

	first := writeReport(t, cfgPath, "-did", "written first")
	second := writeReport(t, cfgPath, "-did", "written second")
	if first == second {
		t.Fatalf("two reports were given one path: %s", first)
	}

	out := captureStdout(t, func() {
		if err := cmdReport([]string{"latest", "-config", cfgPath, "-n", "1"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "written second") {
		t.Errorf("the size filter dropped a real report; latest -n 1 printed:\n%s", out)
	}
}

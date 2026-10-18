package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// laneRecord lays out an assignment record the way `mellions assign open` does:
// assignment.json at the root, the lane's checkout at tree/, scratch beside it.
// The checkout is citeFixture's lane repository, whose line 3 is known.
func laneRecord(t *testing.T) (record string) {
	t.Helper()
	_, lane := citeFixture(t)
	record = filepath.Join(t.TempDir(), "assignments", "x-lane-1016")
	if err := os.MkdirAll(filepath.Join(record, "scratch", "arms"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(record, "assignment.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(lane, filepath.Join(record, "tree")); err != nil {
		t.Fatal(err)
	}
	return record
}

const laneLine3 = "\tproducer, err := neonlink.NewTransactionalProducer(sdkCfg, logger)"

// A review lane posts from its record root or its scratch. Its body cites the
// lane's tree, so a quote wrong for that tree must be denied from there too,
// rather than passing as a path the directory "cannot open".
func TestCiteHook_FromALaneRecordResolvesAgainstItsTree(t *testing.T) {
	record := laneRecord(t)
	wrong := citeBody(t, t.TempDir(), "\thealth, err := neonlink.NewHealthChecker(sdkCfg)")
	right := citeBody(t, t.TempDir(), laneLine3)

	for _, cwd := range []string{record, filepath.Join(record, "scratch"), filepath.Join(record, "scratch", "arms")} {
		reason := runCiteHook(t, cwd, "gh pr review 1 --comment --body-file "+wrong)
		if reason == "" || !strings.Contains(reason, "neonlink.go:3") {
			t.Errorf("from %s a quote wrong for the lane's tree was not denied: %q", cwd, reason)
		}
		if tree := filepath.Join(record, "tree"); !strings.Contains(reason, "resolved against "+tree) {
			t.Errorf("from %s the deny does not name the tree it read, %s: %q", cwd, tree, reason)
		}

		var raw string
		if reason := runCiteHookRaw(t, cwd, "gh pr review 1 --comment --body-file "+right, &raw); reason != "" {
			t.Errorf("from %s a quote right for the lane's tree was denied: %s", cwd, reason)
		}
		if strings.Contains(raw, "cannot open") {
			t.Errorf("from %s a citation the lane's tree holds was reported unopened: %s", cwd, raw)
		}
	}

	// The same holds when the command moves into the record before publishing.
	if reason := runCiteHook(t, t.TempDir(), "cd "+record+" && gh pr review 1 --comment --body-file "+wrong); reason == "" {
		t.Error("a cd into a lane record left a quote wrong for its tree undenied")
	}
}

// A record with no work tree beside it stands for nothing, and a directory
// that is already in a work tree is its own subject: neither is redirected.
func TestLaneCheckout_KeepsWhatItCannotMap(t *testing.T) {
	ctx := t.Context()

	bare := filepath.Join(t.TempDir(), "assignments", "no-tree")
	if err := os.MkdirAll(filepath.Join(bare, "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bare, "assignment.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := laneCheckout(ctx, filepath.Join(bare, "scratch")), filepath.Join(bare, "scratch"); got != want {
		t.Errorf("a record without a tree mapped %s to %s", want, got)
	}

	record := laneRecord(t)
	nested, _ := citeFixture(t)
	inScratch := filepath.Join(record, "scratch", "sibling")
	if err := os.Rename(nested, inScratch); err != nil {
		t.Fatal(err)
	}
	if got := laneCheckout(ctx, inScratch); got != inScratch {
		t.Errorf("a checkout inside scratch was redirected to %s", got)
	}

	// t.TempDir() is under GOTMPDIR when that is set, else TMPDIR; a session
	// may point either into its own lane.
	outside := t.TempDir()
	for d := outside; ; d = filepath.Dir(d) {
		if info, err := os.Stat(filepath.Join(d, "assignment.json")); err == nil && info.Mode().IsRegular() {
			t.Skipf("no directory outside every lane record: the temporary root %s is inside the record at %s; set GOTMPDIR and TMPDIR outside it", outside, d)
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	if got := laneCheckout(ctx, outside); got != outside {
		t.Errorf("a directory outside any record was redirected to %s", got)
	}
}

// A body wrong in more places than the deny shows still names the tree it was
// graded against: the deny shows eight findings, and the name is what says which tree to
// open before correcting anything.
func TestCiteHook_ARedirectedDenyNamesTheTreePastTheCap(t *testing.T) {
	record := laneRecord(t)
	var src, doc strings.Builder
	doc.WriteString("Root cause.\n")
	for i := 1; i <= 12; i++ {
		src.WriteString("var line" + strconv.Itoa(i) + " = " + strconv.Itoa(i) + "\n")
		doc.WriteString("\n`long.go:" + strconv.Itoa(i) + "`\n\n```go\nvar line" + strconv.Itoa(i) + " = 0\n```\n")
	}
	if err := os.WriteFile(filepath.Join(record, "tree", "long.go"), []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	body := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(body, []byte(doc.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	reason := runCiteHook(t, record, "gh pr review 1 --comment --body-file "+body)
	if reason == "" {
		t.Fatal("a body wrong twelve times for the lane's tree was not denied")
	}
	if tree := filepath.Join(record, "tree"); !strings.Contains(reason, "resolved against "+tree) {
		t.Errorf("past the deny's cap the tree it read is not named, %s: %q", tree, reason)
	}
}

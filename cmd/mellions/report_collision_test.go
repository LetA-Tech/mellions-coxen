// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A report file was named from a UTC second, and durable.Write replaces. Two
// reports written inside one second therefore resolved to one path and the
// second destroyed the first, silently — each invocation had already printed
// that path as its own. The loss is owner-facing rather than cosmetic: the
// digest reaches the owner by reading these files, so a report carrying
// "## Needs you" that a quiet successor overwrote never reached him, and left
// nothing behind saying so (#168).

// collisionConfig writes a configuration whose reports live under the test's
// own directory. MELLIONS_HOME is pinned as well: Config.home() reads it, and
// an ambient one would put the digest marker and the shift stream on the
// machine's real records.
func collisionConfig(t *testing.T) (cfgPath, reportsDir string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MELLIONS_HOME", root)
	// assignments_root is pinned with the rest: unpinned, lanesNamingOwner reads
	// the operator's live store and the digest's output depends on the machine
	// the test runs on.
	b, err := json.Marshal(map[string]any{
		"report_root":      root,
		"assignments_root": filepath.Join(root, "assignments"),
		"repos":            []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(root, "config.json")
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, filepath.Join(root, "reports")
}

// writeReport runs one `report write` and returns the path it printed.
func writeReport(t *testing.T, cfgPath string, args ...string) string {
	t.Helper()
	out := captureStdout(t, func() {
		if err := cmdReport(append([]string{"write", "-config", cfgPath}, args...)); err != nil {
			t.Fatal(err)
		}
	})
	for _, line := range strings.Split(out, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), ".md") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("report write printed no path\noutput: %s", out)
	return ""
}

// mdCount is how many reports are on disk.
func mdCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			n++
		}
	}
	return n
}

// atFreshSecond sleeps to the next UTC second boundary, so the writes that
// follow have a whole second to land in. It buys the collision a wide window;
// it does not assume one, which is why every test below asserts that the
// reports it wrote actually did share a second.
func atFreshSecond(t *testing.T) {
	t.Helper()
	time.Sleep(time.Second - time.Duration(time.Now().UTC().Nanosecond()))
}

// stamp is the UTC second a report's name was built from.
func stamp(path string) string {
	base := filepath.Base(path)
	if len(base) < 15 {
		return base
	}
	return base[:15]
}

// TestReportsWrittenInOneSecondAllSurvive is the defect itself: three reports,
// one second, no -id. Before the claim they were one file and two of the three
// bodies were gone.
func TestReportsWrittenInOneSecondAllSurvive(t *testing.T) {
	cfgPath, dir := collisionConfig(t)
	bodies := []string{"the first report", "the second report", "the third report"}

	atFreshSecond(t)
	paths := make([]string, 0, len(bodies))
	for _, body := range bodies {
		paths = append(paths, writeReport(t, cfgPath, "-did", body))
	}

	// Without this the test can pass on a second boundary having proved
	// nothing: three reports in three seconds never collided in the first
	// place, and three distinct files say nothing about the defect.
	for _, p := range paths[1:] {
		if stamp(p) != stamp(paths[0]) {
			t.Fatalf("the writes straddled a second boundary, so no collision was exercised: %v", paths)
		}
	}

	if n := mdCount(t, dir); n != len(bodies) {
		t.Errorf("%d reports written in one second left %d files on disk, want %d\npaths: %v",
			len(bodies), n, len(bodies), paths)
	}
	// The printed path is a claim: this file holds what I just wrote. A report
	// that survives under a name its writer was never told is still a report
	// its writer cannot point anybody at.
	for i, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("report write printed %s, which cannot be read: %v", p, err)
			continue
		}
		if !strings.Contains(string(raw), bodies[i]) {
			t.Errorf("report write printed %s for %q, but that file holds:\n%s", p, bodies[i], raw)
		}
	}
}

// TestReportsWrittenInOneSecondUnderOneIdAllSurvive: the assignment id was the
// only other thing in the name, so one lane writing twice in a second collided
// with itself. Naming the lane never separated them.
func TestReportsWrittenInOneSecondUnderOneIdAllSurvive(t *testing.T) {
	cfgPath, dir := collisionConfig(t)
	const id = "report-collision-42"

	atFreshSecond(t)
	first := writeReport(t, cfgPath, "-id", id, "-did", "what the lane established")
	second := writeReport(t, cfgPath, "-id", id, "-did", "what the lane did next")

	if stamp(first) != stamp(second) {
		t.Fatalf("the writes straddled a second boundary, so no collision was exercised: %s, %s", first, second)
	}
	if first == second {
		t.Fatalf("two reports under one id in one second were given one path: %s", first)
	}
	if n := mdCount(t, dir); n != 2 {
		t.Errorf("one lane writing twice in one second left %d files, want 2", n)
	}
	raw, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("the first report is gone from %s: %v", first, err)
	}
	if !strings.Contains(string(raw), "what the lane established") {
		t.Errorf("the first report's body was replaced by its successor's:\n%s", raw)
	}
}

// TestDigestStillSeesAReportASameSecondSuccessorWouldHaveDestroyed is the
// property that matters. The file count is a proxy for it: what makes the
// collision owner-facing is that reportLines walks the directory, so a report
// asking for the owner that a quiet one overwrote is not late — it is gone,
// and the digest cannot know it ever existed.
func TestDigestStillSeesAReportASameSecondSuccessorWouldHaveDestroyed(t *testing.T) {
	cfgPath, _ := collisionConfig(t)
	const asking = "merge the pull request, it is a protected branch"

	atFreshSecond(t)
	needs := writeReport(t, cfgPath, "-needs-owner", asking)
	quiet := writeReport(t, cfgPath, "-did", "a quiet run with nothing in it")

	if stamp(needs) != stamp(quiet) {
		t.Fatalf("the writes straddled a second boundary, so no collision was exercised: %s, %s", needs, quiet)
	}

	out := captureStdout(t, func() {
		if err := cmdReport([]string{"digest", "-config", cfgPath}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, asking) {
		t.Errorf("a report asking for the owner did not reach the digest after a quiet report\n"+
			"was written in the same second\nasked: %q\ndigest:\n%s", asking, out)
	}
}

// TestClaimReportPathNeverReturnsATakenName exercises the claim without a clock
// at all: the same name asked for repeatedly is what two sessions finishing
// together produce, and no reachable arrangement of the wall clock changes what
// the filesystem is being asked here.
func TestClaimReportPathNeverReturnsATakenName(t *testing.T) {
	dir := t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		path, err := claimReportPath(dir, "20260829-041500")
		if err != nil {
			t.Fatalf("claim %d: %v", i+1, err)
		}
		if seen[path] {
			t.Fatalf("claim %d handed out %s, which was already taken", i+1, path)
		}
		seen[path] = true
		if err := os.WriteFile(path, []byte("report "+path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n := mdCount(t, dir); n != 5 {
		t.Errorf("five claims left %d files, want 5", n)
	}

	// The loop is bounded, and a bound that returns a name it cannot have is
	// the defect back again under a different mechanism.
	full := t.TempDir()
	for i := 0; i < reportSuffixes; i++ {
		p, err := claimReportPath(full, "20260829-041500")
		if err != nil {
			t.Fatalf("filling claim %d: %v", i+1, err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := claimReportPath(full, "20260829-041500"); err == nil {
		t.Error("claimReportPath handed out a name past its bound instead of refusing")
	}
}

// TestAFailedWriteGivesBackOnlyAnEmptyClaim.
//
// durable.Write commits by renaming and then flushes the directory, and the
// flush is what it returns — so it can report failure on a path whose content
// is already on disk and correct. Releasing the claim unconditionally would
// delete that report to tidy up after a flush, which is #168 again in the
// cleanup for it.
func TestAFailedWriteGivesBackOnlyAnEmptyClaim(t *testing.T) {
	dir := t.TempDir()

	claimed := filepath.Join(dir, "20260829-041500.md")
	if err := os.WriteFile(claimed, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	releaseUnwrittenClaim(claimed)
	if _, err := os.Stat(claimed); !os.IsNotExist(err) {
		t.Errorf("an empty claim was not given back: stat %s -> %v", claimed, err)
	}

	// The case that matters: the rename landed, the directory flush did not.
	written := filepath.Join(dir, "20260829-041501.md")
	const body = "# 2026-08-29 04:15 UTC\n\n## Needs you\n\nthe decision is yours\n"
	if err := os.WriteFile(written, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseUnwrittenClaim(written)
	raw, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("a written report was deleted while releasing its claim: %v", err)
	}
	if string(raw) != body {
		t.Errorf("the report on disk is not what was written:\n%s", raw)
	}
}

// TestLatestAnswersWithTheReportWrittenLast: `latest` sorted names as strings,
// and "-" sorts below ".", so once claimReportPath began disambiguating,
// 20260829-041500.md ranked above its own -2 successor and `latest -n 1`
// answered with the older of the two.
func TestLatestAnswersWithTheReportWrittenLast(t *testing.T) {
	cfgPath, dir := collisionConfig(t)

	atFreshSecond(t)
	first := writeReport(t, cfgPath, "-did", "written first")
	second := writeReport(t, cfgPath, "-did", "written second")
	if stamp(first) != stamp(second) {
		t.Fatalf("the writes straddled a second boundary, so no collision was exercised: %s, %s", first, second)
	}

	paths, err := latestReports(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("latest -n 1 returned %d paths", len(paths))
	}
	if paths[0] != second {
		t.Errorf("latest -n 1 answered with %s; the report written last was %s", paths[0], second)
	}
}

// TestLatestDoesNotReadAnIdAsACollisionSuffix.
//
// "<timestamp>-<suffix>" and "<timestamp>-<id ending in -N>" are the same
// string. Reading the tail of an id like "review-16" as suffix 16 sorts that
// lane's report above sixteen reports it was written before, and this
// repository names branches that way.
func TestLatestDoesNotReadAnIdAsACollisionSuffix(t *testing.T) {
	dir := t.TempDir()
	const stamp = "20260829-041500"
	// Written oldest first; "-review-16" is written last and is the newest.
	for _, id := range []string{"review-2", "review-16"} {
		name := filepath.Join(dir, stamp+"-"+id+".md")
		if err := os.WriteFile(name, []byte("report for "+id+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if base, n := reportOrder(stamp + "-review-16.md"); n != 1 {
		t.Errorf("the id review-16 was read as collision suffix %d of %q", n, base)
	}
	paths, err := latestReports(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, stamp+"-review-2.md")
	if paths[0] != want {
		t.Errorf("latest -n 1 answered with %s; by name order it is %s",
			filepath.Base(paths[0]), filepath.Base(want))
	}
	// The bare-timestamp case still reads its suffix as a number.
	if base, n := reportOrder(stamp + "-12.md"); n != 12 || base != stamp {
		t.Errorf("reportOrder(%q-12.md) = %q, %d; want %q, 12", stamp, base, n, stamp)
	}
}

// TestLatestOrdersTheSuffixAsANumber.
//
// Two siblings do not prove the ordering: dropping ".md" alone puts -2 above
// the bare name, because a prefix sorts below what extends it. The suffix has
// to be read as a number from the tenth sibling on, where "-9" sorts above
// "-10" as text and `latest` starts answering with a report six writes old.
//
// No clock: the names are claimed directly, which is what a second holding ten
// reports produces and what a two-write test cannot reach.
func TestLatestOrdersTheSuffixAsANumber(t *testing.T) {
	dir := t.TempDir()
	const stamp = "20260829-041500"

	var last string
	for i := 1; i <= 12; i++ {
		path, err := claimReportPath(dir, stamp)
		if err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf("report number %d\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		last = path
	}

	paths, err := latestReports(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("latest -n 1 returned %d paths", len(paths))
	}
	if paths[0] != last {
		raw, _ := os.ReadFile(paths[0])
		t.Errorf("latest -n 1 answered with %s (%s); the report written last was %s",
			filepath.Base(paths[0]), strings.TrimSpace(string(raw)), filepath.Base(last))
	}
}

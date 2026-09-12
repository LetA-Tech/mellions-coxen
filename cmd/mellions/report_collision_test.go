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

// atSecond is a stated UTC second, offset seconds from a fixed base. Nothing
// about the base matters; what matters is that the test names the second
// instead of the host's clock choosing it.
func atSecond(offset int) time.Time {
	return time.Date(2026, 8, 29, 4, 15, 0, 0, time.UTC).Add(time.Duration(offset) * time.Second)
}

// atSecondStamp is the name every report written at atSecond(0) starts with. It
// is written out rather than derived, so the tests below have an oracle the
// naming code cannot move.
const atSecondStamp = "20260829-041500"

// writeReportAt runs one report write at a stated UTC second and returns the
// path it printed.
//
// The second is stated rather than observed. A report's name has one-second
// resolution, so whether two reports collide is decided by the second they were
// written in — and a test that lets the host's clock decide it is asserting
// about the host's load. Stated, both directions are reachable on purpose and
// neither one is load-sensitive.
func writeReportAt(t *testing.T, dir string, now time.Time, r reportBody) string {
	t.Helper()
	var out strings.Builder
	if err := reportWrite(dir, r, now, &out); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), ".md") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("report write printed no path\noutput: %s", out.String())
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
	_, dir := collisionConfig(t)
	bodies := []string{"the first report", "the second report", "the third report"}

	paths := make([]string, 0, len(bodies))
	for _, body := range bodies {
		paths = append(paths, writeReportAt(t, dir, atSecond(0), reportBody{did: body}))
	}

	// Without this the test can pass having proved nothing: three reports in
	// three seconds never collided in the first place, and three distinct files
	// say nothing about the defect. The second is stated, so this is no longer a
	// window that load can close — it is the assertion that the stated second
	// reached the name, against a literal the naming code cannot move.
	for _, p := range paths {
		if stamp(p) != atSecondStamp {
			t.Fatalf("a report written at %s was named %s, so no collision was exercised: %v",
				atSecondStamp, stamp(p), paths)
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

// TestReportsWrittenInDifferentSecondsTakeTheirOwnNames is the other half of
// the property, and until the clock was a parameter no test could state it: the
// disambiguation is meant to fire only on a real collision. A claim that handed
// out a suffix for every write would satisfy every test above and be wrong —
// each report's name would stop being the second it was written in, which is
// what `latest`, the digest's ordering and every reader of a report directory
// read it for.
func TestReportsWrittenInDifferentSecondsTakeTheirOwnNames(t *testing.T) {
	_, dir := collisionConfig(t)

	first := writeReportAt(t, dir, atSecond(0), reportBody{did: "written in the first second"})
	second := writeReportAt(t, dir, atSecond(1), reportBody{did: "written in the second"})

	// The whole name, not the stamp and not the absence of one suffix. The
	// suffix is the collision's mark, and a report that collided with nothing
	// must carry no mark at all — asking only that it is not "-2" says nothing
	// about "-3", and stamp() truncates at fifteen characters, so every suffix
	// satisfies it.
	for _, c := range []struct{ path, want string }{
		{first, atSecondStamp + ".md"},
		{second, "20260829-041501.md"},
	} {
		if got := filepath.Base(c.path); got != c.want {
			t.Errorf("a report that collided with nothing was named %s, want %s", got, c.want)
		}
	}
	if n := mdCount(t, dir); n != 2 {
		t.Errorf("two reports a second apart left %d files, want 2", n)
	}
	for _, c := range []struct{ path, body string }{
		{first, "written in the first second"},
		{second, "written in the second"},
	} {
		raw, err := os.ReadFile(c.path)
		if err != nil {
			t.Errorf("report write printed %s, which cannot be read: %v", c.path, err)
			continue
		}
		if !strings.Contains(string(raw), c.body) {
			t.Errorf("report write printed %s for %q, but that file holds:\n%s", c.path, c.body, raw)
		}
	}
}

// TestReportsWrittenInOneSecondUnderOneIdAllSurvive: the assignment id was the
// only other thing in the name, so one lane writing twice in a second collided
// with itself. Naming the lane never separated them.
func TestReportsWrittenInOneSecondUnderOneIdAllSurvive(t *testing.T) {
	_, dir := collisionConfig(t)
	const id = "report-collision-42"

	first := writeReportAt(t, dir, atSecond(0), reportBody{assignment: id, did: "what the lane established"})
	second := writeReportAt(t, dir, atSecond(0), reportBody{assignment: id, did: "what the lane did next"})

	if stamp(first) != atSecondStamp || stamp(second) != atSecondStamp {
		t.Fatalf("reports written at %s were named %s and %s, so no collision was exercised",
			atSecondStamp, first, second)
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
	cfgPath, dir := collisionConfig(t)
	const asking = "merge the pull request, it is a protected branch"

	needs := writeReportAt(t, dir, atSecond(0), reportBody{needsOwner: asking})
	quiet := writeReportAt(t, dir, atSecond(0), reportBody{did: "a quiet run with nothing in it"})

	if stamp(needs) != atSecondStamp || stamp(quiet) != atSecondStamp {
		t.Fatalf("reports written at %s were named %s and %s, so no collision was exercised",
			atSecondStamp, needs, quiet)
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

// TestDigestSurfacesABlockedReport holds the other arm of the digest's switch.
// The writer's section heading and the reader's are one literal on each side and
// nothing binds them, so a writer that renamed its own heading would drop every
// blocked shift out of the digest with no reader to notice.
func TestDigestSurfacesABlockedReport(t *testing.T) {
	cfgPath, dir := collisionConfig(t)
	const stopped = "the migration needs an owner who holds production credentials"

	writeReportAt(t, dir, atSecond(0), reportBody{blocked: stopped})

	out := captureStdout(t, func() {
		if err := cmdReport([]string{"digest", "-config", cfgPath}); err != nil {
			t.Fatal(err)
		}
	})
	if want := "blocked: " + stopped; !strings.Contains(out, want) {
		t.Errorf("a blocked report did not reach the digest\nwant a line carrying %q\ndigest:\n%s",
			want, out)
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
	_, dir := collisionConfig(t)

	first := writeReportAt(t, dir, atSecond(0), reportBody{did: "written first"})
	second := writeReportAt(t, dir, atSecond(0), reportBody{did: "written second"})
	if stamp(first) != atSecondStamp || stamp(second) != atSecondStamp {
		t.Fatalf("reports written at %s were named %s and %s, so no collision was exercised",
			atSecondStamp, first, second)
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

// TestCmdReportWriteNamesTheReportFromTheClock binds the call site.
//
// reportWrite takes the second as a parameter, so every test above states one.
// That leaves the command's own argument unasserted: a cmdReport passing a
// constant names every report the same second, and the whole suite above stays
// green because none of it goes through cmdReport's argument.
//
// The tolerance is the point. This asserts the name's second came from the
// clock, not which second it was, so it cannot reintroduce the alignment the
// stated seconds removed: the name is the write instant truncated, and a
// truncated instant is never below the floor of a reading taken before it nor
// above one taken after.
func TestCmdReportWriteNamesTheReportFromTheClock(t *testing.T) {
	cfgPath, dir := collisionConfig(t)

	before := time.Now().UTC()
	if err := cmdReport([]string{"write", "-config", cfgPath, "-did", "a run with a real clock"}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()

	if n := mdCount(t, dir); n != 1 {
		t.Fatalf("one write left %d reports on disk", n)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			name = e.Name()
		}
	}

	named, err := time.Parse(reportStamp, stamp(name))
	if err != nil {
		t.Fatalf("report %s is not named from a UTC second: %v", name, err)
	}
	if floor := before.Truncate(time.Second); named.Before(floor) || named.After(after) {
		t.Errorf("report %s is named %s; the write happened between %s and %s, so the command did not name it from the clock",
			name, named.Format(time.RFC3339), floor.Format(time.RFC3339), after.Format(time.RFC3339))
	}
}

// TestAReportsHeadingIsTheSecondItsNameCarries binds the two readers of one
// instant. reportWrite formats the name and the document's heading from the
// same now, and nothing asserted that they agree — a heading built from a
// different instant leaves the whole package green while the report
// contradicts itself in the owner's hands, which is where reports are read.
func TestAReportsHeadingIsTheSecondItsNameCarries(t *testing.T) {
	_, dir := collisionConfig(t)
	path := writeReportAt(t, dir, atSecond(0), reportBody{did: "one instant, two readers of it"})

	if got, want := filepath.Base(path), atSecondStamp+".md"; got != want {
		t.Fatalf("report named %s, want %s", got, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(strings.TrimSpace(string(raw)), "\n", 2)[0]
	if want := "# 2026-08-29 04:15 UTC"; first != want {
		t.Errorf("the report named %s carries the heading %q, want %q: the name and the document disagree about when it was written",
			filepath.Base(path), first, want)
	}
}

// TestAReportsHeadingCarriesTheLaneItsNameDoes is the same binding one field
// over. reportWrite writes the assignment id into both artifacts — into the
// name at report.go:127-129 and into the heading at :132-134 — and until this
// test nothing asserted the second one: deleting the heading's suffix outright
// left the whole package green, so a report that stopped naming its lane in its
// own title while the file name still carried it passed the gate.
//
// Both wants are whole literals rather than atSecondStamp plus the id, for the
// reason atSecondStamp itself is written out: an oracle assembled from the same
// pieces the naming code uses follows the naming code wherever it goes.
func TestAReportsHeadingCarriesTheLaneItsNameDoes(t *testing.T) {
	_, dir := collisionConfig(t)
	path := writeReportAt(t, dir, atSecond(0), reportBody{
		assignment: "report-collision-42",
		did:        "one lane, two readers of it",
	})

	if got, want := filepath.Base(path), "20260829-041500-report-collision-42.md"; got != want {
		t.Fatalf("report named %s, want %s", got, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(strings.TrimSpace(string(raw)), "\n", 2)[0]
	if want := "# 2026-08-29 04:15 UTC — report-collision-42"; first != want {
		t.Errorf("the report named %s carries the heading %q, want %q: the name and the document disagree about which lane it is from",
			filepath.Base(path), first, want)
	}
}

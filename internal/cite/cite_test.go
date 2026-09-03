// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package cite

import (
	"errors"
	"strings"
	"testing"
)

// tree is a resolver over a fixed set of files. A path it does not hold
// answers with an error, which is what a real tree does for a URL host or
// another repository's path.
type tree map[string]string

func (t tree) read(path string) ([]string, error) {
	body, ok := t[path]
	if !ok {
		return nil, errors.New("no such file")
	}
	return strings.Split(body, "\n"), nil
}

func kinds(fs []Finding) map[string]Kind {
	out := map[string]Kind{}
	for _, f := range fs {
		out[f.Raw] = f.Kind
	}
	return out
}

// The five citations #149 and its comments record as published wrong. Every
// one of them resolves — the file has that many lines — so the existence check
// the issue proposed passes all five. Each must be a finding here, or this
// package does not do the thing it was built for.
func TestPublishedFailuresAreFindings(t *testing.T) {
	files := tree{
		"goals.go": strings.Repeat("x\n", 63) +
			"\tcache.Set(key, out)\n" + // :64, cited for the return at :65
			"\treturn out, nil\n" + // :65
			strings.Repeat("y\n", 23) +
			"}\n" + // :89, cited for the guard at :91
			"\n" +
			"\tif err != nil {\n", // :91
		"internal/advisor/grounding/facts.go": strings.Repeat("z\n", 238) +
			"\t// three lines above the assignment\n" + // :239
			"\tb := 1\n\tc := 2\n" +
			"\tfacts.Balance = acct.Current\n", // :242
		"CONTRIBUTING.md": strings.Repeat("w\n", 81) +
			"Changing logic includes updating or removing its comments.\n" + // :82
			strings.Repeat("v\n", 10) +
			"No public license has been selected. Until the owner establishes one, do not\n", // :93
		"hooks/test-session-digest.sh": strings.Repeat("s\n", 72) +
			"# a comment, six lines below the assertion this used to be\n", // :73
	}

	// How each was actually published: the line named in prose, the claim
	// paraphrased, no quotation of the line itself.
	doc := `The cache write at goals.go:64 returns the built goals, and the guard at
goals.go:89 rejects a nil account. The assignment at
internal/advisor/grounding/facts.go:239 grounds the balance.
Mellions' source carries an unselected licence (CONTRIBUTING.md:82).
The assertion at hooks/test-session-digest.sh:73 passes vacuously.`

	got := kinds(Check(doc, files.read))
	for _, raw := range []string{
		"goals.go:64",
		"goals.go:89",
		"internal/advisor/grounding/facts.go:239",
		"CONTRIBUTING.md:82",
		"hooks/test-session-digest.sh:73",
	} {
		k, ok := got[raw]
		if !ok {
			t.Errorf("%s: published wrong and reported clean", raw)
			continue
		}
		if k != Unbacked {
			t.Errorf("%s: kind = %v, want Unbacked", raw, k)
		}
	}
	if len(got) != 5 {
		t.Errorf("findings = %d, want 5: %v", len(got), got)
	}
}

// The positive control the negative one is worthless without: a citation
// written the way mellions-deep-research asks — the line quoted under it —
// must pass, or the check refuses every honest citation too and gets disabled.
func TestQuotedCitationPasses(t *testing.T) {
	files := tree{"goals.go": strings.Repeat("x\n", 63) + "\tcache.Set(key, out)\n"}
	docs := map[string]string{
		"fenced":        "The write at goals.go:64:\n\n```go\n\tcache.Set(key, out)\n```\n",
		"blockquote":    "The write at goals.go:64:\n\n> cache.Set(key, out)\n",
		"inline span":   "The write at `goals.go:64` — `cache.Set(key, out)` — caches it.\n",
		"indented":      "The write at goals.go:64:\n\n    cache.Set(key, out)\n",
		"grep -n paste": "goals.go:64\n\n```\n64:\tcache.Set(key, out)\n```\n",
		"re-indented":   "goals.go:64\n\n```\ncache.Set(key, out)\n```\n",
	}
	for name, doc := range docs {
		if f := Check(doc, files.read); len(f) != 0 {
			t.Errorf("%s: %v, want clean", name, f[0].Reason())
		}
	}
}

// A line the file does not have is the one case the issue's existence check
// would have caught, and it must still be caught here.
func TestMissingLine(t *testing.T) {
	files := tree{"CONTRIBUTING.md": strings.Repeat("w\n", 95)}
	f := Check("see CONTRIBUTING.md:400 for the rule", files.read)
	if len(f) != 1 || f[0].Kind != Missing {
		t.Fatalf("got %v, want one Missing", f)
	}
}

// A blank line can never be quoted, so a citation to one can never be backed;
// reporting it as an ordinary Unbacked with an empty Actual would tell the
// author their line "says """.
func TestBlankLineReadsAsBlank(t *testing.T) {
	files := tree{"a.go": "package a\n\nfunc F() {}\n"}
	f := Check("see a.go:2", files.read)
	if len(f) != 1 {
		t.Fatalf("got %v, want one finding", f)
	}
	if !strings.Contains(f[0].Reason(), "blank") {
		t.Errorf("Reason() = %q, want it to say the line is blank", f[0].Reason())
	}
}

// A checker that denies on noise gets turned off. Everything here is a
// word:number that is not a code citation, and every one must be silent.
func TestNotCitations(t *testing.T) {
	files := tree{"a.go": strings.Repeat("x\n", 500), "Makefile": strings.Repeat("x\n", 50)}
	doc := `Corrected at issues/656#issuecomment-5459328714 at 15:11 UTC.
The tunnel is http://127.0.0.1:9428 and the host is 192.0.2.6:8428.
Shift 20260829-002635 ran 3:1 against it. See docker.io/library/go:1.22.
A range read of a.go:1-125 is a region, not a line.
Makefile: the target is there. The ratio is 100:1.`
	if f := Check(doc, files.read); len(f) != 0 {
		t.Errorf("denied on noise: %s", f[0].Reason())
	}
}

// A path:line inside a fenced block is quotation, not claim. Pasted `go test`
// and `go vet` output carries a real file and a real number the author is
// reporting; a survey of 40 pull requests and 40 issues in this repository
// found 25 such lines, every one of which would be denied as an unbacked
// citation if quoted text were read as authored text.
func TestQuotedOutputIsNotACitation(t *testing.T) {
	files := tree{
		"cmd/mellions/report.go":                strings.Repeat("x\n", 200),
		"cmd/mellions/report_collision_test.go": strings.Repeat("x\n", 300),
	}
	doc := "`make check` exit 0 after the fix. Before it:\n\n" +
		"```\n" +
		"--- FAIL: TestReportCollision\n" +
		"    report_collision_test.go:117: 3 reports written in one second left 1 file on disk, want 3\n" +
		"```\n\n" +
		"and `go vet` reported:\n\n" +
		"> cmd/mellions/report.go:159: unreachable code\n"
	if f := Check(doc, files.read); len(f) != 0 {
		t.Errorf("read quoted output as a citation: %s", f[0].Reason())
	}
}

// The noise a real corpus carries, from the same survey: 218 lane names with a
// date shard, 59 image tags, ~180 wall-clock times, ~70 ISO timestamps. Every
// one is a word:number and none is a citation.
func TestCorpusNoiseIsSilent(t *testing.T) {
	files := tree{"a.go": strings.Repeat("x\n", 500)}
	doc := `Lane review-a:2030 and review-b:2030 and frontend-42:2030.
Images postgres:18, golang:1, schema:1, docker.io/library/go:1.22.
Runs at 03:08/03:11/03:15 all show startedAt == createdAt, at 2026-08-29T15:11.
Config unless-stopped:0 and session_start:0. Base was 7944 bytes.`
	if f := Check(doc, files.read); len(f) != 0 {
		t.Errorf("denied on corpus noise: %s", f[0].Reason())
	}
}

// The most exact evidence an author can offer is `grep -n` output pasted as
// it came. Reading only the spans on a prose line rejected it, and rejected it
// by asserting the body quoted no line equal to one it quoted verbatim on that
// same line.
func TestUnfencedGrepOutputBacksItsOwnCitation(t *testing.T) {
	files := tree{"internal/cite/cite.go": strings.Repeat("x\n", 56) +
		"\t// Missing: the file has fewer lines than the citation claims.\n"}
	doc := "The Missing kind is declared here:\n\n" +
		"internal/cite/cite.go:57:\t// Missing: the file has fewer lines than the citation claims.\n"
	if f := Check(doc, files.read); len(f) != 0 {
		t.Errorf("denied evidence the body quotes verbatim: %s", f[0].Reason())
	}
}

// Prose here is written with en and em dashes, so a range wearing one is still
// a range and must not be judged as a citation to its first line.
func TestDashRangesAreStillRanges(t *testing.T) {
	for _, dash := range []string{"-", "–", "—"} {
		got := Extract("read internal/cite/cite.go:57" + dash + "63 for the kinds")
		if len(got) != 0 {
			t.Errorf("%q range extracted as a line citation: %v", dash, got)
		}
	}
}

// A path this tree does not hold is not this checker's to judge — another
// repository's file, cited as repo/path:line, cannot be resolved here and
// must not be reported as though it were wrong.
func TestForeignPathIsSilent(t *testing.T) {
	files := tree{"a.go": strings.Repeat("x\n", 10)}
	if f := Check("see advisor-service/internal/grpc/goals.go:64", files.read); len(f) != 0 {
		t.Errorf("judged a path it cannot read: %s", f[0].Reason())
	}
}

// A range is not a citation to a line; a bare line beside it still is.
func TestExtractSkipsRanges(t *testing.T) {
	got := Extract("read goals.go:1-125 then cited goals.go:64")
	if len(got) != 1 || got[0].Raw != "goals.go:64" {
		t.Fatalf("Extract = %v, want only goals.go:64", got)
	}
}

// The same citation written twice is one citation, so a body that repeats it
// does not multiply the denial.
func TestExtractDedupes(t *testing.T) {
	if got := Extract("a.go:1 and again a.go:1"); len(got) != 1 {
		t.Errorf("Extract = %v, want one", got)
	}
}

// The defect this package exists to close, reproduced as it was published:
// paste the range you read, cite a line inside it that says something else.
// The paste carries line 64's text, so a document-global match passes it —
// and that is occurrence #1's own mechanism, a range read feeling like having
// opened the line, surviving the check built to stop it. The block under a
// citation has to start at the line it names.
func TestARangePasteDoesNotBackALineInsideIt(t *testing.T) {
	files := tree{"internal/cite/cite.go": "// Kind is why a citation does not hold.\n" + // :1
		"type Kind int\n" + // :2
		"\n" +
		"const (\n" +
		"\tMissing Kind = iota\n" + // :5
		"\tUnbacked\n" +
		")\n" +
		"\n" +
		"type Finding struct {\n"} // :9
	doc := "The Kind constant is `internal/cite/cite.go:9`:\n\n" +
		"```go\n" +
		"// Kind is why a citation does not hold.\n" +
		"type Kind int\n" +
		"\n" +
		"const (\n" +
		"\tMissing Kind = iota\n" +
		"\tUnbacked\n" +
		")\n" +
		"\n" +
		"type Finding struct {\n" +
		"```\n"
	f := Check(doc, files.read)
	if len(f) != 1 || f[0].Kind != Unbacked {
		t.Fatalf("got %v, want the citation reported unbacked", f)
	}
	if !strings.Contains(f[0].Reason(), "type Finding struct {") {
		t.Errorf("Reason() = %q, want it to report what line 9 says", f[0].Reason())
	}
}

// One "}" written once must not back every citation to a closing brace. That
// is occurrence #2 — "goals.go:89 is a closing brace" — and it is what an
// exclusive pairing is for: a quotation is spent when it backs a citation.
func TestOneQuotationBacksOneCitation(t *testing.T) {
	files := tree{"internal/cite/cite.go": strings.Repeat("x\n", 50) +
		"}\n" + // :51
		strings.Repeat("y\n", 17) +
		"}\n"} // :69
	doc := "The extractor ends at `internal/cite/cite.go:51` and the prose walker at\n" +
		"`internal/cite/cite.go:69`. Both close with `}`.\n"
	f := Check(doc, files.read)
	if len(f) != 1 {
		t.Fatalf("got %d findings, want exactly one — the second brace citation is unbacked: %v", len(f), f)
	}
	if f[0].Raw != "internal/cite/cite.go:51" {
		t.Errorf("unbacked citation = %s, want the one with no quotation on its line", f[0].Raw)
	}
}

// Anchoring alone still lets one quotation serve every citation anchored to
// it — two citations on one line reaching the same span, or two introducing
// one block. A quotation is spent when it backs a citation, so the second
// needs its own.
func TestAQuotationIsSpentOnOneCitation(t *testing.T) {
	files := tree{"internal/cite/cite.go": strings.Repeat("x\n", 50) +
		"}\n" + // :51
		strings.Repeat("y\n", 17) +
		"}\n"} // :69
	docs := map[string]string{
		"one span, two citations on its line": "Both `internal/cite/cite.go:51` and `internal/cite/cite.go:69` close with `}`.\n",
		"one block, two citations above it":   "Both `internal/cite/cite.go:51` and `internal/cite/cite.go:69`:\n\n```go\n}\n```\n",
	}
	for name, doc := range docs {
		f := Check(doc, files.read)
		if len(f) != 1 {
			t.Errorf("%s: got %d findings, want one — a second citation cannot spend the same quotation: %v", name, len(f), f)
		}
	}
}

// Prose wraps, and a citation that lands at the end of a line is still
// introducing the block under its paragraph. Found by running this checker
// over the pull-request body describing it, which cited CONTRIBUTING.md with
// the rest of the sentence on the next line and was denied for it.
func TestACitationReachesTheBlockUnderItsParagraph(t *testing.T) {
	files := tree{"CONTRIBUTING.md": strings.Repeat("w\n", 81) +
		"Changing logic includes updating or removing its comments.\n"}
	doc := "Verified at the artifact — `CONTRIBUTING.md:82`\n" +
		"on `dev`, in a file of 95 lines:\n\n" +
		"```\nChanging logic includes updating or removing its comments.\n```\n"
	if f := Check(doc, files.read); len(f) != 0 {
		t.Errorf("denied a citation whose sentence wraps: %s", f[0].Reason())
	}
}

// A quotation belongs to the citation it is written under. A block sitting
// after a paragraph of prose backs whatever introduced it, not a citation
// further up the document.
func TestBackingMustBeAnchoredToTheCitation(t *testing.T) {
	files := tree{"goals.go": strings.Repeat("x\n", 63) + "\tcache.Set(key, out)\n"}
	doc := "The write at goals.go:64 caches the built goals.\n\n" +
		"Unrelated prose about something else entirely.\n\n" +
		"```go\n\tcache.Set(key, out)\n```\n"
	if f := Check(doc, files.read); len(f) != 1 {
		t.Errorf("got %v, want the citation reported unbacked", f)
	}
}

// The positive control the two above are worthless without: one block per
// citation, each starting at the line its citation names, is the ordinary
// honest shape and must stay silent.
func TestEachCitationWithItsOwnBlockPasses(t *testing.T) {
	files := tree{"internal/cite/cite.go": strings.Repeat("x\n", 50) +
		"}\n" + // :51
		strings.Repeat("y\n", 17) +
		"}\n"} // :69
	doc := "The extractor ends at `internal/cite/cite.go:51`:\n\n" +
		"```go\n}\n```\n\n" +
		"and the prose walker at `internal/cite/cite.go:69`:\n\n" +
		"```go\n}\n```\n"
	if f := Check(doc, files.read); len(f) != 0 {
		t.Errorf("denied a citation quoted the way the Skill asks: %s", f[0].Reason())
	}
}

// A citation's own line is quoted text, so a document that quotes its
// citations must not thereby back them: `goals.go:64` as a span says nothing
// about what line 64 holds.
func TestCitationSpanDoesNotBackItself(t *testing.T) {
	files := tree{"goals.go": strings.Repeat("x\n", 63) + "\tcache.Set(key, out)\n"}
	if f := Check("the write at `goals.go:64` caches it", files.read); len(f) != 1 {
		t.Errorf("got %v, want the citation reported unbacked", f)
	}
}

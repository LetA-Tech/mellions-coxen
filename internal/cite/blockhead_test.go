// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package cite

import (
	"strings"
	"testing"
)

// guardFile is the probe target: line 739 and line 740 read differently.
var guardFile = tree{"internal/guard/findata_schema_pin_guard_test.go": strings.Repeat("x\n", 738) +
	"func TestPinnedFindataSchemaIsCurrent(t *testing.T) {\n" + // :739
	"\tt.Parallel()\n"} // :740

// The refusal's remedy is "quote the line under the citation", and the natural
// way to follow it is a block whose first line is the citation and whose next
// line is the quotation. That block is a citation, graded like one: a wrong
// number refused, the right one admitted and counted.
func TestABlockHeadedByALoneCitationIsGraded(t *testing.T) {
	const line = "func TestPinnedFindataSchemaIsCurrent(t *testing.T) {"
	for name, c := range map[string]struct {
		doc     string
		refused bool
	}{
		"fence, wrong number":     {"```go\ninternal/guard/findata_schema_pin_guard_test.go:740\n" + line + "\n```\n", true},
		"fence, right number":     {"```go\ninternal/guard/findata_schema_pin_guard_test.go:739\n" + line + "\n```\n", false},
		"tilde fence, wrong":      {"~~~\ninternal/guard/findata_schema_pin_guard_test.go:740\n" + line + "\n~~~\n", true},
		"blank before the quote":  {"```\n\ninternal/guard/findata_schema_pin_guard_test.go:740\n\n" + line + "\n```\n", true},
		"indented, wrong number":  {"Before:\n\n    internal/guard/findata_schema_pin_guard_test.go:740\n    " + line + "\n", true},
		"indented, right number":  {"Before:\n\n    internal/guard/findata_schema_pin_guard_test.go:739\n    " + line + "\n", false},
		"after prose, wrong":      {"The guard:\n\n```go\ninternal/guard/findata_schema_pin_guard_test.go:740\n" + line + "\n```\n", true},
		"first of several pairs":  {"```\ninternal/guard/findata_schema_pin_guard_test.go:740\n    " + line + "\ninternal/guard/findata_schema_pin_guard_test.go:739\n    " + line + "\n```\n", true},
		"repo-prefixed, accepted": {"```\nmcfo-finledger/internal/guard/findata_schema_pin_guard_test.go:739\n" + line + "\n```\n", false},
	} {
		if got := len(Extract(c.doc)); got != 1 {
			t.Errorf("%s: %d citations graded, want 1", name, got)
		}
		read := guardFile.read
		if strings.Contains(c.doc, "mcfo-finledger/") {
			read = func(p string) ([]string, error) {
				return guardFile.read(strings.TrimPrefix(p, "mcfo-finledger/"))
			}
		}
		f := firstOnly(Check(c.doc, read))
		if c.refused && len(f) != 1 {
			t.Errorf("%s: findings = %v, want the wrong number refused", name, f)
		}
		if !c.refused && len(f) != 0 {
			t.Errorf("%s: refused a right citation: %s", name, f[0].Reason())
		}
	}
}

// The exemption the block form leans on stays whole: tool output pasted into
// a block carries real paths the author is reporting, and none of it is a
// citation — a test failure, a vet line with a column, `grep -n`, a stack
// frame, and a `grep -c` listing whose lone path:count lines are not headers.
func TestToolOutputOpeningABlockIsStillNotACitation(t *testing.T) {
	files := tree{
		"internal/cite/cite.go":   strings.Repeat("x\n", 600),
		"internal/cite/a_test.go": strings.Repeat("x\n", 300),
		"cmd/mellions/report.go":  strings.Repeat("x\n", 200),
	}
	for name, doc := range map[string]string{
		"go test":                       "```\ninternal/cite/a_test.go:117: 3 reports written, want 3\n--- FAIL: TestX\n```\n",
		"go vet":                        "```\ncmd/mellions/report.go:159:2: unreachable code\nexit status 1\n```\n",
		"grep -n":                       "```\ninternal/cite/cite.go:210:// path:line inside a fenced block or a\ninternal/cite/cite.go:501:\t\tcase x:\n```\n",
		"frame":                         "```\n\t/go/src/x/internal/cite/cite.go:42 +0x1d\nmain.main()\n```\n",
		"grep -c":                       "```\ninternal/cite/cite.go:12\ninternal/cite/a_test.go:40\ncmd/mellions/report.go:3\n```\n",
		"indented":                      "Output:\n\n    internal/cite/cite.go:12\n    internal/cite/a_test.go:40\n",
		"grep -c, extensionless second": "```\ninternal/cite/cite.go:1\nMakefile:1\nREADME.md:6\n```\n",
		"grep -c, later pair":           "```\ninternal/cite/cite.go:3\nx\nMakefile:1\nREADME.md:6\n```\n",
		"grep diagnostic":               "```\ninternal/cite/cite.go:3\ngrep: internal/cite: Is a directory\ncmd/mellions/report.go:5\n```\n",
	} {
		if got := Extract(doc); len(got) != 0 {
			t.Errorf("%s: graded %v as a citation", name, got)
		}
		if f := firstOnly(Check(doc, files.read)); len(f) != 0 {
			t.Errorf("%s: refused tool output: %s", name, f[0].Reason())
		}
	}
}

// A block headed by a citation is that citation's quotation, so a prose
// citation introducing it cannot spend its head.
func TestABlockHeadedByACitationBacksOnlyThatCitation(t *testing.T) {
	files := tree{"a.go": "same()\n", "b.go": "same()\n"}
	for name, doc := range map[string]string{
		"blank between": "The call at `a.go:1`:\n\n```\nb.go:1\nsame()\n```\n",
		"fence under":   "The call at `a.go:1`:\n```\nb.go:1\nsame()\n```\n",
	} {
		f := firstOnly(Check(doc, files.read))
		if len(f) != 1 || f[0].Raw != "a.go:1" {
			t.Errorf("%s: findings = %v, want a.go:1 alone refused and b.go:1 backed by its own block", name, f)
		}
	}
}

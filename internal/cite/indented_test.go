// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package cite

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// execFile is the probe target: its lines 12 and 13 read differently, and the
// test file's line 12 is a third line.
var execFile = tree{
	"pkg/exec.go": strings.Repeat("x\n", 11) +
		"\tmodelOut, err := e.generate(ctx, in)\n" + // :12
		"\treturn modelOut, err\n", // :13
	"pkg/exec_test.go": strings.Repeat("y\n", 11) + "\tgot := run(t)\n", // :12
}

const (
	execLine    = "modelOut, err := e.generate(ctx, in)"
	corrupted   = execLine + "XX"
	execCite    = "`pkg/exec.go:12`"
	testOutput  = "pkg/exec_test.go:12: got 1, want 2"
	testOutputs = testOutput + "\n--- FAIL: TestRun"
)

// graded checks a document the way the gate does: how many citations it reads,
// and whether one of them is refused.
func graded(doc string) (cited int, refused bool) {
	return len(Extract(doc)), len(firstOnly(Check(doc, execFile.read))) > 0
}

// A list item, and a wrapped line of one, is list content however far it is
// indented, so a citation on it is graded: refused on a wrong quotation and
// admitted on a right one.
func TestACitationOnAnIndentedListLineIsGraded(t *testing.T) {
	line := func(prefix, quote string) string {
		return prefix + "read, " + execCite + ": `" + quote + "`\n"
	}
	for name, c := range map[string]struct{ head, prefix string }{
		"bullet nested two spaces":         {"- parent\n", "  - "},
		"bullet nested four spaces":        {"- parent\n", "    - "},
		"bullet nested by a tab":           {"- parent\n", "\t- "},
		"nested under an ordered item":     {"1. parent\n", "    - "},
		"ordered nested four spaces":       {"- parent\n", "    1. "},
		"third level in four-space steps":  {"- a\n    - b\n", "        - "},
		"third level in two-space steps":   {"- a\n  - b\n", "    - "},
		"after a blank line, four spaces":  {"- parent\n\n", "    - "},
		"checkbox item, four spaces":       {"- parent\n", "    - [ ] "},
		"plus bullet nested four spaces":   {"+ parent\n", "    + "},
		"star bullet nested four spaces":   {"* parent\n", "    * "},
		"wrapped continuation of an item":  {"- parent that wraps\n", "      "},
		"paragraph continuation":           {"A paragraph whose second line\n", "    "},
		"continuation by a tab":            {"A paragraph whose second line\n", "\t"},
		"heading, then a nested list item": {"# H\n\n- parent\n", "    - "},
	} {
		if cited, refused := graded(c.head + line(c.prefix, execLine)); cited != 1 || refused {
			t.Errorf("%s, true quote: cited %d citations, refused = %v, want 1 admitted", name, cited, refused)
		}
		if cited, refused := graded(c.head + line(c.prefix, corrupted)); cited != 1 || !refused {
			t.Errorf("%s, corrupted quote: cited %d citations, refused = %v, want 1 refused", name, cited, refused)
		}
	}
}

// An indented code block is quotation: four columns past the margin or the
// content column of the item holding it, and not a paragraph's continuation.
// A go test line inside one is output the author reports, never a claim.
func TestAnIndentedCodeBlockIsStillQuotation(t *testing.T) {
	for name, doc := range map[string]string{
		"after a blank line":            "Output:\n\n    " + testOutputs + "\n",
		"after a blank line, a tab":     "Output:\n\n\t" + testOutputs + "\n",
		"after a heading":               "# Output\n    " + testOutput + "\n",
		"after a fence":                 "```\nx\n```\n    " + testOutput + "\n",
		"deeper than four":              "Output:\n\n        " + testOutput + "\n",
		"after a thematic break":        "---\n    " + testOutput + "\n",
		"in an item, content plus 4":    "- Output:\n\n      " + testOutputs + "\n",
		"in an item, a tab past it":     "- Output:\n\n  \t  " + testOutput + "\n",
		"in a nested item":              "- a\n  - Output:\n\n        " + testOutput + "\n",
		"in a four-space nested item":   "- a\n    - Output:\n\n          " + testOutput + "\n",
		"in an ordered item":            "1. Output:\n\n       " + testOutput + "\n",
		"in an item after its text":     "- Output:\n\n      " + testOutput + "\n\n- next\n",
		"continuing the block":          "Output:\n\n    first\n    " + testOutput + "\n",
		"across a blank line in it":     "Output:\n\n    first\n\n    " + testOutput + "\n",
		"in a block that holds a fence": "Output:\n\n    ```\n    " + testOutput + "\n    ```\n",
		"in a tab-indented fence":       "- Output:\n\n\t```\n\t" + testOutput + "\n\t```\n",
		"in an item, an empty item":     "-\n\n    " + testOutput + "\n",
	} {
		if cited, refused := graded(doc); cited != 0 || refused {
			t.Errorf("%s: cited %d citations, refused = %v, want the block left ungraded", name, cited, refused)
		}
	}
}

// An indented code block cannot interrupt a paragraph, so an indented line
// that continues one is prose and the citation on it is graded.
func TestAnIndentedLineContinuingAParagraphIsNotCode(t *testing.T) {
	for name, doc := range map[string]string{
		"after text":             "The failure was\n    " + testOutput + "\n",
		"after a tab-led line":   "The failure was\n\t" + testOutput + "\n",
		"after an item's text":   "- The failure was\n    " + testOutput + "\n",
		"after a lazy line":      "- The failure was\nreported by the run\n    " + testOutput + "\n",
		"under a bullet's text":  "- Output:\n\n    " + testOutput + "\n",
		"under a deeper bullet":  "- a\n  - Output:\n\n      " + testOutput + "\n",
		"under an ordered item":  "1. Output:\n\n     " + testOutput + "\n",
		"after a blockquote end": "> note\n\ntext\n    " + testOutput + "\n",
	} {
		if cited, _ := graded(doc); cited != 1 {
			t.Errorf("%s: cited %d citations, want 1: the line is prose", name, cited)
		}
	}
}

// A quotation under a nested citation backs it as code in its item: content
// column plus four. Four spaces under a two-column bullet is a paragraph.
func TestAQuotationUnderANestedCitationMustBeCodeInItsItem(t *testing.T) {
	for name, c := range map[string]struct {
		doc     string
		refused bool
	}{
		"nested two, code at content+4":   {"- parent\n  - read " + execCite + ":\n\n        " + execLine + "\n", false},
		"nested four, code at content+4":  {"- parent\n    - read " + execCite + ":\n\n          " + execLine + "\n", false},
		"nested four, wrong code":         {"- parent\n    - read " + execCite + ":\n\n          " + corrupted + "\n", true},
		"top level item, content+4":       {"- read " + execCite + ":\n\n      " + execLine + "\n", false},
		"top level item, only four":       {"- read " + execCite + ":\n\n    " + execLine + "\n", true},
		"nested four, only four past it":  {"- parent\n    - read " + execCite + ":\n\n        " + execLine + "\n", true},
		"fenced block under a nested one": {"- parent\n    - read " + execCite + ":\n\n      ```\n      " + execLine + "\n      ```\n", false},
		"no list, four spaces":            {"Read " + execCite + ":\n\n    " + execLine + "\n", false},
		"no list, wrong code":             {"Read " + execCite + ":\n\n    " + corrupted + "\n", true},
	} {
		if cited, refused := graded(c.doc); cited != 1 || refused != c.refused {
			t.Errorf("%s: cited %d citations, refused = %v, want 1 with refused = %v", name, cited, refused, c.refused)
		}
	}
}

// The inline quotation beside a citation on a four-space nested line backs that
// citation and no other.
func TestAnInlineQuoteBacksACitationOnAFourSpaceNestedLine(t *testing.T) {
	doc := "- parent\n" +
		"    - first, " + execCite + ": `" + execLine + "`\n" +
		"    - second, `pkg/exec.go:13`: `return modelOut, err`\n"
	if f := firstOnly(Check(doc, execFile.read)); len(f) != 0 {
		t.Fatalf("refused a backed citation: %s", f[0].Reason())
	}
	swapped := "- parent\n" +
		"    - first, " + execCite + ": `return modelOut, err`\n" +
		"    - second, `pkg/exec.go:13`: `" + execLine + "`\n"
	if f := firstOnly(Check(swapped, execFile.read)); len(f) != 2 {
		t.Fatalf("findings = %v, want both refused: each quote sits on the other's line", f)
	}
}

// Fences and blockquotes decide before indentation does, in a list as out of one.
func TestFencesAndBlockquotesAreQuotationInsideAList(t *testing.T) {
	for name, doc := range map[string]string{
		"fence in an item":                "- Output:\n\n  ```\n  " + testOutput + "\n  ```\n",
		"fence in a four-space nest":      "- a\n    - Output:\n\n      ```\n      " + testOutput + "\n      ```\n",
		"fence indented four":             "- a\n\n    ```\n    " + testOutput + "\n    ```\n",
		"blockquote in an item":           "- Output:\n  > " + testOutput + "\n",
		"blockquote in a four-space nest": "- a\n    - Output:\n    > " + testOutput + "\n",
		"line continuing a blockquote":    "> note\n    " + testOutput + "\n",
		"lines continuing a blockquote":   "> note\nmore\n    " + testOutput + "\n",
	} {
		if cited, refused := graded(doc); cited != 0 || refused {
			t.Errorf("%s: cited %d citations, refused = %v, want it left ungraded", name, cited, refused)
		}
	}
}

// indentedCode against the CommonMark cases it must get right, by the
// zero-based lines it marks.
func TestIndentedCode(t *testing.T) {
	for name, c := range map[string]struct {
		doc  string
		want []int
	}{
		"after a blank line":                      {"a\n\n    b", []int{2}},
		"a tab":                                   {"a\n\n\tb", []int{2}},
		"spaces then a tab under a bullet":        {"- a\n\n  \tb", nil},
		"spaces then a tab, ordered item":         {"1. a\n\n   \tb", nil},
		"a tab after the marker":                  {"-\ta\n\n        b", []int{2}},
		"a tab after an indented marker":          {"  -\ta\n\n        b", []int{2}},
		"ordered 2 after a nested item":           {"1. a\n   - b\n2. c\n\n       d", []int{4}},
		"a wider marker after a paragraph":        {"9. a\n10. b\n\n       x", nil},
		"three spaces and a tab":                  {"a\n\n   \tb", []int{2}},
		"three spaces":                            {"a\n\n   b", nil},
		"cannot interrupt a paragraph":            {"a\n    b", nil},
		"first line of the document":              {"    a", []int{0}},
		"a run":                                   {"a\n\n    b\n    c\n\n    d", []int{2, 3, 5}},
		"blank line inside it":                    {"    a\n    \n    b", []int{0, 1, 2}},
		"ends at an unindented line":              {"    a\nb\n    c", []int{0}},
		"after a heading":                         {"# h\n    a", []int{1}},
		"after a setext underline":                {"a\n---\n    b", []int{2}},
		"after a setext heading":                  {"a\n===\n    b", []int{2}},
		"after a thematic break":                  {"***\n    a", []int{1}},
		"nested item at two":                      {"- a\n  - b", nil},
		"nested item at four":                     {"- a\n    - b", nil},
		"third level at eight":                    {"- a\n    - b\n        - c", nil},
		"item content plus four":                  {"- a\n\n      b", []int{2}},
		"item content plus three":                 {"- a\n\n     b", nil},
		"four under a bullet":                     {"- a\n\n    b", nil},
		"ordered item content plus four":          {"1. a\n\n       b", []int{2}},
		"ordered item, only four":                 {"1. a\n\n    b", nil},
		"nested item content plus four":           {"- a\n  - b\n\n        c", []int{3}},
		"four-space nest content plus four":       {"- a\n    - b\n\n          c", []int{3}},
		"four-space nest, only eight":             {"- a\n    - b\n\n        c", nil},
		"tab under a bullet":                      {"- a\n\n\tb", nil},
		"two tabs under a bullet":                 {"- a\n\n\t\tb", []int{2}},
		"after the list ends":                     {"- a\n\nb\n\n    c", []int{4}},
		"after the list, indented":                {"- a\n\n    b\n\nc\n\n    d", []int{6}},
		"lazy line then indented":                 {"- a\nb\n    c", nil},
		"empty item then a blank line":            {"-\n\n    a", []int{2}},
		"empty item, text under it":               {"-\n     a", nil},
		"empty item then deep code":               {"-\n        a", []int{1}},
		"a dash under a paragraph":                {"a\n-\n    b", []int{2}},
		"ordered other than 1 does not interrupt": {"a\n2. b\n\n    c", []int{3}},
		"ordered 1 interrupts a paragraph":        {"a\n1. b\n\n    c", nil},
		"blockquote then indented":                {"> a\n    b", []int{1}},
		"blockquote, lazy, then indented":         {"> a\nb\n    c", []int{2}},
		"blockquote, blank, then indented":        {"> a\n\n    b", []int{2}},
		"blockquote then a list marker":           {"> a\n-\n\n    b", []int{3}},
		"a fence closed, then indented":           {"- a\n\n  ```\n  x\n  ```\n\n      b", []int{6}},
		"inside a fence":                          {"```\n    a\n```", nil},
		"thematic break closes the item":          {"- a\n\n***\n\n    b", []int{4}},
		"star item is not a break":                {"* a\n\n      b", []int{2}},
		"crlf":                                    {"- a\r\n    - b\r\n\r\n          c\r\n", []int{3}},
		"item with five-space gap":                {"-     a\n\n      b", []int{2}},
	} {
		code, _ := indentedCode(strings.Split(c.doc, "\n"))
		var got []int
		for i, is := range code {
			if is {
				got = append(got, i)
			}
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %s\nmarked %v, want %v", name, fmt.Sprintf("%q", c.doc), got, c.want)
		}
	}
}

// A fence marker is indented less than four columns past its container, which
// is the margin or the content column of the list item holding the fence. A
// line past that is content of the fence, or code, and never opens or closes
// one. Each document ends in a citation that is backed, so it is admitted when
// the fence before it has closed and left ungraded when it has not.
func TestAFenceMarkerIsLessThanFourColumnsPastItsContainer(t *testing.T) {
	after := "\nRead " + execCite + ": `" + execLine + "`\n"
	for name, c := range map[string]struct {
		doc     string
		cited   int
		refused bool
	}{
		"closer indented three":               {"```\nx\n   ```\n" + after, 1, false},
		"a tab-led marker line is content":    {"```\n\t```\n" + testOutput + "\n```\n" + after, 1, false},
		"a four-space marker line is content": {"```\nx\n    ```\n" + after, 0, false},
		"a four-space marker is code":         {"Output:\n\n    ```\n" + after, 1, false},
		"a tab-led marker is code":            {"Output:\n\n\t```\n" + after, 1, false},
		"fence at an item's content column":   {"- Output:\n\n  ```\n  " + testOutput + "\n  ```\n" + after, 1, false},
		"fence a tab into an item":            {"- Output:\n\n\t```\n\t" + testOutput + "\n\t```\n" + after, 1, false},
		"fence in a four-space nest":          {"- a\n    - Output:\n\n      ```\n      " + testOutput + "\n      ```\n" + after, 1, false},
		"marker four past an item's column":   {"- a\n\n      ```\n" + after, 1, false},
		"unclosed fence swallows the rest":    {"```\nx\n" + after, 0, false},
		"a marker line open in prose":         {"text\n```\n" + testOutput + "\n```\n" + after, 1, false},
		"a true fence still hides output":     {"```\n" + testOutput + "\n```\n" + after, 1, false},
	} {
		if cited, refused := graded(c.doc); cited != c.cited || refused != c.refused {
			t.Errorf("%s: cited %d citations, refused = %v, want %d with refused = %v", name, cited, refused, c.cited, c.refused)
		}
	}
}

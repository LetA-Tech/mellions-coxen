// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package cite

import (
	"strings"
	"testing"
)

// A fence lasts to its closing marker, the end of the document, or the first
// line outside its list item. AFTER stands where a citation is written: graded
// where GitHub renders prose and left alone where it renders code.
func TestAFencedBlockEndsWhereItsListItemEnds(t *testing.T) {
	for name, c := range map[string]struct {
		doc    string
		graded bool
	}{
		// The item ends: <p> or item text after </pre>.
		"item, a blank line, then text":             {"- a\n  ```\n  code\n\nAFTER\n", true},
		"item, then an unindented line, never lazy": {"- a\n  ```\n  code\nAFTER\n", true},
		"nested item at two":                        {"- a\n  - b\n    ```\n    code\n\nAFTER\n", true},
		"nested item at four":                       {"- a\n    - b\n      ```\n      code\n\nAFTER\n", true},
		"nested at two, text in the parent":         {"- a\n  - b\n    ```\n    code\n  AFTER\n", true},
		"nested at four, text in the parent":        {"- a\n    - b\n      ```\n      code\n  AFTER\n", true},
		"three deep, text in the middle one":        {"- a\n  - b\n    - c\n      ```\n      x\n    AFTER\n", true},
		"ordered item":                              {"1. a\n   ```\n   code\n\nAFTER\n", true},
		"ordered item, text a column short":         {"1. a\n   ```\n   code\n  AFTER\n", true},
		"text one column in":                        {"- a\n  ```\n  code\n AFTER\n", true},
		"a tab-indented fence":                      {"- a\n\t```\n\tcode\n\nAFTER\n", true},
		"an item opened empty":                      {"-\n  ```\n  code\n\nAFTER\n", true},
		"the next sibling item":                     {"- a\n  ```\n  code\n- AFTER\n", true},
		"a second item's fence, then text":          {"- a\n  ```\n  x\n\n- b\n  ```\n  y\n\nAFTER\n", true},

		// A marker closes it, and the item goes on.
		"closed, then text after the list":    {"- a\n  ```\n  code\n  ```\n\nAFTER\n", true},
		"closed, then text in the item":       {"- a\n  ```\n  code\n  ```\n  AFTER\n", true},
		"closed, then a lazy line":            {"- a\n  ```\n  code\n  ```\n  para\nAFTER\n", true},
		"a tilde fence closed":                {"- a\n  ~~~\n  code\n  ~~~\n\nAFTER\n", true},
		"a blank line inside, then closed":    {"- a\n  ```\n  code\n\n  more\n  ```\n  AFTER\n", true},
		"a blank line inside, item then ends": {"- a\n  ```\n  code\n\n  more\n\nAFTER\n", true},

		// Nothing ends it.
		"top level, unclosed":                  {"```\ncode\n\nAFTER\n", false},
		"top level, unclosed with an info":     {"```go\ncode\nAFTER\n", false},
		"top level, closed":                    {"```go\ncode\n```\nAFTER\n", true},
		"a longer fence holds a shorter one":   {"````\n```\ncode\n````\nAFTER\n", true},
		"text indented to the item":            {"- a\n  ```\n  code\n  AFTER\n", false},
		"a blank line, then text in the item":  {"- a\n  ```\n  code\n\n  AFTER\n", false},
		"text indented into a nested item":     {"- a\n  - b\n    ```\n    code\n\n    AFTER\n", false},
		"ordered, text at its content column":  {"1. a\n   ```\n   code\n   AFTER\n", false},
		"fence indented past the content":      {"- a\n   ```\n   code\n  AFTER\n", false},
		"a closer four columns past the item":  {"- a\n  ```\n  x\n      ```\n  AFTER\n", false},
		"a tilde fence is not closed by ticks": {"- a\n  ~~~\n  x\n  ```\n  AFTER\n", false},
		"a tilde fence, unclosed":              {"~~~\ncode\nAFTER\n", false},
		"a closer less indented opens another": {"- a\n  ```\n  code\n```\nAFTER\n", false},

		// The info string.
		"backtick info string with a backtick": {"```go `x`\ncode\nAFTER\n", true},
		"a longer run, then a backtick":        {"```` ```\ncode\nAFTER\n", true},
		"tilde info string with a backtick":    {"~~~go `x`\ncode\n~~~\nAFTER\n", true},
		"tilde info string, fence left open":   {"~~~go `x`\ncode\nAFTER\n", false},
		"backtick info string in an item":      {"- a\n  ```go `x`\n  code\n  AFTER\n", true},
	} {
		at := func(quote string) string {
			return strings.Replace(c.doc, "AFTER", "Read "+execCite+": `"+quote+"`", 1)
		}
		wantCited := 0
		if c.graded {
			wantCited = 1
		}
		if cited, refused := graded(at(execLine)); cited != wantCited || refused {
			t.Errorf("%s, true quote: cited %d, refused = %v, want %d admitted", name, cited, refused, wantCited)
		}
		if cited, refused := graded(at(corrupted)); cited != wantCited || refused != c.graded {
			t.Errorf("%s, corrupted quote: cited %d, refused = %v, want %d with refused = %v", name, cited, refused, wantCited, c.graded)
		}
	}
}

// What a fence holds is quotation while it lasts; lines after it ended, or under
// an opener that was no fence, are prose.
func TestWhatAFenceHoldsIsQuotationOnlyWhileItLasts(t *testing.T) {
	after := "\nRead " + execCite + ": `" + execLine + "`\n"
	for name, c := range map[string]struct {
		doc     string
		cited   int
		refused bool
	}{
		"in an item, ended by the item":     {"- Output:\n  ```\n  " + testOutput + "\n\n" + after, 1, false},
		"in a nested item, ended by it":     {"- a\n  - Output:\n    ```\n    " + testOutput + "\n\n" + after, 1, false},
		"in an item, closed":                {"- Output:\n  ```\n  " + testOutput + "\n  ```\n" + after, 1, false},
		"after an item's fence ended":       {"- Output:\n  ```\n  x\n\n" + testOutput + "\n" + after, 2, true},
		"after a closed item fence":         {"- Output:\n  ```\n  x\n  ```\n  " + testOutput + "\n" + after, 2, true},
		"a tilde fence with a backtick":     {"~~~go `x`\n" + testOutput + "\n~~~\n" + after, 1, false},
		"a backtick info string is prose":   {"```go `x`\n" + testOutput + "\n" + after, 2, true},
		"a blockquote after an ended fence": {"- a\n  ```\n  x\n\n> " + testOutput + "\n" + after, 1, false},
		"indented code after an ended one":  {"- a\n  ```\n  x\n\nOutput:\n\n    " + testOutput + "\n" + after, 1, false},
	} {
		if cited, refused := graded(c.doc); cited != c.cited || refused != c.refused {
			t.Errorf("%s: cited %d, refused = %v, want %d with refused = %v", name, cited, refused, c.cited, c.refused)
		}
	}
}

// A fence its item ended is a block of its own: the blockquote after it opens
// another and is not read as more of the fence.
func TestAnEndedFenceDoesNotExtendIntoWhatFollowsIt(t *testing.T) {
	q := quotations("- a\n  ```\n  x\n> y\n")
	if len(q.all) != 2 || len(q.all[0].rest) != 0 || len(q.all[1].texts) != 1 || q.all[1].texts[0] != "y" {
		t.Errorf("blocks = %+v, want the fence holding x and a second block holding y", q.all)
	}
}

// indentedCode reports each line's part in a fence, one letter a line: n none,
// o opener, b body, c closer.
func TestFencedLines(t *testing.T) {
	for name, c := range map[string]struct{ doc, want string }{
		"closed at top level":                 {"```\nx\n```\ny", "obcn"},
		"unclosed at top level":               {"```\nx\n\ny", "obbb"},
		"ended by the item":                   {"- a\n  ```\n  x\n\ny", "nobbn"},
		"ended with no blank line":            {"- a\n  ```\n  x\ny", "nobn"},
		"closed in the item":                  {"- a\n  ```\n  x\n  ```\n  y", "nobcn"},
		"nested, ended by the parent's text":  {"- a\n  - b\n    ```\n    x\n  y", "nnobn"},
		"a closer outside the item opens one": {"- a\n  ```\n  x\n```\ny", "nobob"},
		"a blank line inside stays inside":    {"- a\n  ```\n  x\n\n  y\n  ```", "nobbbc"},
		"an info string with a backtick":      {"```go `x`\ny", "nn"},
		"a tilde info string with a backtick": {"~~~go `x`\ny\n~~~", "obc"},
		"a marker four past the margin":       {"    ```\ny", "nn"},
	} {
		_, fenced := indentedCode(strings.Split(c.doc, "\n"))
		var got strings.Builder
		for _, r := range fenced {
			got.WriteByte("nobc"[r])
		}
		if got.String() != c.want {
			t.Errorf("%s: %q\nroles %s, want %s", name, c.doc, got.String(), c.want)
		}
	}
}

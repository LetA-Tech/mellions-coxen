// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package cite decides whether a document's code citations are backed by the
// code they name.
//
// mellions-deep-research already states the rule and states it correctly — "a
// claim about code is worth its citation: <path>:<line> at the commit you
// verified against ... the quoted lines under it" — and the rule kept failing
// to bind. Five published citations across three consecutive shifts named a
// line that exists and says something else, which a reader cannot distinguish
// from a citation that lands on the code it claims.
//
// Existence is therefore the wrong predicate. Every one of those five
// citations resolved: the file had that many lines. What none of them had was
// the line's text anywhere in the document, because each was written from a
// range read or lifted out of another session's prose rather than opened. So
// the predicate here is that the document carries the line, not that the file
// has one:
//
//   - Missing — the file has no such line, so the citation points at nothing.
//   - Unbacked — the line exists, says something, and the document exhibits no
//     quotation of it that belongs to this citation.
//
// Belonging is the whole of it, and a document-global match does not have it.
// A pasted range carries the text of every line in it, so a citation to a line
// inside one is backed by a document that shows the reader no way to tell which
// of those lines it names — which is the range read #149 is about, passing the
// check built for it. One "}" written once backs every citation to a closing
// brace for the same reason. Having the window is not having opened the line.
//
// So a backing quotation has to be anchored to the citation and exclusive to
// it. Anchored: on the citation's own line — an inline span, or the text after
// `path:N:` in pasted `grep -n` output — or at the head of the quoted block the
// citation introduces, with nothing but blank lines between. Exclusive: a
// quotation backs one citation, so two citations to lines reading alike need
// two quotations.
//
// The head of the block, not anywhere in it: a citation names one line, and the
// block under it starts there. That is what makes the claim checkable by the
// reader rather than only by this package — "the quoted lines under it" with
// the named line first.
//
// That is the existing rule made mechanical rather than a new demand on
// authors: a citation whose line is quoted under it passes, which is the form
// the Skill already asks for.
//
// What is deliberately not a citation, because a checker that denies on noise
// gets disabled: a token whose path does not resolve to a file in the tree
// (a URL's host, an IP and port, a clock time, `issues/656#issuecomment-…`,
// another repository's path), and a line range, which is honest about being a
// region rather than a line.
package cite

import (
	"regexp"
	"strconv"
	"strings"
)

// Citation is one path:line reference a document makes.
type Citation struct {
	// Raw is the citation as written, for pointing the author at it.
	Raw string
	// Path is what precedes the colon, repository-relative.
	Path string
	// Line is the line number claimed.
	Line int
	// At is the document line the citation is written on, which is what
	// anchors a quotation to it.
	At int
}

// Kind is why a citation does not hold.
type Kind int

const (
	// Missing: the file has fewer lines than the citation claims.
	Missing Kind = iota
	// Unbacked: the line exists and the document quotes nothing equal to it.
	Unbacked
)

// Finding is a citation the document cannot back, with what the line says.
type Finding struct {
	Citation
	Kind Kind
	// Actual is the line as it really is, empty where there is no such line.
	Actual string
}

// Reason states the finding in the terms the author has to act on.
func (f Finding) Reason() string {
	switch f.Kind {
	case Missing:
		return f.Raw + ": no such line — the file is shorter than that."
	default:
		if strings.TrimSpace(f.Actual) == "" {
			return f.Raw + ": that line is blank. Nothing there is the claim."
		}
		return f.Raw + ": that line says " + strconv.Quote(strings.TrimSpace(f.Actual)) +
			", and the body quotes no line equal to it."
	}
}

// citation is a path:line token. The path must carry a dot-extension or a
// slash, which is what separates a file from a clock time or a bare number;
// everything past that is decided by whether the path resolves to a file.
// The range arm accepts an en or em dash as well as a hyphen: prose in this
// estate is written with them, and a range read as a bare line citation is
// judged against a line the author never claimed.
var citation = regexp.MustCompile(`(^|[^\w/.:-])((?:[\w.+-]*/)*[\w+-]+\.[\w+-]+|(?:[\w.+-]+/)+[\w.+-]+):(\d+)([-–—]\d+)?`)

// Extract returns every citation a document makes, in the order written.
//
// Two things that look like citations are not. A line range names a region,
// and this package can say nothing about whether the author read any
// particular line of one. And a path:line inside a fenced block or a
// blockquote is quotation rather than claim — a `go test` failure or a
// `go vet` line pasted as evidence carries a real file and a real number that
// the author is reporting, not citing, and denying on those would deny the
// bodies that quote their evidence most carefully. An inline code span is
// scanned, because a backticked `path.go:64` is how most citations are
// written.
func Extract(doc string) []Citation {
	var out []Citation
	seen := map[string]bool{}
	for _, c := range occurrences(doc) {
		if seen[c.Raw] {
			continue
		}
		seen[c.Raw] = true
		out = append(out, c)
	}
	return out
}

// occurrences is every citation token as written, each carrying the document
// line it sits on. The same citation written twice is two occurrences here
// where Extract reports one: backing is anchored, so which of the two places
// it was written in decides whether the document backs it.
func occurrences(doc string) []Citation {
	var out []Citation
	claimed := prose(doc)
	for _, m := range citation.FindAllStringSubmatchIndex(claimed, -1) {
		// A trailing -N makes this a range.
		if m[8] >= 0 {
			continue
		}
		path := claimed[m[4]:m[5]]
		n, err := strconv.Atoi(claimed[m[6]:m[7]])
		if err != nil || n < 1 {
			continue
		}
		out = append(out, Citation{
			Raw:  path + ":" + strconv.Itoa(n),
			Path: path,
			Line: n,
			// From the path, not from the match, whose first group eats the
			// newline before a citation that opens a line.
			At: strings.Count(claimed[:m[4]], "\n"),
		})
	}
	return out
}

// Check reports the citations a document cannot back. read answers with a
// file's lines, or an error where the path is not a file in the tree — which
// is not a finding but the bound on what counts as a citation at all, since a
// path this tree does not have is a URL host, another repository, or prose.
func Check(doc string, read func(path string) ([]string, error)) []Finding {
	q := quotations(doc)
	backed := map[string]bool{}
	first := map[string]Finding{}
	var order []string
	note := func(f Finding) {
		if _, seen := first[f.Raw]; !seen {
			first[f.Raw] = f
		}
	}
	for _, c := range occurrences(doc) {
		if backed[c.Raw] {
			continue
		}
		lines, err := read(c.Path)
		if err != nil {
			continue
		}
		if _, seen := first[c.Raw]; !seen {
			order = append(order, c.Raw)
		}
		if c.Line > len(lines) {
			note(Finding{Citation: c, Kind: Missing})
			continue
		}
		actual := lines[c.Line-1]
		if q.backs(c, normalize(actual)) {
			backed[c.Raw] = true
			continue
		}
		note(Finding{Citation: c, Kind: Unbacked, Actual: actual})
	}
	var out []Finding
	for _, raw := range order {
		if backed[raw] {
			continue
		}
		out = append(out, first[raw])
	}
	return out
}

// prose is the document with everything it quotes rather than states removed,
// line structure kept so the remainder still reads as the author's own text at
// the line it was written on. A line is prose or quotation, never both, so a
// path:line pasted as evidence is never read as a claim. The converse does not
// hold: a prose line carries inline spans, which are quotation sitting on a
// claim's line, and that is exactly where a citation's own backing usually is.
func prose(doc string) string {
	var out strings.Builder
	fence := ""
	for i, line := range strings.Split(doc, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		trimmed := strings.TrimLeft(line, " ")
		closer := strings.TrimRight(trimmed, " \t\r")
		switch {
		case fence != "":
			if len(closer) >= len(fence) && strings.TrimLeft(closer, fence[:1]) == "" {
				fence = ""
			}
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fence = trimmed[:runLen(trimmed)]
		case strings.HasPrefix(trimmed, ">"):
			// a blockquote: what it holds is quotation
		case strings.HasPrefix(line, "    "), strings.HasPrefix(line, "\t"):
			// an indented code block
		default:
			out.WriteString(line)
		}
	}
	return out.String()
}

// quoted is one place a document exhibits a line's text, and the place is half
// of what it is: text alone is a document-global match, which backs a citation
// the author never opened.
type quoted struct {
	// at is the document line an inline quotation sits on, or the line a
	// block opens on.
	at int
	// texts are the normalized readings of the quotation. A block reads as
	// its first non-blank line, and a `grep -n` line reads both as itself
	// and as the text after its path:N: prefix, since either may be what
	// the file's line holds.
	texts []string
	// block is true where this is a quoted block rather than a span on a
	// prose line, which decides how a citation reaches it.
	block bool
	used  bool
}

// quotedText is every quotation a document makes, in document order, each
// keeping the line it sits on so a citation can only reach the ones anchored to
// it: the spans on its own line, and the head of the block it introduces.
type quotedText struct {
	lines []string
	all   []quoted
}

func quotations(doc string) *quotedText {
	q := &quotedText{lines: strings.Split(doc, "\n")}
	fence := ""
	open := -1
	// head records a block's first non-blank line and nothing after it: a
	// citation names one line, and the block under it starts there.
	head := func(s string) {
		if open < 0 || len(q.all[open].texts) > 0 {
			return
		}
		q.all[open].texts = readings(s)
	}
	start := func(i int) {
		q.all = append(q.all, quoted{at: i, block: true})
		open = len(q.all) - 1
	}
	for i, line := range q.lines {
		trimmed := strings.TrimLeft(line, " ")
		closer := strings.TrimRight(trimmed, " \t\r")
		switch {
		case fence != "":
			if len(closer) >= len(fence) && strings.TrimLeft(closer, fence[:1]) == "" {
				fence, open = "", -1
				continue
			}
			head(line)
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fence = trimmed[:runLen(trimmed)]
			start(i)
		case strings.HasPrefix(trimmed, ">"):
			if open < 0 {
				start(i)
			}
			head(strings.TrimPrefix(trimmed, ">"))
		case strings.HasPrefix(line, "    "), strings.HasPrefix(line, "\t"):
			// an indented code block
			if open < 0 {
				start(i)
			}
			head(line)
		default:
			open = -1
			// `grep -n` output pasted into prose without a fence carries the
			// line after the citation on the same line. It is the most exact
			// evidence an author can offer, and reading only the spans on this
			// line rejected it — and said the body quoted nothing equal to a
			// line it quoted verbatim.
			if _, text, ok := grepLine(line); ok {
				if k := normalize(text); k != "" {
					q.all = append(q.all, quoted{at: i, texts: []string{k}})
				}
			}
			for _, s := range spans(line) {
				if k := normalize(s); k != "" {
					q.all = append(q.all, quoted{at: i, texts: []string{k}})
				}
			}
		}
	}
	return q
}

// readings is the normalized forms a quoted line may be read in.
func readings(s string) []string {
	var out []string
	if k := normalize(s); k != "" {
		out = append(out, k)
	}
	if _, text, ok := grepLine(s); ok {
		if k := normalize(text); k != "" && (len(out) == 0 || k != out[0]) {
			out = append(out, k)
		}
	}
	return out
}

// backs answers whether the document quotes want for this citation and takes
// the quotation if it does. A quotation is spent once: two citations to lines
// that read alike need two of them, so one "}" cannot back every citation to a
// closing brace.
func (q *quotedText) backs(c Citation, want string) bool {
	if want == "" || want == c.Raw {
		// A blank line can never be quoted, and a citation quoted as a span
		// is the citation, not its line.
		return false
	}
	for i := range q.all {
		s := &q.all[i]
		if s.used || s.block || s.at != c.At {
			continue
		}
		if s.texts[0] == want {
			s.used = true
			return true
		}
	}
	if b := q.introduced(c.At); b != nil && !b.used {
		for _, t := range b.texts {
			if t == want {
				b.used = true
				return true
			}
		}
	}
	return false
}

// introduced is the quoted block the citation's own paragraph introduces: the
// next block down the document, reached across the rest of the sentence the
// citation is written in and the blank line under it, but not across a second
// paragraph. Prose wraps, and a citation that lands at the end of a line is
// still introducing the block below its paragraph; prose that has moved on to
// something else is not.
func (q *quotedText) introduced(n int) *quoted {
	for i := range q.all {
		b := &q.all[i]
		if !b.block || b.at <= n {
			continue
		}
		ended := false
		for j := n + 1; j < b.at && j < len(q.lines); j++ {
			if strings.TrimSpace(q.lines[j]) == "" {
				ended = true
				continue
			}
			if ended {
				// a paragraph of its own, between the two
				return nil
			}
		}
		return b
	}
	return nil
}

// runLen counts the fence's delimiter run.
func runLen(s string) int {
	n := 0
	for n < len(s) && s[n] == s[0] {
		n++
	}
	return n
}

// spans returns the text of every inline code span on a line. A citation is
// commonly written `path.go:64` with the line quoted beside it in the same
// style, and a span is where that lands.
func spans(line string) []string {
	var out []string
	for {
		i := strings.IndexByte(line, '`')
		if i < 0 {
			return out
		}
		n := runLen(line[i:])
		delim := line[i : i+n]
		rest := line[i+n:]
		j := strings.Index(rest, delim)
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		line = rest[j+n:]
	}
}

// grep is one line of `grep -n` output: a path, a line number, and the line's
// own text after the second colon.
var grep = regexp.MustCompile(`^\s*((?:[\w.+-]*/)*[\w+-]+\.[\w+-]+|(?:[\w.+-]+/)+[\w.+-]+):(\d+):(.*)$`)

// grepLine reads a line as `grep -n` output and returns the citation it makes
// and the text it quotes for it.
func grepLine(line string) (raw, text string, ok bool) {
	m := grep.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1] + ":" + m[2], m[3], true
}

// numbered strips a line-number prefix a paste carries: `grep -n` and
// `sed -n '82p'` output, and the `82\t` and `82 |` forms a reader writes by
// hand. The text after it is what the file's line actually holds.
var numbered = regexp.MustCompile(`^\s*\d+\s*[:|\t]\s?`)

// normalize is the form two texts are compared in: the line's own content,
// free of the indentation a fenced block or a quote changes and of a line
// number a paste carried in. Whitespace inside is collapsed, so a re-wrapped
// quote of the same line still matches it.
func normalize(s string) string {
	s = strings.TrimRight(s, "\r")
	s = numbered.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

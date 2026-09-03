// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package prbody

import (
	"regexp"
	"strings"
)

// span stands in for a code span that has been removed. It is one word wide,
// so a span standing between a negator and a keyword keeps the distance it
// occupied — removing it outright would manufacture an adjacency the author
// did not write.
const span = "￼"

// closing is GitHub's own grammar for a closing reference: one of nine
// keywords, an optional colon, whitespace that may carry a single line break,
// and an issue reference by number, by owner/repo#number, or by URL. Nothing
// else closes an issue — "closing" and "fixing" are not keywords — and the
// line break is what a soft-wrapped paragraph puts between a keyword and its
// number.
var closing = regexp.MustCompile(`(?i)\b(close[sd]?|fix(es|ed)?|resolve[sd]?)\b:?[ \t]*\r?\n?[ \t]*` +
	`(?P<ref>#[0-9]+|[\w.-]+/[\w.-]+#[0-9]+|https?://github\.com/[\w.-]+/[\w.-]+/issues/[0-9]+)`)

var refGroup = closing.SubexpIndex("ref")

// Declared returns the first closing declaration a pull request body actually
// makes: a closing reference in the body's own prose, which no negator in its
// clause denies.
func Declared(body string) (Close, bool) {
	s := strip(body)
	for _, m := range closing.FindAllStringSubmatchIndex(s, -1) {
		if negated(clause(s, m[0])) {
			continue
		}
		return Close{
			Text: strings.Join(strings.Fields(s[m[0]:m[1]]), " "),
			Ref:  s[m[2*refGroup]:m[2*refGroup+1]],
		}, true
	}
	return Close{}, false
}

// strip leaves the body's prose and removes what quotes rather than states:
// fenced code blocks, blockquotes and inline code spans of any delimiter
// width. A keyword inside one of those is a quotation of this rule and not a
// use of it, which is how the rule itself gets written down.
//
// Emphasis markers go last, so a keyword an author bolded away from its number
// still reads as one reference and a negator wearing them still reads as one
// word.
func strip(body string) string {
	var out strings.Builder
	fence := ""
	for i, line := range strings.Split(body, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) > 3 {
			trimmed = ""
		}
		closer := strings.TrimRight(trimmed, " \t\r")
		switch {
		case fence != "":
			if len(closer) >= len(fence) && strings.TrimLeft(closer, fence[:1]) == "" {
				fence = ""
			}
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			fence = trimmed[:runLen(trimmed, 0)]
		case strings.HasPrefix(trimmed, ">"):
			// a blockquote line, kept as a line and not as prose
		default:
			out.WriteString(line)
		}
	}
	return strings.NewReplacer("*", "", "_", "").Replace(spans(out.String()))
}

// spans replaces every inline code span with one placeholder word. A span
// opens on a run of backticks and closes on the next run of exactly the same
// length, which is what makes a doubled-backtick span able to quote a string
// that itself contains a code span. A run that never closes is literal text.
func spans(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '`' {
			out.WriteByte(s[i])
			i++
			continue
		}
		n := runLen(s, i)
		j, closed := i+n, false
		for j < len(s) {
			if s[j] != '`' {
				j++
				continue
			}
			m := runLen(s, j)
			if m == n {
				closed = true
				break
			}
			j += m
		}
		if !closed {
			out.WriteString(s[i : i+n])
			i += n
			continue
		}
		out.WriteString(span)
		i = j + n
	}
	return out.String()
}

func runLen(s string, i int) int {
	c, n := s[i], 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// clause is the text standing before a keyword within its own clause. Sentence
// punctuation, a semicolon, a colon, a comma, a preceding issue reference and a
// line break all end one, so a negation in one clause cannot excuse a close
// declared in the next.
func clause(s string, keyword int) string {
	if i := strings.LastIndexAny(s[:keyword], ".!?;:,#\n"); i >= 0 {
		return s[i+1 : keyword]
	}
	return s[:keyword]
}

var (
	word        = regexp.MustCompile(`[\p{L}\p{N}'’` + span + `-]+`)
	contraction = regexp.MustCompile(`^\p{L}+n['’]t$`)

	// An auxiliary in front of a negator says the verb is what is denied, so
	// the keyword after it is denied however many words stand between.
	auxiliary = words("do does did will would can could shall should is are was were has have had must may might")
	// A bare negator opens a noun phrase and governs the noun, so only one
	// word may stand between it and the keyword.
	bare = words("no not never nothing neither nor without")
	// A coordinating conjunction opens a new clause the way a full stop does,
	// and a negation does not reach past one.
	conjunction = words("and or but")
)

// verbGap bounds the auxiliary arm in words. The widest gap an established
// negation needs is three — "doesn't on this base close #7" — and the bound is
// a bound rather than a proof: a verb negation that drifts five words from its
// keyword is read as a declaration, which is the safe direction for a guard.
const verbGap = 4

// nounGap bounds the bare-negator arm. One word is exactly what "no merge
// closes #7" needs, and one less than "No doubt this closes #7", which is a
// declaration. Distance does not settle which of those two a sentence is, so
// this is a bound and not a proof: "not only closes #7" is still read as a
// denial.
const nounGap = 1

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// negated reports whether a negator in the clause governs the keyword that
// closes it.
func negated(clause string) bool {
	ws := word.FindAllString(strings.ToLower(clause), -1)
	for i, w := range ws {
		gap := ws[i+1:]
		verb := w == "cannot" || contraction.MatchString(w) ||
			((w == "not" || w == "never") && i > 0 && auxiliary[ws[i-1]])
		if verb && len(gap) <= verbGap && !holds(gap, conjunction) {
			return true
		}
		if bare[w] && len(gap) <= nounGap {
			return true
		}
	}
	return false
}

func holds(ws []string, in map[string]bool) bool {
	for _, w := range ws {
		if in[w] {
			return true
		}
	}
	return false
}

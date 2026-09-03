// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"regexp"
	"strconv"
	"strings"
)

// CompletionClaim is a handoff's assertion that the work is finished, and the
// words it made it in.
type CompletionClaim struct {
	// Phrase is what the body said, quoted back so a challenge names the
	// sentence rather than the judgement.
	Phrase string
}

// completionPhrases are the ways a handoff asserts the work itself is done.
//
// The set is deliberately narrow. A handoff that reports finishing a step —
// "member two implemented", "the migration is complete" — is not claiming the
// unit is closed, and a challenge that fired on those would be answered by
// habit within a week and then be worth nothing. What is here is the shape of
// the claim that failed in production: the work, whole, asserted finished.
// "no outstanding work" and "no further work" were here and are not any more.
// Across the 173 handoffs on this host they fired only on foreign artefacts — a
// sibling repository's issue, a pull request waiting on somebody else's merge —
// and never on a lane asserting its own obligation set closed. Precision is what
// a gate is worth: both handoffs that actually failed still trip the phrases
// that remain, so dropping this one costs no recall on the cases it exists for.
var completionPhrases = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(implementation|work|unit|lane|scope)\b[^.\n]{0,40}\bcomplete\b`),
	regexp.MustCompile(`(?i)\bcomplete\b[^.\n]{0,30}\b(implementation|work unit|lane)\b`),
	regexp.MustCompile(`(?i)\bfully (implemented|remediated|delivered|closed)\b`),
	// "nothing further to do" is a completion claim; "nothing further is
	// possible" on a blocked lane is the opposite of one, and the loose form of
	// this pattern challenged exactly that. The object has to be the work.
	regexp.MustCompile(`(?i)\bnothing (remains|is left|further)\b[^.\n]{0,20}\b(to do|to build|to implement|owed|outstanding)\b`),
	regexp.MustCompile(`(?i)\bnothing (remains|is left)\s*[.;]`),
	regexp.MustCompile(`(?i)\bevery (requirement|obligation|item)\b[^.\n]{0,30}\b(met|closed|satisfied|discharged)\b`),
	regexp.MustCompile(`(?i)\b100\s?%\s?(complete|done|implemented)\b`),
	regexp.MustCompile(`(?i)\bready (to|for) (close|closure|verify|verification|accept)\b`),
}

// reconciledInBody are the ways a body already answers the challenge, so a
// handoff that did the work in prose is not asked to repeat it as a flag.
//
// Each names an act over a closed set rather than a feeling of completeness:
// what was enumerated, and against what.
var reconciledInBody = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\benumerat(ed|ing|ion)\b`),
	regexp.MustCompile(`(?i)\breconcil(ed|ing|iation)\b[^.\n]{0,60}\b(against|with|to)\b`),
	regexp.MustCompile(`(?i)\bevery\b[^.\n]{0,40}\b(in|of) (the )?(closed set|enumeration|register|manifest|inventory)\b`),
	regexp.MustCompile(`(?i)\bchecked (each|every)\b[^.\n]{0,40}\bagainst\b`),
}

// qualified are the marks that the sentence carrying a completion word is not
// this lane claiming its own work finished.
//
// Measured against every handoff on this host rather than imagined: of the five
// that tripped the completion words, four were of these shapes — a foreign
// artefact's completion reported as news, a readiness conditional on somebody
// else's merge, a closure explicitly blocked, and a hedge. A gate wrong four
// times out of five is answered by habit inside a week, and then the one time
// it is right is answered by habit too.
var qualified = []*regexp.Regexp{
	// A numbered artefact in the same sentence: the claim is about that issue
	// or pull request, not about this lane's obligation set.
	regexp.MustCompile(`#\d+`),
	// Conditional on something that has not happened.
	regexp.MustCompile(`(?i)(when|once|after|if|pending|awaiting|blocked|subject to)`),
	// Hedged, which is the opposite of the flat assertion this challenges.
	regexp.MustCompile(`(?i)(that I know of|as far as|believe|appears?|seems?|should be|probably)`),
	// Somebody else's to decide, so not a completion this lane is asserting.
	regexp.MustCompile(`(?i)(owner's|reviewer's|theirs to|not (mine|ours) to)`),
}

// sentenceAround returns the sentence a match sits in, which is the unit a
// qualification applies to. A body-wide search would let one blocked sentence
// excuse a completion claim three paragraphs away.
func sentenceAround(body string, at int) string {
	start := strings.LastIndexAny(body[:at], ".;\n")
	if start < 0 {
		start = 0
	} else {
		start++
	}
	end := strings.IndexAny(body[at:], ".;\n")
	if end < 0 {
		end = len(body)
	} else {
		end += at
	}
	return strings.TrimSpace(body[start:end])
}

// Claims reports the completion claim a handoff body makes, if any.
//
// It is a reading of what the body says, not a judgement about whether the work
// is done: only the engineer can decide that, and the point of the challenge is
// to make the deciding happen rather than to decide it here.
func Claims(body string) (CompletionClaim, bool) {
	for _, re := range completionPhrases {
		for _, loc := range re.FindAllStringIndex(body, -1) {
			sentence := sentenceAround(body, loc[0])
			if isQualified(sentence) {
				continue
			}
			return CompletionClaim{Phrase: strings.TrimSpace(body[loc[0]:loc[1]])}, true
		}
	}
	return CompletionClaim{}, false
}

// isQualified reports that a sentence carrying a completion word is not this
// lane asserting its own work finished.
func isQualified(sentence string) bool {
	for _, re := range qualified {
		if re.MatchString(sentence) {
			return true
		}
	}
	return false
}

// Reconciled reports that the body itself already says what closed set the
// completion was checked against.
func Reconciled(body string) bool {
	for _, re := range reconciledInBody {
		if re.MatchString(body) {
			return true
		}
	}
	return false
}

// ChallengeCompletion returns the question a handoff has to answer before it is
// stored, or empty where there is nothing to ask.
//
// The failure it exists for: a work unit handed off as complete twice, where
// completion had been read off the dispatch record's numbered evidence list
// rather than derived from the approved obligation set. Both handoffs were
// truthful about what had been built and wrong about what was owed, and nothing
// between the engineer and the record asked which closed set had been checked.
//
// It asks exactly that, once, and one sentence answers it. Where the body has
// already answered it, or claims nothing, it asks nothing.
func ChallengeCompletion(body, reconciled, residual string) string {
	if strings.TrimSpace(reconciled) != "" || strings.TrimSpace(residual) != "" {
		return ""
	}
	claim, ok := Claims(body)
	if !ok || Reconciled(body) {
		return ""
	}
	var b strings.Builder
	b.WriteString("This handoff claims the work is finished — it says ")
	b.WriteString(strconv.Quote(claim.Phrase))
	b.WriteString(".\n\n")
	b.WriteString("Which closed set did you enumerate to establish that, and where did you read it?\n")
	b.WriteString("Not the evidence list you produced — the obligation set the approved authority\n")
	b.WriteString("states, checked member by member. A lane that reads completion off its own\n")
	b.WriteString("output is complete against its own output.\n\n")
	b.WriteString("  mellions assign handoff <id> -file <f> \\\n")
	b.WriteString("    -reconciled \"<the closed set, and where it is stated>\"\n\n")
	b.WriteString("If you cannot name one, that is the answer, and it is not a handoff:\n\n")
	b.WriteString("  mellions assign handoff <id> -file <f> -residual \"<what is still owed>\"\n")
	return b.String()
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment_test

import (
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// The two handoffs that failed in production, quoted from the record so the
// challenge is tested against the sentences it exists for rather than against
// sentences written to pass it.
const (
	firstFailedHandoff = `# Handoff — IMP-017 custody, recovery, runtime operations and observability

IMP-017 IMPLEMENTATION COMPLETE — 100%. Every requirement in the dispatch's
numbered evidence list is met, all 27 confirmed adversarial findings are
remediated, and 17 falsification arms were run.`

	secondFailedHandoff = `The lane is fully implemented against DISP-017's required completion evidence.
Eighteen records retained, every one PASS. No remaining work in this lane.`
)

// TestTheHandoffsThatFailedAreChallenged is the regression: both bodies were
// truthful about what had been built and wrong about what was owed, and nothing
// asked which closed set completion had been checked against.
func TestTheHandoffsThatFailedAreChallenged(t *testing.T) {
	for name, body := range map[string]string{
		"the first":  firstFailedHandoff,
		"the second": secondFailedHandoff,
	} {
		t.Run(name, func(t *testing.T) {
			challenge := assignment.ChallengeCompletion(body, "", "")
			if challenge == "" {
				t.Fatal("a handoff claiming the work is finished was accepted without being asked what it enumerated")
			}
			if !strings.Contains(challenge, "closed set") {
				t.Fatalf("the challenge does not ask the question: %q", challenge)
			}
			claim, ok := assignment.Claims(body)
			if !ok || claim.Phrase == "" {
				t.Fatal("the claim is not quoted back, so the challenge names a judgement rather than a sentence")
			}
			if !strings.Contains(challenge, claim.Phrase) {
				t.Fatalf("the challenge does not quote what the body said: %q", challenge)
			}
		})
	}
}

// TestOrdinaryHandoffsPassUntouched is the false-positive arm, and it is the
// one that decides whether this is worth having: a gate answered by habit is a
// gate worth nothing. Every body here reports real work honestly and claims no
// completion of the unit.
func TestOrdinaryHandoffsPassUntouched(t *testing.T) {
	for name, body := range map[string]string{
		"a member finished, the unit not":    "Member two is implemented and the migration is complete. Three members remain.",
		"a partial slice":                    "PR-IMP-017-3 lands the readiness enum. The drain lifecycle is next.",
		"blocked":                            "Blocked on SPEC-ESC-002. Nothing further is possible in this lane until it is resolved.",
		"downstream work named as another's": "FO-013 closes at IMP-021 and PE-012 at IMP-018. Neither is this unit's to close.",
		"a residual stated plainly":          "Two residuals stand: the Qdrant drill histories and the PE-012 half.",
		"a merged pull request":              "Merged at dev@c8f57a3. make verify exit 0, 4,493 passing.",
		"an abandoned lane":                  "Setting this down; the premise moved when IMP-016 landed.",
	} {
		t.Run(name, func(t *testing.T) {
			if c := assignment.ChallengeCompletion(body, "", ""); c != "" {
				t.Fatalf("an ordinary handoff was challenged:\nbody: %q\nchallenge: %s", body, c)
			}
		})
	}
}

// TestAnAnsweredClaimIsStored holds the other half: the challenge is satisfied
// by one sentence, so an engineer who did the work is not made to repeat it.
func TestAnAnsweredClaimIsStored(t *testing.T) {
	answers := map[string]struct{ reconciled, residual string }{
		"reconciled by flag": {reconciled: "the card's eleven drill categories against ops.IDs(), member by member", residual: ""},
		"residual by flag":   {reconciled: "", residual: "the Qdrant rebuild histories are still owed"},
	}
	for name, a := range answers {
		t.Run(name, func(t *testing.T) {
			if c := assignment.ChallengeCompletion(firstFailedHandoff, a.reconciled, a.residual); c != "" {
				t.Fatalf("an answered claim was still challenged: %s", c)
			}
		})
	}
}

// TestABodyThatAlreadyAnswersIsNotAskedAgain keeps the ceremony down: a handoff
// that states its enumeration in prose has done the thing the flag exists to
// force, and asking for it again is the bureaucracy this must not become.
func TestABodyThatAlreadyAnswersIsNotAskedAgain(t *testing.T) {
	body := `The implementation is complete. Every one of the seventeen operator procedures
in ops.IDs() was enumerated and checked against the card's completion paragraph;
sixteen carry an executed drill and REGIONAL_RECOVERY carries its citation.`
	if c := assignment.ChallengeCompletion(body, "", ""); c != "" {
		t.Fatalf("a body that named what it enumerated was challenged anyway: %s", c)
	}
}

// TestTheChallengeNamesBothWaysOut so a session meeting it for the first time
// is not left guessing which flag it wants.
func TestTheChallengeNamesBothWaysOut(t *testing.T) {
	c := assignment.ChallengeCompletion(firstFailedHandoff, "", "")
	for _, want := range []string{"-reconciled", "-residual"} {
		if !strings.Contains(c, want) {
			t.Errorf("the challenge does not name %s: %s", want, c)
		}
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package issuegate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/checkout"
)

// tree builds a two-repository work root: the one being worked, and a consumer
// whose code an issue might claim things about. The shape is the one that
// produced the failure this package exists for.
func tree(t *testing.T) (root string, known map[string]string) {
	t.Helper()
	root = t.TempDir()

	write := func(repo, path, body string) {
		abs := filepath.Join(root, repo, path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		// A citation resolves against a checkout, so the fixture is one.
		if err := os.MkdirAll(filepath.Join(root, repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write("analytics-service", "internal/adapters/postgres/gate_test.go",
		strings.Repeat("// filler\n", 60)+
			"\tcashCliff := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)\n"+
			"\tif row.CashCliffDate != nil && !row.CashCliffDate.After(now.AddDate(0, 0, 14)) {\n"+
			"\t\treturn \"CASH_CLIFF_WITHIN_14_DAYS\"\n"+
			"\t}\n")

	// The consumer the model claims fidelity to, and does not have.
	write("goals-service", "internal/app/command/funding_advisory.go",
		"// 3. BUFFER_BELOW_FLOOR — only fires for protected goals.\n"+
			"\tif protectionRule == goal.ProtectionRuleProtected {\n"+
			"\t\toutcome := evaluateBufferCoverageVerdict(verdict)\n"+
			"\t}\n")

	known, err := checkout.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, known
}

func rules(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	return out
}

func has(fs []Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// The rule that matters: a body that builds its case on another repository's
// behaviour, cites nothing inside it, and is wrong.
func TestAnUncitedCrossRepoClaimIsRefused(t *testing.T) {
	_, known := tree(t)

	body := "## Root cause\n" +
		"`internal/adapters/postgres/gate_test.go:61` seeds a fixed date.\n\n" +
		"```go\n" +
		"\tcashCliff := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)\n" +
		"```\n\n" +
		"The model is a faithful reproduction of the goals-service funding advisory,\n" +
		"so the fixture is the only thing wrong.\n"

	f := Check(body, "analytics-service", known)
	if !has(f, RuleUncitedRepo) {
		t.Fatalf("uncited cross-repo claim passed the gate; findings = %v", rules(f))
	}
	for _, x := range f {
		if x.Rule == RuleUncitedRepo && !strings.Contains(x.Detail, "goals-service") {
			t.Errorf("finding does not name the repository to read: %s", x.Detail)
		}
	}
}

// And the other half of the falsification: citing it must clear the rule, or
// the gate is refusing everything and proving nothing.
func TestCitingTheOtherRepositoryClearsTheRule(t *testing.T) {
	_, known := tree(t)

	body := "## Root cause\n" +
		"`internal/adapters/postgres/gate_test.go:61` seeds a fixed date.\n\n" +
		"The model claims fidelity to goals-service. It does not have it —\n" +
		"`goals-service/internal/app/command/funding_advisory.go:2` gates on the\n" +
		"protection rule:\n\n" +
		"```go\n" +
		"\tif protectionRule == goal.ProtectionRuleProtected {\n" +
		"```\n"

	if f := Check(body, "analytics-service", known); len(f) != 0 {
		t.Fatalf("a properly cited body was refused: %v", f)
	}
}

func TestACitationThatDoesNotResolveIsRefused(t *testing.T) {
	_, known := tree(t)

	for name, body := range map[string]string{
		"no such file": "see `internal/adapters/postgres/nothing_here.go:12`\n",
		"past the end": "see `internal/adapters/postgres/gate_test.go:99999`\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := Check(body, "analytics-service", known)
			if !has(f, RuleMissingFile) {
				t.Errorf("citation accepted; findings = %v", rules(f))
			}
		})
	}
}

// A quote is the strongest-reading claim in an issue. It has to be real.
func TestAQuoteNotInTheCitedFileIsRefused(t *testing.T) {
	_, known := tree(t)

	body := "`internal/adapters/postgres/gate_test.go:61` reads:\n\n" +
		"```go\n" +
		"\tcashCliff := time.Date(2027, time.January, 9, 0, 0, 0, 0, time.UTC)\n" +
		"```\n"

	f := Check(body, "analytics-service", known)
	if !has(f, RuleQuoteMismatch) {
		t.Fatalf("a fabricated quote passed; findings = %v", rules(f))
	}
}

func TestAnAccurateQuoteWithElisionPasses(t *testing.T) {
	_, known := tree(t)

	body := "`internal/adapters/postgres/gate_test.go:61`:\n\n" +
		"```go\n" +
		"\tcashCliff := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)\n" +
		"\t// ...\n" +
		"\t\treturn \"CASH_CLIFF_WITHIN_14_DAYS\"\n" +
		"```\n"

	if f := Check(body, "analytics-service", known); len(f) != 0 {
		t.Fatalf("an accurate quote was refused: %v", f)
	}
}

// Output blocks are not source and must not be matched against a file, or
// every issue carrying a test run gets refused.
func TestOutputBlocksAreNotCheckedAgainstSource(t *testing.T) {
	_, known := tree(t)

	body := "`internal/adapters/postgres/gate_test.go:61` fails:\n\n" +
		"```\n" +
		"--- FAIL: TestThing (0.02s)\n" +
		"    gate_test.go:436: reason = \"CASH_CLIFF_WITHIN_14_DAYS\"\n" +
		"```\n"

	if f := Check(body, "analytics-service", known); len(f) != 0 {
		t.Fatalf("an output block was checked as source: %v", f)
	}
}

func TestABodyThatCitesNothingIsRefused(t *testing.T) {
	_, known := tree(t)
	f := Check("The gate is red. I think the fixture is stale.\n", "analytics-service", known)
	if !has(f, RuleNoCitations) {
		t.Errorf("a body with no citations passed; findings = %v", rules(f))
	}
}

// The working repository is named constantly in its own issues; requiring it to
// be cited by prefix would refuse every body.
func TestTheWorkingRepositoryIsNotSubjectToTheCrossRepoRule(t *testing.T) {
	_, known := tree(t)
	body := "analytics-service is red at dev HEAD.\n" +
		"See `internal/adapters/postgres/gate_test.go:61`.\n"
	if f := Check(body, "analytics-service", known); has(f, RuleUncitedRepo) {
		t.Errorf("the working repository triggered the cross-repo rule: %v", f)
	}
}

func TestCitationsAreAttributedByPathPrefix(t *testing.T) {
	_, known := tree(t)
	body := "`internal/x.go:1` and `goals-service/internal/app/command/funding_advisory.go:2`\n"
	cites := Citations(body, known)
	if len(cites) != 2 {
		t.Fatalf("found %d citations, want 2: %v", len(cites), cites)
	}
	if cites[0].Repo != "" {
		t.Errorf("bare path attributed to %q, want the working repository", cites[0].Repo)
	}
	if cites[1].Repo != "goals-service" || cites[1].Path != "internal/app/command/funding_advisory.go" {
		t.Errorf("prefixed path mis-attributed: %+v", cites[1])
	}
}

// Prose about issue numbers, versions and dates must not read as citations, or
// the gate manufactures missing-file findings for ordinary sentences.
func TestProseIsNotMistakenForACitation(t *testing.T) {
	_, known := tree(t)
	body := "Issue 42:1 was merged in v1.5.0 at 02:26 on 2030-01-02, see PR 51.\n" +
		"Real one: `internal/adapters/postgres/gate_test.go:61`.\n"
	cites := Citations(body, known)
	if len(cites) != 1 {
		t.Fatalf("prose parsed as citations: %v", cites)
	}
}

// A repository named only as a substring of another must not count as named.
func TestRepoNameMatchingIsWordBounded(t *testing.T) {
	root := t.TempDir()
	for _, r := range []string{"analytics-core", "analytics-corex"} {
		if err := os.MkdirAll(filepath.Join(root, r, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	known, err := checkout.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := NamedRepos("work in analytics-corex only", "analytics-corex", known); len(got) != 0 {
		t.Errorf("substring match named %v; analytics-core is not mentioned", got)
	}
}

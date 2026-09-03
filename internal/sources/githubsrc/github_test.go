// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package githubsrc

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// fakeGH answers by subcommand so a test states only the calls it cares about.
func fakeGH(t *testing.T, byVerb map[string]string) (Runner, *[]string) {
	t.Helper()
	var calls []string
	return func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		verb := args[0]
		// The api verb covers more than one endpoint, so a test names the one
		// it means. Actions permissions answers enabled unless a test says
		// otherwise: defaulting the other way would silently route every test
		// about something else down the ungated-repository path.
		if verb == "api" && strings.HasSuffix(args[len(args)-1], "/actions/permissions") {
			if body, ok := byVerb["permissions"]; ok {
				return []byte(body), nil
			}
			return []byte(`{"enabled":true}`), nil
		}
		body, ok := byVerb[verb]
		if !ok {
			return []byte("[]"), nil
		}
		return []byte(body), nil
	}, &calls
}

// keyed answers per repository, for the collisions that only exist between two
// of them. A survey runs one Collect over the whole estate, so a defect that
// needs two repositories cannot be shown with a fake that answers one.
func keyed(t *testing.T, byRepoVerb map[string]string) Runner {
	t.Helper()
	return func(_ context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if args[0] == "api" && strings.HasSuffix(args[len(args)-1], "/actions/permissions") {
			for k, v := range byRepoVerb {
				if strings.HasPrefix(k, "permissions:") &&
					strings.Contains(joined, strings.TrimPrefix(k, "permissions:")) {
					return []byte(v), nil
				}
			}
			return []byte(`{"enabled":true}`), nil
		}
		for k, v := range byRepoVerb {
			verb, repo, ok := strings.Cut(k, ":")
			if ok && args[0] == verb && strings.Contains(joined, repo) {
				return []byte(v), nil
			}
		}
		return []byte("[]"), nil
	}
}

func collect(t *testing.T, s *Source, scope signal.Scope) []signal.Signal {
	t.Helper()
	got, err := s.Collect(context.Background(), scope)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return got
}

func TestRefusesToSweepAnOrgWithNoRepos(t *testing.T) {
	run, _ := fakeGH(t, nil)
	s := New(Options{Owner: "example-org", Run: run})
	if _, err := s.Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("collecting with no repos configured or scoped succeeded")
	}
}

func TestOwnerBlockedIssuesAreADistinctKind(t *testing.T) {
	// Available work and work stuck on a person are different facts. Merging
	// them hides how much is waiting on the owner.
	issues := `[
	  {"number":75,"title":"pool warns at idle","url":"u/75","createdAt":"2026-07-20T14:52:33Z","updatedAt":"2026-08-01T00:00:00Z","labels":[{"name":"type:bug"}],"assignees":[],"comments":[{"id":"IC_1"}]},
	  {"number":97,"title":"execute the backfill run","url":"u/97","createdAt":"2026-07-22T00:00:00Z","updatedAt":"2026-07-22T00:00:00Z","labels":[{"name":"pending-operator"}],"assignees":[{"login":"sample-user"}],"comments":[]}
	]`
	run, _ := fakeGH(t, map[string]string{"issue": issues})
	s := New(Options{Owner: "example-org", Repos: []string{"payments-api"}, Run: run})

	got := collect(t, s, signal.Scope{})
	var work, blocked int
	for _, g := range got {
		switch g.Kind {
		case signal.KindWorkItem:
			work++
		case signal.KindBlocked:
			blocked++
			if g.Attrs["assignees"] != "sample-user" {
				t.Errorf("assignee provenance lost: %+v", g.Attrs)
			}
		}
	}
	if work != 1 || blocked != 1 {
		t.Fatalf("work=%d blocked=%d, want 1 and 1: %+v", work, blocked, got)
	}
}

func TestOwnerLabelsAreConfigurableNotHardcoded(t *testing.T) {
	// Another installation names these differently; a constant would make this
	// a one-organization-only tool.
	issues := `[{"number":1,"title":"x","url":"u","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:00Z","labels":[{"name":"awaiting-product"}],"assignees":[],"comments":[]}]`
	run, _ := fakeGH(t, map[string]string{"issue": issues})
	s := New(Options{Owner: "acme", Repos: []string{"thing"}, OwnerLabels: []string{"awaiting-product"}, Run: run})

	got := collect(t, s, signal.Scope{})
	if len(got) != 1 || got[0].Kind != signal.KindBlocked {
		t.Fatalf("custom owner label not honoured: %+v", got)
	}
}

func TestRepeatedlyFailingWorkflowCollapsesToOneSignal(t *testing.T) {
	// A workflow that failed forty times overnight is one broken thing.
	runs := `[
	  {"databaseId":3,"name":"Unit Test Gate","displayTitle":"fix: x","conclusion":"failure","headBranch":"dev","url":"u3","createdAt":"2026-08-25T09:00:00Z","event":"push"},
	  {"databaseId":2,"name":"Unit Test Gate","displayTitle":"fix: x","conclusion":"failure","headBranch":"dev","url":"u2","createdAt":"2026-08-25T08:00:00Z","event":"push"},
	  {"databaseId":1,"name":"DB Integration Baseline Gate","displayTitle":"fix: y","conclusion":"failure","headBranch":"dev","url":"u1","createdAt":"2026-08-25T07:00:00Z","event":"push"}
	]`
	run, _ := fakeGH(t, map[string]string{"run": runs})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	var builds []signal.Signal
	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			builds = append(builds, g)
		}
	}
	if len(builds) != 2 {
		t.Fatalf("got %d build signals, want 2 (newest per workflow): %+v", len(builds), builds)
	}
	if builds[0].URL != "u3" {
		t.Errorf("kept run %q, want the newest (u3)", builds[0].URL)
	}
}

func TestSameWorkflowOnDifferentBranchesStaysDistinct(t *testing.T) {
	runs := `[
	  {"databaseId":2,"name":"Unit Test Gate","displayTitle":"a","conclusion":"failure","headBranch":"dev","url":"u2","createdAt":"2026-08-25T09:00:00Z","event":"push"},
	  {"databaseId":1,"name":"Unit Test Gate","displayTitle":"b","conclusion":"failure","headBranch":"main","url":"u1","createdAt":"2026-08-25T08:00:00Z","event":"push"}
	]`
	run, _ := fakeGH(t, map[string]string{"run": runs})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	n := 0
	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("collapsed two branches into %d signal(s); a red main and a red dev are different problems", n)
	}
}

func TestProviderDetailSurvivesInAttrs(t *testing.T) {
	prs := `[{"number":9,"title":"wire it up","url":"u9","createdAt":"2026-08-20T00:00:00Z","updatedAt":"2026-08-24T00:00:00Z","labels":[],"isDraft":true,"mergeable":"CONFLICTING","headRefName":"fix/thing"}]`
	run, _ := fakeGH(t, map[string]string{"pr": prs})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	got := collect(t, s, signal.Scope{})
	if len(got) != 1 {
		t.Fatalf("want 1 change set, got %d", len(got))
	}
	cs := got[0]
	if cs.Kind != signal.KindChangeSet || cs.ID != "PR #9" {
		t.Fatalf("unexpected signal: %+v", cs)
	}
	for k, want := range map[string]string{"draft": "true", "mergeable": "conflicting", "branch": "fix/thing"} {
		if cs.Attrs[k] != want {
			t.Errorf("Attrs[%s] = %q, want %q — provider detail must reach the model uninterpreted", k, cs.Attrs[k], want)
		}
	}
}

// A session waking to a survey has to be able to tell a peer's draft that is
// ready for review from one that is a stub, without opening either. Body size
// and the review count are what separate them; the survey states both and
// judges neither.
func TestADraftSaysWhetherItCarriesAnythingAndWhetherAnybodyRead(t *testing.T) {
	prs := `[
	 {"number":51,"title":"fix freshness","url":"u51","createdAt":"2026-08-28T19:33:07Z","updatedAt":"2026-08-28T19:33:07Z","labels":[],"isDraft":true,"mergeable":"MERGEABLE","headRefName":"mellions/task-51","author":{"login":"sample-user"},"body":"` + strings.Repeat("x", 9284) + `","reviews":[],"comments":[{},{}]},
	 {"number":42,"title":"wip","url":"u42","createdAt":"2026-08-28T19:33:07Z","updatedAt":"2026-08-28T19:33:07Z","labels":[],"isDraft":true,"mergeable":"MERGEABLE","headRefName":"mellions/stub","author":{"login":"sample-user"},"body":"","reviews":[{},{}]}
	]`
	run, calls := fakeGH(t, map[string]string{"pr": prs})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	got := collect(t, s, signal.Scope{})
	if len(got) != 2 {
		t.Fatalf("want 2 change sets, got %d", len(got))
	}
	by := map[string]map[string]string{}
	for _, cs := range got {
		by[cs.ID] = cs.Attrs
	}

	// The evidenced draft nobody has reviewed — the case a peer leaves behind.
	for k, want := range map[string]string{"body": "9.3kB", "reviews": "0", "comments": "2", "author": "sample-user", "draft": "true"} {
		if by["PR #51"][k] != want {
			t.Errorf("PR #51 Attrs[%s] = %q, want %q", k, by["PR #51"][k], want)
		}
	}
	// The stub that already has eyes on it. 0B must be stated, not omitted:
	// an absent attribute reads as "not collected", which is a different fact.
	for k, want := range map[string]string{"body": "0B", "reviews": "2"} {
		if by["PR #42"][k] != want {
			t.Errorf("PR #42 Attrs[%s] = %q, want %q", k, by["PR #42"][k], want)
		}
	}

	// gh returns nothing it was not asked for, so a field missing from the
	// --json list is a silently empty attribute rather than an error. The field
	// must be reviews and not latestReviews: latestReviews omits reviews left
	// by the pull request's own author, and every session shares one account,
	// so a peer's review of a peer's change set would not be counted.
	var prCall string
	for _, c := range *calls {
		if strings.HasPrefix(c, "pr ") {
			prCall = c
		}
	}
	// comments is requested for the same reason reviews is, and it is the half
	// that keeps the other honest: several repositories in the estate record
	// the merge-gate verdict as a POSTED COMMENT rather than a submitted
	// GitHub review, so a change set with two full independent reviews on it
	// reports reviews 0. Without this field the brief line says "0 reviews"
	// and a shift reads that as unread work.
	for _, field := range []string{"author", "body", "reviews", "comments"} {
		if !strings.Contains(prCall, field) {
			t.Errorf("gh pr list does not request %q — the attribute would be silently empty. call: %s", field, prCall)
		}
	}
}

func TestScopeOverridesConfiguredRepos(t *testing.T) {
	run, calls := fakeGH(t, nil)
	s := New(Options{Owner: "example-org", Repos: []string{"analytics-service"}, Run: run})
	collect(t, s, signal.Scope{Repos: []string{"payments-api"}})

	joined := strings.Join(*calls, "\n")
	if !strings.Contains(joined, "example-org/payments-api") {
		t.Errorf("scope repo was not queried:\n%s", joined)
	}
	if strings.Contains(joined, "analytics-service") {
		t.Errorf("configured repo was queried despite an explicit scope:\n%s", joined)
	}
}

func TestFullyQualifiedRepoIsNotDoublePrefixed(t *testing.T) {
	run, calls := fakeGH(t, nil)
	s := New(Options{Owner: "example-org", Run: run})
	collect(t, s, signal.Scope{Repos: []string{"other-org/thing"}})

	if !strings.Contains(strings.Join(*calls, "\n"), "other-org/thing") {
		t.Errorf("qualified repo mangled: %v", *calls)
	}
	if strings.Contains(strings.Join(*calls, "\n"), "example-org/other-org") {
		t.Errorf("qualified repo was prefixed with the default owner: %v", *calls)
	}
}

func TestUnreachableProviderIsAnErrorNotAnEmptyPicture(t *testing.T) {
	// The property the whole survey depends on: "no open issues" and "could not
	// read the repo" must never be the same result.
	boom := errors.New("gh: not authenticated")
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: func(context.Context, ...string) ([]byte, error) {
		return nil, boom
	}})
	if _, err := s.Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("an unreachable provider returned success")
	}
}

func TestMalformedResponseIsAnError(t *testing.T) {
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: func(context.Context, ...string) ([]byte, error) {
		return []byte("not json"), nil
	}})
	if _, err := s.Collect(context.Background(), signal.Scope{}); err == nil {
		t.Fatal("malformed provider output was accepted as an empty picture")
	}
}

func TestScopeLimitReachesTheProvider(t *testing.T) {
	run, calls := fakeGH(t, nil)
	s := New(Options{Owner: "o", Repos: []string{"r"}, PerRepoLimit: 50, Run: run})
	collect(t, s, signal.Scope{Limit: 3})

	if !slices.ContainsFunc(*calls, func(c string) bool { return strings.Contains(c, "--limit 3") }) {
		t.Errorf("scope limit not passed through: %v", *calls)
	}
}

// TestARepositoryWithIssuesDisabledIsNotAFailure: no issues is a fact about
// the repository, and reading it as an unreachable source made every survey
// of an estate with one such repository INCOMPLETE.
func TestARepositoryWithIssuesDisabledIsNotAFailure(t *testing.T) {
	run := func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "issue" {
			return nil, errors.New("gh issue list: the 'acme/quiet' repository has disabled issues")
		}
		return []byte("[]"), nil
	}
	src := New(Options{Owner: "acme", Repos: []string{"quiet"}, Run: run})
	got, err := src.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatalf("issues disabled was reported as a failure: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d signals from a repository with issues off", len(got))
	}
}

func TestRefusedRunIsNotReportedAsAFailingCheck(t *testing.T) {
	// A run refused before it began carries conclusion "failure" and no steps.
	// Reported as a red suite it sends a reader hunting a defect in code that
	// was never compiled.
	runs := `[
	  {"databaseId":7,"name":"check","displayTitle":"ci: gate pull requests","conclusion":"failure","headBranch":"topic","url":"u7","createdAt":"2026-08-28T17:21:49Z","event":"pull_request"}
	]`
	jobs := `{"jobs":[{"name":"check","conclusion":"failure","steps":[]}]}`
	run, _ := fakeGH(t, map[string]string{"run": runs, "api": jobs})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	var b *signal.Signal
	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			b = &g
		}
	}
	if b == nil {
		t.Fatal("refused run dropped entirely; it is a real problem and must still be reported")
	}
	if b.Attrs["started"] != "false" {
		t.Errorf(`attrs["started"] = %q, want "false"`, b.Attrs["started"])
	}
	if !strings.Contains(b.Title, "never started") {
		t.Errorf("title %q does not say the job never started, so the default read still calls it a failing check", b.Title)
	}
}

func TestRunThatExecutedStepsStaysAPlainFailingCheck(t *testing.T) {
	// The other side of the control: a genuinely red suite must not be
	// excused as infrastructure.
	runs := `[
	  {"databaseId":8,"name":"check","displayTitle":"fix: a real break","conclusion":"failure","headBranch":"dev","url":"u8","createdAt":"2026-08-28T17:21:49Z","event":"push"}
	]`
	jobs := `{"jobs":[{"name":"check","conclusion":"failure","steps":[{"name":"go test"}]}]}`
	run, _ := fakeGH(t, map[string]string{"run": runs, "api": jobs})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	var b *signal.Signal
	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			b = &g
		}
	}
	if b == nil {
		t.Fatal("genuine failing check dropped")
	}
	if _, marked := b.Attrs["started"]; marked {
		t.Errorf(`a run that executed steps was marked started=%q; a real red suite must not read as infrastructure`, b.Attrs["started"])
	}
	if b.Title != "fix: a real break" {
		t.Errorf("title = %q, want it left alone", b.Title)
	}
}

func TestUnreadableJobsListLeavesAFailureAlone(t *testing.T) {
	// The safe direction. If the jobs list cannot be read, the run keeps its
	// plain reading: silently downgrading a failure to "never ran" would hide
	// a real break behind an infrastructure excuse.
	runs := `[
	  {"databaseId":9,"name":"check","displayTitle":"fix: y","conclusion":"failure","headBranch":"dev","url":"u9","createdAt":"2026-08-28T17:21:49Z","event":"push"}
	]`
	run, _ := fakeGH(t, map[string]string{"run": runs, "api": `not json`})
	s := New(Options{Owner: "o", Repos: []string{"r"}, Run: run})

	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			if _, marked := g.Attrs["started"]; marked {
				t.Errorf("unreadable jobs list marked the run as never started")
			}
		}
	}
}

// --- #121: an ID is only unique inside the repository it was read from ---

func TestSameWorkflowNameInTwoRepositoriesSurvivesDedupe(t *testing.T) {
	// The estate has a workflow called CI in almost every repository. Keyed on
	// the bare name, the second repository's red CI is dropped by Dedupe and
	// the survey reports that repository green.
	red := func(id int64, branch string) string {
		return `[{"databaseId":` + strconv.FormatInt(id, 10) + `,"name":"CI","displayTitle":"t",` +
			`"conclusion":"failure","headBranch":"` + branch + `","url":"u/` +
			strconv.FormatInt(id, 10) + `","createdAt":"2026-08-01T00:00:00Z","event":"push"}]`
	}
	s := New(Options{Owner: "example-org", Repos: []string{"alpha", "beta"}, Run: keyed(t, map[string]string{
		"run:alpha": red(1, "main"),
		"run:beta":  red(2, "main"),
		"api:":      `{"jobs":[{"steps":[{"name":"go test"}]}]}`,
	})})

	got := signal.Dedupe(collect(t, s, signal.Scope{}))
	var repos []string
	for _, g := range got {
		if g.Kind == signal.KindBuild {
			repos = append(repos, g.Repo)
		}
	}
	slices.Sort(repos)
	if !slices.Equal(repos, []string{"alpha", "beta"}) {
		t.Fatalf("failing CI survived in %v, want both alpha and beta; a repository "+
			"whose only red check shares a name with another repository's reads as green", repos)
	}
}

func TestSameIssueNumberInTwoRepositoriesSurvivesDedupe(t *testing.T) {
	// Issue numbers are repository-local, so the repository is part of the key.
	issue := func(n int) string {
		return `[{"number":` + strconv.Itoa(n) + `,"title":"t","url":"u","createdAt":"2026-08-01T00:00:00Z",` +
			`"updatedAt":"2026-08-01T00:00:00Z","labels":[],"assignees":[],"comments":[]}]`
	}
	s := New(Options{Owner: "example-org", Repos: []string{"alpha", "beta"}, Run: keyed(t, map[string]string{
		"issue:alpha": issue(7),
		"issue:beta":  issue(7),
	})})

	got := signal.Dedupe(collect(t, s, signal.Scope{}))
	var repos []string
	for _, g := range got {
		if g.Kind == signal.KindWorkItem {
			repos = append(repos, g.Repo)
		}
	}
	slices.Sort(repos)
	if !slices.Equal(repos, []string{"alpha", "beta"}) {
		t.Fatalf("issue #7 survived in %v, want both alpha and beta", repos)
	}
}

func TestOneWorkflowRedOnTwoBranchesSurvivesDedupe(t *testing.T) {
	// failedRuns already treats name and branch as the identity in its own
	// filter. Handing the survey an ID of the name alone let Dedupe collapse
	// what that filter had just decided were two different things.
	runs := `[
	  {"databaseId":1,"name":"CI","displayTitle":"on main","conclusion":"failure","headBranch":"main","url":"u/1","createdAt":"2026-08-01T00:00:00Z","event":"push"},
	  {"databaseId":2,"name":"CI","displayTitle":"on dev","conclusion":"failure","headBranch":"dev","url":"u/2","createdAt":"2026-08-02T00:00:00Z","event":"push"}
	]`
	run, _ := fakeGH(t, map[string]string{"run": runs, "api": `{"jobs":[{"steps":[{"name":"go test"}]}]}`})
	s := New(Options{Owner: "example-org", Repos: []string{"alpha"}, Run: run})

	got := signal.Dedupe(collect(t, s, signal.Scope{}))
	var branches []string
	for _, g := range got {
		if g.Kind == signal.KindBuild {
			branches = append(branches, g.Attrs["branch"])
		}
	}
	slices.Sort(branches)
	if !slices.Equal(branches, []string{"dev", "main"}) {
		t.Fatalf("CI red on two branches survived as %v, want both main and dev", branches)
	}
}

// --- #120: a repository that runs no workflows has no failing checks ---

const disabledRuns = `[
  {"databaseId":7,"name":"Dependabot Updates","displayTitle":"managed","conclusion":"success","headBranch":"main","url":"u/7","createdAt":"2026-08-24T00:00:00Z","event":"dynamic"},
  {"databaseId":9,"name":"CI","displayTitle":"frozen","conclusion":"failure","headBranch":"main","url":"u/9","createdAt":"2026-08-10T11:22:33Z","event":"push"},
  {"databaseId":8,"name":"Docker","displayTitle":"frozen too","conclusion":"failure","headBranch":"dev","url":"u/8","createdAt":"2026-08-09T00:00:00Z","event":"push"}
]`

func TestActionsDisabledIsOneSignalNamingTheStoppage(t *testing.T) {
	run, calls := fakeGH(t, map[string]string{
		"run":         disabledRuns,
		"permissions": `{"enabled":false}`,
	})
	s := New(Options{Owner: "example-org", Repos: []string{"alpha"}, Run: run})

	var builds []signal.Signal
	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			builds = append(builds, g)
		}
	}
	if len(builds) != 1 {
		t.Fatalf("an ungated repository contributed %d build signals, want exactly 1: %+v", len(builds), builds)
	}
	b := builds[0]
	if !strings.Contains(b.Title, "Actions is disabled") {
		t.Errorf("title %q does not name the condition, so a reader still has to infer it", b.Title)
	}
	if !strings.Contains(b.Title, "2026-08-10") {
		t.Errorf("title %q does not carry the last-run date, so the reader cannot tell how long this has been ungated", b.Title)
	}
	if b.Attrs["actions"] != "disabled" {
		t.Errorf(`attrs["actions"] = %q, want "disabled"`, b.Attrs["actions"])
	}
	if b.Age(time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)) < 17*24*time.Hour {
		t.Errorf("age is measured from something other than the last run: %v", b.Updated)
	}
	// The frozen runs must not also be listed, and reading them at all is the
	// per-run jobs call this path exists to avoid.
	for _, c := range *calls {
		if strings.Contains(c, "/jobs") {
			t.Errorf("an ungated repository still read a run's jobs: %q", c)
		}
		if strings.Contains(c, "--status failure") {
			t.Errorf("an ungated repository still listed failing runs: %q", c)
		}
	}
}

// enabledRuns is the same repository with the GitHub-managed run left out, so
// the controls below state "two red workflows" and mean it. A managed run that
// genuinely fails is still a failing check when Actions are on; only the dating
// of a stoppage ignores it.
const enabledRuns = `[
  {"databaseId":9,"name":"CI","displayTitle":"red","conclusion":"failure","headBranch":"main","url":"u/9","createdAt":"2026-08-10T11:22:33Z","event":"push"},
  {"databaseId":8,"name":"Docker","displayTitle":"red too","conclusion":"failure","headBranch":"dev","url":"u/8","createdAt":"2026-08-09T00:00:00Z","event":"push"}
]`

func TestActionsEnabledStillListsEveryFailingCheck(t *testing.T) {
	// The control the acceptance criterion names. A change that collapses this
	// case too is the same defect with the labels swapped.
	run, _ := fakeGH(t, map[string]string{
		"run":         enabledRuns,
		"api":         `{"jobs":[{"steps":[{"name":"go test"}]}]}`,
		"permissions": `{"enabled":true}`,
	})
	s := New(Options{Owner: "example-org", Repos: []string{"alpha"}, Run: run})

	var builds []signal.Signal
	for _, g := range collect(t, s, signal.Scope{}) {
		if g.Kind == signal.KindBuild {
			builds = append(builds, g)
		}
	}
	if len(builds) != 2 {
		t.Fatalf("a gated repository with two red workflows produced %d signals, want 2: %+v", len(builds), builds)
	}
	for _, b := range builds {
		if b.Attrs["actions"] == "disabled" {
			t.Errorf("a gated repository was reported as ungated: %+v", b)
		}
	}
}

func TestUnreadableActionsPermissionsKeepsFailingChecksVisible(t *testing.T) {
	// The safe direction. This call only ever suppresses check runs, so one
	// that cannot be read must leave a genuinely red suite exactly where it was.
	for name, body := range map[string]string{
		"not json":         `not json`,
		"field is absent":  `{"allowed_actions":"all"}`,
		"answer is a list": `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			run, _ := fakeGH(t, map[string]string{
				"run":         enabledRuns,
				"api":         `{"jobs":[{"steps":[{"name":"go test"}]}]}`,
				"permissions": body,
			})
			s := New(Options{Owner: "example-org", Repos: []string{"alpha"}, Run: run})
			var builds int
			for _, g := range collect(t, s, signal.Scope{}) {
				if g.Kind == signal.KindBuild {
					builds++
					if g.Attrs["actions"] == "disabled" {
						t.Fatalf("an unreadable permissions answer reported the repository as ungated")
					}
				}
			}
			if builds != 2 {
				t.Fatalf("got %d failing checks, want 2 — an unreadable permissions answer hid a red suite", builds)
			}
		})
	}
}

func TestEveryUngatedRepositorySaysSoForItself(t *testing.T) {
	// The two issues are one change: keyed without the repository, the ten
	// ungated repositories of this estate would render as a single line and
	// nine of them would go unmentioned — a worse false green than the corpses.
	s := New(Options{Owner: "example-org", Repos: []string{"alpha", "beta"}, Run: keyed(t, map[string]string{
		"run:alpha":         disabledRuns,
		"run:beta":          disabledRuns,
		"permissions:alpha": `{"enabled":false}`,
		"permissions:beta":  `{"enabled":false}`,
	})})

	var repos []string
	for _, g := range signal.Dedupe(collect(t, s, signal.Scope{})) {
		if g.Kind == signal.KindBuild {
			repos = append(repos, g.Repo)
		}
	}
	slices.Sort(repos)
	if !slices.Equal(repos, []string{"alpha", "beta"}) {
		t.Fatalf("ungated repositories reported: %v, want both alpha and beta", repos)
	}
}

func TestStoppageIsDatedFromTheLastRunThatGatedAnything(t *testing.T) {
	// A managed "dynamic" run is not evidence that a repository workflow gated
	// changes after Actions was disabled.
	run, _ := fakeGH(t, map[string]string{"run": disabledRuns, "permissions": `{"enabled":false}`})
	s := New(Options{Owner: "example-org", Repos: []string{"alpha"}, Run: run})

	got := collect(t, s, signal.Scope{})
	if len(got) != 1 {
		t.Fatalf("want 1 signal, got %d", len(got))
	}
	b := got[0]
	if strings.Contains(b.Title, "2026-08-24") {
		t.Fatalf("the stoppage was dated from a GitHub-managed run: %q", b.Title)
	}
	if !strings.Contains(b.Title, "2026-08-10") {
		t.Fatalf("title %q does not date the stoppage from the last gating run (2026-08-10)", b.Title)
	}
	if b.Attrs["last_run_workflow"] != "CI" {
		t.Errorf(`last_run_workflow = %q, want "CI"`, b.Attrs["last_run_workflow"])
	}
}

func TestEveryRunManagedByGitHubMeansNothingObservableGatedIt(t *testing.T) {
	managed := `[{"databaseId":7,"name":"Dependabot Updates","displayTitle":"m","conclusion":"success","headBranch":"main","url":"u/7","createdAt":"2026-08-24T00:00:00Z","event":"dynamic"}]`
	run, _ := fakeGH(t, map[string]string{"run": managed, "permissions": `{"enabled":false}`})
	s := New(Options{Owner: "example-org", Repos: []string{"alpha"}, Run: run})

	got := collect(t, s, signal.Scope{})
	if len(got) != 1 {
		t.Fatalf("want 1 signal, got %d", len(got))
	}
	if strings.Contains(got[0].Title, "2026-08-24") {
		t.Fatalf("a repository whose only runs are GitHub-managed was reported as gated until then: %q", got[0].Title)
	}
	if !strings.Contains(got[0].Title, "nothing observable has gated it") {
		t.Fatalf("title %q does not say that nothing observable gated the repository", got[0].Title)
	}
}

// #186: a draft with an independent review in flight said so in a comment, and
// a peer on another host merged it because nothing it could see without opening
// the pull request said "held". The claim label is what it can see.
func TestAClaimedPullRequestSaysAPeerHoldsIt(t *testing.T) {
	prs := `[
	 {"number":51,"title":"held draft","url":"u51","createdAt":"2030-01-02T04:44:00Z","updatedAt":"2030-01-02T04:44:00Z","labels":[{"name":"` + claim.Label + `"}],"isDraft":true,"mergeable":"MERGEABLE","headRefName":"mellions/held","author":{"login":"sample-user"},"body":"x","reviews":[]},
	 {"number":52,"title":"free draft","url":"u52","createdAt":"2030-01-02T04:44:00Z","updatedAt":"2030-01-02T04:44:00Z","labels":[],"isDraft":true,"mergeable":"MERGEABLE","headRefName":"mellions/free","author":{"login":"sample-user"},"body":"x","reviews":[]}
	]`
	run, _ := fakeGH(t, map[string]string{"pr": prs})
	got := collect(t, New(Options{Owner: "o", Repos: []string{"r"}, Run: run}), signal.Scope{})

	by := map[string]signal.Signal{}
	for _, cs := range got {
		by[cs.ID] = cs
	}
	held := by["PR #51"].Attrs["claimed"]
	if held == "" {
		t.Fatal("a pull request carrying " + claim.Label + " says nothing about being held; " +
			"a peer scanning a survey cannot tell it from an unreviewed draft")
	}
	for _, want := range []string{"Mellions lane", "before merging"} {
		if !strings.Contains(held, want) {
			t.Errorf("claimed = %q, missing %q — the sentence has to be actionable, not a flag", held, want)
		}
	}
	if free := by["PR #179"].Attrs["claimed"]; free != "" {
		t.Errorf("an unclaimed pull request carries claimed=%q; the marker would mean nothing", free)
	}
}

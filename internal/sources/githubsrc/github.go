// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package githubsrc collects engineering signals from GitHub.
//
// It is one implementation of signal.Source among several possible ones. The
// core never imports it: swapping GitHub for GitLab, Jira or a plain tracker is
// a new package here plus a configuration change, never a change to the survey
// or the signal model. That boundary is what stops this becoming a GitHub-only
// tool that has to be rebuilt when the tracker changes.
//
// Provider specifics — assignees, draft state, check names, label taxonomies —
// travel in Signal.Attrs uninterpreted, so GitHub's richness reaches the model
// without the core growing an opinion about it.
package githubsrc

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/claim"
	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Runner executes the gh CLI and returns stdout. Replaced in tests.
//
// Shelling out to gh rather than speaking the REST API directly is deliberate:
// gh already holds the authentication, follows pagination, and tracks GitHub's
// API changes. Reimplementing that would be a second thing to maintain for no
// gain, and the provider boundary that matters is signal.Source, not transport.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

func execRunner(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), firstLine(msg))
	}
	return out, nil
}

// Options configures the source.
type Options struct {
	// Owner is the GitHub org or user. Required.
	Owner string
	// Repos limits collection. Empty means every repo the scope names; if the
	// scope names none either, collection is refused rather than sweeping an
	// entire organization by accident.
	Repos []string
	// OwnerLabels mark work waiting on the owner. Provider-specific by nature,
	// so they are configuration rather than a constant: another installation
	// will have named them differently.
	OwnerLabels []string
	// PerRepoLimit caps items of each type per repository.
	PerRepoLimit int
	// Run executes gh; nil uses the real CLI.
	Run Runner
}

// DefaultPerRepoLimit caps items of each type per repository when nothing
// configures it. It is exported because a survey that shows a repository's
// first fifty issues and says "50" has told the reader a count when what it
// held was a cap; the renderer names the number so the reader can tell.
const DefaultPerRepoLimit = 50

// Source collects issues, pull requests, failing checks and owner-blocked work.
type Source struct{ opts Options }

// New returns a configured source.
func New(o Options) *Source {
	if o.PerRepoLimit <= 0 {
		o.PerRepoLimit = DefaultPerRepoLimit
	}
	if o.Run == nil {
		o.Run = execRunner
	}
	if len(o.OwnerLabels) == 0 {
		o.OwnerLabels = []string{"needs-owner", "pending-owner-decision", "pending-operator"}
	}
	return &Source{opts: o}
}

// Name implements signal.Source.
func (s *Source) Name() string { return "github" }

// Collect gathers work items, change sets, failing checks and blocked work.
//
// A failure against any repository fails the whole collection. That is
// deliberate: a partial GitHub picture rendered as a complete one is how an
// engineer concludes there is nothing to do in a repository it simply could not
// read. The survey records the failure and says the picture is incomplete.
func (s *Source) Collect(ctx context.Context, scope signal.Scope) ([]signal.Signal, error) {
	repos := s.opts.Repos
	if len(scope.Repos) > 0 {
		repos = scope.Repos
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("githubsrc: no repositories configured or scoped; refusing to sweep %s", s.opts.Owner)
	}
	limit := s.opts.PerRepoLimit
	if scope.Limit > 0 {
		limit = scope.Limit
	}

	var out []signal.Signal
	// Per repository, because one of them being unreachable is not a fact about
	// the other twenty. A repository with issues disabled, or one this token
	// cannot see, used to abort the whole sweep — so a single misconfigured
	// corner reported the entire estate as having no open work, which is the
	// one reading a collector must never produce.
	var unreachable []string
	for _, repo := range repos {
		full := s.full(repo)

		issues, err := s.issues(ctx, full, limit)
		if err != nil {
			unreachable = append(unreachable, fmt.Sprintf("%s: %v", repo, err))
			continue
		}
		out = append(out, issues...)

		prs, err := s.pulls(ctx, full, limit)
		if err != nil {
			unreachable = append(unreachable, fmt.Sprintf("%s: %v", repo, err))
			continue
		}
		out = append(out, prs...)

		runs, err := s.failedRuns(ctx, full)
		if err != nil {
			unreachable = append(unreachable, fmt.Sprintf("%s: %v", repo, err))
			continue
		}
		out = append(out, runs...)
	}
	if len(unreachable) > 0 {
		// Both: what was collected, and what could not be. The reader is told
		// which repositories are unaccounted for rather than being handed a
		// picture that looks complete.
		return out, fmt.Errorf("githubsrc: %d of %d repositories could not be read — %s",
			len(unreachable), len(repos), strings.Join(unreachable, "; "))
	}
	return out, nil
}

func (s *Source) full(repo string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	return s.opts.Owner + "/" + repo
}

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	// Comments is an array of comment objects, not a count. gh has always
	// returned it this way; a test fixture that guessed `"comments": 1` agreed
	// with the mistake and only the first real call disagreed.
	Comments []struct{} `json:"comments"`
	// Reviews is one entry per submitted review, not a count, and unlike
	// latestReviews it includes reviews left by the pull request's own author.
	// Every Mellions session pushes as one account, so a peer's review of a
	// peer's change set is author-authored and latestReviews omits it — which
	// would report "nobody has read this" about the review that just happened.
	Reviews []struct{} `json:"reviews"`
	Author  struct {
		Login string `json:"login"`
	} `json:"author"`
	Body      string `json:"body"`
	IsDraft   bool   `json:"isDraft"`
	Mergeable string `json:"mergeable"`
	HeadRef   string `json:"headRefName"`
}

func (g ghIssue) labels() []string {
	out := make([]string, 0, len(g.Labels))
	for _, l := range g.Labels {
		out = append(out, l.Name)
	}
	return out
}

// humanBytes sizes a pull request body. It is a measurement and not a verdict:
// a survey that scored a body as "evidenced" would be ranking, and reading the
// body is the reviewer's job. An empty body prints 0B rather than going absent,
// so "carries nothing" cannot be misread as "was not collected".
func humanBytes(n int) string {
	if n < 1000 {
		return strconv.Itoa(n) + "B"
	}
	return fmt.Sprintf("%.1fkB", float64(n)/1000)
}

func (s *Source) issues(ctx context.Context, full string, limit int) ([]signal.Signal, error) {
	raw, err := s.opts.Run(ctx, "issue", "list", "-R", full, "--state", "open",
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,url,createdAt,updatedAt,labels,assignees,comments")
	if err != nil {
		// A repository with issues switched off has no issues. That is a fact
		// about the repository, and reporting it as an unreachable source made
		// every survey of an estate with one such repository read INCOMPLETE.
		if IssuesDisabled(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []ghIssue
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("githubsrc: decode issues for %s: %w", full, err)
	}

	repo := shortName(full)
	out := make([]signal.Signal, 0, len(items))
	for _, it := range items {
		labels := it.labels()
		kind := signal.KindWorkItem
		// Owner-blocked work is a different fact from ordinary open work: one
		// is available to pick up, the other is waiting on a person. Rendering
		// them together would hide how much is stuck.
		if hasAny(labels, s.opts.OwnerLabels) {
			kind = signal.KindBlocked
		}
		attrs := map[string]string{"type": "issue"}
		if n := len(it.Comments); n > 0 {
			attrs["comments"] = strconv.Itoa(n)
		}
		// A lane somewhere in the estate holds this. The survey is read on
		// every machine and the assignment store is one machine's disk, so
		// without this a session sees an open issue it has no lane for and
		// takes work another host is already doing.
		if hasAny(labels, []string{claim.Label}) {
			attrs["claimed"] = "a Mellions lane holds this — read the issue's claim before taking it"
		}
		if a := logins(it.Assignees); a != "" {
			attrs["assignees"] = a
		}
		out = append(out, signal.Signal{
			Kind: kind, Source: "github", ID: "#" + strconv.Itoa(it.Number),
			Title: it.Title, URL: it.URL, Repo: repo,
			Created: it.CreatedAt, Updated: it.UpdatedAt,
			Labels: labels, Attrs: attrs,
		})
	}
	return out, nil
}

func (s *Source) pulls(ctx context.Context, full string, limit int) ([]signal.Signal, error) {
	raw, err := s.opts.Run(ctx, "pr", "list", "-R", full, "--state", "open",
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,url,createdAt,updatedAt,labels,isDraft,mergeable,headRefName,author,body,reviews,comments")
	if err != nil {
		return nil, err
	}
	var items []ghIssue
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("githubsrc: decode pull requests for %s: %w", full, err)
	}

	repo := shortName(full)
	out := make([]signal.Signal, 0, len(items))
	for _, it := range items {
		attrs := map[string]string{"type": "pull_request"}
		if it.IsDraft {
			attrs["draft"] = "true"
		}
		if it.Mergeable != "" {
			attrs["mergeable"] = strings.ToLower(it.Mergeable)
		}
		if it.HeadRef != "" {
			attrs["branch"] = it.HeadRef
		}
		if it.Author.Login != "" {
			attrs["author"] = it.Author.Login
		}
		// Every Mellions session pushes as the same GitHub account, so author
		// separates a bot's PR from an engineer's and nothing finer; which lane
		// a change set belongs to is its branch, not its author.
		attrs["body"] = humanBytes(len(it.Body))
		attrs["reviews"] = strconv.Itoa(len(it.Reviews))
		// A posted comment can carry a review record without incrementing
		// GitHub's submitted-review count. Print both counts because neither is
		// evidence of the other.
		if n := len(it.Comments); n > 0 {
			attrs["comments"] = strconv.Itoa(n)
		}
		// Draft is one bit and it means three things: unfinished, finished but
		// unreviewed, and finished with a review in flight. A peer choosing
		// work merges the third believing it is the second. The claim is the
		// only one of the three that says so without opening the change set.
		if hasAny(it.labels(), []string{claim.Label}) {
			attrs["claimed"] = "a Mellions lane holds this — a review in flight or a fix mid-way; " +
				"read the claim on the pull request before merging it"
		}
		out = append(out, signal.Signal{
			Kind: signal.KindChangeSet, Source: "github", ID: "PR #" + strconv.Itoa(it.Number),
			Title: it.Title, URL: it.URL, Repo: repo,
			Created: it.CreatedAt, Updated: it.UpdatedAt,
			Labels: it.labels(), Attrs: attrs,
		})
	}
	return out, nil
}

type ghRun struct {
	DatabaseID   int64     `json:"databaseId"`
	Name         string    `json:"name"`
	DisplayTitle string    `json:"displayTitle"`
	Conclusion   string    `json:"conclusion"`
	HeadBranch   string    `json:"headBranch"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"createdAt"`
	Event        string    `json:"event"`
}

type ghJobs struct {
	Jobs []struct {
		Steps []struct{} `json:"steps"`
	} `json:"jobs"`
}

// runStarted reports whether any job in a run executed a step.
//
// A run refused before it began — an exhausted spending limit, a suspended
// account, a disabled runner — is reported with conclusion "failure" and no
// steps at all. Read from the run alone that is indistinguishable from a red
// suite, so a reader is told the code is broken when nothing ever ran.
//
// Any error answers true: a jobs list that cannot be read must never
// reclassify a genuine failure as a refusal.
func (s *Source) runStarted(ctx context.Context, full string, id int64) (bool, error) {
	raw, err := s.opts.Run(ctx, "api", fmt.Sprintf("repos/%s/actions/runs/%d/jobs", full, id))
	if err != nil {
		return true, err
	}
	var got ghJobs
	if err := json.Unmarshal(raw, &got); err != nil {
		return true, fmt.Errorf("githubsrc: decode jobs for %s run %d: %w", full, id, err)
	}
	for _, j := range got.Jobs {
		if len(j.Steps) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// failedRuns reports checks that are currently red.
//
// Only the newest run per workflow is kept. A workflow that failed forty times
// overnight is one broken thing, and forty signals would drown every other
// source in a survey that is meant to be read.
//
// A run that was refused before it began is kept and marked rather than
// dropped: it is a real problem, but it is an infrastructure problem, and
// reporting it as a failing check sends readers to look for a defect in code
// that was never compiled. A repository that runs no workflows at all is a
// third thing again, and answers before any run is read.
func (s *Source) failedRuns(ctx context.Context, full string) ([]signal.Signal, error) {
	if !s.actionsEnabled(ctx, full) {
		return s.actionsDisabled(ctx, full)
	}
	raw, err := s.opts.Run(ctx, "run", "list", "-R", full, "--status", "failure", "--limit", "40",
		"--json", "databaseId,name,displayTitle,conclusion,headBranch,url,createdAt,event")
	if err != nil {
		return nil, err
	}
	var items []ghRun
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("githubsrc: decode runs for %s: %w", full, err)
	}

	repo := shortName(full)
	seen := map[string]bool{}
	out := make([]signal.Signal, 0, len(items))
	for _, it := range items {
		// One identity, computed once and used both here and as the signal's
		// ID. Keeping two workflow runs apart locally on name and branch and
		// then handing the survey an ID of the name alone let signal.Dedupe
		// collapse what this loop had just decided were two different things.
		id := it.Name
		if it.HeadBranch != "" {
			id += "@" + it.HeadBranch
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		attrs := map[string]string{"type": "workflow_run", "conclusion": it.Conclusion}
		if it.HeadBranch != "" {
			attrs["branch"] = it.HeadBranch
		}
		if it.Event != "" {
			attrs["event"] = it.Event
		}
		title := it.DisplayTitle
		if started, err := s.runStarted(ctx, full, it.DatabaseID); err == nil && !started {
			attrs["started"] = "false"
			title = "the job never started, nothing ran — " + title
		}
		out = append(out, signal.Signal{
			Kind: signal.KindBuild, Source: "github", ID: id,
			Title: title, URL: it.URL, Repo: repo,
			Created: it.CreatedAt, Updated: it.CreatedAt, Attrs: attrs,
		})
	}
	return out, nil
}

// lastRunSearch is how many runs are read to find the last one that gated
// anything. GitHub keeps running its own managed workflows in a repository
// whose Actions are switched off, and they arrive far more often than the
// repository's own, so the newest run alone is almost never the answer.
const lastRunSearch = 60

// gating keeps only runs of workflows the repository itself defines.
//
// GitHub reports managed workflows with the event "dynamic" and can keep
// running them after repository Actions are disabled. Local workflow files do
// not produce that event, so managed runs are not evidence of a merge gate.
func gating(in []ghRun) []ghRun {
	out := make([]ghRun, 0, len(in))
	for _, r := range in {
		if r.Event != "dynamic" {
			out = append(out, r)
		}
	}
	return out
}

// ghPermissions is the Actions permissions of one repository. Enabled is a
// pointer so a body that does not carry the field reads as unknown rather than
// as false, which is the difference between "this repository is ungated" and
// "something else answered this call".
type ghPermissions struct {
	Enabled *bool `json:"enabled"`
}

// actionsEnabled reports whether the repository runs workflows at all.
//
// Any error, and any answer that does not state the field, answers true. This
// call only ever suppresses check runs, so one that cannot be made or cannot be
// read must leave a genuinely red suite exactly where it was: the cost of
// answering true wrongly is the noise this collector already printed, and the
// cost of answering false wrongly is a hidden failure.
func (s *Source) actionsEnabled(ctx context.Context, full string) bool {
	raw, err := s.opts.Run(ctx, "api", "repos/"+full+"/actions/permissions")
	if err != nil {
		return true
	}
	var got ghPermissions
	if err := json.Unmarshal(raw, &got); err != nil || got.Enabled == nil {
		return true
	}
	return *got.Enabled
}

// actionsDisabled reports a repository that runs no workflows, as one signal.
//
// Every check run still on record in such a repository is frozen at the moment
// Actions was switched off. Listing them tells a reader that twenty things are
// currently breaking; the true statement is that nothing is being gated at all,
// which is worse and leads somewhere else entirely — a lane that expects CI to
// catch it has no gate but its own falsification. The stoppage is the signal
// and the last run is its date, so both travel on one line.
//
// Dropping the frozen runs is not dropping a signal: the one emitted here says
// everything they collectively said, and says the thing they could not.
func (s *Source) actionsDisabled(ctx context.Context, full string) ([]signal.Signal, error) {
	raw, err := s.opts.Run(ctx, "run", "list", "-R", full, "--limit", strconv.Itoa(lastRunSearch),
		"--json", "name,headBranch,conclusion,url,createdAt,event")
	if err != nil {
		return nil, err
	}
	var items []ghRun
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("githubsrc: decode last run for %s: %w", full, err)
	}
	items = gating(items)

	attrs := map[string]string{
		"type":         "actions_permissions",
		"actions":      "disabled",
		"owner_action": "re-enabling Actions is the owner's and has billing consequences",
	}
	sig := signal.Signal{
		Kind: signal.KindBuild, Source: "github", ID: "actions-disabled",
		Repo: shortName(full), Attrs: attrs,
		URL: "https://github.com/" + full + "/settings/actions",
	}
	if len(items) == 0 {
		sig.Title = "GitHub Actions is disabled and no run of a workflow in this repository " +
			"is on record within the last " + strconv.Itoa(lastRunSearch) + " runs — " +
			"nothing observable has gated it"
		return []signal.Signal{sig}, nil
	}
	last := items[0]
	sig.Title = fmt.Sprintf(
		"GitHub Actions is disabled — nothing has gated this repository since the last run on %s; "+
			"every check still on record here is frozen at that date, not currently failing",
		last.CreatedAt.UTC().Format("2006-01-02"))
	// Age is measured from the last run, so the line reads as how long the
	// repository has been ungated rather than as how old a dead check is.
	sig.Created, sig.Updated = last.CreatedAt, last.CreatedAt
	attrs["last_run"] = last.CreatedAt.UTC().Format(time.RFC3339)
	if last.Name != "" {
		attrs["last_run_workflow"] = last.Name
	}
	if last.Conclusion != "" {
		attrs["last_run_conclusion"] = last.Conclusion
	}
	return []signal.Signal{sig}, nil
}

// IssuesDisabled reports the tracker saying a repository has issues turned off.
func IssuesDisabled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "has disabled issues")
}

func hasAny(have, want []string) bool {
	for _, h := range have {
		for _, w := range want {
			if strings.EqualFold(h, w) {
				return true
			}
		}
	}
	return false
}

func logins(in []struct {
	Login string `json:"login"`
}) string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		out = append(out, a.Login)
	}
	return strings.Join(out, ",")
}

func shortName(full string) string {
	if _, name, ok := strings.Cut(full, "/"); ok {
		return name
	}
	return full
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

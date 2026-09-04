// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/prmerge"
)

// mergeLookBudget is the whole answer's budget, not one command's. The hook
// that calls this has a few seconds before the runtime kills it, and a decision
// that arrives after that is no decision, so a slow tracker means silence
// rather than a late deny.
const mergeLookBudget = 6 * time.Second

// compareFileCap is GitHub's own page size for the files in a comparison. At
// the cap the list is not the whole list, and an empty overlap computed from a
// truncated list would be a false clean.
const compareFileCap = 300

// cmdPRMergeCheck reads a PreToolUse payload on stdin and denies a `gh pr
// merge` whose state cannot support the decision. Everything else is silence.
func cmdPRMergeCheck(ctx context.Context, args []string) error {
	fs := newFlagSet("pr-merge-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	payload := readPayload(os.Stdin)
	if len(payload) == 0 {
		guardUsage("pr-merge-check", "It denies a `gh pr merge` whose mergeability GitHub has "+
			"not computed, or whose branch is behind its base in files the pull request also changes.")
		return nil
	}
	// The escape hatch is the same shape cite-check has: a session that has
	// established the merge is right, and said so, is not made to argue with a
	// guard that cannot read its reasoning.
	if strings.EqualFold(os.Getenv("MELLIONS_MERGE_CHECK"), "off") {
		return nil
	}
	reason := prmerge.Deny(payload, func(cwd string, call prmerge.Call) (prmerge.State, error) {
		return mergeState(ctx, cwd, call)
	})
	if reason == "" {
		return nil
	}
	var d decision
	d.Output.Event = "PreToolUse"
	d.Output.Decide = "deny"
	d.Output.Reason = reason
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// mergeState asks the tracker what it says about the pull request a call names.
//
// Two reads, under one budget. The first is the pull request itself; the second
// is the comparison from its head to its base, which is where "behind" and the
// overlap both come from. mergeStateStatus is deliberately not used for behind:
// GitHub only reports BEHIND where branch protection requires the branch to be
// current, so on a repository without that rule the field is silent about a
// branch that is a hundred commits back.
func mergeState(ctx context.Context, cwd string, call prmerge.Call) (prmerge.State, error) {
	ctx, cancel := context.WithTimeout(ctx, mergeLookBudget)
	defer cancel()
	dir := cwd
	if dir == "" {
		dir = "."
	}

	view := []string{"pr", "view"}
	if call.Selector != "" {
		view = append(view, call.Selector)
	}
	if call.Repo != "" {
		view = append(view, "--repo", call.Repo)
	}
	view = append(view, "--json", "number,url,baseRefName,headRefOid,mergeStateStatus,state,files")

	out, err := lookup(ctx, dir, "gh", view...)
	if err != nil {
		return prmerge.State{}, err
	}
	var pr struct {
		Number     int    `json:"number"`
		URL        string `json:"url"`
		Base       string `json:"baseRefName"`
		Head       string `json:"headRefOid"`
		MergeState string `json:"mergeStateStatus"`
		State      string `json:"state"`
		Files      []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return prmerge.State{}, err
	}
	// A pull request already merged or closed is not a merge decision.
	if pr.Number == 0 || !strings.EqualFold(pr.State, "OPEN") {
		return prmerge.State{}, nil
	}

	state := prmerge.State{
		Number:     pr.Number,
		URL:        pr.URL,
		Base:       pr.Base,
		MergeState: pr.MergeState,
	}
	// UNKNOWN is decided without the second read: there is nothing to compare
	// that would change the answer, and the budget is better left unspent.
	if strings.EqualFold(pr.MergeState, "UNKNOWN") {
		return state, nil
	}
	if pr.Head == "" || pr.Base == "" {
		return state, nil
	}

	repo := call.Repo
	if repo == "" {
		if r, err := lookup(ctx, dir, "gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"); err == nil {
			repo = r
		}
	}
	if repo == "" {
		return state, nil
	}

	// head...base is what the base gained since the divergence: its commits the
	// head does not have, and the files they changed. That is the whole input
	// to both "behind" and the overlap.
	cmp, err := lookup(ctx, dir, "gh", "api",
		"repos/"+repo+"/compare/"+pr.Head+"..."+pr.Base,
		"--jq", `{ahead: .ahead_by, files: [.files[]?.filename]}`)
	if err != nil {
		return state, nil
	}
	var comparison struct {
		Ahead int      `json:"ahead"`
		Files []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(cmp), &comparison); err != nil {
		return state, nil
	}
	state.BehindBy = comparison.Ahead
	state.Truncated = len(comparison.Files) >= compareFileCap

	changed := make(map[string]bool, len(pr.Files))
	for _, f := range pr.Files {
		changed[f.Path] = true
	}
	for _, f := range comparison.Files {
		if changed[f] {
			state.Overlap = append(state.Overlap, f)
		}
	}
	return state, nil
}

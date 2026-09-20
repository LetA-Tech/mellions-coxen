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
			"not computed, or whose branch is behind its base in files the pull request also "+
			"changes and whose content differs at the two tips.")
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
	d.Output.Event = preToolUseEvent
	d.Output.Decide = "deny"
	d.Output.Reason = reason
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// look is one read of the tracker. mergeState takes it rather than calling
// lookup, so the decision it assembles can be exercised on answers a test
// writes instead of on a live pull request.
type look func(ctx context.Context, dir, name string, args ...string) (string, error)

// comparedFile is one file in a comparison. sha is the file's blob at the head
// side of that comparison — the side written second in `compare/A...B` — so two
// comparisons run in opposite directions carry the same file's content at each
// of the two tips, and equal shas are the same blob.
type comparedFile struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

// mergeState asks the tracker what it says about the pull request a call names.
func mergeState(ctx context.Context, cwd string, call prmerge.Call) (prmerge.State, error) {
	return mergeStateFrom(ctx, cwd, call, lookup)
}

// mergeStateFrom assembles the state from reads under one budget. The first is
// the pull request itself; the second is the comparison from its head to its
// base, which is where "behind" and the candidate overlap both come from; the
// third narrows that overlap to the files the merge would actually write over.
// mergeStateStatus is deliberately not used for behind: GitHub only reports
// BEHIND where branch protection requires the branch to be current, so on a
// repository without that rule the field is silent about a branch that is a
// hundred commits back.
func mergeStateFrom(ctx context.Context, cwd string, call prmerge.Call, read look) (prmerge.State, error) {
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

	out, err := read(ctx, dir, "gh", view...)
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
		if r, err := read(ctx, dir, "gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"); err == nil {
			repo = r
		}
	}
	if repo == "" {
		return state, nil
	}

	// head...base is what the base gained since the divergence: its commits the
	// head does not have, and the files they changed. That is the whole input
	// to both "behind" and the overlap.
	cmp, err := read(ctx, dir, "gh", "api",
		"repos/"+repo+"/compare/"+pr.Head+"..."+pr.Base,
		"--jq", `{ahead: .ahead_by, files: [.files[]? | {name: .filename, sha: .sha}]}`)
	if err != nil {
		return state, nil
	}
	var comparison struct {
		Ahead int            `json:"ahead"`
		Files []comparedFile `json:"files"`
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
	var named []string
	for _, f := range comparison.Files {
		if changed[f.Name] {
			named = append(named, f.Name)
		}
	}
	state.Overlap = overwritten(ctx, dir, read, repo, pr.Base, pr.Head, named, comparison.Files)
	return state, nil
}

// overwritten narrows the files both sides changed since the divergence to the
// files a merge would write over. Both sides changing a file does not mean the
// two tips disagree about it: promoting one branch to another by copying its
// commits leaves every copied file named on both sides and identical at both
// tips, and identical content cannot be written over.
//
// atBase is the head...base comparison already read, so its per-file sha is the
// file's blob at the base tip. The read here runs the comparison the other way,
// giving the same file's blob at the head tip.
//
// A file whose content at either tip this cannot establish stays in the overlap:
// the read failing, the file list truncated at GitHub's page size, a sha absent.
// A clean answer computed from a gap is the one answer a merge guard must not
// give, and the cost of keeping a file is a refusal the session can read and
// argue with.
func overwritten(ctx context.Context, dir string, read look, repo, base, head string, named []string, atBase []comparedFile) []string {
	if len(named) == 0 {
		return nil
	}
	out, err := read(ctx, dir, "gh", "api",
		"repos/"+repo+"/compare/"+base+"..."+head,
		"--jq", `[.files[]? | {name: .filename, sha: .sha}]`)
	if err != nil {
		return named
	}
	var atHead []comparedFile
	if err := json.Unmarshal([]byte(out), &atHead); err != nil {
		return named
	}
	headSHA := blobs(atHead)
	baseSHA := blobs(atBase)
	var kept []string
	for _, f := range named {
		h, atH := headSHA[f]
		b, atB := baseSHA[f]
		if atH && atB && h != "" && h == b {
			continue
		}
		kept = append(kept, f)
	}
	return kept
}

// blobs indexes a comparison's files by path, keeping the first sha where a
// path is named twice, so the index does not depend on the order the answer
// arrived in.
func blobs(files []comparedFile) map[string]string {
	out := make(map[string]string, len(files))
	for _, f := range files {
		if _, seen := out[f.Name]; !seen {
			out[f.Name] = f.SHA
		}
	}
	return out
}

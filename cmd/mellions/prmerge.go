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

// mergeLookBudget is the whole answer's budget, not one command's: up to four
// reads of the tracker share it. It is not what bounds a hooked run — the hook
// is declared with a 5s timeout in hooks/hooks.json, under this, so the runtime
// kills the hook first and a tracker slower than that is silence rather than a
// late deny. What this bounds is a run with no runtime over it, and the reads
// after a slow one. A read that fails rather than expires is answered: any
// read up to the base's side of the comparison failing is silence, nothing
// being established to refuse on, and the read of the pull request's own diff
// failing refuses, saying that side was not read.
const mergeLookBudget = 6 * time.Second

// compareFileCap is GitHub's own page size for the files in a comparison. At
// the cap the list is not the whole list, and an empty overlap computed from a
// truncated list would be a false clean.
const compareFileCap = 300

// removedStatus is GitHub's status for a file the head side of a comparison no
// longer has. It is the one status whose sha is not a tip's blob.
const removedStatus = "removed"

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
			"changes and whose content differs at the two tips, or in changed files while the "+
			"pull request's own changes could not be read whole.")
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
// side — the side written second in `compare/A...B` — for every status but
// `removed`, where it is the pre-image instead: the blob the file had before
// the deletion, which is the merge base's, not either tip's. Verified against
// the blobs themselves for modified, added, renamed and removed.
//
// So two comparisons run in opposite directions carry the same file's content
// at each of the two tips, and equal shas are the same blob — everywhere the
// status is not `removed`, which is why the status is read alongside the sha.
type comparedFile struct {
	Name   string `json:"name"`
	SHA    string `json:"sha"`
	Status string `json:"status"`
}

// mergeState asks the tracker what it says about the pull request a call names.
func mergeState(ctx context.Context, cwd string, call prmerge.Call) (prmerge.State, error) {
	return mergeStateFrom(ctx, cwd, call, lookup)
}

// mergeStateFrom assembles the state from reads under one budget. The first is
// the pull request itself; the second is the comparison from its head to its
// base, which is where "behind" and the base's side of the overlap come from;
// the third is the comparison the other way, the pull request's own diff from
// the merge base, which is the only source of its side. The file list the
// tracker keeps for a pull request is not read: a file it names that the head
// never changed since the merge base is one the merge takes from the base.
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
	view = append(view, "--json", "number,url,baseRefName,headRefOid,mergeStateStatus,state")

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
	// head does not have, and the files they changed, each as the base tip has
	// it. Unread, nothing is known about the branch and the guard stays silent.
	atBase, ok := compared(ctx, dir, read, repo, pr.Head, pr.Base)
	if !ok {
		return state, nil
	}
	state.BehindBy = atBase.Ahead
	// A comparison at the page size refuses on its own, and no read of the
	// other side could clear it, so the third read is not spent on it.
	if len(atBase.Files) >= compareFileCap {
		state.Truncated = true
		return state, nil
	}
	if len(atBase.Files) == 0 {
		return state, nil
	}

	// base...head is the pull request's own diff: the files that differ between
	// the merge base and the head, each as the head tip has it. A gap here is
	// not a clean answer, and unread refuses as unread.
	atHead, ok := compared(ctx, dir, read, repo, pr.Base, pr.Head)
	if !ok {
		state.Unread = true
		return state, nil
	}
	overlap, absent := overwritten(atBase.Files, atHead.Files)
	// A base-side file the diff does not name is one the head never changed,
	// where the diff is whole. At the page size it is not whole, and such a
	// file is established neither way. Files the diff does name are decided at
	// any size.
	if absent > 0 && len(atHead.Files) >= compareFileCap {
		state.Truncated = true
		return state, nil
	}
	state.Overlap = overlap
	return state, nil
}

// comparison is one `compare/A...B`: how many commits B has that A does not,
// and the files that differ between their merge base and B.
type comparison struct {
	Ahead int
	Files []comparedFile
}

// compared reads `compare/from...to`. It answers false where the read failed,
// the answer does not parse, or the answer carries no commit count or no file
// list: an answer that is not a comparison would otherwise read as a side that
// changed nothing.
func compared(ctx context.Context, dir string, read look, repo, from, to string) (comparison, bool) {
	out, err := read(ctx, dir, "gh", "api",
		"repos/"+repo+"/compare/"+from+"..."+to,
		"--jq", `{ahead: .ahead_by, files: (if (.files | type) == "array" `+
			`then [.files[] | {name: .filename, sha: .sha, status: .status}] else null end)}`)
	if err != nil {
		return comparison{}, false
	}
	var answer struct {
		Ahead *int            `json:"ahead"`
		Files *[]comparedFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &answer); err != nil || answer.Ahead == nil || answer.Files == nil {
		return comparison{}, false
	}
	return comparison{Ahead: *answer.Ahead, Files: *answer.Files}, true
}

// overwritten is the files a merge would write over: those both sides changed
// since the divergence whose content differs at the two tips. atBase carries
// each file as the base tip has it and atHead as the head tip has it. absent
// counts the base-side files atHead does not name.
//
// A file a whole atHead does not name is one the head never changed since the
// merge base, so the merge takes the base's and nothing is written over. Both sides
// naming a file does not mean the tips disagree about it either: promoting one
// branch to another by copying its commits leaves every copied file named on
// both sides and identical at both tips.
//
// A file both sides name whose content at either tip is not established stays
// in the overlap: a sha absent, a deletion on one side. A clean answer computed
// from a gap is the one answer a merge guard must not give, and the cost of
// keeping a file is a refusal the session can read and argue with.
func overwritten(atBase, atHead []comparedFile) (kept []string, absent int) {
	atH := index(atHead)
	atB := index(atBase)
	for _, f := range atBase {
		h, own := atH[f.Name]
		if !own {
			absent++
			continue
		}
		if agree(atB[f.Name], h) {
			continue
		}
		kept = append(kept, f.Name)
	}
	return kept, absent
}

// agree says the two tips hold the same thing for one file.
//
// A deletion's sha is the pre-image — the blob the file had before it was
// removed, which is the merge base's and neither tip's — so equal shas across a
// deletion are agreement about the past, not about now. Removed on both sides
// is the only agreement a deletion carries: both tips lack the file. Removed on
// one side and the tips differ by the whole file, whatever the shas say.
//
// An empty sha agrees with nothing, another empty sha included.
func agree(b, h comparedFile) bool {
	if b.Status == removedStatus || h.Status == removedStatus {
		return b.Status == removedStatus && h.Status == removedStatus
	}
	return b.SHA != "" && b.SHA == h.SHA
}

// index maps a comparison's files by path, keeping the first entry where a path
// is named twice, so the index does not depend on the order the answer arrived
// in.
func index(files []comparedFile) map[string]comparedFile {
	out := make(map[string]comparedFile, len(files))
	for _, f := range files {
		if _, seen := out[f.Name]; !seen {
			out[f.Name] = f
		}
	}
	return out
}

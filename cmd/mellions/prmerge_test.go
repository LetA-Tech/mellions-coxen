// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/prmerge"
)

// A promotion copies one branch's commits onto the other, so both sides name
// the same files while the two tips hold the same bytes. The overlap the
// refusal is built on is the files a merge writes over, and there are none.
func TestMergeStateOverlapIsContentNotNames(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "main"
	)
	prView := `{"number":97,"url":"https://github.com/` + repo + `/pull/97",` +
		`"baseRefName":"` + base + `","headRefOid":"` + head + `",` +
		`"mergeStateStatus":"CLEAN","state":"OPEN",` +
		`"files":[{"path":"a.go"},{"path":"b.go"},{"path":"only-head.go"}]}`

	// head...base: what the base gained since the divergence, each file's blob
	// at the base tip.
	atBase := `{"ahead":3,"files":[` +
		`{"name":"a.go","sha":"aaa"},{"name":"b.go","sha":"bbb"},{"name":"only-base.go","sha":"ccc"}]}`

	for _, tc := range []struct {
		name       string
		atBase     string // head...base, defaulting to atBase above
		atHead     string // base...head's files, or "" to make that read fail
		want       []string
		wantUnread bool
	}{
		{
			name:   "converged tips are not an overlap",
			atHead: `[{"name":"a.go","sha":"aaa"},{"name":"b.go","sha":"bbb"},{"name":"only-head.go","sha":"ddd"}]`,
			want:   nil,
		},
		{
			name:   "a file that differs is the whole overlap",
			atHead: `[{"name":"a.go","sha":"aaa"},{"name":"b.go","sha":"zzz"},{"name":"only-head.go","sha":"ddd"}]`,
			want:   []string{"b.go"},
		},
		{
			// The pull request's file list names b.go, the base changed it,
			// and the tips differ: the base tip holds bbb and the head tip
			// holds the merge base's blob. The pull request's own diff does
			// not name it, so the merge takes the base's and writes over
			// nothing.
			name:   "a file only the pull request's file list names is not an overlap",
			atHead: `[{"name":"a.go","sha":"aaa"}]`,
			want:   nil,
		},
		{
			name:   "an empty sha establishes nothing and is kept",
			atHead: `[{"name":"a.go","sha":""},{"name":"b.go","sha":"bbb"}]`,
			want:   []string{"a.go"},
		},
		{
			// An empty sha is not a matching sha. The base side always
			// has an entry — named is built from that same list — so
			// what this reaches is the head side being empty while the
			// base side is too, which without the guard compares equal
			// and clears the file on no evidence.
			name:   "an empty sha on the head side is not agreement",
			atBase: `{"ahead":3,"files":[{"name":"a.go","sha":""},{"name":"b.go","sha":"bbb"}]}`,
			atHead: `[{"name":"a.go","sha":""},{"name":"b.go","sha":"bbb"}]`,
			want:   []string{"a.go"},
		},
		{
			// A deletion's sha is the pre-image, so both sides carrying the
			// same sha for a removed file says they agree about the blob
			// before the deletion. What makes this agreement is the deletion
			// being on both sides: neither tip has the file.
			name: "a file both sides removed is not an overlap",
			atBase: `{"ahead":3,"files":[{"name":"a.go","sha":"aaa","status":"modified"},` +
				`{"name":"b.go","sha":"bbb","status":"removed"}]}`,
			atHead: `[{"name":"a.go","sha":"aaa","status":"modified"},` +
				`{"name":"b.go","sha":"bbb","status":"removed"}]`,
			want: nil,
		},
		{
			// The case the independent read found: the base deletes the file,
			// the head leaves its content at the merge base (a mode-only
			// change), so the pre-image sha on one side equals the unchanged
			// blob on the other while the tips differ by the whole file. Git
			// refuses this as a modify/delete before the guard is consulted;
			// the decision no longer rests on git doing so.
			name: "a file removed on one side only is kept",
			atBase: `{"ahead":3,"files":[{"name":"a.go","sha":"aaa","status":"modified"},` +
				`{"name":"b.go","sha":"bbb","status":"removed"}]}`,
			atHead: `[{"name":"a.go","sha":"aaa","status":"modified"},` +
				`{"name":"b.go","sha":"bbb","status":"modified"}]`,
			want: []string{"b.go"},
		},
		{
			// Both in one answer: a.go is in the pull request's own diff and
			// differs, b.go is only in its file list.
			name:   "a differing file is named beside a list-only file that is not",
			atHead: `[{"name":"a.go","sha":"zzz"}]`,
			want:   []string{"a.go"},
		},
		{
			name:       "the read failing names nothing and is not a clean answer",
			atHead:     "",
			want:       nil,
			wantUnread: true,
		},
		{
			// An answer with no commit count is not a comparison. Its file
			// list is empty for that reason, not because the head changed
			// nothing.
			name:       "an answer that is not a comparison is unread",
			atHead:     notAComparison,
			want:       nil,
			wantUnread: true,
		},
		{
			// A commit count and no file list is not a diff that changed
			// nothing.
			name:       "an answer with no file list is unread",
			atHead:     "null",
			want:       nil,
			wantUnread: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base3 := atBase
			if tc.atBase != "" {
				base3 = tc.atBase
			}
			state, err := mergeStateFrom(context.Background(), t.TempDir(),
				prmerge.Call{Selector: "97", Repo: repo},
				stubLook(t, prView, base3, tc.atHead, head, base, repo))
			if err != nil {
				t.Fatalf("mergeStateFrom: %v", err)
			}
			if state.BehindBy != 3 {
				t.Errorf("BehindBy = %d, want 3", state.BehindBy)
			}
			if !sameSet(state.Overlap, tc.want) {
				t.Errorf("Overlap = %v, want %v", state.Overlap, tc.want)
			}
			if state.Unread != tc.wantUnread {
				t.Errorf("Unread = %v, want %v", state.Unread, tc.wantUnread)
			}
			if state.Truncated {
				t.Errorf("Truncated on comparisons far under the page size")
			}
		})
	}
}

// The decision, not the field: a converged promotion reaches `gh pr merge`
// without a refusal, a differing file still refuses and names that file, a
// file only the pull request's file list names is not refused, and a read of
// the pull request's own diff that fails is refused without naming a file.
func TestPRMergeDecisionOnConvergedAndDifferingTips(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "main"
	)
	prView := `{"number":97,"url":"https://github.com/` + repo + `/pull/97",` +
		`"baseRefName":"` + base + `","headRefOid":"` + head + `",` +
		`"mergeStateStatus":"CLEAN","state":"OPEN",` +
		`"files":[{"path":"a.go"},{"path":"b.go"}]}`
	atBase := `{"ahead":28,"files":[{"name":"a.go","sha":"aaa"},{"name":"b.go","sha":"bbb"}]}`

	payload, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"cwd":        t.TempDir(),
		"tool_input": map[string]string{"command": "gh pr merge 97 --repo " + repo + " --merge"},
	})
	if err != nil {
		t.Fatal(err)
	}

	decide := func(atHead string) string {
		read := stubLook(t, prView, atBase, atHead, head, base, repo)
		return prmerge.Deny(payload, func(cwd string, call prmerge.Call) (prmerge.State, error) {
			return mergeStateFrom(context.Background(), cwd, call, read)
		})
	}

	converged := `[{"name":"a.go","sha":"aaa"},{"name":"b.go","sha":"bbb"}]`
	if reason := decide(converged); reason != "" {
		t.Errorf("converged tips were refused:\n%s", reason)
	}

	differing := `[{"name":"a.go","sha":"aaa"},{"name":"b.go","sha":"zzz"}]`
	reason := decide(differing)
	if reason == "" {
		t.Fatal("a file that differs at the two tips was not refused")
	}
	if !strings.Contains(reason, "b.go") {
		t.Errorf("refusal does not name b.go:\n%s", reason)
	}
	if strings.Contains(reason, "a.go") {
		t.Errorf("refusal names a.go, which is identical at both tips:\n%s", reason)
	}
	if !strings.Contains(reason, "1 file") {
		t.Errorf("refusal does not count 1 file:\n%s", reason)
	}

	// The file list above names b.go and the base changed it; the head did not.
	listOnly := `[{"name":"a.go","sha":"aaa"}]`
	if reason := decide(listOnly); reason != "" {
		t.Errorf("a file the head never changed since the merge base was refused:\n%s", reason)
	}

	reason = decide("")
	if reason == "" {
		t.Fatal("an unread pull-request side was read as no overlap")
	}
	if !strings.Contains(reason, "could not be established") {
		t.Errorf("the refusal claims more than it established:\n%s", reason)
	}
	if strings.Contains(reason, "a.go") || strings.Contains(reason, "b.go") {
		t.Errorf("the refusal names a file nothing established:\n%s", reason)
	}
}

// notAComparison is an answer to the base...head read that parses and carries
// no commit count.
const notAComparison = "not-a-comparison"

// stubLook answers the three reads mergeStateFrom makes, and fails the test on
// any call it does not recognise, so a read that moves is not silently served.
// atHead is the files of the base...head comparison; empty makes that read
// fail, and notAComparison answers it with no commit count.
func stubLook(t *testing.T, prView, atBase, atHead, head, base, repo string) look {
	t.Helper()
	return func(_ context.Context, _, name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return prView, nil
		case strings.Contains(joined, "compare/"+head+"..."+base):
			return atBase, nil
		case strings.Contains(joined, "compare/"+base+"..."+head):
			switch atHead {
			case "":
				return "", context.DeadlineExceeded
			case notAComparison:
				return `{"ahead":null,"files":[]}`, nil
			}
			return `{"ahead":2,"files":` + atHead + `}`, nil
		}
		t.Errorf("unexpected read: %s %s", name, joined)
		return "", context.Canceled
	}
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, g := range got {
		seen[g]++
	}
	for _, w := range want {
		seen[w]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// A comparison GitHub truncated at its page size is refused on the truncation
// itself, and no read of the other side could clear it. The three reads share
// one budget, so the read that cannot change the answer is not made at all,
// and with it unmade no file is named.
func TestMergeStateTruncatedDoesNotSpendTheNarrowingRead(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "main"
	)
	var cmpFiles []string
	for i := 0; i < compareFileCap; i++ {
		name := fmt.Sprintf("f%03d.go", i)
		cmpFiles = append(cmpFiles, `{"name":"`+name+`","sha":"s`+strconv.Itoa(i)+`","status":"modified"}`)
	}
	prView := `{"number":97,"url":"u","baseRefName":"` + base + `","headRefOid":"` + head + `",` +
		`"mergeStateStatus":"CLEAN","state":"OPEN"}`
	atBase := `{"ahead":3,"files":[` + strings.Join(cmpFiles, ",") + `]}`

	read := func(_ context.Context, _, name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return prView, nil
		case strings.Contains(joined, "compare/"+head+"..."+base):
			return atBase, nil
		case strings.Contains(joined, "compare/"+base+"..."+head):
			t.Error("the narrowing read was made on a comparison that is not the whole list")
			return "[]", nil
		}
		t.Errorf("unexpected read: %s %s", name, joined)
		return "", context.Canceled
	}

	state, err := mergeStateFrom(context.Background(), t.TempDir(),
		prmerge.Call{Selector: "97", Repo: repo}, read)
	if err != nil {
		t.Fatalf("mergeStateFrom: %v", err)
	}
	if !state.Truncated {
		t.Fatalf("a comparison at the page size is not marked truncated")
	}
	if len(state.Overlap) != 0 {
		t.Errorf("Overlap names %d files with the pull request's side unread", len(state.Overlap))
	}
}

// The pull request's own diff at the page size is not the whole diff, so a
// base-side file it does not name is not established as untouched: that is
// refused as a comparison that cannot be enumerated, not read as clean. One
// file under the page size the list is whole, and the same file is cleared. A
// base-side file the list does name is decided at any size.
func TestMergeStateOwnDiffAtThePageSize(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "main"
	)
	prView := `{"number":97,"url":"u","baseRefName":"` + base + `","headRefOid":"` + head + `",` +
		`"mergeStateStatus":"CLEAN","state":"OPEN"}`

	for _, tc := range []struct {
		name          string
		files         int
		baseFile      string // the one file the base changed, as name and sha
		wantTruncated bool
		wantOverlap   []string
	}{
		{"at the page size, a base file it does not name", compareFileCap,
			`{"name":"beyond-the-page.go","sha":"bbb","status":"modified"}`, true, nil},
		{"one under the page size, a base file it does not name", compareFileCap - 1,
			`{"name":"beyond-the-page.go","sha":"bbb","status":"modified"}`, false, nil},
		{"at the page size, a base file it names with the same blob", compareFileCap,
			`{"name":"f000.go","sha":"s0","status":"modified"}`, false, nil},
		{"at the page size, a base file it names with another blob", compareFileCap,
			`{"name":"f000.go","sha":"other","status":"modified"}`, false, []string{"f000.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var own []string
			for i := 0; i < tc.files; i++ {
				own = append(own, `{"name":"`+fmt.Sprintf("f%03d.go", i)+`","sha":"s`+strconv.Itoa(i)+`","status":"modified"}`)
			}
			state, err := mergeStateFrom(context.Background(), t.TempDir(),
				prmerge.Call{Selector: "97", Repo: repo},
				stubLook(t, prView, `{"ahead":3,"files":[`+tc.baseFile+`]}`,
					"["+strings.Join(own, ",")+"]", head, base, repo))
			if err != nil {
				t.Fatalf("mergeStateFrom: %v", err)
			}
			if state.Truncated != tc.wantTruncated {
				t.Errorf("Truncated = %v with %d files in the pull request's diff, want %v",
					state.Truncated, tc.files, tc.wantTruncated)
			}
			if !sameSet(state.Overlap, tc.wantOverlap) {
				t.Errorf("Overlap = %v, want %v", state.Overlap, tc.wantOverlap)
			}
			payload, err := json.Marshal(map[string]any{
				"tool_name":  "Bash",
				"cwd":        t.TempDir(),
				"tool_input": map[string]string{"command": "gh pr merge 97 --repo " + repo},
			})
			if err != nil {
				t.Fatal(err)
			}
			reason := prmerge.Deny(payload, func(string, prmerge.Call) (prmerge.State, error) { return state, nil })
			if refused := strings.Contains(reason, "could not be established"); refused != tc.wantTruncated {
				t.Errorf("refused as not established = %v, want %v: %q", refused, tc.wantTruncated, reason)
			}
			if named := strings.Contains(reason, "f000.go"); named != (len(tc.wantOverlap) > 0) {
				t.Errorf("refusal names f000.go = %v, want %v: %q", named, len(tc.wantOverlap) > 0, reason)
			}
			if !tc.wantTruncated && len(tc.wantOverlap) == 0 && reason != "" {
				t.Errorf("a merge that writes over nothing was refused: %q", reason)
			}
		})
	}
}

// A base that gained commits and changed no file has no side of an overlap, so
// the read of the pull request's own diff cannot change the answer and is not
// made.
func TestMergeStateBaseChangingNoFileDoesNotSpendTheThirdRead(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "main"
	)
	prView := `{"number":97,"url":"u","baseRefName":"` + base + `","headRefOid":"` + head + `",` +
		`"mergeStateStatus":"CLEAN","state":"OPEN"}`
	read := func(_ context.Context, _, name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return prView, nil
		case strings.Contains(joined, "compare/"+head+"..."+base):
			return `{"ahead":60,"files":[]}`, nil
		case strings.Contains(joined, "compare/"+base+"..."+head):
			t.Error("the pull request's own diff was read with no base-side file to compare it to")
			return `{"ahead":1,"files":[]}`, nil
		}
		t.Errorf("unexpected read: %s %s", name, joined)
		return "", context.Canceled
	}
	state, err := mergeStateFrom(context.Background(), t.TempDir(),
		prmerge.Call{Selector: "97", Repo: repo}, read)
	if err != nil {
		t.Fatalf("mergeStateFrom: %v", err)
	}
	if state.BehindBy != 60 || state.Unread || state.Truncated || len(state.Overlap) != 0 {
		t.Errorf("state = %+v, want 60 behind and nothing else", state)
	}
}

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
		name   string
		atBase string // head...base, defaulting to atBase above
		atHead string // base...head, or "" to make that read fail
		want   []string
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
			name:   "a file the head side does not name is kept",
			atHead: `[{"name":"a.go","sha":"aaa"}]`,
			want:   []string{"b.go"},
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
			name:   "the read failing keeps every name",
			atHead: "",
			want:   []string{"a.go", "b.go"},
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
		})
	}
}

// The decision, not the field: a converged promotion reaches `gh pr merge`
// without a refusal, and a differing file still refuses and names that file.
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
}

// stubLook answers the three reads mergeStateFrom makes, and fails the test on
// any call it does not recognise, so a read that moves is not silently served.
// An empty atHead makes the narrowing read fail.
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
			if atHead == "" {
				return "", context.DeadlineExceeded
			}
			return atHead, nil
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
// itself, and an overlap narrowed against a list that is not the whole list
// would establish nothing either way. The three reads share one budget, so the
// read that cannot change the answer is not made at all.
func TestMergeStateTruncatedDoesNotSpendTheNarrowingRead(t *testing.T) {
	const (
		repo = "LetA-Tech/mellions-coxen"
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "main"
	)
	var prFiles, cmpFiles []string
	for i := 0; i < compareFileCap; i++ {
		name := fmt.Sprintf("f%03d.go", i)
		prFiles = append(prFiles, `{"path":"`+name+`"}`)
		cmpFiles = append(cmpFiles, `{"name":"`+name+`","sha":"s`+strconv.Itoa(i)+`","status":"modified"}`)
	}
	prView := `{"number":97,"url":"u","baseRefName":"` + base + `","headRefOid":"` + head + `",` +
		`"mergeStateStatus":"CLEAN","state":"OPEN","files":[` + strings.Join(prFiles, ",") + `]}`
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
	if len(state.Overlap) != compareFileCap {
		t.Errorf("Overlap holds %d files, want every one of the %d named",
			len(state.Overlap), compareFileCap)
	}
}

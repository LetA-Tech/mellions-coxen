package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type fakeGH struct {
	prs    map[string][]map[string]any // repo -> rows, newest first
	issues map[string][]map[string]any
	calls  []string
}

func (f *fakeGH) run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	arg := func(name string) string {
		for i, a := range args {
			if a == name && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	repo := arg("--repo")
	limit := 0
	fmt.Sscan(arg("--limit"), &limit)
	switch {
	case args[0] == "pr" && args[1] == "list" && arg("--head") != "":
		var b strings.Builder
		for _, r := range f.prs[repo] {
			if r["headRefName"] == arg("--head") {
				fmt.Fprintf(&b, "#%v %v", r["number"], r["state"])
				if r["isDraft"] == true {
					b.WriteString(" (draft)")
				}
			}
		}
		return []byte(b.String() + "\n"), nil
	case args[0] == "pr" && args[1] == "list":
		rows := f.prs[repo]
		if len(rows) > limit {
			rows = rows[:limit]
		}
		return json.Marshal(rows)
	case args[0] == "issue" && args[1] == "list":
		rows := f.issues[repo]
		if len(rows) > limit {
			rows = rows[:limit]
		}
		return json.Marshal(rows)
	case args[0] == "issue" && args[1] == "view":
		for _, r := range f.issues[repo] {
			if fmt.Sprint(r["number"]) == args[2] {
				return []byte(fmt.Sprint(r["state"]) + "\n"), nil
			}
		}
		return nil, fmt.Errorf("not found")
	}
	return nil, fmt.Errorf("unexpected gh %v", args)
}

func pr(n int, state, head string, draft bool) map[string]any {
	return map[string]any{"number": n, "state": state, "isDraft": draft, "headRefName": head}
}

func iss(n int, state string) map[string]any {
	return map[string]any{"number": n, "state": state}
}

func TestTrackerAsksOncePerRepositoryNotPerLane(t *testing.T) {
	f := &fakeGH{
		prs: map[string][]map[string]any{
			"o/a": {pr(9, "OPEN", "lane-1", true), pr(7, "MERGED", "lane-2", false), pr(5, "CLOSED", "lane-2", false)},
			"o/b": {pr(3, "OPEN", "lane-4", false)},
		},
		issues: map[string][]map[string]any{
			"o/a": {iss(8, "OPEN"), iss(6, "CLOSED")},
			"o/b": {iss(2, "OPEN")},
		},
	}
	tr := newGHTracker(context.Background(), "o", 100, f.run)
	ctx := context.Background()
	want := []struct{ repo, branch, pr, issue, state string }{
		{"a", "lane-1", "#9 OPEN (draft)", "#8", "OPEN"},
		{"a", "lane-2", "#7 MERGED#5 CLOSED", "#6", "CLOSED"},
		{"a", "lane-3", "none for this branch", "#8", "OPEN"},
		{"b", "lane-4", "#3 OPEN", "#2", "OPEN"},
		{"b", "lane-5", "none for this branch", "#2", "OPEN"},
	}
	for _, w := range want {
		if got, known := tr.PullRequest(ctx, w.repo, w.branch); !known || got != w.pr {
			t.Errorf("PullRequest(%s, %s) = %q, %v; want %q, true", w.repo, w.branch, got, known, w.pr)
		}
		if got, known := tr.Issue(ctx, w.repo, w.issue); !known || got != w.state {
			t.Errorf("Issue(%s, %s) = %q, %v; want %q, true", w.repo, w.issue, got, known, w.state)
		}
	}
	if len(f.calls) != 4 {
		t.Fatalf("5 lanes in 2 repositories made %d tracker calls, want 4 (one pr list and one issue list per repository):\n%s",
			len(f.calls), strings.Join(f.calls, "\n"))
	}
}

func TestTrackerNeverReadsAbsenceFromAFullListAsNone(t *testing.T) {
	f := &fakeGH{
		prs: map[string][]map[string]any{
			"o/a": {pr(9, "OPEN", "new", false), pr(8, "OPEN", "newer", false), pr(4, "MERGED", "old", false)},
		},
		issues: map[string][]map[string]any{
			"o/a": {iss(7, "OPEN"), iss(6, "OPEN"), iss(1, "CLOSED")},
		},
	}
	tr := newGHTracker(context.Background(), "o", 2, f.run)
	ctx := context.Background()
	if got, known := tr.PullRequest(ctx, "a", "old"); !known || got != "#4 MERGED" {
		t.Errorf("a branch past a full list = %q, %v; want the per-branch answer %q", got, known, "#4 MERGED")
	}
	if got, known := tr.PullRequest(ctx, "a", "never"); !known || got != "none for this branch" {
		t.Errorf("a branch with no pull request = %q, %v; want none, asked per branch", got, known)
	}
	if got, known := tr.Issue(ctx, "a", "#1"); !known || got != "CLOSED" {
		t.Errorf("an issue past a full list = %q, %v; want CLOSED from the per-issue question", got, known)
	}
	perItem := 0
	for _, c := range f.calls {
		if strings.Contains(c, "--head") || strings.HasPrefix(c, "issue view") {
			perItem++
		}
	}
	if perItem != 3 {
		t.Errorf("items a full list does not hold made %d per-item calls, want 3:\n%s", perItem, strings.Join(f.calls, "\n"))
	}
}

func TestTrackerFallsBackWhenTheListFails(t *testing.T) {
	calls := 0
	tr := newGHTracker(context.Background(), "o", 100, func(_ context.Context, args ...string) ([]byte, error) {
		calls++
		return nil, fmt.Errorf("HTTP 504")
	})
	if got, known := tr.PullRequest(context.Background(), "a", "lane"); known {
		t.Errorf("an unreachable tracker answered %q as known; it must stay unknown", got)
	}
	if got, known := tr.Issue(context.Background(), "a", "#1"); known {
		t.Errorf("an unreachable tracker answered issue %q as known; it must stay unknown", got)
	}
	if calls != 4 {
		t.Errorf("a failed list made %d calls, want 4 (each list, then each per-item question)", calls)
	}
}

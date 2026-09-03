// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package continuity

import (
	"context"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/assignment"
)

// A missing worktree is a fact about the worktree, not the end of the reading.
//
// This is the ordinary disruption — a wiped machine, a cleared /tmp, a restarted
// container, somebody's prune — and it used to produce one line saying "absent"
// and nothing else, while the branch, its commits and git's own record of the
// lane sat intact in the repository the assignment names. A session handed that
// slate reads total loss where nothing was lost, and either redoes the work or
// writes the lane off.
func TestAnAbsentWorktreeStillReadsTheSourceRepository(t *testing.T) {
	a := &assignment.Assignment{
		ID: "rec-1", Repo: "svc",
		Branch:   "mellions/rec-1",
		Worktree: "/nowhere/assignments/rec-1/tree",
		Source:   "/repos/svc",
		Base:     "aaaaaaaaaaaa",
	}

	git := func(dir string, args ...string) ([]byte, error) {
		if dir != a.Source {
			t.Errorf("read %q; the only repository left to read is the source", dir)
		}
		switch {
		case args[0] == "rev-parse" && len(args) > 3 && strings.HasPrefix(args[3], "refs/heads/"):
			return []byte("6e9198c6e78c1111111111111111111111111111\n"), nil
		case args[0] == "rev-list" && args[1] == "--count":
			return []byte("3\n"), nil
		case args[0] == "worktree":
			return []byte("worktree /repos/svc\n\nworktree " + a.Worktree + "\nprunable gone\n"), nil
		}
		return nil, errNoUpstream{}
	}

	o := Look(context.Background(), a, git, Tracker{})
	joined := ""
	for _, f := range o.Facts {
		joined += f.Name + " = " + f.Value + "\n"
	}

	for _, want := range []string{
		"absent",              // the worktree itself, still reported
		"still in /repos/svc", // and the branch that outlived it
		"6e9198c6e78c",        // its tip
		"3",                   // how much work is on it
		"worktree add",        // and how to get it back
		// and whether it is published. Every `rev-list --count` here answers 3,
		// so all three commits are on no remote and the lane is unpublished.
		"no remote-tracking ref",
		"prunable",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the reading never mentions %q:\n%s", want, joined)
		}
	}
}

// When the source cannot be read either, that is unknown rather than absent.
func TestAnUnreadableSourceIsUnknownRatherThanLoss(t *testing.T) {
	a := &assignment.Assignment{
		ID: "rec-2", Branch: "mellions/rec-2",
		Worktree: "/nowhere/tree", Source: "/repos/gone", Base: "aaaa",
	}
	git := func(string, ...string) ([]byte, error) { return nil, errNoUpstream{} }

	o := Look(context.Background(), a, git, Tracker{})
	all := strings.Join(o.Unestablished, "\n")
	if !strings.Contains(all, "unknown rather than answered") {
		t.Fatalf("an unreadable source was not reported as unknown:\n%s", all)
	}
	for _, f := range o.Facts {
		if strings.Contains(f.Value, "gone from") {
			t.Fatalf("an unreadable source was reported as the branch being gone: %q", f.Value)
		}
	}
}

type errNoUpstream struct{}

func (errNoUpstream) Error() string { return "fatal: no upstream configured" }

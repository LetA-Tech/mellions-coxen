// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"strings"
	"testing"
)

// TestAGitFailureReportsWhyRatherThanItsProgress.
//
// git writes progress before it fails. Taking the first line of the combined
// output therefore reported "Preparing worktree (new branch 'x')" as the error
// for a branch that already existed — which is not an error at all, and sends
// whoever reads it looking for a problem that is not there.
//
// Found by reopening an assignment whose branch had outlived it.
func TestAGitFailureReportsWhyRatherThanItsProgress(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
		want string
		gone string
	}{
		{
			name: "worktree add over an existing branch",
			out: "Preparing worktree (new branch 'mellions/race')\n" +
				"fatal: a branch named 'mellions/race' already exists\n",
			want: "already exists",
			gone: "Preparing worktree",
		},
		{
			name: "checkout of a branch held elsewhere",
			out: "Switched to branch 'x'\n" +
				"fatal: 'work' is already used by worktree at '/elsewhere'\n",
			want: "already used by worktree",
			gone: "Switched to",
		},
		{
			name: "worktree remove refusing a dirty tree",
			out:  "fatal: '/w' contains modified or untracked files, use --force to delete it\n",
			want: "modified or untracked files",
		},
		{
			name: "several lines all of which matter",
			out:  "error: one thing\nerror: another thing\n",
			want: "error: one thing; error: another thing",
		},
		{
			name: "nothing but progress leaves the output rather than nothing",
			out:  "Preparing worktree (new branch 'x')\n",
			want: "Preparing worktree",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := reason(tc.out)
			if !strings.Contains(got, tc.want) {
				t.Errorf("reason = %q, want it to carry %q", got, tc.want)
			}
			if tc.gone != "" && strings.Contains(got, tc.gone) {
				t.Errorf("reason = %q, still carries the progress line %q", got, tc.gone)
			}
		})
	}
}

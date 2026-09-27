// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// fileEstate is the shared estate plus a load path, a lane nested under a
// shared checkout, a linked worktree the probe recognises, and one assigned
// session: "mine".
func fileEstate() sharedtree.Estate {
	e := estate
	e.Shared = append([]sharedtree.Checkout(nil), estate.Shared...)
	e.Lanes = []string{"/home/you/mellions/assignments", "/home/you/workspace/payments-api/.worktrees"}
	e.LoadPath = "/home/you/mellions-coxen"
	e.OtherTree = func(dir, checkout string) bool {
		return strings.HasPrefix(dir, "/home/you/workspace/data-service/.claude/worktrees/")
	}
	e.Assigned = func(session, cwd string) bool { return session == "mine" }
	return e
}

// filePayload is the PreToolUse shape the runtime sends for a file-writing
// tool: the path is a field of tool_input, keyed file_path, or notebook_path
// for a notebook. Written as raw JSON so a change of field name reds here.
func filePayload(session, tool, cwd, path string) []byte {
	key := "file_path"
	if tool == "NotebookEdit" {
		key = "notebook_path"
	}
	return []byte(`{"session_id":"` + session + `","hook_event_name":"PreToolUse","tool_name":"` + tool +
		`","cwd":"` + cwd + `","tool_input":{"` + key + `":"` + path + `","old_string":"a","new_string":"b"}}`)
}

func TestAFileToolIntoASharedCheckoutIsRefusedForAnAssignedSession(t *testing.T) {
	e := fileEstate()
	for _, tool := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit"} {
		for _, tc := range []struct {
			name, session, cwd, path string
			refused                  bool
		}{
			{"shared checkout file", "mine", lane, "/home/you/workspace/data-service/internal/x.go", true},
			{"shared checkout root file", "mine", lane, "/home/you/workspace/data-service/go.mod", true},
			{"relative path resolved against a cwd in the checkout", "mine",
				"/home/you/workspace/data-service/internal", "x.go", true},
			{"load path file", "mine", lane, "/home/you/mellions-coxen/internal/sharedtree/sharedtree.go", true},
			{"lane file", "mine", lane, lane + "/internal/x.go", false},
			{"lane nested under a shared checkout", "mine", lane,
				"/home/you/workspace/payments-api/.worktrees/p-7/main.go", false},
			{"linked worktree inside the checkout", "mine", lane,
				"/home/you/workspace/data-service/.claude/worktrees/eval/x.go", false},
			{"outside every checkout", "mine", lane, "/tmp/scratch/x.go", false},
			{"sibling name sharing a prefix", "mine", lane, "/home/you/workspace/data-service-notes/x.md", false},
			{"a session holding no assignment", "owner", "/home/you/workspace/data-service",
				"/home/you/workspace/data-service/internal/x.go", false},
		} {
			got := sharedtree.Deny(filePayload(tc.session, tool, tc.cwd, tc.path), e)
			if (got != "") != tc.refused {
				t.Errorf("%s %s: refused=%v, want %v\n%s", tool, tc.name, got != "", tc.refused, got)
			}
		}
	}
}

func TestTheFileRefusalNamesTheSameFileInTheSessionsLane(t *testing.T) {
	got := sharedtree.Deny(filePayload("mine", "Edit", lane,
		"/home/you/workspace/data-service/internal/x.go"), fileEstate())
	for _, want := range []string{
		"`Edit` writes /home/you/workspace/data-service/internal/x.go",
		"data-service checkout every lane on this host is cut from",
		lane + "/internal/x.go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got)
		}
	}
}

func TestWithNoAssignedProbeAFileToolIsNeverRefused(t *testing.T) {
	e := fileEstate()
	e.Assigned = nil
	if got := sharedtree.Deny(filePayload("mine", "Write", lane,
		"/home/you/workspace/data-service/internal/x.go"), e); got != "" {
		t.Errorf("refused with no way to know the session holds a lane:\n%s", got)
	}
}

// A tool that only reads, or one this package does not know, is silence.
func TestAReadToolIsNeverRefused(t *testing.T) {
	for _, tool := range []string{"Read", "Grep", "Glob", "NotebookRead"} {
		if got := sharedtree.Deny(filePayload("mine", tool, lane,
			"/home/you/workspace/data-service/internal/x.go"), fileEstate()); got != "" {
			t.Errorf("%s was refused:\n%s", tool, got)
		}
	}
}

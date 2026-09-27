// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"encoding/json"
	"os"
	"regexp"
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
	e.Ignored = func(path, checkout string) bool {
		return strings.Contains(path, "/.remember/") || strings.Contains(path, "/.claude/") ||
			strings.HasSuffix(path, "/.env")
	}
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
			{"literal name a shell would expand", "mine", lane,
				"/home/you/workspace/data-service/$weird*.go", true},
			{"memory plugin state git ignores in the checkout", "mine", lane,
				"/home/you/workspace/data-service/.remember/remember.md", false},
			{"ignored runtime settings in the checkout", "mine", lane,
				"/home/you/workspace/data-service/.claude/settings.local.json", true},
			{"ignored env file in the checkout", "mine", lane,
				"/home/you/workspace/data-service/.env", true},
			{".remember below the checkout root", "mine", lane,
				"/home/you/workspace/data-service/internal/.remember/x.md", true},
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

// The decision above is reached only if the runtime hands the guard these
// tools at all. A matcher is a regular expression over the tool name, so each
// name is matched the way the runtime matches it, against the groups that run
// shared-tree.sh.
func TestTheHookRunsTheGuardForEveryToolItDecides(t *testing.T) {
	raw, err := os.ReadFile("../../hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"Bash", "Edit", "Write", "MultiEdit", "NotebookEdit"} {
		found := false
		for _, g := range cfg.Hooks["PreToolUse"] {
			re, err := regexp.Compile("^(?:" + g.Matcher + ")$")
			if err != nil || !re.MatchString(tool) {
				continue
			}
			for _, h := range g.Hooks {
				if strings.Contains(h.Command, "/hooks/shared-tree.sh") {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no PreToolUse group runs shared-tree.sh for %s, so the guard never sees it", tool)
		}
	}
}

// The runtime's plugin root can be a copy while the registry reads the
// checkout in place; the checkout is guarded by either name, and the refusal
// names the lane for the repository the checkout is, not the copy's directory.
func TestTheLoadPathIsGuardedByEveryNameItHas(t *testing.T) {
	e := fileEstate()
	e.LoadPath = "/home/you/.claude/plugins/cache/mellions/0.1.0"
	e.LoadAliases = []string{"/home/you/mellions-coxen"}
	e.LoadRepo = "mellions-coxen"
	e.Lane = func(repo, session, cwd string) string {
		if repo == "mellions-coxen" {
			return "/home/you/mellions/assignments/coxen-1/tree"
		}
		return ""
	}
	got := sharedtree.Deny(filePayload("mine", "Edit", lane, "/home/you/mellions-coxen/hooks/hooks.json"), e)
	if !strings.Contains(got, "/home/you/mellions/assignments/coxen-1/tree/hooks/hooks.json") {
		t.Errorf("an edit of the load path by its registry name was not refused naming the lane:\n%s", got)
	}
}

// launchedPayload is filePayload with the transcript path the runtime sends,
// kept under a directory named for the directory the session started in.
func launchedPayload(session, tool, cwd, path, project string) []byte {
	p := filePayload(session, tool, cwd, path)
	return []byte(strings.Replace(string(p), `"hook_event_name"`,
		`"transcript_path":"/home/you/.claude/projects/`+project+`/`+session+`.jsonl","hook_event_name"`, 1))
}

func TestASessionStartedInACheckoutWritesThatCheckoutAndNoOther(t *testing.T) {
	e := fileEstate()
	const dataSvc = "/home/you/workspace/data-service"
	for _, tc := range []struct {
		name, project, path string
		refused             bool
	}{
		{"its own checkout", "-home-you-workspace-data-service", dataSvc + "/internal/x.go", false},
		{"another checkout", "-home-you-workspace-data-service", "/home/you/workspace/payments-api/x.go", true},
		{"a checkout whose name only starts like its own", "-home-you-workspace-data-service-old", dataSvc + "/x.go", true},
		{"started in a subdirectory of the checkout", "-home-you-workspace-data-service-internal", dataSvc + "/x.go", true},
		{"started in the load path, writing the load path", "-home-you-mellions-coxen", "/home/you/mellions-coxen/README.md", true},
		{"started outside every checkout", "-home-you-mellions", dataSvc + "/x.go", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sharedtree.Deny(launchedPayload("mine", "Edit", dataSvc, tc.path, tc.project), e) != ""
			if got != tc.refused {
				t.Fatalf("refused = %v, want %v", got, tc.refused)
			}
		})
	}
}

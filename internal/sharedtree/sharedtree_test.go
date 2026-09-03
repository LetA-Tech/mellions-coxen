// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// The estate this whole file decides against: two surveyed checkouts under one
// work root, lanes elsewhere, and a checkout the installation works in but
// does not survey.
var estate = sharedtree.Estate{
	Shared: []sharedtree.Checkout{
		{Repo: "data-service", Dir: "/home/you/workspace/data-service"},
		{Repo: "payments-api", Dir: "/home/you/workspace/payments-api"},
	},
	Lanes: []string{"/home/you/mellions/assignments"},
	Home:  "/home/you",
	// Keyed on the session as well as the repository: the caller's job is to
	// answer for THIS session, and a lane belonging to another one must not
	// come back.
	Lane: func(repo, session, cwd string) string {
		if repo == "data-service" && session == "mine" {
			return "/home/you/mellions/assignments/data-42/tree"
		}
		return ""
	},
}

const lane = "/home/you/mellions/assignments/data-42/tree"

// crossTreeMutation combines a directory change and destructive Git readback.
const crossTreeMutation = `cd /home/you/workspace/data-service && git checkout --quiet abc1234 -- . 2>/dev/null; ` +
	`sed -n '1,80p' internal/example/handler_test.go`

func TestCrossTreeMutationIsRefused(t *testing.T) {
	got := sharedtree.Deny(payload("Bash", lane, crossTreeMutation), estate)
	if got == "" {
		t.Fatal("the shared-checkout mutation was allowed")
	}
	// Literals, not another call into the renderer: what the session reads is
	// the thing under test, so the oracle cannot move with it.
	for _, want := range []string{
		"/home/you/workspace/data-service",
		"data-service checkout every lane on this host is cut from",
		"git show <rev>:<path>",
		"git archive",
		lane,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q, so the session is told what not to do and\n"+
				"not what to do instead:\n%s", want, got)
		}
	}
}

// A guard that matches nothing reports every session clean, and one that
// matches everything stops the work. Both halves are asserted from one table.
func TestWhatIsRefusedAndWhatIsNot(t *testing.T) {
	for _, c := range []struct {
		name    string
		cwd     string
		command string
		deny    bool
	}{
		// Writing a surveyed checkout, by every route into it.
		{"cd then checkout", lane, "cd /home/you/workspace/data-service && git checkout abc1234 -- .", true},
		{"git -C", lane, "git -C /home/you/workspace/data-service reset --hard origin/dev", true},
		{"git -C glued", lane, "git -C/home/you/workspace/data-service clean -fd", true},
		{"already standing in it", "/home/you/workspace/data-service", "git stash", true},
		{"a subdirectory of it", "/home/you/workspace/data-service/internal/repo", "git restore .", true},
		{"cd relative from the work root", "/home/you/workspace", "cd data-service && git checkout dev", true},
		{"cd through a tilde", lane, "cd ~/workspace/payments-api && git reset --hard", true},
		{"cd up and over", "/home/you/workspace/payments-api/cmd", "cd ../../data-service && git clean -fdx", true},
		{"a subshell", lane, "( cd /home/you/workspace/data-service && git stash pop )", true},
		{"second in a chain", lane, "git -C " + lane + " status && git -C /home/you/workspace/data-service checkout dev", true},
		{"a semicolon chain", "/home/you/workspace/data-service", "echo reading; git apply /tmp/p.patch", true},
		{"switch", "/home/you/workspace/data-service", "git switch dev", true},
		{"pull", "/home/you/workspace/data-service", "git pull --rebase origin dev", true},
		{"commit", "/home/you/workspace/data-service", "git commit -am 'wip'", true},
		{"add", "/home/you/workspace/data-service", "git add -A", true},
		{"rebase", "/home/you/workspace/data-service", "git rebase origin/dev", true},
		{"cherry-pick", "/home/you/workspace/data-service", "git cherry-pick abc123", true},
		{"a quoted path", lane, `cd "/home/you/workspace/data-service" && git checkout HEAD~1 -- .`, true},

		// The reads the guard exists to point at. Every one of these is what a
		// session should do instead, and refusing them would push it back to
		// the command that writes.
		{"show", "/home/you/workspace/data-service", "git show abc1234:internal/repo/repo.go", false},
		{"archive elsewhere", "/home/you/workspace/data-service", `git archive abc1234 | tar -x -C /tmp/x`, false},
		{"diff", "/home/you/workspace/data-service", "git diff abc1234 -- internal/repo", false},
		{"log", "/home/you/workspace/data-service", "git log --oneline -20", false},
		{"status", "/home/you/workspace/data-service", "git status --porcelain", false},
		{"blame", "/home/you/workspace/data-service", "git blame internal/repo/repo.go", false},
		{"fetch writes refs, not the tree", "/home/you/workspace/data-service", "git fetch origin dev", false},
		{"ls-remote", "/home/you/workspace/data-service", "git ls-remote --heads origin", false},
		{"worktree add is how a lane is cut", "/home/you/workspace/data-service", "git worktree add /home/you/mellions/assignments/x/tree -b mellions/x", false},
		{"stash create writes no ref", "/home/you/workspace/data-service", "git stash create", false},
		{"stash list", "/home/you/workspace/data-service", "git stash list", false},
		{"stash show", "/home/you/workspace/data-service", "git stash show -p", false},
		{"clean dry run", "/home/you/workspace/data-service", "git clean -n -d", false},
		{"clean dry run clustered", "/home/you/workspace/data-service", "git clean -nd", false},
		{"apply check", "/home/you/workspace/data-service", "git apply --check /tmp/p.patch", false},
		{"rm dry run", "/home/you/workspace/data-service", "git rm -n x", false},

		// The trees a session owns, where the same commands are its business.
		{"its own lane", lane, "git checkout abc1234 -- .", false},
		{"a lane by -C", "/home/you/workspace/data-service", "git -C " + lane + " reset --hard", false},
		{"another lane", "/home/you/mellions/assignments/other/tree", "git clean -fdx", false},
		{"Mellions' own source, which repos does not survey", "/home/you/mellions-coxen", "git checkout dev", false},
		{"a checkout nobody configured", "/tmp/scratch/clone", "git reset --hard", false},

		// Near misses on the path boundary.
		{"a sibling whose name is a prefix", "/home/you/workspace/data-service-notes", "git reset --hard", false},
		{"the work root itself", "/home/you/workspace", "git status", false},

		// Not this hook's business.
		{"not git", "/home/you/workspace/data-service", "rm -rf internal/repo", false},
		{"the word in prose", lane, `echo "do not git checkout in /home/you/workspace/data-service"`, false},
		{"a path this cannot resolve", "/home/you/workspace/data-service", `cd "$(mktemp -d)" && git checkout dev`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := sharedtree.Deny(payload("Bash", c.cwd, c.command), estate) != ""
			if got != c.deny {
				t.Errorf("Deny(%q) in %s = %v, want %v", c.command, c.cwd, got, c.deny)
			}
		})
	}
}

// Only the Bash tool carries a command line, and a payload this cannot read is
// silence rather than a refusal on a guess.
func TestSilenceWhereThereIsNothingToRead(t *testing.T) {
	for _, c := range []struct {
		name    string
		payload []byte
	}{
		{"another tool", payload("Edit", "/home/you/workspace/data-service", "git reset --hard")},
		{"not json", []byte("git reset --hard")},
		{"empty", nil},
		{"no command", []byte(`{"tool_name":"Bash","cwd":"/home/you/workspace/data-service"}`)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := sharedtree.Deny(c.payload, estate); got != "" {
				t.Errorf("want silence, got:\n%s", got)
			}
		})
	}
}

// An installation with nothing configured refuses nothing: the guard is a
// consequence of knowing where the shared checkouts are, never of a default.
func TestAnEmptyEstateRefusesNothing(t *testing.T) {
	if got := sharedtree.Deny(payload("Bash", "/home/you/workspace/data-service", crossTreeMutation),
		sharedtree.Estate{}); got != "" {
		t.Errorf("want silence with no estate configured, got:\n%s", got)
	}
}

// With no lane open for the repository, the refusal still has to leave the
// session somewhere to go.
func TestWithNoLaneTheRefusalNamesHowToCutOne(t *testing.T) {
	got := sharedtree.Deny(payload("Bash", "/home/you/workspace/payments-api", "git reset --hard"), estate)
	if !strings.Contains(got, "mellions assign open -id <id> -repo payments-api") {
		t.Errorf("the refusal does not say how to cut a lane:\n%s", got)
	}
}

// Find reports which invocation it stopped on, so a caller can say more than
// that something was refused.
func TestFindNamesTheInvocation(t *testing.T) {
	w := sharedtree.Find(crossTreeMutation, lane, estate)
	if w == nil {
		t.Fatal("Find returned nothing for the cross-tree mutation")
	}
	if w.Verb != "checkout" || w.Repo != "data-service" || w.Checkout != "/home/you/workspace/data-service" {
		t.Errorf("got %+v", w)
	}
}

func payload(tool, cwd, command string) []byte {
	return sessionPayload("mine", tool, cwd, command)
}

func sessionPayload(session, tool, cwd, command string) []byte {
	raw, err := json.Marshal(map[string]any{
		"session_id": session,
		"tool_name":  tool,
		"cwd":        cwd,
		"tool_input": map[string]string{"command": command},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

// Reach is what the awareness note needs: a session standing outside the
// shared checkout whose command line steps into it.
func TestReach(t *testing.T) {
	for _, c := range []struct {
		name, cwd, command, want string
	}{
		{"compound command reaches shared checkout", "/home/you/mellions", crossTreeMutation, "/home/you/workspace/data-service"},
		{"a read reaching in", "/home/you/mellions", "cd /home/you/workspace/data-service && grep -rn foo .", "/home/you/workspace/data-service"},
		{"git -C reaching in", lane, "git -C /home/you/workspace/payments-api log --oneline -5", "/home/you/workspace/payments-api"},
		{"standing in it is a different fact", "/home/you/workspace/data-service", "git status", ""},
		{"staying in its lane", lane, "go test ./...", ""},
		{"reaching into a lane", "/home/you/mellions", "cd " + lane + " && git status", ""},
		{"reaching nowhere configured", "/home/you/mellions", "cd /tmp && git status", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := ""
			if r := sharedtree.Reach(c.command, c.cwd, estate); r != nil {
				got = r.Dir
			}
			if got != c.want {
				t.Errorf("Reach(%q) in %s = %q, want %q", c.command, c.cwd, got, c.want)
			}
		})
	}
}

// The refusal must never send a session into a lane that is not its own. A
// host runs several lanes on one repository at a time, and the tree it would
// name is exactly the kind this guard exists to keep sessions out of.
func TestARefusalNeverNamesAnotherSessionsLane(t *testing.T) {
	got := sharedtree.Deny(sessionPayload("somebody-else", "Bash",
		"/home/you/mellions", crossTreeMutation), estate)
	if got == "" {
		t.Fatal("the cross-tree mutation was allowed")
	}
	if strings.Contains(got, "/home/you/mellions/assignments/data-42/tree") {
		t.Errorf("the refusal names a lane held by another session:\n%s", got)
	}
	if !strings.Contains(got, "mellions assign open -id <id> -repo data-service") {
		t.Errorf("with no lane of its own the refusal must say how to cut one:\n%s", got)
	}
}

// A lane worktree cut inside the checkout it came from is still the session's
// own tree. `git worktree add` accepts a path anywhere, assignments_root is
// configuration, and the two can be nested — so the exemption has to hold
// where the lane is under the shared checkout, which is the only arrangement
// in which it decides anything at all.
func TestALaneInsideTheCheckoutIsStillTheSessionsOwn(t *testing.T) {
	nested := sharedtree.Estate{
		Shared: []sharedtree.Checkout{{Repo: "data-service", Dir: "/home/you/workspace/data-service"}},
		Lanes:  []string{"/home/you/workspace/data-service/.worktrees"},
		Home:   "/home/you",
	}
	inLane := "/home/you/workspace/data-service/.worktrees/data-42"
	if got := sharedtree.Deny(payload("Bash", inLane, "git checkout abc1234 -- ."), nested); got != "" {
		t.Errorf("a lane under the checkout was refused its own tree:\n%s", got)
	}
	if got := sharedtree.Deny(payload("Bash", "/home/you/mellions",
		"cd /home/you/workspace/data-service && git checkout abc1234 -- ."), nested); got == "" {
		t.Error("the checkout around the lane was not protected")
	}
}

// Repairing a shared checkout is a write to it, so it is refused like any
// other — and the refusal has to say so, because a session told only "no"
// about a tree it can see is wrong will look for a way round.
func TestTheRefusalNamesRepairAsADecisionRatherThanACommand(t *testing.T) {
	got := sharedtree.Deny(payload("Bash", "/home/you/mellions",
		"git -C /home/you/workspace/data-service restore --source=HEAD --staged --worktree -- ."), estate)
	if got == "" {
		t.Fatal("repairing a shared checkout is a write and was allowed")
	}
	for _, want := range []string{"repairing it is the point", "stash create", "Say so and leave the command"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got)
		}
	}
}

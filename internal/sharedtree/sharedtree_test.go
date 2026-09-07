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

// The load path is a shared checkout like any other, and `git pull --ff-only`
// there is the one command that lands a Mellions fix. Refusing it left the
// guard blocking the only sanctioned way to install anything, including a fix
// to the guard.
//
// Safe on a clean tree, which is what this estate has: no Dirty probe, so
// nothing here reports uncommitted work. The dirty case is its own test,
// because git does NOT refuse it under autostash.
func TestFastForwardPullOfTheLoadPathIsTheOneAllowedWrite(t *testing.T) {
	e := sharedtree.Estate{
		Shared: []sharedtree.Checkout{
			{Repo: "mellions-coxen", Dir: "/home/you/leta/mellions-coxen"},
			{Repo: "coxen-fork", Dir: "/home/you/leta/mellions-coxen-fork"},
			{Repo: "vendored", Dir: "/home/you/leta/mellions-coxen/vendor/data-service"},
			{Repo: "data-service", Dir: "/home/you/workspace/data-service"},
		},
		Home:     "/home/you",
		LoadPath: "/home/you/leta/mellions-coxen",
	}

	for _, tc := range []struct {
		name    string
		command string
		cwd     string
		refused bool
	}{
		{"ff-only pull of the load path lands a fix",
			"git pull --ff-only", "/home/you/leta/mellions-coxen", false},
		{"ff-only via -C also lands it",
			"git -C /home/you/leta/mellions-coxen pull --ff-only", "/tmp", false},
		// `git pull` operates on the repository, not on the directory it is
		// typed in, so a session standing one directory inside the load path is
		// running the same deployment step.
		{"ff-only pull from inside the load path lands a fix",
			"git pull --ff-only", "/home/you/leta/mellions-coxen/internal/sharedtree", false},
		{"ff-only via -C into a subdirectory also lands it",
			"git -C /home/you/leta/mellions-coxen/hooks pull --ff-only", "/tmp", false},
		// A pull that could merge is not a deployment; it resolves conflicts
		// against a tree nobody looked at.
		{"a pull that could merge is still refused",
			"git pull", "/home/you/leta/mellions-coxen", true},
		{"a rebasing pull is still refused",
			"git pull --rebase origin main", "/home/you/leta/mellions-coxen", true},
		// Scoped to the load path. Another shared checkout gains nothing: the
		// exemption is for the deployment step, not for pulling in general.
		{"ff-only pull of another shared checkout is refused",
			"git pull --ff-only", "/home/you/workspace/data-service", true},
		// Every other verb is unchanged, or the exemption widened past a pull.
		{"the load path is not otherwise writable",
			"git checkout main", "/home/you/leta/mellions-coxen", true},
		{"reset in the load path is still refused",
			"git reset --hard origin/main", "/home/you/leta/mellions-coxen", true},
		// The widening is to which paths name the load path, not to which
		// verbs it admits: one directory in, everything else is as before.
		{"a merging pull inside the load path is still refused",
			"git pull", "/home/you/leta/mellions-coxen/hooks", true},
		{"checkout inside the load path is still refused",
			"git checkout main", "/home/you/leta/mellions-coxen/hooks", true},
		// A path that only shares the load path's prefix is a different tree.
		{"a checkout beside the load path gains nothing",
			"git pull --ff-only", "/home/you/leta/mellions-coxen-fork", true},
		// And so is one that lives inside it. Being under the load path is not
		// the question — which repository the pull writes is, and a nested
		// checkout answers with its own.
		{"a checkout nested inside the load path gains nothing",
			"git pull --ff-only", "/home/you/leta/mellions-coxen/vendor/data-service", true},
		// git reads --ff, --no-ff and --ff-only as one setting, so the last of
		// them is the one in force. `--ff-only --no-ff` creates a merge commit,
		// and merging into a tree nobody looked at is what this must not admit.
		{"--no-ff after --ff-only is a merging pull",
			"git pull --ff-only --no-ff", "/home/you/leta/mellions-coxen", true},
		{"--ff after --ff-only is a merging pull",
			"git pull --ff-only --ff origin dev", "/home/you/leta/mellions-coxen", true},
		{"--ff-only last is still the deployment step",
			"git pull --no-ff --ff-only", "/home/you/leta/mellions-coxen", false},
		{"past -- it is a refspec, not an option",
			"git pull origin -- --ff-only", "/home/you/leta/mellions-coxen", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sharedtree.Find(tc.command, tc.cwd, e)
			if tc.refused && got == nil {
				t.Fatalf("%q at %s was allowed; want refused", tc.command, tc.cwd)
			}
			if !tc.refused && got != nil {
				t.Fatalf("%q at %s was refused (%s); it is how a Mellions fix is landed",
					tc.command, tc.cwd, got.Verb)
			}
		})
	}
}

// A load path that sits INSIDE some other repository's checkout resolves, by
// longest match, to that repository — and an exemption keyed on the name alone
// would then cover that whole foreign tree. Found by a session that reviewed
// this change without having written it, against a version that admitted both
// rows below.
func TestALoadPathNestedInAnotherCheckoutExemptsNothing(t *testing.T) {
	e := sharedtree.Estate{
		Shared: []sharedtree.Checkout{
			{Repo: "mcfo-finsys", Dir: "/home/you/workspace/mcfo-finsys"},
		},
		Home:     "/home/you",
		LoadPath: "/home/you/workspace/mcfo-finsys/tools/mellions-coxen",
	}
	for _, cwd := range []string{
		"/home/you/workspace/mcfo-finsys",
		"/home/you/workspace/mcfo-finsys/internal/ledger",
		"/home/you/workspace/mcfo-finsys/tools/mellions-coxen",
	} {
		if sharedtree.Find("git pull --ff-only", cwd, e) == nil {
			t.Errorf("a load path nested in mcfo-finsys exempted %s, which is mcfo-finsys' tree", cwd)
		}
	}
}

// With no load path known, nothing is exempt. An installation that cannot say
// where it loads from must not have the exemption applied to an arbitrary tree.
func TestNoLoadPathExemptsNothing(t *testing.T) {
	e := sharedtree.Estate{
		Shared: []sharedtree.Checkout{{Repo: "mellions-coxen", Dir: "/home/you/leta/mellions-coxen"}},
		Home:   "/home/you",
	}
	if sharedtree.Find("git pull --ff-only", "/home/you/leta/mellions-coxen", e) == nil {
		t.Fatal("an empty LoadPath exempted a shared checkout")
	}
}

// A checkout reachable by two names is listed under both, because a session
// that walked in by the link would otherwise be refused nothing. The exemption
// has to hold across the same two names, or the guard refuses the deployment
// step on every host whose work root is a symlink — the deny is decided by
// containment over both entries, and only one of them can equal a single
// LoadPath string.
func TestTheLoadPathReachedByItsOtherNameIsStillTheLoadPath(t *testing.T) {
	e := sharedtree.Estate{
		Shared: []sharedtree.Checkout{
			{Repo: "mellions-coxen", Dir: "/home/you/leta/mellions-coxen"},
			{Repo: "mellions-coxen", Dir: "/mnt/data/leta/mellions-coxen"},
			{Repo: "data-service", Dir: "/home/you/workspace/data-service"},
		},
		Home: "/home/you",
		// What the plugin registry recorded: the resolved path, while the
		// session stands at the one it walked in by.
		LoadPath: "/mnt/data/leta/mellions-coxen",
	}

	for _, tc := range []struct {
		name    string
		command string
		cwd     string
		refused bool
	}{
		{"the recorded name lands a fix",
			"git pull --ff-only", "/mnt/data/leta/mellions-coxen", false},
		{"the other name is the same tree",
			"git pull --ff-only", "/home/you/leta/mellions-coxen", false},
		{"and one directory inside it",
			"git pull --ff-only", "/home/you/leta/mellions-coxen/hooks", false},
		// The other name gains no verb the recorded one does not have.
		{"a merging pull by the other name is still refused",
			"git pull", "/home/you/leta/mellions-coxen", true},
		{"reset by the other name is still refused",
			"git reset --hard origin/dev", "/home/you/leta/mellions-coxen", true},
		{"another repository is untouched by any of it",
			"git pull --ff-only", "/home/you/workspace/data-service", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sharedtree.Find(tc.command, tc.cwd, e)
			if tc.refused && got == nil {
				t.Fatalf("%q at %s was allowed; want refused", tc.command, tc.cwd)
			}
			if !tc.refused && got != nil {
				t.Fatalf("%q at %s was refused (%s); it is how a Mellions fix is landed",
					tc.command, tc.cwd, got.Verb)
			}
		})
	}
}

// coxen is the load path in the tests below, and dirtyCoxen is an estate whose
// probe reports that tree — and only that tree — as carrying uncommitted work.
const coxen = "/home/you/leta/mellions-coxen"

func coxenEstate(dirty func(string) bool) sharedtree.Estate {
	return sharedtree.Estate{
		Shared: []sharedtree.Checkout{
			{Repo: "mellions-coxen", Dir: coxen},
			{Repo: "data-service", Dir: "/home/you/workspace/data-service"},
		},
		Home:     "/home/you",
		LoadPath: coxen,
		Dirty:    dirty,
	}
}

func asks(seen *[]string, answer bool) func(string) bool {
	return func(dir string) bool {
		*seen = append(*seen, dir)
		return answer
	}
}

// The deployment exemption's whole safety argument was that git refuses what it
// admits. Measured on git 2.53.0, that is false in one case: with
// `rebase.autoStash` or `merge.autoStash` set and an upstream that CAN
// fast-forward, `git pull --ff-only` on a dirty tree stashes, fast-forwards,
// fails to reapply the stash and exits 0, leaving conflict markers in the
// working tree. Exit 0 is what the session and scripts/shifts.sh both read as
// landed.
//
// So a dirty load path is refused. The three cases are separate arms because
// the clean one is what the exemption exists for and the dirty one is what it
// must not admit — assuming either from the other is how this regressed once.
func TestTheDeploymentPullIsRefusedWhenTheLoadPathIsDirty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		cwd     string
		dirty   bool
		refused bool
	}{
		{"a clean load path still lands a fix",
			"git pull --ff-only", coxen, false, false},
		{"a dirty load path is refused",
			"git pull --ff-only", coxen, true, true},
		// A pull is about the repository, not the directory it is typed in, so
		// the tree it writes is dirty either way.
		{"a dirty load path is refused from a subdirectory too",
			"git pull --ff-only", coxen + "/internal/sharedtree", true, true},
		{"a dirty load path is refused through -C",
			"git -C " + coxen + " pull --ff-only", "/tmp", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sharedtree.Find(tc.command, tc.cwd, coxenEstate(func(string) bool { return tc.dirty }))
			if tc.refused && got == nil {
				t.Fatalf("%q in a dirty load path was allowed; under autostash that exits 0 "+
					"and leaves conflict markers in the tree this host loads from", tc.command)
			}
			if !tc.refused && got != nil {
				t.Fatalf("%q in a clean load path was refused, which blocks the only "+
					"sanctioned way to install a fix:\n%s", tc.command,
					got.Reason(coxenEstate(nil), "s", tc.cwd))
			}
		})
	}
}

// A refusal that misdescribes itself costs the session the fix. This one is
// not "you are in the wrong tree" — it is the right tree and the right
// command — and the generic text's "no reflog entry, no stash" is the reverse
// of what autostash does. Literals, so the oracle cannot move with the
// renderer.
func TestTheDirtyDeploymentRefusalSaysWhyRatherThanTheGenericReason(t *testing.T) {
	w := sharedtree.Find("git pull --ff-only", coxen, coxenEstate(func(string) bool { return true }))
	if w == nil {
		t.Fatal("the dirty deployment pull was allowed")
	}
	got := w.Reason(coxenEstate(nil), "s", coxen)
	for _, want := range []string{
		"is the right command",
		"uncommitted changes",
		"exits 0",
		"git -C " + coxen + " status --porcelain",
		"not yours to clear",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q, so the session is not told what to do "+
				"about the tree:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"no reflog entry, no stash",
		"every lane on this host is cut from",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the refusal falls back on the generic reason (%q), which is wrong here "+
				"in both halves:\n%s", unwanted, got)
		}
	}
}

// The probe is asked about the tree the pull writes, and it is asked at all
// only where the exemption would otherwise apply — a refusal that is already
// decided must not go shelling out to git.
func TestTheDirtyProbeIsAskedOnlyWhereTheExemptionWouldApply(t *testing.T) {
	var seen []string
	if sharedtree.Find("git -C "+coxen+"/hooks pull --ff-only", "/tmp",
		coxenEstate(asks(&seen, false))) != nil {
		t.Fatal("a clean load path was refused")
	}
	if len(seen) != 1 || seen[0] != coxen+"/hooks" {
		t.Fatalf("the probe was asked about %v, not about the directory the pull runs in", seen)
	}

	// Another shared checkout is refused on the tree, before any exemption is
	// considered, so nothing is asked.
	seen = nil
	if sharedtree.Find("git pull --ff-only", "/home/you/workspace/data-service",
		coxenEstate(asks(&seen, false))) == nil {
		t.Fatal("a pull of a checkout that is not the load path was allowed")
	}
	if len(seen) != 0 {
		t.Fatalf("the probe ran for a refusal already decided: %v", seen)
	}

	// A merging pull of the load path is refused on its form, so likewise.
	seen = nil
	if sharedtree.Find("git pull --ff-only --no-ff", coxen,
		coxenEstate(asks(&seen, false))) == nil {
		t.Fatal("a merging pull of the load path was allowed")
	}
	if len(seen) != 0 {
		t.Fatalf("the probe ran for a pull already refused on its form: %v", seen)
	}
}

// No probe is "cannot tell", and cannot tell has to leave the exemption
// standing: the cost of guessing dirty is that nothing can install a fix,
// including a fix to this guard. Every existing exemption test relies on this,
// and it is stated here rather than left implicit in their silence.
func TestWithNoDirtyProbeTheExemptionStands(t *testing.T) {
	if got := sharedtree.Find("git pull --ff-only", coxen, coxenEstate(nil)); got != nil {
		t.Fatalf("a nil Dirty probe refused the deployment pull:\n%s",
			got.Reason(coxenEstate(nil), "s", coxen))
	}
}

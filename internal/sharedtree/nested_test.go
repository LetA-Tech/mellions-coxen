// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// A git that bash runs from inside another command writes the tree it runs
// in: a substitution, a shell's -c string or stdin script, and the command
// xargs runs are read like a command line of their own.
func TestAGitBashRunsInsideAnotherCommandIsRead(t *testing.T) {
	const shared = "/home/you/workspace/data-service"
	for _, c := range []struct {
		name    string
		cwd     string
		command string
		deny    bool
	}{
		{"substitution", shared, "echo $(git reset --hard)", true},
		{"substitution in double quotes", shared, `echo "$(git reset --hard)"`, true},
		{"nested substitution", shared, "echo $(echo $(git clean -fdx))", true},
		{"substitution in a cd operand", shared, "cd $(git checkout dev; echo /tmp)", true},
		{"bash -c", shared, "bash -c 'git reset --hard'", true},
		{"eval", shared, "eval 'git reset --hard'", true},
		{"live in an unquoted heredoc", shared, "cat <<EOF\n$(git reset --hard)\nEOF", true},
		{"data in a quoted heredoc", shared, "cat <<'EOF'\n$(git reset --hard)\nEOF", false},
		{"sh -c with a name operand", shared, `sh -c "git reset --hard" name`, true},
		{"bash -ec clustered", shared, "bash -ec 'git stash pop'", true},
		{"bash -o value then -c", shared, "bash -o pipefail -c 'git restore .'", true},
		{"absolute shell path", shared, "/bin/bash -c 'git reset --hard'", true},
		{"bash -c cd into it", lane, "bash -c 'cd /home/you/workspace/data-service && git reset --hard'", true},
		{"bash -c behind a wrapper", shared, "timeout 30 bash -c 'git reset --hard'", true},
		{"nested bash -c", shared, `bash -c "sh -c 'git reset --hard'"`, true},
		{"bash here-string", shared, "bash <<<'git reset --hard'", true},
		{"bash -s here-string", shared, `bash -s <<< "git checkout dev"`, true},
		{"bash heredoc", shared, "bash <<'EOF'\ngit reset --hard\nEOF", true},
		{"xargs", shared, "xargs git rm < files", true},
		{"xargs with options", shared, "xargs -0 -n 1 -I {} git checkout {} -- .", true},
		{"xargs then bash -c", shared, "xargs -r bash -c 'git reset --hard'", true},
		{"a wrapper before git", shared, "timeout 60 git reset --hard", true},
		{"a keyword before git", shared, "if true; then git reset --hard; fi", true},
		{"-c then --", shared, "sh -c -- 'git reset --hard'", true},
		{"-c then -", shared, "bash -c - 'git reset --hard'", true},
		{"-s with an operand reads stdin", shared, "bash -s foo <<<'git reset --hard'", true},
		{"an -o cluster's value then -c", shared, "bash -eo pipefail -c 'git reset --hard'", true},
		{"xargs running xargs", shared, "xargs xargs git checkout --", true},
		{"GIT_DIR alone leaves the tree in cwd", shared, "GIT_DIR=" + lane + "/.git git reset --hard", true},
		{"env -C into it", lane, "env -C /home/you/workspace/data-service git reset --hard", true},

		{"a read in a substitution", shared, "echo $(git status --porcelain)", false},
		{"a read in bash -c", shared, "bash -c 'git log --oneline -5'", false},
		{"a read xargs runs", shared, "xargs git show < commits", false},
		{"a shell running a script file", shared, "bash ./scripts/check.sh <<<'git reset --hard'", false},
		{"bash -c whose string writes nothing", shared, "bash -c 'echo git reset --hard'", false},
		{"a write in its own lane by -c", "/home/you/workspace", "bash -c 'cd " + lane + " && git reset --hard'", false},
		{"a cat heredoc is data", shared, "cat <<'EOF'\ngit reset --hard\nEOF", false},
		{"a quoted substitution is text", shared, "echo '$(git reset --hard)'", false},
		{"GIT_WORK_TREE names the lane", shared, "GIT_DIR=" + lane + "/.git GIT_WORK_TREE=" + lane + " git reset --hard", false},
		{"env -C into the lane", shared, "env -C " + lane + " git reset --hard", false},
		{"sudo -D into the lane", shared, "sudo -D " + lane + " git reset --hard", false},
		{"bash -- names a script file", shared, "bash -- ./x.sh <<<'git reset --hard'", false},
		{"xargs owns the stdin", shared, "xargs bash <<<'git reset --hard'", false},
	} {
		got := sharedtree.Find(c.command, c.cwd, estate) != nil
		if got != c.deny {
			t.Errorf("%s: Find(%q) refused=%v, want %v", c.name, c.command, got, c.deny)
		}
	}
}

// A cd inside a child moves only the child: the git after it still runs where
// the outer command line stood.
func TestACdInsideAChildDoesNotMoveTheOuterWalk(t *testing.T) {
	for _, command := range []string{
		"echo $(cd /home/you/workspace/data-service); git reset --hard",
		"bash -c 'cd /home/you/workspace/data-service'; git reset --hard",
	} {
		if w := sharedtree.Find(command, lane, estate); w != nil {
			t.Errorf("Find(%q) = %+v; the reset runs in the lane", command, w)
		}
	}
}

// Shells nested in heredocs, each with a line the comment rule reads two ways,
// are read well inside the hook's timeout, and the reset after them is found.
func TestNestedShellsAreReadInBoundedTime(t *testing.T) {
	var b strings.Builder
	const depth = 40
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&b, "bash <<D%d\n: # c\n", i)
	}
	for i := depth - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "D%d\n", i)
	}
	b.WriteString("git reset --hard\n")
	start := time.Now()
	w := sharedtree.Find(b.String(), "/home/you/workspace/data-service", estate)
	if took := time.Since(start); took > time.Second {
		t.Errorf("Find took %v on %d nested shells; the hook allows 5s", took, depth)
	}
	if w == nil {
		t.Error("Find = nil; the reset after the nested shells runs in the shared checkout")
	}
}

// The refusal for a nested write reaches the hook's payload entry point.
func TestDenyRefusesASubstitutionWrite(t *testing.T) {
	payload := []byte(`{"tool_name":"Bash","cwd":"/home/you/workspace/data-service","session_id":"mine","tool_input":{"command":"echo $(git reset --hard)"}}`)
	if sharedtree.Deny(payload, estate) == "" {
		t.Error("Deny returned silence for a reset inside $(…) in a shared checkout")
	}
}

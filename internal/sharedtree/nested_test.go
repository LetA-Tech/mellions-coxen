// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"testing"

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

		{"a read in a substitution", shared, "echo $(git status --porcelain)", false},
		{"a read in bash -c", shared, "bash -c 'git log --oneline -5'", false},
		{"a read xargs runs", shared, "xargs git show < commits", false},
		{"a shell running a script file", shared, "bash ./scripts/check.sh <<<'git reset --hard'", false},
		{"bash -c whose string writes nothing", shared, "bash -c 'echo git reset --hard'", false},
		{"a write in its own lane by -c", "/home/you/workspace", "bash -c 'cd " + lane + " && git reset --hard'", false},
		{"a cat heredoc is data", shared, "cat <<'EOF'\ngit reset --hard\nEOF", false},
		{"a quoted substitution is text", shared, "echo '$(git reset --hard)'", false},
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

// The refusal for a nested write reaches the hook's payload entry point.
func TestDenyRefusesASubstitutionWrite(t *testing.T) {
	payload := []byte(`{"tool_name":"Bash","cwd":"/home/you/workspace/data-service","session_id":"mine","tool_input":{"command":"echo $(git reset --hard)"}}`)
	if sharedtree.Deny(payload, estate) == "" {
		t.Error("Deny returned silence for a reset inside $(…) in a shared checkout")
	}
}

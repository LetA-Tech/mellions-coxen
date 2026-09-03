// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package assignment

import (
	"fmt"
	"os/exec"
	"strings"
)

// gitRunner runs git in dir and returns combined output.
//
// Combined rather than stdout alone because git says why it refused on stderr,
// and a worktree that will not detach is exactly the case where the reason
// matters more than the exit code.
func gitRunner(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %s", strings.Join(args, " "), reason(string(out)))
	}
	return out, nil
}

// reason is what git said, with its progress removed.
//
// Taking the first line discarded the cause. git writes progress to stderr
// before it fails, so `worktree add` refusing an existing branch reported
// "Preparing worktree (new branch 'x')" — which is not an error at all, and
// sends the reader looking for a problem that is not there. Observed while
// reopening an assignment whose branch had outlived it.
func reason(out string) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "",
			strings.HasPrefix(line, "Preparing worktree"),
			strings.HasPrefix(line, "HEAD is now at"),
			strings.HasPrefix(line, "Updating files:"),
			strings.HasPrefix(line, "Switched to"),
			strings.HasPrefix(line, "remote:"):
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		// Nothing but progress, which means the failure is the exit code alone.
		return strings.TrimSpace(out)
	}
	return strings.Join(kept, "; ")
}

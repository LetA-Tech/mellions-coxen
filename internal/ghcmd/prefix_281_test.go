// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package ghcmd

import (
	"slices"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/shellsplit"
)

func publishes(noun, verb string) bool {
	return (noun == "pr" || noun == "issue") && (verb == "create" || verb == "comment" || verb == "edit")
}

// found returns the args of the one publishing call in a command line, or
// reports that none was found.
func found(command string) ([]string, bool) {
	for _, c := range shellsplit.Split(command) {
		if args, ok := Args(c.Words, publishes); ok {
			return args, true
		}
	}
	return nil, false
}

// TestAPublishingCallIsFoundWhereverBashRunsIt holds the parse to bash's
// reading of a simple command: a wrapper or a compound keyword before gh still
// runs gh, so the call it runs is the one every guard must see.
func TestAPublishingCallIsFoundWhereverBashRunsIt(t *testing.T) {
	for _, command := range []string{
		`gh pr comment 5 --body 'see a.go:3'`,
		`GH_TOKEN=x /usr/bin/gh pr comment 5 --body 'see a.go:3'`,
		`timeout 60 gh pr comment 5 --body 'see a.go:3'`,
		`timeout -s KILL 60 gh pr comment 5 --body 'see a.go:3'`,
		`command gh pr comment 5 --body 'see a.go:3'`,
		`env gh pr comment 5 --body 'see a.go:3'`,
		`env GH_TOKEN=x gh pr comment 5 --body 'see a.go:3'`,
		`nice -n 5 gh pr comment 5 --body 'see a.go:3'`,
		`nohup gh pr comment 5 --body 'see a.go:3'`,
		`time gh pr comment 5 --body 'see a.go:3'`,
		`for x in 1; do gh pr comment 5 --body 'see a.go:3'; done`,
		`if true; then gh pr comment 5 --body 'see a.go:3'; fi`,
		`if false; then :; else gh pr comment 5 --body 'see a.go:3'; fi`,
		`if false; then :; elif gh pr comment 5 --body 'see a.go:3'; then :; fi`,
		`if gh pr comment 5 --body 'see a.go:3'; then :; fi`,
		`while gh pr comment 5 --body 'see a.go:3'; do break; done`,
		`( gh pr comment 5 --body 'see a.go:3' )`,
		`(gh pr comment 5 --body 'see a.go:3' --repo o/r)`,
		`{ gh pr comment 5 --body 'see a.go:3'; }`,
		`! gh pr comment 5 --body 'see a.go:3'`,
		`xargs gh pr comment 5 --body 'see a.go:3' </dev/null`,
		`stdbuf -oL gh pr comment 5 --body 'see a.go:3'`,
	} {
		t.Run(command, func(t *testing.T) {
			args, ok := found(command)
			if !ok {
				t.Fatalf("no publishing call found in %q: its body would be published ungraded", command)
			}
			i := slices.Index(args, "--body")
			if i < 0 || i+1 >= len(args) || args[i+1] != "see a.go:3" {
				t.Fatalf("args after the verb = %q, want the --body value 'see a.go:3'", args)
			}
		})
	}
}

// TestWhatIsNotAPublishingCallStaysUnread is the control: the widening must not
// read gh out of some other command's quoted text, nor take a gh verb the
// caller does not accept.
func TestWhatIsNotAPublishingCallStaysUnread(t *testing.T) {
	for _, command := range []string{
		`echo "gh pr comment 5 --body 'see a.go:3'"`,
		`git commit -m 'gh pr comment 5 --body x'`,
		`gh pr view 5 --json body`,
		`timeout 60 gh pr view 5`,
		`gh api repos/o/r/issues/5/comments`,
		`timeout 60`,
		`do`,
	} {
		t.Run(command, func(t *testing.T) {
			if args, ok := found(command); ok {
				t.Fatalf("%q was read as a publishing call with args %q", command, args)
			}
		})
	}
}

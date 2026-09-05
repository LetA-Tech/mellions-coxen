// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// archivedState reports whether the remote the load path tracks has been
// archived, which is the one mechanical signal that an installation is dead
// rather than merely behind.
//
// It exists because "0 behind origin/main" is a claim about DISTANCE FROM A
// REMOTE and reads as a claim about being current with the project. Those stop
// being the same thing the moment the project moves, and when they do every
// other line of this diagnostic stays green: the checkout is clean, the
// upstream is reachable, the distance is zero, and none of it is running the
// code anyone is shipping.
//
// What this deliberately does NOT do is compare the remote against the
// `repository` field in the load path's own plugin.json. That was tried against
// the real case and passed it: the superseded checkout declared the superseded
// repository, so both sides agreed and the installation was dead anyway. The
// mismatch was never local to the checkout, which is why the signal has to come
// from the forge.
func archivedState(ctx context.Context, dir string) (archived bool, remote string, known bool) {
	if dir == "" {
		return false, "", false
	}
	remote = originRemote(ctx, dir)
	if remote == "" {
		return false, "", false
	}
	slug := repoSlug(remote)
	if slug == "" {
		return false, remote, false
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return false, remote, false
	}
	c, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "gh", "repo", "view", slug, "--json", "isArchived", "-q", ".isArchived").Output()
	if err != nil {
		// Unreachable, unauthenticated, or a forge that is not GitHub. Unknown
		// is not false: reporting "not archived" from a command that failed is
		// the same substitution this check exists to remove.
		return false, remote, false
	}
	return strings.TrimSpace(string(out)) == "true", remote, true
}

func originRemote(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var slugRe = regexp.MustCompile(`(?:github\.com[:/])([^/]+/[^/]+?)(?:\.git)?/?$`)

// repoSlug extracts owner/name from an https or ssh GitHub remote. A remote it
// cannot parse yields "", which the caller reports as unknown rather than as
// healthy.
func repoSlug(remote string) string {
	m := slugRe.FindStringSubmatch(remote)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// loadPathOriginLine is the doctor row. It is a separate row from "load path
// commit" on purpose: that row answers "is this checkout current with its
// remote", this one answers "is that remote still the project", and merging
// them would reproduce the conflation.
func loadPathOriginLine(ctx context.Context, dir string) (string, string) {
	archived, remote, known := archivedState(ctx, dir)
	switch {
	case remote == "":
		return "unknown", "the load path has no origin remote, so whether it is still the project cannot be established here"
	case !known:
		return "unknown", remote + " — not checked (gh absent, unauthenticated, or not a GitHub remote). " +
			"Distance from a remote is not evidence that the remote is still the project"
	case archived:
		return "ABSENT", remote + " is ARCHIVED — this installation is dead. Every other line here " +
			"reports the checkout against that remote and stays green while nothing shipping is being run. " +
			"Re-install from the current repository: `mellions install -from <checkout>`"
	default:
		return "present", remote + " is live"
	}
}

// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package stale

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// behind builds a remote whose dev is one commit ahead of a local checkout.
// The checkout's working tree holds internal/a.go with `old := legacy()` on
// line 3; dev has removed that line and added internal/fresh.go with
// `added := fresh()` on line 5. It returns the checkout and dev's commit as the
// remote holds it.
func behind(t *testing.T) (checkout, devCommit string) {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "payments-api.git")
	seed := filepath.Join(root, "seed")
	checkout = filepath.Join(root, "payments-api")
	git(t, root, "init", "--quiet", "--bare", "--initial-branch=dev", remote)
	git(t, root, "init", "--quiet", "--initial-branch=dev", seed)
	write(t, seed, "internal/a.go", lines(10, map[int]string{3: "old := legacy()"}))
	git(t, seed, "add", ".")
	git(t, seed, "commit", "--quiet", "-m", "one")
	git(t, seed, "remote", "add", "origin", remote)
	git(t, seed, "push", "--quiet", "origin", "dev")
	git(t, root, "clone", "--quiet", "--branch", "dev", remote, checkout)

	write(t, seed, "internal/a.go", lines(10, nil))
	write(t, seed, "internal/fresh.go", lines(20, map[int]string{5: "added := fresh()"}))
	git(t, seed, "add", ".")
	git(t, seed, "commit", "--quiet", "-m", "two")
	git(t, seed, "push", "--quiet", "origin", "dev")
	return checkout, git(t, remote, "rev-parse", "dev")
}

// TestPremiseIsCheckedAtTheRemoteWorkingBranchNotTheCheckout: a shared
// checkout behind its remote's working branch must neither invent a stale
// premise for code that exists on that branch nor hide one for code that is
// gone from it.
func TestPremiseIsCheckedAtTheRemoteWorkingBranchNotTheCheckout(t *testing.T) {
	co, dev := behind(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	s := New(Options{
		Owner: "example-org", Repos: []string{"payments-api"},
		Checkouts: map[string]string{"payments-api": co},
		Run: runnerFor(t,
			item{Number: 1, Title: "cites code only dev holds", CreatedAt: old(),
				Body: "`internal/fresh.go:5`:\n\n```go\nadded := fresh()\n```\n"},
			item{Number: 2, Title: "cites code dev removed", CreatedAt: old(),
				Body: "`internal/a.go:3`:\n\n```go\nold := legacy()\n```\n"},
		),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	byID := map[string]signal.Signal{}
	for _, g := range got {
		byID[g.ID] = g
	}
	if g, ok := byID["#1"]; ok {
		t.Errorf("a citation that holds on origin/dev was reported stale from the checkout's tree:\n%s", g.Detail)
	}
	g, ok := byID["#2"]
	if !ok {
		t.Fatalf("a quote origin/dev no longer holds was not reported; signals = %+v", got)
	}
	if g.Attrs["checked_commit"] != dev || g.Attrs["checked_ref"] != "origin/dev" {
		t.Errorf("checked_ref/checked_commit = %q/%q, want origin/dev/%s", g.Attrs["checked_ref"], g.Attrs["checked_commit"], dev)
	}
	if g.Attrs["checkout_path"] != co {
		t.Errorf("checkout_path = %q, want the checkout %q", g.Attrs["checkout_path"], co)
	}
	if strings.Contains(g.Detail, tmp) || !strings.Contains(g.Detail, "payments-api@"+dev[:12]) {
		t.Errorf("the finding names the extraction rather than the repository at its commit:\n%s", g.Detail)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("extractions left behind after Collect: %v", left)
	}
}

// TestUnreachableWorkingBranchIsUnreadableNotReadInPlace: a checkout whose
// working branch cannot be fetched is reported as unscanned, never scanned at
// whatever its working tree holds.
func TestUnreachableWorkingBranchIsUnreadableNotReadInPlace(t *testing.T) {
	co, _ := behind(t)
	git(t, co, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	s := New(Options{
		Owner: "example-org", Repos: []string{"payments-api"},
		Checkouts: map[string]string{"payments-api": co},
		Run: runnerFor(t, item{Number: 3, Title: "cites code only dev holds", CreatedAt: old(),
			Body: "`internal/fresh.go:5`:\n\n```go\nadded := fresh()\n```\n"}),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if len(got) != 0 {
		t.Errorf("a checkout whose branch could not be fetched was scanned in place: %+v", got)
	}
	if err == nil || !strings.Contains(err.Error(), "payments-api") || !strings.Contains(err.Error(), "working branch") {
		t.Fatalf("err = %v; want payments-api reported unscanned at its working branch", err)
	}
}

// TestUnreadableSiblingLeavesItsCitationsUncheckedAndIsReported: a checkout
// that is only there to resolve citations into it, and cannot be read, must
// neither turn those citations into "no such path" findings in the repository
// that cites them nor drop out of the report of what went unread.
func TestUnreadableSiblingLeavesItsCitationsUncheckedAndIsReported(t *testing.T) {
	co, _ := behind(t)
	ledger, _ := behind(t)
	write(t, ledger, "internal/ledger/post.go", lines(40, nil))
	git(t, ledger, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	s := New(Options{
		Owner: "example-org", Repos: []string{"payments-api"},
		Checkouts: map[string]string{"payments-api": co, "ledger": ledger},
		Run: runnerFor(t,
			item{Number: 4, Title: "cites the sibling by prefix", CreatedAt: old(),
				Body: "`ledger/internal/ledger/post.go:30` posts the entry."},
			item{Number: 5, Title: "cites a path only the sibling holds", CreatedAt: old(),
				Body: "`internal/ledger/post.go:30` posts the entry."},
		),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	for _, g := range got {
		t.Errorf("%s reported from a sibling that could not be read:\n%s", g.ID, g.Detail)
	}
	if err == nil || !strings.Contains(err.Error(), "ledger") {
		t.Fatalf("err = %v; want the unread sibling ledger reported", err)
	}
}

// TestFindingInASiblingNamesThatSiblingAtItsCommit: a quote that resolves in a
// sibling's tree is attributed to that sibling and the commit it was read at,
// never to the extraction directory that no longer exists.
func TestFindingInASiblingNamesThatSiblingAtItsCommit(t *testing.T) {
	co, _ := behind(t)
	ledger, ledgerDev := behind(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	s := New(Options{
		Owner: "example-org", Repos: []string{"payments-api"},
		Checkouts: map[string]string{"payments-api": co, "ledger": ledger},
		Run: runnerFor(t, item{Number: 6, Title: "quotes the sibling", CreatedAt: old(),
			Body: "`ledger/internal/a.go:3`:\n\n```go\nold := legacy()\n```\n"}),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("a quote the sibling's working branch removed was not reported: %+v", got)
	}
	g := got[0]
	if strings.Contains(g.Detail, tmp) || !strings.Contains(g.Detail, "ledger@"+ledgerDev[:12]+"/internal/a.go") {
		t.Errorf("the finding does not name the sibling at its commit:\n%s", g.Detail)
	}
	if g.Attrs["sibling_commits"] != "ledger@"+ledgerDev {
		t.Errorf("sibling_commits = %q, want ledger@%s", g.Attrs["sibling_commits"], ledgerDev)
	}
}

// TestRefusedFetchStillReadsTheRemoteCommitItAlreadyHas: concurrent fetches
// into one shared checkout leave all but one refused, usually after the winner
// brought the commit. The commit the remote names is read whenever the
// checkout holds it, whatever its remote-tracking ref says.
func TestRefusedFetchStillReadsTheRemoteCommitItAlreadyHas(t *testing.T) {
	co, dev := behind(t)
	git(t, co, "fetch", "--quiet", "--refmap=", "origin", "dev:refs/side/dev")
	gitDir := git(t, co, "rev-parse", "--absolute-git-dir")
	lock := filepath.Join(gitDir, "refs", "remotes", "origin", "dev.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Dir(lock), "dev.lock", "")
	if out, err := exec.Command("git", "-C", co, "fetch", "--quiet", "origin", "dev").CombinedOutput(); err == nil {
		t.Fatalf("precondition: a fetch with origin/dev locked succeeded:\n%s", out)
	}
	s := New(Options{
		Owner: "example-org", Repos: []string{"payments-api"},
		Checkouts: map[string]string{"payments-api": co},
		Run: runnerFor(t, item{Number: 7, Title: "cites code dev removed", CreatedAt: old(),
			Body: "`internal/a.go:3`:\n\n```go\nold := legacy()\n```\n"}),
	})
	got, err := s.Collect(context.Background(), signal.Scope{})
	if err != nil {
		t.Fatalf("a refused fetch made a checkout that holds the commit unreadable: %v", err)
	}
	if len(got) != 1 || got[0].Attrs["checked_commit"] != dev {
		t.Fatalf("want #7 reported at %s; got %+v", dev, got)
	}
}
